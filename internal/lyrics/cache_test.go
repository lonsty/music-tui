package lyrics

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/eilianxiao/music-tui/internal/library"
)

// ── CachedProvider tests ──────────────────────────────────────────────────────

type stubProvider struct {
	lines []Line
	err   error
	calls int
}

func (s *stubProvider) Fetch(_ context.Context, _ library.Track) ([]Line, error) {
	s.calls++
	return s.lines, s.err
}

func TestCachedProvider_HitsOnSecondCall(t *testing.T) {
	dir := t.TempDir()
	stub := &stubProvider{lines: []Line{{Time: 1 * time.Second, Text: "hello"}}}
	cp := NewCachedProvider(stub, dir)

	track := library.Track{ID: "abc123", Path: "/music/song.mp3"}

	// First call — cache miss, hits stub.
	lines1, err := cp.Fetch(context.Background(), track)
	if err != nil {
		t.Fatalf("first fetch error: %v", err)
	}
	if len(lines1) != 1 || lines1[0].Text != "hello" {
		t.Fatalf("unexpected lines: %v", lines1)
	}
	if stub.calls != 1 {
		t.Errorf("expected 1 stub call, got %d", stub.calls)
	}

	// Second call — should hit cache, not stub.
	lines2, err := cp.Fetch(context.Background(), track)
	if err != nil {
		t.Fatalf("second fetch error: %v", err)
	}
	if len(lines2) != 1 || lines2[0].Text != "hello" {
		t.Fatalf("cache returned wrong lines: %v", lines2)
	}
	if stub.calls != 1 {
		t.Errorf("stub called %d times (should still be 1)", stub.calls)
	}
}

func TestCachedProvider_CachesNegativeResult(t *testing.T) {
	dir := t.TempDir()
	stub := &stubProvider{lines: nil}
	cp := NewCachedProvider(stub, dir)

	track := library.Track{ID: "nolyrics", Path: "/music/song.mp3"}

	_, _ = cp.Fetch(context.Background(), track) // first call writes nil to cache
	_, _ = cp.Fetch(context.Background(), track) // second call should use cache

	if stub.calls != 1 {
		t.Errorf("stub should be called once even for nil result, got %d", stub.calls)
	}
}

func TestCachedProvider_DoesNotCacheErrors(t *testing.T) {
	dir := t.TempDir()
	stub := &stubProvider{err: errors.New("network error")}
	cp := NewCachedProvider(stub, dir)

	track := library.Track{ID: "err", Path: "/music/song.mp3"}

	_, _ = cp.Fetch(context.Background(), track) // error — should not cache
	_, _ = cp.Fetch(context.Background(), track) // should hit stub again

	if stub.calls != 2 {
		t.Errorf("expected 2 stub calls (errors not cached), got %d", stub.calls)
	}
}

func TestCachedProvider_EvictsExpiredEntry(t *testing.T) {
	dir := t.TempDir()

	// Write an artificially old cache entry.
	track := library.Track{ID: "old", Path: "/music/song.mp3"}
	key := cacheKey(track)
	path := dir + "/" + key + ".json"
	oldEntry := `{"fetched_at":1,"lines":[{"t":1000,"s":"stale"}]}`
	if err := os.WriteFile(path, []byte(oldEntry), 0o644); err != nil {
		t.Fatal(err)
	}

	stub := &stubProvider{lines: []Line{{Time: 2 * time.Second, Text: "fresh"}}}
	cp := NewCachedProvider(stub, dir)

	lines, err := cp.Fetch(context.Background(), track)
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if len(lines) == 0 || lines[0].Text != "fresh" {
		t.Errorf("expected fresh lines, got %v", lines)
	}
	if stub.calls != 1 {
		t.Errorf("expected stub call after expiry, got %d calls", stub.calls)
	}
}

// ── cacheKey tests ────────────────────────────────────────────────────────────

func TestCacheKey_UsesTrackID(t *testing.T) {
	t1 := library.Track{ID: "abc", Path: "/song.mp3"}
	t2 := library.Track{ID: "abc", Path: "/other.mp3"}
	if cacheKey(t1) != cacheKey(t2) {
		t.Error("same ID should produce same cache key regardless of path")
	}
}

func TestCacheKey_FallsBackToTitleArtist(t *testing.T) {
	t1 := library.Track{Title: "Hello", Artist: "Adele"}
	t2 := library.Track{Title: "Hello", Artist: "Adele"}
	t3 := library.Track{Title: "Hello", Artist: "Other"}
	if cacheKey(t1) != cacheKey(t2) {
		t.Error("same title+artist should produce same key")
	}
	if cacheKey(t1) == cacheKey(t3) {
		t.Error("different artist should produce different key")
	}
}

// ── ChainProvider tests ───────────────────────────────────────────────────────

func TestChainProvider_ReturnsFirstNonNil(t *testing.T) {
	p1 := &stubProvider{lines: nil}
	p2 := &stubProvider{lines: []Line{{Text: "found"}}}
	p3 := &stubProvider{lines: []Line{{Text: "should not reach"}}}

	chain := &ChainProvider{Providers: []Provider{p1, p2, p3}}
	lines, err := chain.Fetch(context.Background(), library.Track{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 1 || lines[0].Text != "found" {
		t.Errorf("expected p2 result, got %v", lines)
	}
	if p3.calls != 0 {
		t.Errorf("p3 should not be called, got %d calls", p3.calls)
	}
}

func TestChainProvider_SkipsErrors(t *testing.T) {
	p1 := &stubProvider{err: errors.New("fail")}
	p2 := &stubProvider{lines: []Line{{Text: "ok"}}}

	chain := &ChainProvider{Providers: []Provider{p1, p2}}
	lines, _ := chain.Fetch(context.Background(), library.Track{})
	if len(lines) == 0 || lines[0].Text != "ok" {
		t.Errorf("expected p2 result after p1 error, got %v", lines)
	}
}

func TestChainProvider_ReturnsNilWhenAllMiss(t *testing.T) {
	chain := &ChainProvider{Providers: []Provider{
		&stubProvider{lines: nil},
		&stubProvider{lines: nil},
	}}
	lines, err := chain.Fetch(context.Background(), library.Track{})
	if err != nil || lines != nil {
		t.Errorf("expected (nil, nil), got (%v, %v)", lines, err)
	}
}
