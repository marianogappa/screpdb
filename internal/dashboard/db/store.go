package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Store struct {
	defaultDB     *sql.DB
	replayScoped  func() *sql.DB
	withFiltered  func(*string, func(*sql.DB) error) error
}

func NewStore(defaultDB *sql.DB, replayScoped func() *sql.DB, withFiltered func(*string, func(*sql.DB) error) error) *Store {
	return &Store{
		defaultDB:    defaultDB,
		replayScoped: replayScoped,
		withFiltered: withFiltered,
	}
}

type Row struct {
	row *sql.Row
}

func (r *Row) Scan(dest ...any) error {
	if r == nil || r.row == nil {
		return errors.New("row is nil")
	}
	return r.row.Scan(dest...)
}

type Rows struct {
	rows *sql.Rows
}

func (r *Rows) Next() bool {
	if r == nil || r.rows == nil {
		return false
	}
	return r.rows.Next()
}

func (r *Rows) Scan(dest ...any) error {
	if r == nil || r.rows == nil {
		return errors.New("rows is nil")
	}
	return r.rows.Scan(dest...)
}

func (r *Rows) Err() error {
	if r == nil || r.rows == nil {
		return nil
	}
	return r.rows.Err()
}

func (r *Rows) Close() error {
	if r == nil || r.rows == nil {
		return nil
	}
	return r.rows.Close()
}

func (r *Rows) Columns() ([]string, error) {
	if r == nil || r.rows == nil {
		return nil, errors.New("rows is nil")
	}
	return r.rows.Columns()
}

func (s *Store) DefaultExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	res, err := s.defaultDB.ExecContext(ctx, query, args...)
	logIfNoteworthy("EXEC", query, time.Since(start), 0, err)
	return res, err
}

func (s *Store) DefaultQueryContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	start := time.Now()
	rows, err := s.defaultDB.QueryContext(ctx, query, args...)
	logIfNoteworthy("QUERY", query, time.Since(start), 0, err)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

func (s *Store) DefaultQueryRowContext(ctx context.Context, query string, args ...any) *Row {
	start := time.Now()
	row := s.defaultDB.QueryRowContext(ctx, query, args...)
	logIfNoteworthy("QUERYROW", query, time.Since(start), 0, nil)
	return &Row{row: row}
}

func (s *Store) DefaultQueryRow(query string, args ...any) *Row {
	start := time.Now()
	row := s.defaultDB.QueryRow(query, args...)
	logIfNoteworthy("QUERYROW", query, time.Since(start), 0, nil)
	return &Row{row: row}
}

func (s *Store) ReplayQueryContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	db := s.replayScoped()
	start := time.Now()
	rows, err := db.QueryContext(ctx, query, args...)
	logIfNoteworthy("QUERY", query, time.Since(start), 0, err)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

func (s *Store) ReplayQueryRowContext(ctx context.Context, query string, args ...any) *Row {
	start := time.Now()
	row := s.replayScoped().QueryRowContext(ctx, query, args...)
	logIfNoteworthy("QUERYROW", query, time.Since(start), 0, nil)
	return &Row{row: row}
}

func (s *Store) WithFilteredConnection(replaysFilterSQL *string, fn func(*sql.DB) error) error {
	return s.withFiltered(replaysFilterSQL, fn)
}

func QueryContextOnDB(ctx context.Context, db *sql.DB, query string, args ...any) (*Rows, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

func QueryRowContextOnDB(ctx context.Context, db *sql.DB, query string, args ...any) *Row {
	return &Row{row: db.QueryRowContext(ctx, query, args...)}
}

func ExecContextOnDB(ctx context.Context, db *sql.DB, query string, args ...any) (sql.Result, error) {
	return db.ExecContext(ctx, query, args...)
}

func QueryOnDB(db *sql.DB, query string, args ...any) (*Rows, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

func ScanDynamicRows(rows *Rows) ([]map[string]any, []string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	results := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, err
		}

		row := make(map[string]any, len(columns))
		for i, col := range columns {
			val := values[i]
			switch v := val.(type) {
			case []byte:
				row[col] = string(v)
			case nil:
				row[col] = nil
			default:
				row[col] = v
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return results, columns, nil
}

func ScanDynamicSQLRows(rows *sql.Rows) ([]map[string]any, []string, error) {
	return ScanDynamicRows(&Rows{rows: rows})
}

func ScanFirstColumn(rows *Rows) ([]any, error) {
	values := []any{}
	for rows.Next() {
		var value any
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if b, ok := value.([]byte); ok {
			value = string(b)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func ScanFirstColumnSQLRows(rows *sql.Rows) ([]any, error) {
	return ScanFirstColumn(&Rows{rows: rows})
}
