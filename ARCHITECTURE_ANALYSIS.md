# Music-TUI 项目架构深度分析

## 一、项目概览

**项目**: music-tui  
**语言**: Go 1.24.2  
**类型**: 终端音乐播放器  
**主要依赖**: 
- `charmbracelet/bubbletea` — TUI 框架
- `gopxl/beep/v2` — 音频播放
- `mattn/go-sqlite3` — 数据持久化
- `bogem/id3v2` — ID3 标签解析

---

## 二、目录结构与文件统计

```
music-tui/
├── cmd/
│   ├── music-tui/
│   │   └── main.go                 # 应用入口
│   └── playtest/
│       └── main.go                 # 测试工具
├── internal/
│   ├── audio/
│   │   ├── player.go              # 核心播放器实现（~580行）
│   │   ├── player_test.go
│   │   └── source.go              # StreamSource 接口
│   ├── library/
│   │   ├── track.go               # Track 数据结构
│   │   ├── formats.go
│   │   ├── scanner.go
│   │   └── sort.go
│   ├── lyrics/
│   │   ├── provider.go            # lyrics.Provider 接口
│   │   ├── chain.go               # ChainProvider 实现
│   │   ├── lrc.go                 # LRC 解析器（~574行）
│   │   ├── cache.go
│   │   ├── lrc_test.go
│   │   └── online/
│   │       ├── lrclib.go          # Lrclib 在线提供者
│   │       └── lrclib_test.go
│   ├── provider/
│   │   ├── provider.go            # TrackProvider 接口
│   │   └── local/
│   │       └── local.go           # 本地文件提供者
│   ├── store/
│   │   ├── db.go                  # 数据库管理
│   │   ├── settings_keys.go       # 配置常数
│   │   └── sync.go                # 文件扫描同步
│   └── tui/
│       ├── app.go                 # 根 Bubble Tea 模型（~250行）
│       ├── model.go               # 消息与状态定义
│       ├── actions.go             # 命令实现（~495行）
│       ├── keys.go                # 键盘处理入口
│       ├── render.go              # UI 渲染核心
│       ├── render_*.go            # 各组件渲染
│       └── ...
└── docs/                           # 代码审查文档
```

**TUI 层代码行数**: ~3816 行（所有 *.go 文件总和）

---

## 三、核心架构分析

### 3.1 分层设计

```
┌─────────────────────────────────────────────────────────────────────┐
│                          TUI 层 (tui/)                              │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────────────┐  │
│  │ App 模型 │  消息队列  │ 键盘处理  │  渲染引擎  │   状态管理        │  │
│  └──────────┴──────────┴──────────┴──────────┴──────────────────┘  │
│                              ↓                                       │
│                        Bubble Tea                                   │
└─────────────────────────────────────────────────────────────────────┘
         ↓                              ↓                    ↓
┌─────────────────────┐  ┌──────────────────────┐  ┌──────────────────┐
│  播放器层           │  │  数据层              │  │  歌词层          │
│  (audio/)           │  │  (library/store)     │  │  (lyrics/)       │
│                     │  │                      │  │                  │
│ ┌─────────────────┐ │  │ ┌────────────────┐  │  │ ┌──────────────┐ │
│ │Player(beep)     │ │  │ │Track 结构体    │  │  │ │Provider i/f  │ │
│ │StreamSource i/f │ │  │ │Database (SQL)  │  │  │ │ChainProvider │ │
│ │LocalSource      │ │  │ │TrackProvider i/f│ │  │ │LrcLib online │ │
│ │retroProcessor   │ │  │ │LocaProvider    │  │  │ │Local LRC     │ │
│ └─────────────────┘ │  │ └────────────────┘  │  │ └──────────────┘ │
└─────────────────────┘  └──────────────────────┘  └──────────────────┘
         ↓
    ┌──────────────────────────┐
    │  系统资源（文件系统、网络）│
    └──────────────────────────┘
```

### 3.2 状态容器设计 (app.go)

**App 结构体采用 embedding 分组**，优点是保持字段在 App 层级直接访问（扁平 API），缺点是字段众多时容易混淆：

