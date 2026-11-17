package tui

import (
	"os"
	"path/filepath"
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

// ── Dispatch ──────────────────────────────────────────────────────────────────

func TestDispatchFullscreenHelpAndEsc(t *testing.T) {
	a := testApp()
	a.currentView = viewFullscreen
	a.W, a.H = 100, 60

	a.handleKey(runeMsg("?"))
	if a.activeOvl != overlayHelp {
		t.Fatal("help overlay did not open above fullscreen")
	}
	if !strings.Contains(a.render(), T("help_title")) {
		t.Fatal("help overlay is not rendered in fullscreen")
	}

	a.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if a.activeOvl != overlayNone {
		t.Fatal("Esc did not close the help overlay")
	}
	if a.currentView != viewFullscreen {
		t.Fatal("Esc also exited fullscreen")
	}
}

func TestDispatchTextInputPassthrough(t *testing.T) {
	a := testApp()
	a.nameInput = textinput.New()
	a.inputMode = playlistInputCreate
	a.nameInput.Focus()

	for _, k := range []string{"q", "j", "k"} {
		a.handleKey(runeMsg(k))
	}
	if got := a.nameInput.Value(); got != "qjk" {
		t.Fatalf("name input = %q, want qjk", got)
	}

	b := testApp()
	b.addInput = textinput.New()
	b.addingTracks = true
	b.addInput.Focus()
	for _, k := range []string{"q", "j", "k"} {
		b.handleKey(runeMsg(k))
	}
	if got := b.addInput.Value(); got != "qjk" {
		t.Fatalf("add-tracks input = %q, want qjk", got)
	}

	// Media keys must not fire while typing.
	if action, ok := a.resolveAction(tea.KeyMsg{Type: tea.KeyF9}); ok {
		t.Fatalf("F9 resolved to %s inside a text input", action)
	}
}

func TestDispatchSettingsEditReload(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	dir := t.TempDir()
	a := testApp()
	a.st = st
	a.musicDir = dir
	a.activeOvl = overlaySettings
	a.settingsEditing = true
	a.musicDirInput = textinput.New()
	a.settingsInput = textinput.New()
	a.musicDirInput.SetValue(dir)
	a.handleKey(tea.KeyMsg{Type: tea.KeyCtrlR})
	if a.activeOvl != overlaySettings || a.settingsEditing {
		t.Fatal("Ctrl+R in settings edit did not save while keeping the overlay open")
	}
	if !a.loading {
		t.Fatal("Ctrl+R in settings edit did not start the rescan")
	}
}

func TestDispatchGroupLikeTarget(t *testing.T) {
	a := testApp(
		library.Track{ID: "t1", Title: "one", Album: "A"},
		library.Track{ID: "t2", Title: "two", Album: "A"},
	)
	a.activeTab = tabLocal
	a.view = libraryViewAlbums
	a.groupCursor = 0
	a.rebuildGroups()
	if got := a.likeTarget(); got == nil || got.ID != "t1" {
		t.Fatalf("group like target = %v, want t1", got)
	}
}

func TestDispatchModalAndQueueOpen(t *testing.T) {
	a := testApp()
	a.activeOvl = overlayQueue
	if action, ok := a.resolveAction(runeMsg("n")); ok {
		t.Fatalf("queue overlay leaked global action %s", action)
	}
	if action, ok := a.resolveAction(runeMsg("Q")); ok {
		t.Fatalf("legacy Q resolved to %s", action)
	}

	a.activeOvl = overlayNone
	a.handleKey(runeMsg("4"))
	if a.activeOvl != overlayQueue {
		t.Fatal("4 did not open the queue overlay")
	}
	a.activeOvl = overlayNone
	a.activeTab = tabPlaylist
	if action, ok := a.resolveAction(runeMsg("n")); !ok || action != actionNext {
		t.Fatalf("n in playlist list resolved to %q, want playback.next", action)
	}
}

// ── Display ───────────────────────────────────────────────────────────────────

func TestHintsSixSlotsAndDrop(t *testing.T) {
	a := testApp()
	a.activeTab = tabLocal
	hints := a.shortHints(scopeLocalTracks)
	if len(hints) != 6 {
		t.Fatalf("short hints = %d, want 6", len(hints))
	}
	if hints[0].action != actionHelp || !hints[0].pinned {
		t.Fatal("slot 0 is not pinned help")
	}
	if hints[5].action != actionQuit || !hints[5].pinned {
		t.Fatal("slot 5 is not pinned quit")
	}
	a.framePlayback.state = audio.StatePlaying
	groupHints := a.shortHints(scopeGroup)
	if len(groupHints) < 2 || groupHints[1].action != actionDrillDown {
		t.Fatal("group primary slot was replaced by a playback action")
	}
	a.framePlayback.state = audio.StateStopped

	// Narrow terminal drops slot 4, then 3, then 2.
	kept := dropShortHints(hints, hintWidth, 0)
	if len(kept) != 3 {
		t.Fatalf("pinned hints = %d, want 3", len(kept))
	}
	for _, h := range kept {
		if !h.pinned {
			t.Fatalf("non-pinned hint %s survived", h.action)
		}
	}
}

func TestEveryShortSlotIsBoundOnItsPage(t *testing.T) {
	a := testApp()
	for scope, slots := range shortSlots {
		pageScopes := scopesForPage(a, scope)
		available := map[Scope]bool{}
		for _, s := range pageScopes {
			available[s] = true
		}
		for _, action := range slots {
			bound := false
			for _, idx := range registry.byAction[action] {
				for _, s := range registry.bindings[idx].Scopes {
					if available[s] {
						bound = true
					}
				}
			}
			if !bound {
				t.Errorf("scope %s shows %s but no registry binding is reachable", scope, action)
			}
		}
	}
}

func scopesForPage(a *App, scope Scope) []Scope {
	a.currentView = viewNormal
	a.activeOvl = overlayNone
	a.settingsEditing = false
	a.inputMode = playlistInputNone
	a.addingTracks = false
	a.overlayCreating = false
	switch scope {
	case scopeLocalTracks:
		a.activeTab, a.view = tabLocal, libraryViewTracks
	case scopeGroup, scopeLocalGroups:
		a.activeTab, a.view = tabLocal, libraryViewAlbums
	case scopePlaylistList:
		a.activeTab, a.depth = tabPlaylist, playlistDepthList
	case scopePlaylistTracks:
		a.activeTab, a.depth = tabPlaylist, playlistDepthTracks
	case scopeFullscreen:
		a.currentView = viewFullscreen
	}
	return a.activeScopes()
}

func TestDispatchPlaylistListNavigation(t *testing.T) {
	a := testApp()
	a.mqPlRow = NewMarquee("", marqueeSep)
	a.activeTab = tabPlaylist
	a.depth = playlistDepthList
	a.playlists = []store.Playlist{{ID: "p1", Name: "one"}, {ID: "p2", Name: "two"}}
	a.listCursor = 0

	a.handleKey(runeMsg("j"))
	if a.listCursor != 1 {
		t.Fatalf("j did not move the playlist cursor: %d", a.listCursor)
	}
	a.handleKey(runeMsg("k"))
	if a.listCursor != 0 {
		t.Fatalf("k did not move the playlist cursor back: %d", a.listCursor)
	}
	a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if a.listCursor != 1 {
		t.Fatalf("G did not jump to the last playlist: %d", a.listCursor)
	}
}

func TestDispatchGroupActionsResolve(t *testing.T) {
	a := testApp(
		library.Track{ID: "t1", Title: "one", Album: "A"},
		library.Track{ID: "t2", Title: "two", Album: "A"},
	)
	scopes := scopesForPage(a, scopeGroup)
	// Flat track-list actions must not leak into grouped views: they would
	// operate on the hidden cursorPos instead of the selected group.
	for _, key := range []string{"a", "i", "e", "E"} {
		if got, ok := registry.Resolve(scopes, runeMsg(key), a); ok {
			t.Errorf("group key %q leaked flat action %s", key, got)
		}
	}
	if got, ok := registry.Resolve(scopes, runeMsg("L"), a); !ok || got != actionLike {
		t.Errorf("group like action resolved to %q, want library.like", got)
	}
}

func TestRegistryFootersAndFilterChipAreRendered(t *testing.T) {
	a := &App{W: 100, H: 60, activeOvl: overlayHelp,
		searchInput: textinput.New()}
	help := a.renderHelpOverlay()
	for _, want := range []string{"↑/↓", "Esc"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help footer missing %q", want)
		}
	}

	search := testApp(
		library.Track{ID: "t1", Title: "one"},
		library.Track{ID: "t2", Title: "two"},
	)
	search.W, search.H = 100, 30
	search.activeOvl = overlaySearch
	out := search.renderSharedTrackList(search.localTrackListCtx(), 60, 12)
	for _, want := range []string{"❮Enter❯", "❮Esc❯"} {
		if !strings.Contains(out, want) {
			t.Fatalf("search input footer missing %q", want)
		}
	}

	search.activeOvl = overlayNone
	search.searchInput.SetValue("one")
	filtered := search.renderSharedTrackList(search.localTrackListCtx(), 60, 12)
	if !strings.Contains(filtered, "❮Esc❯") {
		t.Fatal("active filter header missing the registry clear-filter chip")
	}
}

