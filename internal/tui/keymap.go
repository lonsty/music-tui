package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ActionID is a stable action name. Specs reference ActionIDs; physical keys
// live only in the registry.
type ActionID string

// Scope identifies one context layer. activeScopes returns them in priority
// order; the first scope that binds a key consumes it.
type Scope string

// KeyGroup is the fixed display order used by every hint layer.
type KeyGroup int

const (
	groupExit KeyGroup = iota
	groupPrimary
	groupNav
	groupPlayback
	groupObject
	groupView
	groupSystem
)

const (
	scopeGlobal                  Scope = "global"
	scopeTracklist               Scope = "tracklist"
	scopeGroup                   Scope = "group"
	scopeLocalTracks             Scope = "local.tracks"
	scopeLocalGroups             Scope = "local.groups"
	scopePlaylistList            Scope = "playlist.list"
	scopePlaylistTracks          Scope = "playlist.tracks"
	scopeFullscreen              Scope = "fullscreen"
	scopeConfirm                 Scope = "confirm"
	scopeOverlayHelp             Scope = "overlay.help"
	scopeOverlayInfo             Scope = "overlay.info"
	scopeOverlayQueue            Scope = "overlay.queue"
	scopeOverlayAddList          Scope = "overlay.add.list"
	scopeOverlayAddCreate        Scope = "overlay.add.create"
	scopeOverlaySettingsBrowse   Scope = "overlay.settings.browse"
	scopeOverlaySettingsEdit     Scope = "overlay.settings.edit"
	scopeInputSearch             Scope = "input.search"
	scopeInputAddTracks          Scope = "input.addtracks"
	scopeInputName               Scope = "input.name"
)

// Action IDs. Grouped by KeyGroup for readability only; the registry order
// determines nothing except stable sorting when all else ties.
const (
	actionCancel            ActionID = "overlay.cancel"
	actionQuit              ActionID = "global.quit"
	actionForceQuit         ActionID = "global.force_quit"
	actionPlay              ActionID = "playback.play"
	actionPause             ActionID = "playback.pause"
	actionOpenPlaylist      ActionID = "playlist.open"
	actionSeekLine          ActionID = "lyrics.seek_line"
	actionConfirm           ActionID = "overlay.confirm"
	actionSave              ActionID = "overlay.save"
	actionDeleteConfirm     ActionID = "playlist.delete_confirm"
	actionCreateAndAdd      ActionID = "playlist.create_add"
	actionCommitName        ActionID = "playlist.commit_name"
	actionAddTracksToggle   ActionID = "playlist.add_tracks_toggle"
	actionSearchPlay        ActionID = "library.search_play"
	actionSearchCancel      ActionID = "library.search_cancel"
	actionSettingsEdit      ActionID = "settings.edit"
	actionSettingsCommit    ActionID = "settings.commit"
	actionSettingsCancel    ActionID = "settings.cancel"
	actionSettingsClose     ActionID = "settings.close"
	actionSettingsScrollUp  ActionID = "overlay.scroll_up"
	actionSettingsScrollDown ActionID = "overlay.scroll_down"
	actionOverlayScroll      ActionID = "overlay.scroll"

	actionNavDown    ActionID = "nav.down"
	actionNavUp      ActionID = "nav.up"
	actionNavTop     ActionID = "nav.top"
	actionNavBottom  ActionID = "nav.bottom"
	actionNavPageDown ActionID = "nav.page_down"
	actionNavPageUp  ActionID = "nav.page_up"
	actionJumpPlaying ActionID = "nav.jump_playing"
	actionBrowseUp   ActionID = "lyrics.browse_up"
	actionBrowseDown ActionID = "lyrics.browse_down"

	actionNext          ActionID = "playback.next"
	actionPrev          ActionID = "playback.prev"
	actionSeekBack      ActionID = "playback.seek_back"
	actionSeekForward   ActionID = "playback.seek_forward"
	actionVolumeUp      ActionID = "playback.volume_up"
	actionVolumeDown    ActionID = "playback.volume_down"
	actionMode          ActionID = "playback.mode"
	actionChip          ActionID = "playback.chip"
	actionLofiDown      ActionID = "playback.lofi_down"
	actionLofiUp        ActionID = "playback.lofi_up"
	actionOffsetEarlier ActionID = "lyrics.offset_earlier"
	actionOffsetLater   ActionID = "lyrics.offset_later"
	actionMediaPlay     ActionID = "global.media_play"
	actionMediaStop     ActionID = "global.media_stop"
	actionMediaNext     ActionID = "global.media_next"
	actionMediaPrev     ActionID = "global.media_prev"
	actionMediaVolDown  ActionID = "global.media_vol_down"
	actionMediaVolUp    ActionID = "global.media_vol_up"

	actionSearch        ActionID = "library.search"
	actionClearFilter   ActionID = "library.clear_filter"
	actionAddToPlaylist ActionID = "library.add_to_playlist"
	actionTrackInfo     ActionID = "library.track_info"
	actionLike          ActionID = "library.like"
	actionQueueAppend   ActionID = "queue.append"
	actionQueueInsert   ActionID = "queue.insert_next"
	actionQueueRemove   ActionID = "queue.remove"
	actionQueueClear    ActionID = "queue.clear"
	actionQueueClose    ActionID = "queue.close"
	actionPlaylistNew   ActionID = "playlist.new"
	actionPlaylistRename ActionID = "playlist.rename"
	actionPlaylistDelete ActionID = "playlist.delete"
	actionPlaylistAddTrack ActionID = "playlist.add_track"
	actionPlaylistRemove  ActionID = "playlist.remove_track"
	actionPlaylistUndo    ActionID = "playlist.undo"
	actionPlaylistMoveDown ActionID = "playlist.move_down"
	actionPlaylistMoveUp   ActionID = "playlist.move_up"
	actionPlaylistBack     ActionID = "playlist.back"

	actionViewTracks  ActionID = "library.view_tracks"
	actionViewAlbums  ActionID = "library.view_albums"
	actionViewArtists ActionID = "library.view_artists"
	actionDrillDown   ActionID = "library.drill_down"
	actionGroupBack   ActionID = "library.back"
	actionQueueOpen   ActionID = "queue.open"
	actionFullscreen  ActionID = "playback.fullscreen"
	actionRightPanel  ActionID = "playback.right_panel"
	actionCollapse    ActionID = "playback.collapse_panel"
	actionSwitchTab   ActionID = "global.switch_tab"
	actionSettingsOpen ActionID = "settings.open"
	actionHelp        ActionID = "global.help"
	actionReload      ActionID = "global.reload"
)

