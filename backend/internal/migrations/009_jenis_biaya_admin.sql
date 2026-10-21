-- 009_jenis_biaya_admin.sql
-- Jenis biaya admin transfer: 'kliring' (Rp 2.900) atau 'rtgs' (Rp 30.000).
-- Baris lama memakai 'kliring' agar nilai biaya admin tidak berubah.
-- Pembayaran tunai (bank penerima kosong) tidak dikenakan biaya admin.
ALTER TABLE trx_invoice ADD COLUMN IF NOT EXISTS jenis_biaya_admin TEXT NOT NULL DEFAULT 'kliring';
