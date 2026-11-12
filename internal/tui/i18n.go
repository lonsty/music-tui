package tui

import "sync/atomic"

// Lang represents a UI display language.
type Lang int

const (
	// LangEN is American English (default).
	LangEN Lang = iota
	// LangZH is Simplified Chinese.
	LangZH
)

// langSnapshot is an immutable (language, translation table) pair.  View and
// Update may run on different goroutines, so the active language is published
// as a single atomic pointer instead of two plain globals.
type langSnapshot struct {
	lang  Lang
	table map[string]string
}

var activeLang atomic.Pointer[langSnapshot]

// SetLang changes the active UI language for the current process.
// It is safe to call at any time; the change takes effect on the next render.
func SetLang(l Lang) {
	activeLang.Store(&langSnapshot{lang: l, table: translations[l]})
}

// ActiveLang returns the currently active UI language.
func ActiveLang() Lang { return activeLang.Load().lang }

// T returns the localised string for key in the active language.
// Falls back to English when the key is absent in the active language.
// Returns key itself when not found in any language, so missing translations
// degrade gracefully rather than panicking.
//
// Performance: the snapshot holds the inner translation map directly, so the
// common case is one pointer load plus one map lookup.
func T(key string) string {
	snap := activeLang.Load()
	if s, ok := snap.table[key]; ok {
		return s
	}
	// Fallback path: look up in English (only reached when the active language
	// is missing a key, which should not happen in production).
	if snap.lang != LangEN {
		if s, ok := translations[LangEN][key]; ok {
			return s
		}
	}
	return key
}

