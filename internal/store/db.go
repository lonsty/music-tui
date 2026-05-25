// Package store manages the persistent SQLite database for music-tui.
//
// The database is stored in the XDG data directory:
//
//	~/.local/share/music-tui/music-tui.db
//
// It holds two tables:
//   - settings  — key/value application configuration
//   - tracks    — the scanned music library
package store

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver

	"github.com/eilianxiao/music-tui/internal/library"
)

// Store wraps a SQLite connection and provides typed access to the database.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at dbPath.
// It creates all required tables and indexes if they do not already exist,
// and enables WAL mode for better concurrent read performance.
func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Single connection to avoid locking issues.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate applies any pending schema migrations in order.
//
// Each migration is identified by a monotonically increasing version number.
// Applied versions are recorded in the schema_migrations table so that each
// migration runs exactly once.  New tables or ALTER TABLE statements should
// be added as a new entry at the end of the migrations slice.
func migrate(db *sql.DB) error {
	// Bootstrap the migrations tracker table first (idempotent).
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// ordered list of all migrations; append new entries here.
	migrations := []struct {
		version int
		sql     string
	}{
		{1, `
			CREATE TABLE IF NOT EXISTS settings (
				key   TEXT PRIMARY KEY,
				value TEXT NOT NULL
			);

			CREATE TABLE IF NOT EXISTS tracks (
				id            TEXT PRIMARY KEY,
				path          TEXT NOT NULL UNIQUE,
				title         TEXT NOT NULL DEFAULT '',
				artist        TEXT NOT NULL DEFAULT '',
				album_artist  TEXT NOT NULL DEFAULT '',
				album         TEXT NOT NULL DEFAULT '',
				year          TEXT NOT NULL DEFAULT '',
				track_number  TEXT NOT NULL DEFAULT '',
				genre         TEXT NOT NULL DEFAULT '',
				comment       TEXT NOT NULL DEFAULT '',
				duration_ms   INTEGER NOT NULL DEFAULT 0,
				cover_path    TEXT NOT NULL DEFAULT '',
				source        INTEGER NOT NULL DEFAULT 0,
				mtime         INTEGER NOT NULL DEFAULT 0,
				added_at      INTEGER NOT NULL DEFAULT 0
			);

			CREATE INDEX IF NOT EXISTS idx_tracks_sort
				ON tracks(album_artist, year, album, track_number);
		`},
		// version 2: playlists tables (歌单管理)
		{2, `
			CREATE TABLE IF NOT EXISTS playlists (
				id         TEXT PRIMARY KEY,
				name       TEXT NOT NULL,
				created_at INTEGER NOT NULL DEFAULT 0
			);

			CREATE TABLE IF NOT EXISTS playlist_tracks (
				playlist_id TEXT    NOT NULL,
				track_id    TEXT    NOT NULL,
				position    INTEGER NOT NULL DEFAULT 0,
				PRIMARY KEY (playlist_id, track_id)
			);
		`},
		// version 3: add provider_id column to tracks for multi-source support
		{3, `
			ALTER TABLE tracks ADD COLUMN provider_id TEXT NOT NULL DEFAULT 'local';
		`},
	}

	for _, m := range migrations {
		var applied int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, m.version,
		).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.version, err)
		}
		if applied > 0 {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.version, err)
		}
		if _, err := tx.Exec(m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", m.version, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_migrations(version) VALUES(?)`, m.version,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.version, err)
		}
	}
	return nil
}

// ── Settings ──────────────────────────────────────────────────────────────────

