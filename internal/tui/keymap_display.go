package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"

	"github.com/lonsty/music-tui/internal/audio"
)

// keyHint is one generated chip. Physical key names never appear outside the
// registry; renderers only use keyHint.key.
type keyHint struct {
	key    string
	label  string
	action ActionID
	slot   int
	pinned bool
}

type helpSection struct {
	title string
	rows  []keyHint
}

// shortSlots is the fixed six-slot T1 template. Slot 0/1/5 are pinned; narrow
// terminals drop slots 4, 3, 2 in that order.
var shortSlots = map[Scope][]ActionID{
	scopeLocalTracks:    {actionHelp, actionPlay, actionNavDown, actionNext, actionSearch, actionQuit},
	scopeLocalGroups:    {actionHelp, actionDrillDown, actionNavDown, actionNext, actionViewAlbums, actionQuit},
	scopeGroup:          {actionHelp, actionDrillDown, actionNavDown, actionNext, actionViewAlbums, actionQuit},
	scopePlaylistList:   {actionHelp, actionOpenPlaylist, actionNavDown, actionNext, actionPlaylistNew, actionQuit},
	scopePlaylistTracks: {actionHelp, actionPlay, actionNavDown, actionNext, actionPlaylistRemove, actionPlaylistBack},
	scopeFullscreen:     {actionHelp, actionPause, actionBrowseDown, actionSeekForward, actionOffsetLater, actionFullscreen},
}

// footerSlots is the T2 template. The cancel action is always last.
var footerSlots = map[Scope][]ActionID{
	scopeOverlayQueue:          {actionQueueRemove, actionQueueClear, actionQueueClose},
	scopeOverlayAddList:        {actionConfirm, actionPlaylistNew, actionCancel},
	scopeOverlayAddCreate:      {actionCreateAndAdd, actionCancel},
	scopeConfirm:               {actionDeleteConfirm, actionCancel},
	scopeInputName:             {actionCommitName, actionCancel},
	scopeInputAddTracks:        {actionAddTracksToggle, actionCancel},
	scopeInputSearch:           {actionSearchPlay, actionSearchCancel},
	scopeOverlaySettingsBrowse: {actionSettingsEdit, actionReload, actionSettingsClose},
	scopeOverlaySettingsEdit:   {actionSettingsCommit, actionSettingsCancel},
	scopeOverlayHelp:           {actionOverlayScroll, actionCancel},
	scopeOverlayInfo:           {actionOverlayScroll, actionCancel},
	scopeFullscreen:            {},
	scopeLocalTracks:           {},
	scopePlaylistTracks:        {},
	scopePlaylistList:          {},
	scopeGroup:                 {},
}

func hintFor(a *App, action ActionID, slot int, pinned bool) (keyHint, bool) {
	bd, ok := registry.bindingFor(action)
	if !ok {
		return keyHint{}, false
	}
	if bd.Enabled != nil && !bd.Enabled(a) {
		return keyHint{}, false
	}
	labelKey := bd.LabelKey
	switch {
	case action == actionPause && a.framePlayback.state == audio.StatePaused:
		labelKey = "hint_resume"
	case action == actionRightPanel && a.rightMode == rightPanelLyrics:
		labelKey = "hint_player"
	case action == actionCollapse && a.rightCollapsed:
		labelKey = "hint_expand"
	}
	return keyHint{
		key:    bd.ShortKey,
		label:  T(labelKey),
		action: action,
		slot:   slot,
		pinned: pinned,
	}, true
}

func (a *App) shortHints(s Scope) []keyHint {
	slots, ok := shortSlots[s]
	if !ok {
		return nil
	}
	var out []keyHint
	for i, action := range slots {
		if action == "" {
			continue
		}
		// Primary slot follows playback state only where the slot is a play
		// action; group and playlist-list slots keep their own primary action.
		if i == 1 && (s == scopeLocalTracks || s == scopePlaylistTracks) {
			if a.framePlayback.state == audio.StatePlaying || a.framePlayback.state == audio.StatePaused {
				action = actionPause
			}
		}
		h, ok := hintFor(a, action, i, i == 0 || i == 1 || i == 5)
		if ok {
			out = append(out, h)
		}
	}
	return out
}

func (a *App) footerHints(s Scope) []keyHint {
	actions, ok := footerSlots[s]
	if !ok {
		return nil
	}
	var out []keyHint
	for i, action := range actions {
		h, ok := hintFor(a, action, i, i == len(actions)-1)
		if ok {
			out = append(out, h)
		}
	}
	return out
}