```go
type App struct {
    player   *audio.Player       // 核心播放器
    st       *store.Store        // 持久化数据库
    musicDir string              // 音乐库根目录
    session  *SessionState       // 恢复状态

    // Embed 分组（4 个子模型）
    PlaybackState  // 播放状态：currentTrack, volume, playMode
    LibraryState   // 库状态：tracks, filtered, cursor
    ChipState      // 8-bit 转换：chipMode, chipPath, tmpDir
    LyricsState    // 歌词状态：lines, activeIdx, provider
    
    // UI 组件
    searchInput textinput.Model
    settingsInput textinput.Model
    musicDirInput textinput.Model
    progressBar progress.Model
    lyricsVP viewport.Model
    
    // UI 状态
    W, H int
    currentView view
    activeTab tabID        // 🔑 支持多 tab
    activeOvl overlay
    
    // 其他
    mqTitle, mqMeta, mqArtist, mqAlbum, mqRow *Marquee  // 滚动文本
    coverRendered string   // 缓存的封面艺术
}
```

**设计评价**:
- ✅ Embedding 使子状态逻辑分离
- ✅ `tabID` 已为 tabLocal/tabOnline 预留
- ⚠️  Marquee 和渲染缓存的生命周期管理复杂
- ⚠️  新增歌单 tab 需要新的 embedding 子模型

---

### 3.3 消息与命令模式 (model.go, actions.go)

**消息定义**：

```go
// model.go 定义了所有 Tea 消息
type tickMsg time.Time              // 定时刷新（500ms）
type trackDoneMsg struct{}           // 播放结束
type scanDoneMsg struct{tracks, err} // 库扫描结束
type playResultMsg struct{track, idx, err} // 播放结果
type lyricsLoadedMsg struct{trackID, lines} // 歌词加载完成
type chip8DoneMsg struct{originPath, chipPath, err} // 8-bit 转换完成
```

**命令模式** (actions.go 第 1~495 行)：

```go
// 播放命令
func (a *App) cmdPlayTrack(idx int) tea.Cmd
func (a *App) cmdPlayNext() tea.Cmd
func (a *App) cmdPlayPrev() tea.Cmd

// 寻道命令
func (a *App) cmdSeek(delta time.Duration) tea.Cmd
func (a *App) cmdSeekAndResume(pos time.Duration) tea.Cmd

// 搜索/过滤
func (a *App) applyFilter() // 实时过滤 a.tracks → a.filtered

// 8-bit 转换
func (a *App) cmdToggleChip() tea.Cmd // 状态机：off → convert → crossfade

// 库同步
func (a *App) cmdSyncLibrary() tea.Cmd // 后台扫描 musicDir
```

**设计特点**：
- ✅ 每个操作都返回 `tea.Cmd`，状态变更在 `Update()` via 消息
- ✅ 8-bit 转换有显式状态机（chipBusy, chipMode, chipConverting）
- ✅ 搜索通过 `applyFilter()` 实时更新 filtered 列表
- ⚠️  `applyFilter()` 是同步调用，不能阻塞 UI

---

### 3.4 播放器层解耦 (audio/)

**核心接口**：

```go
// StreamSource — 音源抽象
type StreamSource interface {
    Open(ctx context.Context) (beep.StreamSeekCloser, beep.Format, error)
}

// LocalSource 实现
type LocalSource struct {
    Path string
}
func (s LocalSource) Open(ctx) (...) {
    f, _ := os.Open(s.Path)
    streamer, format, _ := mp3.Decode(f)  // ❗ 硬编码 MP3
    return streamer, format, nil
}

// Player — 播放器
type Player struct {
    mu       sync.Mutex
    streamer beep.StreamSeekCloser  // beep 流
    retro    *retroProcessor         // 8-bit 效果
    ctrl     *beep.Ctrl              // 暂停控制
    vol      *effects.Volume         // 音量效果
    format   beep.Format
    state    State
    retroIdx int
    volume   float64
    onDone   func()
}
```

**设计评价**：

| 维度 | 现状 | 评价 |
|------|------|------|
| **格式支持** | 仅 MP3 | ❗ LocalSource.Open() 硬编码 `mp3.Decode()` |
| **音源解耦** | StreamSource i/f | ✅ 接口设计良好 |
| **播放器与格式** | 完全分离 | ✅ Player 只看 beep.StreamSeekCloser |
| **扩展成本（新格式）** | 中等 | 需要实现新 StreamSource (FLAC/AAC/WAV) |
| **在线流支持** | 设计可行 | ✅ HTTPSource 可继承 StreamSource |
| **并发安全** | 良好 | ✅ p.mu 保护状态；speaker.Lock() 保护 beep 操作 |

**支持新格式的改动**：

