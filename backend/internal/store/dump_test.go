package store

import (
	"strings"
	"testing"
)

func validDump(body string) []byte {
	return []byte("-- eBKU DATABASE DUMP\n-- VERSION: 1\n-- TABLES: bantuan,trx_invoice,trx_ledger\nBEGIN;\n" +
		"DELETE FROM bantuan;\n" +
		"INSERT INTO bantuan (id, nama) VALUES\n(1, 'Sekolah');\n" +
		"SELECT setval(pg_get_serial_sequence('bantuan','id'), 1, true);\nCOMMIT;\n" + body)
}

func TestValidateDumpAcceptsOwnFormat(t *testing.T) {
	if err := ValidateDump(validDump("")); err != nil {
		t.Fatalf("dump valid ditolak: %v", err)
	}
}

func TestValidateDumpRejectsDangerousStatements(t *testing.T) {
	cases := []string{
		"DROP TABLE bantuan;",
		"ALTER TABLE bantuan ADD COLUMN x INT;",
		"CREATE EXTENSION plpython3u;",
		"COPY bantuan FROM PROGRAM 'calc.exe';",
		"SELECT pg_read_file('/etc/passwd');",
		"TRUNCATE bantuan;",
	}
	for _, c := range cases {
		err := ValidateDump(validDump(c))
		if err == nil {
			t.Errorf("statement berbahaya lolos: %s", c)
		}
	}
}

func TestValidateDumpRejectsUnknownTable(t *testing.T) {
	data := []byte("-- eBKU DATABASE DUMP\n-- VERSION: 1\n-- TABLES: bantuan,trx_invoice,trx_ledger\nBEGIN;\n" +
		"DELETE FROM rahasia;\nCOMMIT;\n")
	if err := ValidateDump(data); err == nil || !strings.Contains(err.Error(), "tidak dikenal") {
		t.Fatalf("tabel tak dikenal harus ditolak, err=%v", err)
	}
}

func TestValidateDumpRejectsTooLarge(t *testing.T) {
	data := make([]byte, MaxDumpBytes+1)
	if err := ValidateDump(data); err == nil {
		t.Fatal("dump terlalu besar harus ditolak")
	}
}
