// Package tui implements the Bubble Tea application for music-tui.
package tui

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/audio"
	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/lyrics"
	"github.com/lonsty/music-tui/internal/lyrics/online"
	"github.com/lonsty/music-tui/internal/provider"
	"github.com/lonsty/music-tui/internal/store"
)

// SessionState holds the persisted state that is restored on the next launch.
type SessionState struct {
	LastTrackID    string // preferred: stable ID (since v0.4)
	LastPositionMs int64
	WasPlaying     bool // if true, load the track but start paused
	Volume         float64
	PlayMode       int
	RetroIdx       int
	Chip8Options   string
}

// PlaybackState holds all current playback-related state.
// It is embedded in App and accessed via a.volume, a.playMode, etc.
type PlaybackState struct {
	currentTrack *library.Track
	// currentIdx was removed.  The position of currentTrack in the filtered
	// list is computed on demand via filteredIdx(currentTrack.ID) when needed
	// (e.g. cmdPlayNext), which is O(N) but rare and correct after filter
	// changes.  Storing a cached index caused subtle bugs when the filtered
	// list was rebuilt without updating the index.
	volume   float64  // [0.0, maxVolume]; 1.0 = unity gain
	playMode playMode // sequential / loop / single / random
	retroIdx int      // retro effect preset index (0 = off)
}

// LibraryState holds the local track library and filter/cursor state.
// It is embedded in App and accessed via a.tracks, a.filteredIdxs, etc.
//
// Design: a.tracks is the single source of truth for the track list.
// a.filteredIdxs is a lightweight index table — a slice of positions into
// a.tracks — rebuilt whenever the filter changes.  All UI code accesses
// tracks via filtered() / filteredTrack(pos), which are zero-copy reads
// of a.tracks[a.filteredIdxs[pos]].
//
// a.cursorPos is the position in a.filteredIdxs (not in a.tracks).  It is
// O(1) to move and O(1) to read the corresponding track.  The cursor is
// clamped to [0, len(filteredIdxs)-1] after every filter rebuild.
//
// a.shuffleIDs stores Track.ID values in shuffled order.  Unlike the old
// []int approach, IDs remain valid after filter changes — the next-track
// logic skips IDs that are not currently in filteredIdxs.
type LibraryState struct {
	tracks       []library.Track
	filteredIdxs []int            // positions in a.tracks that pass the current filter
	shuffleIDs   []string         // Track.ID values in shuffled order (random play mode)
	cursorPos    int              // position in filteredIdxs of the cursor-selected track
	formatPref   formatPreference // active format-display preference
	lastQuery    string           // last applied search query; used to detect query changes

	// Grouped browsing.  The group caches are rebuilt whenever the filter or
	// format preference changes; render and key handling only read them.
	view         libraryView
	prevView     libraryView // view to restore when a search filter is cleared
	groupCursor  int
	albumGroups  []trackGroup
	artistGroups []trackGroup
	drilldown    *groupRef // non-nil while showing one album/artist's tracks
}

// ChipState holds the 8-bit chip conversion state.
// It is embedded in App and accessed via a.chipMode, a.chipBusy, etc.
type ChipState struct {
	chipMode       bool   // currently playing the 8-bit converted version
	chipBusy       bool   // conversion or crossfade in progress (locked)
	chipConverting bool   // true only during p2chip conversion (not crossfade)
	chipPath       string // path to the cached 8-bit mp3 (in tmpDir)
	chipOrigin     string // Track.Path for which chipPath was generated
	tmpDir         string // temp directory; created on startup, removed on exit
	chip8Options   string // extra CLI options forwarded to p2chip
}

// PlaylistState holds playlist management state.
// It is embedded in App and accessed via a.playlists, a.listCursor, etc.
type PlaylistState struct {
	depth       playlistDepth     // current navigation level
	playlists   []store.Playlist  // cached list of all playlists (index 0 = Favorites)
	listCursor  int               // cursor in the playlist list (depth == list)
	plTracks    []library.Track   // tracks of the currently open playlist
	trackCursor int               // cursor in the track list (depth == tracks)
	inputMode   playlistInputMode // purpose of nameInput (none / create / rename)
	nameInput   textinput.Model   // inline input for new-playlist name or rename

	// addingTracks is true while the inline "add tracks" search panel is open.
	addingTracks bool
	addInput     textinput.Model // search box inside the add-tracks panel
	addResults   []int           // indices into a.tracks that match addInput
	addCursor    int             // cursor in addResults
	addDelta     map[string]int  // net change per track: +1 added, -1 removed, 0 cancelled

	// plListMaxColW caches the max display width of the right-column
	// track count across all playlists.  Recomputed in playlistsLoadedMsg
	// so the render loop never queries the DB.
	plListMaxColW int

	// ovlPlaylists is the cached playlist list used by overlayAddToPlaylist.
	// It is loaded when the overlay opens.
	ovlPlaylists []store.Playlist
	ovlPlCursor  int // cursor inside the overlay list

	// pendingTrackAnchor is set to true when entering a playlist's track
	// list and the cursor should be anchored to the currently playing track
	// once the tracks are loaded.  Cleared in the playlistTracksLoadedMsg
	// handler after the anchor is resolved.
	pendingTrackAnchor bool

	// restorePlaylistID is set on startup when the session was inside a
	// playlist's track list.  The playlistsLoadedMsg handler reads this
	// and triggers a track load if the playlist still exists.
	restorePlaylistID string
}

// QueueState holds the temporary play queue.  It is never persisted: quitting
// discards it.
type QueueState struct {
	ids    []string // Track.ID values in play order
	cursor int      // cursor inside the queue overlay
}