1. **FLAC 支持**：
   ```go
   import "github.com/gopxl/beep/v2/flac"
   
   type FLACSource struct{ Path string }
   func (s FLACSource) Open(ctx) (...) {
       f, _ := os.Open(s.Path)
       streamer, format, _ := flac.Decode(f)  // 只需换这一行
       return streamer, format, nil
   }
   ```
   
2. **HTTP 流**：
   ```go
   type HTTPSource struct{ URL string }
   func (s HTTPSource) Open(ctx) (...) {
       resp, _ := http.NewRequestWithContext(ctx, "GET", s.URL, nil)
       // resp.Body 实现 io.ReadSeekCloser
       streamer, format, _ := mp3.Decode(resp.Body)
       return streamer, format, nil
   }
   ```

3. **TUI 集成**：
   ```go
   // provider/local/local.go 改动
   func (p *Provider) StreamSource(...) StreamSource {
       switch detectFormat(track.Path) {
       case "mp3": return audio.LocalSource{Path: track.Path}
       case "flac": return audio.FLACSource{Path: track.Path}
       case "http": return audio.HTTPSource{URL: track.URL}
       }
   }
   ```

---

### 3.5 数据与库管理 (store/, library/, provider/)

#### 3.5.1 数据模型

```go
// Track — 曲目元数据
type Track struct {
    ID          string      // hash ID
    Title       string      // ID3 TIT2
    Artist      string      // ID3 TPE1
    AlbumArtist string      // ID3 TPE2
    Album       string
    Year        string
    TrackNumber string
    Genre       string
    Comment     string
    Duration    time.Duration
    Path        string      // 仅本地: /path/to/song.mp3
    URL         string      // 仅网易: http://stream/...
    Source      Source      // 枚举: SourceLocal / SourceNetease
    ProviderID  string      // 多源支持: "local", "netease", "qq"
    CoverArt    []byte      // 扫描时的 ID3 APIC 帧
    CoverPath   string      // 缓存后的路径
}

func (t *Track) DisplayTitle() string   // 元数据 → 文件名
func (t *Track) DisplayArtist() string  // 元数据 → "Unknown Artist"
func (t *Track) Format() string         // 文件扩展名 → "MP3"
```

#### 3.5.2 数据库架构 (db.go 第 1~150 行)

**迁移机制**（版本化）：

```sql
-- Migration 1: 初始化
CREATE TABLE tracks (
    id, path, title, artist, ... Duration_ms, source, ...
);
CREATE INDEX idx_tracks_sort ON tracks(album_artist, year, album, track_number);

-- Migration 2: 歌单支持 ✅ 已预留
CREATE TABLE playlists (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at INTEGER
);
CREATE TABLE playlist_tracks (
    playlist_id TEXT,
    track_id TEXT,
    position INTEGER,
    PRIMARY KEY (playlist_id, track_id)
);

-- Migration 3: 多源支持 ✅ 已添加
ALTER TABLE tracks ADD COLUMN provider_id TEXT NOT NULL DEFAULT 'local';
```

**设计评价**：

| 维度 | 现状 | 评价 |
|------|------|------|
| **本地轨道字段** | Path 必填 | ✅ 仅本地才需要 Path |
| **Playlist 预留** | Migration 2 | ✅ 表结构已定义，功能未实现 |
| **多源设计** | provider_id | ✅ Track.ProviderID 允许混合本地/网易/QQ |
| **扩展成本** | 低 | 只需新增 Migration 及对应字段 |

#### 3.5.3 TrackProvider 接口 (provider.go)

```go
type TrackProvider interface {
    ID() string  // "local", "netease", "qq"
    
    // 搜索
    Search(ctx, query string, page, pageSize int) ([]Track, error)
    
    // 获取音源
    StreamSource(ctx, track Track) (audio.StreamSource, error)
    
    // 库同步（可选）
    SyncLibrary(ctx, progress) error
}

// 现有实现：local.Provider
type Provider struct {
    st *store.Store
    musicDir string
    coverDir string
}
```

**接入网易云的改动**：

```go
// 新增文件: internal/provider/netease/netease.go
type Provider struct {
    client   *http.Client
    sessionID string  // 登录状态
}

func (p *Provider) ID() string { return "netease" }

func (p *Provider) Search(ctx, query, page, pageSize) ([]Track, error) {
    // 调用网易 API: GET /weapi/search/get
    // 返回 []Track{Source: SourceNetease, ProviderID: "netease", URL: "..."}
}

func (p *Provider) StreamSource(ctx, track) (audio.StreamSource, error) {
    // 获取流 URL: GET /weapi/song/enhance/player/url
    return audio.HTTPSource{URL: streamURL}, nil
}

func (p *Provider) SyncLibrary(ctx, progress) error {
    // 网易云不支持本地扫描
    return nil
}
```

