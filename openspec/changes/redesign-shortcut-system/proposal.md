# Proposal

## Why

现有快捷键以 120 余处手写 `switch msg.String()` 分散在 11 个文件中，键位提示又分布在状态栏、浮层 footer、帮助浮层、README、i18n 表与 golden 快照等处。实现、文案与规格靠人工同步，已经产生全屏帮助浮层无法关闭、`,` 设置键死键、输入框的 `q` / `j` / `k` 被劫持、帮助浮层列出当前页面并不存在的键等缺陷。工具尚未正式发布，此时可以一次性建立集中式键位注册表与上下文栈，把键位提示改为单一来源生成，无需向下兼容。

## What Changes

- **BREAKING** 新增 `keybinding-system` capability：集中式键位注册表、上下文栈解析、冲突检测，以及状态栏 / footer / 帮助浮层 / 文档四层提示的生成与顺序规则。
- **BREAKING** 重排键位：`v` 循环切层级改为 `1` / `2` / `3` 直达；`Q` 队列改为 `4`；`e` / `z` 入队改为 `e` 队尾 / `E` 插到下一首；歌单列表与「加入歌单」浮层的新建改为 `N`；歌单重命名改为 `F2`；`n` 全局只做下一曲；`L` 统一作用于选中曲目。
- **BREAKING** 旧键 `v`、`Q`、`z` 及旧语义 `r` 重命名、`n` 新建直接解绑。不留 alias、不做兼容、不写废弃说明；按下静默无效果，且不出现在任何提示中。
- **BREAKING** 全屏 `,` 设置死键移除，改为 `,` / `.` 作为歌词偏移等效键；设置面板只在主视图打开。
- 文本输入 scope 不再接收任何单键动作或媒体 F 键；浮层为模态，只认自身绑定与 `Esc`、`?`、`Ctrl+R`、媒体 F 键。
- 状态栏最多 6 个 chip，固定六槽与 pinned 丢弃顺序；浮层 footer 2-4 个 chip；帮助浮层改为五区、当前上下文优先；四层提示全部由注册表生成。
- 五份 spec 中的键位字面量改为稳定的 ActionID 引用，物理键只在注册表定义；`README.md`、`i18n.go`、代码注释、帮助浮层、footer chip、10 份 golden 快照与相关测试同步更新；键名不再内嵌 i18n 正文。
- 不新增依赖，复用现有 `github.com/charmbracelet/bubbles v1.0.0` 的 `key.Binding` / `key.Matches` / `help.KeyMap`；不改数据模型，无 DB 迁移。

## Capabilities

### New Capabilities

- `keybinding-system`: 键位注册表的单一来源、上下文栈解析与优先级、同 scope 冲突检测与终端禁用组合校验，以及四层键位提示的生成、固定分组顺序与窄终端丢弃规则。

### Modified Capabilities

- `app-shell`: 全局重新扫描在除搜索框与命名框外的所有 scope 可用；界面文案本地化扩展到所有键位 chip 与描述。
- `library-browsing`: 浏览层级切换由循环键改为直达键；过滤清除与空结果提示改由注册表生成。
- `lyrics-playback`: 歌词偏移增加 `,` / `.` 等效键，且只在同步歌词生效。
- `playback-queue`: 入队键改为 `e` / `E`，队列面板键改为 `4`。
- `playlist-management`: 从选择浮层新建歌单键改为 `N`；新增 `F2` 重命名；`L` 统一作用于选中曲目。

## Impact

- 代码：`internal/tui/keys*.go`、`groups.go`、`queue.go`、`context_*.go`、`update.go`、`render_statusbar.go`、`render_overlays.go`、`render_tracklist_shared.go`、`i18n.go`、`tracklist_context.go`，新增键位注册表与生成脚本。
- 文档：`README.md`（含生成块）、`ARCHITECTURE.md`、`CONTRIBUTING.md`、代码注释。
- 测试：`phase4_test.go` 的浮层新建断言改为 `N`；新增键位一致性测试；10 份 `testdata/golden/*.golden` 重生成。
- 规格：5 份 main spec 在 archive / sync 时更新；本 change 只写 delta spec。
- 依赖与数据：无新增依赖；`settings_keys.go` 中无键位持久化项，无 DB 迁移；`KeyLastActiveTab` 与 `tabID` 只追加规则不变。