// LyricsState holds the lyrics for the currently playing track.
// It is embedded in App and accessed via a.lines, a.activeIdx, etc.
type LyricsState struct {
	lines         []lyrics.Line   // parsed LRC lines sorted by timestamp; nil = no lyrics
	activeIdx     int             // index of the currently highlighted line (-1 = none)
	prevActiveIdx int             // activeIdx from the previous tick, for change detection
	trackID       string          // Track.ID for which lines was loaded (stale-check)
	provider      lyrics.Provider // chain of local + online providers; set in NewApp
	lyricsLoading bool            // true while a background fetch is in-flight
	synced        bool            // true when at least one line has a non-zero timestamp

	// Manual browse state (fullscreen lyrics panel only).
	//
	// browseCenterIdx is the absolute index of the line pinned at the centre
	// of the visible window.  -1 means "follow playback" (centre = activeIdx).
	// Once set by ↑/↓ it does NOT move with activeIdx — the playing line
	// scrolls past independently while the cursor stays put.
	//
	// browseTicks counts tickInterval ticks since the last manual scroll.
	// When browseTicks reaches browseFadeOutTicks, browseExpired is set to
	// true.  The cursor is NOT immediately reset; instead, the next time
	// activeIdx advances to a new line, the cursor smoothly snaps back to
	// follow playback by clearing browseCenterIdx.
	browseCenterIdx int
	browseTicks     int
	browseExpired   bool
}

// App is the root Bubble Tea model.
type App struct {
	player   *audio.Player
	st       *store.Store // persistent database (may be nil for tests)
	musicDir string       // current music library root

	// session holds the state loaded from the DB at startup; it is consumed
	// by cmdRestoreSession on the first Init tick and then zeroed out.
	session *SessionState

	// providerMap maps a TrackProvider ID (e.g. "netease") to its implementation.
	// When playing a track, cmdPlayTrack looks up the track's ProviderID here to
	// obtain a StreamSource.  Tracks with ProviderID "" or "local" fall back to
	// LocalSource{Path: track.Path} without requiring a map entry.
	providerMap map[string]provider.TrackProvider

	// Embed functional sub-models for clean grouping.
	// All fields remain accessible directly on *App (e.g. a.volume, a.cursor).
	PlaybackState
	LibraryState
	ChipState
	LyricsState
	PlaylistState
	QueueState

	// ── Search ───────────────────────────────────────────────────────────────
	searchInput textinput.Model

	// ── Settings overlay ─────────────────────────────────────────────────────
	settingsInput  textinput.Model // p2chip options
	musicDirInput  textinput.Model // music library directory
	settingsActive settingsField   // active field in the settings overlay

	// ── Progress bar ─────────────────────────────────────────────────────────
	progressBar progress.Model

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

	// ── Loading ──────────────────────────────────────────────────────────────
	loading bool
	scanErr error

	// ── Network request cancellation ─────────────────────────────────────────
	// netCancel cancels the most recent in-flight network request (lyrics fetch,
	// online search).  Call it before starting a new request so that stale
	// responses from the old request are never applied to the new track's state.
	netCancel context.CancelFunc

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
	coverRendered string
	coverW        int // outerCols value used to produce coverRendered

	// ── Favorites / Like ─────────────────────────────────────────────────────
	// isFavorite caches whether the currently playing track is in the
	// Favorites playlist.  Updated on track change and after Like/Unlike.
	isFavorite bool
}

