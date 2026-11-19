package store

import (
	"context"
	"os"
	"testing"
	"time"

	"ebku/internal/migrations"
)

// TestIntegrationPostgres dijalankan hanya jika TEST_DATABASE_URL diset.
// Contoh:
//
//	TEST_DATABASE_URL=postgres://ebku:test@127.0.0.1:5432/ebku_test?sslmode=disable go test ./internal/store -run Integration
func TestIntegrationPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL tidak diset")
	}
	ctx := context.Background()
	st, err := New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		t.Fatal(err)
	}

	nominal := int64(100_000_000)
	bid, err := st.CreateBantuan(ctx, Bantuan{Nama: "Integration Test", NamaSekolah: "Sekolah Uji", Nominal: &nominal})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Pool.Exec(context.Background(), `DELETE FROM bantuan WHERE id=$1`, bid)

	inv := InvoiceInput{
		Tanggal:    time.Now(),
		NomorBukti: "INV-INT-1",
		Uraian:     "Uji integrasi",
		Bruto:      10_000_000,
		JenisMenu:  1,
		FlagPpn:    2,
		FlagPph:    2, // PPh 23 2%
	}
	iid, err := st.CreateInvoice(ctx, bid, inv)
	if err != nil {
		t.Fatal(err)
	}
	if iid == 0 {
		t.Fatal("invoice id kosong")
	}
	if _, err := st.CreateInvoice(ctx, bid, inv); err != ErrDuplicateInvoiceNo {
		t.Fatalf("duplikat nomor bukti: err=%v", err)
	}

	// Tagihan dengan PPh 23 harus bisa disetor sekali saja.
	if err := st.CreateSetorPajak(ctx, SetorInput{
		BantuanID: bid, JenisPajak: "PPh 23", InvoiceIDs: []int64{iid},
		Tanggal: time.Now(), NTPN: "NTPN-INT",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSetorPajak(ctx, SetorInput{
		BantuanID: bid, JenisPajak: "PPh 23", InvoiceIDs: []int64{iid},
		Tanggal: time.Now(), NTPN: "NTPN-INT-2",
	}); err != ErrTaxAlreadyDeposited {
		t.Fatalf("setor ganda: err=%v", err)
	}
	if err := st.UpdateInvoice(ctx, bid, iid, inv); err != ErrInvoiceHasTaxDeposit {
		t.Fatalf("edit setelah setor: err=%v", err)
	}
	if err := st.DeleteInvoice(ctx, bid, iid); err != ErrInvoiceHasTaxDeposit {
		t.Fatalf("hapus setelah setor: err=%v", err)
	}
}

// TestIntegrationRealisasiPivot memastikan komponen/aktivitas yang belum punya
// realisasi tetap muncul di rekap realisasi (pivot) dengan nilai 0.
func TestIntegrationRealisasiPivot(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL tidak diset")
	}
	ctx := context.Background()
	st, err := New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		t.Fatal(err)
	}

	nominal := int64(100_000_000)
	bid, err := st.CreateBantuan(ctx, Bantuan{Nama: "Integration Pivot", NamaSekolah: "Sekolah Uji", Nominal: &nominal})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Pool.Exec(context.Background(), `DELETE FROM bantuan WHERE id=$1`, bid)

	kid, err := st.CreateKegiatan(ctx, bid, "Kegiatan A")
	if err != nil {
		t.Fatal(err)
	}
	skid, err := st.CreateSubKegiatan(ctx, bid, kid, "Sub A")
	if err != nil {
		t.Fatal(err)
	}
	aid, err := st.CreateAktivitas(ctx, bid, skid, "Aktivitas A")
	if err != nil {
		t.Fatal(err)
	}
	realizedID, err := st.CreateKomponen(ctx, bid, aid, "Komponen Direalisasi", 5_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateKomponen(ctx, bid, aid, "Komponen Belum Direalisasi", 7_000_000); err != nil {
		t.Fatal(err)
	}

	if _, err := st.CreateInvoice(ctx, bid, InvoiceInput{
		Tanggal: time.Now(), NomorBukti: "INV-PIVOT-1", Uraian: "Uji pivot",
		Bruto: 1_000_000, JenisMenu: 1, FlagPpn: 0, FlagPph: 0,
		Realisasi: []RealisasiInput{{KomponenID: realizedID, Bruto: 1_000_000}},
	}); err != nil {
		t.Fatal(err)
	}

	rows, total, err := st.ListRealisasiPivot(ctx, bid, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("jumlah baris pivot = %d, ingin 2 (termasuk yang belum direalisasi): %+v", len(rows), rows)
	}
	byName := map[string]PivotRow{}
	for _, r := range rows {
		byName[r.Komponen] = r
	}
	if r := byName["Komponen Direalisasi"]; r.Pagu != 5_000_000 || r.Realisasi != 1_000_000 {
		t.Errorf("komponen direalisasi: pagu=%d realisasi=%d", r.Pagu, r.Realisasi)
	}
	if r := byName["Komponen Belum Direalisasi"]; r.Pagu != 7_000_000 || r.Realisasi != 0 {
		t.Errorf("komponen belum direalisasi: pagu=%d realisasi=%d, ingin 7000000/0", r.Pagu, r.Realisasi)
	}
	if total.Pagu != 12_000_000 || total.Realisasi != 1_000_000 {
		t.Errorf("total pivot: pagu=%d realisasi=%d", total.Pagu, total.Realisasi)
	}
}
