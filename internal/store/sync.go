package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/lonsty/music-tui/internal/library"
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
//
// Parse work is distributed across a worker pool (min(NumCPU, 8) goroutines)
// to exploit parallelism in tag reading and audio decoding.  DB writes remain
// on the caller goroutine (single-threaded) to avoid SQLite lock contention.
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

	// ── Batch-fetch all stored mtimes in one query ────────────────────────
	// This avoids N individual TrackMtime queries (one per file) — a single
	// SELECT returns the entire map in O(1) round-trips.
	storedMtimes, err := s.AllTrackMtimes()
	if err != nil {
		// Non-fatal: fall back to treating every file as new.
		if firstErr == nil {
			firstErr = err
		}
		storedMtimes = make(map[string]int64)
	}

	// ── Determine which files need (re-)parsing ───────────────────────────
	type candidate struct {
		path  string
		mtime int64
		isNew bool
	}
	existingPaths := make(map[string]struct{}, total)
	var candidates []candidate

	for _, path := range paths {
		existingPaths[path] = struct{}{}

		info, statErr := os.Stat(path)
		if statErr != nil {
			if firstErr == nil {
				firstErr = statErr
			}
			continue
		}
		mtime := info.ModTime().Unix()
		storedMtime, exists := storedMtimes[path]

		if exists && storedMtime == mtime {
			continue // up-to-date — skip
		}
		candidates = append(candidates, candidate{
			path:  path,
			mtime: mtime,
			isNew: !exists,
		})
	}

	// ── Worker pool: parse files concurrently ─────────────────────────────
	// DB writes happen on the main goroutine (serialised) so that SQLite's
	// single-writer model is never stressed.
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	type result struct {
		track library.Track
		mtime int64
		isNew bool
		err   error
	}

	workCh := make(chan candidate, numWorkers*2)
	resultCh := make(chan result, numWorkers*2)

	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range workCh {
				t, parseErr := library.ParseTrackWithCover(c.path, coverCacheDir)
				select {
				case resultCh <- result{
					track: t,
					mtime: c.mtime,
					isNew: c.isNew,
					err:   parseErr,
				}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Close resultCh once all workers finish.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Feed candidates to workers (respects context cancellation).
	go func() {
		defer close(workCh)
		for _, c := range candidates {
			select {
			case workCh <- c:
			case <-ctx.Done():
				return
			}
		}
	}()

	// ── Main goroutine: drain results and write to DB ─────────────────────
	// Writes are batched into one transaction with a prepared statement and
	// committed every syncCommitEvery rows.  This keeps the first scan of a
	// large library from paying one fsync per file while bounding WAL growth.
	const syncCommitEvery = 1000

	var (
		tx          *sql.Tx
		stmt        *sql.Stmt
		pending     int
		batchFailed error
	)
	startBatch := func() {
		t, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			batchFailed = err
			return
		}
		st, err := t.PrepareContext(ctx, upsertTrackSQL)
		if err != nil {
			_ = t.Rollback()
			batchFailed = err
			return
		}
		tx, stmt = t, st
	}
	flushBatch := func() {
		if stmt != nil {
			_ = stmt.Close()
		}
		if tx != nil {
			if err := tx.Commit(); err != nil && batchFailed == nil {
				batchFailed = err
			}
		}
		tx, stmt, pending = nil, nil, 0
	}

	startBatch()
	done := total - len(candidates) // files skipped (already up-to-date)
	for r := range resultCh {
		done++
		if r.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("parse %q: %w", r.track.Path, r.err)
			}
			if onProgress != nil {
				onProgress(done, total)
			}
			continue
		}

		if batchFailed == nil && stmt != nil && ctx.Err() == nil {
			if _, dbErr := stmt.ExecContext(ctx, upsertTrackArgs(r.track, r.mtime)...); dbErr != nil {
				if firstErr == nil {
					firstErr = dbErr
				}
			} else if r.isNew {
				added++
			} else {
				updated++
			}
			pending++
			if pending >= syncCommitEvery {
				flushBatch()
				if batchFailed == nil {
					startBatch()
				}
			}
		}

		if onProgress != nil {
			onProgress(done, total)
		}
	}
	flushBatch()
	if batchFailed != nil && firstErr == nil {
		firstErr = batchFailed
	}

	// Report skipped files' progress after workers are done (for accurate bar).
	// done already counts them; emit final value if needed.
	if onProgress != nil && done < total {
		onProgress(total, total)
	}

	// ── Prune deleted files ───────────────────────────────────────────────
	if ctx.Err() == nil {
		n, pruneErr := s.PruneMissing(existingPaths)
		if pruneErr != nil && firstErr == nil {
			firstErr = pruneErr
		}
		deleted = n
	}

	return added, updated, deleted, firstErr
}