// GetSetting returns the value for key, or ("", nil) when the key is absent.
func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM settings WHERE key = ?`, key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting inserts or replaces a setting value.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings(key, value) VALUES(?,?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	return err
}

// SetSettings writes multiple key/value pairs in a single transaction.
func (s *Store) SetSettings(pairs map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		`INSERT INTO settings(key, value) VALUES(?,?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
	)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer func() { _ = stmt.Close() }()
	for k, v := range pairs {
		if _, err := stmt.Exec(k, v); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ── Tracks ────────────────────────────────────────────────────────────────────

// AllTracks returns all tracks from the database in the standard sort order:
// album artist → year → album → track number → title.
func (s *Store) AllTracks() ([]library.Track, error) {
	rows, err := s.db.Query(`
		SELECT id, path, title, artist, album_artist, album,
		       year, track_number, genre, comment,
		       duration_ms, cover_path, source
		FROM tracks
		ORDER BY
			LOWER(COALESCE(NULLIF(album_artist,''), artist, '')),
			year,
			LOWER(album),
			CAST(SUBSTR(track_number, 1, INSTR(track_number||'/', '/')-1) AS INTEGER),
			LOWER(title)
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tracks []library.Track
	for rows.Next() {
		var t library.Track
		var durationMs int64
		var source int
		if err := rows.Scan(
			&t.ID, &t.Path, &t.Title, &t.Artist, &t.AlbumArtist, &t.Album,
			&t.Year, &t.TrackNumber, &t.Genre, &t.Comment,
			&durationMs, &t.CoverPath, &source,
		); err != nil {
			return nil, err
		}
		t.Duration = time.Duration(durationMs) * time.Millisecond
		t.Source = library.Source(source)
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// TrackMtime returns the stored mtime (unix seconds) for path, or 0 if absent.
func (s *Store) TrackMtime(path string) (int64, error) {
	var mtime int64
	err := s.db.QueryRow(
		`SELECT mtime FROM tracks WHERE path = ?`, path,
	).Scan(&mtime)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return mtime, err
}

// UpsertTrack inserts or replaces a track record.
func (s *Store) UpsertTrack(t library.Track, mtime int64) error {
	_, err := s.db.Exec(`
		INSERT INTO tracks
			(id, path, title, artist, album_artist, album,
			 year, track_number, genre, comment,
			 duration_ms, cover_path, source, mtime, added_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET
			id           = excluded.id,
			title        = excluded.title,
			artist       = excluded.artist,
			album_artist = excluded.album_artist,
			album        = excluded.album,
			year         = excluded.year,
			track_number = excluded.track_number,
			genre        = excluded.genre,
			comment      = excluded.comment,
			duration_ms  = excluded.duration_ms,
			cover_path   = excluded.cover_path,
			source       = excluded.source,
			mtime        = excluded.mtime
	`,
		t.ID, t.Path, t.Title, t.Artist, t.AlbumArtist, t.Album,
		t.Year, t.TrackNumber, t.Genre, t.Comment,
		t.Duration.Milliseconds(), t.CoverPath,
		int(t.Source), mtime, time.Now().Unix(),
	)
	return err
}

// DeleteTrack removes a track by its file path.
func (s *Store) DeleteTrack(path string) error {
	_, err := s.db.Exec(`DELETE FROM tracks WHERE path = ?`, path)
	return err
}

// PruneMissing removes all tracks whose paths are NOT in existingPaths.
// Returns the number of rows deleted.
func (s *Store) PruneMissing(existingPaths map[string]struct{}) (int, error) {
	rows, err := s.db.Query(`SELECT path FROM tracks`)
	if err != nil {
		return 0, err
	}
	var toDelete []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if _, ok := existingPaths[p]; !ok {
			toDelete = append(toDelete, p)
		}
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(toDelete) == 0 {
		return 0, nil
	}

	// Batch DELETE in groups of batchSize to avoid hitting SQLite's
	// variable-binding limit (SQLITE_LIMIT_VARIABLE_NUMBER = 999 by default).
	const batchSize = 500
	deleted := 0
	for i := 0; i < len(toDelete); i += batchSize {
		end := i + batchSize
		if end > len(toDelete) {
			end = len(toDelete)
		}
		batch := toDelete[i:end]

		placeholders := strings.Repeat("?,", len(batch))
		placeholders = placeholders[:len(placeholders)-1] // trim trailing comma
		args := make([]any, len(batch))
		for j, p := range batch {
			args[j] = p
		}
		res, err := s.db.Exec(
			"DELETE FROM tracks WHERE path IN ("+placeholders+")",
			args...,
		)
		if err != nil {
			return deleted, err
		}
		n, _ := res.RowsAffected()
		deleted += int(n)
	}
	return deleted, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// TrackID derives a stable 16-hex-char ID from a file path.
func TrackID(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h[:8])
}

// DataDir returns the XDG-compliant data directory for music-tui,
// creating it if necessary.
//
//	~/.local/share/music-tui/
func DataDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(base, "music-tui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// CoverCacheDir returns the directory used for cached cover-art files,
// creating it if necessary.
//
//	~/.cache/music-tui/covers/
func CoverCacheDir() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(base, "music-tui", "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
