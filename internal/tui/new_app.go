package tui

import (
	"os"
	"strconv"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/lonsty/music-tui/internal/audio"
	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/store"
)

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
		netCancel:      func() {}, // no-op until a network request is started
		rightCollapsed: rightCollapsed,
		rightMode:      rightMode,
		PlaylistState: PlaylistState{
			depth:             initialDepth,
			listCursor:        initialListCursor,
			trackCursor:       initialTrackCursor,
			nameInput:         ni,
			restorePlaylistID: initialPlaylistID,
		},
		OverlayState: OverlayState{
			addInput: ai,
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
		app.rebuildTrackPos()
		app.groupsDirty = true
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
