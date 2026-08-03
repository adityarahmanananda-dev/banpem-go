package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"ebku/internal/money"
	"ebku/internal/store"
)

type versiInfo struct {
	Nominal    int64
	TotalTurun int64
	ShowTabs   bool
	Versi      string
	Base       int64
}

func (s *Server) versiInfo(ctx context.Context, id int64, versi string) versiInfo {
	b, _ := s.Store.GetBantuan(ctx, id)
	nominal := int64(0)
	if b.Nominal != nil {
		nominal = *b.Nominal
	}
	if nominal == 0 {
		sa, _ := s.Store.GetSaldoAwal(ctx, id)
		if sa != nil {
			nominal = sa.Saldo
		}
	}
	total := s.Store.TotalTurun(ctx, id)
	vi := versiInfo{Nominal: nominal, TotalTurun: total, Versi: "rencana", Base: nominal}
	if versi == "real" {
		vi.Versi = "real"
	}
	vi.ShowTabs = nominal > 0 && total < nominal
	if vi.ShowTabs && vi.Versi == "real" {
		vi.Base = s.Store.FirstPencairanNominal(ctx, id)
	}
	return vi
}

type ledgerView struct {
	store.Ledger
	KreditDisp int64
	SaldoDisp  int64
}

// applyVersion menghitung ulang saldo berjalan untuk tab aktif. Base adalah
// nilai baris 'saldo_awal': total hibah (nominal) di tab rencana, atau nilai
// pencairan pertama di tab real. Baris saldo_awal hanya mengisi kolom SALDO;
// kolom kredit dibiarkan kosong.
func applyVersion(rows []store.Ledger, base int64) []ledgerView {
	out := make([]ledgerView, 0, len(rows))
	running := base
	for _, e := range rows {
		lv := ledgerView{Ledger: e, KreditDisp: e.Kredit, SaldoDisp: e.Saldo}
		if e.JenisTransaksi == "saldo_awal" {
			lv.KreditDisp = 0
			lv.SaldoDisp = base
			running = base
		} else {
			running = running - e.Debit + e.Kredit
			lv.SaldoDisp = running
		}
		out = append(out, lv)
	}
	return out
}

