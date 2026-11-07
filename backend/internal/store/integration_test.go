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
