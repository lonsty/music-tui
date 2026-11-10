# Design

## Context

当前实现是单个 Bubble Tea 模型 `tui.App`，状态按 `PlaybackState` / `LibraryState` / `ChipState` / `LyricsState` / `PlaylistState` / `OnlineState` 分组嵌入。两个主视图共享 `TrackListContext` 引擎；`app.Init` 只启动定时器与会话恢复，不触发扫描；`main.go` 开启了 `tea.WithMouseCellMotion()` 但项目内没有任何 `MouseMsg` 处理。

数据层 `store` 已具备 `playlist_tracks.position`，但没有排序/插入方法；`Playlist` 结构没有曲目计数，渲染层因此在每帧逐项查询。`library.Track` 的 `DisplayArtist()` 返回英文 `"Unknown Artist"` 哨兵值，TUI 直接展示。

见 `proposal.md` 了解动机与阶段划分，见各能力 `specs/` 了解行为契约。

## Goals / Non-Goals

**Goals:**

- 每个阶段都能独立交付、独立回滚，不依赖后续阶段。
- 阶段 1 只改现有结构，不引入新的架构层。
- 曲库分组、队列、偏移等新状态都收敛到既有 `App` 子状态组，不继续膨胀根结构体。
- 渲染路径与数据库彻底解耦，所有查询前移到消息处理阶段。

**Non-Goals:**

- 不引入鼠标交互，不做可点击/可滚动界面。
- 不做流媒体或在线音源；Online 占位直接删除。
- 不做持久化播放队列、跨设备同步、账号体系。
- 不重写 Bubble Tea 架构，不引入状态管理库。
- 不做音频无缝播放（gapless）；本变更只保证控制流，不承诺消除解码间隙。

## Decisions

### D1. 删除 Online 占位而不是隐藏

删除 `tabOnline`、`OnlineState`、`renderOnlinePlaceholder`、`renderNormalBody` 分支、Tab 栏条目及相关 i18n。`tabOnline` 是最后一个枚举值，删除不改变 `tabPlaylist=0` / `tabLocal=1` 的持久化数值，`KeyLastActiveTab` 无需迁移。

替代方案：配置开关隐藏。否决——没有实现计划的入口不应占用导航和用户注意力。

### D2. 移除鼠标捕获，不做半成品

删除 `tea.WithMouseCellMotion()`。终端用户高频使用拖选复制，捕获鼠标却无处理会直接破坏既有习惯。

替代方案：实现滚轮滚动与点击选中。否决——收益低、跨终端差异大，且会扩大本期测试面；未来如需可做成显式 opt-in 参数。

### D3. 启动扫描走现有 `cmdSyncLibrary`

`App.Init` 在 `st != nil && musicDir != ""` 时追加 `cmdSyncLibrary()`，并置 `loading=true`。扫描在 `tea.Cmd` goroutine 中执行，`scanDoneMsg` 统一收口，沿用现有错误与曲库保留逻辑。

全局 `Ctrl+R` 只在 `activeOvl == overlayNone` 时接管，避免与设置编辑框、搜索框、歌单命名框冲突；设置面板内原有 `Ctrl+R` 行为保留。

替代方案：新增后台定时扫描。否决——本变更先把“启动即同步”做对，定时器属于额外配置面。

### D4. 搜索过滤保持可见，Esc 清空

`/` 输入时沿用 overlay；Enter 播放并关闭输入框后，`searchInput.Value()` 保留。列表头部在查询非空时渲染查询 chip 与“Esc 清除”。共享列表的 Esc 处理顺序改为：先清搜索过滤，再执行上下文 `OnEsc`（如歌单返回列表）。

替代方案：Enter 后清空过滤。否决——用户按 Enter 是为了边听边继续挑歌，清空会让上下文丢失。

### D5. 曲库分组采用“视图枚举 + 下钻引用”

新增 `libraryView` 枚举：`tracks`、`albums`、`artists`；新增 `groupCursor`、`drilldown`、`prevView`。分组在 `applyFilter` 之后一次性构建并缓存，渲染只读缓存。`v` 循环视图；在分组视图 Enter 设置 `drilldown` 并切到 `tracks`，Esc 清除 `drilldown` 回到 `prevView`。

搜索查询非空时强制 `tracks` 视图；清空查询后恢复 `prevView`。这样搜索语义只作用于曲目，不引入“搜索分组名”的第二套规则。

替代方案：为 Albums/Artists 各建一个独立 Tab。否决——Tab 是模式切换，分组是同一曲库的浏览维度，独立 Tab 会让 Tab 栏继续膨胀。

### D6. 列表上下文改为索引访问，并缓存 ID 位置

`TrackListContext` 的 `Tracks() []Track` 替换为 `Count() int` + `TrackAt(pos) Track`，消除每帧/每键的切片复制。Local 的 `TrackAt` 读取 `tracks[filteredIdxs[pos]]`。

`applyFilter` 与歌单加载时重建 `posByID map[string]int`，`filteredPos` 改为 O(1)。基准测试保留并新增一条“渲染 N 行不分配切片”的回归断言。

替代方案：保留 `Tracks()` 但缓存切片。否决——缓存失效点多，索引访问没有这个问题。

### D7. 歌单计数随列表一次查询

