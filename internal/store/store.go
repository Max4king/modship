package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ryan3311/modship/internal/model"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no CGO
)

// Store wraps a SQLite database for modship state.
type Store struct {
	db *sql.DB
}

// Open creates or opens the SQLite database at path and runs migrations.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite serializes writes anyway
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS servers (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL UNIQUE,
    provider            TEXT NOT NULL,
    modpack_slug        TEXT NOT NULL,
    modpack_name        TEXT NOT NULL DEFAULT '',
    file_id             TEXT NOT NULL,
    game_version        TEXT NOT NULL DEFAULT '',
    java_image          TEXT NOT NULL DEFAULT 'java17',
    memory              TEXT NOT NULL DEFAULT '10G',
    domain              TEXT NOT NULL DEFAULT '',
    cloudflare_record_id TEXT NOT NULL DEFAULT '',
    container_id        TEXT NOT NULL DEFAULT '',
    compose_path        TEXT NOT NULL DEFAULT '',
    state               TEXT NOT NULL DEFAULT 'creating',
    whitelist           TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);`)
	return err
}

// CreateServer inserts a new server row.
func (s *Store) CreateServer(ctx context.Context, srv *model.Server) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if srv.CreatedAt.IsZero() {
		srv.CreatedAt = time.Now()
	}
	if srv.UpdatedAt.IsZero() {
		srv.UpdatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO servers (id, name, provider, modpack_slug, modpack_name, file_id,
    game_version, java_image, memory, domain, cloudflare_record_id,
    container_id, compose_path, state, whitelist, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		srv.ID, srv.Name, srv.Provider, srv.ModpackSlug, srv.ModpackName, srv.FileID,
		srv.GameVersion, srv.JavaImage, srv.Memory, srv.Domain, srv.CloudflareRecordID,
		srv.ContainerID, srv.ComposePath, srv.State, joinWhitelist(srv.Whitelist),
		now, now)
	return err
}

// GetServer retrieves a server by ID.
func (s *Store) GetServer(ctx context.Context, id string) (*model.Server, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
    id, name, provider, modpack_slug, modpack_name, file_id,
    game_version, java_image, memory, domain, cloudflare_record_id,
    container_id, compose_path, state, whitelist, created_at, updated_at
    FROM servers WHERE id = ?`, id)
	return scanServer(row)
}

// GetServerByName retrieves a server by its name (unique).
func (s *Store) GetServerByName(ctx context.Context, name string) (*model.Server, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
    id, name, provider, modpack_slug, modpack_name, file_id,
    game_version, java_image, memory, domain, cloudflare_record_id,
    container_id, compose_path, state, whitelist, created_at, updated_at
    FROM servers WHERE name = ?`, name)
	return scanServer(row)
}

// ListServers returns all servers ordered by creation time descending.
func (s *Store) ListServers(ctx context.Context) ([]*model.Server, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
    id, name, provider, modpack_slug, modpack_name, file_id,
    game_version, java_image, memory, domain, cloudflare_record_id,
    container_id, compose_path, state, whitelist, created_at, updated_at
    FROM servers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Server
	for rows.Next() {
		srv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

// UpdateServer updates mutable fields of an existing server.
func (s *Store) UpdateServer(ctx context.Context, srv *model.Server) error {
	srv.UpdatedAt = time.Now()
	_, err := s.db.ExecContext(ctx, `
UPDATE servers SET
    state = ?, container_id = ?, cloudflare_record_id = ?,
    compose_path = ?, game_version = ?, java_image = ?, memory = ?,
    domain = ?, whitelist = ?, updated_at = ?
WHERE id = ?`,
		srv.State, srv.ContainerID, srv.CloudflareRecordID,
		srv.ComposePath, srv.GameVersion, srv.JavaImage, srv.Memory,
		srv.Domain, joinWhitelist(srv.Whitelist),
		srv.UpdatedAt.Format(time.RFC3339), srv.ID)
	return err
}

// DeleteServer removes a server row.
func (s *Store) DeleteServer(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id)
	return err
}

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func scanServer(sc scanner) (*model.Server, error) {
	var (
		srv          model.Server
		whitelistRaw string
		createdRaw   string
		updatedRaw   string
	)
	err := sc.Scan(
		&srv.ID, &srv.Name, &srv.Provider, &srv.ModpackSlug, &srv.ModpackName,
		&srv.FileID, &srv.GameVersion, &srv.JavaImage, &srv.Memory, &srv.Domain,
		&srv.CloudflareRecordID, &srv.ContainerID, &srv.ComposePath, &srv.State,
		&whitelistRaw, &createdRaw, &updatedRaw,
	)
	if err != nil {
		return nil, err
	}
	srv.Whitelist = splitWhitelist(whitelistRaw)
	srv.CreatedAt, _ = time.Parse(time.RFC3339, createdRaw)
	srv.UpdatedAt, _ = time.Parse(time.RFC3339, updatedRaw)
	return &srv, nil
}

func joinWhitelist(w []string) string {
	return strings.Join(w, ",")
}

func splitWhitelist(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