func (s *Server) handleBKU(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	vi := s.versiInfo(r.Context(), id, r.URL.Query().Get("versi"))
	rows, err := store.ListLedger(r.Context(), s.Store.Pool, id, "bku")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	views := applyVersion(rows, vi.Base)
	var totDebit, totKredit int64
	for _, v := range views {
		totDebit += v.Debit
		totKredit += v.KreditDisp
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Rows      []ledgerView
		TotDebit  int64
		TotKredit int64
		Versi     versiInfo
	}{
		baseView: baseView{Title: "Buku Kas Umum", Active: "bku", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Rows: views, TotDebit: totDebit, TotKredit: totKredit, Versi: vi,
	}
	s.render(w, r, "bku.html", data)
}

func (s *Server) handleBKUReorder(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	var body struct {
		Order []int64 `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.Store.ReorderLedger(r.Context(), id, "bku", body.Order); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleBKUTambahGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		E store.Ledger
	}{
		baseView: baseView{Title: "Input Manual BKU", Active: "bku", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
	}
	s.render(w, r, "bku_manual_form.html", data)
}

func parseLedgerForm(r *http.Request) store.LedgerEntry {
	debit, _ := money.Parse(formStr(r, "debit"))
	kredit, _ := money.Parse(formStr(r, "kredit"))
	tgl := store.ParseDate(formStr(r, "tanggal"))
	return store.LedgerEntry{
		Tanggal:        &tgl,
		NomorBukti:     formStr(r, "nomor_bukti"),
		Uraian:         formStr(r, "uraian"),
		Debit:          debit,
		Kredit:         kredit,
		JenisTransaksi: formStr(r, "jenis_transaksi"),
	}
}

func (s *Server) handleBKUTambahPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	e := parseLedgerForm(r)
	if e.JenisTransaksi == "" {
		e.JenisTransaksi = "manual"
	}
	if e.Uraian == "" {
		s.setFlash(w, "danger", "Uraian wajib diisi.")
		s.redirect(w, r, "/bantuan/"+i64s(id)+"/bku/tambah")
		return
	}
	e.BantuanID = id
	if err := s.Store.CreateManualLedger(r.Context(), id, e); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/bku/tambah")
		return
	}
	s.setFlash(w, "success", "Entri manual berhasil ditambahkan.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/bku")
}

func (s *Server) handleBKUEditGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	eid, err := pathID(r, "eid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	e, err := s.Store.GetLedgerEntry(r.Context(), eid, "bku")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if e.InvoiceID != nil {
		s.fail(w, r, errInvalidManual, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		E store.Ledger
	}{
		baseView: baseView{Title: "Edit Entri Manual", Active: "bku", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		E: e,
	}
	s.render(w, r, "bku_manual_form.html", data)
}

var errInvalidManual = errManual("entri ini tidak dapat diedit")

type errManual string

func (e errManual) Error() string { return string(e) }

func (s *Server) handleBKUEditPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	eid, err := pathID(r, "eid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	e := parseLedgerForm(r)
	e.BantuanID = id
	if err := s.Store.UpdateManualLedger(r.Context(), eid, e); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/bku/edit/"+i64s(eid))
		return
	}
	s.setFlash(w, "success", "Entri berhasil diperbarui.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/bku")
}

func (s *Server) handleBKUHapus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	eid, err := pathID(r, "eid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if err := s.Store.DeleteManualLedger(r.Context(), eid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/bku")
		return
	}
	s.setFlash(w, "success", "Entri berhasil dihapus.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/bku")
}

func (s *Server) handleBKUDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	eid, err := pathID(r, "eid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	e, err := s.Store.GetLedgerEntry(r.Context(), eid, "bku")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	data := struct {
		baseView
		E store.Ledger
	}{
		baseView: baseView{Title: "Detail Entri", Active: "bku", Bantuan: &b,
			Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		E: e,
	}
	s.render(w, r, "bku_detail.html", data)
}

// ---- setor pajak ----

type setorFormView struct {
	baseView
	Jenis      string
	Invoices   []setorInvoiceView
	SetorID    int64
	Tanggal    string
	NTBN       string
	NTPN       string
	Keterangan string
	IsEdit     bool
}

type setorInvoiceView struct {
	Invoice store.Invoice
	Value   int64
	Paid    bool
	Checked bool
}

func (s *Server) setorFormData(r *http.Request, sid int64, edit bool) (*setorFormView, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		return nil, err
	}
	v := &setorFormView{
		baseView: baseView{Title: "Setor Pajak", Active: "bku",
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Jenis:   formStr(r, "jenis"),
		IsEdit:  edit,
		Tanggal: store.TodayString(),
	}
	if v.Jenis == "" {
		v.Jenis = "PPN"
	}
	if edit && sid > 0 {
		var oldJenis string
		if err := s.Store.Pool.QueryRow(r.Context(), `SELECT jenis_pajak FROM rekap_pajak WHERE setor_ledger_id=$1 AND jenis_pajak<>'' ORDER BY id LIMIT 1`, sid).Scan(&oldJenis); err == nil && oldJenis != "" {
			v.Jenis = oldJenis
		}
	}
	paid, err := s.Store.ListSetorJenisPerInvoice(r.Context(), id)
	if err != nil {
		return nil, err
	}
	checked := map[int64]bool{}
	if edit && sid > 0 {
		rows, err := s.Store.Pool.Query(r.Context(), `SELECT invoice_id FROM rekap_pajak WHERE setor_ledger_id=$1 AND invoice_id IS NOT NULL`, sid)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var iid int64
			if rows.Scan(&iid) == nil {
				checked[iid] = true
			}
		}
		rows.Close()
		le, err := s.Store.GetLedgerEntry(r.Context(), sid, "bku")
		if err == nil {
			v.SetorID = sid
			if le.Tanggal != nil {
				v.Tanggal = le.Tanggal.Format("2006-01-02")
			}
			v.NTBN = extractNoBukti(le.NomorBukti, "NTB")
			v.NTPN = extractNoBukti(le.NomorBukti, "NTPN")
			if le.Uraian != "" && le.Uraian != "Penyetoran "+v.Jenis {
				v.Keterangan = le.Uraian
			}
		}
	}
	invoices, err := s.Store.ListInvoices(r.Context(), id, "")
	if err != nil {
		return nil, err
	}
	for _, inv := range invoices {
		val := store.TaxValueForJenis(inv, v.Jenis)
		if val <= 0 {
			continue
		}
		siv := setorInvoiceView{Invoice: inv, Value: val}
		for _, p := range paid[inv.ID] {
			if p == v.Jenis {
				siv.Paid = true
			}
		}
		if checked[inv.ID] {
			siv.Checked = true
		}
		v.Invoices = append(v.Invoices, siv)
	}
	return v, nil
}

func extractNoBukti(nb, prefix string) string {
	for _, line := range splitLines(nb) {
		if idx := indexOf(line, prefix+"="); idx >= 0 {
			return line[idx+len(prefix)+1:]
		}
	}
	return ""
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func (s *Server) handleSetorGet(w http.ResponseWriter, r *http.Request) {
	v, err := s.setorFormData(r, 0, false)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	v.FlashType, v.FlashMsg = ft, fm
	s.render(w, r, "setor_form.html", v)
}

func parseSetorForm(r *http.Request) (store.SetorInput, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return store.SetorInput{}, err
	}
	in := store.SetorInput{
		BantuanID:  id,
		JenisPajak: formStr(r, "jenis_pajak"),
		Tanggal:    store.ParseDate(formStr(r, "tanggal")),
		NTBN:       formStr(r, "ntbn"),
		NTPN:       formStr(r, "ntpn"),
		Keterangan: formStr(r, "keterangan"),
	}
	for _, v := range r.Form["invoice_ids"] {
		iid, _ := strconv.ParseInt(v, 10, 64)
		if iid > 0 {
			in.InvoiceIDs = append(in.InvoiceIDs, iid)
		}
	}
	return in, nil
}

func (s *Server) handleSetorPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	in, err := parseSetorForm(r)
	if err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/bku/setor-pajak")
		return
	}
	if len(in.InvoiceIDs) == 0 {
		s.setFlash(w, "danger", "Pilih minimal satu tagihan.")
		s.redirect(w, r, "/bantuan/"+i64s(id)+"/bku/setor-pajak")
		return
	}
	if err := s.Store.CreateSetorPajak(r.Context(), in); err != nil {
		if err == store.ErrNoTaxValue {
			s.setFlash(w, "danger", "Tidak ada nilai pajak untuk tagihan terpilih.")
			s.redirect(w, r, "/bantuan/"+i64s(id)+"/bku/setor-pajak")
			return
		}
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/bku/setor-pajak")
		return
	}
	s.setFlash(w, "success", "Penyetoran pajak berhasil disimpan.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/rekap-pajak")
}

func (s *Server) handleSetorDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	sid, err := pathID(r, "sid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	e, err := s.Store.GetLedgerEntry(r.Context(), sid, "bku")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	rows, err := s.Store.Pool.Query(r.Context(), `SELECT rp.ntbn, rp.ntpn, rp.invoice_id, rp.jenis_pajak, i.uraian
		FROM rekap_pajak rp LEFT JOIN trx_invoice i ON i.id=rp.invoice_id
		WHERE rp.setor_ledger_id=$1 ORDER BY rp.id`, sid)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	defer rows.Close()
	type rpRow struct {
		NTBN   string
		NTPN   string
		IID    *int64
		Jenis  string
		Uraian string
	}
	var rekap []rpRow
	for rows.Next() {
		var rp rpRow
		if err := rows.Scan(&rp.NTBN, &rp.NTPN, &rp.IID, &rp.Jenis, &rp.Uraian); err != nil {
			s.fail(w, r, err, "/")
			return
		}
		rekap = append(rekap, rp)
	}
	data := struct {
		baseView
		E     store.Ledger
		Rekap []rpRow
	}{
		baseView: baseView{Title: "Detail Setor Pajak", Active: "bku", Bantuan: &b,
			Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		E: e, Rekap: rekap,
	}
	s.render(w, r, "setor_detail.html", data)
}

func (s *Server) handleSetorEditGet(w http.ResponseWriter, r *http.Request) {
	sid, err := pathID(r, "sid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	v, err := s.setorFormData(r, sid, true)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	v.FlashType, v.FlashMsg = ft, fm
	s.render(w, r, "setor_form.html", v)
}

func (s *Server) handleSetorEditPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	sid, err := pathID(r, "sid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	in, err := parseSetorForm(r)
	if err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/bku/setor-pajak/edit/"+i64s(sid))
		return
	}
	if len(in.InvoiceIDs) == 0 {
		s.setFlash(w, "danger", "Pilih minimal satu tagihan.")
		s.redirect(w, r, "/bantuan/"+i64s(id)+"/bku/setor-pajak/edit/"+i64s(sid))
		return
	}
	if err := s.Store.UpdateSetorPajak(r.Context(), sid, in); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/bku/setor-pajak/edit/"+i64s(sid))
		return
	}
	s.setFlash(w, "success", "Penyetoran pajak berhasil diperbarui.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/rekap-pajak")
}

func (s *Server) handleSetorHapus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	sid, err := pathID(r, "sid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if err := s.Store.DeleteSetorPajak(r.Context(), sid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/rekap-pajak")
		return
	}
	s.setFlash(w, "success", "Penyetoran pajak berhasil dihapus.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/rekap-pajak")
}
