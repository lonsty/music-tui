package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func runeMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestKeymapValidate(t *testing.T) {
	if err := registry.Validate(); err != nil {
		t.Fatalf("default registry is invalid: %v", err)
	}
}

func TestKeymapNewBindingAndMatches(t *testing.T) {
	kb, ok := keyBindingFor(actionVolumeUp)
	if !ok {
		t.Fatal("volume-up binding missing")
	}
	if !key.Matches(runeMsg("+"), kb) || !key.Matches(runeMsg("="), kb) {
		t.Fatal("multi-key binding does not match both keys")
	}
	if key.Matches(runeMsg("q"), kb) {
		t.Fatal("binding matched an unrelated key")
	}

	bd, _ := registry.bindingFor(actionPlaylistUndo)
	disabled := registry.keyBinding(bd, nil)
	disabled.SetEnabled(false)
	if disabled.Enabled() {
		t.Fatal("SetEnabled(false) did not disable the binding")
	}
	if key.Matches(runeMsg("u"), disabled) {
		t.Fatal("disabled binding still matched")
	}
}

func TestKeymapScopesResolveFirstMatch(t *testing.T) {
	a := testApp()
	esc := tea.KeyMsg{Type: tea.KeyEsc}

	action, ok := registry.Resolve([]Scope{scopeOverlayQueue, scopeGlobal}, esc, a)
	if !ok || action != actionQueueClose {
		t.Fatalf("queue Esc resolved to %q, want queue.close", action)
	}
	action, ok = registry.Resolve([]Scope{scopeLocalTracks, scopeTracklist, scopeGlobal}, esc, a)
	if !ok || action != actionCancel {
		t.Fatalf("tracklist Esc resolved to %q, want overlay.cancel", action)
	}
}

func TestKeymapValidationFailures(t *testing.T) {
	dup := newRegistry([]Binding{
		b(ActionID("test.one"), []string{"x"}, "x", "x", "", "", groupObject, 1, []Scope{"test.scope"}),
		b(ActionID("test.two"), []string{"x"}, "x", "x", "", "", groupObject, 1, []Scope{"test.scope"}),
	})
	if err := dup.Validate(); err == nil || !strings.Contains(err.Error(), "test.scope") {
		t.Fatalf("duplicate key error = %v, want scope context", err)
	}

	forbidden := newRegistry([]Binding{
		b(actionQuit, []string{"ctrl+i"}, "Ctrl+I", "Ctrl+I", "", "", groupSystem, 1, []Scope{scopeGlobal}),
	})
	if err := forbidden.Validate(); err == nil || !strings.Contains(err.Error(), "ctrl+i") {
		t.Fatalf("forbidden key error = %v, want ctrl+i", err)
	}

	missing := newRegistry([]Binding{
		b(actionQuit, []string{"q"}, "q", "q", "missing_label_key_for_test", "", groupSystem, 1, []Scope{scopeGlobal}),
	})
	if err := missing.Validate(); err == nil || !strings.Contains(err.Error(), "missing_label_key_for_test") {
		t.Fatalf("missing translation error = %v, want key name", err)
	}
}

func TestKeymapFinalKeyTable(t *testing.T) {
	a := testApp()
	scopes := []Scope{scopeLocalTracks, scopePlaylistTracks, scopePlaylistList,
		scopeOverlayAddList, scopeTracklist, scopeGroup, scopeGlobal}

	for _, k := range []string{"v", "Q", "z"} {
		if action, ok := registry.Resolve(scopes, runeMsg(k), a); ok {
			t.Errorf("legacy key %q still bound to %s", k, action)
		}
	}

	cases := []struct {
		msg  tea.KeyMsg
		want ActionID
	}{
		{runeMsg("1"), actionViewTracks},
		{runeMsg("2"), actionViewAlbums},
		{runeMsg("3"), actionViewArtists},
		{runeMsg("4"), actionQueueOpen},
		{runeMsg("e"), actionQueueAppend},
		{runeMsg("E"), actionQueueInsert},
		{runeMsg("N"), actionPlaylistNew},
		{tea.KeyMsg{Type: tea.KeyF2}, actionPlaylistRename},
		{runeMsg("n"), actionNext},
	}
	for _, tc := range cases {
		got, ok := registry.Resolve(scopes, tc.msg, a)
		if !ok || got != tc.want {
			t.Errorf("key %q resolved to %q, want %s", tc.msg.String(), got, tc.want)
		}
	}

	// Comma is settings in the main view and lyric offset in fullscreen.
	if got, ok := registry.Resolve([]Scope{scopeLocalTracks, scopeTracklist, scopeGlobal}, runeMsg(","), a); !ok || got != actionSettingsOpen {
		t.Errorf("main-view comma resolved to %q, want settings.open", got)
	}
	if got, ok := registry.Resolve([]Scope{scopeFullscreen, scopeGlobal}, runeMsg(","), a); !ok || got != actionOffsetEarlier {
		t.Errorf("fullscreen comma resolved to %q, want lyrics.offset_earlier", got)
	}

	// In track lists comma opens settings and dot is unbound; only brackets
	// adjust the lyric offset there.
	list := []Scope{scopeLocalTracks, scopeTracklist, scopeGlobal}
	if got, ok := registry.Resolve(list, runeMsg(","), a); !ok || got != actionSettingsOpen {
		t.Errorf("list comma resolved to %q, want settings.open", got)
	}
	if action, ok := registry.Resolve(list, runeMsg("."), a); ok {
		t.Errorf("list dot resolved to %q, want no binding", action)
	}
	if got, ok := registry.Resolve(list, runeMsg("["), a); !ok || got != actionOffsetEarlier {
		t.Errorf("list bracket resolved to %q, want lyrics.offset_earlier", got)
	}
}

func TestKeymapHelpKeyMap(t *testing.T) {
	a := testApp()
	a.activeTab = tabLocal
	km := a.keyMapFor(scopeLocalTracks)
	if got := len(km.ShortHelp()); got == 0 || got > 6 {
		t.Fatalf("ShortHelp returned %d bindings, want 1..6", got)
	}
	if got := len(km.FullHelp()); got != 5 {
		t.Fatalf("FullHelp returned %d sections, want 5", got)
	}
}
