-- 005_backfill_bupot_admin.sql
-- Perbaikan data lama (idempotent):
--   1. Baris BKU 'pungut_ppn' / 'pungut_pph' (termasuk PPh 21) yang nomor
--      buktinya kosong/salah diisi ulang dengan nomor bupot dari tagihan.
--   2. Baris Bank 'biaya_admin' nomor buktinya disamakan dengan nomor bukti
--      pembayaran utama pada invoice yang sama.
-- Kedua nomor_bukti juga ditulis ke kolom nomor_bupot untuk pungut BKU.

-- 1a. Pungut PPN: nomor_bukti/nomor_bupot = nomor_bupot_ppn tagihan.
UPDATE trx_ledger l
SET nomor_bukti = i.nomor_bupot_ppn,
    nomor_bupot = i.nomor_bupot_ppn
FROM trx_invoice i
WHERE l.invoice_id = i.id
  AND l.jenis_transaksi = 'pungut_ppn'
  AND i.nomor_bupot_ppn <> ''
  AND l.nomor_bukti IS DISTINCT FROM i.nomor_bupot_ppn;

-- 1b. Pungut PPh (PPh 21/22/23): nomor_bukti/nomor_bupot = nomor_bupot_pph tagihan.
UPDATE trx_ledger l
SET nomor_bukti = i.nomor_bupot_pph,
    nomor_bupot = i.nomor_bupot_pph
FROM trx_invoice i
WHERE l.invoice_id = i.id
  AND l.jenis_transaksi = 'pungut_pph'
  AND i.nomor_bupot_pph <> ''
  AND l.nomor_bukti IS DISTINCT FROM i.nomor_bupot_pph;

-- 2. Biaya admin Bank: nomor bukti = nomor bukti pembayaran utama invoice sama.
UPDATE trx_bank_ledger b
SET nomor_bukti = p.nomor_bukti
FROM trx_bank_ledger p
WHERE b.jenis_transaksi = 'biaya_admin'
  AND b.invoice_id IS NOT NULL
  AND b.invoice_id = p.invoice_id
  AND p.jenis_transaksi = 'pembayaran'
  AND p.nomor_bukti <> ''
  AND b.nomor_bukti IS DISTINCT FROM p.nomor_bukti;