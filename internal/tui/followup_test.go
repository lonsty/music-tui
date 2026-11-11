package tui

import (
	"strings"
	"testing"
)

func TestQuitRequiresSecondPress(t *testing.T) {
	a := &App{}
	if cmd := a.quitOrConfirm(); cmd != nil {
		t.Fatal("first q quit immediately")
	}
	if a.quitArmedAt.IsZero() {
		t.Fatal("first q did not arm quit")
	}
	if a.statusMsg != T("quit_confirm") {
		t.Fatalf("status = %q", a.statusMsg)
	}
	if cmd := a.quitOrConfirm(); cmd == nil {
		t.Fatal("second q did not quit")
	}
}

func TestOtherKeyDisarmsQuit(t *testing.T) {
	a := testApp()
	a.activeTab = tabLocal
	a.quitOrConfirm()
	a.handleKey(keyMsg("j"))
	if !a.quitArmedAt.IsZero() {
		t.Fatal("another key did not disarm quit")
	}
}

func TestAddPanelEmptyMessageIsLocalized(t *testing.T) {
	old := ActiveLang()
	defer SetLang(old)
	SetLang(LangZH)

	a := &App{}
	var sb strings.Builder
	a.renderAddTrackResults(&sb, 40, 5)
	out := sb.String()
	if !strings.Contains(out, "没有匹配") {
		t.Fatalf("add panel empty text = %q", out)
	}
	if strings.Contains(out, "按 a 添加") {
		t.Fatalf("add panel reused the playlist-empty hint: %q", out)
	}
}

func TestOverlayHintsAreLocalized(t *testing.T) {
	old := ActiveLang()
	defer SetLang(old)
	SetLang(LangZH)

	a := &App{W: 100, H: 30}
	for name, out := range map[string]string{
		"delete": a.renderDeletePlaylistOverlay(),
		"add":    a.renderAddToPlaylistOverlay(),
	} {
		if strings.Contains(out, "cancel") || strings.Contains(out, "confirm") {
			t.Fatalf("%s overlay leaks English hint:\n%s", name, out)
		}
		if !strings.Contains(out, "取消") {
			t.Fatalf("%s overlay missing localized cancel hint:\n%s", name, out)
		}
	}
}

func TestPlaylistBackHintPresent(t *testing.T) {
	if T("playlist_back_hint") == "" {
		t.Fatal("playlist back hint is empty")
	}
}
