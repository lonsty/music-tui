# Design

## Context

见 `proposal.md - Why`。当前约束：

- Go 1.24.2，CGO 必启；本地 Go 1.22 需要 `GOTOOLCHAIN=auto`。
- Bubble Tea 单线程调用 `Update` / `View`；`tea.Cmd` 在独立 goroutine 执行。
- SQLite 单连接（`SetMaxOpenConns(1)`），WAL 开启。
- 现有基准只覆盖微型工具函数（`filteredPos`、`colSlice`、`wrapText`、歌词样式），没有任何 `View`、`Update`、`applyFilter` 或存储写入基准。
- 现有测试基线：`go build` / `go vet` / `go test` / `gofmt` 全绿；`go test -race` 仅在 `internal/audio` 因 oto 初始化竞态失败，属已知第三方问题。
- 覆盖率：`lyrics` 80%，`audio` 35%，`store` 28%，`tui` 23%，`library` / `provider` 0%。

## Goals / Non-Goals

**Goals:**

- 在不改变正常路径输出与交互的前提下，消除大曲库热路径上的可测量浪费。
- 让代码结构与文档重新符合 `ARCHITECTURE.md` 声明的不变量。
- 建立可重复的性能基线，使后续改动能判断回归。
- 补齐开源仓库的最低协作基础设施。

**Non-Goals:**

- 不新增功能、Tab、设置项或快捷键。
- 不改变渲染输出的逐字节结果（唯一例外是 `applyFormatPreference` 污染曲库的缺陷修复）。
- 不重写 TUI 架构，不做跨包边界重组。
- 不修复 oto 初始化竞态。
- 不做英文文档翻译。

## Decisions

### D1. 行为保持用 golden 渲染测试锁定

在重构前，为代表性状态生成 golden 输出：普通双栏、全屏播放器、全屏歌词、帮助/设置/队列/歌单浮层、搜索态，固定 `W/H`、语言、图标集与曲目数据。重构后逐字节比对。

替代方案：只依赖现有行为测试。现有测试只覆盖部分按键与文案，无法发现布局或 ANSI 序列漂移，故不采用。

golden 只覆盖确定性内容；时间、进度条动画等随 tick 变化的字段在测试数据中固定。

### D2. 过滤与搜索改为“位置直出”，分组缓存惰性重建

- `selectTrackIndices` 不再复制 `[]library.Track`：直接遍历 `tracks` 下标并追加满足查询的位置。
- 格式偏好增加按下标过滤的内部实现，与现有 `applyFormatPreference` 共用同一判定；分组路径仍走原函数。
- 空查询 + `formatPrefAll` + 无 drilldown 时返回缓存的恒等索引切片。该切片只在 `tracks` 替换时重建，且约定只读（现状已满足：`filteredIdxs` 只有整体赋值，无原地改写）。
- 抽 `groupsDirty` 标志：`tracks` 或 `formatPref` 变化时置脏；`activeGroups()` 首次读取时重建。搜索期间不触发重建。
- 搜索输入转发路径比较过滤前后查询值，光标闪烁等非按键消息不再重复过滤。

替代方案：建立倒排索引或增量搜索。实现与内存成本高，10k 规模下线性扫描已足够，不做。

风险控制：新增回归测试覆盖空查询、前缀查询、格式偏好、drilldown 四种组合的结果一致性。

### D3. 队列查找走 ID → 库内位置缓存

新增 `trackPosByID map[string]int`，与 `tracks` 同步重建。`findTrackByID` 用位置取回副本，保持现有“返回副本”语义。`posByID` 继续只服务过滤列表，两者职责分开。

替代方案：`map[string]*library.Track`。指针会引入别名与并发读写风险，且与现有副本语义不一致，不采用。

### D4. 歌词宽度在加载时计算一次

`LyricsState` 增加 `maxTextW`，在 `lyricsLoadedMsg` 处理时计算；终端宽度变化时置零并在下一次渲染重算。`renderLyricsScroll` 不再每帧扫描全部行。

