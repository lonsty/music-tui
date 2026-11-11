package tui

import (
	"time"
)

// ── Layout helpers ────────────────────────────────────────────────────────────

const (
	topPad     = 0 // no top padding — altscreen starts at row 0
	tabBarH    = 1 // single line for tab labels (no separator row)
	statusBarH = 1
	borderH    = 2
	borderW    = 2

	// Layout proportions and thresholds.
	miniPlayerMinWidth   = 60   // terminal width below which the mini player is hidden
	trackListWidthRatio  = 0.55 // fraction of total width allocated to the track list
	trackListMinWidth    = 20   // minimum track list width in columns
	fullPlayerWidthRatio = 0.40 // fraction of total width allocated to the full-screen player panel
	fullPlayerMinWidth   = 24   // minimum full-screen player width in columns
	coverMinSize         = 4    // minimum cover art outerRows value

	// marqueeSep is the separator inserted between repetitions of scrolling text.
	marqueeSep = "  •  "

	// statusMsgDuration is how long a status-bar message stays visible before
	// it is automatically cleared.
	statusMsgDuration = 2 * time.Second
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

// trackPageSize is the number of rows moved by PageUp/PageDown: one viewport
// of track rows (panel height minus the header and divider).
func (a *App) trackPageSize() int {
	n := a.panelInnerH() - 2
	if n < 1 {
		return 1
	}
	return n
}

// showMiniPlayer returns true when the terminal is wide enough and the right
// panel is not collapsed.
func (a *App) showMiniPlayer() bool { return !a.rightCollapsed && a.W >= miniPlayerMinWidth }

// trackListOuterW returns the outer (border-inclusive) width of the track list.
func (a *App) trackListOuterW() int {
	if !a.showMiniPlayer() {
		return a.W
	}
	w := int(float64(a.W) * trackListWidthRatio)
	if w < trackListMinWidth {
		return trackListMinWidth
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
	w := int(float64(a.W) * fullPlayerWidthRatio)
	if w < fullPlayerMinWidth {
		return fullPlayerMinWidth
	}
	return w
}
func (a *App) fullLyricsOuterW() int { return a.W - a.fullPlayerOuterW() }
func (a *App) fullPlayerInnerW() int { return a.fullPlayerOuterW() - borderW }
func (a *App) fullLyricsW() int      { return a.fullLyricsOuterW() - borderW }
