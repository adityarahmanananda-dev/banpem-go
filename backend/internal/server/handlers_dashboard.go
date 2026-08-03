package server

import (
	"context"
	"net/http"

	"ebku/internal/store"
)

type dashboardStat struct {
	ID            int64
	Nama          string
	NamaSekolah   string
	NamaRekening  string
	NomorRekening string
	Bank          string
	Nominal       *int64
	SaldoAwal     int64
	TotalTurun    int64
	TotalDebit    int64
	TotalKredit   int64
	SaldoBKU      int64
	InvoiceCount  int64
	TotalBelanja  int64
	TotalPajak    int64
	JasaGiro      int64
	SaldoBank     int64
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.Store.Pool.Query(ctx, `SELECT b.id, b.nama, b.nama_sekolah, b.nama_rekening, b.nomor_rekening, b.bank, b.nominal,
		COALESCE(sa.saldo_awal,0) AS saldo_awal,
		COALESCE(pc.total,0) AS total_turun,
		COALESCE(ls.debit,0) AS total_debit,
		COALESCE(ls.kredit,0) AS total_kredit,
		COALESCE(le.saldo,0) AS saldo_bku,
		COALESCE(i.cnt,0) AS invoice_count,
		COALESCE(i.total,0) AS total_belanja,
		COALESCE(ip.total,0) AS total_pajak,
		COALESCE(jg.total,0) AS jasa_giro,
		COALESCE(be.saldo,0) AS saldo_bank
		FROM bantuan b
		LEFT JOIN saldo_awal sa ON sa.bantuan_id=b.id
		LEFT JOIN (SELECT bantuan_id, SUM(nominal) total FROM pencairan_hibah GROUP BY bantuan_id) pc ON pc.bantuan_id=b.id
		LEFT JOIN (SELECT bantuan_id, SUM(debit) debit, SUM(kredit) kredit FROM trx_ledger GROUP BY bantuan_id) ls ON ls.bantuan_id=b.id
		LEFT JOIN LATERAL (SELECT saldo FROM trx_ledger WHERE bantuan_id=b.id ORDER BY nomor DESC LIMIT 1) le ON true
		LEFT JOIN (SELECT bantuan_id, COUNT(*) cnt, SUM(bruto) total FROM trx_invoice GROUP BY bantuan_id) i ON i.bantuan_id=b.id
		LEFT JOIN (SELECT bantuan_id, SUM(nilai_ppn)+SUM(nilai_pph) total FROM trx_invoice GROUP BY bantuan_id) ip ON ip.bantuan_id=b.id
		LEFT JOIN (SELECT bantuan_id, SUM(nominal) total FROM jasa_giro GROUP BY bantuan_id) jg ON jg.bantuan_id=b.id
		LEFT JOIN LATERAL (SELECT saldo FROM trx_bank_ledger WHERE bantuan_id=b.id ORDER BY nomor DESC LIMIT 1) be ON true
		ORDER BY b.id`)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	defer rows.Close()
	var stats []dashboardStat
	for rows.Next() {
		var st dashboardStat
		if err := rows.Scan(&st.ID, &st.Nama, &st.NamaSekolah, &st.NamaRekening, &st.NomorRekening, &st.Bank, &st.Nominal,
			&st.SaldoAwal, &st.TotalTurun, &st.TotalDebit, &st.TotalKredit, &st.SaldoBKU,
			&st.InvoiceCount, &st.TotalBelanja, &st.TotalPajak, &st.JasaGiro, &st.SaldoBank); err != nil {
			s.fail(w, r, err, "/")
			return
		}
		stats = append(stats, st)
	}
	if err := rows.Err(); err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Stats []dashboardStat
	}{
		baseView: baseView{Title: "Dashboard", Active: "dashboard", FlashType: ft, FlashMsg: fm, Q: map[string]string{}},
		Stats:    stats,
	}
	s.render(w, r, "dashboard.html", data)
}

// summary menghitung angka ringkas untuk sidebar sebuah bantuan.
func (s *Server) summary(ctx context.Context, id int64, b *store.Bantuan) *bantuanSummary {
	sm := &bantuanSummary{Bantuan: b}
	if b == nil {
		return sm
	}
	st, _ := s.Store.GetSaldoAwal(ctx, id)
	if st != nil {
		sm.SaldoAwal = st.Saldo
	}
	sm.TotalTurun = s.Store.TotalTurun(ctx, id)
	sm.JasaGiro = s.Store.TotalJasaGiro(ctx, id)
	_ = s.Store.Pool.QueryRow(ctx, `SELECT
		(SELECT COALESCE(SUM(debit),0) FROM trx_ledger WHERE bantuan_id=$1),
		(SELECT COALESCE(SUM(kredit),0) FROM trx_ledger WHERE bantuan_id=$1),
		(SELECT saldo FROM trx_ledger WHERE bantuan_id=$1 ORDER BY nomor DESC LIMIT 1),
		(SELECT saldo FROM trx_bank_ledger WHERE bantuan_id=$1 ORDER BY nomor DESC LIMIT 1),
		(SELECT COUNT(*) FROM trx_invoice WHERE bantuan_id=$1),
		(SELECT COALESCE(SUM(bruto),0) FROM trx_invoice WHERE bantuan_id=$1),
		(SELECT COALESCE(SUM(nilai_ppn),0)+COALESCE(SUM(nilai_pph),0) FROM trx_invoice WHERE bantuan_id=$1)`,
		id).Scan(&sm.TotalDebit, &sm.TotalKredit, &sm.SaldoBKU, &sm.SaldoBank, &sm.InvoiceCount, &sm.TotalBelanja, &sm.TotalPajak)
	_ = s.Store.Pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM kegiatan WHERE bantuan_id=$1),
		(SELECT COUNT(*) FROM sub_kegiatan sk JOIN kegiatan k ON k.id=sk.kegiatan_id WHERE k.bantuan_id=$1),
		(SELECT COUNT(*) FROM komponen ko JOIN sub_kegiatan sk ON sk.id=ko.sub_kegiatan_id JOIN kegiatan k ON k.id=sk.kegiatan_id WHERE k.bantuan_id=$1)`,
		id).Scan(&sm.KegiatanCount, &sm.SubKegiatanCount, &sm.KomponenCount)
	return sm
}