// NewApp creates the application model. Call WithProgram after tea.NewProgram.
//
// st may be nil (useful in tests). tracks is the pre-loaded library from the DB.
// sess carries the persisted state from the previous run (may be nil).
func NewApp(player *audio.Player, st *store.Store, musicDir string, tracks []library.Track, sess *SessionState) *App {
	chip8Opts := ""
	if sess != nil {
		chip8Opts = sess.Chip8Options
	}

	// Restore UI language from the database before any rendering occurs.
	if st != nil {
		if lang, _ := st.GetSetting(store.KeyLanguage); lang == store.ValLanguageZH {
			SetLang(LangZH)
		}
	}

	// Restore format preference from the database.
	var fmtPref formatPreference
	if st != nil {
		if raw, _ := st.GetSetting(store.KeyFormatPreference); raw != "" {
			fmtPref = parseFormatPref(raw)
		}
	}

	// Restore right-panel layout preferences.
	var rightCollapsed bool
	var rightMode rightPanelMode
	if st != nil {
		if v, _ := st.GetSetting(store.KeyRightCollapsed); v == store.ValRightCollapsed {
			rightCollapsed = true
		}
		if v, _ := st.GetSetting(store.KeyRightPanelMode); v != "" {
			rightMode = parseRightPanelMode(v)
		}
	}

	// Restore icon set preference.
	if st != nil {
		if v, _ := st.GetSetting(store.KeyIconSet); v != "" {
			setIconSet(parseIconSet(v))
		}
	}

	// Restore last active tab. Default to tabLocal on first run.
	initialTab := tabLocal
	if st != nil {
		if v, _ := st.GetSetting(store.KeyLastActiveTab); v != "" {
			if id, err := strconv.Atoi(v); err == nil && id >= 0 && id < int(tabCount) {
				initialTab = tabID(id)
			}
		}
	}

	// Restore playlist view depth if the last active tab was Playlists.
	var initialDepth playlistDepth
	var initialListCursor int
	var initialTrackCursor int
	var initialPlaylistID string
	if initialTab == tabPlaylist && st != nil {
		if v, _ := st.GetSetting(store.KeyPlaylistDepth); v == "tracks" {
			initialDepth = playlistDepthTracks
			if c, _ := st.GetSetting(store.KeyPlaylistCursor); c != "" {
				if n, err := strconv.Atoi(c); err == nil && n >= 0 {
					initialListCursor = n
				}
			}
			if c, _ := st.GetSetting(store.KeyPlaylistTrackCursor); c != "" {
				if n, err := strconv.Atoi(c); err == nil && n >= 0 {
					initialTrackCursor = n
				}
			}
			initialPlaylistID, _ = st.GetSetting(store.KeyPlaylistID)
		} else {
			initialDepth = playlistDepthList
			if c, _ := st.GetSetting(store.KeyPlaylistCursor); c != "" {
				if n, err := strconv.Atoi(c); err == nil && n >= 0 {
					initialListCursor = n
				}
			}
		}
	}

	ti := textinput.New()
	ti.Placeholder = T("search_placeholder")
	ti.CharLimit = 128

	si := textinput.New()
	si.Placeholder = "--sf2 nes --onset 0.5"
	si.CharLimit = 256
	si.Width = 42

	di := textinput.New()
	di.Placeholder = "/path/to/music"
	di.CharLimit = 512
	di.Width = 42

	// Playlist inline name-input (new / rename).
	ni := textinput.New()
	ni.Placeholder = T("playlist_new_prompt")
	ni.CharLimit = 64

	// Playlist "add tracks" search input.
	ai := textinput.New()
	ai.Placeholder = T("search_placeholder")
	ai.CharLimit = 128

	// Ensure the built-in Favorites playlist exists in the DB.
	if st != nil {
		_ = st.EnsureFavorites()
	}

	prog := progress.New(
		progress.WithGradient("#89B4FA", "#CBA6F7"),
		progress.WithoutPercentage(),
	)

	tmpDir, err := os.MkdirTemp("", "music-tui-*")
	if err != nil {
		// Fall back to the system temp directory; p2chip will still work.
		tmpDir = os.TempDir()
	}

	// Build the lyrics provider chain using the factory function so it can be
	// rebuilt later when the user changes provider settings.
	lyricsProvider := buildLyricsProvider()

	vol := 1.0
	mode := playModeLoop
	rIdx := 0
	if sess != nil {
		if sess.Volume > 0 {
			vol = sess.Volume
		}
		mode = playMode(sess.PlayMode)
		rIdx = sess.RetroIdx
	}

	app := &App{
		player:         player,
		st:             st,
		musicDir:       musicDir,
		activeTab:      initialTab,
		searchInput:    ti,
		settingsInput:  si,
		musicDirInput:  di,
		progressBar:    prog,
		loading:        false,
		mqTitle:        NewMarquee("", marqueeSep),
		mqMeta:         NewMarquee("", marqueeSep),
		mqArtist:       NewMarquee("", marqueeSep),
		mqAlbum:        NewMarquee("", marqueeSep),
		mqRow:          NewMarquee("", marqueeSep),
		mqPlRow:        NewMarquee("", marqueeSep),
		session:        sess,
		providerMap:    map[string]provider.TrackProvider{},
		netCancel:      func() {}, // no-op until a network request is started
		rightCollapsed: rightCollapsed,
		rightMode:      rightMode,
		PlaylistState: PlaylistState{
			depth:             initialDepth,
			listCursor:        initialListCursor,
			trackCursor:       initialTrackCursor,
			nameInput:         ni,
			addInput:          ai,
			restorePlaylistID: initialPlaylistID,
		},
		PlaybackState: PlaybackState{
			volume:   vol,
			playMode: mode,
			retroIdx: rIdx,
		},
		LibraryState: LibraryState{
			formatPref: fmtPref,
		},
		ChipState: ChipState{
			tmpDir:       tmpDir,
			chip8Options: chip8Opts,
		},
		LyricsState: LyricsState{
			activeIdx:       -1,
			prevActiveIdx:   -1,
			browseCenterIdx: -1,
			provider:        lyricsProvider,
		},
	}

	// Apply volume to player immediately.
	player.SetVolume(vol)
	// Apply retro preset.
	player.SetRetroPreset(rIdx)

	if len(tracks) > 0 {
		library.SortByArtistAlbum(tracks)
		app.tracks = tracks
		// Apply format preference and any existing search query so the initial
		// filtered list is consistent with the user's persisted settings.
		app.applyFilter()
		app.rebuildShuffleIDs()
		// DO NOT set statusMsg here — the "Loaded N tracks" message should
		// only appear after an explicit library reload (Ctrl+R → scanDoneMsg).
		// Setting it on every startup causes a visible status-bar flash/jump.
		// Pre-populate currentTrack so the first rendered frame already shows
		// the correct playing state (no flash/blank period while the async
		// restore Cmd is in flight).
		// Prefer ID lookup (stable since v0.4); fall back to path for legacy sessions.
		if sess != nil && sess.LastTrackID != "" {
			for _, t := range app.tracks {
				if t.ID != sess.LastTrackID {
					continue
				}
				tc := t
				app.currentTrack = &tc
				// Position the cursor on the current track.
				if pos := app.filteredPos(t.ID); pos >= 0 {
					app.cursorPos = pos
				}
				app.syncMarquees()
				app.syncRowMarquee()
				break
			}
		}
		// Always initialise mqRow for the cursor position so that row 0 is
		// never blank on startup (e.g. after sequential playback ended and
		// currentTrack was cleared before the last quit).
		if app.currentTrack == nil {
			app.syncRowMarquee()
		}
	} else {
		app.statusMsg = T("no_tracks_hint")
	}

	return app
}

