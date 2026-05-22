// Package tui implements the Bubble Tea application for music-tui.
package tui

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/eilianxiao/music-tui/internal/audio"
	"github.com/eilianxiao/music-tui/internal/library"
	"github.com/eilianxiao/music-tui/internal/store"
)

// SessionState holds the persisted state that is restored on the next launch.
type SessionState struct {
	LastTrackPath  string
	LastPositionMs int64
	WasPlaying     bool // if true, load the track but start paused
	Volume         float64
	PlayMode       int
	RetroIdx       int
	Cursor         int
	Chip8Options   string
}

// App is the root Bubble Tea model.
type App struct {
	player   *audio.Player
	st       *store.Store // persistent database (may be nil for tests)
	musicDir string       // current music library root

	// session holds the state loaded from the DB at startup; it is consumed
	// by cmdRestoreSession on the first Init tick and then zeroed out.
	session *SessionState

	// ── Library ──────────────────────────────────────────────────────────────
	tracks       []library.Track
	filtered     []library.Track
	shuffleOrder []int // indices into filtered, used when playMode == random
	cursor       int
	currentTrack *library.Track
	currentIdx   int // index of currentTrack in filtered (for next/prev)

	// ── Playback ─────────────────────────────────────────────────────────────
	volume    float64  // [0.0, 2.0]; 1.0 = unity gain
	playMode  playMode // sequential / loop / single / random
	retroIdx  int      // retro effect preset index (0 = off)

	// ── 8-bit mode ───────────────────────────────────────────────────────────
	chipMode       bool   // currently playing the 8-bit converted version
	chipBusy       bool   // conversion or crossfade in progress (locked)
	chipConverting bool   // true only during p2chip conversion (not crossfade)
	chipPath       string // path to the cached 8-bit mp3 (in tmpDir)
	chipOrigin     string // Track.Path for which chipPath was generated
	tmpDir         string // temp directory; created on startup, removed on exit
	chip8Options   string // extra CLI options forwarded to p2chip

	// ── Search ───────────────────────────────────────────────────────────────
	searchInput textinput.Model

	// ── Settings overlay ─────────────────────────────────────────────────────
	settingsInput  textinput.Model // p2chip options
	musicDirInput  textinput.Model // music library directory
	settingsActive int             // 0 = musicDirInput active, 1 = settingsInput active

	// ── Progress bar ─────────────────────────────────────────────────────────
	progressBar progress.Model

	// ── Lyrics viewport (fullscreen) ─────────────────────────────────────────
	lyricsVP viewport.Model

	// ── UI state ─────────────────────────────────────────────────────────────
	W, H        int
	currentView view
	activeTab   tabID
	activeOvl   overlay

	// ── Status / feedback ────────────────────────────────────────────────────
	statusMsg string

	// ── Loading ──────────────────────────────────────────────────────────────
	loading bool
	scanErr error

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
	mqRow    *Marquee // selected list row "artist — title"

	// ── Cover art cache ───────────────────────────────────────────────────────
	// Pre-rendered terminal art string for the current track's cover.
	// Invalidated whenever the track changes or the box size changes.
	coverRendered string
	coverW        int // outerCols value used to produce coverRendered
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

	ti := textinput.New()
	ti.Placeholder = "Search… (s: artist  a: album  t: title)"
	ti.CharLimit = 128

	si := textinput.New()
	si.Placeholder = "--sf2 nes --onset 0.5"
	si.CharLimit = 256
	si.Width = 42

	di := textinput.New()
	di.Placeholder = "/path/to/music"
	di.CharLimit = 512
	di.Width = 42

	prog := progress.New(
		progress.WithGradient("#89B4FA", "#CBA6F7"),
		progress.WithoutPercentage(),
	)

	tmpDir, err := os.MkdirTemp("", "music-tui-*")
	if err != nil {
		// Fall back to the system temp directory; p2chip will still work.
		tmpDir = os.TempDir()
	}

	vol := 1.0
	mode := playModeLoop
	rIdx := 0
	cursor := 0
	if sess != nil {
		if sess.Volume > 0 {
			vol = sess.Volume
		}
		mode = playMode(sess.PlayMode)
		rIdx = sess.RetroIdx
		cursor = sess.Cursor
	}

	app := &App{
		player:        player,
		st:            st,
		musicDir:      musicDir,
		volume:        vol,
		searchInput:   ti,
		settingsInput: si,
		musicDirInput: di,
		progressBar:   prog,
		loading:       false,
		playMode:      mode,
		retroIdx:      rIdx,
		mqTitle:       NewMarquee("", "  •  "),
		mqMeta:        NewMarquee("", "  •  "),
		mqArtist:      NewMarquee("", "  •  "),
		mqAlbum:       NewMarquee("", "  •  "),
		mqRow:         NewMarquee("", "  •  "),
		tmpDir:        tmpDir,
		chip8Options:  chip8Opts,
		session:       sess,
	}

	// Apply volume to player immediately.
	player.SetVolume(vol)
	// Apply retro preset.
	player.SetRetroPreset(rIdx)

	if len(tracks) > 0 {
		library.SortByArtistAlbum(tracks)
		app.tracks = tracks
		app.filtered = tracks
		app.rebuildShuffle()
		app.statusMsg = fmt.Sprintf("Loaded %d tracks", len(tracks))
		// Restore cursor, clamped to valid range.
		if cursor >= len(tracks) {
			cursor = len(tracks) - 1
		}
		if cursor < 0 {
			cursor = 0
		}
		app.cursor = cursor

		// Pre-populate currentTrack so the first rendered frame already shows
		// the correct playing state (no flash/blank period while the async
		// restore Cmd is in flight).
		if sess != nil && sess.LastTrackPath != "" {
			for i, t := range tracks {
				if t.Path == sess.LastTrackPath {
					tc := t
					app.currentTrack = &tc
					app.currentIdx = i
					app.cursor = i
					app.syncMarquees()
					app.syncRowMarquee() // populate mqRow for gradient list row
					break
				}
			}
		}
	} else {
		app.statusMsg = "No tracks — open Settings (,) and reload the library"
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
	wasPlaying := "0"
	if state == audio.StatePlaying || state == audio.StatePaused {
		wasPlaying = "1"
	}

	lastTrackPath := ""
	if a.currentTrack != nil {
		lastTrackPath = a.currentTrack.Path
	}

	// Only write last_position_ms when the player is alive; skip the write
	// when pos==0 and the player is stopped (would overwrite a valid position
	// saved moments earlier by the q-key handler).
	posMs := pos.Milliseconds()
	pairs := map[string]string{
		"volume":          strconv.FormatFloat(a.volume, 'f', 4, 64),
		"play_mode":       strconv.Itoa(int(a.playMode)),
		"retro_idx":       strconv.Itoa(a.retroIdx),
		"last_track_path": lastTrackPath,
		"was_playing":     wasPlaying,
		"cursor":          strconv.Itoa(a.cursor),
		"chip8_options":   a.chip8Options,
	}
	if posMs > 0 || state != audio.StateStopped {
		pairs["last_position_ms"] = strconv.FormatInt(posMs, 10)
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
	if a.session != nil && a.session.LastTrackPath != "" {
		cmds = append(cmds, a.cmdRestoreSession())
	}
	return tea.Batch(cmds...)
}

// cmdScanLibrary scans the music directory in a goroutine.
func (a *App) cmdScanLibrary() tea.Cmd {
	dir := a.musicDir
	return func() tea.Msg {
		tracks, err := library.ScanDir(dir)
		if err == nil {
			library.SortByArtistAlbum(tracks)
		}
		return scanDoneMsg{tracks: tracks, err: err}
	}
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
		if msg.err == nil {
			a.tracks = msg.tracks
			a.filtered = msg.tracks
			a.rebuildShuffle()
			a.statusMsg = fmt.Sprintf("Loaded %d tracks", len(a.tracks))
		} else {
			a.statusMsg = "Scan error: " + msg.err.Error()
		}
		return a, nil

	case playResultMsg:
		if msg.err != nil {
			a.statusMsg = "󰅚  " + msg.err.Error()
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
		a.currentIdx = msg.idx
		a.cursor = msg.idx
		a.statusMsg = ""
		a.syncMarquees()
		a.syncRowMarquee()

		// If we were in chip mode, automatically start converting the new track.
		if wasChipMode {
			return a, a.cmdToggleChip()
		}
		return a, nil

	case noopMsg:
		return a, nil

	case chip8DoneMsg:
		if msg.err != nil {
			// Conversion failed — unlock and surface the error.
			a.chipBusy = false
			a.chipConverting = false
			a.statusMsg = "󰅚  8-bit convert failed: " + msg.err.Error()
			return a, nil
		}
		// Cache the result; conversion phase is done, crossfade phase begins.
		a.chipPath = msg.chipPath
		a.chipOrigin = msg.originPath
		a.chipConverting = false
		pos := a.player.Position()
		return a, func() tea.Msg {
			_ = a.player.CrossfadeTo(msg.chipPath, pos)
			a.chipMode = true
			a.chipBusy = false
			return noopMsg{}
		}

	case tickMsg:
		a.tickMarquees()
		return a, tick()

	case trackDoneMsg:
		return a, a.cmdPlayNext()

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

	// Forward to settings input while settings overlay is active.
	if a.activeOvl == overlaySettings {
		var cmd tea.Cmd
		if a.settingsActive == 0 {
			a.musicDirInput, cmd = a.musicDirInput.Update(msg)
		} else {
			a.settingsInput, cmd = a.settingsInput.Update(msg)
		}
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

// showMiniPlayer returns true when the terminal is wide enough.
func (a *App) showMiniPlayer() bool { return a.W >= 100 }

// trackListOuterW returns the outer (border-inclusive) width of the track list.
func (a *App) trackListOuterW() int {
	if !a.showMiniPlayer() {
		return a.W
	}
	w := int(float64(a.W) * 0.55)
	if w < 20 {
		return 20
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
	w := int(float64(a.W) * 0.40)
	if w < 24 {
		return 24
	}
	return w
}
func (a *App) fullLyricsOuterW() int { return a.W - a.fullPlayerOuterW() }
func (a *App) fullPlayerInnerW() int { return a.fullPlayerOuterW() - borderW }
func (a *App) fullLyricsW() int      { return a.fullLyricsOuterW() - borderW }

// ── Shuffle helpers ───────────────────────────────────────────────────────────

// rebuildShuffle creates a fresh shuffled index order for a.filtered.
func (a *App) rebuildShuffle() {
	n := len(a.filtered)
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	rand.Shuffle(n, func(i, j int) { order[i], order[j] = order[j], order[i] })
	a.shuffleOrder = order
}

// ── Marquee helpers ───────────────────────────────────────────────────────────

// syncRowMarquee updates mqRow to match the current cursor position.
// The text is the middle-column content: Album · Artist · Title.
func (a *App) syncRowMarquee() {
	if a.cursor >= len(a.filtered) {
		a.mqRow.SetText("")
		return
	}
	t := a.filtered[a.cursor]
	a.mqRow.SetText(rowMidText(t))
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
	a.mqMeta.SetText(t.DisplayArtist() + " · " + t.Album)
	a.mqArtist.SetText(t.DisplayArtist())
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
	if outerRows < 4 {
		outerRows = 4
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
func (a *App) tickMarquees() {
	listW := a.trackListInnerW()
	const leftFixW = 2
	const rightColW = 10
	midAvail := listW - leftFixW - rightColW
	if midAvail < 4 {
		midAvail = 4
	}

	miniW := a.miniPlayerW() - 4
	if miniW <= 0 {
		miniW = 20
	}
	fullW := a.fullPlayerInnerW() - 4
	if fullW <= 0 {
		fullW = 20
	}

	// Sync the selected-row marquee text each tick so it follows cursor moves.
	if a.cursor < len(a.filtered) {
		t := a.filtered[a.cursor]
		a.mqRow.SetText(rowMidText(t))
	} else {
		a.mqRow.SetText("")
	}

	a.mqRow.Tick(midAvail)
	a.mqTitle.Tick(miniW)
	a.mqMeta.Tick(miniW)
	a.mqArtist.Tick(fullW)
	a.mqAlbum.Tick(fullW)
}
