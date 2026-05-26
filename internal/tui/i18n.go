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

	// ── Status bar ───────────────────────────────────────────────────────
	"state_stopped":    "  Stopped ",
	"state_playing":    "  Playing ",
	"state_paused":     "  Paused  ",
	"scanning_library": "Scanning library…",
	"hint_pause":       "Pause",
	"hint_resume":      "Resume",
	"hint_back":        "Back",
	"hint_next":        "Next",
	"hint_prev":        "Prev",
	"hint_seek":        "Seek",
	"hint_vol":         "Vol",
	"hint_mode":        "Mode",
	"hint_quit":        "Quit",
	"hint_search":      "Search",
	"hint_help":        "Help",
	"hint_play":        "Play",
	"hint_settings":    "Settings",
	"chip_converting":  " 8-bit Converting… ",
	"chip_switching":   " 8-bit Switching… ",
	"chip_busy":        " 8-bit… ",
	"chip_active":      " 8-bit ",

	// ── Overlays ─────────────────────────────────────────────────────────
	"help_title": "Keyboard shortcuts",
	"help_close": "Any key to close",

	"action_move_down":       "Move down",
	"action_move_up":         "Move up",
	"action_top_bottom":      "Top / Bottom",
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
	"action_volume":          "Volume up / down",
	"action_switch_tab":      "Switch tab",
	"action_this_help":       "This help",
	"action_quit":            "Quit",
	"action_media_prev_next": "Prev / Next  (media key mapping)",
	"action_media_play":      "Play / Pause  (media key mapping)",
	"action_media_vol":       "Vol down / up  (media key mapping)",

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
	"settings_lang_label":   "Language",
	"settings_lang_en":      "English",
	"settings_lang_zh":      "中文",

	// ── Format preference ────────────────────────────────────────────────
	"settings_fmt_label":      "Format filter",
	"fmt_pref_all":            "All formats",
	"fmt_pref_lossless_first": "Lossless first",
	"fmt_pref_lossless_only":  "Lossless only",
	"fmt_pref_mp3_only":       "MP3 only",

	// ── Tabs ─────────────────────────────────────────────────────────────
	"tab_local":          "Local",
	"tab_online":         "Online",
	"tab_playlist":       "Playlists",
	"online_coming_soon": "Coming soon — planned features:",
	"online_back_hint":   "Press Tab to switch back to Local",

	// ── Library panel header ──────────────────────────────────────────────
	"library_title": "Library",
	// library_count: single format arg = total filtered track count
	"library_count": "%d tracks",
	// library_count_playing: two format args = current position, total
	// shown when a track from the filtered list is playing.
	"library_count_playing": "%d/%d tracks",

	// ── App messages ─────────────────────────────────────────────────────
	"track_removed":      "Playing track was removed from library",
	"no_tracks_hint":     "No tracks — open Settings (,) and reload the library",
	"search_placeholder": "Search… (s: artist  a: album  t: title  f: format)",
},
	LangZH: {
		// ── Player ───────────────────────────────────────────────────────────
		"no_track_selected":   "未选择曲目",
		"press_enter_to_play": "按 Enter 播放",
		"no_lyrics":           "无歌词",
		"lyrics_loading":      "歌词加载中…",
		"lyrics_hint_lrc":     "将 .lrc 文件放在音频文件旁边",

		// ── Status bar ───────────────────────────────────────────────────────
		"state_stopped":    "  已停止 ",
		"state_playing":    "  播放中 ",
		"state_paused":     "  已暂停 ",
		"scanning_library": "正在扫描音乐库…",
		"hint_pause":       "暂停",
		"hint_resume":      "继续",
		"hint_back":        "返回",
		"hint_next":        "下一首",
		"hint_prev":        "上一首",
		"hint_seek":        "快进/退",
		"hint_vol":         "音量",
		"hint_mode":        "模式",
		"hint_quit":        "退出",
		"hint_search":      "搜索",
		"hint_help":        "帮助",
		"hint_play":        "播放",
		"hint_settings":    "设置",
		"chip_converting":  " 8-bit 转换中… ",
		"chip_switching":   " 8-bit 切换中… ",
		"chip_busy":        " 8-bit… ",
		"chip_active":      " 8-bit ",

		// ── Overlays ─────────────────────────────────────────────────────────
		"help_title": "快捷键",
		"help_close": "任意键关闭",

		"action_move_down":       "向下移动",
		"action_move_up":         "向上移动",
		"action_top_bottom":      "跳转顶部 / 底部",
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
		"action_volume":          "音量增加 / 减小",
		"action_switch_tab":      "切换标签页",
		"action_this_help":       "显示此帮助",
		"action_quit":            "退出",
		"action_media_prev_next": "上一首 / 下一首（媒体键映射）",
		"action_media_play":      "播放 / 暂停（媒体键映射）",
		"action_media_vol":       "音量减小 / 增大（媒体键映射）",

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
		"settings_lang_label":   "语言",
		"settings_lang_en":      "English",
		"settings_lang_zh":      "中文",

		// ── Format preference ────────────────────────────────────────────────
		"settings_fmt_label":      "格式筛选",
		"fmt_pref_all":            "显示全部",
		"fmt_pref_lossless_first": "无损优先",
		"fmt_pref_lossless_only":  "仅无损",
		"fmt_pref_mp3_only":       "仅 MP3",

		// ── Tabs ─────────────────────────────────────────────────────────────
		"tab_local":          "本地",
		"tab_online":         "在线",
		"tab_playlist":       "歌单",
		"online_coming_soon": "即将推出 — 计划功能：",
		"online_back_hint":   "按 Tab 切换回本地",

		// ── Library panel header ──────────────────────────────────────────────
		"library_title":         "音乐库",
		"library_count":         "%d 首",
		"library_count_playing": "%d/%d 首",

		// ── App messages ─────────────────────────────────────────────────────
		"track_removed":      "当前播放曲目已从音乐库中移除",
		"no_tracks_hint":     "没有曲目 — 打开设置（,）并重新加载音乐库",
		"search_placeholder": "搜索… (s: 艺术家  a: 专辑  t: 标题  f: 格式)",
	},
}

func init() {
	// Initialise activeLangMap after the translations var is ready.
	// This guarantees T() works correctly even before SetLang is called.
	activeLangMap = translations[LangEN]
}