---

### 3.6 歌词系统 (lyrics/)

#### 3.6.1 接口设计

```go
// Provider — 歌词来源抽象
type Provider interface {
    Fetch(ctx context.Context, track library.Track) ([]Line, error)
}

// Line — 单条歌词
type Line struct {
    Time time.Duration  // 播放位置
    Text string         // 歌词文本
}
```

#### 3.6.2 实现体系

```
ChainProvider
├── LocalLRCProvider
│   ├── <dir>/<name>.lrc        (优先级1)
│   ├── <dir>/Lyrics/<name>.lrc  (优先级2)
│   ├── <dir>/lyrics/<name>.lrc  (优先级3)
│   ├── <dir>/<name>.srt         (优先级4，SRT格式)
│   └── ID3 USLT 帧              (优先级5，ID3标签)
│
└── CachedProvider
    └── LrcLibProvider
        ├── /api/get?track_name&artist_name&album_name&duration
        └── /api/search?q=artist+title (备选)
```

**解析能力**（lrc.go）：

- ✅ 标准 LRC: `[mm:ss.xx]text`
- ✅ 多时间戳: `[mm:ss.xx][mm:ss.xx]text`
- ✅ QQ 逐字: `[mm:ss]字[mm:ss]字` → 合并为单行
- ✅ 增强 LRC: `<mm:ss>word` 标签剥离
- ✅ SRT 字幕: `hh:mm:ss,mmm --> ...`
- ✅ 纯文本: 无时间戳 → `Time=0`

**设计评价**：

| 维度 | 现状 | 评价 |
|------|------|------|
| **Provider 链** | ChainProvider | ✅ 支持故障转移（本地失败→在线） |
| **动态增删** | 静态初始化 | ⚠️  需重启才能切换 Provider 顺序 |
| **在线扩展** | LrcLib 示范 | ✅ 模式可复用到网易云/QQ |
| **缓存策略** | CachedProvider | ✅ 避免重复网络请求 |

#### 3.6.3 接入网易云歌词

```go
// internal/lyrics/online/netease.go
type NeteaseProvider struct {
    client *http.Client
}

func (p *NeteaseProvider) Fetch(ctx, track) ([]Line, error) {
    // 调用网易 API: GET /weapi/song/lyric
    // 响应: { lrc: { lyric: "[mm:ss.xx]..." } }
    
    resp, _ := p.getNeteaseAPI(ctx, track)
    if resp.Lrc != nil && resp.Lrc.Lyric != "" {
        return lyrics.ParseLRCString(resp.Lrc.Lyric), nil
    }
    return nil, nil
}

// 在 app.go 中注册
lyricsProvider = &lyrics.ChainProvider{
    Providers: []lyrics.Provider{
        lyrics.LocalLRCProvider{},
        lyrics.NewCachedProvider(online.NewLrcLibProvider(), cacheDir),
        lyrics.NewCachedProvider(online.NewNeteaseProvider(), cacheDir),  // 新增
    },
}
```

---

### 3.7 UI 层 (tui/)

#### 3.7.1 视图与 Tab 设计

```go
// 视图状态
type view int
const (
    viewNormal     view = iota  // 两面板: 列表 + 迷你播放器
    viewFullscreen             // 全屏: 播放器 + 歌词
)

// Tab 状态（支持多 tab）
type tabID int
const (
    tabLocal  tabID = iota
    tabOnline       // 占位符 — 未实现
)

// 覆盖层（模态框）
type overlay int
const (
    overlayNone overlay = iota
    overlayHelp
    overlaySearch
    overlayInfo
    overlaySettings
)

// 播放模式
type playMode int
const (
    playModeSequential = iota
    playModeLoop
    playModeSingle
    playModeRandom
)
```

**设计评价**：

| 维度 | 现状 | 评价 |
|------|------|------|
| **Tab 扩展** | tabLocal/tabOnline | ✅ 已为在线/播放列表预留 tabID |
| **新增"歌单"Tab** | 需改动 | 改动点: app.go 中状态管理、keys.go 中键盘处理、render.go 中渲染 |
| **国际化** | UI 字符串内联 | ❌ 无 i18n 抽象层 |

#### 3.7.2 键盘处理 (keys.go 第 1~100 行)

