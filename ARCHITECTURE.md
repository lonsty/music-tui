# 架构说明

> Last verified: 2026-11-11
>
> 维护规则：本文只记录稳定的分层、状态所有权和不变量。不要在这里维护文件清单、行数或逐文件说明；需要细节时直接链接代码。修改包边界、持久化结构、TUI 状态所有权或并发模型时，必须同步更新本文。

## 1. 分层与依赖方向

```text
cmd/music-tui
      │
      ▼
internal/tui ──► internal/audio
      │              │
      ├──► internal/store
      ├──► internal/library
      └──► internal/lyrics ──► internal/lyrics/online
```

- 依赖只能向下，不允许反向导入；`library` 是最底层的数据模型。
- `store` 负责所有 SQL。TUI 渲染路径不得直接查询数据库。
- `audio` 通过 `StreamSource` 接受本地文件；在线音源接入时实现该接口，不经过已删除的空 provider 抽象。
- `lyrics` 通过 `Provider` 接口组合本地 `.lrc` 与在线缓存。

## 2. TUI 状态模型

`tui.App` 是唯一的根模型，按领域嵌入状态组：

| 状态组 | 负责 |
|--------|------|
| `PlaybackState` | 当前曲目、音量、播放模式、Lo-Fi 档位 |
| `LibraryState` | 曲库、过滤索引、分组缓存、浏览视图、下钻 |
| `ChipState` | 8-bit 转换状态与临时目录 |
| `LyricsState` | 歌词行、当前高亮、浏览游标、时间偏移 |
| `PlaylistState` | 歌单列表、曲目列表、输入模式、撤销状态 |
| `OverlayState` | “加入歌单”浮层与内联添加曲目面板的临时状态 |
| `QueueState` | 临时播放队列 |

关键不变量：

- `tracks` 是曲库唯一真相；`filteredIdxs` 是它的位置索引表。
- `posByID` 是 `filteredIdxs` 的 ID → 位置缓存，两者必须同时重建。
- 任何改动 `filteredIdxs` 的路径都必须调用 `rebuildFilteredPos()`。
- `trackPosByID` 是 `tracks` 的 ID → 位置缓存；替换 `tracks` 时必须重建。
- 空查询且无格式偏好时，`filteredIdxs` 直接引用 `identityIdxs`；该表只读。
- 分组缓存（`albumGroups`、`artistGroups`）在 `tracks` 或格式偏好变化时标记为脏，首次读取时重建，渲染只读。
- Local 与歌单曲目层级共用 `TrackListContext` 引擎；差异只放在 context 回调里。

## 3. 事件与并发

- Bubble Tea 的 `Update` 是唯一允许写入 `App` 状态的地方。`tea.Cmd` 只计算目标值并以消息返回，不直接改字段。
- `tea.Cmd` 的 goroutine 只允许访问线程安全的播放器、网络客户端和只读配置；结果通过消息回到 `Update`。
- 语言与图标集是进程级全局状态，通过原子快照发布；`View` 与 `Update` 可并发读取。
- 曲库扫描最多同时运行一个：`scanCancel` 取消在途任务，`loading` 阻止重入；退出时取消。
- 切歌或搜索时会取消上一笔网络请求（`netCancel`），避免旧响应污染新状态。
- `tickMsg` 每 500ms 驱动进度、跑马灯和歌词高亮；所有时间相关状态以它为准。
- 每次渲染在入口取一次播放位置/时长/状态快照，渲染函数不再重复进入播放器锁。

## 4. 持久化

- SQLite 位于 XDG 数据目录；schema 迁移集中在 `internal/store/db.go` 的单个迁移表。
- 新增字段/表一律追加迁移，不修改已发布的迁移。
- 用户配置、会话和按曲目歌词偏移存在 `settings` 表，键名集中在 `settings_keys.go`。
- 歌单顺序存于 `playlist_tracks.position`，排序/插入必须在一个事务内重写位置。
- 播放队列不落库，退出即清空。

## 5. 扩展检查清单

- **新增音频格式**：同时更新 `library/formats.go` 白名单与 `audio/source.go` 解码器注册。
- **新增 Tab**：`tabID` 只能追加，不能重排；持久化的 `last_active_tab` 依赖数值稳定。
- **新增浮层**：在 `keymap.go` 注册表中为该 scope 声明绑定；按键分发、渲染分发、状态栏 / footer / 帮助浮层 / README 表格均由注册表生成，无需手工同步多条优先级链。
- **新增设置项**：加 `store.Key*`、双语 i18n、`NewApp` 恢复逻辑和设置面板布局计划。
- **新增用户文案**：必须同时写入 `i18n.go` 的中英文表，禁止在渲染/状态代码里写字面量。
- **新增后台任务**：返回 `tea.Msg`，不要在 goroutine 里改 `App` 字段。

## 6. 验证

```bash
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go vet ./...
openspec validate --strict  # 有进行中的 OpenSpec 变更时
```

## 7. 非目标

- 不追求录音室级无缝播放（gapless）；当前只保证控制流连续。
- 不提供鼠标交互，保留终端原生选择/复制。
- 不把 `internal/tui` 拆成微服务式模块；共享引擎优先于抽象层。