// Cleanup saves the current session state to the database and removes
// the temporary directory.  Call this when the application exits.
func (a *App) Cleanup() {
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
	// saved moments earlier by the q-key handler).
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
	_ = a.st.SetSettings(pairs)
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

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.W, a.H = msg.Width, msg.Height
		a.progressBar.Width = a.miniPlayerW() - 6
		a.lyricsVP.Width = a.fullLyricsW() - 2
		a.lyricsVP.Height = a.fullBodyH() - 4
		return a, nil

	case scanDoneMsg:
		a.loading = false
		a.scanErr = msg.err
		// Update tracks regardless of error: cmdSyncLibrary always returns
		// whatever is in the DB even when syncErr is non-nil, so we show
		// the existing library and surface the error in the status bar.
		if msg.tracks != nil {
			a.tracks = msg.tracks
			// Re-apply the current search query and format preference so the
			// filtered list stays consistent after a library reload.
			a.applyFilter()
			a.rebuildShuffleIDs()
			// Re-anchor currentIdx to the playing track's ID so the position
			// is correct after the library list is replaced.
			// If the playing track is no longer in the library, stop playback.
			if a.currentTrack != nil {
				if pos := a.filteredPos(a.currentTrack.ID); pos >= 0 {
					// Track still exists — update cursor to its new position.
					a.cursorPos = pos
				} else {
					// Track no longer exists — stop immediately.
					a.player.Stop()
					a.currentTrack = nil
					a.cursorPos = 0
					a.statusMsg = T("track_removed")
					a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
				}
			}
			// Always clamp cursor and sync the row marquee after a
			// library reload, even when no track is playing.  Without
			// this the first render frame shows a stale/empty row.
			if n := a.filteredLen(); n > 0 {
				if a.cursorPos >= n {
					a.cursorPos = n - 1
				}
				if a.cursorPos < 0 {
					a.cursorPos = 0
				}
				a.syncRowMarquee()
			} else {
				a.cursorPos = 0
				a.mqRow.SetText("")
			}
		}
		if msg.err != nil {
			a.statusMsg = fmt.Sprintf(T("sync_error"), msg.err.Error())
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		} else if a.libScanFromReload {
			// Only show "Loaded N tracks" when the scan was explicitly
			// triggered by Ctrl+R, not on every startup.
			a.statusMsg = fmt.Sprintf(T("loaded_tracks"), len(a.tracks))
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			a.libScanFromReload = false
		}
		return a, nil

	case playResultMsg:
		if msg.err != nil {
			a.statusMsg = fmt.Sprintf(T("playback_error"), msg.err.Error())
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return a, nil
		}

		// Remember whether we were in chip mode before the track change.
		wasChipMode := a.chipMode

		// Reset all chip state unconditionally on track change.
		// Any in-flight conversion/crossfade goroutine will finish naturally
		// but its result will be ignored because chipOrigin won't match.
		a.chipMode = false
		a.chipBusy = false
		a.chipConverting = false

		a.currentTrack = msg.track
		// Position the cursor on the newly playing track.
		if msg.track != nil {
			if pos := a.filteredPos(msg.track.ID); pos >= 0 {
				a.cursorPos = pos
			}
		}
		a.statusMsg = ""
		a.syncMarquees()
		// Sync the correct row marquee depending on the active view.
		// When the Playlist tab's track level is visible, mqRow must
		// reflect plTracks[trackCursor]; otherwise it follows the
		// Local tab's cursorPos.
		if a.activeTab == tabPlaylist && a.depth == playlistDepthTracks {
			a.syncPlaylistTrackMarquee()
		} else {
			a.syncRowMarquee()
		}

		// Reset lyrics and kick off background load for the new track.
		a.lines = nil
		a.activeIdx = -1
		a.prevActiveIdx = -1
		a.trackID = ""
		a.synced = false
		a.lyricsLoading = msg.track != nil // loading until lyricsLoadedMsg arrives
		a.browseCenterIdx = -1
		a.browseTicks = 0
		a.browseExpired = false

		// Reset isFavorite for the new track; will be updated by cmdCheckFavorite.
		a.isFavorite = false

		var cmds []tea.Cmd
		// If we were in chip mode, automatically start converting the new track.
		if wasChipMode {
			cmds = append(cmds, a.cmdToggleChip())
		}
		if msg.track != nil {
			cmds = append(cmds, a.cmdLoadLyrics(*msg.track))
			cmds = append(cmds, a.cmdCheckFavorite(msg.track.ID))
		}
		return a, tea.Batch(cmds...)

	case playlistsLoadedMsg:
		if msg.err != nil {
			a.statusMsg = T("playlist_load_error") + ": " + msg.err.Error()
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		} else {
			a.playlists = msg.playlists
			// Clamp cursor after reload (e.g. after delete).
			if a.listCursor >= len(a.playlists) {
				a.listCursor = len(a.playlists) - 1
			}
			if a.listCursor < 0 {
				a.listCursor = 0
			}
			// Sync the playlist row marquee after any list reload.
			a.syncPlaylistRowMarquee()
			// Recompute the right-column width cache after list changes.
			a.refreshPlListMaxColW()
			if msg.successMsg != "" {
				a.statusMsg = msg.successMsg
				a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			}
		}
		// Restore track-level view on startup.
		restoreCmd := a.maybeRestorePlaylistTracks()
		return a, restoreCmd

	case playlistTracksLoadedMsg:
		if msg.err != nil {
			a.statusMsg = T("playlist_load_error") + ": " + msg.err.Error()
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return a, nil
		}
		// Keep the cached list counts fresh for every playlist mutation.
		for i := range a.playlists {
			if a.playlists[i].ID == msg.playlistID {
				a.playlists[i].TrackCount = msg.count
				a.refreshPlListMaxColW()
				break
			}
		}
		// Only replace the visible track list when the message is about the
		// playlist currently open.  Adding from another tab must not overwrite
		// the track list being viewed.
		if a.activeTab == tabPlaylist && a.depth == playlistDepthTracks &&
			a.currentPlaylist().ID == msg.playlistID {
			a.plTracks = msg.tracks
			if a.trackCursor >= len(a.plTracks) {
				a.trackCursor = len(a.plTracks) - 1
			}
			if a.trackCursor < 0 {
				a.trackCursor = 0
			}
			// Anchor cursor to the currently playing track when entering
			// a playlist's track list (pendingTrackAnchor == true),
			// matching the behaviour of mature players (cmus, ncmpcpp)
			// that jump to the playing song when entering a new list.
			// Only anchor on explicit entry, not on background reloads
			// (e.g. favoriteChangedMsg) which should preserve the cursor.
			if a.pendingTrackAnchor {
				a.anchorPlaylistTrackCursor()
			}
			a.pendingTrackAnchor = false
			// Sync mqRow so the selected row renders the correct track
			// text immediately, rather than showing stale content from a
			// previous tab or playlist.
			a.syncPlaylistTrackMarquee()
		}
		return a, nil

	case favoriteChangedMsg:
		if msg.trackID == "" {
			return a, nil
		}
		// Update isFavorite only when this is about the currently playing track.
		if a.currentTrack != nil && msg.trackID == a.currentTrack.ID {
			a.isFavorite = msg.isFavorite
		}
		// Show feedback only for explicit user actions (trackTitle is set by
		// cmdToggleFavorite; cmdCheckFavorite leaves it empty).
		if msg.trackTitle == "" {
			return a, nil
		}
		{
			if msg.isFavorite {
				a.statusMsg = iconHeartFilled() + " " + msg.trackTitle
			} else {
				a.statusMsg = iconHeartEmpty() + " " + msg.trackTitle
			}
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		}
		// If we're viewing the Favorites playlist, reload its track list.
		// The playlist list is also refreshed so cached counts stay accurate.
		if a.activeTab == tabPlaylist &&
			a.depth == playlistDepthTracks &&
			a.currentPlaylist().ID == store.FavoritesPlaylistID {
			return a, tea.Batch(
				a.cmdLoadPlaylistTracks(store.FavoritesPlaylistID),
				a.cmdLoadPlaylists(),
			)
		}
		return a, a.cmdLoadPlaylists()

	case playlistPlayMsg:
		// Replace the local library filteredIdxs with the playlist track order.
		// First, ensure all playlist tracks are present in a.tracks; add missing ones.
		if msg.err != nil {
			a.statusMsg = T("playlist_load_error") + ": " + msg.err.Error()
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return a, nil
		}
		existing := make(map[string]int, len(a.tracks))
		for i, t := range a.tracks {
			existing[t.ID] = i
		}
		newIdxs := make([]int, 0, len(msg.tracks))
		for _, pt := range msg.tracks {
			if idx, ok := existing[pt.ID]; ok {
				newIdxs = append(newIdxs, idx)
			}
		}
		if len(newIdxs) == 0 {
			a.statusMsg = T("no_tracks_hint")
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return a, nil
		}
		// Warn when some playlist tracks are no longer in the library.
		if len(newIdxs) < len(msg.tracks) {
			a.statusMsg = fmt.Sprintf(T("playlist_tracks_missing"), len(msg.tracks)-len(newIdxs))
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		}
		a.filteredIdxs = newIdxs
		pos := msg.startPos
		if pos >= len(newIdxs) {
			pos = 0
		}
		a.cursorPos = pos
		a.syncRowMarquee()
		return a, a.cmdPlayTrack(pos)

	case noopMsg:
		return a, nil

	case chip8DoneMsg:
		if msg.err != nil {
			// Conversion failed — unlock and surface the error.
			a.chipBusy = false
			a.chipConverting = false
			a.statusMsg = fmt.Sprintf(T("chip_convert_failed"), msg.err.Error())
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return a, nil
		}
		// Cache the result; conversion phase is done, crossfade phase begins.
		a.chipPath = msg.chipPath
		a.chipOrigin = msg.originPath
		a.chipConverting = false
		pos := a.player.Position()
		// Crossfade runs in a tea.Cmd goroutine; state mutations happen via
		// chipCrossfadeDoneMsg so we never write App fields from a goroutine.
		return a, func() tea.Msg {
			_ = a.player.CrossfadeTo(msg.chipPath, pos)
			return chipCrossfadeDoneMsg{chipMode: true}
		}

	case chipCrossfadeDoneMsg:
		a.chipMode = msg.chipMode
		a.chipBusy = false
		return a, nil

	case lyricsLoadedMsg:
		// Discard stale results — track may have changed while loading.
		if a.currentTrack != nil && msg.trackID == a.currentTrack.ID {
			if msg.err != nil {
				// Fetch failed (e.g. network timeout, context cancelled).
				// Leave lines nil so the UI shows "No lyrics" rather than stale data.
				a.lines = nil
				a.synced = false
			} else {
				a.lines = msg.lines
				a.activeIdx = -1
				a.trackID = msg.trackID
				// Determine whether any line carries a real timestamp.
				// Pure plain-text lyrics (all Time=0) are shown statically
				// without any active-line highlight.
				a.synced = false
				for _, l := range msg.lines {
					if l.Time > 0 {
						a.synced = true
						break
					}
				}
			}
		}
		a.lyricsLoading = false
		return a, nil

	case playbackEndedMsg:
		// Sequential mode reached the end of the list — clear the playing
		// state so the status bar correctly shows Stopped.
		a.currentTrack = nil
		a.isFavorite = false
		a.syncMarquees()
		// Re-anchor cursor row text so row 0 isn't blank.  Sync the
		// correct marquee depending on the active view.
		if a.activeTab == tabPlaylist && a.depth == playlistDepthTracks {
			a.syncPlaylistTrackMarquee()
		} else {
			a.syncRowMarquee()
		}
		return a, nil

	case tickMsg:
		a.tickMarquees()
		a.syncLyricsActive()
		// Clear transient status messages after their expiry time.
		if !a.statusMsgExpiry.IsZero() && time.Now().After(a.statusMsgExpiry) {
			a.statusMsg = ""
			a.statusMsgExpiry = time.Time{}
		}
		// Browse cursor expiry: count ticks while cursor is active.
		// Do NOT reset immediately — wait for the next line change so the
		// snap-back happens at a natural lyric boundary (smooth transition).
		if a.browseCenterIdx >= 0 && !a.browseExpired {
			a.browseTicks++
			if a.browseTicks >= browseFadeOutTicks {
				a.browseExpired = true
			}
		}
		return a, tick()

	case trackDoneMsg:
		return a, a.cmdAdvance()

	case tea.KeyMsg:
		return a, a.handleKey(msg)

	case progress.FrameMsg:
		prog, cmd := a.progressBar.Update(msg)
		a.progressBar = prog.(progress.Model)
		return a, cmd

	case viewport.Model:
		a.lyricsVP = msg
		return a, nil
	}

	// Forward to search input while search overlay is active.
	if a.activeOvl == overlaySearch {
		var cmd tea.Cmd
		a.searchInput, cmd = a.searchInput.Update(msg)
		a.applyFilter()
		return a, cmd
	}

	return a, nil
}

