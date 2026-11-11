# Tasks

## 1. 基线与行为锁定

- [x] 1.1 新增 golden 渲染测试骨架，固定 `W/H`、语言、图标集与测试曲库，覆盖普通双栏、全屏播放器、全屏歌词、搜索态、帮助/设置/队列/歌单/新建歌单浮层；用当前 HEAD 生成基线文件，验证 `go test ./internal/tui -run 'Golden'` 通过。
- [x] 1.2 为 `internal/library` 补单测：`SortByArtistAlbum`、`QualityOf`/格式白名单、`ParseTrackWithCover`（用临时目录生成最小标签文件）；验证 `go test ./internal/library` 通过且覆盖率不再为 0。
- [x] 1.3 为 `internal/store` 补迁移与同步测试：临时库首次建表、重复 `Open` 幂等、`SyncDir` 新增/更新/删除分类；验证 `go test ./internal/store` 通过。
- [x] 1.4 新增基准：`BenchmarkApplyFilter10K`、`BenchmarkSelectTrackIndices10K`、`BenchmarkViewNormal10K`、`BenchmarkViewFullscreen10K`、`BenchmarkRenderQueueOverlay200`、`BenchmarkUpdateKey10K`；验证 `make bench` 可运行，并把改动前数值记录到 PR 描述。
- [x] 1.5 把现有 i18n 覆盖测试扩展为同时检查动态键与空翻译（复用 `T()` 调用点扫描），验证 `go test ./internal/tui -run 'Translation'` 通过。

## 2. 开源基础设施与文档

