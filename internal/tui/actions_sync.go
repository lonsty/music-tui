package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/store"
)

// ── Library sync ──────────────────────────────────────────────────────────────

// cmdSyncLibrary runs an incremental sync of a.musicDir against the database,
// then reloads the full track list and sends a scanDoneMsg.
// Any sync error is surfaced in the status bar via scanDoneMsg.err.
func (a *App) cmdSyncLibrary() tea.Cmd {
	if a.st == nil || a.musicDir == "" {
		return nil
	}
	dir := a.musicDir
	st := a.st

	// Cancel any previous scan before starting a new one.
	if a.scanCancel != nil {
		a.scanCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.scanCancel = cancel

	return func() tea.Msg {
		defer cancel()
		coverDir, _ := store.CoverCacheDir()
		_, _, _, syncErr := store.SyncDir(ctx, dir, st, coverDir, nil)
		tracks, dbErr := st.AllTracks()
		if dbErr != nil {
			return scanDoneMsg{err: dbErr}
		}
		// Return tracks even when syncErr is non-nil: we show whatever is in
		// the DB and surface the sync error in the status bar.
		if len(tracks) == 0 {
			tracks = []library.Track{} // always non-nil slice
		}
		return scanDoneMsg{tracks: tracks, err: syncErr}
	}
}
