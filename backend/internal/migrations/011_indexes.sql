-- 011_indexes.sql : index tambahan untuk query yang sering dipakai laporan.
-- Tidak memakai UNIQUE agar aman terhadap data lama yang mungkin duplikat.

CREATE INDEX IF NOT EXISTS idx_aktivitas_sub ON aktivitas(sub_kegiatan_id);
CREATE INDEX IF NOT EXISTS idx_komponen_aktivitas ON komponen(aktivitas_id);
CREATE INDEX IF NOT EXISTS idx_realisasi_komponen ON trx_invoice_realisasi(komponen_id);
CREATE INDEX IF NOT EXISTS idx_invoice_bantuan_tanggal ON trx_invoice(bantuan_id, tanggal);
CREATE INDEX IF NOT EXISTS idx_pencairan_bantuan_tahap ON pencairan_hibah(bantuan_id, tahap);
CREATE INDEX IF NOT EXISTS idx_rekap_bantuan ON rekap_pajak(bantuan_id);
CREATE INDEX IF NOT EXISTS idx_rekap_invoice ON rekap_pajak(invoice_id);
CREATE INDEX IF NOT EXISTS idx_rekap_setor ON rekap_pajak(setor_ledger_id);
CREATE INDEX IF NOT EXISTS idx_bank_bantuan_tanggal ON trx_bank_ledger(bantuan_id, tanggal);
CREATE INDEX IF NOT EXISTS idx_ledger_bantuan_tanggal ON trx_ledger(bantuan_id, tanggal);