- [x] 2.1 新增 MIT `LICENSE`，版权行为 “Copyright (c) 2025-2026 music-tui contributors”；验证 README 的 License 段落与文件一致。
- [x] 2.2 新增 `.github/workflows/ci.yml`：Ubuntu 安装 `libasound2-dev`，执行 `gofmt -l` 零输出检查、`go build ./...`、`go vet ./...`、`go test ./...`，并新增排除 `internal/audio` 的 `-race` 任务；验证工作流在 feature 分支的一次 push 上全绿。
- [x] 2.3 新增 `Makefile`（`build`/`test`/`vet`/`fmt`/`bench`）、`CONTRIBUTING.md`、`CHANGELOG.md`（Keep a Changelog）；验证 `make build test vet fmt` 全部成功。
- [x] 2.4 修正 README：clone URL 改为 `github.com/lonsty/music-tui`，补 `a`/`l`/`\`/`L` 与全屏 `?`/`[`/`]`/`↑↓`/`Enter` 键位，修正设置面板键位与 Tab 顺序，补 F8 说明与 `GOTOOLCHAIN=auto` 要求；逐条对照 `internal/tui/keys*.go` 与 `model.go` 验证。
- [x] 2.5 修正 `ARCHITECTURE.md` 与 `internal/store/db.go` 的迁移说明，统一为“已发布迁移不可修改，只能追加”；验证文档描述与实现一致。
- [x] 2.6 `.gitignore` 追加 `.pytest_cache/`；验证 `git status` 不再出现无关缓存目录。

## 3. 正确性修复

- [x] 3.1 修复 `applyFormatPreference` 在 lossless-only / MP3-only 下复用输入底层数组的问题（改为不修改入参）；新增测试断言 `rebuildGroups` 前后 `a.tracks` 顺序与内容不变，验证 `go test ./internal/tui` 通过。
- [x] 3.2 将 `cmdNextPlayMode` / `cmdRetroUp` / `cmdRetroDown` 改为返回专用消息，状态只在 `Update` 写；新增测试断言 `Update` 后 `playMode`/`retroIdx` 正确、shuffle 已重建，验证 `go test ./internal/tui` 通过。
- [x] 3.3 将 i18n 语言表与图标集改为原子快照；新增并发测试（多 goroutine 调 `SetLang`/`T`、`setIconSet`/`icon`）并验证 `go test -race ./internal/tui -run 'Concurrent'` 通过。
- [x] 3.4 新增持久化错误上报助手与中英文 i18n 键；验证失败写触发日志与状态栏提示（用已关闭的临时库构造失败），并验证 i18n 覆盖测试通过。
- [x] 3.5 为扫描增加 `context.WithCancel` 与 `loading` 防重入，修复 feeder 协程在 worker 退出后阻塞的问题；新增测试验证取消后 `SyncDir` 及时返回、连续触发只启动一次扫描。
- [x] 3.6 让 `internal/audio` 测试可移植：去掉硬编码本机路径，文件不存在或 `NewPlayer` 失败时 `t.Skip`；验证 `go test ./internal/audio` 在无音频文件的机器上跳过并通过。

## 4. 存储层

- [x] 4.1 将迁移改为逐版本追加，新增 migration 2：`playlist_tracks(playlist_id, position)` 索引；验证新建库与 v1 旧库升级后 schema 一致，`go test ./internal/store` 通过。
- [x] 4.2 新增 `tracks` 排序表达式索引，与 `AllTracks` 的 `ORDER BY` 完全一致；新增测试断言 `EXPLAIN QUERY PLAN` 不出现 `USE TEMP B-TREE`。
- [x] 4.3 `SyncDir` 改为单事务 + prepared upsert，每 1000 行分批提交，保持部分失败继续语义；新增测试覆盖全成功、部分解析失败、取消中途，验证计数与数据库内容正确。
- [x] 4.4 为 `MoveTrack` 增加 `(playlist_id, position)` 命中验证与回归测试，确认 J/K 顺序结果不变；验证 `go test ./internal/store` 通过。

## 5. 性能优化

- [ ] 5.1 重写 `selectTrackIndices` 为位置直出、去掉整库 Track 复制；新增表驱动测试覆盖空查询、前缀、格式偏好、drilldown，验证结果与旧实现一致，`BenchmarkSelectTrackIndices10K` 的分配显著下降。
- [ ] 5.2 增加恒等索引缓存与 `groupsDirty` 惰性重建；新增测试覆盖“搜索→清除→切换 Albums”“格式切换后进入 Albums”，验证分组内容正确、搜索期间不重建。
- [ ] 5.3 搜索输入转发路径仅在查询值变化时调用 `applyFilter`；新增测试用光标闪烁消息验证 `filteredIdxs` 不变且未重建。
- [ ] 5.4 新增 `trackPosByID` 缓存并改造 `findTrackByID`；新增测试覆盖队列中包含被过滤曲目，验证 `BenchmarkRenderQueueOverlay200` 不再随曲库线性增长。
- [ ] 5.5 歌词 `maxTextW` 在加载时计算、宽度变化时失效；新增测试验证高亮行与截断结果不变，验证不再逐帧全量测量。
- [ ] 5.6 `Marquee` 缓存 `textW` 并在 `Tick` 复用；新增测试验证滚动起止与文本宽度不变，基准分配下降。
- [ ] 5.7 `gradientText` 增加 ASCII 无分配快路径；验证 golden 渲染测试逐字节一致，并记录该函数基准变化。
- [ ] 5.8 封面占位图纳入 `coverRendered` 缓存；新增测试断言无封面曲目多帧渲染结果相同且只构建一次。
- [ ] 5.9 将每帧构造的样式提升为包级变量，状态栏提示宽度改为单次求和；验证 golden 一致，`BenchmarkViewNormal10K` 分配下降。
- [ ] 5.10 音频位置/时长改为每帧快照；验证 golden 一致，并新增测试断言渲染期间只读取一次播放器位置。

## 6. 结构整理与死代码

- [ ] 6.1 删除 `internal/provider`、`internal/provider/local`、`App.providerMap`、`resolveSource` 的 provider 分支及四个未使用导出符号；验证 `go build ./...`、`go vet ./...`、`go test ./...` 通过，`grep` 无残留引用。
- [ ] 6.2 从 `PlaylistState` 抽出 `OverlayState` 并嵌入 `App`，字段名不变；验证 golden 与全量测试通过。
- [ ] 6.3 按 `design.md` D11 拆分 `app.go`；每移动一个文件后验证 `go build ./...` 与 golden 测试通过，最终 `app.go` 只保留必要的入口与共享类型。
- [ ] 6.4 拆分 `keys.go` 与 `actions.go`；每移动一个文件后验证构建与 golden 通过。
- [ ] 6.5 拆分 `render_player.go`；验证 golden 逐字节一致。
- [ ] 6.6 合并语义完全相同的滚动夹取与提示宽度助手（仅在输出一致时）；验证 golden 与相关单测通过。
- [ ] 6.7 更新 `ARCHITECTURE.md` 与 `README.md` 以反映删除的 provider 层、新增的 `OverlayState`、原子全局状态与扫描取消模型；验证文档与代码一致且不带逐文件清单。

## 7. 集成验证

- [ ] 7.1 运行完整门禁：`CGO_ENABLED=1 go build ./...`、`go test ./...`、`go vet ./...`、`gofmt -l cmd internal` 零输出；记录结果。
- [ ] 7.2 运行 `go test -race $(go list ./... | grep -v internal/audio)`，验证通过；`internal/audio` 保留已知 oto 竞态说明。
- [ ] 7.3 真实二进制 pty 冒烟：启动、Tab 切换、搜索、播放/暂停、打开队列与设置、退出，验证无 panic 且退出码为 0。
- [ ] 7.4 用 `benchstat` 对比 `design.md` 记录的基线，把结果写入 PR 描述；确认新增基准路径的分配与耗时符合预期。
- [ ] 7.5 运行 `openspec validate improve-maintainability-and-performance --strict`，验证通过。

## Workflow follow-up

- 在评审通过后归档该 change。
- 归档后运行 `openspec validate --strict` 确认主 specs 未受影响。
