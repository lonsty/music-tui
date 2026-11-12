# Tasks

## 1. 注册表与上下文栈

- [ ] 1.1 新建 `internal/tui/keymap.go`，定义 ActionID、`Keys`、`DisplayKey`、`LabelKey`、`DescKey`、`Group`、`ShortRank`、`Scopes`、`Enabled` 字段与注册表；验证：`go build ./internal/tui` 通过且字段覆盖 design.md D1。
- [ ] 1.2 用 `bubbles/key` 的 `key.NewBinding` / `key.Matches` / `help.KeyMap` 实现注册表到绑定、匹配与帮助数据的转换；验证：单元测试覆盖一键多别名与 `SetEnabled(false)` 过滤。
- [ ] 1.3 实现固定上下文栈 `input > confirm > overlay > fullscreen > settings > group > tracklist > global`，首个绑定该键的 scope 消费按键；验证：单元测试覆盖同键在不同 scope 落到不同 ActionID。
- [ ] 1.4 实现启动校验：同 scope 重复键、禁用组合清单、ActionID 中英描述齐全，失败阻止启动并报出 scope / 键 / ActionID；验证：三项各有失败用例且错误信息含具体字段。
- [ ] 1.5 新增 `internal/tui/keymap_test.go` 覆盖 1.1-1.4；验证：`CGO_ENABLED=1 go test ./internal/tui -run Keymap` 通过。

## 2. 最终键表与分发

- [ ] 2.1 按 design.md D4 表填充全部 ActionID 与最终物理键，`v`、`Q`、`z` 及旧语义不绑定；验证：注册表快照断言 `v` / `Q` / `z` 无绑定，`1`/`2`/`3`/`4`/`e`/`E`/`N`/`F2`/`n` 一一对应。
- [ ] 2.2 把 `keys*.go`、`groups.go`、`queue.go`、`context_*.go`、`update.go` 的手写 `switch msg.String()` 改为注册表 `key.Matches` 分发；验证：`grep -rn 'msg.String()' internal/tui --include='*.go'` 无生产代码命中。
- [ ] 2.3 修全屏浮层陷阱：`overlay` 排在 `fullscreen` 之前，且 `render.go` 渲染链同步；验证：新增测试断言全屏按 `?` 显示帮助浮层、按 `Esc` 只关帮助而保留全屏。
- [ ] 2.4 修文本输入劫持：`q`、`j`、`k` 与 F6-F12 不再在输入态被拦截；`overlay.settings.edit` 保留 `Ctrl+R` 保存并重扫；验证：新增测试覆盖命名框与 add-tracks 搜索框输入 `q`/`j`/`k` 可写入，以及设置编辑态 `Ctrl+R` 仍触发重扫。
- [ ] 2.5 统一 `library.like` 目标为选中曲目，修分组视图 `L` / `a` / `i` / `e` / `E` 操作目标错位；验证：新增测试断言分组视图下动作作用于 `groupCursor` 对应曲目。
- [ ] 2.6 把 `phase4_test.go:86-129` 的加入歌单浮层新建断言由 `n` 改为 `N`；验证：`CGO_ENABLED=1 go test ./internal/tui -run 'TestPhase4|Test.*AddToPlaylist'` 通过。
- [ ] 2.7 新增分发顺序测试：浮层内 `q` 无动作、`4` 打开队列、`Q` 无动作、`n` 不新建歌单；验证：`CGO_ENABLED=1 go test ./internal/tui -run Dispatch` 通过。

## 3. 四层提示渲染与 golden

- [ ] 3.1 实现 T1 状态栏六槽固定顺序与 pinned 丢弃（上下文动作 → 播放 → 导航），`?`、主操作、退出永不丢弃；验证：新增测试覆盖 80 列完整显示与窄终端丢弃顺序，并核对 `render_statusbar.go` 注释与实现一致。
- [ ] 3.2 实现 T2 浮层 footer：2-4 chip，`Esc` 永远最后；浮层打开时状态栏不出 chip；验证：新增测试逐浮层断言 chip 顺序与最后一个为 `Esc`。
- [ ] 3.3 实现 T3 帮助浮层五区 `当前上下文 → 全局 → 曲库 → 歌单 → 媒体键`，3/4 区去重，新增 `help_section_current` 中英 i18n key；验证：新增测试断言分区顺序、当前上下文区内容与去重结果。
- [ ] 3.4 实现 T4 `go:generate` 回填 README 的 `<!-- BEGIN GENERATED KEYS -->` / `<!-- END GENERATED KEYS -->` 块，每 scope 一张表；验证：`go generate ./...` 后生成块与注册表一致且标记外内容未被改动。
- [ ] 3.5 实现固定分组顺序与组内 `ShortRank` 降序、ActionID 升序排序函数，T1/T2/T3/T4 共用；验证：新增测试断言同 scope 两次渲染顺序一致、共享键槽位一致。
- [ ] 3.6 `TrackListContext` 增加 `EmptyHints []ActionID`，空态提示行由注册表生成 chip；验证：新增测试断言空态 chip 随注册表改键变化。
- [ ] 3.7 重生成 `golden_test.go` 全部 10 个 case 对应的 `testdata/golden/*.golden`，并补断言帮助浮层快照覆盖全部分区行；验证：`CGO_ENABLED=1 go test ./internal/tui -run TestGoldenRender` 通过且 `help.golden` 行数大于原 29 行。
- [ ] 3.8 新增显示测试：页面不显示未绑定键、`playlist.undo` 仅在撤销窗口出现、`playback.right_panel` 仅在 `showMiniPlayer()` 为真时出现、`queue.open` 始终出现；验证：`CGO_ENABLED=1 go test ./internal/tui -run Hints` 通过。

