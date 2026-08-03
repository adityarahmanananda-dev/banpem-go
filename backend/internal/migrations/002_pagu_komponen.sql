-- 002_pagu_komponen.sql : kolom pagu anggaran per komponen
-- Idempotent: dipakai ulang untuk import database dan startup.

ALTER TABLE komponen ADD COLUMN IF NOT EXISTS pagu BIGINT NOT NULL DEFAULT 0;
