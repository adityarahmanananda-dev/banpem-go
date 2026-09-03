-- 006_setor_nomor_bukti_ntpn.sql
-- Nomor bukti entri 'setor_pajak' cukup memuat "NTPN=..." (tanpa NTB).
-- Data lama yang berbentuk "NTB=...\nNTPN=..." maupun "NTB=\n...\nNTPN=\n..."
-- dipangkas jadi satu baris "NTPN=<nilai>". Bila tidak ada NTPN, NTB
-- dipertahankan. Idempotent.

UPDATE trx_ledger
SET nomor_bukti = CASE
  WHEN POSITION('NTPN=' IN nomor_bukti) > 0
    THEN 'NTPN=' || BTRIM(SUBSTRING(nomor_bukti FROM POSITION('NTPN=' IN nomor_bukti) + 5), E'\r\n \t')
  WHEN POSITION('NTB=' IN nomor_bukti) > 0
    THEN 'NTB=' || BTRIM(SUBSTRING(nomor_bukti FROM POSITION('NTB=' IN nomor_bukti) + 4), E'\r\n \t')
  ELSE nomor_bukti
END
WHERE jenis_transaksi = 'setor_pajak'
  AND nomor_bukti <> '';

UPDATE trx_bank_ledger
SET nomor_bukti = CASE
  WHEN POSITION('NTPN=' IN nomor_bukti) > 0
    THEN 'NTPN=' || BTRIM(SUBSTRING(nomor_bukti FROM POSITION('NTPN=' IN nomor_bukti) + 5), E'\r\n \t')
  WHEN POSITION('NTB=' IN nomor_bukti) > 0
    THEN 'NTB=' || BTRIM(SUBSTRING(nomor_bukti FROM POSITION('NTB=' IN nomor_bukti) + 4), E'\r\n \t')
  ELSE nomor_bukti
END
WHERE jenis_transaksi = 'setor_pajak'
  AND nomor_bukti <> '';