// View implements tea.Model.
func (a *App) View() string {
	if a.W == 0 {
		return "Loading…"
	}
	return a.render()
}

// ── Layout helpers ────────────────────────────────────────────────────────────

const (
	topPad     = 0 // no top padding — altscreen starts at row 0
	tabBarH    = 1 // single line for tab labels (no separator row)
	statusBarH = 1
	borderH    = 2
	borderW    = 2

	// Layout proportions and thresholds.
	miniPlayerMinWidth   = 60   // terminal width below which the mini player is hidden
	trackListWidthRatio  = 0.55 // fraction of total width allocated to the track list
	trackListMinWidth    = 20   // minimum track list width in columns
	fullPlayerWidthRatio = 0.40 // fraction of total width allocated to the full-screen player panel
	fullPlayerMinWidth   = 24   // minimum full-screen player width in columns
	coverMinSize         = 4    // minimum cover art outerRows value

	// marqueeSep is the separator inserted between repetitions of scrolling text.
	marqueeSep = "  •  "

	// statusMsgDuration is how long a status-bar message stays visible before
	// it is automatically cleared.
	statusMsgDuration = 2 * time.Second
)

// bodyH is the available height for the main content panels.
func (a *App) bodyH() int {
	h := a.H - topPad - tabBarH - statusBarH
	if h < 4 {
		return 4
	}
	return h
}