替代方案：按 `(len(lines), width)` 做 memo。状态字段更直观，且已有明确失效点。

### D5. 渲染分配收敛，输出不变

- `Marquee` 在 `SetText` 时缓存 `textW`；`Tick` 用缓存判断是否需要滚动。
- `gradientText` 对 `r < 0x80` 直接按 1 列计宽，非 ASCII 保持现有 `strWidth(string(r))` 路径，保证输出逐字节一致。
- 封面占位图写入现有 `coverRendered` 缓存，用 `coverW` 与“占位”标记共同失效。
- 每帧构造的 `lipgloss.NewStyle()` 提升为包级变量。
- 状态栏提示宽度先求和一次，再从头丢弃候选，避免 `O(n²)` 重算。
- 音频位置/时长在 `render()` 入口取一次快照，存于帧内字段，供各渲染函数读取；不改变公开 API。`View` 由 Bubble Tea 串行调用，帧内字段无并发风险。

替代方案：把位置作为参数贯穿所有渲染函数。改动面大且易漏，帧内快照更小。

### D6. 存储：版本化迁移、批量写入与索引

- 保留 migration 1 原文不动；新增 migration 2：`playlist_tracks(playlist_id, position)` 索引与 `tracks` 排序表达式索引。迁移表逐版本追加，既有数据库自动升级。
- 表达式索引必须与 `AllTracks` 的 `ORDER BY` 完全一致。已用 `EXPLAIN QUERY PLAN` 验证 SQLite 能命中该索引并消除临时 B-tree。
- `SyncDir` 在单个事务内使用 prepared upsert，每 1000 行提交一次；单条解析失败仍记录 `firstErr` 并继续，保持现有“部分成功”语义。
- 同步修正 `db.go` 顶部与 `ARCHITECTURE.md` 冲突的迁移注释：统一为“已发布迁移不可修改，只能追加”。

替代方案：用外部 `sqlite3` 批量导入。引入进程与格式转换，不做。

### D7. 消除 `tea.Cmd` 写状态与全局变量竞态

- `cmdNextPlayMode` / `cmdRetroUp` / `cmdRetroDown` 计算目标值并返回专用消息；`Update` 落状态并执行依赖状态的逻辑（shuffle 重建、播放器档位设置）。播放器本身线程安全，可在 Cmd 内调用，但 `App` 字段只在 `Update` 写。
- i18n 改为 `atomic.Pointer[langSnapshot]`，快照含语言与翻译表；`SetLang` 原子替换，`T()` 一次原子读 + 一次 map 查找。
- 图标集改为 `atomic.Int32`。

替代方案：全局 `sync.RWMutex`。渲染每帧取锁会增加热路径开销，原子快照更合适。

### D8. 持久化失败可见

新增统一上报助手：写日志；在 `Update` 调用路径上同时设置状态栏错误提示（新增中英 i18n 键）。`Cleanup` 等无 UI 路径只记录日志。不改变成功路径。

### D9. 扫描可取消、防重入

`App` 持有 `scanCancel`。触发同步时：若 `loading` 为真直接返回；否则创建 `context.WithCancel` 并保存。`Cleanup` 与退出路径取消。`SyncDir` 的 feeder 协程在发送 `workCh` 时同时监听 `ctx.Done()`，避免 worker 退出后永久阻塞。

替代方案：仅加 `loading` 守卫。能防叠加但无法在退出时停止解析，故同时引入取消。

### D10. 删除死代码，保留数据字段

删除 `internal/provider`、`internal/provider/local`、`App.providerMap`、`resolveSource` 的 provider 分支，以及四个未使用导出符号。`library.Track.ProviderID` 与数据库列保留，避免无谓的数据迁移。

这些删除是 **BREAKING**（对潜在外部导入者），但仓库是应用而非公共库；在 CHANGELOG 中标注。

### D11. 文件拆分只做机械移动

目标布局（同包，不新增包边界）：

