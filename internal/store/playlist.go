package store

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"github.com/lonsty/music-tui/internal/library"
)

// Playlist represents a named, ordered collection of tracks.
type Playlist struct {
	// ID is a stable, opaque identifier derived from the playlist name and
	// creation time.  It is stored in the playlists.id column.
	ID string
	// Name is the human-readable playlist name.
	Name string
	// CreatedAt is when the playlist was created.
	CreatedAt time.Time
}

// ── Playlist CRUD ──────────────────────────────────────────────────────────────

// CreatePlaylist creates a new playlist with the given name and returns it.
// The ID is derived from the name and creation timestamp so it is stable for
// a given (name, time) pair but does not require an external UUID library.
func (s *Store) CreatePlaylist(name string) (Playlist, error) {
	now := time.Now()
	key := fmt.Sprintf("%s\x00%d", name, now.UnixNano())
	h := sha256.Sum256([]byte(key))
	id := fmt.Sprintf("%x", h[:8]) // 16-char hex

	_, err := s.db.Exec(
		`INSERT INTO playlists(id, name, created_at) VALUES (?, ?, ?)`,
		id, name, now.Unix(),
	)
	if err != nil {
		return Playlist{}, fmt.Errorf("create playlist: %w", err)
	}
	return Playlist{ID: id, Name: name, CreatedAt: now}, nil
}

// GetPlaylists returns all playlists ordered by creation time (oldest first).
func (s *Store) GetPlaylists() ([]Playlist, error) {
	rows, err := s.db.Query(
		`SELECT id, name, created_at FROM playlists ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("get playlists: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pls []Playlist
	for rows.Next() {
		var p Playlist
		var ts int64
		if err := rows.Scan(&p.ID, &p.Name, &ts); err != nil {
			return nil, fmt.Errorf("scan playlist row: %w", err)
		}
		p.CreatedAt = time.Unix(ts, 0)
		pls = append(pls, p)
	}
	return pls, rows.Err()
}

// DeletePlaylist removes a playlist and all of its track associations.
func (s *Store) DeletePlaylist(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("delete playlist begin: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM playlist_tracks WHERE playlist_id = ?`, id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete playlist tracks: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM playlists WHERE id = ?`, id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete playlist: %w", err)
	}
	return tx.Commit()
}

// RenamePlaylist updates the name of an existing playlist.
func (s *Store) RenamePlaylist(id, name string) error {
	res, err := s.db.Exec(`UPDATE playlists SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("rename playlist: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rename playlist: id %q not found", id)
	}
	return nil
}

// ── Playlist track associations ────────────────────────────────────────────────

// AddTrackToPlaylist inserts track trackID into playlist playlistID at the
// given position.  If the track is already in the playlist the call is a
// no-op (INSERT OR IGNORE).
func (s *Store) AddTrackToPlaylist(playlistID, trackID string, position int) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO playlist_tracks(playlist_id, track_id, position)
		 VALUES (?, ?, ?)`,
		playlistID, trackID, position,
	)
	if err != nil {
		return fmt.Errorf("add track to playlist: %w", err)
	}
	return nil
}

// RemoveTrackFromPlaylist removes a single track from a playlist.
func (s *Store) RemoveTrackFromPlaylist(playlistID, trackID string) error {
	_, err := s.db.Exec(
		`DELETE FROM playlist_tracks WHERE playlist_id = ? AND track_id = ?`,
		playlistID, trackID,
	)
	if err != nil {
		return fmt.Errorf("remove track from playlist: %w", err)
	}
	return nil
}

// GetPlaylistTracks returns all tracks in a playlist ordered by their position.
// Tracks that have been removed from the library (orphaned references) are
// silently skipped via the INNER JOIN.
func (s *Store) GetPlaylistTracks(playlistID string) ([]library.Track, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.path, t.title, t.artist, t.album_artist, t.album,
		       t.year, t.track_number, t.genre, t.comment,
		       t.duration_ms, t.cover_path, t.format
		FROM playlist_tracks pt
		INNER JOIN tracks t ON t.id = pt.track_id
		WHERE pt.playlist_id = ?
		ORDER BY pt.position ASC
	`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("get playlist tracks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tracks []library.Track
	for rows.Next() {
		var t library.Track
		var durationMs int64
		if err := rows.Scan(
			&t.ID, &t.Path, &t.Title, &t.Artist, &t.AlbumArtist, &t.Album,
			&t.Year, &t.TrackNumber, &t.Genre, &t.Comment,
			&durationMs, &t.CoverPath, &t.FileFormat,
		); err != nil {
			return nil, fmt.Errorf("scan playlist track row: %w", err)
		}
		t.Duration = time.Duration(durationMs) * time.Millisecond
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// PlaylistTrackCount returns the number of tracks in a playlist.
// Returns 0 for an unknown playlist ID.
func (s *Store) PlaylistTrackCount(playlistID string) (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = ?`, playlistID,
	).Scan(&n)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("playlist track count: %w", err)
	}
	return n, nil
}
