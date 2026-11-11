# Proposal

## Why

功能已经成型，但代码库在三个方面阻碍长期维护：

1. **大曲库热路径存在可测量的浪费**：搜索每输入一个字符都会复制整份曲库并重建分组；队列浮层每帧对全库做线性查找；歌词面板每帧重新测量全部歌词行；首次扫描对每个文件各开一次事务。
2. **部分架构不变量只写在文档里，代码并未遵守**：`tea.Cmd` 闭包直接写 `App` 字段，i18n 与图标全局表被 `View` 与 `Update` 并发读写，持久化失败被静默丢弃；迁移策略在代码注释与 `ARCHITECTURE.md` 中互相矛盾。
3. **缺少开源协作基础设施**：README 声明 MIT 但无 `LICENSE`；CI 只在打 tag 时构建，从不跑测试与静态检查；贡献指南、变更日志、统一构建入口均缺失。

用户要求不改变现有功能与用户可见行为。因此本次变更只做行为保持的性能、结构、正确性与文档加固，不新增功能，不改界面文案与交互语义。

## What Changes

### 1. 性能（输出与交互保持一致）

- **过滤与搜索**：空查询直接返回恒等索引，不再复制整份 `[]library.Track`；筛选结果改为直接产出源切片位置，去掉中间 `idToPos` 往返；搜索态不再重建不会被显示的分组缓存；光标闪烁消息不再触发重复过滤。
- **队列浮层**：建立 `track ID → 库内位置` 缓存，消除每帧 `O(queue × library)` 线性查找。
- **歌词渲染**：`maxTextW` 在歌词加载时计算一次，终端尺寸变化时失效，不再逐帧扫描全部行。
- **渲染分配**：`Marquee` 缓存文本宽度；渐变文本对 ASCII 使用无分配快路径；封面占位图纳入现有缓存；移除每帧构造的 `lipgloss.NewStyle()`；状态栏提示宽度改为单次求和。
- **持久化**：`SyncDir` 在单事务内用 prepared statement 批量 upsert，并按批提交；为 `playlist_tracks(playlist_id, position)` 补索引；为 `AllTracks` 的 `ORDER BY` 补表达式索引。
- **音频访问**：每次渲染只取一次播放位置与时长快照，避免重复进入播放器锁。
- **基准**：新增 `View`、`Update`、`applyFilter`、`selectTrackIndices`、队列浮层和同步写入的 benchmark，先记录基线再改动，并在 CI 中保留可复跑的基准入口。

### 2. 结构（只移动代码，不改逻辑）

- 按现有代码分块拆分超大文件：`app.go`、`keys.go`、`actions.go`、`render_player.go`，单文件目标不超过约 500 行。
- 将浮层 UI 状态从 `PlaylistState` 抽到独立的 `OverlayState`，保持字段访问路径与初始化顺序不变。
- 合并语义完全相同的滚动夹取与提示宽度计算助手；仅在输出逐字节一致时合并宽度算法。

### 3. 正确性（正常路径行为不变）

- `tea.Cmd` 闭包不再写 `App` 字段：播放模式与 Lo-Fi 档位改为消息回传，在 `Update` 中落状态。
- i18n 语言表与图标注册表改为不可变快照的原子替换，消除 `View` / `Update` 并发读写。
- 持久化失败不再静默：至少记录日志，并在会话保存失败时通过状态栏提示。
- 扫描支持取消并防重入：扫描进行中再次触发 `Ctrl+R` 不叠加新的扫描；退出时取消在途扫描。
- 明确迁移策略：新列/新表通过追加版本化迁移落地，使既有数据库能升级；同步修正 `ARCHITECTURE.md` 与代码注释。
- **唯一可见行为修正**：修复 `applyFormatPreference` 原地过滤（`tracks[:0]`）污染 `a.tracks` 底层数组的缺陷。`rebuildGroups` 传入曲库时，lossless-only / MP3-only 偏好会把过滤结果写回曲库，造成顺序与内容错乱。修复后该场景从错误结果变为正确结果，其余路径行为不变。
- 补上 `internal/library` 等零测试包的关键单测，并为本次修改的过滤、同步、索引路径添加回归测试。
- 让音频测试可移植：当前 `internal/audio` 测试硬编码本机音乐文件绝对路径，并在 `init()` 中无条件下初始化播放器；改为文件不存在或无音频设备时跳过，使 CI 可运行。

### 4. 死代码移除 **BREAKING**

删除运行时从未使用的导出 API 与包：

- `internal/provider`、`internal/provider/local` 及 `App.providerMap` / `resolveSource` 中的无效分支（保留 `Track.ProviderID` 数据字段）。
- `internal/library.ScanDir`、`internal/audio.Player.PlayAt`、`internal/store.Store.DeleteTrack`、`TrackListContext.ExtraHints`。

这些符号在仓库内无调用方。模块本身是应用而非公共库，因此不承诺外部导入兼容；仍需在 CHANGELOG 中标注。

### 5. 文档与仓库基础设施

- 新增 MIT `LICENSE`（与 README 声明一致）。
- 新增 `.github/workflows/ci.yml`：push / PR 上运行 `gofmt -l`、`go build`、`go vet`、`go test`，`-race` 排除已知 oto 初始化竞态的 `internal/audio`。
- 新增 `CONTRIBUTING.md`、`CHANGELOG.md`、`Makefile`（`build` / `test` / `vet` / `bench` 目标）。
- 修正 README：clone URL、设置面板键位、缺失的快捷键、Tab 顺序、F8 媒体键说明、工具链要求。
- 同步 `ARCHITECTURE.md` 与实现：迁移策略、状态所有权、全局状态并发模型。
- `.gitignore` 补充无关的 `.pytest_cache`。

### 不在本次范围

- 任何新功能、新 Tab、新设置项或交互变更。
- 跨包边界重组（除删除死代码外）。
- 被视为“可见行为”的缺陷修复，例如按字素簇截断文本这类会改变渲染结果的修正；除上文明确列出的格式偏好污染修复外，如需处理其他可见缺陷，单独提案。
- 英文文档翻译；`ARCHITECTURE.md`、`CLAUDE.md` 保持中文，README 维持中英双语。
- 修复 `internal/audio` 的第三方 oto 初始化竞态。

## Capabilities

### New Capabilities

无。本次是行为保持的重构、工具与文档变更，不引入新的系统行为契约。

### Modified Capabilities

无。`app-shell`、`library-browsing`、`playback-queue`、`playlist-management`、`lyrics-playback` 的行为契约均保持不变；所有快捷键、界面文案、持久化语义与失败提示路径维持现状。按 OpenSpec 规则，本变更在 `.openspec.yaml` 设置 `skip_specs: true`，不伪造行为需求。

## Impact

- **代码**：`internal/tui`（文件拆分与热路径）、`internal/store`（同步事务、索引、迁移）、`internal/library`（删除死代码、补测试）、`internal/audio`（删除死代码、渲染快照）、`cmd/music-tui`。
- **删除的包/API**：`internal/provider`、`internal/provider/local`；五个未使用的导出符号（见上）。
- **数据库**：新增索引与版本化迁移；既有数据库在启动时自动升级，数据不重写。
- **依赖**：不新增第三方依赖。
- **CI / 仓库**：新增 CI 工作流、LICENSE、CONTRIBUTING、CHANGELOG、Makefile。
- **风险**：超大文件拆分与状态迁移可能引入行为漂移；以现有测试、新增回归测试、基准对比和真实二进制 pty 冒烟为门禁，逐阶段提交。
