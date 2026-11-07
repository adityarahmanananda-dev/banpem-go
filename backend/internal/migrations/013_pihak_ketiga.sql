-- 013_pihak_ketiga.sql : master data pihak ketiga + rekening, dan migrasi
-- data vendor/rekening dari tagihan lama ke master data.
-- Idempotent: migration runner hanya menjalankannya sekali, tetapi semua
-- statement tetap aman bila dijalankan ulang secara manual.

CREATE TABLE IF NOT EXISTS pihak_ketiga (
  id BIGSERIAL PRIMARY KEY,
  nama TEXT NOT NULL,
  npwp TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_pihak_ketiga_nama
  ON pihak_ketiga (lower(btrim(nama)));

CREATE TABLE IF NOT EXISTS pihak_ketiga_rekening (
  id BIGSERIAL PRIMARY KEY,
  pihak_ketiga_id BIGINT NOT NULL REFERENCES pihak_ketiga(id) ON DELETE CASCADE,
  nama_rekening TEXT NOT NULL DEFAULT '',
  nomor_rekening TEXT NOT NULL DEFAULT '',
  bank TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_pihak_ketiga_rekening
  ON pihak_ketiga_rekening (
    pihak_ketiga_id,
    lower(btrim(nama_rekening)),
    lower(btrim(nomor_rekening)),
    lower(btrim(bank))
  );

ALTER TABLE trx_invoice
  ADD COLUMN IF NOT EXISTS pihak_ketiga_id BIGINT REFERENCES pihak_ketiga(id) ON DELETE SET NULL;
ALTER TABLE trx_invoice
  ADD COLUMN IF NOT EXISTS pihak_ketiga_rekening_id BIGINT REFERENCES pihak_ketiga_rekening(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_pihak_ketiga ON trx_invoice(pihak_ketiga_id);
CREATE INDEX IF NOT EXISTS idx_invoice_pihak_rekening ON trx_invoice(pihak_ketiga_rekening_id);

-- 1. Buat satu pihak ketiga per nama rekening (case-insensitive).
INSERT INTO pihak_ketiga (nama, npwp)
SELECT DISTINCT ON (kunci) nama, npwp
FROM (
  SELECT
    lower(btrim(COALESCE(NULLIF(nama_rekening, ''), 'Pihak Ketiga'))) AS kunci,
    COALESCE(NULLIF(btrim(nama_rekening), ''), 'Pihak Ketiga') AS nama,
    COALESCE(btrim(npwp), '') AS npwp,
    id
  FROM trx_invoice
  WHERE btrim(nama_rekening) <> ''
     OR btrim(nomor_rekening) <> ''
     OR btrim(bank) <> ''
     OR btrim(npwp) <> ''
) x
ORDER BY kunci, (npwp <> '') DESC, id
ON CONFLICT DO NOTHING;

-- 2. Buat rekening unik untuk setiap kombinasi nama/nomor/bank.
INSERT INTO pihak_ketiga_rekening (pihak_ketiga_id, nama_rekening, nomor_rekening, bank)
SELECT p.id,
       COALESCE(NULLIF(btrim(i.nama_rekening), ''), p.nama),
       btrim(i.nomor_rekening),
       btrim(i.bank)
FROM trx_invoice i
JOIN pihak_ketiga p
  ON lower(btrim(p.nama)) = lower(btrim(COALESCE(NULLIF(i.nama_rekening, ''), 'Pihak Ketiga')))
WHERE btrim(i.nama_rekening) <> ''
   OR btrim(i.nomor_rekening) <> ''
   OR btrim(i.bank) <> ''
   OR btrim(i.npwp) <> ''
ON CONFLICT DO NOTHING;

-- 3. Hubungkan invoice lama ke master yang baru.
UPDATE trx_invoice i
SET pihak_ketiga_id = x.pid,
    pihak_ketiga_rekening_id = x.rid
FROM (
  SELECT i2.id AS iid, p.id AS pid, r.id AS rid
  FROM trx_invoice i2
  JOIN pihak_ketiga p
    ON lower(btrim(p.nama)) = lower(btrim(COALESCE(NULLIF(i2.nama_rekening, ''), 'Pihak Ketiga')))
  JOIN pihak_ketiga_rekening r
    ON r.pihak_ketiga_id = p.id
   AND lower(btrim(r.nama_rekening)) = lower(btrim(COALESCE(NULLIF(i2.nama_rekening, ''), p.nama)))
   AND lower(btrim(r.nomor_rekening)) = lower(btrim(i2.nomor_rekening))
   AND lower(btrim(r.bank)) = lower(btrim(i2.bank))
) x
WHERE i.id = x.iid;
