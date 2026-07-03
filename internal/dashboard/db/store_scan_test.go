package db

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newScanDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.Exec(`CREATE TABLE t (id INTEGER, name TEXT, blob BLOB, opt TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO t (id, name, blob, opt) VALUES
		(1, 'alpha', x'6162', 'x'),
		(2, 'beta', x'6364', NULL)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return conn
}

func TestScanDynamicRows(t *testing.T) {
	conn := newScanDB(t)
	rows, err := QueryOnDB(conn, `SELECT id, name, blob, opt FROM t ORDER BY id`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	results, columns, err := ScanDynamicRows(rows)
	if err != nil {
		t.Fatalf("ScanDynamicRows: %v", err)
	}
	wantCols := []string{"id", "name", "blob", "opt"}
	if len(columns) != len(wantCols) {
		t.Fatalf("columns = %v, want %v", columns, wantCols)
	}
	for i, c := range wantCols {
		if columns[i] != c {
			t.Fatalf("columns[%d] = %q, want %q", i, columns[i], c)
		}
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(results))
	}
	// []byte columns are stringified.
	if results[0]["blob"] != "ab" {
		t.Errorf("blob row0 = %v (%T), want \"ab\"", results[0]["blob"], results[0]["blob"])
	}
	if results[0]["name"] != "alpha" {
		t.Errorf("name row0 = %v, want alpha", results[0]["name"])
	}
	// NULL columns preserved as nil.
	if results[1]["opt"] != nil {
		t.Errorf("opt row1 = %v, want nil", results[1]["opt"])
	}
}

func TestScanFirstColumn(t *testing.T) {
	conn := newScanDB(t)
	rows, err := QueryOnDB(conn, `SELECT name FROM t ORDER BY id`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	values, err := ScanFirstColumn(rows)
	if err != nil {
		t.Fatalf("ScanFirstColumn: %v", err)
	}
	if len(values) != 2 || values[0] != "alpha" || values[1] != "beta" {
		t.Fatalf("values = %v", values)
	}
}

func TestScanFirstColumnStringifiesBytes(t *testing.T) {
	conn := newScanDB(t)
	rows, err := QueryOnDB(conn, `SELECT blob FROM t ORDER BY id`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	values, err := ScanFirstColumn(rows)
	if err != nil {
		t.Fatalf("ScanFirstColumn: %v", err)
	}
	if len(values) != 2 || values[0] != "ab" || values[1] != "cd" {
		t.Fatalf("blob values = %v", values)
	}
}

func TestNilRowAndRowsWrappers(t *testing.T) {
	var nilRow *Row
	if err := nilRow.Scan(new(int)); err == nil {
		t.Error("nil *Row.Scan should error")
	}
	if err := (&Row{}).Scan(new(int)); err == nil {
		t.Error("empty Row.Scan should error")
	}

	var nilRows *Rows
	if nilRows.Next() {
		t.Error("nil *Rows.Next should be false")
	}
	if err := nilRows.Scan(new(int)); err == nil {
		t.Error("nil *Rows.Scan should error")
	}
	if err := nilRows.Err(); err != nil {
		t.Errorf("nil *Rows.Err should be nil, got %v", err)
	}
	if err := nilRows.Close(); err != nil {
		t.Errorf("nil *Rows.Close should be nil, got %v", err)
	}
	if _, err := nilRows.Columns(); err == nil {
		t.Error("nil *Rows.Columns should error")
	}
}

func TestQueryRowContextOnDB(t *testing.T) {
	conn := newScanDB(t)
	var name string
	if err := QueryRowContextOnDB(context.Background(), conn, `SELECT name FROM t WHERE id = ?`, 2).Scan(&name); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if name != "beta" {
		t.Errorf("name = %q, want beta", name)
	}
}