// panelInnerH is the inner content height inside a bordered panel.
func (a *App) panelInnerH() int {
	h := a.bodyH() - borderH
	if h < 2 {
		return 2
	}
	return h
}

// trackPageSize is the number of rows moved by PageUp/PageDown: one viewport
// of track rows (panel height minus the header and divider).
func (a *App) trackPageSize() int {
	n := a.panelInnerH() - 2
	if n < 1 {
		return 1
	}
	return n
}

// showMiniPlayer returns true when the terminal is wide enough and the right
// panel is not collapsed.
func (a *App) showMiniPlayer() bool { return !a.rightCollapsed && a.W >= miniPlayerMinWidth }

// trackListOuterW returns the outer (border-inclusive) width of the track list.
func (a *App) trackListOuterW() int {
	if !a.showMiniPlayer() {
		return a.W
	}
	w := int(float64(a.W) * trackListWidthRatio)
	if w < trackListMinWidth {
		return trackListMinWidth
	}
	return w
}

// miniPlayerOuterW returns the outer width of the mini player panel.
func (a *App) miniPlayerOuterW() int { return a.W - a.trackListOuterW() }

// trackListInnerW / miniPlayerW are inner content widths.
func (a *App) trackListInnerW() int { return a.trackListOuterW() - borderW }
func (a *App) miniPlayerW() int     { return a.miniPlayerOuterW() - borderW }

// Fullscreen layout
func (a *App) fullBodyH() int {
	h := a.H - topPad - statusBarH - borderH
	if h < 4 {
		return 4
	}
	return h
}
func (a *App) fullPlayerOuterW() int {
	w := int(float64(a.W) * fullPlayerWidthRatio)
	if w < fullPlayerMinWidth {
		return fullPlayerMinWidth
	}
	return w
}
func (a *App) fullLyricsOuterW() int { return a.W - a.fullPlayerOuterW() }
func (a *App) fullPlayerInnerW() int { return a.fullPlayerOuterW() - borderW }
func (a *App) fullLyricsW() int      { return a.fullLyricsOuterW() - borderW }

// ── Filter / index helpers ────────────────────────────────────────────────────

// filteredLen returns the number of tracks that pass the current filter.
func (a *App) filteredLen() int { return len(a.filteredIdxs) }

// filteredTrack returns the track at position pos in the filtered list.
// Returns a zero Track if pos is out of range.
func (a *App) filteredTrack(pos int) library.Track {
	if pos < 0 || pos >= len(a.filteredIdxs) {
		return library.Track{}
	}
	return a.tracks[a.filteredIdxs[pos]]
}

