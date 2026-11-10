package tui

import (
	"fmt"
	"strconv"
	"time"

	"github.com/lonsty/music-tui/internal/lyrics"
	"github.com/lonsty/music-tui/internal/store"
)

const (
	lyricsOffsetStep = 500 * time.Millisecond
	maxLyricsOffset  = 30 * time.Second
)

func lyricsOffsetKey(trackID string) string {
	return store.KeyLyricsOffsetPrefix + trackID
}

// loadLyricsOffset reads the persisted offset for a track.
func loadLyricsOffset(st *store.Store, trackID string) time.Duration {
	if st == nil || trackID == "" {
		return 0
	}
	raw, err := st.GetSetting(lyricsOffsetKey(trackID))
	if err != nil || raw == "" {
		return 0
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

// saveLyricsOffset persists the current track's offset.
func (a *App) saveLyricsOffset() {
	if a.st == nil || a.currentTrack == nil {
		return
	}
	_ = a.st.SetSetting(
		lyricsOffsetKey(a.currentTrack.ID),
		strconv.FormatInt(a.offset.Milliseconds(), 10),
	)
}

// adjustLyricsOffset shifts the timing offset and updates the highlight.
func (a *App) adjustLyricsOffset(delta time.Duration) {
	if a.currentTrack == nil {
		return
	}
	if !a.synced || len(a.lines) == 0 {
		a.statusMsg = T("lyrics_offset_synced_only")
		a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		return
	}
	next := a.offset + delta
	if next > maxLyricsOffset {
		next = maxLyricsOffset
	}
	if next < -maxLyricsOffset {
		next = -maxLyricsOffset
	}
	if next == a.offset {
		return
	}
	a.offset = next
	a.syncLyricsActive()
	a.saveLyricsOffset()
	a.statusMsg = fmt.Sprintf(T("lyrics_offset_status"), a.offset.Seconds())
	a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
}

// lyricIndexAt returns the index of the last line whose timestamp is at or
// before pos, after applying the offset.  Returns -1 when no line has started.
func lyricIndexAt(lines []lyrics.Line, pos, offset time.Duration) int {
	adjusted := pos - offset
	idx := -1
	for i, l := range lines {
		if l.Time <= adjusted {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// lyricSeekTarget converts a lyric line timestamp to a playback position.
func lyricSeekTarget(l lyrics.Line, offset time.Duration) time.Duration {
	target := l.Time + offset
	if target < 0 {
		return 0
	}
	return target
}
