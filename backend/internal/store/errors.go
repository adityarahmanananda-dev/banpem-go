package store

import "errors"

var (
	ErrInUse                 = errors.New("data masih dipakai transaksi lain")
	ErrInvoiceHasTaxDeposit  = errors.New("tagihan sudah memiliki setoran pajak; batalkan setoran terlebih dahulu")
	ErrTaxAlreadyDeposited   = errors.New("pajak tagihan tersebut sudah disetor")
	ErrRealisasiMismatch     = errors.New("total realisasi harus sama dengan bruto tagihan")
	ErrBudgetExceeded        = errors.New("realisasi melebihi pagu komponen")
	ErrDuplicateInvoiceNo    = errors.New("nomor bukti tagihan sudah dipakai")
	ErrPencairanExceedsTotal = errors.New("total pencairan melebihi nominal bantuan")
	ErrInvalidInput          = errors.New("input tidak valid")
	ErrNotFound              = errors.New("data tidak ditemukan")
)