func TestFooterEscLast(t *testing.T) {
	a := testApp()
	for s, actions := range footerSlots {
		if len(actions) == 0 {
			continue
		}
		hints := a.footerHints(s)
		if len(hints) == 0 {
			t.Errorf("scope %s has no footer hints", s)
			continue
		}
		if got := hints[len(hints)-1].key; got != "Esc" {
			t.Errorf("scope %s last chip = %q, want Esc", s, got)
		}
	}
}

func TestHelpSectionsOrderAndDedup(t *testing.T) {
	a := testApp()
	a.activeTab = tabLocal
	sections := a.helpSectionsRendered(scopeLocalTracks)
	if len(sections) != 5 {
		t.Fatalf("help sections = %d, want 5", len(sections))
	}
	seen := map[ActionID]bool{}
	for _, sec := range sections[:4] {
		for _, row := range sec.rows {
			if seen[row.action] {
				t.Fatalf("duplicate help row %s", row.action)
			}
			seen[row.action] = true
		}
	}
	if len(sections[4].rows) != 6 {
		t.Fatalf("media section rows = %d, want 6", len(sections[4].rows))
	}
	for _, row := range sections[1].rows {
		if mediaActions[row.action] {
			t.Fatalf("global section duplicates media action %s", row.action)
		}
	}
}

