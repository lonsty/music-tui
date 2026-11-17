package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/audio"
	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/store"
)

var auditANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func auditLines(s string) []string {
	return strings.Split(auditANSI.ReplaceAllString(s, ""), "\n")
}

// P0: grouped views must not resolve flat-track-list actions against the
// hidden cursorPos.
func TestGroupScopeHasNoTracklistLeak(t *testing.T) {
	a := testApp(
		library.Track{ID: "t1", Title: "one", Album: "A"},
		library.Track{ID: "t2", Title: "two", Album: "A"},
		library.Track{ID: "t3", Title: "three", Album: "B"},
	)
	scopes := scopesForPage(a, scopeGroup)
	for _, s := range scopes {
		if s == scopeLocalTracks || s == scopeTracklist {
			t.Fatalf("group scope stack leaks %s", s)
		}
	}
	for _, key := range []string{"a", "i", "e", "E"} {
		if action, ok := registry.Resolve(scopes, runeMsg(key), a); ok {
			t.Errorf("group view still resolves %q to %s", key, action)
		}
	}
	if got := a.likeTarget(); got == nil || got.ID != "t1" {
		t.Fatalf("group like target = %v, want selected group's t1", got)
	}
}

// Chinese status bar must distinguish cursor movement from track change.
func TestChineseNavAndPlaybackLabelsDiffer(t *testing.T) {
	SetLang(LangZH)
	defer SetLang(LangEN)
	if T("hint_nav") == T("hint_next_prev") {
		t.Fatalf("j/k and n/p share Chinese label %q", T("hint_nav"))
	}
}

// Status bar must never exceed terminal width, even with a long status message.
func TestStatusBarFitsWidthWithStatusMessage(t *testing.T) {
	for _, w := range []int{40, 60, 80, 120} {
		a := testApp(library.Track{ID: "t1", Title: "one"})
		a.W, a.H = w, 30
		a.statusMsg = strings.Repeat("very long status message ", 8)
		a.framePlayback.state = audio.StatePlaying
		line := a.renderStatusBar()
		if got := strWidth(line); got > w {
			t.Errorf("W=%d status bar visible width %d", w, got)
		}
	}
}

// Playlist track headers must not wrap at narrow widths.
func TestHeaderDoesNotWrapAtNarrowWidth(t *testing.T) {
	a := testApp(
		library.Track{ID: "t1", Title: "one"},
		library.Track{ID: "t2", Title: "two"},
	)
	a.W, a.H = 40, 30
	a.activeTab = tabPlaylist
	a.depth = playlistDepthTracks
	a.playlists = []store.Playlist{{ID: "p", Name: "A Very Long Playlist Name That Keeps Going"}}
	a.plTracks = []library.Track{{ID: "t1", Title: "one"}, {ID: "t2", Title: "two"}}
	a.mqPlRow = NewMarquee("", marqueeSep)
	a.rightCollapsed = true
	out := auditANSI.ReplaceAllString(a.renderSharedTrackList(a.playlistTrackCtx(), 34, 8), "")
	lines := auditLines(out)
	if len(lines) < 3 {
		t.Fatal("too few header rows")
	}
	if !strings.Contains(lines[2], "─") {
		t.Fatalf("divider moved off row 3: %q", lines[2])
	}
}

// Help overlay must surface tracklist-only bindings.
func TestHelpIncludesTracklistBindings(t *testing.T) {
	a := &App{W: 100, H: 120, activeOvl: overlayHelp, searchInput: textinput.New()}
	out := auditANSI.ReplaceAllString(a.renderHelpOverlay(), "")
	for _, want := range []string{"❮e❯", "❮E❯", "PgDn", "❮[❯"} {
		if !strings.Contains(out, want) {
			t.Errorf("help overlay missing %q", want)
		}
	}
}

// ? inside a modal overlay must not destroy that overlay.
func TestOverlayHelpPreservesUnderlyingOverlay(t *testing.T) {
	a := testApp(library.Track{ID: "t1", Title: "one"})
	a.activeOvl = overlayAddToPlaylist
	a.handleKey(runeMsg("?"))
	if a.activeOvl != overlayHelp {
		t.Fatalf("? did not open help, activeOvl=%v", a.activeOvl)
	}
	a.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if a.activeOvl != overlayAddToPlaylist {
		t.Fatalf("Esc destroyed underlying overlay, activeOvl=%v", a.activeOvl)
	}
}

