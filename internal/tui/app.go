// Package tui implements the Bubble Tea application for music-tui.
package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/audio"
	"github.com/lonsty/music-tui/internal/store"
)

// App is the root Bubble Tea model.
type App struct {
	player   *audio.Player
	st       *store.Store // persistent database (may be nil for tests)
	musicDir string       // current music library root

	// session holds the state loaded from the DB at startup; it is consumed
	// by cmdRestoreSession on the first Init tick and then zeroed out.
	session *SessionState

	// Embed functional sub-models for clean grouping.
	// All fields remain accessible directly on *App (e.g. a.volume, a.cursor).
	PlaybackState
	LibraryState
	ChipState
	LyricsState
	PlaylistState
	OverlayState
	QueueState

	// ── Search ───────────────────────────────────────────────────────────────
	searchInput textinput.Model

	// ── Settings overlay ─────────────────────────────────────────────────────
	settingsInput  textinput.Model // p2chip options
	musicDirInput  textinput.Model // music library directory
	settingsActive settingsField   // active field in the settings overlay

	// ── Progress bar ─────────────────────────────────────────────────────────
	progressBar progress.Model

	// framePlayback is refreshed once at the start of every render and read by
	// render helpers, so a single frame takes one player lock snapshot instead
	// of one per call site.
	framePlayback playbackFrame

	// ── Lyrics viewport (fullscreen) ─────────────────────────────────────────
	lyricsVP viewport.Model

	// ── UI state ─────────────────────────────────────────────────────────────
	W, H            int
	currentView     view
	activeTab       tabID
	activeOvl       overlay
	ovlScrollRow    int            // scroll offset (in rows) for the active overlay
	settingsEditing bool           // true = text input active in settings overlay (editing mode)
	rightMode       rightPanelMode // player or lyrics content in the right panel
	rightCollapsed  bool           // true = right panel hidden, track list takes full width

	// ── Status / feedback ────────────────────────────────────────────────────
	statusMsg         string
	statusMsgExpiry   time.Time // when non-zero, statusMsg is cleared after this time
	libScanFromReload bool      // true when the current scan was triggered by Ctrl+R (not startup)
	quitArmedAt       time.Time // first q press; a second q within the window quits

	// ── Loading ──────────────────────────────────────────────────────────────
	loading bool
	scanErr error

	// ── Network request cancellation ─────────────────────────────────────────
	// netCancel cancels the most recent in-flight network request (lyrics fetch,
	// online search).  Call it before starting a new request so that stale
	// responses from the old request are never applied to the new track's state.
	netCancel context.CancelFunc

	// scanCancel cancels the in-flight library scan, if any.  A scan is a
	// single background job; starting a new one cancels the previous one.
	scanCancel context.CancelFunc

	// ── Enter-twice-to-fullscreen ────────────────────────────────────────────
	// Tracks the ID of the track that was selected last time Enter was pressed.
	// A second Enter on the same track opens the fullscreen player.
	lastEnterID string

	// ── Marquee scrollers ─────────────────────────────────────────────────────
	// One Marquee per text field that may need to scroll.
	mqTitle  *Marquee // player title
	mqMeta   *Marquee // "artist · album" line
	mqArtist *Marquee // artist-only (fullscreen)
	mqAlbum  *Marquee // album-only  (fullscreen)
	mqRow    *Marquee // selected track-list row (Local tab + Playlist track level)
	mqPlRow  *Marquee // selected playlist-list row (Playlist list level)

	// ── Cover art cache ───────────────────────────────────────────────────────
	// Pre-rendered terminal art string for the current track's cover.
	// Invalidated whenever the track changes or the box size changes.
	coverRendered    string
	coverW           int  // outerCols value used to produce coverRendered
	coverPlaceholder bool // true when coverRendered is the no-art placeholder

	// ── Favorites / Like ─────────────────────────────────────────────────────
	// isFavorite caches whether the currently playing track is in the
	// Favorites playlist.  Updated on track change and after Like/Unlike.
	isFavorite bool
}

// Cleanup saves the current session state to the database and removes
// the temporary directory.  Call this when the application exits.
func (a *App) Cleanup() {
	if a.scanCancel != nil {
		a.scanCancel()
	}
	a.saveSession()
	if a.tmpDir != "" {
		_ = os.RemoveAll(a.tmpDir)
	}
}

