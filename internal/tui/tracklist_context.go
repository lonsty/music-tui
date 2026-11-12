package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
)

// TrackListContext describes the data and behaviour of one track-list panel.
// Both the Local tab and the Playlist tab's track level use the same rendering
// and key-handling engine (renderSharedTrackList / handleSharedTrackListKey);
// this struct is the only thing that differs between them.
type TrackListContext struct {
	// ── Data ─────────────────────────────────────────────────────────────────

	// Count returns the number of tracks currently displayed.
	Count func() int

	// TrackAt returns the track at position pos in the displayed list.
	// Returns a zero Track when pos is out of range.
	TrackAt func(pos int) library.Track

	// CursorPos is a pointer to the cursor index within the displayed list.
	CursorPos *int

	// PlayingPos returns the position of the track with the given ID inside
	// the displayed list, or -1 if the track is not present.
	// Used to show "N/total" in the header and to highlight the playing row.
	PlayingPos func(currentTrackID string) int

	// PageSize returns the number of rows moved by PageUp/PageDown.
	PageSize func() int

	// ── Rendering ─────────────────────────────────────────────────────────────

	// HeaderTitle returns the left-aligned title string of the panel header
	// (icon + label, already styled via lipgloss).
	HeaderTitle func() string

	// HeaderRight returns an optional right-aligned annotation appended after
	// the track count (e.g. "  ← Playlists" for the Playlist track level).
	// Return "" to omit.
	HeaderRight func() string

	// UseMarquee controls whether the selected row scrolls via mqRow.Render()
	// (true = Local) or is statically truncated (false = Playlist / Online).
	UseMarquee bool

	// ShowSearchBar controls whether the search action is available in this
	// context and the header is replaced by the search input while it is open.
	ShowSearchBar bool

	// ── Action callbacks ──────────────────────────────────────────────────────

	// OnEnter is called when the play action fires on a row.
	// pos is the position within the displayed list.
	OnEnter func(a *App, pos int) tea.Cmd

	// OnExtraKeys is called for any action the shared engine does not handle.
	// Return (true, cmd) to consume it; return (false, nil) to fall through.
	OnExtraKeys func(a *App, action ActionID, msg tea.KeyMsg) (handled bool, cmd tea.Cmd)

	// OnEsc is called when the cancel action fires with no overlay active.
	// Local: clears the search overlay.
	// Playlist track level: returns to the playlist list.
	OnEsc func(a *App)

	// SyncMarquee is called after every cursor movement to update the row
	// marquee text.  When nil, the default syncRowMarquee() is used (Local
	// tab behaviour: reads from filteredIdxs[cursorPos]).  Playlist tab must
	// supply its own implementation that reads from plTracks[trackCursor].
	SyncMarquee func(a *App)

	// LikeTarget returns the track the like action should toggle in Favorites.
	// Return nil to disable the Like action in this context.
	LikeTarget func(a *App) *library.Track

	// EmptyMsg is the i18n key shown when Count() returns zero.
	// Defaults to "no_tracks_hint" when empty.
	EmptyMsg string

	// EmptyHints lists actions whose chips are rendered in the empty state.
	// Chips come from the key registry, so changing a key needs no text edit.
	EmptyHints []ActionID
}