// Library search with no results must not append the unrelated Settings chip.
func TestSearchEmptyStateNoUnrelatedChip(t *testing.T) {
	a := testApp(library.Track{ID: "t1", Title: "one"})
	a.activeOvl = overlayNone
	a.searchInput.SetValue("zzzz")
	a.applyFilter()
	out := auditANSI.ReplaceAllString(a.renderSharedTrackList(a.localTrackListCtx(), 50, 10), "")
	if strings.Contains(out, "No results") && strings.Contains(out, "Settings") {
		t.Error("search-empty state shows unrelated Settings chip")
	}
}

// Expired undo must still give feedback.
func TestUndoExpiredStillGivesFeedback(t *testing.T) {
	a := testApp(library.Track{ID: "t1", Title: "one"})
	a.playlists = []store.Playlist{{ID: "p", Name: "P"}}
	a.plTracks = []library.Track{{ID: "t1", Title: "one"}}
	a.activeTab = tabPlaylist
	a.depth = playlistDepthTracks
	a.undoRemove = &undoRemove{expiresAt: time.Now().Add(-time.Second)}
	a.handleKey(runeMsg("u"))
	if a.statusMsg != T("undo_expired") {
		t.Fatalf("expired undo status = %q", a.statusMsg)
	}
}

// Add-tracks toggle label must not always say "add".
func TestAddTracksToggleLabelIsNeutral(t *testing.T) {
	a := testApp(library.Track{ID: "t1", Title: "one"})
	hints := a.footerHints(scopeInputAddTracks)
	if len(hints) == 0 {
		t.Fatal("no footer hints")
	}
	label := hints[0].label
	if !strings.Contains(label, "移除") && !strings.Contains(label, "Remove") {
		t.Errorf("toggle footer label %q never mentions removal", label)
	}
}

// Empty queue footer must not offer remove/clear.
func TestEmptyQueueFooterOnlyClose(t *testing.T) {
	a := testApp()
	hints := a.footerHints(scopeOverlayQueue)
	for _, h := range hints {
		if h.action == actionQueueRemove || h.action == actionQueueClear {
			t.Errorf("empty queue footer still offers %s", h.action)
		}
	}
}

// Settings section titles must exist and differ from field labels.
func TestSettingsSectionTitlesDiffer(t *testing.T) {
	for _, pair := range [][2]string{
		{"settings_section_lang", "settings_lang_label"},
		{"settings_section_fmt", "settings_fmt_label"},
		{"settings_section_icon", "settings_icon_set_label"},
	} {
		section, ok := translations[LangZH][pair[0]]
		if !ok || section == "" {
			t.Errorf("missing section key %s", pair[0])
			continue
		}
		if section == T(pair[1]) {
			t.Errorf("%s duplicates %s", pair[0], pair[1])
		}
	}
}

// LabelKeys must describe their action.
func TestLabelKeysDescribeActions(t *testing.T) {
	cases := map[ActionID]string{
		actionTrackInfo:   "信息",
		actionClearFilter: "清除",
		actionSeekLine:    "定位",
		actionMediaStop:   "停止",
	}
	SetLang(LangZH)
	defer SetLang(LangEN)
	for action, want := range cases {
		bd, ok := registry.bindingFor(action)
		if !ok {
			t.Errorf("missing binding %s", action)
			continue
		}
		if !strings.Contains(T(bd.LabelKey), want) {
			t.Errorf("%s label %q does not contain %q", action, T(bd.LabelKey), want)
		}
	}
}

// README generated tables must not repeat the same physical key.
func TestReadmeGeneratedBlockNoDuplicateKeys(t *testing.T) {
	old := ActiveLang()
	defer SetLang(old)
	SetLang(LangZH)
	block := ReadmeKeyTables()
	var table []string
	check := func() {
		seen := map[string]bool{}
		for _, line := range table {
			if !strings.HasPrefix(line, "| `") {
				continue
			}
			key := strings.SplitN(strings.TrimPrefix(line, "| `"), "`", 2)[0]
			if seen[key] {
				t.Errorf("README table repeats key %q", key)
			}
			seen[key] = true
		}
		table = nil
	}
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(line, "### ") {
			check()
			continue
		}
		table = append(table, line)
	}
	check()
}