// saveSession persists all user-facing state to the settings table so it can
// be restored on the next launch.
//
// It must be called while the player is still active (before player.Stop),
// so that Position() returns the correct playback offset.  Cleanup() also
// calls it as a fallback for abnormal exits, but if the player has already
// been stopped the position will be 0.
func (a *App) saveSession() {
	if a.st == nil {
		return
	}
	pos := a.player.Position()
	state := a.player.State()
	wasPlaying := store.ValWasPlayingNo
	if state == audio.StatePlaying || state == audio.StatePaused {
		wasPlaying = store.ValWasPlayingYes
	}

	lastTrackID := ""
	if a.currentTrack != nil {
		lastTrackID = a.currentTrack.ID
	}

	// Only write last_position_ms when the player is alive; skip the write
	// when pos==0 and the player is stopped (would overwrite a valid position
	// saved moments earlier by the quit handler).
	posMs := pos.Milliseconds()
	collapsedVal := store.ValRightExpanded
	if a.rightCollapsed {
		collapsedVal = store.ValRightCollapsed
	}
	pairs := map[string]string{
		store.KeyVolume:         strconv.FormatFloat(a.volume, 'f', 4, 64),
		store.KeyPlayMode:       strconv.Itoa(int(a.playMode)),
		store.KeyRetroIdx:       strconv.Itoa(a.retroIdx),
		store.KeyLastTrackID:    lastTrackID,
		store.KeyWasPlaying:     wasPlaying,
		store.KeyChip8Options:   a.chip8Options,
		store.KeyRightCollapsed: collapsedVal,
		store.KeyRightPanelMode: rightPanelModeKey(a.rightMode),
		store.KeyIconSet:        iconSetKey(ActiveIconSet()),
		store.KeyLastActiveTab:  strconv.Itoa(int(a.activeTab)),
	}
	// Save playlist depth if currently in Playlists tab.
	if a.activeTab == tabPlaylist {
		if a.depth == playlistDepthTracks {
			pairs[store.KeyPlaylistDepth] = "tracks"
			pairs[store.KeyPlaylistCursor] = strconv.Itoa(a.listCursor)
			pairs[store.KeyPlaylistTrackCursor] = strconv.Itoa(a.trackCursor)
			pl := a.currentPlaylist()
			pairs[store.KeyPlaylistID] = pl.ID
		} else {
			pairs[store.KeyPlaylistDepth] = "list"
			pairs[store.KeyPlaylistCursor] = strconv.Itoa(a.listCursor)
		}
	}
	if posMs > 0 || state != audio.StateStopped {
		pairs[store.KeyLastPositionMs] = strconv.FormatInt(posMs, 10)
	}
	if err := a.st.SetSettings(pairs); err != nil {
		// Cleanup runs after the TUI has stopped, so there is no status bar
		// left to notify; the log is the only visible trace.
		log.Printf("persist session: %v", err)
	}
}

// saveSetting persists one setting and reports failures instead of silently
// dropping them.  Call it from Update handlers.
func (a *App) saveSetting(key, value string) {
	if a.st == nil {
		return
	}
	a.reportPersistErr(a.st.SetSetting(key, value))
}

// reportPersistErr logs a failed settings write and surfaces it in the status
// bar.  A nil error is ignored.
func (a *App) reportPersistErr(err error) {
	if err == nil {
		return
	}
	log.Printf("persist settings: %v", err)
	a.statusMsg = fmt.Sprintf(T("settings_save_error"), err)
	a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
}

// WithProgram wires the audio "done" callback to send a trackDoneMsg.
// Must be called after tea.NewProgram is created.
func (a *App) WithProgram(p *tea.Program) {
	a.player.SetOnDone(func() {
		p.Send(trackDoneMsg{})
	})
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{tick()}
	if a.session != nil && a.session.LastTrackID != "" {
		cmds = append(cmds, a.cmdRestoreSession())
	}
	// Sync the library on every launch so files added while the app was
	// closed show up without requiring a manual reload.
	if a.st != nil && a.musicDir != "" {
		a.loading = true
		cmds = append(cmds, a.cmdSyncLibrary())
	}
	// Preload playlists when the initial tab is Playlists.
	if a.activeTab == tabPlaylist && a.st != nil {
		cmds = append(cmds, a.cmdLoadPlaylists())
	}
	return tea.Batch(cmds...)
}

// View implements tea.Model.
func (a *App) View() string {
	if a.W == 0 {
		return "Loading…"
	}
	return a.render()
}
