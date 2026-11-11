package tui

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/lyrics"
	"github.com/lonsty/music-tui/internal/store"
)

func testLines() []lyrics.Line {
	return []lyrics.Line{
		{Time: 0, Text: "one"},
		{Time: time.Second, Text: "two"},
		{Time: 2 * time.Second, Text: "three"},
	}
}

func TestLyricIndexAtAppliesOffset(t *testing.T) {
	lines := testLines()
	tests := []struct {
		pos    time.Duration
		offset time.Duration
		want   int
	}{
		{1200 * time.Millisecond, 0, 1},
		{1200 * time.Millisecond, time.Second, 0},
		{1200 * time.Millisecond, -time.Second, 2},
		{0, 0, 0},
		{-time.Second, 0, -1},
	}
	for _, tt := range tests {
		if got := lyricIndexAt(lines, tt.pos, tt.offset); got != tt.want {
			t.Errorf("lyricIndexAt(%v, %v) = %d, want %d", tt.pos, tt.offset, got, tt.want)
		}
	}
}

func TestLyricSeekTargetUsesOffset(t *testing.T) {
	line := lyrics.Line{Time: 2 * time.Second}
	if got := lyricSeekTarget(line, time.Second); got != 3*time.Second {
		t.Fatalf("seek target = %v, want 3s", got)
	}
	if got := lyricSeekTarget(lyrics.Line{Time: 0}, -time.Second); got != 0 {
		t.Fatalf("negative seek target = %v, want 0", got)
	}
}

func TestAdjustLyricsOffsetAndPersist(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	a := testApp(library.Track{ID: "t1", Title: "one"})
	a.st = st
	a.currentTrack = &a.tracks[0]
	a.lines = testLines()
	a.synced = true

	a.adjustLyricsOffset(lyricsOffsetStep)
	if a.offset != lyricsOffsetStep {
		t.Fatalf("offset = %v, want %v", a.offset, lyricsOffsetStep)
	}
	raw, _ := st.GetSetting(store.KeyLyricsOffsetPrefix + "t1")
	if raw != "500" {
		t.Fatalf("persisted offset = %q, want 500", raw)
	}
	if got := loadLyricsOffset(st, "t1"); got != lyricsOffsetStep {
		t.Fatalf("loaded offset = %v", got)
	}

	a.offset = maxLyricsOffset
	a.adjustLyricsOffset(lyricsOffsetStep)
	if a.offset != maxLyricsOffset {
		t.Fatalf("offset exceeded max: %v", a.offset)
	}
}

func TestOffsetKeysAndPlainTextGuard(t *testing.T) {
	a := testApp(library.Track{ID: "t1", Title: "one"})
	a.currentTrack = &a.tracks[0]
	a.lines = testLines()
	a.synced = true
	a.H, a.W = 30, 100

	ctx := a.localTrackListCtx()
	ctx.UseMarquee = false
	a.handleSharedTrackListKey(ctx, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	if a.offset != lyricsOffsetStep {
		t.Fatalf("] set offset %v, want %v", a.offset, lyricsOffsetStep)
	}

	a.synced = false
	a.offset = 0
	a.handleSharedTrackListKey(ctx, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	if a.offset != 0 {
		t.Fatalf("plain lyrics changed offset to %v", a.offset)
	}
	if a.statusMsg != T("lyrics_offset_synced_only") {
		t.Fatalf("status = %q, want synced-only hint", a.statusMsg)
	}
}

func TestFilteredPosUsesCache(t *testing.T) {
	tracks := make([]library.Track, 10000)
	for i := range tracks {
		tracks[i] = library.Track{ID: fmt.Sprintf("id-%d", i), Title: "t"}
	}
	a := testApp(tracks...)
	if len(a.posByID) != len(tracks) {
		t.Fatalf("posByID len = %d, want %d", len(a.posByID), len(tracks))
	}
	if got := a.filteredPos("id-9999"); got != 9999 {
		t.Fatalf("filteredPos = %d, want 9999", got)
	}
	ctx := a.localTrackListCtx()
	if allocs := testing.AllocsPerRun(100, func() {
		_ = ctx.TrackAt(5000)
	}); allocs != 0 {
		t.Fatalf("TrackAt allocates %v times per call", allocs)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		_ = a.filteredPos("id-5000")
	}); allocs != 0 {
		t.Fatalf("filteredPos allocates %v times per call", allocs)
	}
}

func TestSettingsFieldRowsMatchPlan(t *testing.T) {
	rows := settingsFieldRows()
	expected := map[settingsField]settingsRow{
		settingsFieldMusicDir: settingsRowDir,
		settingsFieldChipOpts: settingsRowOpts,
		settingsFieldLanguage: settingsRowLang,
		settingsFieldFormat:   settingsRowFmt,
		settingsFieldIconSet:  settingsRowIcon,
	}
	prev := -1
	for f := settingsField(0); f < settingsFieldCount; f++ {
		row, ok := expected[f]
		if !ok {
			if rows[f] <= prev {
				t.Fatalf("field %d row %d is not after %d", f, rows[f], prev)
			}
			prev = rows[f]
			continue
		}
		if rows[f] < 0 || rows[f] >= len(settingsBodyPlan) || settingsBodyPlan[rows[f]] != row {
			t.Fatalf("field %d maps to row %d, not %v", f, rows[f], row)
		}
		if rows[f] <= prev {
			t.Fatalf("field %d row %d is not after %d", f, rows[f], prev)
		}
		prev = rows[f]
	}

	a := &App{settingsActive: settingsFieldMusicDir}
	a.settingsMoveNext()
	if a.ovlScrollRow != rows[settingsFieldChipOpts] {
		t.Fatalf("settingsMoveNext scrolled to %d, want %d", a.ovlScrollRow, rows[settingsFieldChipOpts])
	}
}
