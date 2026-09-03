package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Store struct {
	defaultDB    *sql.DB
	replayScoped func() *sql.DB
}

func NewStore(defaultDB *sql.DB, replayScoped func() *sql.DB) *Store {
	return &Store{
		defaultDB:    defaultDB,
		replayScoped: replayScoped,
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
