package tui

import (
	"fmt"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/store"
)

// ── Playlist commands ─────────────────────────────────────────────────────────

// cmdLoadPlaylists fetches all playlists from the DB and returns them via
// playlistsLoadedMsg.
func (a *App) cmdLoadPlaylists() tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return playlistsLoadedMsg{}
		}
		pls, err := a.st.GetPlaylists()
		return playlistsLoadedMsg{playlists: pls, err: err}
	}
}

// cmdLoadPlaylistTracks fetches the tracks for playlistID from the DB.
func (a *App) cmdLoadPlaylistTracks(playlistID string) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return playlistTracksLoadedMsg{playlistID: playlistID}
		}
		tracks, err := a.st.GetPlaylistTracks(playlistID)
		count, _ := a.st.PlaylistTrackCount(playlistID)
		return playlistTracksLoadedMsg{playlistID: playlistID, tracks: tracks, count: count, err: err}
	}
}

// cmdCreatePlaylist creates a new playlist with name and reloads the list.
func (a *App) cmdCreatePlaylist(name string) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return playlistsLoadedMsg{}
		}
		if _, err := a.st.CreatePlaylist(name); err != nil {
			return playlistsLoadedMsg{err: err}
		}
		pls, err := a.st.GetPlaylists()
		return playlistsLoadedMsg{playlists: pls, err: err}
	}
}

// cmdDeletePlaylist deletes the playlist with id and reloads the list.
func (a *App) cmdDeletePlaylist(id string) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return playlistsLoadedMsg{}
		}
		if err := a.st.DeletePlaylist(id); err != nil {
			return playlistsLoadedMsg{err: err}
		}
		pls, err := a.st.GetPlaylists()
		return playlistsLoadedMsg{playlists: pls, err: err}
	}
}

// cmdRenamePlaylist renames the playlist with id and reloads the list.
func (a *App) cmdRenamePlaylist(id, name string) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return playlistsLoadedMsg{}
		}
		if err := a.st.RenamePlaylist(id, name); err != nil {
			return playlistsLoadedMsg{err: err}
		}
		pls, err := a.st.GetPlaylists()
		return playlistsLoadedMsg{playlists: pls, err: err, successMsg: fmt.Sprintf(T("playlist_renamed"), name)}
	}
}

// cmdAddTrackToPlaylist adds trackID to playlistID and reloads the playlist's
// track list if we are currently viewing that playlist.
func (a *App) cmdAddTrackToPlaylist(playlistID, trackID string) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return playlistTracksLoadedMsg{playlistID: playlistID}
		}
		if in, _ := a.st.IsTrackInPlaylist(playlistID, trackID); in {
			tracks, _ := a.st.GetPlaylistTracks(playlistID)
			count, _ := a.st.PlaylistTrackCount(playlistID)
			return playlistTracksLoadedMsg{
				playlistID: playlistID,
				tracks:     tracks,
				count:      count,
				duplicate:  true,
			}
		}
		// Use current track count as append position.
		count, _ := a.st.PlaylistTrackCount(playlistID)
		if err := a.st.AddTrackToPlaylist(playlistID, trackID, count); err != nil {
			return playlistTracksLoadedMsg{playlistID: playlistID, err: err}
		}
		tracks, err := a.st.GetPlaylistTracks(playlistID)
		newCount, _ := a.st.PlaylistTrackCount(playlistID)
		return playlistTracksLoadedMsg{playlistID: playlistID, tracks: tracks, count: newCount, err: err}
	}
}

// cmdMoveTrack moves a track within its playlist and returns the reloaded list.
func (a *App) cmdMoveTrack(playlistID, trackID string, delta int) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return playlistTracksLoadedMsg{playlistID: playlistID}
		}
		if err := a.st.MoveTrack(playlistID, trackID, delta); err != nil {
			return playlistTracksLoadedMsg{playlistID: playlistID, err: err}
		}
		tracks, err := a.st.GetPlaylistTracks(playlistID)
		count, _ := a.st.PlaylistTrackCount(playlistID)
		return playlistTracksLoadedMsg{playlistID: playlistID, tracks: tracks, count: count, err: err}
	}
}

// cmdInsertTrackAt restores a removed track at its original position.
func (a *App) cmdInsertTrackAt(playlistID, trackID string, position int) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return playlistTracksLoadedMsg{playlistID: playlistID}
		}
		if err := a.st.InsertTrackAt(playlistID, trackID, position); err != nil {
			return playlistTracksLoadedMsg{playlistID: playlistID, err: err}
		}
		tracks, err := a.st.GetPlaylistTracks(playlistID)
		count, _ := a.st.PlaylistTrackCount(playlistID)
		return playlistTracksLoadedMsg{playlistID: playlistID, tracks: tracks, count: count, err: err}
	}
}

