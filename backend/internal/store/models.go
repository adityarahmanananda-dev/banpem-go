package store

import "time"

type Bantuan struct {
	ID               int64
	Nama             string
	NamaSekolah      string
	NamaRekening     string
	NomorRekening    string
	Bank             string
	NPWP             string
	KepalaSekolah    string
	NIPKepalaSekolah string
	Bendahara        string
	NIPBendahara     string
	Nominal          *int64
	CreatedAt        time.Time
}

type SaldoAwal struct {
	ID        int64
	BantuanID int64
	Saldo     int64
	Tanggal   *time.Time
}

type Invoice struct {
	ID                   int64
	BantuanID            int64
	Tanggal              time.Time
	NomorBukti           string
	Uraian               string
	Bruto                int64
	JenisMenu            int
	FlagPpn              int
	FlagPph              int
	KategoriNarasumber   *int
	DPP                  int64
	DPPNilaiLain         int64
	NilaiPPN             int64
	NilaiPPH             int64
	JenisPPH             *string
	NilaiNetto           int64
	NamaRekening         string
	NomorRekening        string
	Bank                 string
	NPWP                 string
	NomorBupotPPN        string
	NomorBupotPPH        string
	BiayaAdminDibebankan string
	SortOrder            int
}

type Ledger struct {
	ID             int64
	BantuanID      int64
	Nomor          int
	Tanggal        *time.Time
	NomorBukti     string
	Uraian         string
	Debit          int64
	Kredit         int64
	Saldo          int64
	InvoiceID      *int64
	JenisTransaksi string
	NomorBupot     *string
}

type JasaGiro struct {
	ID        int64
	BantuanID int64
	Tanggal   *time.Time
	Nominal   int64
	Uraian    string
}

type Pencairan struct {
	ID         int64
	BantuanID  int64
	Tahap      int
	Tanggal    *time.Time
	Nominal    int64
	Keterangan string
}

type Kegiatan struct {
	ID        int64
	BantuanID int64
	Nama      string
}

type SubKegiatan struct {
	ID         int64
	KegiatanID int64
	Nama       string
}

type Komponen struct {
	ID            int64
	SubKegiatanID int64
	Nama          string
	Pagu          int64
}

type RekapPajak struct {
	ID             int64
	BantuanID      int64
	TanggalPosting *time.Time
	NTBN           string
	NTPN           string
	InvoiceID      *int64
	JasaGiro       int64
	SetorLedgerID  *int64
	JenisPajak     string
}

type Realisasi struct {
	ID         int64
	InvoiceID  int64
	KomponenID int64
	Bruto      int64
	NilaiPPN   int64
	NilaiPPH   int64
	NilaiNetto int64
}
