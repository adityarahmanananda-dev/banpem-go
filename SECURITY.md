# Security Policy

## Supported deployment

Banpem-GO is intended for a single school per deployment, behind HTTPS and restricted network access (localhost, LAN, or VPN). Do not expose the app directly to the public internet without a reverse proxy/TLS.

## Reporting a vulnerability

Please report security issues privately to the maintainer (do not open a public issue with exploit details). Include:

- affected version/commit,
- reproduction steps,
- impact,
- suggested fix (if any).

## Deployment checklist

- Set `ADMIN_PASSWORD` (minimum 10 characters) only for first-run provisioning; change it immediately after login.
- Set `COOKIE_SECURE=true` when served over HTTPS.
- Use PostgreSQL with `sslmode=require` (or stronger) for remote databases.
- Restrict `backups/` to the service account (`0700` directory, `0600` files) and keep offsite encrypted copies.
- Do not commit `.env`, database dumps, or `backups/`.
- Keep dependencies updated (`go get -u`, `go mod tidy`, image rebuilds).
- Review audit logs (`/admin/audit`) and user roles regularly.
- Use a reverse proxy that adds HSTS and terminates TLS; the app already sends CSP, `X-Frame-Options`, `X-Content-Type-Options`, and `Referrer-Policy`.

## Data handling

The database contains personal data: names, NIP, NPWP, bank accounts, and transaction details. Treat exports and backups as confidential. Grant `viewer` role only to users who need read-only access; `admin` role can export/restore the database and manage users.