// Binding is one registry row. Keys are the physical key strings understood by
// bubbles/key. DisplayKey/ShortKey are render labels, not bindings.
type Binding struct {
	Action     ActionID
	Keys       []string
	DisplayKey string
	ShortKey   string
	LabelKey   string
	DescKey    string
	Group      KeyGroup
	ShortRank  int
	Scopes     []Scope
	Enabled    func(*App) bool
}

func b(action ActionID, keys []string, display, short, label, desc string, group KeyGroup, rank int, scopes []Scope, enabled ...func(*App) bool) Binding {
	var enabledFn func(*App) bool
	if len(enabled) > 0 {
		enabledFn = enabled[0]
	}
	return Binding{
		Action: action, Keys: keys, DisplayKey: display, ShortKey: short,
		LabelKey: label, DescKey: desc, Group: group, ShortRank: rank,
		Scopes: scopes, Enabled: enabledFn,
	}
}

func allScopes(s ...Scope) []Scope { return s }

// keyBindings is the single source of truth for physical keys.
var keyBindings = []Binding{
	// ── Exit / cancel ────────────────────────────────────────────────────────
	b(actionCancel, []string{"esc"}, "Esc", "Esc", "overlay_cancel", "action_cancel", groupExit, 100,
		allScopes(scopeOverlayHelp, scopeOverlayInfo, scopeOverlayAddList, scopeConfirm, scopeGlobal)),
	b(actionCancel, []string{"?"}, "?", "?", "hint_help", "action_close", groupExit, 97,
		allScopes(scopeOverlayHelp)),
	b(actionCancel, []string{"i"}, "i", "i", "hint_help", "action_close", groupExit, 97,
		allScopes(scopeOverlayInfo)),
	b(actionQuit, []string{"q"}, "q", "q", "hint_quit", "action_quit", groupExit, 95,
		allScopes(scopeGlobal)),
	b(actionForceQuit, []string{"ctrl+c"}, "Ctrl+C", "Ctrl+C", "hint_quit", "action_force_quit", groupExit, 99,
		allScopes(scopeGlobal)),
	b(actionPlaylistBack, []string{"esc"}, "Esc", "Esc", "hint_back", "action_playlist_back", groupExit, 98,
		allScopes(scopePlaylistTracks)),
	b(actionGroupBack, []string{"esc"}, "Esc", "Esc", "hint_back", "action_back", groupExit, 98,
		allScopes(scopeGroup)),
	b(actionClearFilter, []string{"esc"}, "Esc", "Esc", "hint_back", "action_clear_filter", groupExit, 99,
		allScopes(scopeLocalTracks), func(a *App) bool { return strings.TrimSpace(a.searchInput.Value()) != "" }),
	b(actionQueueClose, []string{"esc"}, "Esc", "Esc", "queue_close", "action_close", groupExit, 98,
		allScopes(scopeOverlayQueue)),
	b(actionSettingsClose, []string{"esc"}, "Esc", "Esc", "settings_cancel", "action_close", groupExit, 98,
		allScopes(scopeOverlaySettingsBrowse)),
	b(actionSettingsCancel, []string{"esc"}, "Esc", "Esc", "settings_cancel", "action_cancel", groupExit, 98,
		allScopes(scopeOverlaySettingsEdit)),
	b(actionSearchCancel, []string{"esc"}, "Esc", "Esc", "overlay_cancel", "action_cancel", groupExit, 98,
		allScopes(scopeInputSearch)),
	b(actionCancel, []string{"esc"}, "Esc", "Esc", "overlay_cancel", "action_cancel", groupExit, 98,
		allScopes(scopeOverlayAddCreate, scopeInputName, scopeInputAddTracks)),
	b(actionFullscreen, []string{"esc", "f"}, "Esc / f", "Esc", "hint_back", "action_fullscreen", groupExit, 96,
		allScopes(scopeFullscreen)),

	// ── Primary ─────────────────────────────────────────────────────────────
	b(actionPlay, []string{"enter"}, "Enter", "Enter", "hint_play", "action_play", groupPrimary, 90,
		allScopes(scopeLocalTracks, scopePlaylistTracks)),
	b(actionPause, []string{" "}, "Space", "Space", "hint_pause", "action_pause_resume", groupPrimary, 90,
		allScopes(scopeGlobal)),
	b(actionOpenPlaylist, []string{"enter"}, "Enter", "Enter", "hint_enter_playlist", "action_open_playlist", groupPrimary, 90,
		allScopes(scopePlaylistList)),
	b(actionDrillDown, []string{"enter"}, "Enter", "Enter", "hint_open_group", "action_open_group", groupPrimary, 90,
		allScopes(scopeGroup)),
	b(actionSeekLine, []string{"enter"}, "Enter", "Enter", "hint_play", "action_seek_line", groupPrimary, 90,
		allScopes(scopeFullscreen)),
	b(actionConfirm, []string{"enter"}, "Enter", "Enter", "overlay_confirm", "action_confirm", groupPrimary, 90,
		allScopes(scopeOverlayAddList)),
	b(actionDeleteConfirm, []string{"enter"}, "Enter", "Enter", "overlay_confirm", "action_delete_playlist", groupPrimary, 90,
		allScopes(scopeConfirm)),
	b(actionCreateAndAdd, []string{"enter"}, "Enter", "Enter", "settings_save", "action_create_add", groupPrimary, 90,
		allScopes(scopeOverlayAddCreate)),
	b(actionCommitName, []string{"enter"}, "Enter", "Enter", "settings_save", "action_save", groupPrimary, 90,
		allScopes(scopeInputName)),
	b(actionAddTracksToggle, []string{"enter"}, "Enter", "Enter", "overlay_add", "action_add_track", groupPrimary, 90,
		allScopes(scopeInputAddTracks)),
	b(actionSearchPlay, []string{"enter"}, "Enter", "Enter", "hint_play", "action_play", groupPrimary, 90,
		allScopes(scopeInputSearch)),
	b(actionSettingsEdit, []string{"enter"}, "Enter", "Enter", "settings_edit", "action_settings_edit", groupPrimary, 90,
		allScopes(scopeOverlaySettingsBrowse)),
	b(actionSettingsCommit, []string{"enter"}, "Enter", "Enter", "settings_save", "action_save", groupPrimary, 90,
		allScopes(scopeOverlaySettingsEdit)),

	// ── Navigation ──────────────────────────────────────────────────────────
	b(actionNavDown, []string{"j", "down"}, "j / ↓", "j/k", "hint_nav", "action_move_down", groupNav, 80,
		allScopes(scopeTracklist, scopeGroup, scopePlaylistList, scopeOverlayHelp, scopeOverlayInfo, scopeOverlayQueue, scopeOverlayAddList)),
	b(actionNavUp, []string{"k", "up"}, "k / ↑", "↑/↓", "hint_prev", "action_move_up", groupNav, 79,
		allScopes(scopeTracklist, scopeGroup, scopePlaylistList, scopeOverlayHelp, scopeOverlayInfo, scopeOverlayQueue, scopeOverlayAddList)),
	b(actionNavTop, []string{"g"}, "g", "g", "hint_page", "action_top", groupNav, 70,
		allScopes(scopeTracklist, scopeGroup, scopePlaylistList, scopeOverlayQueue)),
	b(actionNavBottom, []string{"G"}, "G", "G", "hint_page", "action_bottom", groupNav, 69,
		allScopes(scopeTracklist, scopeGroup, scopePlaylistList, scopeOverlayQueue)),
	b(actionNavPageDown, []string{"pgdown", "ctrl+d"}, "PgDn / Ctrl+D", "PgDn", "hint_page", "action_page_down", groupNav, 68,
		allScopes(scopeTracklist)),
	b(actionNavPageUp, []string{"pgup", "ctrl+u"}, "PgUp / Ctrl+U", "PgUp", "hint_page", "action_page_up", groupNav, 67,
		allScopes(scopeTracklist)),
	b(actionJumpPlaying, []string{"c"}, "c", "c", "hint_jump_playing", "action_jump_playing", groupNav, 66,
		allScopes(scopeTracklist, scopeGroup)),
	b(actionBrowseUp, []string{"up", "k"}, "↑ / k", "↑/↓", "hint_prev", "action_browse_up", groupNav, 78,
		allScopes(scopeFullscreen)),
	b(actionBrowseDown, []string{"down", "j"}, "↓ / j", "↑/↓", "hint_browse", "action_browse_down", groupNav, 77,
		allScopes(scopeFullscreen)),
	b(actionSettingsScrollUp, []string{"up", "k"}, "↑ / k", "↑/↓", "hint_prev", "action_scroll_up", groupNav, 78,
		allScopes(scopeOverlaySettingsBrowse)),
	b(actionSettingsScrollDown, []string{"down", "j"}, "↓ / j", "↑/↓", "hint_next", "action_scroll_down", groupNav, 77,
		allScopes(scopeOverlaySettingsBrowse)),
	// Display-only chip for help/info footers; scroll keys are already bound
	// by actionNavUp/actionNavDown in those scopes.
	b(actionOverlayScroll, []string{"up", "down"}, "↑ / ↓", "↑/↓", "hint_scroll", "action_scroll", groupNav, 76,
		allScopes()),
	b(actionNavDown, []string{"down"}, "↓", "↓", "hint_nav", "action_move_down", groupNav, 76,
		allScopes(scopeInputSearch)),
	b(actionNavUp, []string{"up"}, "↑", "↑", "hint_prev", "action_move_up", groupNav, 75,
		allScopes(scopeInputSearch)),
	b(actionNavDown, []string{"down"}, "↓", "↓", "hint_nav", "action_move_down", groupNav, 76,
		allScopes(scopeInputAddTracks)),
	b(actionNavUp, []string{"up"}, "↑", "↑", "hint_prev", "action_move_up", groupNav, 75,
		allScopes(scopeInputAddTracks)),

	// ── Playback ────────────────────────────────────────────────────────────
	b(actionNext, []string{"n"}, "n", "n/p", "hint_next_prev", "action_next", groupPlayback, 74,
		allScopes(scopeGlobal)),
	b(actionPrev, []string{"p"}, "p", "n/p", "hint_next_prev", "action_prev", groupPlayback, 73,
		allScopes(scopeGlobal)),
	b(actionSeekBack, []string{"<"}, "<", "< >", "hint_seek", "action_seek_back", groupPlayback, 72,
		allScopes(scopeGlobal)),
	b(actionSeekForward, []string{">"}, ">", "< >", "hint_seek", "action_seek_forward", groupPlayback, 71,
		allScopes(scopeGlobal)),
	b(actionVolumeUp, []string{"+", "="}, "+ / =", "+/-", "hint_vol", "action_volume_up", groupPlayback, 70,
		allScopes(scopeGlobal)),
	b(actionVolumeDown, []string{"-"}, "-", "+/-", "hint_vol", "action_volume_down", groupPlayback, 69,
		allScopes(scopeGlobal)),
	b(actionMode, []string{"m"}, "m", "m", "hint_mode", "action_cycle_mode", groupPlayback, 68,
		allScopes(scopeGlobal)),
	b(actionChip, []string{"b"}, "b", "b", "hint_chip", "action_chip", groupPlayback, 67,
		allScopes(scopeGlobal)),
	b(actionLofiDown, []string{"r"}, "r", "r/R", "hint_retro", "action_lofi_down", groupPlayback, 66,
		allScopes(scopeGlobal)),
	b(actionLofiUp, []string{"R"}, "R", "r/R", "hint_retro", "action_lofi_up", groupPlayback, 65,
		allScopes(scopeGlobal)),
	b(actionOffsetEarlier, []string{"[", ","}, "[ / ,", "[ ]", "hint_offset", "action_offset_earlier", groupPlayback, 64,
		allScopes(scopeFullscreen)),
	b(actionOffsetLater, []string{"]", "."}, "] / .", "[ ]", "hint_offset", "action_offset_later", groupPlayback, 63,
		allScopes(scopeFullscreen)),
	b(actionOffsetEarlier, []string{"["}, "[", "[ ]", "hint_offset", "action_offset_earlier", groupPlayback, 62,
		allScopes(scopeTracklist)),
	b(actionOffsetLater, []string{"]"}, "]", "[ ]", "hint_offset", "action_offset_later", groupPlayback, 61,
		allScopes(scopeTracklist)),
	b(actionMediaPlay, []string{"f7"}, "F7", "F7", "hint_pause", "action_media_play", groupPlayback, 60,
		allScopes(scopeGlobal)),
	b(actionMediaStop, []string{"f8"}, "F8", "F8", "hint_pause", "action_media_stop", groupPlayback, 59,
		allScopes(scopeGlobal)),
	b(actionMediaNext, []string{"f9"}, "F9", "F9", "hint_next", "action_media_next", groupPlayback, 58,
		allScopes(scopeGlobal)),
	b(actionMediaPrev, []string{"f6"}, "F6", "F6", "hint_prev", "action_media_prev", groupPlayback, 57,
		allScopes(scopeGlobal)),
	b(actionMediaVolDown, []string{"f11"}, "F11", "F11", "hint_vol", "action_media_vol_down", groupPlayback, 56,
		allScopes(scopeGlobal)),
	b(actionMediaVolUp, []string{"f12"}, "F12", "F12", "hint_vol", "action_media_vol_up", groupPlayback, 55,
		allScopes(scopeGlobal)),

	// ── Object actions ──────────────────────────────────────────────────────
	b(actionSearch, []string{"/"}, "/", "/", "hint_search", "action_search", groupObject, 54,
		allScopes(scopeLocalTracks)),
	b(actionAddToPlaylist, []string{"a"}, "a", "a", "hint_playlist_add_track", "action_add_to_playlist", groupObject, 53,
		allScopes(scopeLocalTracks)),
	b(actionTrackInfo, []string{"i"}, "i", "i", "hint_help", "action_track_info", groupObject, 52,
		allScopes(scopeLocalTracks, scopePlaylistTracks)),
	b(actionLike, []string{"L"}, "L", "L", "hint_like_toggle", "action_like", groupObject, 51,
		allScopes(scopeTracklist, scopeGroup)),
	b(actionQueueAppend, []string{"e"}, "e", "e", "hint_queue", "action_queue_append", groupObject, 50,
		allScopes(scopeTracklist)),
	b(actionQueueInsert, []string{"E"}, "E", "E", "hint_queue", "action_queue_insert_next", groupObject, 49,
		allScopes(scopeTracklist)),
	b(actionQueueRemove, []string{"x", "delete"}, "x", "x", "queue_remove", "action_remove_track", groupObject, 48,
		allScopes(scopeOverlayQueue)),
	b(actionQueueClear, []string{"C"}, "C", "C", "queue_clear", "action_queue_clear", groupObject, 47,
		allScopes(scopeOverlayQueue)),
	b(actionPlaylistNew, []string{"N"}, "N", "N", "hint_playlist_new", "action_new_playlist", groupObject, 46,
		allScopes(scopePlaylistList, scopeOverlayAddList)),
	b(actionPlaylistRename, []string{"f2"}, "F2", "F2", "hint_playlist_rename", "action_rename_playlist", groupObject, 45,
		allScopes(scopePlaylistList)),
	b(actionPlaylistDelete, []string{"d"}, "d", "d", "hint_playlist_delete", "action_delete_playlist", groupObject, 44,
		allScopes(scopePlaylistList)),
	b(actionPlaylistAddTrack, []string{"a"}, "a", "a", "hint_playlist_add_track", "action_add_track", groupObject, 43,
		allScopes(scopePlaylistTracks)),
	b(actionPlaylistRemove, []string{"x"}, "x", "x", "hint_playlist_remove_track", "action_remove_track", groupObject, 42,
		allScopes(scopePlaylistTracks)),
	b(actionPlaylistUndo, []string{"u"}, "u", "u", "hint_undo", "action_undo", groupObject, 41,
		allScopes(scopePlaylistTracks), func(a *App) bool {
			return a.undoRemove != nil && time.Now().Before(a.undoRemove.expiresAt)
		}),
	b(actionPlaylistMoveDown, []string{"J"}, "J", "J/K", "hint_move_track", "action_move_track_down", groupObject, 40,
		allScopes(scopePlaylistTracks)),
	b(actionPlaylistMoveUp, []string{"K"}, "K", "J/K", "hint_move_track", "action_move_track_up", groupObject, 39,
		allScopes(scopePlaylistTracks)),

	// ── Views / panels ──────────────────────────────────────────────────────
	b(actionViewTracks, []string{"1"}, "1", "1/2/3", "hint_view", "action_view_tracks", groupView, 38,
		allScopes(scopeLocalTracks, scopeLocalGroups, scopeGroup)),
	b(actionViewAlbums, []string{"2"}, "2", "1/2/3", "hint_view", "action_view_albums", groupView, 37,
		allScopes(scopeLocalTracks, scopeLocalGroups, scopeGroup)),
	b(actionViewArtists, []string{"3"}, "3", "1/2/3", "hint_view", "action_view_artists", groupView, 36,
		allScopes(scopeLocalTracks, scopeLocalGroups, scopeGroup)),
	b(actionQueueOpen, []string{"4"}, "4", "4", "hint_queue", "action_queue_open", groupView, 35,
		allScopes(scopeGlobal)),
	b(actionFullscreen, []string{"f"}, "f", "f", "hint_fullscreen", "action_fullscreen", groupView, 34,
		allScopes(scopeGlobal)),
	b(actionRightPanel, []string{"l"}, "l", "l", "hint_lyrics", "action_lyrics_mode", groupView, 33,
		allScopes(scopeGlobal), func(a *App) bool { return a.showMiniPlayer() }),
	b(actionCollapse, []string{`\`}, `\`, `\`, "hint_collapse", "action_collapse_panel", groupView, 32,
		allScopes(scopeGlobal)),
	b(actionSwitchTab, []string{"tab"}, "Tab", "Tab", "hint_switch_tab", "action_switch_tab", groupView, 31,
		allScopes(scopeGlobal)),
	b(actionSettingsOpen, []string{","}, ",", ",", "hint_settings", "action_settings", groupView, 30,
		allScopes(scopeLocalTracks, scopeLocalGroups, scopeGroup, scopePlaylistTracks, scopePlaylistList, scopeGlobal)),

	// ── System ──────────────────────────────────────────────────────────────
	b(actionHelp, []string{"?"}, "?", "?", "hint_help", "action_this_help", groupSystem, 29,
		allScopes(scopeGlobal)),
	b(actionReload, []string{"ctrl+r"}, "Ctrl+R", "Ctrl+R", "hint_reload", "action_reload_library", groupSystem, 28,
		allScopes(scopeGlobal, scopeOverlaySettingsBrowse, scopeOverlaySettingsEdit)),
}

// forbiddenKeys are legacy terminal encodings that cannot be distinguished
// reliably. Ctrl+C is intentionally present because the app owns SIGINT in
// raw mode; it is handled before all other scopes.
var forbiddenKeys = map[string]string{
	"alt+":      "Alt is ESC-prefixed and collides with CSI",
	"ctrl+i":    "indistinguishable from tab",
	"ctrl+m":    "indistinguishable from enter",
	"ctrl+[":    "indistinguishable from esc",
	"ctrl+@":    "indistinguishable from ctrl+space",
	"ctrl+space": "produces NUL",
	"ctrl+s":    "terminal flow control",
	"ctrl+q":    "terminal flow control",
	"ctrl+z":    "terminal suspend",
}

type keyRegistry struct {
	bindings []Binding
	byScope  map[Scope][]int
	byAction map[ActionID][]int
}

var registry = newRegistry(keyBindings)

func newRegistry(bindings []Binding) *keyRegistry {
	r := &keyRegistry{
		bindings: bindings,
		byScope:  make(map[Scope][]int),
		byAction: make(map[ActionID][]int),
	}
	for i, bd := range r.bindings {
		for _, s := range bd.Scopes {
			r.byScope[s] = append(r.byScope[s], i)
		}
		r.byAction[bd.Action] = append(r.byAction[bd.Action], i)
	}
	return r
}

func (r *keyRegistry) bindingFor(action ActionID) (Binding, bool) {
	idx := r.byAction[action]
	if len(idx) == 0 {
		return Binding{}, false
	}
	return r.bindings[idx[0]], true
}

func (r *keyRegistry) keyBinding(bd Binding, app *App) key.Binding {
	kb := key.NewBinding(
		key.WithKeys(bd.Keys...),
		key.WithHelp(bd.DisplayKey, T(bd.DescKey)),
	)
	if bd.Enabled != nil && app != nil && !bd.Enabled(app) {
		kb.SetEnabled(false)
	}
	return kb
}

// Resolve returns the first ActionID whose binding matches msg in scope order.
// Input and overlay scopes are modal: only their own bindings plus the small
// always-allowed set (force quit, help, reload, media keys) can fire.
func (r *keyRegistry) Resolve(scopes []Scope, msg tea.KeyMsg, app *App) (ActionID, bool) {
	top := Scope("")
	if len(scopes) > 0 {
		top = scopes[0]
	}
	modal := isInputScope(top) || isOverlayScope(top) || top == scopeConfirm
	for i, s := range scopes {
		for _, idx := range r.byScope[s] {
			bd := r.bindings[idx]
			if modal && i > 0 && !alwaysAllowedInModal(bd.Action, top) {
				continue
			}
			if bd.Enabled != nil && app != nil && !bd.Enabled(app) {
				continue
			}
			if key.Matches(msg, r.keyBinding(bd, app)) {
				return bd.Action, true
			}
		}
	}
	return "", false
}

func alwaysAllowedInModal(action ActionID, scope Scope) bool {
	if action == actionForceQuit {
		return true
	}
	if action == actionReload {
		return scope == scopeOverlaySettingsEdit || !isInputScope(scope)
	}
	if action == actionHelp {
		return !isInputScope(scope)
	}
	switch action {
	case actionMediaPlay, actionMediaStop, actionMediaNext, actionMediaPrev,
		actionMediaVolDown, actionMediaVolUp:
		return !isInputScope(scope)
	}
	return false
}

func isInputScope(s Scope) bool {
	switch s {
	case scopeInputSearch, scopeInputAddTracks, scopeInputName,
		scopeOverlayAddCreate, scopeOverlaySettingsEdit:
		return true
	}
	return false
}

func isOverlayScope(s Scope) bool {
	switch s {
	case scopeConfirm, scopeOverlayHelp, scopeOverlayInfo, scopeOverlayQueue,
		scopeOverlayAddList, scopeOverlayAddCreate, scopeOverlaySettingsBrowse,
		scopeOverlaySettingsEdit:
		return true
	}
	return false
}

// activeScopes returns scopes in fixed priority order.
func (a *App) activeScopes() []Scope {
	var scopes []Scope

	// 1. Text input scopes (modal).
	switch {
	case a.activeOvl == overlaySettings && a.settingsEditing:
		scopes = append(scopes, scopeOverlaySettingsEdit)
	case a.activeOvl == overlaySearch:
		scopes = append(scopes, scopeInputSearch)
	case a.activeOvl == overlayAddToPlaylist && a.overlayCreating:
		scopes = append(scopes, scopeOverlayAddCreate)
	case a.activeTab == tabPlaylist && a.inputMode != playlistInputNone:
		scopes = append(scopes, scopeInputName)
	case a.addingTracks:
		scopes = append(scopes, scopeInputAddTracks)
	}

	// 2. Confirm.
	if a.activeOvl == overlayDeletePlaylist {
		scopes = append(scopes, scopeConfirm)
	}

	// 3. Overlay layers (rendered above fullscreen).
	switch a.activeOvl {
	case overlayHelp:
		scopes = append(scopes, scopeOverlayHelp)
	case overlayInfo:
		scopes = append(scopes, scopeOverlayInfo)
	case overlayQueue:
		scopes = append(scopes, scopeOverlayQueue)
	case overlayAddToPlaylist:
		if !a.overlayCreating {
			scopes = append(scopes, scopeOverlayAddList)
		}
	case overlaySettings:
		if !a.settingsEditing {
			scopes = append(scopes, scopeOverlaySettingsBrowse)
		}
	}

	// 4. Fullscreen.
	if a.currentView == viewFullscreen {
		scopes = append(scopes, scopeFullscreen)
	}

	// 5. Group view.
	if a.activeTab == tabLocal && a.view != libraryViewTracks && a.currentView == viewNormal && a.activeOvl == overlayNone {
		scopes = append(scopes, scopeGroup, scopeLocalGroups, scopeLocalTracks, scopeTracklist)
	}

	// 6. Track-list scopes.
	if a.currentView == viewNormal && a.activeOvl == overlayNone {
		switch {
		case a.activeTab == tabPlaylist && a.depth == playlistDepthList:
			scopes = append(scopes, scopePlaylistList)
		case a.activeTab == tabPlaylist && a.depth == playlistDepthTracks:
			scopes = append(scopes, scopePlaylistTracks, scopeTracklist)
		case a.activeTab == tabLocal && a.view == libraryViewTracks:
			scopes = append(scopes, scopeLocalTracks, scopeTracklist)
		}
	}

	// 7. Global fallback.
	scopes = append(scopes, scopeGlobal)
	return scopes
}

func (a *App) resolveAction(msg tea.KeyMsg) (ActionID, bool) {
	return registry.Resolve(a.activeScopes(), msg, a)
}

// sortedBindings returns enabled bindings for one scope in fixed display order.
func (r *keyRegistry) sortedBindings(s Scope, app *App) []Binding {
	var out []Binding
	for _, idx := range r.byScope[s] {
		bd := r.bindings[idx]
		if bd.Enabled != nil && app != nil && !bd.Enabled(app) {
			continue
		}
		out = append(out, bd)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		if out[i].ShortRank != out[j].ShortRank {
			return out[i].ShortRank > out[j].ShortRank
		}
		return out[i].Action < out[j].Action
	})
	return out
}

// ValidateKeyRegistry checks the registry invariants at application startup.
func ValidateKeyRegistry() error { return registry.Validate() }

// Validate checks duplicate keys, forbidden encodings and i18n coverage.
func (r *keyRegistry) Validate() error {
	for s, idxs := range r.byScope {
		seen := map[string]ActionID{}
		for _, idx := range idxs {
			bd := r.bindings[idx]
			for _, k := range bd.Keys {
				if prev, ok := seen[k]; ok && prev != bd.Action {
					return fmt.Errorf("scope %s: key %q bound to both %s and %s", s, k, prev, bd.Action)
				}
				seen[k] = bd.Action
				lower := strings.ToLower(k)
				for bad, reason := range forbiddenKeys {
					if strings.HasPrefix(lower, bad) {
						return fmt.Errorf("scope %s: %s uses forbidden key %q: %s", s, bd.Action, k, reason)
					}
				}
				if strings.Contains(lower, "ctrl+shift+") {
					return fmt.Errorf("scope %s: %s uses forbidden ctrl+shift key %q", s, bd.Action, k)
				}
			}
		}
	}
	for _, k := range []string{"hint_resume", "hint_player", "hint_expand"} {
		for _, lang := range []Lang{LangEN, LangZH} {
			if strings.TrimSpace(translations[lang][k]) == "" {
				return fmt.Errorf("missing %v conditional label translation for %q", lang, k)
			}
		}
	}
	for _, bd := range r.bindings {
		for _, lang := range []Lang{LangEN, LangZH} {
			for _, k := range []string{bd.LabelKey, bd.DescKey} {
				if k == "" {
					continue
				}
				if strings.TrimSpace(translations[lang][k]) == "" {
					return fmt.Errorf("action %s: missing %v translation for %q", bd.Action, lang, k)
				}
			}
		}
	}
	return nil
}

// appKeyMap implements help.KeyMap from the registry.
type appKeyMap struct {
	app   *App
	scope Scope
}

var _ help.KeyMap = appKeyMap{}

func (m appKeyMap) ShortHelp() []key.Binding {
	var out []key.Binding
	for _, h := range m.app.shortHints(m.scope) {
		bd, ok := registry.bindingFor(h.action)
		if !ok {
			continue
		}
		out = append(out, registry.keyBinding(bd, m.app))
	}
	return out
}

func (m appKeyMap) FullHelp() [][]key.Binding {
	var out [][]key.Binding
	for _, sec := range m.app.helpSectionsRendered(m.scope) {
		var col []key.Binding
		for _, row := range sec.rows {
			bd, ok := registry.bindingFor(row.action)
			if !ok {
				continue
			}
			col = append(col, registry.keyBinding(bd, m.app))
		}
		out = append(out, col)
	}
	return out
}

func (a *App) keyMapFor(s Scope) help.KeyMap { return appKeyMap{app: a, scope: s} }