## 4. 文档、文案与注释同步

- [ ] 4.1 改写 README 生成块之外的段落：`:17-24` 特性表、`:164` 浮层新建说明、`:311` 媒体键提示、`:356`、`:378`，全部按最终键表；验证：`grep -nE '`v`|`Q`|`z`|按 `n` 新建' README.md` 无旧键残留。
- [ ] 4.2 清理 i18n：键名不再内嵌正文；`8-bit` 改 `hint_chip` 并补中英；`Spc` 归一为 `Space`；删除矛盾的 `help_close`、启用 `help_close_key`、把 `action_reload_library` 接进帮助全局区；其余未用 key 按注册表接线或删除；验证：`CGO_ENABLED=1 go test ./internal/tui -run Translation` 通过且 EN/ZH key 集合相等。
- [ ] 4.3 `i18n_test.go` 增加断言：文案值不含 `❮`、`press <key>` 或 `按 <单键>` 模式；验证：新增用例构造含键名文案时测试失败。
- [ ] 4.4 逐条同步代码注释清单：`render_statusbar.go:27-40,112-113,133-134,159,177-178`、`keys_tracklist_shared.go:17-26,327,348,419,430`、`keys.go:23-24,99-119,141,150,173,191`、`keys_search.go:42-43,59`、`keys_fullscreen.go:111`、`model.go:58`、`tracklist_context.go:49,59-60,75`、`app.go:154`、`update.go:76`、`groups.go:180-181`、`render_tab_playlist.go:153`、`render_tracklist_shared.go:40,89` 及 `render_overlays.go` 键位注释；验证：源码扫描测试断言注释中不出现旧键序列与硬编码键名。
- [ ] 4.5 更新 `ARCHITECTURE.md:71` 为「注册表自动生成渲染与提示，新增浮层只需声明 scope 与绑定」，并在 `CONTRIBUTING.md:33` 补 `go generate ./...` 说明；验证：按文档步骤执行 `go generate ./...` 成功。
- [ ] 4.6 新增一致性测试：spec 中出现的每个 ActionID 都存在于注册表；渲染与提示文件无硬编码键 chip；README 生成块与注册表一致；验证：`CGO_ENABLED=1 go test ./internal/tui -run Consistency` 通过。

## 5. 集成验证

- [ ] 5.1 跑完整门禁 `CGO_ENABLED=1 go build ./... && CGO_ENABLED=1 go test ./... && CGO_ENABLED=1 go vet ./...`；验证：三条命令全部退出码 0。
- [ ] 5.2 跑 `CGO_ENABLED=1 go test -race ./...`；验证：通过，若 `internal/audio` 的 oto 初始化竞态复现则隔离或书面说明。
- [ ] 5.3 真机 pty 冒烟：启动、切 Tab、`1/2/3` 切层级、`4` 开队列、`N` 新建、`F2` 重命名、`E`/`e` 入队、`?` 帮助、退出；验证：全程无 panic 且各动作产生预期界面变化。
- [ ] 5.4 做一次独立 review，检查所有用户可见文案中英齐全、`filteredIdxs` 与 `posByID` 同步、渲染路径无 DB 查询、后台 `tea.Cmd` 不直接写 App 状态、`tabID` 只追加；验证：review 结论记录且无未修复的严重问题。
- [ ] 5.5 跑 `openspec validate redesign-shortcut-system --strict`；验证：退出码 0 且无 warning。

## Workflow follow-up

- 实现完成且门禁通过后，按 OpenSpec archive 流程归档本 change。
- 归档 / sync 时把 6 份 delta spec 合并进 `openspec/specs/`，其中 `keybinding-system` 会新建 main spec。
