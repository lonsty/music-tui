package tui

// Lang represents a UI display language.
type Lang int

const (
	// LangEN is American English (default).
	LangEN Lang = iota
	// LangZH is Simplified Chinese.
	LangZH
)

// activeLang is the current UI language.  Change it via SetLang.
var activeLang = LangEN

// activeLangMap caches the translation map for the current language so that
// T() can perform a single map lookup on the hot render path instead of two
// (once for the active language, once for the English fallback).
// Updated atomically by SetLang.
var activeLangMap map[string]string

// SetLang changes the active UI language for the current process.
// It is safe to call before or after NewApp; the change takes effect on the
// next render tick.
func SetLang(l Lang) {
	activeLang = l
	activeLangMap = translations[l]
}

// ActiveLang returns the currently active UI language.
func ActiveLang() Lang { return activeLang }

// T returns the localised string for key in the active language.
// Falls back to English when the key is absent in the active language.
// Returns key itself when not found in any language, so missing translations
// degrade gracefully rather than panicking.
//
// Performance: activeLangMap is a direct reference to the inner translation
// map, making the common case a single map lookup with no intermediate
// allocation.
func T(key string) string {
	if s, ok := activeLangMap[key]; ok {
		return s
	}
	// Fallback path: look up in English (only reached when the active language
	// is missing a key, which should not happen in production).
	if activeLang != LangEN {
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
	"no_track_selected":   "No track selected",
	"press_enter_to_play": "Press Enter to play",
	"no_lyrics":           "No lyrics",
	"lyrics_loading":      "loading lyrics…", // icon prepended at call site
	"lyrics_hint_lrc":     "Place a .lrc file next to the audio file",
	"lyrics_panel_title":  "Lyrics",

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
	"chip_converting":     " 8-bit Converting… ",
	"chip_switching":      " 8-bit Switching… ",
	"chip_busy":           " 8-bit… ",
	"chip_active":         " 8-bit ",

	// ── Overlays ─────────────────────────────────────────────────────────
	"help_title":         "Keyboard shortcuts",
	"help_close":         "Any key to close",
	"help_close_key":     "Esc / ? to close  ·  ↑↓ to scroll",
	"overlay_hint_close": "Esc to close  ·  ↑↓ to scroll",

	"action_move_down":       "Move down",
	"action_move_up":         "Move up",
	"action_top_bottom":      "Top / Bottom",
	"action_jump_playing":    "Jump to playing track",
	"action_page":            "Page up / down",
	"action_view":            "Cycle view  Tracks / Albums / Artists",
	"action_play":            "Play  (2nd Enter → Fullscreen)",
	"action_pause_resume":    "Pause / Resume",
	"action_next_prev":       "Next / Previous",
	"action_seek":            "Seek −5s / +5s",
	"action_cycle_mode":      "Cycle play mode",
	"action_chip":            "Toggle 8-bit chip mode  (converts + crossfades)",
	"action_lofi":            "Lo-fi effect  lower / raise sample rate",
	"action_settings":        "Settings  (music dir · p2chip options · Ctrl+R reload)",
	"action_search":          "Search  (s: artist  a: album  t: title  f: format)",
	"action_track_info":      "Track info",
	"action_fullscreen":      "Toggle fullscreen",
	"action_lyrics_mode":     "Toggle lyrics / player panel",
	"action_collapse_panel":  "Collapse / expand right panel",
	"action_volume":          "Volume up / down",
	"action_switch_tab":      "Switch tab",
	"action_this_help":       "This help",
	"action_quit":            "Quit",
	"action_media_prev_next": "Prev / Next  (media key mapping)",
	"action_media_play":      "Play / Pause  (media key mapping)",
	"action_media_vol":       "Vol down / up  (media key mapping)",

	"action_like":            "Toggle like / Favorites",
	"action_reload_library":  "Reload library",
	"action_open_playlist":   "Open playlist",
	"action_new_playlist":    "New playlist",
	"action_rename_playlist": "Rename playlist",
	"action_delete_playlist": "Delete playlist",
	"action_remove_track":    "Remove track",

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
	"settings_cancel":       "cancel",
	"settings_switch":       "switch field",
	"settings_edit":         "edit / toggle",
	"settings_navigate":     "navigate",
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
	"no_tracks_hint":       "No tracks — open Settings (,) and reload the library",
	"search_placeholder":   "Search… (s: artist  a: album  t: title  f: format)",
	"sync_error":           "Sync error: %s",
	"loaded_tracks":        "Loaded %d tracks",
	"reloading_library":    "Reloading library…",
	"dir_not_found":        "Directory not found: %s",
	"chip_convert_failed":  "8-bit convert failed: %s",
	"playback_error":       "Playback failed: %s",
	"unknown_artist":       "Unknown Artist",
	"search_filter_active": "Search: %s  ·  Esc to clear",
	"search_clear_hint":    "Press Esc to clear the filter",
	"no_search_results":    "No results for \"%s\"",
	"track_not_in_list":    "Playing track is not in this list",

	// ── Playlist tab ─────────────────────────────────────────────────────
	"playlist_header":         "Playlists",
	"playlist_favorites_name": "Favorites",
	"playlist_empty":          "No playlists yet",
	"playlist_renamed":        "Renamed to %s",
	"playlist_tracks_empty":   "No tracks yet — press a to add",
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
	"playlist_removed":        "Removed from playlist",
	"playlist_removed_detail": "Removed %s",
	// playlist_add_remove_summary: two args = addCount, removeCount
	"playlist_add_remove_summary": "Added %d, removed %d tracks",
	"playlist_back_hint":          "",
	"playlist_add_title":          "Add to Playlist",
	"hint_playlist_new":           "New",
	"hint_playlist_rename":        "Rename",
	"hint_playlist_delete":        "Delete",
	"hint_playlist_add_track":     "Add tracks",
	"hint_playlist_remove_track":  "Remove",
},
	LangZH: {
		// ── Player ───────────────────────────────────────────────────────────
		"no_track_selected":   "未选择曲目",
		"press_enter_to_play": "按 Enter 播放",
		"no_lyrics":           "无歌词",
		"lyrics_loading":      "歌词加载中…",
		"lyrics_hint_lrc":     "将 .lrc 文件放在音频文件旁边",
		"lyrics_panel_title":  "歌词",

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
		"chip_converting":     " 8-bit 转换中… ",
		"chip_switching":      " 8-bit 切换中… ",
		"chip_busy":           " 8-bit… ",
		"chip_active":         " 8-bit ",

		// ── Overlays ─────────────────────────────────────────────────────────
		"help_title":         "快捷键",
		"help_close":         "任意键关闭",
		"help_close_key":     "Esc / ? 关闭  ·  ↑↓ 滚动",
		"overlay_hint_close": "Esc 关闭  ·  ↑↓ 滚动",

		"action_move_down":       "向下移动",
		"action_move_up":         "向上移动",
		"action_top_bottom":      "跳转顶部 / 底部",
		"action_jump_playing":    "跳到正在播放",
		"action_page":            "向上 / 向下翻页",
		"action_view":            "切换视图  曲目 / 专辑 / 艺术家",
		"action_play":            "播放（再次 Enter → 全屏）",
		"action_pause_resume":    "暂停 / 继续",
		"action_next_prev":       "下一首 / 上一首",
		"action_seek":            "快退 / 快进 5s",
		"action_cycle_mode":      "切换播放模式",
		"action_chip":            "切换 8-bit 音效（转换 + 淡入淡出）",
		"action_lofi":            "Lo-fi 效果  降低 / 提升采样率",
		"action_settings":        "设置（音乐目录 · p2chip 选项 · Ctrl+R 重载）",
		"action_search":          "搜索（s: 艺术家  a: 专辑  t: 标题  f: 格式）",
		"action_track_info":      "曲目信息",
		"action_fullscreen":      "切换全屏",
		"action_lyrics_mode":     "切换歌词 / 播放器面板",
		"action_collapse_panel":  "折叠 / 展开右侧面板",
		"action_volume":          "音量增加 / 减小",
		"action_switch_tab":      "切换标签页",
		"action_this_help":       "显示此帮助",
		"action_quit":            "退出",
		"action_media_prev_next": "上一首 / 下一首（媒体键映射）",
		"action_media_play":      "播放 / 暂停（媒体键映射）",
		"action_media_vol":       "音量减小 / 增大（媒体键映射）",

		"action_like":            "收藏 / 取消收藏",
		"action_reload_library":  "重新加载曲库",
		"action_open_playlist":   "打开歌单",
		"action_new_playlist":    "新建歌单",
		"action_rename_playlist": "重命名歌单",
		"action_delete_playlist": "删除歌单",
		"action_remove_track":    "移除曲目",

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
		"settings_cancel":       "取消",
		"settings_switch":       "切换字段",
		"settings_edit":         "编辑 / 切换",
		"settings_navigate":     "移动",
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
		"no_tracks_hint":       "没有曲目 — 打开设置（,）并重新加载音乐库",
		"search_placeholder":   "搜索… (s: 艺术家  a: 专辑  t: 标题  f: 格式)",
		"sync_error":           "同步出错：%s",
		"loaded_tracks":        "已加载 %d 首",
		"reloading_library":    "正在重新加载音乐库…",
		"dir_not_found":        "目录不存在：%s",
		"chip_convert_failed":  "8-bit 转换失败：%s",
		"playback_error":       "播放失败：%s",
		"unknown_artist":       "未知艺术家",
		"search_filter_active": "搜索：%s · Esc 清除",
		"search_clear_hint":    "按 Esc 清除筛选",
		"no_search_results":    "没有匹配“%s”的结果",
		"track_not_in_list":    "当前播放曲目不在当前列表中",

		// ── Playlist tab ─────────────────────────────────────────────────────
		"playlist_header":             "歌单",
		"playlist_favorites_name":     "我的最爱",
		"playlist_empty":              "暂无歌单",
		"playlist_renamed":            "已重命名为 %s",
		"playlist_tracks_empty":       "暂无曲目 — 按 a 添加",
		"playlist_tracks_missing":     "%d 首曲目已不在库中 — 已跳过",
		"playlist_track_count":        "%d 首",
		"playlist_new_prompt":         "歌单名称",
		"playlist_rename_prompt":      "重命名为",
		"playlist_delete_confirm":     "确定删除此歌单？",
		"playlist_delete_title":       "删除歌单",
		"playlist_load_error":         "加载歌单失败",
		"playlist_added":              "已加入 %s",
		"playlist_removed":            "已从歌单移除",
		"playlist_removed_detail":     "已移除 %s",
		"playlist_add_remove_summary": "已添加 %d 首，已移除 %d 首",
		"playlist_back_hint":          "",
		"playlist_add_title":          "加入歌单",
		"hint_playlist_new":           "新建",
		"hint_playlist_rename":        "重命名",
		"hint_playlist_delete":        "删除",
		"hint_playlist_add_track":     "添加曲目",
		"hint_playlist_remove_track":  "移除",
	},
}

func init() {
	// Initialise activeLangMap after the translations var is ready.
	// This guarantees T() works correctly even before SetLang is called.
	activeLangMap = translations[LangEN]
}
