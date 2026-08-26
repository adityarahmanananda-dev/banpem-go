package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"ebku/internal/money"
	"ebku/internal/store"
	"ebku/internal/tax"
)

type realisasiView struct {
	KomponenID    int64
	KegiatanID    int64
	SubKegiatanID int64
	AktivitasID   int64
	Bruto         int64
	Kegiatan      string
	SubKegiatan   string
	Aktivitas     string
	Komponen      string
}

type invoiceFormView struct {
	baseView
	Inv       *store.Invoice
	Realisasi []realisasiView
	Kegiatans []store.KegiatanTree
	JenisMenu int
	FlagPpn   int
	FlagPph   int
	Kategori  *int
	IsEdit    bool
}

func (s *Server) invoiceFormData(r *http.Request, iid int64, edit bool) (*invoiceFormView, *store.Bantuan, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, nil, err
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		return nil, nil, err
	}
	kegs, err := s.Store.ListKegiatanTree(r.Context(), id)
	if err != nil {
		return nil, nil, err
	}
	v := &invoiceFormView{
		baseView: baseView{Title: "Input Tagihan", Active: "tagihan",
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Kegiatans: kegs,
		IsEdit:    edit,
	}
	if edit && iid > 0 {
		inv, err := s.Store.GetInvoice(r.Context(), iid)
		if err != nil {
			return nil, nil, err
		}
		v.Inv = &inv
		v.JenisMenu = inv.JenisMenu
		v.FlagPpn = inv.FlagPpn
		v.FlagPph = inv.FlagPph
		v.Kategori = inv.KategoriNarasumber
		reals, err := s.Store.ListRealisasi(r.Context(), iid)
		if err != nil {
			return nil, nil, err
		}
		for _, rl := range reals {
			ch, err := s.Store.GetKomponenChain(r.Context(), rl.KomponenID)
			if err != nil {
				continue
			}
			v.Realisasi = append(v.Realisasi, realisasiView{
				KomponenID: rl.KomponenID, Bruto: rl.Bruto,
				KegiatanID: ch.KegiatanID, SubKegiatanID: ch.SubID, AktivitasID: ch.AktivitasID,
				Kegiatan: ch.KegiatanNama, SubKegiatan: ch.SubNama, Aktivitas: ch.AktivitasNama, Komponen: ch.KomponenNama,
			})
		}
	}
	return v, &b, nil
}

func (s *Server) handleTagihanGet(w http.ResponseWriter, r *http.Request) {
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
	invoices, err := s.Store.ListInvoices(r.Context(), id, "sort")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Invoices []store.Invoice
	}{
		baseView: baseView{Title: "Daftar Tagihan", Active: "tagihan", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Invoices: invoices,
	}
	s.render(w, r, "tagihan_list.html", data)
}

func (s *Server) handleTagihanTambahGet(w http.ResponseWriter, r *http.Request) {
	v, _, err := s.invoiceFormData(r, 0, false)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	s.render(w, r, "tagihan_form.html", v)
}

func parseInvoiceForm(r *http.Request) (store.InvoiceInput, error) {
	bruto, err := money.Parse(formStr(r, "bruto_utama"))
	if err != nil {
		return store.InvoiceInput{}, err
	}
	in := store.InvoiceInput{
		Tanggal:              store.ParseDate(formStr(r, "tanggal")),
		NomorBukti:           formStr(r, "nomor_bukti"),
		Uraian:               formStr(r, "uraian"),
		Bruto:                bruto,
		JenisMenu:            int(formInt(r, "jenis_menu")),
		FlagPpn:              int(formInt(r, "flag_ppn")),
		FlagPph:              int(formInt(r, "flag_pph")),
		NamaRekening:         formStr(r, "nama_rekening"),
		NomorRekening:        formStr(r, "nomor_rekening"),
		Bank:                 formStr(r, "bank"),
		NPWP:                 formStr(r, "npwp"),
		NomorBupotPPN:        formStr(r, "nomor_bupot_ppn"),
		NomorBupotPPH:        formStr(r, "nomor_bupot_pph"),
		BiayaAdminDibebankan: formStr(r, "biaya_admin_dibebankan"),
	}
	if in.JenisMenu == 0 {
		in.JenisMenu = 1
	}
	if in.FlagPpn == 0 {
		in.FlagPpn = 2
	}
	if in.FlagPph == 0 {
		in.FlagPph = 3
	}
	if in.BiayaAdminDibebankan == "" {
		in.BiayaAdminDibebankan = "tidak"
	}
	// Koreksi nilai pajak (bila diisi, menggantikan hasil perhitungan).
	if s := formStr(r, "ppn"); s != "" {
		if v, err := money.Parse(s); err == nil {
			in.PPNOverride = &v
		}
	}
	if s := formStr(r, "pph"); s != "" {
		if v, err := money.Parse(s); err == nil {
			in.PPHOverride = &v
		}
	}
	if k := formStr(r, "kategori_narasumber"); k != "" {
		ki := int(formInt(r, "kategori_narasumber"))
		in.KategoriNarasumber = &ki
	}
	komponenIDs := r.Form["komponen_id"]
	brutos := r.Form["bruto"]
	for i := range komponenIDs {
		if i >= len(brutos) {
			break
		}
		kid, _ := strconv.ParseInt(komponenIDs[i], 10, 64)
		bv, _ := money.Parse(brutos[i])
		if kid <= 0 || bv <= 0 {
			continue
		}
		in.Realisasi = append(in.Realisasi, store.RealisasiInput{KomponenID: kid, Bruto: bv})
	}
	return in, nil
}

