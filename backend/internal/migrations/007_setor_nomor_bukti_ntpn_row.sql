-- 007_setor_nomor_bukti_ntpn_row.sql
-- Nomor bukti entri 'setor_pajak' ditampilkan dalam dua baris:
--   NTPN
--   =<nilai>
-- Data lama ("NTPN=<nilai>", "NTPN=\n<nilai>", atau "NTB=<nilai>") dikonversi
-- ke format baris ganda. Idempotent: format baris ganda dibiarkan apa adanya.

UPDATE trx_ledger
SET nomor_bukti = CASE
  WHEN POSITION('NTPN' IN nomor_bukti) > 0
    THEN 'NTPN' || E'\n=' || BTRIM(SUBSTRING(nomor_bukti FROM POSITION('NTPN' IN nomor_bukti) + 4), E'\r\n \t=')
  WHEN POSITION('NTB' IN nomor_bukti) > 0
    THEN 'NTB' || E'\n=' || BTRIM(SUBSTRING(nomor_bukti FROM POSITION('NTB' IN nomor_bukti) + 3), E'\r\n \t=')
  ELSE nomor_bukti
END
WHERE jenis_transaksi = 'setor_pajak'
  AND nomor_bukti <> '';

UPDATE trx_bank_ledger
SET nomor_bukti = CASE
  WHEN POSITION('NTPN' IN nomor_bukti) > 0
    THEN 'NTPN' || E'\n=' || BTRIM(SUBSTRING(nomor_bukti FROM POSITION('NTPN' IN nomor_bukti) + 4), E'\r\n \t=')
  WHEN POSITION('NTB' IN nomor_bukti) > 0
    THEN 'NTB' || E'\n=' || BTRIM(SUBSTRING(nomor_bukti FROM POSITION('NTB' IN nomor_bukti) + 3), E'\r\n \t=')
  ELSE nomor_bukti
END
WHERE jenis_transaksi = 'setor_pajak'
  AND nomor_bukti <> '';