```go
func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
    // 全局快捷键（任何状态）
    if msg == "ctrl+c" { return a.cmdQuit() }
    
    // 媒体键（映射到 F6-F12）
    // F7: Play/Pause, F8: Stop, F9: Next, F6: Prev
    // F11: Vol-, F12: Vol+
    
    // 模态分派
    if a.currentView == viewFullscreen { return a.handleFullscreenKey() }
    
    switch a.activeOvl {
    case overlayHelp: return a.handleHelpKey()
    case overlaySearch: return a.handleSearchKey()
    case overlaySettings: return a.handleSettingsKey()
    default: return a.handleNormalKey()
    }
}
```

#### 3.7.3 渲染结构

```go
// render.go — 风格定义与通用组件
// render_player.go — 播放器面板
// render_tab_tracklist.go — 轨道列表
// render_statusbar.go — 状态栏
// render_overlays.go — 模态框
// 等等...

// 所有 UI 字符串硬编码在渲染函数中
// 例如: "Search… (s: artist  a: album  t: title)"
```

**多语言成本**：
- ❌ 无 i18n 框架；所有字符串内联
- 改造成本: 提取所有 `"string"` → 常数或资源文件
- 可行方案: 
  ```go
  // i18n/messages.go
  var Messages = map[string]map[string]string{
      "en": {"search_prompt": "Search… (s: artist  a: album  t: title)"},
      "zh": {"search_prompt": "搜索… (s: 艺术家  a: 专辑  t: 标题)"},
  }
  ```

---

## 四、架构问题回答

### A. 播放器层（功能2、3）

**Q1: StreamSource 接口是否足够扩展？**

✅ **充分**。接口简洁，仅需返回 `beep.StreamSeekCloser` 和 `Format`。

- LocalSource 处理本地文件
- HTTPSource 处理网络流
- 新格式只需新建 StreamSource 实现

**Q2: 当前与 MP3/beep 的耦合程度？**

| 层级 | 耦合点 | 影响 |
|------|--------|------|
| Player | 无 | ✅ 完全松耦合，只依赖 beep 接口 |
| LocalSource | 紧耦合 | ❗ `mp3.Decode()` 硬编码；需创建新 Source |
| TUI | 无 | ✅ 调用 Player i/f，不知道底层格式 |

支持 FLAC/AAC/WAV 的成本：
```
低 - 仅需在 provider.StreamSource() 中感知格式，
      相应创建 FLACSource / AACSource 等
```

**Q3: 在线音乐 vs 本地播放的差异？**

| 维度 | 本地 | 在线 |
|------|------|------|
| **Source** | LocalSource{Path} | HTTPSource{URL} |
| **寻道** | os.File.Seek() | HTTP Range 请求 |
| **字节** | 文件系统 | 网络（缓冲区） |
| **遵循接口** | StreamSource | StreamSource |
| **改动成本** | 低 | 中（需要缓冲/超时处理） |

---

### B. 库/数据源层（功能1、2）

**Q1: Track 是否为 local-only 设计？**

⚠️ **部分**。结构体支持多源，但有 local-specific 字段：

```go
type Track struct {
    Path       string  // ❗ 仅本地
    URL        string  // ❗ 仅在线
    Source     Source  // 枚举：local/netease（旧）
    ProviderID string  // 多源标识（新）
}

// local-specific 字段:
DisplayTitle()      // Path → 文件名降级
DisplayAlbumArtist()
Format()            // Path → 扩展名
```

改造成本（支持混合源）：
- ✅ 已通过 Migration 3 + ProviderID 预留设计
- ⚠️  需填充 URL 字段（网易云来源）
- ⚠️  SearchLibrary 需区分 Source

**Q2: 数据库是否预留 Playlist 表？**

✅ **已预留**（Migration 2）：

```sql
CREATE TABLE playlists (id, name, created_at);
CREATE TABLE playlist_tracks (playlist_id, track_id, position);
```

迁移机制：
- ✅ 版本化 migration 框架
- ✅ 新 migration 追加到切片末尾
- ⚠️  向下兼容需小心（不能 DROP/RENAME）

**Q3: TrackProvider 接口是否可直接接入网易云/QQ？**

✅ **完全可行**。见上文 Provider 设计章节。

```go
// internal/provider/netease/netease.go
type Provider struct { ... }
func (p *Provider) ID() string { return "netease" }
func (p *Provider) Search(...) { ... }
func (p *Provider) StreamSource(...) { ... }
```

新增网易云的改动点：
```
1. provider/netease/ — 实现 TrackProvider
2. cmd/main.go — 初始化 Provider
3. app.go — 注册到 provider 列表
4. TUI 中如果显示 provider 标记 — 可选
```