func TestShortHintsStable(t *testing.T) {
	a := testApp()
	a.activeTab = tabLocal
	first := a.shortHints(scopeLocalTracks)
	second := a.shortHints(scopeLocalTracks)
	if len(first) != len(second) {
		t.Fatal("hint count changed between renders")
	}
	for i := range first {
		if first[i].action != second[i].action || first[i].key != second[i].key {
			t.Fatalf("hint %d changed between renders", i)
		}
	}
}

func TestEmptyHintsFromRegistry(t *testing.T) {
	a := testApp()
	ctx := a.localTrackListCtx()
	if len(ctx.EmptyHints) == 0 {
		t.Fatal("EmptyHints is empty")
	}
	bd, ok := registry.bindingFor(ctx.EmptyHints[0])
	if !ok {
		t.Fatal("EmptyHints references an unknown action")
	}
	out := a.renderSharedTrackList(ctx, 60, 10)
	if !strings.Contains(out, bd.ShortKey) {
		t.Fatalf("empty state does not render the registry chip %q", bd.ShortKey)
	}
}

func TestEnabledHints(t *testing.T) {
	a := testApp()
	a.activeTab = tabPlaylist
	a.depth = playlistDepthTracks
	// Undo stays bound even when expired: the handler reports "undo expired"
	// instead of silently dropping the key.
	for _, expired := range []bool{false, true} {
		expires := time.Now().Add(time.Second)
		if expired {
			expires = time.Now().Add(-time.Second)
		}
		a.undoRemove = &undoRemove{expiresAt: expires}
		scopes := []Scope{scopePlaylistTracks, scopeTracklist, scopeGlobal}
		got, ok := registry.Resolve(scopes, runeMsg("u"), a)
		if !ok || got != actionPlaylistUndo {
			t.Fatalf("expired=%v undo resolved to %q", expired, got)
		}
	}
}

// ── Consistency ───────────────────────────────────────────────────────────────

var specActionID = regexp.MustCompile("`([a-z][a-z0-9_]*\\.[a-z0-9_.]+)`")

func TestConsistencySpecActionIDs(t *testing.T) {
	files, err := filepath.Glob("../../openspec/changes/redesign-shortcut-system/specs/*/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 6 {
		t.Fatalf("delta spec files = %d, want 6", len(files))
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range specActionID.FindAllStringSubmatch(string(data), -1) {
			if _, ok := registry.bindingFor(ActionID(m[1])); !ok {
				t.Errorf("%s references unknown ActionID %q", path, m[1])
			}
		}
	}
}

func TestConsistencyNoHardcodedChips(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	hardcoded := regexp.MustCompile(`"❮[A-Za-z]`)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if hardcoded.Match(data) {
			t.Errorf("%s contains a hardcoded key chip", name)
		}
	}
}

func TestConsistencyReadmeGeneratedBlock(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	const begin = "<!-- BEGIN GENERATED KEYS -->"
	const end = "<!-- END GENERATED KEYS -->"
	i := strings.Index(src, begin)
	j := strings.Index(src, end)
	if i < 0 || j < 0 || j < i {
		t.Fatal("README missing generated-key markers")
	}
	got := strings.TrimSpace(src[i+len(begin) : j])

	old := ActiveLang()
	defer SetLang(old)
	SetLang(LangZH)
	want := strings.TrimSpace(ReadmeKeyTables())
	if got != want {
		t.Fatal("README generated block is stale; run go generate ./...")
	}
}

var staleKeyComment = regexp.MustCompile("`v`|`Q`|`z`|m/b/r/R|j/k/g/G|Spc|PgUp / PgDn|q-key|Tab-cycling|Ctrl\\+R →|按 n 新建|press q|❮[A-Za-z]")

func TestConsistencyCommentsHaveNoStaleKeys(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if staleKeyComment.MatchString(line) {
				t.Errorf("%s:%d comment mentions a stale key: %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
