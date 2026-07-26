// Package sqliterepo persists proxy history to a single-writer SQLite
// database (via the pure-Go modernc.org/sqlite driver, so no cgo/system
// SQLite is required). It's the only implementation of
// entity.HistoryRepository.
package sqliterepo

import (
	"database/sql"

	_ "modernc.org/sqlite"

	"nullsec/backend/internal/entity"
)

type Repository struct {
	db *sql.DB
}

func Open(path string) (*Repository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	// SQLite only allows one writer at a time; a concurrent proxy handling
	// several requests in parallel (e.g. a browser loading many assets at
	// once) will otherwise hit SQLITE_BUSY. A single connection serializes
	// all access instead of trying (and failing) to write concurrently.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		return nil, err
	}
	r := &Repository{db: db}
	if err := r.migrate(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) migrate() error {
	_, err := r.db.Exec(`
CREATE TABLE IF NOT EXISTS requests (
	id TEXT PRIMARY KEY,
	method TEXT NOT NULL,
	url TEXT NOT NULL,
	host TEXT NOT NULL,
	headers TEXT NOT NULL,
	body TEXT NOT NULL,
	status INTEGER NOT NULL DEFAULT 0,
	resp_headers TEXT NOT NULL DEFAULT '',
	resp_body TEXT NOT NULL DEFAULT '',
	size INTEGER NOT NULL DEFAULT 0,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	outcome TEXT NOT NULL DEFAULT 'completed',
	created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_requests_host ON requests(host);
CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status);
CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at);
`)
	return err
}

// Insert implements entity.HistoryRepository.
func (r *Repository) Insert(req *entity.Request) error {
	_, err := r.db.Exec(`
INSERT INTO requests (id, method, url, host, headers, body, status, resp_headers, resp_body, size, duration_ms, outcome, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.Method, req.URL, req.Host, req.Headers, req.Body, req.Status, req.RespHeaders, req.RespBody, req.Size, req.DurationMs, req.Outcome, req.CreatedAt,
	)
	return err
}

func where(f entity.RequestFilter) (string, []any) {
	query := ` WHERE 1=1`
	var args []any
	if f.Host != "" {
		query += ` AND host LIKE ?`
		args = append(args, "%"+f.Host+"%")
	}
	if f.Method != "" {
		query += ` AND method = ?`
		args = append(args, f.Method)
	}
	if f.Status != 0 {
		query += ` AND status = ?`
		args = append(args, f.Status)
	}
	return query, args
}

// List implements entity.HistoryRepository. It returns full rows (including
// request/response bodies) matching f — used where the body content itself
// is needed (e.g. the passive scanner). Prefer ListSummary for anything
// that only renders a row list.
func (r *Repository) List(f entity.RequestFilter) ([]entity.Request, error) {
	clause, args := where(f)
	// clause is always a fixed skeleton of `?` placeholders built by
	// where() — no user-supplied value is ever concatenated directly into
	// the query string; every value travels through args.
	query := `SELECT id, method, url, host, headers, body, status, resp_headers, resp_body, size, duration_ms, outcome, created_at FROM requests` + clause + ` ORDER BY created_at DESC` //nolint:gosec // G202: placeholders only, see comment above
	if f.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, f.Limit)
		if f.Offset > 0 {
			query += ` OFFSET ?`
			args = append(args, f.Offset)
		}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.Request
	for rows.Next() {
		var req entity.Request
		if err := rows.Scan(&req.ID, &req.Method, &req.URL, &req.Host, &req.Headers, &req.Body, &req.Status, &req.RespHeaders, &req.RespBody, &req.Size, &req.DurationMs, &req.Outcome, &req.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

// ListSummary implements entity.HistoryRepository. It's the fast path for
// the history table: no body/header columns are read off disk at all, and
// paired with Count it supports real pagination instead of always pulling
// everything up to Limit.
func (r *Repository) ListSummary(f entity.RequestFilter) ([]entity.RequestSummary, error) {
	clause, args := where(f)
	query := `SELECT id, method, url, host, status, size, duration_ms, outcome, created_at FROM requests` + clause + ` ORDER BY created_at DESC` //nolint:gosec // G202: placeholders only, see List() above
	if f.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, f.Limit)
		if f.Offset > 0 {
			query += ` OFFSET ?`
			args = append(args, f.Offset)
		}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []entity.RequestSummary{}
	for rows.Next() {
		var s entity.RequestSummary
		if err := rows.Scan(&s.ID, &s.Method, &s.URL, &s.Host, &s.Status, &s.Size, &s.DurationMs, &s.Outcome, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Count implements entity.HistoryRepository, returning the total number of
// rows matching f, ignoring Limit/Offset — used to compute total page count.
func (r *Repository) Count(f entity.RequestFilter) (int, error) {
	clause, args := where(f)
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM requests`+clause, args...).Scan(&total)
	return total, err
}

// Clear implements entity.HistoryRepository, deleting all recorded requests.
func (r *Repository) Clear() error {
	_, err := r.db.Exec(`DELETE FROM requests`)
	return err
}

// Get implements entity.HistoryRepository.
func (r *Repository) Get(id string) (*entity.Request, error) {
	row := r.db.QueryRow(`SELECT id, method, url, host, headers, body, status, resp_headers, resp_body, size, duration_ms, outcome, created_at FROM requests WHERE id = ?`, id)
	var req entity.Request
	if err := row.Scan(&req.ID, &req.Method, &req.URL, &req.Host, &req.Headers, &req.Body, &req.Status, &req.RespHeaders, &req.RespBody, &req.Size, &req.DurationMs, &req.Outcome, &req.CreatedAt); err != nil {
		return nil, err
	}
	return &req, nil
}