// filteredPos returns the position of the track with the given ID in the
// filtered list, or -1 if it is not present.
func (a *App) filteredPos(id string) int {
	for i, idx := range a.filteredIdxs {
		if a.tracks[idx].ID == id {
			return i
		}
	}
	return -1
}

// cursorTrack returns the track currently under the cursor.
// Returns nil when the filtered list is empty.
func (a *App) cursorTrack() *library.Track {
	if len(a.filteredIdxs) == 0 {
		return nil
	}
	pos := a.cursorPos
	if pos < 0 {
		pos = 0
	}
	if pos >= len(a.filteredIdxs) {
		pos = len(a.filteredIdxs) - 1
	}
	t := a.tracks[a.filteredIdxs[pos]]
	return &t
}

// ── Shuffle helpers ───────────────────────────────────────────────────────────

// rebuildShuffleIDs creates a fresh shuffled ID order from the current
// filtered list.  IDs are stable across subsequent filter changes; the
// cmdPlayNext logic skips any ID no longer present in filteredIdxs.
func (a *App) rebuildShuffleIDs() {
	ids := make([]string, len(a.filteredIdxs))
	for i, idx := range a.filteredIdxs {
		ids[i] = a.tracks[idx].ID
	}
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	a.shuffleIDs = ids
}

// ── Marquee helpers ───────────────────────────────────────────────────────────

// syncRowMarquee updates mqRow to match the current cursor position.
// The text is the middle-column content: Album · Artist · Title.
func (a *App) syncRowMarquee() {
	t := a.cursorTrack()
	if t == nil {
		a.mqRow.SetText("")
		return
	}
	a.mqRow.SetText(rowMidText(*t))
}

// syncMarquees updates all Marquee texts from the current track.
// Call whenever the current track changes.
func (a *App) syncMarquees() {
	// Invalidate cover art cache whenever the track changes.
	a.coverRendered = ""

	if a.currentTrack == nil {
		a.mqTitle.SetText("")
		a.mqMeta.SetText("")
		a.mqArtist.SetText("")
		a.mqAlbum.SetText("")
		return
	}
	t := a.currentTrack
	a.mqTitle.SetText(t.DisplayTitle())
	a.mqMeta.SetText(displayArtist(*t) + " · " + t.Album)
	a.mqArtist.SetText(displayArtist(*t))
	a.mqAlbum.SetText(t.Album)
}

// getCoverArt renders the current track's embedded cover art inside a square
// bordered box that fits within maxOuterCols columns and maxOuterRows character
// rows.
//
// The largest visual square is chosen: each character row equals 2 pixel rows,
// so outerCols = outerRows*2.  Both maxOuterCols and maxOuterRows constrain the
// result.  The image is scaled with its original aspect ratio preserved
// ("contain"): the longest edge fills the inner area and the shorter edge is
// centred with blank padding.  Falls back to buildCoverPlaceholderSized when no
// cover art is available or rendering fails.
//
// Results are cached and reused as long as the track and box dimensions are
// unchanged.
func (a *App) getCoverArt(maxOuterCols, maxOuterRows int) string {
	outerRows := maxOuterCols / 2
	if maxOuterRows > 0 && outerRows > maxOuterRows {
		outerRows = maxOuterRows
	}
	if outerRows < coverMinSize {
		outerRows = coverMinSize
	}
	outerCols := outerRows * 2

	if a.currentTrack == nil {
		return buildCoverPlaceholderSized(outerCols, outerRows)
	}

	// Prefer in-memory CoverArt (fresh scan); fall back to CoverPath (DB load).
	coverData := a.currentTrack.CoverArt
	if len(coverData) == 0 && a.currentTrack.CoverPath != "" {
		if data, err := os.ReadFile(a.currentTrack.CoverPath); err == nil {
			coverData = data
			// Cache back into the track to avoid re-reading every frame.
			a.currentTrack.CoverArt = data
		}
	}

	if len(coverData) == 0 {
		return buildCoverPlaceholderSized(outerCols, outerRows)
	}
	if a.coverRendered != "" && a.coverW == outerCols {
		return a.coverRendered
	}

	rendered := renderCoverArt(coverData, outerCols, outerRows)
	if rendered == "" {
		return buildCoverPlaceholderSized(outerCols, outerRows)
	}
	a.coverRendered = rendered
	a.coverW = outerCols
	return a.coverRendered
}

// tickMarquees advances all Marquee scroll positions.
// It does NOT update marquee text — that is handled by syncRowMarquee()
// (called on cursor movement) and syncMarquees() (called on track change).
// Keeping text updates out of the tick path avoids a rowMidText() allocation
// every 500 ms regardless of whether the cursor or track has changed.
func (a *App) tickMarquees() {
	listW := a.trackListInnerW()
	const leftFixW = 2
	const rightColW = 10
	midAvail := listW - leftFixW - rightColW
	if midAvail < 4 {
		midAvail = 4
	}

	// Playlist list level: left icon (2) + right count (8).
	const plRightColW = 8
	plMidAvail := listW - leftFixW - plRightColW
	if plMidAvail < 4 {
		plMidAvail = 4
	}

	miniW := a.miniPlayerW() - 4
	if miniW <= 0 {
		// Collapsed / narrow: use a small threshold so that even short
		// titles (e.g. 10 CJK chars = 20 display cols) trigger scrolling.
		miniW = 16
	}
	fullW := a.fullPlayerInnerW() - 4
	if fullW <= 0 {
		fullW = 20
	}

	a.mqRow.Tick(midAvail)
	a.mqPlRow.Tick(plMidAvail)
	a.mqTitle.Tick(miniW)
	a.mqMeta.Tick(miniW)
	a.mqArtist.Tick(fullW)
	a.mqAlbum.Tick(fullW)
}