| 原文件 | 拆分后 |
|--------|--------|
| `app.go` | `state.go`（状态结构）、`new_app.go`、`update.go` + `update_library.go` / `update_playback.go` / `update_playlist.go` / `update_lyrics.go`、`layout.go`、`filter_index.go`、`marquee_cover.go`、`lyrics_provider.go` |
| `keys.go` | `keys_fullscreen.go`、`keys_search.go`、`keys_settings.go`、`keys_playlist.go`、`keys_overlays.go`，保留分发入口 |
| `actions.go` | `actions_playback.go`、`actions_filter.go`、`actions_settings.go`、`actions_sync.go`、`actions_chip.go`、`actions_playlist.go` |
| `render_player.go` | `render_player_mini.go`、`render_player_full.go`、`render_lyrics.go`、`render_lyrics_mini.go` |

每个文件一次移动提交，不改函数体；每步跑构建与测试。`OverlayState` 从 `PlaylistState` 抽出并嵌入 `App`，字段名不变以保持访问路径。

替代方案：一次性大迁移。review 与回滚成本过高，不采用。

### D12. CI 与仓库文件

- `.github/workflows/ci.yml`：`push` 与 `pull_request`；Ubuntu 安装 `libasound2-dev`；macOS 直接构建。步骤：`gofmt -l` 零输出检查、`go build ./...`、`go vet ./...`、`go test ./...`；独立 `-race` 任务排除 `internal/audio`。
- `LICENSE`：MIT，版权行使用 “Copyright (c) 2025-2026 music-tui contributors”，避免绑定个人。
- `CONTRIBUTING.md`：环境要求、三件套门禁、提交与分支约定。
- `CHANGELOG.md`：Keep a Changelog 格式，首条记录本次 **BREAKING** 删除与唯一可见缺陷修复。
- `Makefile`：`build` / `test` / `vet` / `fmt` / `bench`，统一注入 `CGO_ENABLED=1` 与 `GOTOOLCHAIN=auto`。
- `.gitignore` 追加 `.pytest_cache/`。

### D13. 基准策略

新增基准：`BenchmarkApplyFilter10K`（空查询/前缀查询/lossless-first）、`BenchmarkSelectTrackIndices10K`、`BenchmarkViewNormal10K`、`BenchmarkViewFullscreen10K`、`BenchmarkRenderQueueOverlay200`、`BenchmarkUpdateKey10K`、`BenchmarkSyncUpsert10K`（临时库）。改动前后各跑一次并用 `benchstat` 记录到 `design.md` 的实现记录或 PR 描述。

CI 不设绝对耗时阈值（机器差异大），只保留可手动复跑的 `make bench`。分配次数（`-benchmem`）作为 review 依据。

基线（改动前，Apple M2 Pro，`-benchtime 200x`）：

| 基准 | 结果 |
|------|------|
| `BenchmarkFilteredPos10K` | 10.0 ns/op |
| `BenchmarkFilteredPosLinear10K` | 8139 ns/op |
| `BenchmarkTrackAt10K` | 13.75 ns/op |
| `BenchmarkColSlice_CJK` | 1040 ns/op |
| `BenchmarkWrapText_CJK` | 2426 ns/op |
| `BenchmarkLyricStyleForDistance_Frame` | 34653 ns/op |

上述基准均未覆盖本次主要改动路径，因此新增基准是前置任务，而不是事后验证。

## Risks / Trade-offs

