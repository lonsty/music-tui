package store

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/eilianxiao/music-tui/internal/library"
)

// SyncDir performs an incremental sync between the filesystem and the database.
//
// For every supported audio file found under dir:
//   - If the path is not in the DB → parse metadata and insert.
//   - If the path is in the DB but mtime has changed → re-parse and update.
//   - If the path is already up-to-date → skip.
//
// After scanning, any DB records whose paths no longer exist on disk are removed.
//
// onProgress is called after each file is processed with (filesProcessed, totalFiles).
// It may be nil.
//
// Returns the number of tracks added, updated, deleted and the first non-fatal
// error encountered (scanning continues even after errors).
func SyncDir(
	ctx context.Context,
	dir string,
	s *Store,
	coverCacheDir string,
	onProgress func(done, total int),
) (added, updated, deleted int, firstErr error) {
	// ── Collect all audio files ───────────────────────────────────────────
	var paths []string
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Record the first walk error but continue scanning other entries.
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if library.IsSupportedAudio(path) {
			paths = append(paths, path)
		}
		return nil
	})
	if walkErr != nil && firstErr == nil {
		firstErr = walkErr
	}
	total := len(paths)

	// ── Process each file ─────────────────────────────────────────────────
	existingPaths := make(map[string]struct{}, total)
	for i, path := range paths {
		if ctx.Err() != nil {
			break
		}
		existingPaths[path] = struct{}{}

		info, err := os.Stat(path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if onProgress != nil {
				onProgress(i+1, total)
			}
			continue
		}
		mtime := info.ModTime().Unix()

		storedMtime, err := s.TrackMtime(path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if onProgress != nil {
				onProgress(i+1, total)
			}
			continue
		}

		if storedMtime == mtime {
			// Up-to-date — skip parsing.
			if onProgress != nil {
				onProgress(i+1, total)
			}
			continue
		}

		// New or changed — (re-)parse.
		track, err := library.ParseTrackWithCover(path, coverCacheDir)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("parse %q: %w", path, err)
			}
			if onProgress != nil {
				onProgress(i+1, total)
			}
			continue
		}

		isNew := storedMtime == 0
		if err := s.UpsertTrack(track, mtime); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else if isNew {
			added++
		} else {
			updated++
		}

		if onProgress != nil {
			onProgress(i+1, total)
		}
	}

	// ── Prune deleted files ───────────────────────────────────────────────
	if ctx.Err() == nil {
		n, err := s.PruneMissing(existingPaths)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		deleted = n
	}

	return added, updated, deleted, firstErr
}

// isSupportedAudio is provided by library.IsSupportedAudio (library/formats.go).
// This file previously had a duplicate; it has been removed.
