# Design

## Context

见 `proposal.md` 的 Why。当前实现有 11 个文件包含按键分发，120 余处 `switch msg.String()` 字面量；全仓未使用 `bubbles/key` 或任何集中式 keymap。模式由单个 `activeOvl` 枚举加 8 个并列状态字段表达，`keys.go:100-179` 的 if 顺序隐式决定优先级，`render.go:232-262` 另有一条必须手工同步的渲染链。键位提示分布在状态栏、浮层 footer、帮助浮层、README、i18n 表与 golden 快照。已确认缺陷：全屏短路使 `?` 帮助浮层无法显示、`,` 设置键为死键、输入框的 `q` / `j` / `k` 被抢、帮助浮层列出当前页面不存在的键、分组视图对象操作目标错位。

已有可复用资产：`TrackListContext`（`tracklist_context.go:9-82`）已是分层上下文对象；`actions_*.go` 的命令层已集中；`go.mod` 已有 `github.com/charmbracelet/bubbles v1.0.0`。`settings_keys.go` 无键位持久化项。

## Goals / Non-Goals

**Goals:**

- 物理键只在注册表定义一次；分发、状态栏、footer、帮助浮层、README 全部派生。
- 用上下文栈替代隐式 if 链，修掉输入劫持与全屏浮层陷阱。
- 落地最终键表，五个规格改用稳定 ActionID 引用。
- 每个页面显示哪些键、按什么顺序，有固定规则且可测试。
- 文档、注释、hint、help、i18n、golden 全部同步。

**Non-Goals:**

- 用户自定义键位、键位持久化、兼容开关、迁移代码。
- 新增静音、鼠标支持、数字前缀计数、Helix 式前缀层。
- 修改 `tabID` 枚举、`KeyLastActiveTab` 或任何 DB schema。
- 新增第三方依赖。

## Decisions

### D1. 注册表是唯一来源，ActionID 为稳定动作词汇

每条记录包含：`ActionID`、`Keys []string`、`DisplayKey`、`LabelKey`、`DescKey`、`Group`、`ShortRank`、`Scopes`、`Enabled(App) bool`。不设 `Alias` 字段。`ActionID` 如 `playback.next`、`queue.append` 是面向规格与文档的稳定动作名，不是 Go 类型或函数名；规格场景通过它描述触发条件，物理键只在 `Keys` 中出现。

替代方案：规格继续写物理键，另加一致性测试。否决——改键仍需编辑 5 份规格，单源不彻底。

### D2. 复用已安装的 bubbles v1.0.0

用 `key.NewBinding(key.WithKeys(...), key.WithHelp(...))` 表达一条记录，`key.Matches` 做匹配，`help.KeyMap.ShortHelp() / FullHelp()` 做提示渲染，禁用项用 `SetEnabled(false)`。`ShortHelp` 自带按宽度截断，但本设计把 T1 上限设为 6 个 chip，截断只是窄终端兜底。

替代方案：自建 `map[string]ActionID`。否决——已安装依赖覆盖多键绑定、启用过滤与帮助渲染，自建代码更多。替代方案：启用 Kitty keyboard protocol 以支持 `Ctrl+Shift+字母`。否决——终端覆盖不确定，且本设计直接禁用这些组合。

### D3. 上下文栈替代隐式优先级链

固定顺序：`input > confirm > overlay > fullscreen > settings > group > tracklist > global`。按键自栈顶向下解析，首个绑定该键的 scope 消费它。`global` 只在未被捕获时兜底。

- 文本输入 scope（`search.inline`、`addtracks.inline`、`playlist.rename`、`overlay.add.create`、`overlay.settings.edit`）只把可打印字符交给输入框。例外：`overlay.settings.edit` 显式绑定 `global.reload`，保存目录后重扫，保留现网行为。
- 浮层为模态，只认自身绑定加 `Esc`、`?`、`global.reload` 与媒体 F 键；不继承 `playback.next` 等全局播放键。
- `overlay` 排在 `fullscreen` 之前，且渲染链同步，修掉全屏帮助浮层陷阱。

替代方案：保留现有 if 链，只补几个分支。否决——输入劫持与浮层陷阱都源于优先级链缺层，补分支会继续漂移。

### D4. 最终键表

按固定分组顺序列出。`Scope` 列中 `global` 表示所有非文本输入 scope 可用；未列出的 scope 不绑定该键。