func (s *Server) handleTagihanPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	in, err := parseInvoiceForm(r)
	if err != nil {
		s.setFlash(w, "danger", "Nominal bruto tidak valid.")
		s.redirect(w, r, "/bantuan/"+i64s(id)+"/tagihan/tambah")
		return
	}
	if in.Uraian == "" {
		s.setFlash(w, "danger", "Uraian wajib diisi.")
		s.redirect(w, r, "/bantuan/"+i64s(id)+"/tagihan/tambah")
		return
	}
	if _, err := s.Store.CreateInvoice(r.Context(), id, in); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/tagihan/tambah")
		return
	}
	s.setFlash(w, "success", "Tagihan berhasil disimpan.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/tagihan")
}

func (s *Server) handleTagihanEditGet(w http.ResponseWriter, r *http.Request) {
	iid, err := pathID(r, "iid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	v, _, err := s.invoiceFormData(r, iid, true)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	s.render(w, r, "tagihan_form.html", v)
}

func (s *Server) handleTagihanEditPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	iid, err := pathID(r, "iid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	in, err := parseInvoiceForm(r)
	if err != nil {
		s.setFlash(w, "danger", "Nominal bruto tidak valid.")
		s.redirect(w, r, "/bantuan/"+i64s(id)+"/tagihan/edit/"+i64s(iid))
		return
	}
	if err := s.Store.UpdateInvoice(r.Context(), iid, in); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/tagihan/edit/"+i64s(iid))
		return
	}
	s.setFlash(w, "success", "Tagihan berhasil diperbarui.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/tagihan")
}

func (s *Server) handleTagihanHapus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	iid, err := pathID(r, "iid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if err := s.Store.DeleteInvoice(r.Context(), iid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/tagihan")
		return
	}
	s.setFlash(w, "success", "Tagihan berhasil dihapus.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/tagihan")
}

func (s *Server) handleTagihanDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	iid, err := pathID(r, "iid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	inv, err := s.Store.GetInvoice(r.Context(), iid)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	reals, _ := s.Store.ListRealisasi(r.Context(), iid)
	var rv []realisasiView
	for _, rl := range reals {
		ch, err := s.Store.GetKomponenChain(r.Context(), rl.KomponenID)
		if err != nil {
			continue
		}
		rv = append(rv, realisasiView{
			KomponenID: rl.KomponenID, Bruto: rl.Bruto,
			KegiatanID: ch.KegiatanID, SubKegiatanID: ch.SubID, AktivitasID: ch.AktivitasID,
			Kegiatan: ch.KegiatanNama, SubKegiatan: ch.SubNama, Aktivitas: ch.AktivitasNama, Komponen: ch.KomponenNama,
		})
	}
	ledgerRows, _ := store.ListLedger(r.Context(), s.Store.Pool, id, "bku")
	bankRows, _ := store.ListLedger(r.Context(), s.Store.Pool, id, "bank")
	var bku []store.Ledger
	var bank []store.Ledger
	for _, e := range ledgerRows {
		if e.InvoiceID != nil && *e.InvoiceID == iid {
			bku = append(bku, e)
		}
	}
	for _, e := range bankRows {
		if e.InvoiceID != nil && *e.InvoiceID == iid {
			bank = append(bank, e)
		}
	}
	data := struct {
		baseView
		Inv       store.Invoice
		Realisasi []realisasiView
		BKU       []store.Ledger
		Bank      []store.Ledger
	}{
		baseView: baseView{Title: "Detail Tagihan", Active: "tagihan", Bantuan: &b,
			Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Inv: inv, Realisasi: rv, BKU: bku, Bank: bank,
	}
	s.render(w, r, "tagihan_detail.html", data)
}

func (s *Server) handleTagihanDaftar(w http.ResponseWriter, r *http.Request) {
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
	invoices, err := s.Store.ListInvoices(r.Context(), id, "sort")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Invoices []store.Invoice
	}{
		baseView: baseView{Title: "Daftar Tagihan", Active: "tagihan", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Invoices: invoices,
	}
	s.render(w, r, "tagihan_daftar.html", data)
}

// handleAPIHitungPajak menghasilkan preview pajak untuk form tagihan.
func (s *Server) handleAPIHitungPajak(w http.ResponseWriter, r *http.Request) {
	bruto, _ := money.Parse(r.URL.Query().Get("bruto"))
	jm := atoi0(r.URL.Query().Get("jenis_menu"))
	fp := atoi0(r.URL.Query().Get("flag_ppn"))
	fh := atoi0(r.URL.Query().Get("flag_pph"))
	if jm == 0 {
		jm = 1
	}
	if fp == 0 {
		fp = 2
	}
	if fh == 0 {
		fh = 3
	}
	var kategori *int
	if k := r.URL.Query().Get("kategori_narasumber"); k != "" {
		ki := atoi0(k)
		kategori = &ki
	}
	res := tax.Hitung(bruto, jm, fp, fh, kategori)
	id, err := pathID(r, "id")
	var admin int64
	if err == nil {
		if b, err := s.Store.GetBantuan(r.Context(), id); err == nil {
			admin = store.AdminFee(b.Bank, r.URL.Query().Get("bank"))
		}
	}
	var jenisPPH *string
	if res.JenisPPH != nil {
		jj := *res.JenisPPH
		jenisPPH = &jj
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"dpp": res.DPP, "dpp_nilai_lain": res.DPPNilaiLain, "ppn": res.PPN,
		"pph": res.PPH, "netto": res.Netto, "jenis_pph": jenisPPH,
		"biaya_admin": admin, "netto_final": res.Netto - admin,
	})
}

func atoi0(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
