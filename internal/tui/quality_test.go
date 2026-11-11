package tui

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/lonsty/music-tui/internal/audio"
	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/store"
)

func TestApplyFormatPreferenceDoesNotMutateInput(t *testing.T) {
	tracks := []library.Track{
		{ID: "1", Title: "one", FileFormat: "MP3"},
		{ID: "2", Title: "two", FileFormat: "FLAC"},
		{ID: "3", Title: "three", FileFormat: "MP3"},
	}

	for _, pref := range []formatPreference{formatPrefLosslessOnly, formatPrefMP3Only, formatPrefLosslessFirst} {
		before := append([]library.Track(nil), tracks...)
		_ = applyFormatPreference(tracks, pref)
		for i := range tracks {
			if tracks[i].ID != before[i].ID {
				t.Fatalf("pref %v mutated input at %d: got %q, want %q", pref, i, tracks[i].ID, before[i].ID)
			}
		}
	}
}

// TestRebuildGroupsKeepsLibraryOrder is the regression test for the in-place
// filtering bug: rebuildGroups used to pass a.tracks to a filter that reused
// its backing array, corrupting album order for lossless-only / MP3-only.
func TestRebuildGroupsKeepsLibraryOrder(t *testing.T) {
	tracks := []library.Track{
		{ID: "1", Title: "one", Album: "A", FileFormat: "MP3"},
		{ID: "2", Title: "two", Album: "B", FileFormat: "FLAC"},
		{ID: "3", Title: "three", Album: "C", FileFormat: "MP3"},
	}
	a := &App{LibraryState: LibraryState{tracks: tracks, formatPref: formatPrefMP3Only}}
	before := append([]library.Track(nil), a.tracks...)

	a.rebuildGroups()

	for i := range a.tracks {
		if a.tracks[i].ID != before[i].ID {
			t.Fatalf("library corrupted at %d: got %q, want %q", i, a.tracks[i].ID, before[i].ID)
		}
	}
}

func TestPlayModeAndRetroMessages(t *testing.T) {
	a := &App{PlaybackState: PlaybackState{playMode: playModeLoop, retroIdx: 0}}

	msg := a.cmdNextPlayMode()()
	if _, ok := msg.(playModeChangedMsg); !ok {
		t.Fatalf("cmdNextPlayMode returned %T, want playModeChangedMsg", msg)
	}
	if _, _ = a.Update(msg); a.playMode != playModeSingle {
		t.Fatalf("playMode = %v, want single", a.playMode)
	}

	msg = a.cmdRetroUp()()
	if _, ok := msg.(retroChangedMsg); !ok {
		t.Fatalf("cmdRetroUp returned %T, want retroChangedMsg", msg)
	}
	if _, _ = a.Update(msg); a.retroIdx != 1 {
		t.Fatalf("retroIdx = %d, want 1", a.retroIdx)
	}

	// Upper bound must clamp.
	a.retroIdx = audio.RetroPresetCount - 1
	msg = a.cmdRetroUp()()
	if _, _ = a.Update(msg); a.retroIdx != audio.RetroPresetCount-1 {
		t.Fatalf("retroIdx = %d, want clamp at %d", a.retroIdx, audio.RetroPresetCount-1)
	}
}

func TestConcurrentLangAndIconAccess(t *testing.T) {
	oldLang, oldIcons := ActiveLang(), ActiveIconSet()
	t.Cleanup(func() {
		SetLang(oldLang)
		setIconSet(oldIcons)
	})

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if i%2 == 0 {
					SetLang(LangZH)
					_ = T("library_title")
					_ = ActiveLang()
				} else {
					setIconSet(iconSetEmoji)
					_ = iconPlay()
					_ = ActiveIconSet()
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestSaveSettingReportsFailure(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	a := &App{st: st, searchInput: textinput.New()}
	a.saveSetting(store.KeyLanguage, store.ValLanguageEN)

	if a.statusMsg == "" {
		t.Fatal("failed settings write did not surface a status message")
	}
	if a.statusMsgExpiry.IsZero() {
		t.Fatal("status message expiry was not set")
	}
}

func TestReloadLibraryRejectsConcurrentScan(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	a := &App{st: st, musicDir: t.TempDir(), loading: true}
	if cmd := a.reloadLibrary(); cmd != nil {
		t.Fatal("reloadLibrary started a second scan while one was running")
	}
	if a.libScanFromReload {
		t.Fatal("rejected reload must not set libScanFromReload")
	}
}

func TestCleanupCancelsScan(t *testing.T) {
	cancelled := false
	a := &App{scanCancel: func() { cancelled = true }}
	a.Cleanup()
	if !cancelled {
		t.Fatal("Cleanup did not cancel the in-flight scan")
	}
}
