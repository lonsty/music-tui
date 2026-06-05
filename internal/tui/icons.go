package tui

// Package-level icon system.
//
// All Nerd Font glyphs, emoji, and plain-text fallbacks are defined here.
// Call the icon accessor functions (iconPlay, iconPause, …) from render code
// instead of embedding raw glyph literals — this makes it trivial to switch
// icon styles at runtime and keeps the font dependency explicit.
//
// Icon set selection
// ──────────────────
//   iconSetNerdFont (default) — Nerd Font private-use glyphs (U+E000–U+F8FF).
//     Requires a Nerd Font patched terminal font.
//   iconSetEmoji — Strictly single-column Unicode symbols (no wide emoji).
//     Works in any terminal without special fonts.
//   iconSetPlain — ASCII/empty fallbacks.
//     Maximum compatibility; some decorative icons become empty strings.
//
// Width contract
// ──────────────
// All Nerd Font glyphs used here render as exactly 1 terminal cell.
// All emoji alternatives are chosen to be 1 terminal cell wide.
// Plain alternatives are 0–2 ASCII characters; callers that hard-code
// icon width as 1 must switch to strWidth(iconXxx()) when Plain is active.

// iconSet controls which icon style is used for rendering.
type iconSet int

const (
	// iconSetNerdFont uses Nerd Font private-use glyphs.  Requires a patched
	// terminal font; glyphs are exactly 1 terminal cell wide.
	iconSetNerdFont iconSet = iota
	// iconSetEmoji uses strictly single-column Unicode symbols that work
	// without any special font installation.
	iconSetEmoji
	// iconSetPlain uses ASCII characters or empty strings for maximum
	// compatibility.  Decorative icons (library, lyrics, …) become "".
	iconSetPlain
	iconSetCount // sentinel — must be last
)

// activeIconSet is the process-wide icon style.  Change it with setIconSet.
var activeIconSet = iconSetNerdFont

// setIconSet changes the active icon set.  Safe to call at any time; the
// change takes effect on the next render tick.
func setIconSet(s iconSet) { activeIconSet = s }

// ActiveIconSet returns the currently active icon set.
func ActiveIconSet() iconSet { return activeIconSet }

// icon selects between three alternatives based on the active icon set.
// nf is the Nerd Font glyph, emoji is the single-column Unicode symbol, and
// plain is the ASCII fallback (may be empty).
func icon(nf, emoji, plain string) string {
	switch activeIconSet {
	case iconSetEmoji:
		return emoji
	case iconSetPlain:
		return plain
	default: // iconSetNerdFont
		return nf
	}
}

// ── Playback controls ─────────────────────────────────────────────────────────

// iconPlay returns the play glyph.
func iconPlay() string { return icon("󰐊", "▶", ">") }

// iconPause returns the pause glyph.
func iconPause() string { return icon("󰏤", "⏸", "=") }

// iconNext returns the skip-forward (next track) glyph.
func iconNext() string { return icon("󰒭", "⏭", ">>") }

// iconPrev returns the skip-backward (previous track) glyph.
func iconPrev() string { return icon("󰒮", "⏮", "<<") }

// ── Volume ────────────────────────────────────────────────────────────────────

// iconVolumeOn returns the audible-volume glyph.
func iconVolumeOn() string { return icon("󰕾", "♪", "V") }

// iconVolumeMute returns the muted-volume glyph.
func iconVolumeMute() string { return icon("󰖁", "✕", "M") }

// ── Play modes ────────────────────────────────────────────────────────────────

// iconPlayMode returns the glyph for the given playMode value.
func iconPlayMode(m playMode) string {
	switch m {
	case playModeSequential:
		return icon("󰒿", "→", "-")
	case playModeLoop:
		return icon("󰑖", "↺", "o")
	case playModeSingle:
		return icon("󰑘", "1", "1")
	case playModeRandom:
		return icon("󰒝", "⇄", "~")
	}
	return "?"
}

// ── Navigation / content ──────────────────────────────────────────────────────

// iconLibrary returns the local music library glyph.
func iconLibrary() string { return icon("󰋌", "♫", "") }

// iconOnline returns the online music tab glyph.
func iconOnline() string { return icon("󰖟", "⊕", "") }

// iconSearch returns the search prompt glyph.
func iconSearch() string { return icon("󰍉", "⌕", "/") }

// iconLyrics returns the lyrics / music-box glyph.
func iconLyrics() string { return icon("󰝚", "♩", "") }

// iconMusic returns the generic music / placeholder glyph.
func iconMusic() string { return icon("󰎄", "♪", "") }

// iconPlaying returns the "currently playing" indicator glyph shown in the
// track list next to the active row.
func iconPlaying() string { return icon("󰎆", "▶", ">") }

// iconSpinner returns the loading / spinner glyph.
func iconSpinner() string { return icon("󰔟", "⟳", "~") }

// iconLyricsSync returns the lyrics-sync / microphone glyph used in the
// online feature list.
func iconLyricsSync() string { return icon("󰍋", "♬", "") }

// ── Status / feedback ─────────────────────────────────────────────────────────

// iconError returns the error / alert-circle glyph.
func iconError() string { return icon("󰅚", "✗", "!") }

// iconHelp returns the help-circle glyph used as the help overlay title icon.
func iconHelp() string { return icon("󰋼", "?", "?") }

// iconInfo returns the information-circle glyph used as the info overlay title
// icon.
func iconInfo() string { return icon("󰋽", "i", "i") }

// iconPlaylist returns the playlist / queue glyph for the Playlists tab.
func iconPlaylist() string { return icon("󰲸", "♫", "") }

// iconHeartFilled returns the filled heart glyph (track is in Favorites).
// NF: nf-fa-heart  Emoji: ❤  Plain: <3
func iconHeartFilled() string { return icon("\uf004", "❤", "<3") }

// iconHeartEmpty returns the outline heart glyph (track is NOT in Favorites).
// NF: nf-fa-heart-o  Emoji: ♡  Plain: "" (invisible — only filled heart is shown)
func iconHeartEmpty() string { return icon("\uf08a", "♡", "") }

// iconWithSpace returns the icon followed by two spaces when the icon is
// non-empty, or an empty string when the icon is empty (Plain mode).
// Use this instead of icon() + "  " to avoid orphaned spaces in Plain mode.
func iconWithSpace(ic string) string {
	if ic == "" {
		return ""
	}
	return ic + "  "
}
