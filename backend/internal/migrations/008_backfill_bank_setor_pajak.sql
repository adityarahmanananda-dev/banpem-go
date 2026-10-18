-- 008_backfill_bank_setor_pajak.sql
-- Perbaikan data lama (idempotent): setiap entri 'setor_pajak' wajib ada di
-- kedua buku (BKU dan Buku Kas Bank) karena penyetoran pajak mengurangi saldo
-- bank. Bila ada entri BKU 'setor_pajak' yang belum punya pasangan di
-- trx_bank_ledger, tambahkan entri bank yang sesuai. Pemasangan dilakukan
-- berurut memakai ROW_NUMBER dengan kunci (bantuan_id, tanggal, uraian, debit).
-- Kolom nomor/saldo dirapikan oleh ResyncAllLedger saat startup.

INSERT INTO trx_bank_ledger
  (bantuan_id, nomor, tanggal, nomor_bukti, uraian, debit, kredit, saldo, invoice_id, jenis_transaksi)
SELECT l.bantuan_id, 0, l.tanggal, l.nomor_bukti, l.uraian, l.debit, 0, 0, NULL, 'setor_pajak'
FROM (
  SELECT id, bantuan_id, tanggal, nomor_bukti, uraian, debit,
         ROW_NUMBER() OVER (
           PARTITION BY bantuan_id, tanggal, uraian, debit
           ORDER BY id
         ) AS rn
  FROM trx_ledger
  WHERE jenis_transaksi = 'setor_pajak'
) l
WHERE l.rn > (
  SELECT COUNT(*)
  FROM trx_bank_ledger b
  WHERE b.jenis_transaksi = 'setor_pajak'
    AND b.bantuan_id = l.bantuan_id
    AND b.tanggal IS NOT DISTINCT FROM l.tanggal
    AND b.uraian = l.uraian
    AND b.debit = l.debit
);
