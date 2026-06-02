package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/lyrics"
	"github.com/lonsty/music-tui/internal/store"
)

// ── View ─────────────────────────────────────────────────────────────────────

// tickInterval is the period between progress-refresh ticks.
// It is also the time unit for browseFadeOutTicks in actions.go.
const tickInterval = 500 * time.Millisecond

// view represents the top-level screen being shown.
type view int

const (
	viewNormal     view = iota // Two-panel: track list + mini player
	viewFullscreen             // Full-screen player + lyrics
)

// ── Settings overlay fields ───────────────────────────────────────────────────

// settingsField identifies the active input field in the settings overlay.
// The numeric values are used as the settingsActive index; the order determines
// the Tab-cycling sequence.
type settingsField int

const (
	settingsFieldMusicDir  settingsField = iota // music library directory input
	settingsFieldChipOpts                       // p2chip options input
	settingsFieldLanguage                       // language toggle (no text input)
	settingsFieldFormat                         // format preference toggle (no text input)
	settingsFieldIconSet                        // icon set toggle (no text input)
	settingsFieldCount                          // sentinel — must be last
)

// ── Right panel mode ─────────────────────────────────────────────────────────

// rightPanelMode controls the content shown in the right mini-player panel
// when the normal view is active and the panel is not collapsed.
type rightPanelMode int

const (
	rightPanelPlayer rightPanelMode = iota // default: cover art + single lyric line + controls
	rightPanelLyrics                       // scrolling lyrics panel (same as fullscreen) + controls
)

// rightPanelModeKey returns the settings-DB string for a rightPanelMode value.
func rightPanelModeKey(m rightPanelMode) string {
	switch m {
	case rightPanelLyrics:
		return "lyrics"
	default:
		return "player"
	}
}

// parseRightPanelMode converts a DB string back to a rightPanelMode value.
// Unknown or empty strings map to rightPanelPlayer (the default).
func parseRightPanelMode(s string) rightPanelMode {
	switch s {
	case "lyrics":
		return rightPanelLyrics
	default:
		return rightPanelPlayer
	}
}

// settingsFieldBodyRow maps each settingsField to its row index within the
// Settings overlay's scrollable body.  Used to keep ovlScrollRow in sync
// with the selected field when the user navigates with ↑/↓.
var settingsFieldBodyRow = [settingsFieldCount]int{
	settingsFieldMusicDir: 3,
	settingsFieldChipOpts: 8,
	settingsFieldLanguage: 15,
	settingsFieldFormat:   19,
	settingsFieldIconSet:  23,
}

// ── Icon set serialisation ────────────────────────────────────────────────────

// iconSetKey returns the settings-DB string for an iconSet value.
func iconSetKey(s iconSet) string {
	switch s {
	case iconSetEmoji:
		return "emoji"
	case iconSetPlain:
		return "plain"
	default:
		return "nerd"
	}
}

// parseIconSet converts a DB string back to an iconSet value.
// Unknown or empty strings map to iconSetNerdFont (the default).
func parseIconSet(s string) iconSet {
	switch s {
	case "emoji":
		return iconSetEmoji
	case "plain":
		return iconSetPlain
	default:
		return iconSetNerdFont
	}
}

// iconSetDisplayLabel returns a short localised label for the given iconSet,
// used in the Settings overlay value column.
func iconSetDisplayLabel(s iconSet) string {
	switch s {
	case iconSetEmoji:
		return T("icon_set_emoji")
	case iconSetPlain:
		return T("icon_set_plain")
	default:
		return T("icon_set_nerd")
	}
}

// ── Overlay ───────────────────────────────────────────────────────────────────

// overlay is a modal layer drawn on top of the current view.
type overlay int

const (
	overlayNone          overlay = iota
	overlayHelp                  // ? key
	overlaySearch                // / key
	overlayInfo                  // i key — track detail
	overlaySettings              // , key — settings
	overlayAddToPlaylist         // a key in Library tab — add cursor track to a playlist
)

// ── Tab ───────────────────────────────────────────────────────────────────────

// tabID identifies the active top-level tab.
type tabID int

const (
	tabLocal    tabID = iota
	tabOnline         // online music search — placeholder
	tabPlaylist       // playlist management
	tabCount          // sentinel — must be the last constant
)

// ── Playlist navigation ───────────────────────────────────────────────────────

// playlistDepth controls which level of the playlist tab is shown.
type playlistDepth int

const (
	playlistDepthList   playlistDepth = iota // top level: list of all playlists
	playlistDepthTracks                      // drill-down: tracks inside one playlist
)

// playlistInputMode identifies the purpose of the inline text-input box.
type playlistInputMode int

const (
	playlistInputNone   playlistInputMode = iota // no input active
	playlistInputCreate                          // creating a new playlist
	playlistInputRename                          // renaming the selected playlist
)

// ── Play mode ────────────────────────────────────────────────────────────────

