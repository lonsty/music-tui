# music-tui

终端音乐播放器，基于 Go + Bubble Tea，支持本地 MP3 文件、SQLite 曲库持久化和芯片音乐风格转换。

A terminal music player built with Go and Bubble Tea. Plays local MP3 files, persists the library in SQLite, and can convert tracks to chiptune style via [p2chip](https://github.com/lonsty/p2chip).

---

## 功能特性 / Features

| 功能 | 说明 |
|------|------|
| 本地 MP3 播放 | 支持暂停、继续、上一曲/下一曲，切歌时自动播放 |
| 曲库持久化 | SQLite 存储曲目及全量 ID3 标签（专辑艺术家、年份、曲目编号、流派等），启动无需重新扫描 |
| 排序 | 按专辑艺术家 → 年份 → 专辑 → 曲目编号排列 |
| 会话恢复 | 退出后再次打开自动恢复播放位置、音量、播放模式、光标位置等 |
| 芯片音乐模式 | `b` 键调用 p2chip 将当前曲目转换为 8-bit 风格，1.2 秒淡入淡出切换 |
| Lo-Fi 效果 | `r`/`R` 键实时切换 7 个虚拟采样率预设（IIR 低通 + 采样保持） |
| 渐变高亮 | 播放中行和播放器标题使用 Catppuccin `blue→mauve→pink` 渐变色 |
| 跑马灯 | 过长的专辑 · 艺术家 · 标题在列表和播放器中自动滚动 |
| 封面艺术 | 读取 ID3 APIC 帧渲染为终端色块图，懒加载并缓存到磁盘 |
| 全屏播放器 | `f` 键切换（封面 + 进度条 + 歌词占位 + 控制条） |
| 搜索过滤 | `/` 键实时搜索，支持 `s:` 艺术家 `a:` 专辑 `t:` 标题 前缀 |
| 设置面板 | `,` 键配置音乐目录和 p2chip 参数，`Ctrl+R` 触发增量扫描 |
| 多播放模式 | 顺序 / 循环 / 单曲 / 随机 |

---

## 依赖 / Dependencies

### 系统依赖

```bash
# macOS — 必须
xcode-select --install   # 提供 clang（CGO 需要）
brew install ffmpeg      # 8-bit 模式的音频处理

# 8-bit 模式（可选，不使用时无需安装）
pip install p2chip onnxruntime 'scipy<1.12'
brew install fluidsynth
```

> **Linux**：需要 ALSA 开发库 (`libasound2-dev` / `alsa-lib-devel`) 以及 GCC。

### Go 依赖（自动管理）

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI 框架
- [Beep v2](https://github.com/gopxl/beep) — 音频播放
- [id3v2](https://github.com/bogem/id3v2) — MP3 标签读取
- [go-sqlite3](https://github.com/mattn/go-sqlite3) — SQLite（**需要 CGO**）

---

## 安装 / Installation

```bash
git clone https://github.com/eilianxiao/music-tui
cd music-tui

# 本地编译（CGO 必须启用）
CGO_ENABLED=1 go build -o music-tui ./cmd/music-tui

./music-tui ~/Music   # 首次运行指定目录
./music-tui           # 后续直接运行，目录从数据库读取
```

### 交叉编译 / Cross-compilation

使用 `build.sh`（需要 [zig](https://ziglang.org/)）：

```bash
brew install zig        # macOS 上安装 zig
chmod +x build.sh
./build.sh              # macOS 上：构建 macOS arm64/amd64 + Windows
                        # Linux 上：构建 Linux arm64/amd64 + Windows
```

> **Linux 目标**必须在 Linux 主机上构建（oto/v3 用 `pkg-config alsa` 查找 ALSA 头文件，macOS 上不存在）。  
> Linux 上需要先安装：`sudo apt install libasound2-dev`

也可以通过打 tag 触发 GitHub Actions 自动构建所有平台：

```bash
git tag v1.0.0 && git push origin v1.0.0
```

输出文件 (`bin/`)：

| 文件 | 平台 | 构建主机 |
|------|------|---------|
| `music-tui-darwin-arm64` | macOS Apple Silicon | macOS |
| `music-tui-darwin-amd64` | macOS Intel | macOS |
| `music-tui-linux-amd64` | Linux x86_64 | Linux |
| `music-tui-linux-arm64` | Linux ARM64 | Linux |
| `music-tui-windows-amd64.exe` | Windows x86_64 | macOS 或 Linux |

> **Linux 运行依赖**：`libasound2`（ALSA，桌面 Linux 均预装）和 glibc。  
> **Windows / macOS**：无额外运行时依赖。

---

## 快速上手 / Quick Start

首次启动时曲库为空：

1. 按 `,` 打开设置面板
2. 在 **Directory** 栏填入音乐目录（默认已预填）
3. 按 `Ctrl+R` 触发扫描（首次扫描可能需要数秒到数分钟，取决于曲库大小）
4. 扫描完成后列表自动刷新

---

## 快捷键 / Keyboard Shortcuts

### 普通模式

| 键 | 功能 |
|----|------|
| `j` / `↓` | 向下移动光标 |
| `k` / `↑` | 向上移动光标 |
| `g` / `G` | 跳到顶部 / 底部 |
| `Enter` | 播放选中曲目（对当前播放曲目再按一次 → 全屏） |
| `Space` | 暂停 / 继续 |
| `n` / `p` | 下一曲 / 上一曲（切歌总是开始播放） |
| `m` | 循环切换播放模式（顺序 → 循环 → 单曲 → 随机） |
| `+` / `-` | 音量 +0.1 / -0.1 |
| `f` | 切换全屏播放器 |
| `b` | 开启 / 关闭 8-bit 芯片模式 |
| `r` / `R` | Lo-Fi 效果降档 / 升档 |
| `/` | 搜索（`s:` 艺术家  `a:` 专辑  `t:` 标题） |
| `i` | 曲目详情（含完整 ID3 标签） |
| `,` | 打开设置面板 |
| `?` | 帮助面板（完整快捷键列表） |
| `Tab` | 切换标签页（Local / Online） |
| `q` / `Ctrl+C` | 退出并保存会话状态 |

### 多媒体键映射 / Media Key Mapping

终端程序无法直接接收系统媒体键（⏯ ⏭ ⏮）。music-tui 的做法是：将以下 F 键定义为"媒体键接收端"，由用户通过系统工具把媒体键重映射为对应的 F 键，按下媒体键时系统实际发出的是 F 键事件，终端转发给 music-tui。

| F 键 | 功能 |
|------|------|
| `F6` | 上一曲 |
| `F7` | 播放 / 暂停 |
| `F8` | 停止 |
| `F9` | 下一曲 |
| `F11` | 音量减 |
| `F12` | 音量加 |

#### macOS — Karabiner-Elements（推荐）

[Karabiner-Elements](https://karabiner-elements.pqrs.org/) 是 macOS 上最可靠的按键重映射工具，无需命令行配置。

**方式一：图形界面（Simple Modifications）**

1. 打开 Karabiner-Elements → **Simple Modifications** 标签
2. 选择目标键盘，点击 **Add item**，添加以下映射：

| From（按下的键） | To（发送的键） |
|----------------|--------------|
| `rewind` | `f6` |
| `play_or_pause` | `f7` |
| `fastforward` | `f9` |
| `volume_down` | `f11` |
| `volume_up` | `f12` |

**方式二：JSON 配置文件**

将以下内容保存为 `~/.config/karabiner/assets/complex_modifications/music-tui.json`，然后在 Karabiner-Elements → **Complex Modifications** → **Add rule** 中启用：

```json
{
  "title": "music-tui media keys",
  "rules": [
    {
      "description": "Map media keys to F6-F12 for music-tui",
      "manipulators": [
        {
          "type": "basic",
          "from": { "consumer_key_code": "rewind" },
          "to": [{ "key_code": "f6" }]
        },
        {
          "type": "basic",
          "from": { "consumer_key_code": "play_or_pause" },
          "to": [{ "key_code": "f7" }]
        },
        {
          "type": "basic",
          "from": { "consumer_key_code": "fastforward" },
          "to": [{ "key_code": "f9" }]
        },
        {
          "type": "basic",
          "from": { "consumer_key_code": "volume_decrement" },
          "to": [{ "key_code": "f11" }]
        },
        {
          "type": "basic",
          "from": { "consumer_key_code": "volume_increment" },
          "to": [{ "key_code": "f12" }]
        }
      ]
    }
  ]
}
```

> **注意**：此配置会将媒体键全局重映射为 F 键，macOS 系统自带的媒体控制（控制 Music.app 等）将不再响应。如需仅在终端聚焦时生效，可在 Complex Modifications 中为每条规则添加 `"conditions": [{"type": "frontmost_application_if", "bundle_identifiers": ["com.apple.Terminal", "com.googlecode.iterm2"]}]`。

#### macOS — skhd

[skhd](https://github.com/koekeishiya/skhd) 原生支持媒体键绑定，无需 Karabiner，更轻量。

```sh
brew install koekeishiya/formulae/skhd
skhd --install-service && skhd --start-service
```

**授权辅助功能**（必须，否则 skhd 无法监听按键）：

系统设置 → 隐私与安全 → 辅助功能 → 点击 `+` 添加 `/opt/homebrew/bin/skhd` → 开启开关

在 `~/.skhdrc`（或 `~/.config/skhd/skhdrc`）中添加：

```
play       : osascript -e 'tell application "System Events" to key code 96'   # F7 播放/暂停
previous   : osascript -e 'tell application "System Events" to key code 97'   # F6 上一曲
next       : osascript -e 'tell application "System Events" to key code 101'  # F9 下一曲
rewind     : osascript -e 'tell application "System Events" to key code 100'  # F8 停止
sound_down : osascript -e 'tell application "System Events" to key code 103'  # F11 音量减
sound_up   : osascript -e 'tell application "System Events" to key code 111'  # F12 音量加
```

重载配置：

```sh
skhd -r
```

> **注意**：skhd 拦截媒体键是全局生效的，系统其他应用（如 Music.app）将不再收到这些键。

#### Linux — xbindkeys + xdotool

```sh
# 安装工具
sudo apt install xbindkeys xdotool   # Debian/Ubuntu
sudo dnf install xbindkeys xdotool   # Fedora

# 生成默认配置
xbindkeys --defaults > ~/.xbindkeysrc
```

在 `~/.xbindkeysrc` 中追加：

```sh
# music-tui 媒体键映射
"xdotool key F7"
  XF86AudioPlay

"xdotool key F6"
  XF86AudioPrev

"xdotool key F9"
  XF86AudioNext

"xdotool key F8"
  XF86AudioStop

"xdotool key F11"
  XF86AudioLowerVolume

"xdotool key F12"
  XF86AudioRaiseVolume
```

```sh
# 重新加载配置（无需重启）
pkill xbindkeys; xbindkeys
```

> 提示：直接在终端里用 `Space`/`n`/`p`/`+`/`-` 快捷键效果完全相同，更简便。

### 全屏模式

| 键 | 功能 |
|----|------|
| `Esc` / `f` | 返回列表 |
| `Space` | 暂停 / 继续 |
| `n` / `p` | 下一曲 / 上一曲 |
| `+` / `-` | 音量 |
| `m` | 切换播放模式 |
| `b` | 8-bit 芯片模式 |
| `r` / `R` | Lo-Fi 效果 |
| `,` | 设置面板 |
| `q` | 退出 |

### 设置面板（`,` 键）

| 键 | 功能 |
|----|------|
| `Tab` | 在 Directory / Options 输入框之间切换 |
| `Ctrl+R` | 保存目录并立即触发增量扫描 |
| `Enter` | 保存所有设置并关闭 |
| `Esc` | 放弃修改并关闭 |

---

## 数据存储 / Data Storage

| 路径 | 内容 |
|------|------|
| `~/.local/share/music-tui/music-tui.db` | SQLite 数据库：曲库、配置、会话状态 |
| `~/.cache/music-tui/covers/` | ID3 封面图缓存（按文件路径 SHA256 命名） |
| `$TMPDIR/music-tui-*/` | 8-bit 转换临时文件（退出时自动清理） |

XDG 环境变量 `XDG_DATA_HOME` / `XDG_CACHE_HOME` 受支持。

---

## 8-bit 芯片模式 / 8-bit Mode

按 `b` 键时：

1. 状态栏显示 **8-bit Converting…**
2. 后台调用 `p2chip` 将当前曲目转换为芯片音乐风格 MP3（约 10–60 秒）
3. 转换完成后 1.2 秒淡出原曲、淡入 8-bit 版本，自动跳转到相同时间点
4. 状态栏显示 **8-bit**

再次按 `b` 淡出切回原曲。已转换文件缓存在临时目录，同一曲目再次按 `b` 无需重新转换。

切歌时若处于 8-bit 模式，新曲目自动触发转换。

**p2chip 参数**（设置面板 Options 栏）示例：

```
--sf2 nes --onset 0.6 --trim 0:60
--sf2 gameboy --min-note 200
```

---

## Lo-Fi 模式 / Lo-Fi Mode

`r` 键降低虚拟采样率（效果更重），`R` 键升高（效果减弱）。共 7 档：

| 档位 | 虚拟采样率 | 状态栏 |
|------|-----------|--------|
| 0 | off | — |
| 1 | 11.0 kHz | `11.0k` |
| 2 | 5.5 kHz | `5.5k` |
| 3 | 2.8 kHz | `2.8k` |
| 4 | 1.4 kHz | `1.4k` |
| 5 | 689 Hz | `689` |
| 6 | 344 Hz | `344` |

IIR 低通滤波器防止混叠，保持干净的降采样质感（无噪声）。

---

## 开发 / Development

```bash
CGO_ENABLED=1 go build ./...   # 编译（CGO 必需）
go test ./...                  # 运行测试
go vet ./...                   # 静态检查
```

也可以用 `build.sh` 同时编译所有平台（见上方安装章节）。

---

## 许可证 / License

MIT