---

### C. 歌词层（功能4）

**Q1: lyrics.Provider 接口定义？**

```go
type Provider interface {
    Fetch(ctx context.Context, track library.Track) ([]Line, error)
}

type Line struct {
    Time time.Duration
    Text string
}
```

**简洁充分** ✅

**Q2: ChainProvider 是否支持动态增减？**

```go
type ChainProvider struct {
    Providers []Provider
}

func (c *ChainProvider) Fetch(...) {
    for _, p := range c.Providers {  // 遍历
        lines, err := p.Fetch(...)
        if lines != nil { return lines, nil }
    }
    return nil, nil
}
```

⚠️ **静态初始化**：在 app.go 中一次性构建，运行时不支持修改。

动态增减的成本：
```
中 - 需要 runtime 锁保护 c.Providers
     + 暴露 Add/Remove 方法
     + 可能导致竞态（Fetch 与 Add 并发）
```

**Q3: Lrclib 模式能否复用到网易云/QQ？**

✅ **完全可行**。

```go
// internal/lyrics/online/netease.go
type NeteaseProvider struct {
    client *http.Client
}

func (p *NeteaseProvider) Fetch(ctx, track) ([]Line, error) {
    // 1. 查询网易 API
    // 2. 解析响应（LRC 格式）
    // 3. 返回 []Line
}

// 集成
lyricsProvider = &lyrics.ChainProvider{
    Providers: []lyrics.Provider{
        lyrics.LocalLRCProvider{},
        lyrics.NewCachedProvider(online.NewLrcLibProvider(), cacheDir),
        lyrics.NewCachedProvider(online.NewNeteaseProvider(), cacheDir),
    },
}
```

差异点：
- LrcLib: 元数据查询 → HTTP 请求 ✅ 通用模式
- NetEase: 需登录态、API Key（部分端点需付费）
- QQ: 类似
- 成本: 低（模式相同）

---

### D. UI 层（功能1、5）

**Q1: tabID 设计是否为多 tab 扩展做准备？**

✅ **充分**。已预留 tabOnline：

```go
type tabID int
const (
    tabLocal  tabID = iota
    tabOnline
)
```

增加"歌单" tab 的成本：
```
中 - 改动点:
  1. model.go: 新增 tabPlaylist
  2. app.go: 新增状态管理 (playlists, selectedPlaylist)
  3. keys.go: 新增快捷键处理
  4. render_tab_*.go: 新增歌单渲染
  5. actions.go: 新增歌单操作命令
```

**Q2: App 状态管理方式对扩展的影响？**

Embedding 设计：
```go
type App struct {
    PlaybackState
    LibraryState
    ChipState
    LyricsState
    ...
}
```

新增"歌单" tab 需要新 embedding 子模型：
```go
type PlaylistState struct {
    playlists        []Playlist
    selectedPlaylist int
    tracks           []Track  // 当前歌单中的轨道
}

type App struct {
    PlaybackState
    LibraryState
    ChipState
    LyricsState
    PlaylistState   // ← 新增
}
```

**扩展成本评估**：

| 变更 | 成本 | 说明 |
|------|------|------|
| 新增 Tab 定义 | 低 | 加常数 |
| 状态容器 | 中 | 新 Embedding |
| 键盘处理 | 中 | 新分支逻辑 |
| 渲染 | 高 | 新文件 + 布局逻辑 |
| 命令层 | 高 | CRUD + 数据库 |
| **总体** | **高** | ~500-800 行 |

**Q3: 多语言支持现状？**

❌ **无**。所有 UI 字符串硬编码在渲染函数中。

```go
// render.go
ti.Placeholder = "Search… (s: artist  a: album  t: title)"

// render_overlays.go
overlayText := "Press ? for help"
```

i18n 改造成本：
```
高（一次性）～ 低（增量维护）

步骤:
  1. 创建 i18n/messages.go — 翻译库
  2. 遍历 render*.go 提取字符串 (~200 处)
  3. 替换为消息 key
  4. 支持语言切换
  
改动行数: ~500
```

---

### E. 整体架构评价

#### 依赖关系图

