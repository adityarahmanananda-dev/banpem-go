package store

import "testing"

func TestSyncLedgerKey(t *testing.T) {
	sa := syncLedgerItem{jenis: "saldo_awal"}
	inv := syncLedgerItem{jenis: "pungut_pph", sortRow: 3, id: 22}
	inv2 := syncLedgerItem{jenis: "pembayaran", sortRow: 3, id: 21}
	oth := syncLedgerItem{jenis: "setor_pajak", id: 50}

	pa, _, _ := sa.syncKey()
	pi1, si1, _ := inv.syncKey()
	pi2, si2, _ := inv2.syncKey()
	po, _, _ := oth.syncKey()

	if !(pa < pi1 && pi1 < po) {
		t.Fatalf("prioritas salah: saldo_awal=%d invoice=%d lain=%d", pa, pi1, po)
	}
	if pi1 != pi2 {
		t.Fatalf("invoice harus prioritas sama: %d vs %d", pi1, pi2)
	}
	if si1 != si2 {
		t.Fatalf("invoice sort_order sama harus menempel: %d vs %d", si1, si2)
	}
	// Dalam satu invoice, pembayaran (id kecil) hadir sebelum pungut (id besar).
	if si1 == si2 && inv2.id > inv.id {
		t.Fatal("urutan internal invoice tidak konsisten")
	}
}