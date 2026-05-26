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
	// KeyLastTrackPath is the file path of the last played track.
	KeyLastTrackPath = "last_track_path"
	// KeyLastPositionMs is the playback position in milliseconds of the last track.
	KeyLastPositionMs = "last_position_ms"
	// KeyWasPlaying indicates whether the player was active at last exit ("1" = yes).
	KeyWasPlaying = "was_playing"
	// KeyCursor is the cursor position in the track list at last exit.
	KeyCursor = "cursor"
	// KeyChip8Options holds extra CLI options forwarded to p2chip.
	KeyChip8Options = "chip8_options"
	// KeyLanguage stores the UI display language ("en" or "zh").
	// An empty value or "en" means English; "zh" means Simplified Chinese.
	KeyLanguage = "language"
)
