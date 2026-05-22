# music-tui

终端音乐播放器，基于 Go + Bubble Tea，支持本地 MP3 文件、SQLite 曲库持久化和芯片音乐风格转换。

A terminal music player built with Go and Bubble Tea.  Plays local MP3 files, persists the library in SQLite, and can convert tracks to chiptune style via [p2chip](https://github.com/lonsty/p2chip).

---

## 功能特性 / Features

| 功能 | 说明 |
|------|------|
| 本地 MP3 播放 | 支持暂停、继续、上一曲/下一曲 |
| 曲库持久化 | SQLite 存储曲目及全量 ID3 标签，启动无需重新扫描 |
| 会话恢复 | 退出后再次打开自动恢复播放位置、音量、播放模式等 |
| 芯片音乐模式 | `b` 键调用 p2chip 将当前曲目转换为 8-bit 风格并淡入淡出切换 |
| Lo-Fi 效果 | `r`/`R` 键实时应用 IIR 低通 + 采样保持降档效果 |
| 渐变文字 | 播放中行和播放器标题使用 Catppuccin 渐变色高亮 |
| 跑马灯滚动 | 过长的标题、艺术家、专辑信息自动跑马灯 |
| 封面艺术 | 读取 ID3 APIC 帧并渲染为 Unicode 色块图，缓存到本地文件 |
| 全屏播放器 | `f` 键切换全屏视图（封面 + 歌词占位 + 进度条） |
| 搜索过滤 | `/` 键快速搜索，支持 `s:` `a:` `t:` 前缀指定字段 |
| 设置面板 | `,` 键打开设置，可配置音乐目录和 p2chip 参数 |
| 多播放模式 | 顺序 / 循环 / 单曲 / 随机 |

---

## 依赖 / Dependencies

### 系统依赖

```bash
# macOS
brew install ffmpeg   # 音频裁剪（--trim）
# p2chip 是可选的 Python CLI，只在使用 8-bit 模式时需要
pip install p2chip onnxruntime 'scipy<1.12'
brew install fluidsynth   # p2chip 渲染依赖
```

### Go 依赖（自动管理）

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI 框架
- [Beep](https://github.com/gopxl/beep) — 音频播放
- [id3v2](https://github.com/bogem/id3v2) — MP3 标签读取
- [go-sqlite3](https://github.com/mattn/go-sqlite3) — SQLite（需要 CGO）

---

## 安装 / Installation

```bash
git clone https://github.com/eilianxiao/music-tui
cd music-tui
go build -o music-tui ./cmd/music-tui
./music-tui ~/Music
```

> CGO 必须启用（go-sqlite3 要求）。macOS 上 Xcode Command Line Tools 已提供所需工具链。

---

## 使用 / Usage

```bash
music-tui [music_dir]   # 首次运行指定目录；后续从数据库加载，可在设置中修改
```

启动后若数据库为空，按 `,` 进入设置，确认目录后按 `Ctrl+R` 扫描加载。

---

## 快捷键 / Keyboard Shortcuts

### 全局

| 键 | 功能 |
|----|------|
| `?` | 帮助面板 |
| `q` / `Ctrl+C` | 退出并保存状态 |

### 列表导航

| 键 | 功能 |
|----|------|
| `j` / `↓` | 向下 |
| `k` / `↑` | 向上 |
| `g` / `G` | 顶部 / 底部 |
| `Enter` | 播放（再按一次进入全屏） |
| `/` | 搜索（`s:` 艺术家  `a:` 专辑  `t:` 标题） |
| `i` | 曲目详情（含全量 ID3 标签） |
| `,` | 设置面板 |

### 播放控制

| 键 | 功能 |
|----|------|
| `Space` | 暂停 / 继续 |
| `n` / `p` | 下一曲 / 上一曲 |
| `m` | 切换播放模式 |
| `+` / `-` | 音量 +/- |
| `f` | 切换全屏播放器 |

### 音效

| 键 | 功能 |
|----|------|
| `b` | 开启 / 关闭 8-bit 芯片模式（调用 p2chip 转换，淡入淡出切换） |
| `r` / `R` | Lo-Fi 效果降档 / 升档（7 个虚拟采样率预设） |

---

## 设置面板 / Settings

按 `,` 打开，`Tab` 在两个输入框之间切换：

- **Directory** — 音乐目录路径，`Ctrl+R` 触发增量扫描
- **Options** — p2chip 额外参数，如 `--sf2 nes --onset 0.6`

`Enter` 保存，`Esc` 放弃。

---

## 数据存储 / Data Storage

| 路径 | 内容 |
|------|------|
| `~/.local/share/music-tui/music-tui.db` | SQLite 数据库（曲库 + 配置 + 会话状态） |
| `~/.cache/music-tui/covers/` | 封面艺术缓存文件 |
| `/tmp/music-tui-*/` | p2chip 转换的临时文件（退出自动清理） |

---

## 8-bit 模式 / 8-bit Mode

按 `b` 键时，music-tui 在后台调用 `p2chip` 将当前曲目转换为芯片音乐风格 MP3，转换完成后通过 1.2 秒淡出/淡入效果切换播放，并跳转到原曲的相同时间点。

再次按 `b` 淡出切回原曲。已转换的文件缓存在临时目录，同一首曲子无需重复转换。

切歌时若处于 8-bit 模式，新曲目也会自动触发转换。

p2chip 参数可在设置面板中配置。

---

## 开发 / Development

```bash
go build ./...          # 编译
go test ./...           # 运行测试（暂无覆盖 TUI 层的集成测试）
go vet ./...            # 静态检查
```

---

## 许可证 / License

MIT
