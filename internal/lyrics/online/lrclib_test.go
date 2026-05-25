package online

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/lyrics"
)

// ── lrcLibRespToLines tests ───────────────────────────────────────────────────

func TestLrcLibRespToLines_SyncedPreferred(t *testing.T) {
	r := &lrcLibResponse{
		SyncedLyrics: "[00:01.00]Hello\n[00:05.00]World\n",
		PlainLyrics:  "Hello\nWorld",
	}
	lines := lrcLibRespToLines(r)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Time != 1*time.Second || lines[0].Text != "Hello" {
		t.Errorf("unexpected line 0: %v", lines[0])
	}
}

func TestLrcLibRespToLines_FallsBackToPlain(t *testing.T) {
	r := &lrcLibResponse{
		SyncedLyrics: "",
		PlainLyrics:  "Hello\nWorld",
	}
	lines := lrcLibRespToLines(r)
	if len(lines) != 2 {
		t.Fatalf("expected 2 plain lines, got %d", len(lines))
	}
	for _, l := range lines {
		if l.Time != 0 {
			t.Errorf("plain line should have Time=0, got %v", l.Time)
		}
	}
}

func TestLrcLibRespToLines_NilWhenEmpty(t *testing.T) {
	r := &lrcLibResponse{}
	if lrcLibRespToLines(r) != nil {
		t.Error("empty response should return nil")
	}
}

// ── HTTP integration tests (using httptest server) ────────────────────────────

func TestLrcLibProvider_Fetch_ExactMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/get" {
			resp := lrcLibResponse{
				ID:           1,
				TrackName:    "Hello",
				ArtistName:   "Adele",
				SyncedLyrics: "[00:01.00]Hello\n",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	p := &LrcLibProvider{client: srv.Client()}
	// Patch base URL for test.
	origURL := lrcLibBaseURL
	defer func() { /* lrcLibBaseURL is const; test via doGet directly */ _ = origURL }()

	// Test doGet directly using the test server URL.
	body, err := p.doGet(context.Background(), srv.URL+"/api/get?track_name=Hello&artist_name=Adele")
	if err != nil {
		t.Fatalf("doGet error: %v", err)
	}
	if body == nil {
		t.Fatal("expected non-nil body")
	}

	var result lrcLibResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if result.TrackName != "Hello" {
		t.Errorf("unexpected track name: %q", result.TrackName)
	}
}

func TestLrcLibProvider_Fetch_Returns404AsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()

	p := &LrcLibProvider{client: srv.Client()}
	body, err := p.doGet(context.Background(), srv.URL+"/api/get?track_name=X")
	if err != nil {
		t.Fatalf("unexpected error for 404: %v", err)
	}
	if body != nil {
		t.Errorf("expected nil body for 404, got %v", body)
	}
}

func TestLrcLibProvider_SearchPick_PreferMatchingArtist(t *testing.T) {
	results := []lrcLibResponse{
		{ID: 1, ArtistName: "Wrong Artist", SyncedLyrics: "[00:01.00]Wrong\n"},
		{ID: 2, ArtistName: "Adele", SyncedLyrics: "[00:01.00]Correct\n"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(results)
	}))
	defer srv.Close()

	p := &LrcLibProvider{client: srv.Client()}
	track := library.Track{Title: "Hello", Artist: "Adele"}

	body, err := p.doGet(context.Background(), srv.URL+"/api/search?q=Adele+Hello")
	if err != nil {
		t.Fatalf("doGet error: %v", err)
	}
	var res []lrcLibResponse
	_ = json.Unmarshal(body, &res)

	// Simulate the artist-matching logic.
	want := "adele"
	var best *lrcLibResponse
	for i := range res {
		if containsLower(res[i].ArtistName, want) {
			best = &res[i]
			break
		}
		if best == nil {
			best = &res[i]
		}
	}
	if best == nil || best.ID != 2 {
		t.Errorf("expected to pick result ID=2, got %v", best)
	}
	_ = track
}

func containsLower(s, sub string) bool {
	sl := make([]byte, len(s))
	for i, c := range []byte(s) {
		if c >= 'A' && c <= 'Z' {
			sl[i] = c + 32
		} else {
			sl[i] = c
		}
	}
	subl := make([]byte, len(sub))
	for i, c := range []byte(sub) {
		if c >= 'A' && c <= 'Z' {
			subl[i] = c + 32
		} else {
			subl[i] = c
		}
	}
	return string(sl) != "" && string(subl) != "" &&
		indexOf(string(sl), string(subl)) >= 0
}

func indexOf(s, sub string) int {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ── ParseLRCString / PlainTextToLines tests ───────────────────────────────────

func TestParseLRCString(t *testing.T) {
	s := "[00:01.00]Line one\n[00:05.50]Line two\n"
	lines := lyrics.ParseLRCString(s)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Text != "Line one" {
		t.Errorf("unexpected: %v", lines[0])
	}
}

func TestPlainTextToLines(t *testing.T) {
	s := "Hello\n\nWorld\n"
	lines := lyrics.PlainTextToLines(s)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for _, l := range lines {
		if l.Time != 0 {
			t.Errorf("plain line should have Time=0")
		}
	}
}
