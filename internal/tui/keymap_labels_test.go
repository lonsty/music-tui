package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/lonsty/music-tui/internal/store"
)

var labelANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func cleanLines(s string) []string {
	return strings.Split(labelANSI.ReplaceAllString(s, ""), "\n")
}

func descOf(action ActionID) string {
	bd, ok := registry.bindingFor(action)
	if !ok {
		return ""
	}
	return T(bd.DescKey)
}

// Issue 2 and 8: opposing key pairs must have distinct descriptions.
func TestPairedActionsHaveDistinctDescriptions(t *testing.T) {
	pairs := [][2]ActionID{
		{actionNext, actionPrev},
		{actionSeekBack, actionSeekForward},
		{actionVolumeUp, actionVolumeDown},
		{actionLofiDown, actionLofiUp},
		{actionOffsetEarlier, actionOffsetLater},
		{actionNavTop, actionNavBottom},
		{actionNavPageDown, actionNavPageUp},
		{actionQueueAppend, actionQueueInsert},
		{actionPlaylistMoveDown, actionPlaylistMoveUp},
		{actionMediaPrev, actionMediaNext},
		{actionMediaVolDown, actionMediaVolUp},
		{actionBrowseUp, actionBrowseDown},
		{actionSettingsScrollUp, actionSettingsScrollDown},
	}
	for _, lang := range []Lang{LangEN, LangZH} {
		SetLang(lang)
		for _, pair := range pairs {
			a, b := descOf(pair[0]), descOf(pair[1])
			if a == "" || b == "" || a == b {
				t.Errorf("%v: %s and %s share description %q", lang, pair[0], pair[1], a)
			}
		}
	}
	SetLang(LangEN)
}

// Issues 1, 7 and 8: status-bar chips must name the action they represent.
func TestStatusBarChipLabelsMatchActions(t *testing.T) {
	SetLang(LangZH)
	defer SetLang(LangEN)
	a := testApp()
	hints := a.shortHints(scopeLocalTracks)
	labels := map[ActionID]string{}
	for _, h := range hints {
		labels[h.action] = h.label
	}
	if labels[actionNavDown] != "下/上一首" {
		t.Errorf("j/k label = %q, want 下/上一首", labels[actionNavDown])
	}

	full := a.shortHints(scopeFullscreen)
	fullLabels := map[ActionID]string{}
	for _, h := range full {
		fullLabels[h.action] = h.label
	}
	if fullLabels[actionBrowseDown] != "浏览歌词" {
		t.Errorf("fullscreen ↑/↓ label = %q, want 浏览歌词", fullLabels[actionBrowseDown])
	}
	if fullLabels[actionSeekForward] == "" || fullLabels[actionOffsetLater] == "" ||
		fullLabels[actionSeekForward] == fullLabels[actionOffsetLater] {
		t.Errorf("seek and offset chips share label %q", fullLabels[actionSeekForward])
	}
}

// Issue 4: track info must work in the playlist track list.
func TestTrackInfoInPlaylistTracks(t *testing.T) {
	a := testApp()
	scopes := []Scope{scopePlaylistTracks, scopeTracklist, scopeGlobal}
	got, ok := registry.Resolve(scopes, runeMsg("i"), a)
	if !ok || got != actionTrackInfo {
		t.Fatalf("playlist tracks i resolved to %q, want library.track_info", got)
	}
}

// Issue 3: grouped views keep the collapsed play status in the header.
func TestGroupViewCollapsedPlayStatus(t *testing.T) {
	a := newGoldenApp(t)
	a.view = libraryViewAlbums
	a.rightCollapsed = true
	a.rebuildGroups()
	out := labelANSI.ReplaceAllString(a.renderGroupList(50, 10), "")
	if !strings.Contains(out, "Blue in Green") {
		t.Fatal("grouped view lost the collapsed playback status")
	}
}

// Issue 5: every empty-state line is centred individually.
func TestEmptyStateLinesIndividuallyCentered(t *testing.T) {
	a := newGoldenApp(t)
	SetLang(LangZH)
	defer SetLang(LangEN)
	a.activeTab = tabPlaylist
	a.depth = playlistDepthTracks
	a.plTracks = nil
	a.playlists = []store.Playlist{{ID: "p", Name: "P"}}
	out := labelANSI.ReplaceAllString(a.renderSharedTrackList(a.playlistTrackCtx(), 50, 10), "")
	for _, ln := range cleanLines(out) {
		if strings.Contains(ln, "─") || strings.Contains(ln, "╭") || strings.Contains(ln, "╰") {
			continue
		}
		body := strings.TrimPrefix(strings.TrimSuffix(ln, "│"), "│")
		trimmed := strings.TrimSpace(body)
		if trimmed == "" ||
			(!strings.Contains(trimmed, "暂无曲目") && !strings.Contains(trimmed, "❮a❯")) {
			continue
		}
		left := strings.Index(body, trimmed)
		right := strWidth(body) - left - strWidth(trimmed)
		if left < right-1 || left > right+1 {
			t.Errorf("empty-state line %q not centred: left %d, right %d", trimmed, left, right)
		}
	}
}

// Issue 6: the input footer sits above the results in both search panels.
func TestSearchInputFootersSharePosition(t *testing.T) {
	search := newGoldenApp(t)
	search.activeOvl = overlaySearch
	search.searchInput.SetValue("blue")
	searchLines := cleanLines(labelANSI.ReplaceAllString(
		search.renderSharedTrackList(search.localTrackListCtx(), 50, 10), ""))
	searchFooter := lineContaining(searchLines, "❮Enter❯")
	searchResults := lineContaining(searchLines, "Kind of Blue")
	if searchFooter < 0 || searchResults < 0 || searchFooter >= searchResults {
		t.Fatalf("library search footer %d not above results %d", searchFooter, searchResults)
	}

	add := newGoldenApp(t)
	add.activeTab = tabPlaylist
	add.depth = playlistDepthTracks
	add.addingTracks = true
	add.addInput = textinput.New()
	addLines := cleanLines(labelANSI.ReplaceAllString(add.renderPlaylistAddPanel(50, 10), ""))
	addFooter := lineContaining(addLines, "❮Enter❯")
	addResults := lineContaining(addLines, "No matching tracks")
	if addFooter < 0 || addResults < 0 || addFooter >= addResults {
		t.Fatalf("add-tracks footer %d not above results %d", addFooter, addResults)
	}
}

func lineContaining(lines []string, needle string) int {
	for i, ln := range lines {
		if strings.Contains(ln, needle) {
			return i
		}
	}
	return -1
}