// playMode controls how the next track is chosen when the current one ends.
type playMode int

const (
	playModeSequential playMode = iota // play list top-to-bottom, stop at end
	playModeLoop                       // repeat list indefinitely
	playModeSingle                     // repeat current track
	playModeRandom                     // pick a random next track
	playModeCount                      // sentinel — must be the last constant
)

// ── Format preference ─────────────────────────────────────────────────────────

// formatPreference controls which format versions of a track are shown in the
// library when multiple copies exist in different formats (e.g. both .mp3 and
// .flac for the same album).
type formatPreference int

const (
	// formatPrefAll shows every file regardless of format (default).
	// No deduplication is applied; the user sees all copies.
	formatPrefAll formatPreference = iota

	// formatPrefLosslessFirst deduplicates tracks that share the same
	// (artist, album, title) triple.  For each group, only the highest-quality
	// format version is retained (FLAC > WAV > OGG > MP3).  If only a lossy
	// copy exists it is still shown.
	formatPrefLosslessFirst

	// formatPrefLosslessOnly shows only lossless tracks (FLAC, WAV).
	// Lossy-only tracks (MP3, OGG) are hidden entirely.
	formatPrefLosslessOnly

	// formatPrefMP3Only shows only MP3 tracks.
	formatPrefMP3Only

	formatPrefCount // sentinel — must be the last constant
)

// formatPrefLabel returns a short display label for a formatPreference value.
func formatPrefLabel(p formatPreference) string {
	switch p {
	case formatPrefLosslessFirst:
		return T("fmt_pref_lossless_first")
	case formatPrefLosslessOnly:
		return T("fmt_pref_lossless_only")
	case formatPrefMP3Only:
		return T("fmt_pref_mp3_only")
	default:
		return T("fmt_pref_all")
	}
}

// formatPrefKey returns the settings-DB string for a formatPreference value.
func formatPrefKey(p formatPreference) string {
	switch p {
	case formatPrefLosslessFirst:
		return "lossless_first"
	case formatPrefLosslessOnly:
		return "lossless_only"
	case formatPrefMP3Only:
		return "mp3_only"
	default:
		return "all"
	}
}

// parseFormatPref converts a DB string back to a formatPreference value.
// Unknown strings map to formatPrefAll.
func parseFormatPref(s string) formatPreference {
	switch s {
	case "lossless_first":
		return formatPrefLosslessFirst
	case "lossless_only":
		return formatPrefLosslessOnly
	case "mp3_only":
		return formatPrefMP3Only
	default:
		return formatPrefAll
	}
}

// ── Tea messages ─────────────────────────────────────────────────────────────

// tickMsg is fired every 500 ms to refresh the progress bar.
type tickMsg time.Time

// trackDoneMsg is sent by the audio player when a track ends naturally.
type trackDoneMsg struct{}

// scanDoneMsg carries the result of a background library scan.
type scanDoneMsg struct {
	tracks []library.Track
	err    error
}

// playResultMsg is returned after a play attempt.
// track is nil on error.  The position in the filtered list is intentionally
// not included — the UI layer resolves position from track.ID on the Update
// path, so the message is stable across filter changes that occur between
// the Cmd goroutine launch and the message delivery.
type playResultMsg struct {
	track *library.Track
	err   error
}

// noopMsg is returned by Cmds that have no state to communicate.
// Never return nil from a tea.Cmd — it causes a panic.
type noopMsg struct{}

// playlistsLoadedMsg carries the result of loading all playlists from the DB.
type playlistsLoadedMsg struct {
	playlists []store.Playlist
	err       error
}

// playlistTracksLoadedMsg carries the tracks for one playlist.
type playlistTracksLoadedMsg struct {
	tracks []library.Track
	err    error
}

// favoriteChangedMsg is sent after a Like/Unlike operation completes.
// isFavorite is the new state after the toggle.
type favoriteChangedMsg struct {
	trackID    string
	isFavorite bool
}

// chip8DoneMsg is sent by the background goroutine that runs p2chip.
// On success chipPath holds the converted mp3 path; err is non-nil on failure.
type chip8DoneMsg struct {
	originPath string // Track.Path of the source file
	chipPath   string // path to the generated 8-bit mp3
	err        error
}

// chipCrossfadeDoneMsg is sent after a chip-mode crossfade (on or off) completes.
// chipMode is the new desired state: true = now playing 8-bit, false = back to original.
type chipCrossfadeDoneMsg struct{ chipMode bool }

// lyricsLoadedMsg is sent by cmdLoadLyrics when lyrics have been fetched.
// lines is nil when no lyrics are available (not an error).
// err is non-nil when the fetch itself failed; the UI should handle it
// gracefully (e.g. display "No lyrics" rather than crashing).
type lyricsLoadedMsg struct {
	trackID string
	lines   []lyrics.Line
	err     error
}

// tick fires a tickMsg after tickInterval.
func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
