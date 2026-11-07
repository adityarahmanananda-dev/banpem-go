# Changelog

## 2026-11-07 — Third-party master & invoice form layout

- Added `pihak_ketiga` and `pihak_ketiga_rekening` master data (one party can have multiple bank accounts).
- Added migration `013_pihak_ketiga.sql` that backfills existing invoice vendor/account/NPWP data into the new master and links old invoices.
- Added `/pihak-ketiga` management pages (party CRUD, multiple accounts, inline account edit/delete).
- Invoice form now has selectable third party + account; selecting them autofills account name, bank, account number, and NPWP.
- Server overrides invoice payment snapshot fields from master data when party/account IDs are selected, preventing tampered account data.
- Reorganized invoice input into clear sections: invoice info, third party/payee, tax/fees, realization, and actions.
- Backup/restore now includes third-party master tables and invoice party references.

## 2026-11-07 — Pagination, search, dashboard charts, observability

### Features
- Added pagination (10/25/50/100 per page) to invoice list, BKU, Bank Cash Book, and spending recap.
- Added global search across grants, invoices, and both ledgers (`/search`).
- Added date-range filters (`dari`/`sampai`) to invoice list, BKU/Bank, tax recap, spending recap, and spending export.
- Added a 12-month dashboard spending bar chart and grant disbursement progress bars.
- Added dashboard filter by grant; summary cards, grant cards, and monthly chart follow the selected `bantuan_id`.

### Observability
- Added Prometheus-compatible `/metrics` (request counts, latency histogram, DB up, login failures).
- Added `X-Request-ID` propagation and request-ID logging for tracing correlation.
- Added `error_log` table, `/admin/errors`, and unexpected-error recording.
- Added optional `ALERT_WEBHOOK_URL` alerts for errors, panics, and database-down health checks (5-minute cooldown per alert type).
- Added `METRICS_TOKEN` support for Prometheus scraping.

## 2026-11-07 — Security & integrity hardening

### Security
- Added multi-user authentication with bcrypt password hashing, DB-backed sessions, and roles (`admin`, `bendahara`, `viewer`).
- Added CSRF protection (double-submit cookie + `X-CSRF-Token` for JSON requests).
- Added security headers (CSP, `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`, HSTS behind TLS).
- Restricted database import/export and user/audit pages to `admin`.
- Hardened database restore: size limit, version check, table allow-list, dangerous statement rejection, and `RESTORE` confirmation.
- Added login rate limiting and request body limits.
- Vendored Bootstrap/Icons/Sortable locally (removed CDN dependency).
- Scoped object access by `bantuan_id` across invoices, ledgers, payments, disbursements, and master data.
- Restricted manual ledger endpoints to `jenis_transaksi='manual'`; bank deletion to `biaya_admin`; tax deletion/edit to `setor_pajak`.

### Data integrity
- Fixed `PajakBelumSetorAsOf` argument bug (BAP tax outstanding was always 0).
- Blocked invoice edit/delete when tax has already been deposited.
- Blocked duplicate tax deposits and duplicate invoice numbers.
- Validated invoice realization totals, component budget ceilings, and component ownership.
- Synced opening balance when the first disbursement is edited/deleted.
- Blocked disbursement totals exceeding the grant nominal.
- Blocked deleting master data that still has realization rows.
- Added advisory locks per grant for transaction mutations.
- Added migration tracking (`schema_migrations`) and transactional migrations.
- Added missing indexes.

### Validation & UX
- Strict money/date parsing; negative/zero amounts and invalid dates are rejected.
- PPN/PPh overrides are bounded so netto cannot go negative.
- Added audit log page, user management page, login/logout UI.
- Hid write actions for `viewer` in key views.
- Added CSRF-aware form injection and safer master-data option rendering.
- Added DB-aware `/healthz`, request logging, graceful shutdown, and server timeouts.
- Backups now use `.sql`, `0600` permissions, and startup backup is configurable.

### Tests & CI
- Added tests for strict money parsing, dump validation, template parsing, CSRF injection, and safe redirects.
- CI now runs `gofmt` check, build, vet, and `go test -race` for all packages.
