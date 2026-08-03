package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type dumpCol struct {
	Name string
	Typ  string // text | int | bigint | date | timestamptz
}

var dumpTables = []struct {
	Name string
	Cols []dumpCol
}{
	{"bantuan", []dumpCol{{"id", "bigint"}, {"nama", "text"}, {"nama_sekolah", "text"}, {"nama_rekening", "text"}, {"nomor_rekening", "text"}, {"bank", "text"}, {"npwp", "text"}, {"kepala_sekolah", "text"}, {"nip_kepala_sekolah", "text"}, {"bendahara", "text"}, {"nip_bendahara", "text"}, {"nominal", "bigint"}, {"created_at", "timestamptz"}}},
	{"saldo_awal", []dumpCol{{"id", "bigint"}, {"bantuan_id", "bigint"}, {"saldo_awal", "bigint"}, {"tanggal", "date"}, {"created_at", "timestamptz"}}},
	{"kegiatan", []dumpCol{{"id", "bigint"}, {"bantuan_id", "bigint"}, {"nama", "text"}, {"created_at", "timestamptz"}}},
	{"sub_kegiatan", []dumpCol{{"id", "bigint"}, {"kegiatan_id", "bigint"}, {"nama", "text"}, {"created_at", "timestamptz"}}},
	{"komponen", []dumpCol{{"id", "bigint"}, {"sub_kegiatan_id", "bigint"}, {"nama", "text"}, {"pagu", "bigint"}, {"created_at", "timestamptz"}}},
	{"pencairan_hibah", []dumpCol{{"id", "bigint"}, {"bantuan_id", "bigint"}, {"tahap", "int"}, {"tanggal", "date"}, {"nominal", "bigint"}, {"keterangan", "text"}, {"created_at", "timestamptz"}}},
	{"trx_invoice", []dumpCol{{"id", "bigint"}, {"bantuan_id", "bigint"}, {"tanggal", "date"}, {"nomor_bukti", "text"}, {"uraian", "text"}, {"bruto", "bigint"}, {"jenis_menu", "int"}, {"flag_ppn", "int"}, {"flag_pph", "int"}, {"kategori_narasumber", "int"}, {"dpp", "bigint"}, {"dpp_nilai_lain", "bigint"}, {"nilai_ppn", "bigint"}, {"nilai_pph", "bigint"}, {"jenis_pph", "text"}, {"nilai_netto", "bigint"}, {"nama_rekening", "text"}, {"nomor_rekening", "text"}, {"bank", "text"}, {"npwp", "text"}, {"nomor_bupot_ppn", "text"}, {"nomor_bupot_pph", "text"}, {"biaya_admin_dibebankan", "text"}, {"sort_order", "int"}}},
	{"trx_invoice_realisasi", []dumpCol{{"id", "bigint"}, {"invoice_id", "bigint"}, {"komponen_id", "bigint"}, {"bruto", "bigint"}, {"nilai_ppn", "bigint"}, {"nilai_pph", "bigint"}, {"nilai_netto", "bigint"}}},
	{"jasa_giro", []dumpCol{{"id", "bigint"}, {"bantuan_id", "bigint"}, {"tanggal", "date"}, {"nominal", "bigint"}, {"uraian", "text"}}},
	{"trx_ledger", []dumpCol{{"id", "bigint"}, {"bantuan_id", "bigint"}, {"nomor", "int"}, {"tanggal", "date"}, {"nomor_bukti", "text"}, {"uraian", "text"}, {"debit", "bigint"}, {"kredit", "bigint"}, {"saldo", "bigint"}, {"invoice_id", "bigint"}, {"jenis_transaksi", "text"}, {"nomor_bupot", "text"}}},
	{"trx_bank_ledger", []dumpCol{{"id", "bigint"}, {"bantuan_id", "bigint"}, {"nomor", "int"}, {"tanggal", "date"}, {"nomor_bukti", "text"}, {"uraian", "text"}, {"debit", "bigint"}, {"kredit", "bigint"}, {"saldo", "bigint"}, {"invoice_id", "bigint"}, {"jenis_transaksi", "text"}}},
	{"rekap_pajak", []dumpCol{{"id", "bigint"}, {"bantuan_id", "bigint"}, {"tanggal_posting", "date"}, {"ntbn", "text"}, {"ntpn", "text"}, {"invoice_id", "bigint"}, {"jasa_giro", "bigint"}, {"setor_ledger_id", "bigint"}, {"jenis_pajak", "text"}}},
}

func sqlValue(v any, typ string) string {
	if v == nil {
		return "NULL"
	}
	switch typ {
	case "text":
		return "'" + strings.ReplaceAll(fmt.Sprint(v), "'", "''") + "'"
	case "int", "bigint":
		return fmt.Sprint(v)
	case "date":
		if t, ok := v.(time.Time); ok {
			return "'" + t.Format("2006-01-02") + "'"
		}
		return "'" + strings.ReplaceAll(fmt.Sprint(v), "'", "''") + "'"
	case "timestamptz":
		if t, ok := v.(time.Time); ok {
			return "'" + t.UTC().Format("2006-01-02 15:04:05.999999-07") + "'"
		}
		return "'" + strings.ReplaceAll(fmt.Sprint(v), "'", "''") + "'"
	}
	return "NULL"
}

