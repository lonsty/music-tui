// Package tui implements the Bubble Tea application for music-tui.
package tui

import (
	"fmt"
	"math/rand"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/eilianxiao/music-tui/internal/audio"
	"github.com/eilianxiao/music-tui/internal/library"
)

// App is the root Bubble Tea model.
type App struct {
	player   *audio.Player
	musicDir string

	// ── Library ──────────────────────────────────────────────────────────────
	tracks       []library.Track
	filtered     []library.Track
	shuffleOrder []int // indices into filtered, used when playMode == random
	cursor       int
	currentTrack *library.Track
	currentIdx   int // index of currentTrack in filtered (for next/prev)

	// ── Playback ─────────────────────────────────────────────────────────────
	volume   float64  // [0.0, 2.0]; 1.0 = unity gain
	playMode playMode // sequential / loop / single / random

	// ── Search ───────────────────────────────────────────────────────────────
	searchInput textinput.Model

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
}

// NewApp creates the application model. Call WithProgram after tea.NewProgram.
func NewApp(player *audio.Player, musicDir string) *App {
	ti := textinput.New()
	ti.Placeholder = "Search… (s: artist  a: album  t: title)"
	ti.CharLimit = 128

	prog := progress.New(
		progress.WithGradient("#89B4FA", "#CBA6F7"),
		progress.WithoutPercentage(),
	)

	return &App{
		player:      player,
		musicDir:    musicDir,
		volume:      1.0,
		searchInput: ti,
		progressBar: prog,
		loading:     true,
		playMode:    playModeLoop,
	}
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
	return tea.Batch(tick(), a.cmdScanLibrary())
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
		} else {
			a.currentTrack = msg.track
			a.currentIdx = msg.idx
			a.cursor = msg.idx
			a.statusMsg = ""
		}
		return a, nil

	case noopMsg:
		return a, nil

	case tickMsg:
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
