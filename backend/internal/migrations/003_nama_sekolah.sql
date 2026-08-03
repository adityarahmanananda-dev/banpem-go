-- 003_nama_sekolah.sql : pisahkan nama bantuan dan nama sekolah
-- Idempotent: dipakai ulang untuk import database dan startup.

ALTER TABLE bantuan ADD COLUMN IF NOT EXISTS nama_sekolah TEXT NOT NULL DEFAULT '';
