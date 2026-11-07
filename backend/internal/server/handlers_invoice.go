package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

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
	Inv                *store.Invoice
	Realisasi          []realisasiView
	Kegiatans          []store.KegiatanTree
	Parties            []store.PihakKetiga
	SelectedPartyID    int64
	SelectedRekeningID int64
	JenisMenu          int
	FlagPpn            int
	FlagPph            int
	Kategori           *int
	IsEdit             bool
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
	parties, err := s.Store.ListPihakKetiga(r.Context())
	if err != nil {
		return nil, nil, err
	}
	v := &invoiceFormView{
		baseView: baseView{Title: "Input Tagihan", Active: "tagihan",
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Kegiatans: kegs,
		Parties:   parties,
		IsEdit:    edit,
	}
	if edit && iid > 0 {
		if inv, err := s.Store.GetInvoice(r.Context(), iid); err == nil {
			if inv.PihakKetigaID != nil {
				v.SelectedPartyID = *inv.PihakKetigaID
			}
			if inv.PihakKetigaRekeningID != nil {
				v.SelectedRekeningID = *inv.PihakKetigaRekeningID
			}
		}
	}
	if edit && iid > 0 {
		inv, err := s.Store.GetInvoice(r.Context(), iid)
		if err != nil {
			return nil, nil, err
		}
		if inv.BantuanID != id {
			return nil, nil, store.ErrNotFound
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
	from, to := dateRangeFromQuery(r)
	invoices = filterInvoicesByDate(invoices, from, to)
	page, perPage := pageParams(r)
	total := len(invoices)
	pagination := buildPagination(r, page, perPage, total)
	invoices = paginateSlice(invoices, pagination.Page, perPage)
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Invoices   []store.Invoice
		Pagination Pagination
		Dari       string
		Sampai     string
	}{
		baseView: baseView{Title: "Daftar Tagihan", Active: "tagihan", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Invoices: invoices, Pagination: pagination,
		Dari: r.URL.Query().Get("dari"), Sampai: r.URL.Query().Get("sampai"),
	}
	s.render(w, r, "tagihan_list.html", data)
}

func filterInvoicesByDate(invoices []store.Invoice, from, to *time.Time) []store.Invoice {
	if from == nil && to == nil {
		return invoices
	}
	out := make([]store.Invoice, 0, len(invoices))
	for _, inv := range invoices {
		if dateInRange(inv.Tanggal, from, to) {
			out = append(out, inv)
		}
	}
	return out
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
	bruto, err := money.ParsePositive(formStr(r, "bruto_utama"))
	if err != nil {
		return store.InvoiceInput{}, err
	}
	tgl, err := store.ParseDateStrict(formStr(r, "tanggal"))
	if err != nil {
		return store.InvoiceInput{}, err
	}
	in := store.InvoiceInput{
		Tanggal:              tgl,
		NomorBukti:           formStr(r, "nomor_bukti"),
		Uraian:               formStr(r, "uraian"),
		Bruto:                bruto,
		JenisMenu:            int(formInt(r, "jenis_menu")),
		FlagPpn:              int(formInt(r, "flag_ppn")),
		FlagPph:              int(formInt(r, "flag_pph")),
		TarifPPH:             formFloat(r, "tarif_pph"),
		NamaRekening:         formStr(r, "nama_rekening"),
		NomorRekening:        formStr(r, "nomor_rekening"),
		Bank:                 formStr(r, "bank"),
		NPWP:                 formStr(r, "npwp"),
		NomorBupotPPN:        formStr(r, "nomor_bupot_ppn"),
		NomorBupotPPH:        invoiceBupotPPH(r),
		BiayaAdminDibebankan: formStr(r, "biaya_admin_dibebankan"),
		JenisBiayaAdmin:      formStr(r, "jenis_biaya_admin"),
	}
	if v := formInt(r, "pihak_ketiga_id"); v > 0 {
		in.PihakKetigaID = &v
	}
	if v := formInt(r, "pihak_ketiga_rekening_id"); v > 0 {
		in.PihakKetigaRekeningID = &v
	}
	if in.JenisMenu < 1 || in.JenisMenu > 5 {
		in.JenisMenu = 1
	}
	if in.FlagPpn < 1 || in.FlagPpn > 2 {
		in.FlagPpn = 2
	}
	if in.FlagPph < 1 || in.FlagPph > 5 {
		in.FlagPph = 3
	}
	if in.BiayaAdminDibebankan != "ya" {
		in.BiayaAdminDibebankan = "tidak"
	}
	in.JenisBiayaAdmin = store.NormalizeJenisBiayaAdmin(in.JenisBiayaAdmin)
	// Koreksi nilai pajak (bila diisi, menggantikan hasil perhitungan).
	if v := formStr(r, "ppn"); v != "" {
		ppn, err := money.ParseNonNegative(v)
		if err != nil {
			return store.InvoiceInput{}, err
		}
		in.PPNOverride = &ppn
	}
	if v := formStr(r, "pph"); v != "" {
		pph, err := money.ParseNonNegative(v)
		if err != nil {
			return store.InvoiceInput{}, err
		}
		in.PPHOverride = &pph
	}
	if in.PPNOverride != nil || in.PPHOverride != nil {
		var ppn, pph int64
		if in.PPNOverride != nil {
			ppn = *in.PPNOverride
		}
		if in.PPHOverride != nil {
			pph = *in.PPHOverride
		}
		if ppn > in.Bruto || pph > in.Bruto || ppn+pph > in.Bruto {
			return store.InvoiceInput{}, money.ErrInvalidAmount
		}
	}
	if v := formStr(r, "kategori_narasumber"); v != "" {
		ki := int(formInt(r, "kategori_narasumber"))
		if ki < 1 || ki > 4 {
			return store.InvoiceInput{}, store.ErrInvalidInput
		}
		in.KategoriNarasumber = &ki
	}
	komponenIDs := r.Form["komponen_id"]
	brutos := r.Form["bruto"]
	var realisasiTotal int64
	for i := range komponenIDs {
		if i >= len(brutos) {
			break
		}
		kid, _ := strconv.ParseInt(komponenIDs[i], 10, 64)
		bv, perr := money.ParsePositive(brutos[i])
		if kid <= 0 {
			continue
		}
		if perr != nil {
			return store.InvoiceInput{}, perr
		}
		in.Realisasi = append(in.Realisasi, store.RealisasiInput{KomponenID: kid, Bruto: bv})
		realisasiTotal += bv
	}
	if realisasiTotal > in.Bruto {
		return store.InvoiceInput{}, store.ErrRealisasiMismatch
	}
	return in, nil
}

// invoiceBupotPPH mengambil nomor bupot PPh sesuai jenis pengeluaran aktif:
// menu 1 (Barang/Jasa) -> nomor_bupot_pph, menu 2 (Honor Narasumber) ->
// nomor_bupot_pph_ns, menu 3 (Honor Peserta) -> nomor_bupot_pph_pst. Nama field
// dipisahkan agar nilai dari menu tersembunyi tidak menimpa nilai yang terisi.
func invoiceBupotPPH(r *http.Request) string {
	switch int(formInt(r, "jenis_menu")) {
	case 2:
		return formStr(r, "nomor_bupot_pph_ns")
	case 3:
		return formStr(r, "nomor_bupot_pph_pst")
	default:
		return formStr(r, "nomor_bupot_pph")
	}
}

func (s *Server) handleTagihanPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	in, err := parseInvoiceForm(r)
	if err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/tagihan/tambah")
		return
	}
	if in.Uraian == "" {
		s.setFlash(w, "danger", "Uraian wajib diisi.")
		s.redirect(w, r, "/bantuan/"+i64s(id)+"/tagihan/tambah")
		return
	}
	if (in.FlagPph == 4 || in.FlagPph == 5) && in.TarifPPH <= 0 {
		s.setFlash(w, "danger", "Tarif PPh wajib diisi lebih dari 0.")
		s.redirect(w, r, "/bantuan/"+i64s(id)+"/tagihan/tambah")
		return
	}
	iid, err := s.Store.CreateInvoice(r.Context(), id, in)
	if err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/tagihan/tambah")
		return
	}
	s.audit(r, &id, "create", "invoice", &iid, map[string]any{"uraian": in.Uraian, "bruto": in.Bruto})
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
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/tagihan/edit/"+i64s(iid))
		return
	}
	if (in.FlagPph == 4 || in.FlagPph == 5) && in.TarifPPH <= 0 {
		s.setFlash(w, "danger", "Tarif PPh wajib diisi lebih dari 0.")
		s.redirect(w, r, "/bantuan/"+i64s(id)+"/tagihan/edit/"+i64s(iid))
		return
	}
	if in.Uraian == "" {
		s.setFlash(w, "danger", "Uraian wajib diisi.")
		s.redirect(w, r, "/bantuan/"+i64s(id)+"/tagihan/edit/"+i64s(iid))
		return
	}
	if err := s.Store.UpdateInvoice(r.Context(), id, iid, in); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/tagihan/edit/"+i64s(iid))
		return
	}
	s.audit(r, &id, "update", "invoice", &iid, map[string]any{"uraian": in.Uraian, "bruto": in.Bruto})
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
	if err := s.Store.DeleteInvoice(r.Context(), id, iid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/tagihan")
		return
	}
	s.audit(r, &id, "delete", "invoice", &iid, nil)
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
	if err != nil || inv.BantuanID != id {
		s.fail(w, r, store.ErrNotFound, "/")
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
	from, to := dateRangeFromQuery(r)
	invoices = filterInvoicesByDate(invoices, from, to)
	page, perPage := pageParams(r)
	pagination := buildPagination(r, page, perPage, len(invoices))
	invoices = paginateSlice(invoices, pagination.Page, perPage)
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Invoices   []store.Invoice
		Pagination Pagination
		Dari       string
		Sampai     string
	}{
		baseView: baseView{Title: "Daftar Tagihan", Active: "tagihan", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Invoices: invoices, Pagination: pagination,
		Dari: r.URL.Query().Get("dari"), Sampai: r.URL.Query().Get("sampai"),
	}
	s.render(w, r, "tagihan_daftar.html", data)
}