// playlistDisplayName returns the display name for a playlist, using the
// localised name when it is the built-in Favorites playlist.
func (a *App) playlistDisplayName(pl store.Playlist) string {
	if pl.ID == store.FavoritesPlaylistID {
		return T("playlist_favorites_name")
	}
	return pl.Name
}

// keepOvlCursorVisible adjusts ovlScrollRow so that ovlPlCursor is inside
// the visible body area of the playlist-picker overlay.
func (a *App) keepOvlCursorVisible() {
	// Approximate body-available rows: terminal height minus overlay chrome.
	// renderScrollableOverlay: boxBorderH=2 + boxPadV=2 + outerMargin=2 +
	// titleRows=2 + hintRows=2 = 10 total fixed rows.
	const chromeRows = 10
	bodyAvail := a.H - chromeRows
	if bodyAvail < 1 {
		bodyAvail = 1
	}
	if a.ovlPlCursor < a.ovlScrollRow {
		a.ovlScrollRow = a.ovlPlCursor
	} else if a.ovlPlCursor >= a.ovlScrollRow+bodyAvail {
		a.ovlScrollRow = a.ovlPlCursor - bodyAvail + 1
	}
}

// refreshPlListMaxColW recomputes the cached right-column width for playlist
// track counts.  Must be called after playlists are loaded or track counts
// change (add/remove).  Without a store the minimum width (4) is used.
func (a *App) refreshPlListMaxColW() {
	const minW = 4
	a.plListMaxColW = minW
	for _, pl := range a.playlists {
		if w := strWidth(fmt.Sprintf(T("playlist_track_count"), pl.TrackCount)); w > a.plListMaxColW {
			a.plListMaxColW = w
		}
	}
}

// maybeRestorePlaylistTracks triggers a track load for the playlist that was
// last open when the app exited.  Returns the load command or nil.
// Only effective once (clears restorePlaylistID after the first call).
func (a *App) maybeRestorePlaylistTracks() tea.Cmd {
	if a.restorePlaylistID == "" || a.depth != playlistDepthTracks {
		return nil
	}
	plID := a.restorePlaylistID
	a.restorePlaylistID = ""
	// Verify the playlist still exists.
	for _, pl := range a.playlists {
		if pl.ID == plID {
			return a.cmdLoadPlaylistTracks(plID)
		}
	}
	return nil
}

// syncPlaylistRowMarquee updates mqPlRow to match the currently selected
// playlist in the playlist list level.
func (a *App) syncPlaylistRowMarquee() {
	if a.listCursor < 0 || a.listCursor >= len(a.playlists) {
		a.mqPlRow.SetText("")
		return
	}
	pl := a.playlists[a.listCursor]
	name := a.playlistDisplayName(pl)
	a.mqPlRow.SetText(name)
}

// anchorPlaylistTrackCursor moves trackCursor to the currently playing
// track when it exists in the playlist, matching the behaviour of mature
// players (cmus, ncmpcpp) that always jump to the playing song when
// entering or switching to a track list.
func (a *App) anchorPlaylistTrackCursor() {
	if a.currentTrack != nil && len(a.plTracks) > 0 {
		for i, t := range a.plTracks {
			if t.ID == a.currentTrack.ID {
				a.trackCursor = i
				return
			}
		}
	}
}

// syncPlaylistTrackMarquee updates mqRow to match the currently selected
// track in the playlist track level.  This is necessary because the Playlist
// tab's track level shares mqRow with the Local tab; without syncing, the
// selected row may render stale text from a different view.
func (a *App) syncPlaylistTrackMarquee() {
	if a.trackCursor >= 0 && a.trackCursor < len(a.plTracks) {
		a.mqRow.SetText(rowMidText(a.plTracks[a.trackCursor]))
	} else {
		a.mqRow.SetText("")
	}
}

// syncLyricsActive updates activeIdx to the lyric line that should be
// highlighted at the current playback position.
//
// For plain-text (unsynchronised) lyrics where all timestamps are zero,
// activeIdx is kept at -1 so no line is highlighted — the entire list is
// shown at uniform brightness.
//
// When the browse cursor has expired (browseExpired=true) and the playback
// line advances to a new index, the cursor is snapped back to follow mode
// at that natural lyric boundary rather than jumping mid-line.
func (a *App) syncLyricsActive() {
	if len(a.lines) == 0 || !a.synced {
		a.activeIdx = -1
		return
	}
	pos := a.player.Position()
	idx := -1
	for i, l := range a.lines {
		if l.Time <= pos {
			idx = i
		} else {
			break
		}
	}

	// Snap-back on natural line advance when browse has expired.
	if a.browseExpired && idx != a.prevActiveIdx && idx >= 0 {
		a.browseCenterIdx = -1
		a.browseTicks = 0
		a.browseExpired = false
	}

	a.prevActiveIdx = a.activeIdx
	a.activeIdx = idx
}

// buildLyricsProvider constructs the lyrics provider chain from the current
// configuration.  It is called at startup and can be called again whenever
// the user changes provider settings (e.g. enabling or disabling online lookup).
//
// Chain order: local .lrc files → cached lrclib.net.
// If the lyrics cache directory cannot be resolved, the chain falls back to
// local-only so that offline usage is unaffected.
func buildLyricsProvider() lyrics.Provider {
	base := lyrics.LocalLRCProvider{}
	cacheDir, err := store.LyricsCacheDir()
	if err != nil {
		return base
	}
	cachedOnline := lyrics.NewCachedProvider(online.NewLrcLibProvider(), cacheDir)
	return &lyrics.ChainProvider{
		Providers: []lyrics.Provider{base, cachedOnline},
	}
}