- **文件拆分引入隐性行为漂移** → 只做机械移动；每次移动后跑 golden 测试与全量测试；一次提交只动一个文件。
- **恒等索引切片被下游原地修改** → 约定只读；回归测试覆盖过滤后再次过滤、格式切换、drilldown 进出；必要时改为返回副本。
- **分组惰性重建漏掉失效点** → 所有写 `tracks` / `formatPref` 的位置集中调用 `markGroupsDirty()`；测试覆盖“搜索→清除→切换 Albums”与“格式偏好切换后进入 Albums”。
- **批量事务改变部分失败语义** → 每批提交，失败仍继续；测试覆盖“部分文件解析失败仍写入其余文件”。
- **表达式索引与 `ORDER BY` 漂移** → 索引定义与查询共用一段 SQL 常量/注释；`EXPLAIN QUERY PLAN` 测试断言不得出现 `USE TEMP B-TREE`。
- **原子快照替换后 `T()` 语义变化** → 快照同时保存语言与表，英文回退逻辑保持不变；i18n 覆盖测试继续通过。
- **删除 provider 包影响未来在线音源** → 当前为零调用死代码；未来引入在线音源时按 `TrackProvider` 契约重新设计，CHANGELOG 说明。
- **CI 中音频测试无设备** → 测试改为探测文件与设备，不可用即 `t.Skip`；ISO 上仍保留 `-race`（排除 audio）。
- **golden 测试对终端样式敏感** → 固定语言、图标集、尺寸与 `lipgloss` 渲染环境；不比对含时间/动画的字段。

## Migration Plan

按阶段提交，每阶段独立可回滚：

1. **基线与门禁**：新增渲染 golden 测试、缺失单测、基准；补 LICENSE/CI/Makefile/CONTRIBUTING/CHANGELOG；修 README 与迁移注释。此阶段不改运行逻辑。
2. **正确性**：修 `applyFormatPreference` 污染；`tea.Cmd` 状态回传；i18n/icon 原子化；持久化错误上报；扫描取消与防重入；音频测试可移植。
3. **存储**：版本化迁移与索引；批量 upsert。用临时数据库测试升级路径（v1→v2）与幂等性。
4. **性能**：过滤/搜索、队列缓存、歌词宽度、渲染分配、音频快照。每项先跑基准，再改，再跑基准。
5. **结构与死代码**：删除未使用导出；拆分文件与 `OverlayState`。只移动代码。
6. **收尾**：`go test -race`（排除 audio）、`openspec validate --strict`、真实二进制 pty 冒烟、更新 ARCHITECTURE/README/CHANGELOG。

回滚：每阶段一个提交；任一门禁失败即回退该阶段提交，不影响已完成阶段。数据库迁移只追加索引，回滚代码后旧版本仍可打开数据库（多出的索引无害）。

## Open Questions

- `LICENSE` 版权行暂定 “music-tui contributors”；若需署名个人或组织，在实现前替换即可，不影响其他设计。

## 实现记录（实现后补充）

环境：Apple M2 Pro，`-benchtime 100x`，10k 曲库。基线为本变更第一个提交前的数值；实现后为性能阶段提交后的数值。

| 基准 | 改动前 | 改动后 | 分配变化 |
|------|--------|--------|----------|
| `ApplyFilter10K/Empty` | 3466 µs | 403 µs | 46158 → 35 allocs |
| `ApplyFilter10K/Prefix` | 3145 µs | 394 µs | 56130 → 10008 allocs |
| `ApplyFilter10K/LosslessFirst` | 8508 µs | 3642 µs | 126293 → 80071 allocs |
| `SelectTrackIndices10K/Empty` | 899 µs | 132 µs | 35 → 1 alloc |
| `ViewNormal10K` | 639 µs | 737 µs | 2160 → 1792 allocs |
| `ViewFullscreen10K` | 618 µs | 591 µs | 2136 → 1783 allocs |
| `RenderQueueOverlay200` | 2684 µs | 903 µs | 5336 → 5336 allocs |
| `UpdateKey10K` | 689 ns | 810 ns | 408 B → 408 B |

说明：

- `ApplyFilter` 前缀路径仍有每曲目 `strings.ToLower` 分配，属于搜索语义本身；进一步优化需要预计算小写索引，超出本次范围。
- `ViewNormal10K` 耗时在 100x 采样下波动较大；分配下降是本项的主要可复现收益。
- `RenderQueueOverlay200` 的分配未变，但查找从 `O(queue × library)` 降为 `O(queue)`。
- 无 `benchstat` 可用，故以同一机器两次基准输出直接对比。

