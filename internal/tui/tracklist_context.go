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

	// Tracks returns the currently displayed track slice (already filtered /
	// ordered).  Called on every render; must be cheap.
	Tracks func() []library.Track

	// CursorPos is a pointer to the cursor index within the Tracks() slice.
	CursorPos *int

	// TotalCount returns the display count shown in the header (e.g. the
	// filtered length for Local, or the DB count for Playlist).
	TotalCount func() int

	// PlayingPos returns the position of the track with the given ID inside
	// Tracks(), or -1 if the track is not present in the current list.
	// Used to show "N/total" in the header and to highlight the playing row.
	PlayingPos func(currentTrackID string) int

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

	// ShowSearchBar controls whether the "/" key activates the search overlay
	// and the header is replaced by the search input while it is open.
	ShowSearchBar bool

	// ── Key callbacks ─────────────────────────────────────────────────────────

	// OnEnter is called when the user presses Enter on a row.
	// pos is the position within Tracks().
	OnEnter func(a *App, pos int) tea.Cmd

	// OnExtraKeys is called for any key that the shared engine does not handle.
	// Return (true, cmd) to consume the key; return (false, nil) to let the
	// engine fall through to its default no-op.
	OnExtraKeys func(a *App, msg tea.KeyMsg) (handled bool, cmd tea.Cmd)

	// OnEsc is called when Esc is pressed and no overlay is active.
	// Local: clears the search overlay.
	// Playlist track level: returns to the playlist list.
	OnEsc func(a *App)

	// SyncMarquee is called after every cursor movement to update the row
	// marquee text.  When nil, the default syncRowMarquee() is used (Local
	// tab behaviour: reads from filteredIdxs[cursorPos]).  Playlist tab must
	// supply its own implementation that reads from plTracks[trackCursor].
	SyncMarquee func(a *App)

	// LikeTarget returns the track that the "L" key should toggle in Favorites.
	// Return nil to disable the Like action in this context.
	LikeTarget func(a *App) *library.Track

	// ── Status-bar hints ──────────────────────────────────────────────────────

	// ExtraHints returns context-specific hints appended after the shared
	// ones.  Return nil when no extra hints are needed.
	ExtraHints func(a *App) []statusHint

	// EmptyMsg is the i18n key shown when Tracks() returns an empty slice.
	// Defaults to "no_tracks_hint" when empty.
	EmptyMsg string
}
