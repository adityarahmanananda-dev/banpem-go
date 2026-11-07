package server

import (
	"net/http"
	"strings"
	"time"
)

type searchBantuan struct {
	ID            int64
	Nama          string
	NamaSekolah   string
	Bank          string
	NomorRekening string
}

type searchInvoice struct {
	BantuanID  int64
	Bantuan    string
	InvoiceID  int64
	Tanggal    time.Time
	NomorBukti string
	Uraian     string
	Bruto      int64
}

type searchLedger struct {
	BantuanID  int64
	Bantuan    string
	LedgerID   int64
	Sumber     string
	Tanggal    *time.Time
	NomorBukti string
	Uraian     string
	Jenis      string
	Debit      int64
	Kredit     int64
}

type searchResults struct {
	Query    string
	Grants   []searchBantuan
	Invoices []searchInvoice
	Ledger   []searchLedger
	Total    int
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	data := struct {
		baseView
		searchResults
	}{
		baseView:      baseView{Title: "Pencarian", Active: "search", Q: map[string]string{"q": q}},
		searchResults: searchResults{Query: q},
	}
	if q == "" {
		s.render(w, r, "search.html", data)
		return
	}
	pattern := "%" + q + "%"
	ctx := r.Context()

	rows, err := s.Store.Pool.Query(ctx, `SELECT id, nama, nama_sekolah, bank, nomor_rekening
		FROM bantuan
		WHERE nama ILIKE $1 OR nama_sekolah ILIKE $1 OR bank ILIKE $1 OR nomor_rekening ILIKE $1
		ORDER BY nama LIMIT 25`, pattern)
	if err != nil {
		s.fail(w, r, err, "/search")
		return
	}
	for rows.Next() {
		var b searchBantuan
		if err := rows.Scan(&b.ID, &b.Nama, &b.NamaSekolah, &b.Bank, &b.NomorRekening); err != nil {
			rows.Close()
			s.fail(w, r, err, "/search")
			return
		}
		data.Grants = append(data.Grants, b)
	}
	rows.Close()

	rows, err = s.Store.Pool.Query(ctx, `SELECT i.bantuan_id, b.nama, i.id, i.tanggal, i.nomor_bukti, i.uraian, i.bruto
		FROM trx_invoice i JOIN bantuan b ON b.id=i.bantuan_id
		WHERE i.uraian ILIKE $1 OR i.nomor_bukti ILIKE $1 OR i.nama_rekening ILIKE $1 OR i.npwp ILIKE $1
		ORDER BY i.tanggal DESC, i.id DESC LIMIT 30`, pattern)
	if err != nil {
		s.fail(w, r, err, "/search")
		return
	}
	for rows.Next() {
		var inv searchInvoice
		if err := rows.Scan(&inv.BantuanID, &inv.Bantuan, &inv.InvoiceID, &inv.Tanggal,
			&inv.NomorBukti, &inv.Uraian, &inv.Bruto); err != nil {
			rows.Close()
			s.fail(w, r, err, "/search")
			return
		}
		data.Invoices = append(data.Invoices, inv)
	}
	rows.Close()

	rows, err = s.Store.Pool.Query(ctx, `
		SELECT bantuan_id, bantuan, ledger_id, sumber, tanggal, nomor_bukti, uraian, jenis, debit, kredit FROM (
			SELECT l.bantuan_id, b.nama AS bantuan, l.id AS ledger_id, 'BKU' AS sumber, l.tanggal,
			       l.nomor_bukti, l.uraian, l.jenis_transaksi AS jenis, l.debit, l.kredit
			FROM trx_ledger l JOIN bantuan b ON b.id=l.bantuan_id
			WHERE l.uraian ILIKE $1 OR l.nomor_bukti ILIKE $1
			UNION ALL
			SELECT l.bantuan_id, b.nama, l.id, 'Bank', l.tanggal, l.nomor_bukti, l.uraian,
			       l.jenis_transaksi, l.debit, l.kredit
			FROM trx_bank_ledger l JOIN bantuan b ON b.id=l.bantuan_id
			WHERE l.uraian ILIKE $1 OR l.nomor_bukti ILIKE $1
		) x
		ORDER BY tanggal DESC NULLS LAST, ledger_id DESC LIMIT 40`, pattern)
	if err != nil {
		s.fail(w, r, err, "/search")
		return
	}
	for rows.Next() {
		var l searchLedger
		if err := rows.Scan(&l.BantuanID, &l.Bantuan, &l.LedgerID, &l.Sumber, &l.Tanggal,
			&l.NomorBukti, &l.Uraian, &l.Jenis, &l.Debit, &l.Kredit); err != nil {
			rows.Close()
			s.fail(w, r, err, "/search")
			return
		}
		data.Ledger = append(data.Ledger, l)
	}
	rows.Close()
	data.Total = len(data.Grants) + len(data.Invoices) + len(data.Ledger)
	s.audit(r, nil, "search", "system", nil, map[string]any{"q": q, "results": data.Total})
	s.render(w, r, "search.html", data)
}