```
       ┌─────────────────────────┐
       │       cmd/main          │
       │                         │
       └─────────────────────────┘
              ↓
       ┌─────────────────────────┐
       │    tui/app (TUI 层)     │
       │                         │
       └─────────────────────────┘
        ↙          ↓         ↘        ↖
   ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
   │ audio  │ │library │ │ store  │ │lyrics  │
   │        │ │        │ │        │ │        │
   └────────┘ └────────┘ └────────┘ └────────┘
        ↓                      ↑         ↓
   ┌────────────────────────────────────────────┐
   │   provider  (TrackProvider 统一接口)       │
   │                                            │
   │   ├── local/                               │
   │   ├── netease/ (待实现)                    │
   │   └── qq/ (待实现)                         │
   └────────────────────────────────────────────┘
        ↓                      ↓
   ┌────────────────────────────────────────────┐
   │   系统资源层                               │
   │   ├── 文件系统 (os, io)                    │
   │   ├── 网络 (http, net)                     │
   │   ├── 数据库 (sqlite3)                     │
   │   └── 音频设备 (beep, cpal)                │
   └────────────────────────────────────────────┘
```

#### 耦合分析

**紧耦合问题**：

| 耦合点 | 位置 | 影响 | 优先级 |
|--------|------|------|--------|
| MP3 硬编码 | audio/source.go | 新格式需新 Source | 中 |
| 本地扫描 | library/scanner.go | 上线轨道不支持 | 低 |
| 内联 UI 字符串 | tui/render*.go | 无 i18n | 低 |
| TabID 枚举扩展 | tui/model.go | 需编辑 enum | 低 |

**松耦合优点**：

✅ Provider i/f — 完全松耦合  
✅ StreamSource i/f — 支持任意音源  
✅ lyrics.Provider — 支持链式增删  
✅ TrackProvider — 支持多源混合

#### 架构强度评分

| 维度 | 分数 | 备注 |
|------|------|------|
| **接口设计** | 8/10 | 核心 i/f 简洁有力；少数硬编码 |
| **分层清晰** | 8/10 | 4 层分离；TUI 与数据层完全独立 |
| **扩展性** | 7/10 | Provider 框架良好；UI 扩展需改动多处 |
| **并发安全** | 9/10 | 音频层 lock 规范；TUI 单 goroutine |
| **错误处理** | 7/10 | 及时返回 error；部分地方吞错误 |
| **测试友好** | 6/10 | 依赖注入良好；缺乏 mock 工具 |
| **文档** | 5/10 | 注释覆盖有限；无架构文档 |
| **总体评分** | 7.5/10 | 架构扎实；适合渐进式扩展 |

---

## 五、关键设计决策

### 5.1 正确的设计

✅ **Provider 接口模式**  
允许无限扩展音源，无需修改播放器或 TUI 代码。

✅ **StreamSource 抽象**  
音源完全与播放器分离，支持本地/网络/嵌入资源。

✅ **Lyrics ChainProvider**  
故障转移机制优雅，避免单点故障。

✅ **消息驱动的 TUI 架构**  
所有异步操作通过消息返回，状态变更集中在 Update()。

✅ **数据库版本迁移**  
支持向前兼容扩展；已预留 Playlist 和多源表。

### 5.2 可以改进的地方

⚠️ **MP3 硬编码**  
LocalSource.Open() 应该支持格式自动识别。

```go
// 改造后
func (s LocalSource) Open(ctx) (...) {
    f, _ := os.Open(s.Path)
    defer f.Close()
    
    // 根据扩展名选择解码器
    switch strings.ToLower(filepath.Ext(s.Path)) {
    case ".mp3": return mp3.Decode(f)
    case ".flac": return flac.Decode(f)
    case ".wav": return wav.Decode(f)
    // ...
    }
}
```

⚠️ **无 i18n 框架**  
所有 UI 字符串内联。建议创建 `i18n/messages.go`。

⚠️ **ChainProvider 不支持动态修改**  
运行时无法添加/删除 Provider。需加锁和暴露接口。

⚠️ **Marquee 生命周期不清晰**  
4 个 Marquee 对象的初始化、缓存失效逻辑复杂。

---

## 六、实现优先级建议

### 功能1：在线播放（网易云）

**改动清单**：

| 层 | 文件 | 改动 |
|----|------|------|
| Provider | provider/netease/netease.go | 新增(~300行) |
| Audio | audio/source.go | HTTPSource(~20行) |
| UI | tui/render_tab_tracklist.go | 显示 Source 标记(~10行) |
| 数据库 | store/db.go | Migration 4: netease_session(可选) |
| **总计** | | ~330 行 |

### 功能2：新格式支持

**改动清单**：

