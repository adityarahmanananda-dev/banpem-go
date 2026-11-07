# Banpem-GO — Government Grant Cash Book (e-BKU)

[![CI](https://github.com/adityarahmanananda-dev/banpem-go/actions/workflows/ci.yml/badge.svg)](https://github.com/adityarahmanananda-dev/banpem-go/actions/workflows/ci.yml)

A Go web app for administering **government grant cash books (e-BKU)** at schools (in use at SMK Negeri 26 Jakarta). It manages every grant account: opening balance, phased disbursement, invoices, and official financial reports — using **integer-based money arithmetic (cents) + banker's rounding (ROUND HALF-EVEN)** at every step (the specification forbids floats).

## Screenshot

![UI demo (dummy data)](docs/screenshot.png)

> Screenshot is a UI mockup with dummy data — not real data.

## Features

- **Grant management** — CRUD per grant, automatic phase-1 disbursement (70% of the amount when > Rp100,000,000), opening balance.
- **Tax engine** (`internal/tax`) — VAT 12%, PPh 22 (1.5%), PPh 23 (2%), PPh 21 (5/15/2.5% by speaker class), 5 invoice menu types (Goods/Services, Speaker Honorarium, Participant Honorarium, Transport, Per-diem). Ships with 5 mandatory test vectors in `tax_test.go`.
- **Two ledgers** — *General Cash Book (BKU)* and *Bank Cash Book* with automatic entries per invoice (payment, VAT/income-tax withholding, Rp2,900 admin fee for interbank transfers), running balance, and a "Real vs 100% Plan" offset view.
- **Tax deposit workflow** — multi-invoice tax deposits with NTB/NTPN references, plus **bank interest** entries.
- **Hierarchical master data** — Activity → Sub-activity → Activity → Component (with per-component budget ceiling), cascading dropdowns for invoice realization rows and proportional tax allocation.
- **Third-party master** — vendor/payee with NPWP and multiple bank accounts; selecting a third party + account in the invoice form autofills account name, bank, account number, and NPWP. Legacy invoice payment details are migrated into this master.
- **Reports & export** — Excel (`.xlsx`), PDF, and Word (`.docx`) for **BKU, Bank Cash Book, Tax Recap, Spending Recap, Invoice List, RAB, Realization Recap, Fund Usage Recap** — all with Indonesian signature blocks, A4 layout, repeating table headers, and specific Excel styling (header `#4472C4`, totals `#D9E2F3`).
- **Row reordering** of ledger/spending via drag-and-drop (SortableJS).
- **Pagination, global search, and date-range filters** on invoice lists, BKU/Bank, and spending recap.
- **Dashboard chart & filter** — 12-month spending bar chart plus grant disbursement progress; summary cards and chart can be filtered by grant (`?bantuan_id=`).
- **Observability** — Prometheus-compatible `/metrics`, request ID/tracing correlation, error log (`/admin/errors`), and optional webhook alerts.
- **Authentication & roles** — multi-user login (admin/bendahara/viewer), bcrypt password hashing, DB-backed sessions, CSRF protection, login rate limiting.
- **Audit trail** — `audit_log` records user, action, entity, and details for key mutations (admin page `/admin/audit`).
- **Database backup** — SQL-dump snapshot at every startup (keeps the last 10), plus admin-only export/import via UI with strict statement validation and confirmation.
- **Security hardening** — local vendored Bootstrap/Icons/Sortable (no CDN), CSP/security headers, request body limits, DB-aware health check, graceful shutdown.

## Tech stack

- **Go 1.24** — stdlib `net/http` + `html/template` with `embed.FS` for templates/static.
- **PostgreSQL** — `jackc/pgx/v5` (pgxpool).
- **Export** — `go-pdf/fpdf` (PDF), `xuri/excelize/v2` (XLSX), and a custom `.docx` writer (zip/XML); Arimo font embedded.
- **Frontend** — Bootstrap 5.3 + Bootstrap Icons + SortableJS (vendored locally), `app.js`/`app.css` embedded into the binary.
- **Docker** — multi-stage Dockerfile (`golang:1.24-alpine` → `alpine:3.20`, non-root, static binary) + `docker-compose.yml`.

## Installation & running

### Docker

```bash
cp .env.example .env          # isi DATABASE_URL dan ADMIN_PASSWORD
docker compose up --build     # app on :8080
```

Saat pertama kali dijalankan, user admin dibuat dari `ADMIN_USERNAME`/`ADMIN_PASSWORD`. Segera ganti password setelah login.

### Local

```bash
cd backend
DATABASE_URL=... ADMIN_PASSWORD=... go run ./cmd/server
```

SQL migrations run automatically at startup and are recorded in `schema_migrations`.

### Tests

```bash
cd backend
go test ./...
# integration test (butuh PostgreSQL khusus test):
TEST_DATABASE_URL='postgres://ebku:test@127.0.0.1:5432/ebku_test?sslmode=disable' \
  go test ./internal/store -run Integration -v
```

## Configuration (env)

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | — | PostgreSQL DSN (required; gunakan `sslmode=require` untuk koneksi publik) |
| `ADMIN_USERNAME` | `admin` | Username admin pertama (hanya saat tabel `users` kosong) |
| `ADMIN_PASSWORD` | — | Password admin pertama, minimal 10 karakter (wajib saat instalasi awal) |
| `BACKUP_DIR` | `backups` (Docker: `/backups`) | Backup snapshot directory |
| `ADDR` | `:8080` | Server listen address |
| `COOKIE_SECURE` | `false` | Set `true` jika diakses lewat HTTPS |
| `REPORT_CITY` | `Jakarta` | Kota pada blok tanda tangan laporan |
| `SESSION_TTL_HOURS` | `12` | Umur session login |
| `BACKUP_ON_STARTUP` | `true` | Buat backup `.sql` setiap startup |
| `RESYNC_LEDGER_ON_STARTUP` | `false` | Tulis ulang seluruh ledger saat startup (hanya untuk perbaikan data lama) |
| `METRICS_TOKEN` | — | Bearer token untuk `GET /metrics`; jika kosong hanya admin yang bisa akses |
| `ALERT_WEBHOOK_URL` | — | Webhook JSON untuk alert error/panic/database down |

Copy `.env.example` to `.env` for local config. **Never commit `.env`** (git-ignored).

## Project structure

```
backend/
├── cmd/server/main.go        # entry point: env, DB, migrate, backup, serve
├── Dockerfile, go.mod, go.sum
├── internal/
│   ├── server/               # HTTP layer + templates (login, users, audit, reports)
│   ├── store/                # data layer (models, ledger, dump, users, audit, app_*)
│   ├── tax/                  # tax engine (+tax_test.go)
│   ├── money/                # cent arithmetic, HALF-EVEN (+money_test.go)
│   ├── export/               # report model + excel.go, pdf.go, word.go, fonts
│   └── migrations/           # 001_init.sql … 013_pihak_ketiga.sql
└── migrations/001_init.sql   # legacy schema copy
```

## Notes

- The full behavior specification is in `PROMPT_eBKU_STACK_AGNOSTIC.txt` (828 lines, Indonesian).
- `docker-compose.yml` only defines the `app` service and depends on an external Postgres (e.g. Supabase pooler); there is no built-in `db` service.
- Export/import database hanya untuk role `admin`. File backup berekstensi `.sql` (format `.db` lama tetap diterima). Import mewajibkan konfirmasi `RESTORE` dan menolak statement DDL/administratif.
- `/metrics` bisa diakses admin atau dengan header `Authorization: Bearer <METRICS_TOKEN>`. Metrik mencakup request count/latency histogram, `banpem_db_up`, dan login failures.
- Error tak terduga dicatat ke tabel `error_log` dan dapat dilihat di `/admin/errors`; jika `ALERT_WEBHOOK_URL` diset, panic/error/database-down dikirim sebagai JSON webhook (dibatasi 1 alert per jenis per 5 menit).
- Aplikasi menyimpan data pribadi (nama, NIP, NPWP, rekening bank). Aktifkan HTTPS (`COOKIE_SECURE=true`), batasi akses jaringan, dan amankan direktori `backups/`.