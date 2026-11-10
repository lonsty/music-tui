package store

// Settings key constants used throughout the application.
// Using named constants prevents typos and makes rename-safe.
const (
	// KeyMusicDir is the root directory of the local music library.
	KeyMusicDir = "music_dir"
	// KeyVolume is the playback volume level [0.0, 2.0].
	KeyVolume = "volume"
	// KeyPlayMode is the current play mode (sequential/loop/single/random).
	KeyPlayMode = "play_mode"
	// KeyRetroIdx is the retro effect preset index.
	KeyRetroIdx = "retro_idx"
	// KeyLastTrackID is the stable ID (sha256-derived hex) of the last played track.
	KeyLastTrackID = "last_track_id"
	// KeyLastPositionMs is the playback position in milliseconds of the last track.
	KeyLastPositionMs = "last_position_ms"
	// KeyWasPlaying indicates whether the player was active at last exit.
	// Use ValWasPlayingYes / ValWasPlayingNo to read and write this value.
	KeyWasPlaying = "was_playing"
	// KeyChip8Options holds extra CLI options forwarded to p2chip.
	KeyChip8Options = "chip8_options"
	// KeyLanguage stores the UI display language.
	// Use ValLanguageEN / ValLanguageZH to read and write this value.
	KeyLanguage = "language"
	// KeyFormatPreference stores the audio format display preference.
	// Valid values are defined by the formatPreference enum in the tui package.
	KeyFormatPreference = "format_preference"
	// KeyRightCollapsed stores whether the right player panel is collapsed.
	// Use ValRightCollapsed / ValRightExpanded to read and write this value.
	KeyRightCollapsed = "right_collapsed"
	// KeyRightPanelMode stores the right panel display mode.
	// Valid values are defined by the rightPanelMode enum in the tui package.
	KeyRightPanelMode = "right_panel_mode"
	// KeyIconSet stores the icon rendering style.
	// Valid values are defined by the iconSet enum in the tui package.
	// Absent or unknown values default to ValIconSetNerdFont.
	KeyIconSet = "icon_set"
	// KeyLastActiveTab stores the numeric tabID of the last used tab.
	// It is written on every tab switch and restored on startup.
	KeyLastActiveTab = "last_active_tab"
	// KeyPlaylistDepth stores the depth within the Playlist tab.
	// "list" or "tracks".  Written on exit, restored on startup.
	KeyPlaylistDepth = "playlist_depth"
	// KeyPlaylistCursor stores the listCursor position in the playlist view.
	KeyPlaylistCursor = "playlist_cursor"
	// KeyPlaylistTrackCursor stores the trackCursor position within a playlist's track list.
	KeyPlaylistTrackCursor = "playlist_track_cursor"
	// KeyPlaylistID stores the currently open playlist ID when depth == tracks.
	KeyPlaylistID = "playlist_id"
	// KeyLyricsOffsetPrefix is the prefix for per-track lyrics timing offsets,
	// stored in milliseconds.  Append the Track.ID to form the full key.
	KeyLyricsOffsetPrefix = "lyrics_offset."
)

// Settings value constants for keys with a fixed set of valid values.
// Grouping values alongside their keys prevents scattered string literals.
const (
	// ValWasPlayingYes / ValWasPlayingNo are the two states for KeyWasPlaying.
	ValWasPlayingYes = "yes"
	ValWasPlayingNo  = "no"

	// ValLanguageEN / ValLanguageZH are the two states for KeyLanguage.
	ValLanguageEN = "en"
	ValLanguageZH = "zh"

	// ValRightCollapsed / ValRightExpanded are the two states for KeyRightCollapsed.
	ValRightCollapsed = "collapsed"
	ValRightExpanded  = "expanded"

	// ValIconSetNerdFont / ValIconSetEmoji / ValIconSetPlain are the three
	// states for KeyIconSet.
	ValIconSetNerdFont = "nerd"
	ValIconSetEmoji    = "emoji"
	ValIconSetPlain    = "plain"
)