| 层 | 文件 | 改动 |
|----|------|------|
| Audio | audio/source.go | FLAC/AAC/WAV Source(~80行) |
| Library | library/track.go | Format() 识别新格式(无需改) |
| Scanner | library/scanner.go | 扫描 .flac/.aac/.wav(~10行) |
| **总计** | | ~90 行 |

### 功能3：HTTP 流播放

**改动清单**：

| 层 | 文件 | 改动 |
|----|------|------|
| Audio | audio/source.go | HTTPSource(~50行) |
| Provider | provider/netease/netease.go | StreamSource 返回 HTTP(无需改) |
| UI | 无 | |
| **总计** | | ~50 行 |

### 功能4：网易云歌词

**改动清单**：

| 层 | 文件 | 改动 |
|----|------|------|
| Lyrics | lyrics/online/netease.go | 新增(~150行) |
| App | tui/app.go | ChainProvider 增加 Netease(~3行) |
| **总计** | | ~153 行 |

### 功能5：播放列表管理

**改动清单**：

| 层 | 文件 | 改动 |
|----|------|------|
| Model | tui/model.go | tabPlaylist 常数(~1行) |
| App | tui/app.go | PlaylistState embedding(~20行) |
| Store | store/db.go | Migration 已预留 |
| Actions | tui/actions.go | 播放列表命令(~150行) |
| Keys | tui/keys.go | 快捷键处理(~30行) |
| Render | tui/render_tab_playlists.go | 新文件(~200行) |
| **总计** | | ~401 行 |

---

## 七、建议与总结

### 7.1 架构评价

**强项**：
- ✅ 接口设计简洁有力（StreamSource, TrackProvider, lyrics.Provider）
- ✅ 分层明确（TUI/业务/数据完全隔离）
- ✅ 并发安全（音频层规范，TUI 单线程）
- ✅ 数据持久化支持版本迁移

**改进空间**：
- ⚠️  MP3 硬编码需要格式自动识别
- ⚠️  缺乏 i18n 框架
- ⚠️  UI 文本全内联
- ⚠️  ChainProvider 无动态管理

### 7.2 演进路线

1. **第一阶段**：HTTP 流播放（支持网易云）
   - 实现 HTTPSource
   - 实现 netease.Provider
   - 预期：~1 周 / 1 人

2. **第二阶段**：格式扩展 + 网易歌词
   - FLAC/AAC/WAV 支持
   - NeteaseProvider for lyrics
   - 预期：~3 天

3. **第三阶段**：播放列表
   - 实现歌单 tab
   - CRUD 命令
   - 预期：~1 周

4. **第四阶段**：i18n 与 UI 打磨
   - 提取所有字符串
   - 建立翻译框架
   - 预期：~5 天

### 7.3 关键代码审查建议

在接纳新功能（网易云、QQ Music）前，请：

1. **验证 Provider 接口**  
   确保 Search() 返回统一的 Track 结构体，Provider 字段填充正确。

2. **检查 StreamSource 错误处理**  
   HTTP 流需要超时、重试、缓冲区管理。

3. **测试 ChainProvider 故障转移**  
   验证本地 LRC 缺失时是否正确回退到在线。

4. **性能基准**  
   搜索、扫描、歌词加载的延迟指标。

---

## 附录：文件索引

| 文件 | 行数 | 职责 |
|------|------|------|
| cmd/music-tui/main.go | 138 | 应用启动、会话恢复 |
| internal/audio/player.go | 580 | 核心播放器（beep wrapper） |
| internal/audio/source.go | 42 | StreamSource 接口 |
| internal/library/track.go | 97 | Track 数据结构 |
| internal/store/db.go | 150+ | SQLite 数据库管理 |
| internal/store/settings_keys.go | 25 | 配置常数 |
| internal/provider/provider.go | 35 | TrackProvider 接口 |
| internal/provider/local/local.go | 70 | 本地文件提供者 |
| internal/lyrics/provider.go | 26 | lyrics.Provider 接口 |
| internal/lyrics/chain.go | 30 | ChainProvider 实现 |
| internal/lyrics/lrc.go | 574 | LRC/SRT/USLT 解析 |
| internal/lyrics/online/lrclib.go | 210 | Lrclib API 集成 |
| internal/tui/app.go | 250 | 根 Bubble Tea 模型 |
| internal/tui/model.go | 127 | 消息与状态定义 |
| internal/tui/actions.go | 495 | 命令实现 |
| internal/tui/keys.go | 100+ | 键盘处理 |
| internal/tui/render*.go | 2000+ | UI 渲染 |

---

*分析完成于 2026-05-26*