| Group | ActionID | Keys | Scope |
|---|---|---|---|
| 退出/取消 | `overlay.cancel` | `Esc` | 嵌套 scope 与全部浮层 |
| 退出/取消 | `global.quit` | `q` | 根 scope |
| 主操作 | `playback.play` | `Enter` | tracklist、groups、playlist.list |
| 主操作 | `playback.pause` | `Space` | playing / paused scope |
| 主操作 | `playlist.open` | `Enter` | playlist.list |
| 主操作 | `lyrics.seek_line` | `Enter` | fullscreen |
| 导航 | `nav.down` / `nav.up` | `j` / `↓`，`k` / `↑` | 全部列表与滚动浮层 |
| 导航 | `nav.bottom` / `nav.top` | `G` / `g` | 全部列表 |
| 导航 | `nav.page_down` / `nav.page_up` | `PageDown` / `Ctrl+D`，`PageUp` / `Ctrl+U` | tracklist |
| 导航 | `nav.jump_playing` | `c` | tracklist、groups |
| 播放 | `playback.next` / `playback.prev` | `n` / `p` | global |
| 播放 | `playback.seek_forward` / `seek_back` | `>` / `<` | global、fullscreen |
| 播放 | `playback.volume_up` / `volume_down` | `+` / `=`，`-` | global、fullscreen |
| 播放 | `playback.mode` | `m` | global、fullscreen |
| 播放 | `playback.chip` | `b` | global、fullscreen |
| 播放 | `playback.lofi_down` / `lofi_up` | `r` / `R` | global、fullscreen |
| 播放 | `lyrics.offset_earlier` / `offset_later` | `[` / `]`，`,` / `.` | fullscreen 与 tracklist（同步歌词）；`,` / `.` 仅在 fullscreen 生效 |
| 当前对象 | `library.search` | `/` | local.tracks |
| 当前对象 | `library.clear_filter` | `Esc` | 过滤态 |
| 当前对象 | `library.add_to_playlist` | `a` | local.tracks |
| 当前对象 | `library.track_info` | `i` | local.tracks、playlist.tracks |
| 当前对象 | `library.like` | `L` | tracklist 与 group，目标为选中曲目；playlist.list 不绑定 |
| 当前对象 | `queue.append` | `e` | tracklist |
| 当前对象 | `queue.insert_next` | `E` | tracklist |
| 当前对象 | `queue.remove` | `x` | playlist.tracks、overlay.queue |
| 当前对象 | `queue.clear` | `C` | overlay.queue |
| 当前对象 | `playlist.add_track` | `a` | playlist.tracks |
| 当前对象 | `playlist.remove_track` | `x` | playlist.tracks |
| 当前对象 | `playlist.undo` | `u` | playlist.tracks（撤销窗口内） |
| 当前对象 | `playlist.move_down` / `move_up` | `J` / `K` | playlist.tracks |
| 当前对象 | `playlist.new` | `N` | playlist.list、overlay.add.list |
| 当前对象 | `playlist.rename` | `F2` | playlist.list |
| 当前对象 | `playlist.delete` | `d` | playlist.list |
| 视图/面板 | `library.view_tracks` / `view_albums` / `view_artists` | `1` / `2` / `3` | local |
| 视图/面板 | `queue.open` | `4` | global |
| 视图/面板 | `playback.fullscreen` | `f` | global |
| 视图/面板 | `playback.right_panel` | `l` | global（`showMiniPlayer()` 为真时） |
| 视图/面板 | `playback.collapse_panel` | `\` | global |
| 视图/面板 | `global.switch_tab` | `Tab` | global |
| 视图/面板 | `settings.open` | `,` | 主视图 |
| 视图/面板 | `settings.close` | `Esc` | overlay.settings |
| 系统 | `global.help` | `?` | global |
| 系统 | `global.reload` | `Ctrl+R` | global 与 overlay.settings.edit |
| 系统 | `global.media_prev` / `media_next` / `media_play` / `media_stop` / `media_vol_down` / `media_vol_up` | `F6` / `F9` / `F7` / `F8` / `F11` / `F12` | 非文本输入 scope |
| 系统 | `global.force_quit` | `Ctrl+C` | 全部 scope |

`,` 在主视图打开设置，在 fullscreen 作为歌词偏移等效键；同一物理键在不同 scope 绑定不同 ActionID，由栈隔离，不视为冲突。`v`、`Q`、`z` 完全不绑定。`r` 只做 Lo-Fi 降档，`n` 全局只做下一曲。`Ctrl+C` 是唯一保留的 legacy 控制组合：应用在 raw mode 下自行处理中断，注册表校验清单将它排除。

替代方案：Helix 式前缀层（`Space` leader）。否决——音乐播放器主流是单键，且两次击键增加高频操作成本。替代方案：保留 `v` 循环并只补 `1/2/3`。否决——用户已确认不留 alias。

### D5. 四层提示与六槽顺序

- T1 状态栏：最多 6 chip，固定槽位 `?` → 主操作 → 导航 → 播放 → 上下文动作 → 退出。窄终端按 `上下文动作 → 播放 → 导航` 顺序丢弃；`?`、主操作、退出标记 pinned，永不丢弃。宽度用现有 `strWidth` 按中英实测，目标 80 列放下 6 chip。
- T2 浮层 footer：2-4 chip，顺序为 `主操作/确认 → 上下文动作（按分组序） → Esc 取消/关闭`。滚动浮层附 `↑/↓` 文字提示，不占 chip。浮层打开时状态栏不出 chip。
- T3 帮助浮层：五区固定顺序 `当前上下文 → 全局 → 曲库 → 歌单 → 媒体键`；3/4 区去掉已在 1/2 出现的行。新增 i18n key `help_section_current`，中英各一。
- T4 README：`<!-- BEGIN GENERATED KEYS -->` / `<!-- END GENERATED KEYS -->` 标记块由 `go:generate` 回填，按 T3 组合 scope 生成表，顺序与 T3 一致。
- 曲库搜索与歌单「添加曲目」两个内联输入面板的 T2 footer SHALL 都紧跟输入行下一行，位置一致；不放在结果区底部。
- 空态多行文案 SHALL 逐行独立居中，不依赖 `lipgloss.Place` 对多行块按最宽行左对齐的行为。

替代方案：保留当前「从右往左丢弃」。否决——`render_statusbar.go:28-40,112-113` 的注释声称「priority 1 = always shown (quit)」，与实现相反；pinned + 显式丢弃顺序可测试且注释一致。

### D6. 固定分组顺序

分组顺序 `退出/取消 → 主操作 → 导航 → 播放 → 当前对象 → 视图/面板 → 系统`。组内按 `ShortRank` 降序、ActionID 升序。T1/T2/T3/T4 共用同一排序函数，保证同一键在不同页面位置稳定。

相对动作（`n`/`p`、`<`/`>`、`+`/`-`、`r`/`R`、`[`/`]`、`g`/`G`、`e`/`E`、`J`/`K`、F6/F9、F11/F12、`↑`/`↓` 等）SHALL 各有独立描述，不共用「A / B」合并文案。状态栏 chip 标签 SHALL 描述该 chip 的动作：`j/k` 为「下/上一首」、全屏 `↑/↓` 为「浏览歌词」、`< >` 为快退/进、`[ ]` 为歌词偏移，两组不得同标签。

### D7. 每页矩阵

| Scope | T1 槽 2 / 3 / 4 / 5 | T1 槽 6 | T2 footer（顺序） |
|---|---|---|---|
| `local.tracks` | `Enter`/`Space` · `j/k` · `n/p` · `/` | `q` | — |
| `local.groups` | `Enter` · `j/k` · `n/p` · `1/2/3` | `q` | — |
| `playlist.list` | `Enter` · `j/k` · `n/p` · `N` | `q` | — |
| `playlist.tracks` | `Enter`/`Space` · `j/k` · `n/p` · `x` | `Esc` | — |
| `fullscreen` | `Space` · `↑/↓` · `< >` · `[ ]` | `Esc` | — |
| `search.inline` | 无 T1 | — | `Enter 播放 / Esc 清除` |
| `addtracks.inline` | 无 T1 | — | `Enter 添加 / Esc 关闭` |
| `overlay.queue` | 无 T1 | — | `x 移除 / C 清空 / Esc 关闭` |
| `overlay.add.list` | 无 T1 | — | `Enter 加入 / N 新建 / Esc 取消` |
| `overlay.add.create` | 无 T1 | — | `Enter 保存 / Esc 取消` |
| `overlay.delete` | 无 T1 | — | `Enter 确认 / Esc 取消` |
| `playlist.rename` | 无 T1 | — | `Enter 保存 / Esc 取消` |
| `overlay.settings.browse` | 无 T1 | — | `Enter 编辑 / Ctrl+R 重扫 / Esc 关闭` |
| `overlay.settings.edit` | 无 T1 | — | `Enter 保存 / Esc 取消` |
| `overlay.help` | 无 T1 | — | `↑/↓ 滚动 / Esc 或 ? 关闭` |
| `overlay.info` | 无 T1 | — | `i 或 Esc 关闭 / ↑/↓ 滚动` |

`Enabled` 谓词控制三项：`playlist.undo` 只在撤销窗口内出现；`playback.right_panel` 只在 `showMiniPlayer()` 为真时出现；`queue.open` 始终出现。当前 `keys_tracklist_shared.go:445-447` 只在队列非空时追加 `Q` hint 但绑定始终可用，属于提示与可用性脱钩，须改为由 `Enabled` 决定。

折叠右栏后，曲目视图、分组视图（Albums / Artists）与歌单列表 SHALL 都在头部右侧显示同一份压缩播放状态；分组视图不得缺失。

### D8. 文案与键名归一

i18n 正文不再内嵌键名。`TrackListContext` 增加 `EmptyHints []ActionID`，空态提示行由注册表生成 chip。`i18n_test.go` 增加断言：文案值不含 `❮`、`press <key>` 或 `按 <单键>` 模式。需改动的既有文案：`queue_empty`（EN/ZH 的 `e 或 z`）、`action_settings`、`action_quit`、`playlist_tracks_empty`、`search_clear_hint`、`press_enter_to_play` 中含键名的部分；保留 `Enter`、`Esc`、`Ctrl+R` 等稳定键名的做法不采用，统一改为动作描述加 chip。

键名显示统一：`Space`（不再有 `Spc`）、`PgUp / PgDn`、`j / ↓`、`Ctrl+U / Ctrl+D`。`8-bit` 硬编码改 `hint_chip` 并补中英。8 个未使用的 i18n key 清理或接线：`help_close` 文案「Any key to close」与实际 `?` / `Esc` 矛盾，删除；启用 `help_close_key`；`action_reload_library` 补进帮助浮层全局区；其余按注册表接线或删除。

替代方案：保留键名内嵌文案，加一致性测试。否决——注册表改键仍需回改文案，单源不彻底。

### D9. 规格与文档生成

五份 delta spec 的场景用 ActionID 描述触发条件。新增 `keybinding-system` spec 的「最终键位映射」需求固定物理键。一致性测试断言：spec 中出现的每个 ActionID 都存在于注册表。README 生成块由 `go:generate` 回填；生成块之外的散文段（`:17-24`、`:164`、`:311`、`:356`、`:378`）人工按最终键表改写。`ARCHITECTURE.md:71` 从「新增浮层须同步两条链」改为「注册表自动生成渲染与提示，新增浮层只需声明 scope 与绑定」。`CONTRIBUTING.md:33` 补 `go generate ./...` 说明。

### D10. 注释同步清单

无法自动生成，逐条人工核对：`render_statusbar.go:27-40,112-113,133-134,159,177-178`；`keys_tracklist_shared.go:17-26,327,348,419,430`；`keys.go:23-24,99-119,141,150,173,191`；`keys_search.go:42-43,59`；`keys_fullscreen.go:111`；`model.go:58`；`tracklist_context.go:49,59-60,75`；`app.go:154`；`update.go:76`；`groups.go:180-181`；`render_tab_playlist.go:153`；`render_tracklist_shared.go:40,89`；`render_overlays.go` 中描述键位的注释。所有列出键位序列的注释改为引用 ActionID 或直接删除。

### D11. Golden 快照

T1 从 13 chip 降到 6 chip，帮助浮层分区变化，footer chip 变化。`golden_test.go` 的 10 个 case（`normal`、`search`、`album-groups`、`fullscreen`、`fullscreen-plain-lyrics`、`help`、`settings`、`queue`、`playlist-list`、`add-to-playlist`）全部受影响，10 份 `testdata/golden/*.golden` 重生成，并补一条断言：帮助浮层快照覆盖全部分区行，不再只覆盖前 19 个 chip。

### D12. 测试与验证策略

- 分发顺序测试：全屏 `?` 打开帮助并可关闭；浮层内 `q` 无动作；`4` 打开队列。
- 输入透传测试：`q`、`j`、`k`、F 键在命名框与 add-tracks 搜索框可输入；`Ctrl+R` 在 `overlay.settings.edit` 仍触发保存并重扫。
- 注册表测试：同 scope 无重复键、无禁用组合、中英描述齐全、spec ActionID 全部存在、README 生成块一致、渲染文件无硬编码键 chip。
- 显示测试：六槽顺序与 pinned 丢弃、帮助浮层五区顺序、footer `Esc` 最后、两次渲染顺序一致。
- 回归：`phase4_test.go:86-129` 的浮层新建断言改为 `N`；其余按键测试键位不变。
- 每阶段门禁：`CGO_ENABLED=1 go build ./... && CGO_ENABLED=1 go test ./... && CGO_ENABLED=1 go vet ./...`。大型改动追加 `go test -race ./...`（`internal/audio` 的 oto 初始化竞态属第三方既有问题，需隔离或说明）、真机 pty 冒烟（启动、导航、退出无 panic）与一次独立 review。最后 `openspec validate redesign-shortcut-system --strict`。

### D13. 阶段划分

1. 注册表 + 上下文栈 + 最终键表一次落地，同时修全屏 `?` 陷阱、`,` 死键、输入劫持；本组交付注册表单元测试与分发顺序测试。
2. T1/T2/T3 从注册表渲染，切六槽 pinned 丢弃顺序；同步 README 生成块、i18n、注释、帮助浮层、footer chip、10 份 golden；本组交付显示测试与文档一致性测试。
3. 一致性测试补齐、真机 pty 冒烟、独立 review、`openspec validate --strict`。

每阶段结束跑门禁并提交，再进入下一阶段。无 alias 迁移期，不需要「零行为变化」阶段。

### D14. 无兼容处理

不设 alias 字段、不设兼容开关、不写废弃说明、不做设置迁移。旧键按下静默无效果。因 `settings_keys.go` 中无键位持久化项，无 DB 迁移；`KeyLastActiveTab` 与 `tabID` 只追加规则不变。

## Risks / Trade-offs

- [旧键静默失效，任何漏改的文档或注释都会直接误导用户] → 生成块 + 源码扫描测试 + 注释清单逐条核对 + 真机冒烟。
- [`1/2/3/4` 占用数字键，无法再支持 ncspot 式 `10j` 计数前缀] → 计数前缀列为 Non-Goal；音乐播放器高频操作不需要它。
- [浮层模态后浮层内无法用 `n` / `p` 切歌] → 保留媒体 F 键与 `global.reload`，`Esc` 可退；代价可接受。
- [`,` 在主视图与 fullscreen 含义不同] → 注册表按 scope 绑定，提示只显示当前 scope 的绑定。
- [`N`、`E`、`F2`、`J/K`、`C`、`G` 需要 Shift 或功能键，非 US 布局成本更高] → 仅低频动作使用；主操作、导航、播放保持单键；`[`/`]` 提供 `,`/`.` 等效键。
- [legacy 终端无法区分 `Ctrl+I` / `Ctrl+M` / `Ctrl+[` / `Ctrl+Shift+字母`] → 禁用组合清单写进注册表启动校验；不使用 `Alt`。
- [注册表成为单点，写错一处全应用按键失效] → 启动校验重复键、禁用组合与文案缺失；一致性测试覆盖分发、提示与文档。
- [i18n 正文移除键名需要新增 `EmptyHints` 字段] → 只加一个字段，空态渲染复用现有 chip 渲染函数。
- [生成脚本可能覆盖 README 手写段落] → 只替换标记块内内容，标记外段落人工改写并加测试断言。
- [帮助浮层五区可能变长] → `renderScrollableOverlay` 已有滚动能力，footer 保留 `↑/↓` 提示。

## Migration Plan

1. 无 DB 迁移、无数据转换、无兼容开关。`settings_keys.go` 无需改动。
2. 阶段 1 在 worktree 内完成并通过门禁后提交；阶段 2、3 依序提交。回滚以阶段为单位 `git revert`，不涉及数据修复。
3. 交付前真机 pty 冒烟：启动、切 Tab、`1/2/3` 切层级、`4` 开队列、`N` 新建、`F2` 重命名、`E`/`e` 入队、`?` 帮助、退出无 panic。
4. 实现完成并通过 `openspec validate redesign-shortcut-system --strict` 后走 OpenSpec archive；archive / sync 阶段把 6 份 delta spec 合并进 `openspec/specs/`，其中 `keybinding-system` 会新建 main spec。
