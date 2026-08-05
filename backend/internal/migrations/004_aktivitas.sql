-- 004_aktivitas.sql : level Aktivitas di antara Sub Kegiatan dan Komponen
-- Idempotent: dipakai ulang untuk import database dan startup.

CREATE TABLE IF NOT EXISTS aktivitas (
  id BIGSERIAL PRIMARY KEY,
  sub_kegiatan_id BIGINT NOT NULL REFERENCES sub_kegiatan(id) ON DELETE CASCADE,
  nama TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE komponen ADD COLUMN IF NOT EXISTS aktivitas_id BIGINT REFERENCES aktivitas(id) ON DELETE SET NULL;

-- Aktivitas default per sub kegiatan yang belum punya (sekali saja).
INSERT INTO aktivitas (sub_kegiatan_id, nama)
  SELECT sk.id, sk.nama FROM sub_kegiatan sk
  WHERE NOT EXISTS (SELECT 1 FROM aktivitas a WHERE a.sub_kegiatan_id = sk.id);

-- Hubungkan komponen lama ke aktivitas default sub kegiatan-nya.
UPDATE komponen ko SET aktivitas_id = (
  SELECT a.id FROM aktivitas a WHERE a.sub_kegiatan_id = ko.sub_kegiatan_id ORDER BY a.id LIMIT 1
) WHERE ko.aktivitas_id IS NULL;