`store.Playlist` 增加 `TrackCount int`；`GetPlaylists` 改为 `LEFT JOIN playlist_tracks ... GROUP BY` 一次取回计数。渲染层只读 `pl.TrackCount`。

切换到歌单 Tab 时刷新一次 `cmdLoadPlaylists()`（单查询），增删命令在 `playlistTracksLoadedMsg` 中更新当前歌单计数并同步缓存。

替代方案：保留逐项 `PlaylistTrackCount` 并在渲染层缓存。否决——缓存失效逻辑分散在增删、导入、删除多个入口，容易再次退化成每帧查询。

### D8. 队列是 ID 列表，独立于过滤列表

`QueueState`：`ids []string`、`cursor int`。入队查 ID 去重；播放队列项走新增的 `cmdPlayTrackByID`，解析 `a.tracks` 中的 `Track` 后复用 `playResultMsg`，不修改 `filteredIdxs`。

消费优先级：队列 > 播放模式。单曲循环只在队列为空时重复当前曲。`n`、自然播完、F9 共用同一队列检查函数，避免三处逻辑漂移。

`Q` 打开队列浮层；队列不写数据库，退出即丢。

替代方案：复用 `filteredIdxs` 作为队列。否决——队列项不应改变曲库/歌单的可见顺序，且过滤变化会让索引失效。

### D9. 歌单排序用位置重写，撤销用“插入并后移”

新增 `store.MoveTrack(playlistID, trackID, delta)`：在事务内读取有序 `track_id`，交换相邻项，再按新顺序重写 `position`。曲目数量级小，整表重写比维护空洞位置更简单可靠。

撤销用 `store.InsertTrackAt(playlistID, trackID, position)`：事务内把 `position >= target` 的项统一 +1，再插入，避免位置冲突。App 保存 `lastRemoved{trackID, position, expiresAt}`，5 秒窗口按 500ms tick 判断，不额外起定时器。

替代方案：浮点 position。否决——需要 schema 迁移，且与本变更“无 schema 变化”的约束冲突。

### D10. 从浮层新建歌单复用现有输入流程

`overlayAddToPlaylist` 中按 `n` 关闭浮层、打开歌单命名输入；创建成功后串行执行“加入当前曲目”，复用 `cmdCreatePlaylist` + `cmdAddTrackToPlaylist`。不新增独立创建流程。

### D11. 歌词偏移存 settings 表，随歌词一起加载

新增 key 前缀 `lyrics_offset.` + `trackID`，值为毫秒。`cmdLoadLyrics` 在同一 goroutine 内读取偏移并随 `lyricsLoadedMsg` 返回，避免在 `Update` 里同步查库。

高亮计算改为 `pos - offset`；浏览定位 seek 到 `line.Time + offset`。纯文本歌词（无时间戳）不读不写偏移，按键只给提示。

替代方案：歌词文件旁边写 sidecar。否决——用户目录可能只读，且现有 settings 表已承担按曲目状态。

### D12. 设置面板布局由构建函数产生

抽取 `settingsLayout()` 返回 `rows []string` 与 `fieldRow map[settingsField]int`。渲染与键盘导航共用同一份结果，删除硬编码的 `settingsFieldBodyRow` 数组。

替代方案：保留硬编码并加测试。否决——测试只能事后发现错位，构建函数从结构上消除问题。

## Risks / Trade-offs

- [删除鼠标捕获让偏好鼠标的用户失望] → README 明确说明；未来以 opt-in 参数补回，不默认开启。
- [启动扫描与 UI 共用单连接 SQLite，扫描期间切歌单可能变慢] → 扫描保持批量写入；`busy_timeout` 已存在；列表查询在阶段 1 后降为单查询。
- [Tab 数值持久化依赖枚举顺序] → 删除末尾项安全；在 `tabID` 注释中写明“只能追加，不能重排”，后续若需重排改为名称持久化。
- [分组视图与共享列表引擎并存增加状态分支] → 分组视图只负责行渲染与光标，不复制播放/浮层逻辑；按键先分流再进入共享引擎。
- [队列优先级可能与用户对单曲循环的预期冲突] → 规格已明确“队列优先”；队列面板显示“下一首”，状态栏在队列非空时显示队列数。
- [整表重写 position 在超大歌单上变慢] → 歌单通常为百级；若未来出现万级歌单，改为间隔 position 或独立排序表。
- [每曲目歌词偏移 key 增长] → 仅在用户实际调整时写入；键数量上限等于曲库曲目数，可接受。
- [阶段 1 删 Tab 会影响保存的 `last_active_tab=2`] → 启动恢复已有 `id < tabCount` 校验，2 会回落到默认 Local，不会崩溃。

## Migration Plan

1. 阶段 1 合入并验证：`CGO_ENABLED=1 go build ./... && go test ./... && go vet ./...`。
2. 阶段 2–5 各自独立提交/合入；每阶段结束跑同一组命令，并手工验证该阶段新增场景。
3. 无数据库 schema 迁移；新增 settings key 为追加式，旧版本读取时忽略未知 key。
4. 回滚按阶段整体 revert；不存在跨阶段数据依赖，回滚后旧数据仍可读。

## Open Questions

无。会影响规格、方案或任务拆分的未知项已在决策中确定。