// dropShortHints removes non-pinned chips until the rendered width fits.
func dropShortHints(hints []keyHint, hintWidth func(keyHint) int, available int) []keyHint {
	total := 0
	for _, h := range hints {
		total += hintWidth(h)
	}
	kept := append([]keyHint(nil), hints...)
	for total > available {
		drop := -1
		// Prefer higher slot numbers (context, playback, navigation).
		for i := len(kept) - 1; i >= 0; i-- {
			if !kept[i].pinned {
				drop = i
				break
			}
		}
		if drop < 0 {
			break
		}
		total -= hintWidth(kept[drop])
		kept = append(kept[:drop], kept[drop+1:]...)
	}
	return kept
}

func hintWidth(h keyHint) int {
	return 1 + strWidth(h.key) + 1 + 1 + strWidth(h.label) + 1
}

func (a *App) helpSections(current Scope) []helpSection {
	seen := map[ActionID]bool{}
	collect := func(scopes ...Scope) []keyHint {
		var rows []keyHint
		for _, s := range scopes {
			for _, bd := range registry.sortedBindings(s, a) {
				if seen[bd.Action] || mediaActions[bd.Action] {
					continue
				}
				seen[bd.Action] = true
				rows = append(rows, keyHint{key: bd.DisplayKey, label: T(bd.DescKey), action: bd.Action})
			}
		}
		return rows
	}

	var media []keyHint
	for _, bd := range registry.sortedBindings(scopeGlobal, a) {
		if mediaActions[bd.Action] {
			media = append(media, keyHint{key: bd.DisplayKey, label: T(bd.DescKey), action: bd.Action})
		}
	}
	return []helpSection{
		{T("help_section_current"), collect(current)},
		{T("help_section_global"), collect(scopeGlobal)},
		{T("help_section_local"), collect(scopeLocalTracks, scopeLocalGroups, scopeGroup)},
		{T("help_section_playlist"), collect(scopePlaylistList, scopePlaylistTracks, scopeOverlayAddList, scopeOverlayAddCreate)},
		{T("help_section_media"), media},
	}
}

// mediaActions limits the last help section to media-key bindings.
var mediaActions = map[ActionID]bool{
	actionMediaPlay: true, actionMediaStop: true, actionMediaNext: true,
	actionMediaPrev: true, actionMediaVolDown: true, actionMediaVolUp: true,
}

func mediaRows(rows []keyHint) []keyHint {
	var out []keyHint
	for _, r := range rows {
		if mediaActions[r.action] {
			out = append(out, r)
		}
	}
	return out
}

// helpSectionsRendered applies media filtering and removes empty sections.
func (a *App) helpSectionsRendered(current Scope) []helpSection {
	sections := a.helpSections(current)
	sections[4].rows = mediaRows(sections[4].rows)
	var out []helpSection
	for _, s := range sections {
		if len(s.rows) > 0 {
			out = append(out, s)
		}
	}
	return out
}

// ReadmeKeyTables renders the generated README block. The generator sets LangZH
// before calling it.
func ReadmeKeyTables() string {
	var sb strings.Builder
	scopes := []struct {
		title  string
		scopes []Scope
	}{
		{"全局 / Global", []Scope{scopeGlobal}},
		{"普通模式 / Normal", []Scope{scopeLocalTracks, scopeTracklist}},
		{"分组视图 / Groups", []Scope{scopeGroup, scopeLocalGroups, scopeLocalTracks, scopeTracklist}},
		{"歌单列表 / Playlists", []Scope{scopePlaylistList}},
		{"歌单曲目 / Playlist tracks", []Scope{scopePlaylistTracks, scopeTracklist}},
		{"全屏 / Fullscreen", []Scope{scopeFullscreen}},
		{"队列浮层 / Queue", []Scope{scopeOverlayQueue}},
		{"设置面板 / Settings", []Scope{scopeOverlaySettingsBrowse, scopeOverlaySettingsEdit}},
	}
	for _, s := range scopes {
		sb.WriteString(fmt.Sprintf("### %s\n\n| 键 | 功能 |\n|----|------|\n", s.title))
		seen := map[ActionID]bool{}
		for _, scope := range s.scopes {
			for _, bd := range registry.sortedBindings(scope, nil) {
				if seen[bd.Action] {
					continue
				}
				seen[bd.Action] = true
				sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", bd.DisplayKey, T(bd.DescKey)))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// currentContextScope derives the underlying page even while an overlay is
// open, so the help overlay can show the correct current-context section.
func (a *App) currentContextScope() Scope {
	if a.currentView == viewFullscreen {
		return scopeFullscreen
	}
	if a.activeTab == tabPlaylist {
		if a.depth == playlistDepthTracks {
			return scopePlaylistTracks
		}
		return scopePlaylistList
	}
	if a.view != libraryViewTracks {
		return scopeGroup
	}
	return scopeLocalTracks
}

// keyBindingFor exposes one binding for tests and the README generator.
func keyBindingFor(action ActionID) (key.Binding, bool) {
	bd, ok := registry.bindingFor(action)
	if !ok {
		return key.Binding{}, false
	}
	return registry.keyBinding(bd, nil), true
}