// translations is the complete UI string table.
// Add new languages by inserting a new key in the outer map and providing
// values for every key defined in the LangEN map.
var translations = map[Lang]map[string]string{LangEN: {
	// ── Player ───────────────────────────────────────────────────────────
	"no_track_selected":         "No track selected",
	"press_enter_to_play":       "No track playing",
	"no_lyrics":                 "No lyrics",
	"lyrics_loading":            "loading lyrics…", // icon prepended at call site
	"lyrics_hint_lrc":           "Place a .lrc file next to the audio file",
	"lyrics_panel_title":        "Lyrics",
	"lyrics_offset_status":      "Lyrics offset %+.1fs",
	"lyrics_offset_synced_only": "Timing offset only applies to synced lyrics",

	// ── Status bar ───────────────────────────────────────────────────────
	"state_stopped":       "  Stopped ",
	"state_playing":       "  Playing ",
	"state_paused":        "  Paused  ",
	"scanning_library":    "Scanning library…",
	"hint_pause":          "Pause",
	"hint_resume":         "Resume",
	"hint_back":           "Back",
	"hint_next":           "Next",
	"hint_prev":           "Prev",
	"hint_seek":           "Seek",
	"hint_vol":            "Vol",
	"hint_mode":           "Mode",
	"hint_quit":           "Quit",
	"hint_search":         "Search",
	"hint_help":           "Help",
	"hint_play":           "Play",
	"hint_settings":       "Settings",
	"hint_lyrics":         "Lyrics",
	"hint_player":         "Player",
	"hint_collapse":       "Collapse",
	"hint_expand":         "Expand",
	"hint_switch_tab":     "Tab",
	"hint_next_prev":      "Next/Prev",
	"hint_fullscreen":     "Fullscreen",
	"hint_enter_playlist": "Open",
	"hint_retro":          "Retro",
	"hint_like_toggle":    "Like",
	"hint_view":           "View",
	"hint_open_group":     "Open",
	"hint_jump_playing":   "Now playing",
	"hint_page":           "Page",
	"hint_queue":          "Queue",
	"hint_reload":         "Reload",
	"hint_undo":           "Undo",
	"hint_move_track":     "Move",
	"hint_scroll":         "Scroll",
	"chip_converting":     " 8-bit Converting… ",
	"chip_switching":      " 8-bit Switching… ",
	"chip_busy":           " 8-bit… ",
	"chip_active":         " 8-bit ",

	// ── Overlays ─────────────────────────────────────────────────────────
	"help_title":         "Keyboard shortcuts",
		"help_close_key":     "Close this panel",

	"action_move_down":       "Move down",
	"action_move_up":         "Move up",
	"action_jump_playing":    "Jump to playing track",
	"action_play":            "Play selected track",
	"action_pause_resume":    "Pause / Resume",
	"action_cycle_mode":      "Cycle play mode",
	"action_queue_open":      "Open play queue",
	"action_chip":            "Toggle 8-bit chip mode  (converts + crossfades)",
	"action_settings":        "Settings  (music dir · p2chip options)",
	"action_search":          "Search  (s: artist  a: album  t: title  f: format)",
	"action_track_info":      "Track info",
	"action_fullscreen":      "Toggle fullscreen",
	"action_lyrics_mode":     "Toggle lyrics / player panel",
	"action_collapse_panel":  "Collapse / expand right panel",
	"action_switch_tab":      "Switch tab",
	"action_this_help":       "This help",
	"action_quit":            "Quit",
	"action_media_play":      "Play / Pause  (media key mapping)",
	"action_media_next":      "Next track  (media key mapping)",
	"hint_nav": "Down/Up",
	"hint_browse": "Browse lyrics",
	"hint_offset": "Lyrics offset",
	"action_next": "Next track",
	"action_prev": "Previous track",
	"action_seek_back": "Seek −5s",
	"action_seek_forward": "Seek +5s",
	"action_volume_up": "Volume up",
	"action_volume_down": "Volume down",
	"action_lofi_down": "Lo-fi lower sample rate",
	"action_lofi_up": "Lo-fi raise sample rate",
	"action_offset_earlier": "Lyrics earlier (0.5s)",
	"action_offset_later": "Lyrics later (0.5s)",
	"action_top": "Top",
	"action_bottom": "Bottom",
	"action_page_down": "Page down",
	"action_page_up": "Page up",
	"action_browse_up": "Browse lyrics up",
	"action_browse_down": "Browse lyrics down",
	"action_scroll_up": "Scroll up",
	"action_scroll_down": "Scroll down",
	"action_queue_append": "Add to queue tail",
	"action_queue_insert_next": "Play next",
	"action_move_track_down": "Move track down",
	"action_move_track_up": "Move track up",
	"action_media_prev": "Previous track (media key mapping)",
	"action_media_vol_down": "Volume down (media key mapping)",
	"action_media_vol_up": "Volume up (media key mapping)",
	"action_media_stop":      "Stop  (media key mapping)",

	"action_like":            "Toggle like / Favorites",
	"action_add_to_playlist": "Add to a playlist",
	"action_reload_library":  "Reload library",
	"action_open_playlist":   "Open playlist",
	"action_new_playlist":    "New playlist",
	"action_rename_playlist": "Rename playlist",
	"action_delete_playlist": "Delete playlist",
	"action_remove_track":    "Remove track",
	"action_undo":            "Undo remove  (5 seconds)",

	"action_cancel":          "Cancel / close",
	"action_force_quit":      "Quit immediately",
	"action_seek_line":       "Seek to selected lyric line",
	"action_confirm":         "Confirm",
	"action_save":            "Save",
	"action_create_add":      "Create playlist and add the track",
	"action_add_track":       "Add or remove track",
	"action_settings_edit":   "Edit field / toggle value",
	"action_close":           "Close",
	"action_clear_filter":    "Clear search filter",
	"action_playlist_back":   "Back to playlist list",
	"action_back":            "Back",
	"action_scroll":          "Scroll",
	"action_queue_clear":     "Clear play queue",
	"action_view_tracks":     "Show tracks view",
	"action_view_albums":     "Show albums view",
	"action_view_artists":    "Show artists view",
	"action_open_group":      "Open group",

	"hint_chip":              "Chip",
	"help_section_current":   "── Current context ──────────────────────────",
	"help_section_global":   "── Global ────────────────────────────────",
	"help_section_local":    "── Local Tab ─────────────────────────────",
	"help_section_playlist": "── Playlists Tab ─────────────────────────",
	"help_section_media":    "── Media Keys ────────────────────────────",

	"info_title":    "Track Info",
	"info_no_track": "No track selected",

	"label_title":        "Title",
	"label_artist":       "Artist",
	"label_album_artist": "Album Artist",
	"label_album":        "Album",
	"label_year":         "Year",
	"label_track":        "Track",
	"label_genre":        "Genre",
	"label_comment":      "Comment",
	"label_duration":     "Duration",
	"label_format":       "Format",
	"label_path":         "Path",

	"settings_title":        "Settings",
	"settings_section_lib":  "Music Library",
	"settings_section_chip": "8-bit Conversion  (p2chip)",
	"settings_dir_label":    "Directory",
	"settings_opts_label":   "Options",
	"settings_reload_hint":  "reload library  (adds new · removes missing)",
	"settings_opts_hint":    "Extra options appended to the p2chip command.",
	"settings_opts_example": "e.g.  --sf2 nes --onset 0.6",
	"settings_save":         "save",
	"settings_save_error":   "Could not save settings: %v",
	"settings_cancel":       "cancel",
		"settings_edit":         "edit / toggle",
	"settings_lang_label":   "Language",
	"settings_lang_en":      "English",
	"settings_lang_zh":      "中文",

	// ── Format preference ────────────────────────────────────────────────
	"settings_fmt_label":      "Format filter",
	"fmt_pref_all":            "All formats",
	"fmt_pref_lossless_first": "Lossless first",
	"fmt_pref_lossless_only":  "Lossless only",
	"fmt_pref_mp3_only":       "MP3 only",

	// ── Icon set ─────────────────────────────────────────────────────────
	"settings_icon_set_label": "Icons",
	"icon_set_nerd":           "Nerd Font",
	"icon_set_emoji":          "Emoji",
	"icon_set_plain":          "Plain",

	// ── Tabs ─────────────────────────────────────────────────────────────
	"tab_local":    "Local",
	"tab_playlist": "Playlists",

	// ── Library panel header ──────────────────────────────────────────────
	"library_title":        "Library",
	"library_view_albums":  "Albums",
	"library_view_artists": "Artists",
	"unknown_album":        "Unknown Album",
	"clear_search_first":   "Clear the search first",
	// library_count: single format arg = total filtered track count
	"library_count": "%d tracks",
	// library_count_playing: two format args = current position, total
	// shown when a track from the filtered list is playing.
	"library_count_playing": "%d/%d tracks",

	// ── App messages ─────────────────────────────────────────────────────
	"track_removed":        "Playing track was removed from library",
	"no_tracks_hint":       "No tracks — open Settings and reload the library",
	"search_placeholder":   "Search… (s: artist  a: album  t: title  f: format)",
	"sync_error":           "Sync error: %s",
	"loaded_tracks":        "Loaded %d tracks",
	"reloading_library":    "Reloading library…",
	"dir_not_found":        "Directory not found: %s",
	"chip_convert_failed":  "8-bit convert failed: %s",
	"playback_error":       "Playback failed: %s",
	"unknown_artist":       "Unknown Artist",
	"search_filter_active": "Search: %s",
	"search_clear_hint":    "Clear the filter",
	"no_search_results":    "No results for \"%s\"",
	"track_not_in_list":    "Playing track is not in this list",

	// ── Playlist tab ─────────────────────────────────────────────────────
	"playlist_header":         "Playlists",
	"playlist_favorites_name": "Favorites",
	"playlist_empty":          "No playlists yet",
	"playlist_renamed":        "Renamed to %s",
	"playlist_tracks_empty":   "No tracks yet",
	// playlist_tracks_missing: single arg = count of missing tracks
	"playlist_tracks_missing": "%d track(s) not in library — skipped",
	// playlist_track_count: single arg = count
	"playlist_track_count":    "%d tracks",
	"playlist_new_prompt":     "New playlist name",
	"playlist_rename_prompt":  "Rename to",
	"playlist_delete_confirm": "Delete this playlist?",
	"playlist_delete_title":   "Delete Playlist",
	"playlist_load_error":     "Failed to load playlists",
	"playlist_added":          "Added to %s",
		"playlist_removed_detail": "Removed %s",
	// playlist_add_remove_summary: two args = addCount, removeCount
	"playlist_add_remove_summary": "Added %d, removed %d tracks",
	"playlist_back_hint":          "← Back",
	"playlist_add_no_results":     "No matching tracks",
	"overlay_add":                 "add",
	"overlay_confirm":             "confirm",
	"overlay_cancel":              "cancel",
	"quit_confirm":                "Confirm quit",
	"playlist_add_title":          "Add to Playlist",
	"playlist_create_add_title":   "New Playlist",
	"playlist_created_added":      "Created %s and added the track",
	"playlist_duplicate":          "Already in the playlist",
	"undo_restored":               "Restored %s",
	"undo_expired":                "Undo expired",
	// ── Queue ────────────────────────────────────────────────────────────
	"queue_title":                "Play Queue",
	"queue_empty":                "Queue is empty",
	"queue_duplicate":            "Already in the queue",
	"queue_added_next":           "Queued to play next",
	"queue_added_back":           "Added to the queue",
	"queue_remove":               "remove",
	"queue_clear":                "clear all",
	"queue_close":                "close",
	"queue_missing_track":        "(missing track)",
	"queue_count":                "%d queued",
	"hint_playlist_new":          "New",
	"hint_playlist_rename":       "Rename",
	"hint_playlist_delete":       "Delete",
	"hint_playlist_add_track":    "Add tracks",
	"hint_playlist_remove_track": "Remove",
},
	LangZH: {
		// ── Player ───────────────────────────────────────────────────────────
		"no_track_selected":         "未选择曲目",
		"press_enter_to_play":       "尚未播放曲目",
		"no_lyrics":                 "无歌词",
		"lyrics_loading":            "歌词加载中…",
		"lyrics_hint_lrc":           "将 .lrc 文件放在音频文件旁边",
		"lyrics_panel_title":        "歌词",
		"lyrics_offset_status":      "歌词偏移 %+.1fs",
		"lyrics_offset_synced_only": "仅同步歌词支持时间偏移",

		// ── Status bar ───────────────────────────────────────────────────────
		"state_stopped":       "  已停止 ",
		"state_playing":       "  播放中 ",
		"state_paused":        "  已暂停 ",
		"scanning_library":    "正在扫描音乐库…",
		"hint_pause":          "暂停",
		"hint_resume":         "继续",
		"hint_back":           "返回",
		"hint_next":           "下一首",
		"hint_prev":           "上一首",
		"hint_seek":           "快退/进",
		"hint_vol":            "音量",
		"hint_mode":           "模式",
		"hint_quit":           "退出",
		"hint_search":         "搜索",
		"hint_help":           "帮助",
		"hint_play":           "播放",
		"hint_settings":       "设置",
		"hint_lyrics":         "歌词",
		"hint_player":         "播放器",
		"hint_collapse":       "折叠",
		"hint_expand":         "展开",
		"hint_switch_tab":     "切换标签",
		"hint_next_prev":      "下/上一首",
		"hint_fullscreen":     "全屏",
		"hint_enter_playlist": "进入歌单",
		"hint_retro":          "复古效果",
		"hint_like_toggle":    "收藏",
		"hint_view":           "视图",
		"hint_open_group":     "打开",
		"hint_jump_playing":   "正在播放",
		"hint_page":           "翻页",
		"hint_queue":          "队列",
		"hint_reload":         "重扫",
		"hint_undo":           "撤销",
		"hint_move_track":     "移动",
		"hint_scroll":         "滚动",
		"chip_converting":     " 8-bit 转换中… ",
		"chip_switching":      " 8-bit 切换中… ",
		"chip_busy":           " 8-bit… ",
		"chip_active":         " 8-bit ",

		// ── Overlays ─────────────────────────────────────────────────────────
		"help_title":         "快捷键",
				"help_close_key":     "关闭面板",

		"action_move_down":       "向下移动",
		"action_move_up":         "向上移动",
		"action_jump_playing":    "跳到正在播放",
		"action_play":            "播放选中曲目",
		"action_pause_resume":    "暂停 / 继续",
		"action_cycle_mode":      "切换播放模式",
		"action_queue_open":      "打开播放队列",
		"action_chip":            "切换 8-bit 音效（转换 + 淡入淡出）",
		"action_settings":        "设置（音乐目录 · p2chip 选项）",
		"action_search":          "搜索（s: 艺术家  a: 专辑  t: 标题  f: 格式）",
		"action_track_info":      "曲目信息",
		"action_fullscreen":      "切换全屏",
		"action_lyrics_mode":     "切换歌词 / 播放器面板",
		"action_collapse_panel":  "折叠 / 展开右侧面板",
		"action_switch_tab":      "切换标签页",
		"action_this_help":       "显示此帮助",
		"action_quit":            "退出",
		"action_media_play":      "播放 / 暂停（媒体键映射）",
		"action_media_next":      "下一首（媒体键映射）",
		"hint_nav": "下/上一首",
		"hint_browse": "浏览歌词",
		"hint_offset": "歌词偏移",
		"action_next": "下一首",
		"action_prev": "上一首",
		"action_seek_back": "快退 5 秒",
		"action_seek_forward": "快进 5 秒",
		"action_volume_up": "音量增大",
		"action_volume_down": "音量减小",
		"action_lofi_down": "Lo-Fi 降档（降低采样率）",
		"action_lofi_up": "Lo-Fi 升档（提升采样率）",
		"action_offset_earlier": "歌词提前 0.5 秒",
		"action_offset_later": "歌词延后 0.5 秒",
		"action_top": "跳到顶部",
		"action_bottom": "跳到底部",
		"action_page_down": "向下翻页",
		"action_page_up": "向上翻页",
		"action_browse_up": "向上浏览歌词",
		"action_browse_down": "向下浏览歌词",
		"action_scroll_up": "向上滚动",
		"action_scroll_down": "向下滚动",
		"action_queue_append": "加入队列末尾",
		"action_queue_insert_next": "插到下一首播放",
		"action_move_track_down": "曲目下移一位",
		"action_move_track_up": "曲目上移一位",
		"action_media_prev": "上一首（媒体键映射）",
		"action_media_vol_down": "音量减小（媒体键映射）",
		"action_media_vol_up": "音量增大（媒体键映射）",
		"action_media_stop":      "停止（媒体键映射）",

		"action_like":            "收藏 / 取消收藏",
		"action_add_to_playlist": "加入歌单",
		"action_reload_library":  "重新加载曲库",
		"action_open_playlist":   "打开歌单",
		"action_new_playlist":    "新建歌单",
		"action_rename_playlist": "重命名歌单",
		"action_delete_playlist": "删除歌单",
		"action_remove_track":    "移除曲目",
		"action_undo":            "撤销移除（5 秒内）",

		"action_cancel":          "取消 / 关闭",
		"action_force_quit":      "立即退出",
		"action_seek_line":       "定位到选中歌词行",
		"action_confirm":         "确认",
		"action_save":            "保存",
		"action_create_add":      "新建歌单并加入当前曲目",
		"action_add_track":       "添加或移除曲目",
		"action_settings_edit":   "编辑字段 / 切换取值",
		"action_close":           "关闭",
		"action_clear_filter":    "清除搜索筛选",
		"action_playlist_back":   "返回歌单列表",
		"action_back":            "返回",
		"action_scroll":          "滚动",
		"action_queue_clear":     "清空播放队列",
		"action_view_tracks":     "显示曲目视图",
		"action_view_albums":     "显示专辑视图",
		"action_view_artists":    "显示艺术家视图",
		"action_open_group":      "打开分组",

		"hint_chip":              "芯片",
		"help_section_current":   "── 当前上下文 ────────────────────────────",
		"help_section_global":   "── 全局 ──────────────────────────────────",
		"help_section_local":    "── 本地标签 ──────────────────────────────",
		"help_section_playlist": "── 歌单标签 ──────────────────────────────",
		"help_section_media":    "── 媒体键 ────────────────────────────────",

		"info_title":    "曲目信息",
		"info_no_track": "未选择曲目",

		"label_title":        "标题",
		"label_artist":       "艺术家",
		"label_album_artist": "专辑艺术家",
		"label_album":        "专辑",
		"label_year":         "年份",
		"label_track":        "曲目号",
		"label_genre":        "流派",
		"label_comment":      "备注",
		"label_duration":     "时长",
		"label_format":       "格式",
		"label_path":         "路径",

		"settings_title":        "设置",
		"settings_section_lib":  "音乐库",
		"settings_section_chip": "8-bit 转换（p2chip）",
		"settings_dir_label":    "目录",
		"settings_opts_label":   "选项",
		"settings_reload_hint":  "重新加载音乐库（添加新文件 · 删除缺失文件）",
		"settings_opts_hint":    "附加到 p2chip 命令的额外选项。",
		"settings_opts_example": "例：--sf2 nes --onset 0.6",
		"settings_save":         "保存",
		"settings_save_error":   "保存设置失败：%v",
		"settings_cancel":       "取消",
				"settings_edit":         "编辑 / 切换",
		"settings_lang_label":   "语言",
		"settings_lang_en":      "English",
		"settings_lang_zh":      "中文",

		// ── Format preference ────────────────────────────────────────────────
		"settings_fmt_label":      "格式筛选",
		"fmt_pref_all":            "显示全部",
		"fmt_pref_lossless_first": "无损优先",
		"fmt_pref_lossless_only":  "仅无损",
		"fmt_pref_mp3_only":       "仅 MP3",

		// ── Icon set ─────────────────────────────────────────────────────────
		"settings_icon_set_label": "图标风格",
		"icon_set_nerd":           "Nerd Font",
		"icon_set_emoji":          "Emoji",
		"icon_set_plain":          "纯文字",

		// ── Tabs ─────────────────────────────────────────────────────────────
		"tab_local":    "本地",
		"tab_playlist": "歌单",

		// ── Library panel header ──────────────────────────────────────────────
		"library_title":         "音乐库",
		"library_view_albums":   "专辑",
		"library_view_artists":  "艺术家",
		"unknown_album":         "未知专辑",
		"clear_search_first":    "请先清除搜索",
		"library_count":         "%d 首",
		"library_count_playing": "%d/%d 首",

		// ── App messages ─────────────────────────────────────────────────────
		"track_removed":        "当前播放曲目已从音乐库中移除",
		"no_tracks_hint":       "没有曲目 — 打开设置并重新加载音乐库",
		"search_placeholder":   "搜索… (s: 艺术家  a: 专辑  t: 标题  f: 格式)",
		"sync_error":           "同步出错：%s",
		"loaded_tracks":        "已加载 %d 首",
		"reloading_library":    "正在重新加载音乐库…",
		"dir_not_found":        "目录不存在：%s",
		"chip_convert_failed":  "8-bit 转换失败：%s",
		"playback_error":       "播放失败：%s",
		"unknown_artist":       "未知艺术家",
		"search_filter_active": "搜索：%s",
		"search_clear_hint":    "清除筛选",
		"no_search_results":    "没有匹配“%s”的结果",
		"track_not_in_list":    "当前播放曲目不在当前列表中",

		// ── Playlist tab ─────────────────────────────────────────────────────
		"playlist_header":             "歌单",
		"playlist_favorites_name":     "我的最爱",
		"playlist_empty":              "暂无歌单",
		"playlist_renamed":            "已重命名为 %s",
		"playlist_tracks_empty":       "暂无曲目",
		"playlist_tracks_missing":     "%d 首曲目已不在库中 — 已跳过",
		"playlist_track_count":        "%d 首",
		"playlist_new_prompt":         "歌单名称",
		"playlist_rename_prompt":      "重命名为",
		"playlist_delete_confirm":     "确定删除此歌单？",
		"playlist_delete_title":       "删除歌单",
		"playlist_load_error":         "加载歌单失败",
		"playlist_added":              "已加入 %s",
				"playlist_removed_detail":     "已移除 %s",
		"playlist_add_remove_summary": "已添加 %d 首，已移除 %d 首",
		"playlist_back_hint":          "← 返回",
		"playlist_add_no_results":     "没有匹配的曲目",
		"overlay_add":                 "添加",
		"overlay_confirm":             "确认",
		"overlay_cancel":              "取消",
		"quit_confirm":                "确认退出",
		"playlist_add_title":          "加入歌单",
		"playlist_create_add_title":   "新建歌单",
		"playlist_created_added":      "已创建 %s 并添加曲目",
		"playlist_duplicate":          "已在歌单中",
		"undo_restored":               "已恢复 %s",
		"undo_expired":                "撤销已过期",
		// ── Queue ────────────────────────────────────────────────────────────
		"queue_title":                "播放队列",
		"queue_empty":                "队列为空",
		"queue_duplicate":            "已在队列中",
		"queue_added_next":           "已加入下一首播放",
		"queue_added_back":           "已加入队列",
		"queue_remove":               "移除",
		"queue_clear":                "清空",
		"queue_close":                "关闭",
		"queue_missing_track":        "（曲目已缺失）",
		"queue_count":                "队列 %d",
		"hint_playlist_new":          "新建",
		"hint_playlist_rename":       "重命名",
		"hint_playlist_delete":       "删除",
		"hint_playlist_add_track":    "添加曲目",
		"hint_playlist_remove_track": "移除",
	},
}

func init() {
	// Initialise the active snapshot after the translations var is ready.
	// This guarantees T() works correctly even before SetLang is called.
	SetLang(LangEN)
}