// DumpDatabase menghasilkan SQL dump penuh seluruh database.
func (s *Store) DumpDatabase(ctx context.Context) ([]byte, error) {
	var b strings.Builder
	b.WriteString("-- eBKU DATABASE DUMP\n-- VERSION: 1\n")
	tables := make([]string, len(dumpTables))
	for i, t := range dumpTables {
		tables[i] = t.Name
	}
	b.WriteString("-- TABLES: " + strings.Join(tables, ",") + "\n")
	b.WriteString("-- CREATED: " + time.Now().Format(time.RFC3339) + "\n")
	b.WriteString("BEGIN;\n\n")
	for _, t := range dumpTables {
		colNames := make([]string, len(t.Cols))
		for i, c := range t.Cols {
			colNames[i] = c.Name
		}
		colSQL := strings.Join(colNames, ", ")
		rows, err := s.Pool.Query(ctx, `SELECT `+colSQL+` FROM `+t.Name)
		if err != nil {
			return nil, err
		}
		var values [][]any
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				rows.Close()
				return nil, err
			}
			values = append(values, vals)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(values) > 0 {
			fmt.Fprintf(&b, "DELETE FROM %s;\n", t.Name)
			batch := 100
			for start := 0; start < len(values); start += batch {
				end := start + batch
				if end > len(values) {
					end = len(values)
				}
				fmt.Fprintf(&b, "INSERT INTO %s (%s) VALUES\n", t.Name, colSQL)
				for i := start; i < end; i++ {
					row := values[i]
					parts := make([]string, len(row))
					for j, v := range row {
						parts[j] = sqlValue(v, t.Cols[j].Typ)
					}
					comma := ","
					if i == end-1 {
						comma = ";"
					}
					fmt.Fprintf(&b, "(%s)%s\n", strings.Join(parts, ", "), comma)
				}
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("-- reset sequences\n")
	b.WriteString("SELECT setval(pg_get_serial_sequence('bantuan','id'), COALESCE((SELECT MAX(id) FROM bantuan),1), (SELECT MAX(id) FROM bantuan) IS NOT NULL);\n")
	for _, t := range dumpTables {
		if t.Name == "bantuan" {
			continue
		}
		fmt.Fprintf(&b, "SELECT setval(pg_get_serial_sequence('%s','id'), COALESCE((SELECT MAX(id) FROM %s),1), (SELECT MAX(id) FROM %s) IS NOT NULL);\n", t.Name, t.Name, t.Name)
	}
	b.WriteString("\nCOMMIT;\n")
	return []byte(b.String()), nil
}

// ValidateDump memeriksa bahwa isi dump sah (memuat penanda versi dan tabel inti).
func ValidateDump(data []byte) error {
	text := string(data)
	if !strings.Contains(text, "-- eBKU DATABASE DUMP") {
		return fmt.Errorf("file bukan dump e-BKU")
	}
	for _, core := range []string{"bantuan", "trx_invoice", "trx_ledger"} {
		if !strings.Contains(text, "-- TABLES:") {
			return fmt.Errorf("struktur dump tidak valid")
		}
		line := ""
		for _, l := range strings.Split(text, "\n") {
			if strings.HasPrefix(l, "-- TABLES:") {
				line = l
				break
			}
		}
		if !strings.Contains(line, core) {
			return fmt.Errorf("tabel inti %s tidak ditemukan di struktur dump", core)
		}
	}
	if !strings.Contains(text, "COMMIT;") {
		return fmt.Errorf("dump tidak lengkap")
	}
	return nil
}

// ApplyDump mengganti seluruh isi database dengan isi dump.
func (s *Store) ApplyDump(ctx context.Context, data []byte) error {
	if _, err := s.Pool.Exec(ctx, string(data)); err != nil {
		return fmt.Errorf("restore gagal: %w", err)
	}
	return nil
}

// BackupFile menyimpan dump ke file baru di folder dir dengan nama ber-timestamp.
func (s *Store) BackupFile(ctx context.Context, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := s.DumpDatabase(ctx)
	if err != nil {
		return "", err
	}
	name := "ebku_backup_" + time.Now().Format("20060102_150405") + ".db"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, s.PruneBackups(dir, 10)
}

// PruneBackups menyisakan N backup terakhir, sisanya dihapus.
func (s *Store) PruneBackups(dir string, keep int) error {
	entries, err := filepath.Glob(filepath.Join(dir, "ebku_backup_*.db"))
	if err != nil {
		return err
	}
	sort.Strings(entries)
	if len(entries) <= keep {
		return nil
	}
	for _, e := range entries[:len(entries)-keep] {
		_ = os.Remove(e)
	}
	return nil
}
