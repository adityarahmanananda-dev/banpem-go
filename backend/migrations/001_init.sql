-- 001_init.sql : skema 12 tabel logis e-BKU
-- Idempotent: dipakai ulang untuk import database dan startup.

CREATE TABLE IF NOT EXISTS bantuan (
  id BIGSERIAL PRIMARY KEY,
  nama TEXT NOT NULL DEFAULT '',
  nama_rekening TEXT NOT NULL DEFAULT '',
  nomor_rekening TEXT NOT NULL DEFAULT '',
  bank TEXT NOT NULL DEFAULT '',
  npwp TEXT NOT NULL DEFAULT '',
  kepala_sekolah TEXT NOT NULL DEFAULT '',
  nip_kepala_sekolah TEXT NOT NULL DEFAULT '',
  bendahara TEXT NOT NULL DEFAULT '',
  nip_bendahara TEXT NOT NULL DEFAULT '',
  nominal BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS saldo_awal (
  id BIGSERIAL PRIMARY KEY,
  bantuan_id BIGINT NOT NULL UNIQUE REFERENCES bantuan(id) ON DELETE CASCADE,
  saldo_awal BIGINT NOT NULL DEFAULT 0,
  tanggal DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS trx_invoice (
  id BIGSERIAL PRIMARY KEY,
  bantuan_id BIGINT NOT NULL REFERENCES bantuan(id) ON DELETE CASCADE,
  tanggal DATE NOT NULL,
  nomor_bukti TEXT NOT NULL DEFAULT '',
  uraian TEXT NOT NULL DEFAULT '',
  bruto BIGINT NOT NULL DEFAULT 0,
  jenis_menu INT NOT NULL DEFAULT 1,
  flag_ppn INT NOT NULL DEFAULT 2,
  flag_pph INT NOT NULL DEFAULT 3,
  kategori_narasumber INT,
  dpp BIGINT NOT NULL DEFAULT 0,
  dpp_nilai_lain BIGINT NOT NULL DEFAULT 0,
  nilai_ppn BIGINT NOT NULL DEFAULT 0,
  nilai_pph BIGINT NOT NULL DEFAULT 0,
  jenis_pph TEXT,
  nilai_netto BIGINT NOT NULL DEFAULT 0,
  nama_rekening TEXT NOT NULL DEFAULT '',
  nomor_rekening TEXT NOT NULL DEFAULT '',
  bank TEXT NOT NULL DEFAULT '',
  npwp TEXT NOT NULL DEFAULT '',
  nomor_bupot_ppn TEXT NOT NULL DEFAULT '',
  nomor_bupot_pph TEXT NOT NULL DEFAULT '',
  biaya_admin_dibebankan TEXT NOT NULL DEFAULT 'tidak',
  sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS trx_ledger (
  id BIGSERIAL PRIMARY KEY,
  bantuan_id BIGINT NOT NULL REFERENCES bantuan(id) ON DELETE CASCADE,
  nomor INT NOT NULL,
  tanggal DATE,
  nomor_bukti TEXT NOT NULL DEFAULT '',
  uraian TEXT NOT NULL DEFAULT '',
  debit BIGINT NOT NULL DEFAULT 0,
  kredit BIGINT NOT NULL DEFAULT 0,
  saldo BIGINT NOT NULL DEFAULT 0,
  invoice_id BIGINT REFERENCES trx_invoice(id) ON DELETE SET NULL,
  jenis_transaksi TEXT NOT NULL DEFAULT '',
  nomor_bupot TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS trx_bank_ledger (
  id BIGSERIAL PRIMARY KEY,
  bantuan_id BIGINT NOT NULL REFERENCES bantuan(id) ON DELETE CASCADE,
  nomor INT NOT NULL,
  tanggal DATE,
  nomor_bukti TEXT NOT NULL DEFAULT '',
  uraian TEXT NOT NULL DEFAULT '',
  debit BIGINT NOT NULL DEFAULT 0,
  kredit BIGINT NOT NULL DEFAULT 0,
  saldo BIGINT NOT NULL DEFAULT 0,
  invoice_id BIGINT REFERENCES trx_invoice(id) ON DELETE SET NULL,
  jenis_transaksi TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rekap_pajak (
  id BIGSERIAL PRIMARY KEY,
  bantuan_id BIGINT NOT NULL REFERENCES bantuan(id) ON DELETE CASCADE,
  tanggal_posting DATE,
  ntbn TEXT NOT NULL DEFAULT '',
  ntpn TEXT NOT NULL DEFAULT '',
  invoice_id BIGINT REFERENCES trx_invoice(id) ON DELETE SET NULL,
  jasa_giro BIGINT NOT NULL DEFAULT 0,
  setor_ledger_id BIGINT,
  jenis_pajak TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS jasa_giro (
  id BIGSERIAL PRIMARY KEY,
  bantuan_id BIGINT NOT NULL REFERENCES bantuan(id) ON DELETE CASCADE,
  tanggal DATE,
  nominal BIGINT NOT NULL DEFAULT 0,
  uraian TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS kegiatan (
  id BIGSERIAL PRIMARY KEY,
  bantuan_id BIGINT NOT NULL REFERENCES bantuan(id) ON DELETE CASCADE,
  nama TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sub_kegiatan (
  id BIGSERIAL PRIMARY KEY,
  kegiatan_id BIGINT NOT NULL REFERENCES kegiatan(id) ON DELETE CASCADE,
  nama TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS komponen (
  id BIGSERIAL PRIMARY KEY,
  sub_kegiatan_id BIGINT NOT NULL REFERENCES sub_kegiatan(id) ON DELETE CASCADE,
  nama TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS trx_invoice_realisasi (
  id BIGSERIAL PRIMARY KEY,
  invoice_id BIGINT NOT NULL REFERENCES trx_invoice(id) ON DELETE CASCADE,
  komponen_id BIGINT NOT NULL REFERENCES komponen(id) ON DELETE CASCADE,
  bruto BIGINT NOT NULL DEFAULT 0,
  nilai_ppn BIGINT NOT NULL DEFAULT 0,
  nilai_pph BIGINT NOT NULL DEFAULT 0,
  nilai_netto BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pencairan_hibah (
  id BIGSERIAL PRIMARY KEY,
  bantuan_id BIGINT NOT NULL REFERENCES bantuan(id) ON DELETE CASCADE,
  tahap INT NOT NULL DEFAULT 1,
  tanggal DATE,
  nominal BIGINT NOT NULL DEFAULT 0,
  keterangan TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ledger_bantuan ON trx_ledger(bantuan_id, nomor);
CREATE INDEX IF NOT EXISTS idx_bank_bantuan ON trx_bank_ledger(bantuan_id, nomor);
CREATE INDEX IF NOT EXISTS idx_invoice_bantuan ON trx_invoice(bantuan_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_realisasi_invoice ON trx_invoice_realisasi(invoice_id);
CREATE INDEX IF NOT EXISTS idx_rekap_setor ON rekap_pajak(setor_ledger_id);
CREATE INDEX IF NOT EXISTS idx_sub_kegiatan ON sub_kegiatan(kegiatan_id);
CREATE INDEX IF NOT EXISTS idx_komponen_sub ON komponen(sub_kegiatan_id);
