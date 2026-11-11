package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/lyrics"
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
	identityIdxs []int            // cached 0..len(tracks)-1 slice for the unfiltered view
	posByID      map[string]int   // Track.ID → position in filteredIdxs (O(1) lookup)
	trackPosByID map[string]int   // Track.ID → position in tracks (O(1) lookup)
	shuffleIDs   []string         // Track.ID values in shuffled order (random play mode)
	cursorPos    int              // position in filteredIdxs of the cursor-selected track
	formatPref   formatPreference // active format-display preference
	lastQuery    string           // last applied search query; used to detect query changes

	// Grouped browsing.  The group caches are rebuilt lazily whenever tracks or
	// the format preference change; render and key handling only read them.
	view         libraryView
	prevView     libraryView // view to restore when a search filter is cleared
	groupCursor  int
	albumGroups  []trackGroup
	artistGroups []trackGroup
	groupsBuilt  bool
	groupsDirty  bool
	groupsPref   formatPreference // format preference the group caches were built with
	drilldown    *groupRef        // non-nil while showing one album/artist's tracks
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

// undoRemove records a removed playlist track so it can be restored within
// the undo window.
type undoRemove struct {
	playlistID string
	trackID    string
	position   int
	title      string
	expiresAt  time.Time
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

	// undoRemove is the most recent track removal, kept for the 5-second
	// undo window.
	undoRemove *undoRemove

	// plListMaxColW caches the max display width of the right-column
	// track count across all playlists.  Recomputed in playlistsLoadedMsg
	// so the render loop never queries the DB.
	plListMaxColW int

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

// OverlayState holds transient UI state for panels layered over the playlist
// view: the add-to-playlist picker and the inline add-tracks search panel.
type OverlayState struct {
	// addingTracks is true while the inline "add tracks" search panel is open.
	addingTracks bool
	addInput     textinput.Model // search box inside the add-tracks panel
	addResults   []int           // indices into a.tracks that match addInput
	addCursor    int             // cursor in addResults
	addDelta     map[string]int  // net change per track: +1 added, -1 removed, 0 cancelled

	// overlayCreating is true while the add-to-playlist overlay shows its
	// inline "create playlist" input.  ovlNewTrackID remembers which track
	// should be added to the newly created playlist.
	overlayCreating bool
	ovlNewTrackID   string

	// ovlPlaylists is the cached playlist list used by overlayAddToPlaylist.
	// It is loaded when the overlay opens.
	ovlPlaylists []store.Playlist
	ovlPlCursor  int // cursor inside the overlay list
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
	offset        time.Duration   // per-track timing offset; positive delays lyrics
	maxTextW      int             // widest lyric line, computed once per load

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
