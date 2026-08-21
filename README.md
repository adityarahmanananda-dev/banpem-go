# Banpem-GO — e-BKU Bantuan Pemerintah Sekolah

Web app Go untuk administrasi **Buku Kas Umum (e-BKU)** bantuan pemerintah di sekolah (dipakai di SMK Negeri 26 Jakarta). Mengelola setiap akun bantuan: saldo awal, pencairan bertahap, tagihan, dan laporan keuangan resmi — dengan **aritmetika uang berbasis integer (sen) + pembulatan bankir (ROUND HALF-EVEN)** di setiap langkah (spesifikasi melarang float).

## Fitur

- **Manajemen Bantuan** — CRUD per bantuan, auto-pencairan tahap 1 (70% nominal saat > Rp100.000.000), saldo awal.
- **Mesin Pajak** (`internal/tax`) — PPN 12%, PPh 22 (1,5%), PPh 23 (2%), PPh 21 (5/15/2,5% per golongan narasumber), 5 jenis menu tagihan (Barang/Jasa, Honor Narasumber, Honor Peserta, Transport, Uang Harian). Disertai 5 vektor uji wajib di `tax_test.go`.
- **Dua buku besar** — *Buku Kas Umum* (BKU) dan *Buku Kas Bank* dengan entri otomatis per tagihan (pembayaran, pungut PPN/PPh, biaya admin Rp2.900 untuk transfer antar-bank), perhitungan saldo berjalan, dan tampilan offset "Real vs Rencana 100%".
- **Workflow setor pajak** — setor pajak multi-tagihan dengan referensi NTB/NTPN, plus entri **Jasa Giro**.
- **Master data berjenjang** — Kegiatan → Sub Kegiatan → Aktivitas → Komponen (dengan pagu per komponen), dropdown berjenjang untuk baris realisasi tagihan dan alokasi pajak proporsional.
- **Laporan & export** — Excel (`.xlsx`), PDF, dan Word (`.docx`) untuk **BKU, Buku Kas Bank, Rekap Pajak, Rekap Belanja, Daftar Tagihan, RAB, Rekap Realisasi, Rekapitulasi Penggunaan Dana** — semua dengan blok tanda tangan Indonesia, layout A4, header tabel berulang, styling Excel spesifik (header `#4472C4`, total `#D9E2F3`).
- **Urut ulang baris** ledger/belanja via drag-and-drop (SortableJS).
- **Backup database** — snapshot SQL-dump di setiap startup (simpan 10 terakhir), export/import via UI dengan validasi struktur.

## Tech stack

- **Go 1.24** — stdlib `net/http` + `html/template` dengan `embed.FS` untuk template/static.
- **PostgreSQL** — `jackc/pgx/v5` (pgxpool).
- **Export** — `go-pdf/fpdf` (PDF), `xuri/excelize/v2` (XLSX), dan writer `.docx` buatan sendiri (zip/XML); font Arimo di-embed.
- **Frontend** — Bootstrap 5.3 + Bootstrap Icons + SortableJS (CDN), `app.js`/`app.css` di-embed ke binary.
- **Docker** — multi-stage Dockerfile (`golang:1.24-alpine` → `alpine:3.20`, non-root, binary statis) + `docker-compose.yml`.

## Instalasi & menjalankan

### Docker

```bash
docker compose up --build     # app di :8080
```

### Lokal

```bash
cd backend
go run ./cmd/server           # butuh DATABASE_URL
```

Migrasi SQL berjalan otomatis saat startup.

## Konfigurasi (env)

| Variabel | Default | Keterangan |
|---|---|---|
| `DATABASE_URL` | — | DSN PostgreSQL (wajib) |
| `BACKUP_DIR` | `/backups` | Direktori snapshot backup |
| `ADDR` | `:8080` | Alamat listen server |

Salin `.env.example` ke `.env` untuk konfigurasi lokal. **Jangan commit `.env`** (sudah di-ignore git).

## Struktur project

```
backend/
├── cmd/server/main.go        # entry point: env, DB, migrate, backup, serve
├── Dockerfile, go.mod, go.sum
├── internal/
│   ├── server/               # HTTP layer + 24 template HTML
│   ├── store/                # data layer (models, ledger, dump, app_*)
│   ├── tax/                  # mesin pajak (+tax_test.go)
│   ├── money/                # aritmetika sen, HALF-EVEN (+money_test.go)
│   ├── export/               # report model + excel.go, pdf.go, word.go, fonts
│   └── migrations/           # 001_init.sql … 004_aktivitas.sql
└── migrations/001_init.sql   # salinan schema lama
```

## Catatan

- Spesifikasi perilaku lengkap ada di `PROMPT_eBKU_STACK_AGNOSTIC.txt` (828 baris, Indonesia).
- `docker-compose.yml` hanya mendefinisikan service `app` dan bergantung pada Postgres eksternal (mis. Supabase pooler); tidak ada service `db` bawaan.