// handleAPIHitungPajak menghasilkan preview pajak untuk form tagihan.
func (s *Server) handleAPIHitungPajak(w http.ResponseWriter, r *http.Request) {
	bruto, err := money.ParsePositive(r.URL.Query().Get("bruto"))
	if err != nil {
		http.Error(w, "nominal bruto tidak valid", http.StatusBadRequest)
		return
	}
	jm := atoi0(r.URL.Query().Get("jenis_menu"))
	fp := atoi0(r.URL.Query().Get("flag_ppn"))
	fh := atoi0(r.URL.Query().Get("flag_pph"))
	tp := parseFloatStr(r.URL.Query().Get("tarif_pph"))
	if jm < 1 || jm > 5 {
		jm = 1
	}
	if fp < 1 || fp > 2 {
		fp = 2
	}
	if fh < 1 || fh > 5 {
		fh = 3
	}
	var kategori *int
	if k := r.URL.Query().Get("kategori_narasumber"); k != "" {
		ki := atoi0(k)
		if ki < 1 || ki > 4 {
			http.Error(w, "kategori narasumber tidak valid", http.StatusBadRequest)
			return
		}
		kategori = &ki
	}
	res := tax.Hitung(bruto, jm, fp, fh, kategori, tp)
	id, err := pathID(r, "id")
	var admin int64
	if err == nil {
		if b, err := s.Store.GetBantuan(r.Context(), id); err == nil {
			admin = store.AdminFee(b.Bank, r.URL.Query().Get("bank"), r.URL.Query().Get("jenis_biaya_admin"))
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
