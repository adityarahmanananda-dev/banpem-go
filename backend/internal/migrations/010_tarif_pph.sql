-- 010_tarif_pph.sql
-- Tarif PPh manual (persen) untuk PPh 21 dan PPh Pasal 4 Ayat 2 pada
-- jenis pengeluaran Barang/Jasa. Nilai 0 berarti tidak memakai tarif manual.
ALTER TABLE trx_invoice ADD COLUMN IF NOT EXISTS tarif_pph DOUBLE PRECISION NOT NULL DEFAULT 0;