// cmdCreatePlaylistAndAdd creates a playlist and adds trackID to it.
func (a *App) cmdCreatePlaylistAndAdd(name, trackID string) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return playlistsLoadedMsg{}
		}
		pl, err := a.st.CreatePlaylist(name)
		if err != nil {
			return playlistsLoadedMsg{err: err}
		}
		if trackID != "" {
			if err := a.st.AddTrackToPlaylist(pl.ID, trackID, 0); err != nil {
				return playlistsLoadedMsg{err: err}
			}
		}
		pls, err := a.st.GetPlaylists()
		return playlistsLoadedMsg{
			playlists:  pls,
			err:        err,
			successMsg: fmt.Sprintf(T("playlist_created_added"), name),
		}
	}
}

// cmdRemoveTrackFromPlaylist removes trackID from playlistID and reloads.
func (a *App) cmdRemoveTrackFromPlaylist(playlistID, trackID string) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return playlistTracksLoadedMsg{playlistID: playlistID}
		}
		if err := a.st.RemoveTrackFromPlaylist(playlistID, trackID); err != nil {
			return playlistTracksLoadedMsg{playlistID: playlistID, err: err}
		}
		tracks, err := a.st.GetPlaylistTracks(playlistID)
		count, _ := a.st.PlaylistTrackCount(playlistID)
		return playlistTracksLoadedMsg{playlistID: playlistID, tracks: tracks, count: count, err: err}
	}
}

// cmdToggleFavorite adds or removes the current track from the Favorites
// playlist and returns a favoriteChangedMsg with the new state.
func (a *App) cmdToggleFavorite(track library.Track) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return noopMsg{}
		}
		in, err := a.st.IsTrackInPlaylist(store.FavoritesPlaylistID, track.ID)
		if err != nil {
			log.Printf("cmdToggleFavorite: check in playlist: %v", err)
			return noopMsg{}
		}
		if in {
			if err := a.st.RemoveTrackFromPlaylist(store.FavoritesPlaylistID, track.ID); err != nil {
				log.Printf("cmdToggleFavorite: remove from favorites: %v", err)
				return noopMsg{}
			}
			return favoriteChangedMsg{trackID: track.ID, trackTitle: track.DisplayTitle(), isFavorite: false}
		}
		count, _ := a.st.PlaylistTrackCount(store.FavoritesPlaylistID)
		if err := a.st.AddTrackToPlaylist(store.FavoritesPlaylistID, track.ID, count); err != nil {
			log.Printf("cmdToggleFavorite: add to favorites: %v", err)
			return noopMsg{}
		}
		return favoriteChangedMsg{trackID: track.ID, trackTitle: track.DisplayTitle(), isFavorite: true}
	}
}

// cmdCheckFavorite queries whether trackID is in Favorites and returns
// a favoriteChangedMsg so the UI can update isFavorite.
func (a *App) cmdCheckFavorite(trackID string) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return favoriteChangedMsg{trackID: trackID, isFavorite: false}
		}
		in, err := a.st.IsTrackInPlaylist(store.FavoritesPlaylistID, trackID)
		if err != nil {
			log.Printf("cmdCheckFavorite: %v", err)
		}
		return favoriteChangedMsg{trackID: trackID, isFavorite: in}
	}
}

// cmdPlayPlaylist loads playlistID's tracks and plays from position pos.
// It replaces filteredIdxs with the playlist's track order.
func (a *App) cmdPlayPlaylist(playlistID string, pos int) tea.Cmd {
	return func() tea.Msg {
		if a.st == nil {
			return noopMsg{}
		}
		tracks, err := a.st.GetPlaylistTracks(playlistID)
		if err != nil {
			return playlistPlayMsg{err: fmt.Errorf("load playlist: %w", err)}
		}
		if len(tracks) == 0 {
			return playlistPlayMsg{err: fmt.Errorf("%s", T("playlist_tracks_empty"))}
		}
		if pos >= len(tracks) {
			pos = 0
		}
		return playlistPlayMsg{tracks: tracks, startPos: pos}
	}
}

// playlistPlayMsg asks Update to replace filteredIdxs with the playlist
// track order and play from startPos.  When err is set, the playlist could
// not be loaded and no track list replacement should happen.
type playlistPlayMsg struct {
	tracks   []library.Track
	startPos int
	err      error
}

// respecting single-quoted and double-quoted sub-strings.
// Malformed quoting (unclosed quotes) is handled leniently.
func shellSplit(s string) []string {
	var tokens []string
	var cur strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				cur.WriteByte(c)
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			} else {
				cur.WriteByte(c)
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == ' ' || c == '\t':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}
