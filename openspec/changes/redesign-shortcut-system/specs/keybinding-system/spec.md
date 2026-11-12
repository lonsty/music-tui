# Spec Delta

## Purpose

定义集中式键位注册表、上下文栈解析与四层键位提示的稳定行为，使物理键只在注册表定义一次，所有页面显示的键位、顺序与文案都由同一来源生成，避免实现、文档与规格之间发生漂移。

## ADDED Requirements

### Requirement: 键位注册表单一来源

物理键与 ActionID 的对应关系 SHALL 只在键位注册表中定义。按键分发、状态栏、浮层 footer、帮助浮层与 README SHALL 全部从注册表派生，SHALL NOT 各自硬编码键名。

#### Scenario: 改一处同步生效

- **WHEN** 注册表中某个 ActionID 的绑定键被修改并重新运行生成
- **THEN** 分发、状态栏、footer、帮助浮层与 README 同时显示新键
- **AND** 没有位置仍显示旧键

#### Scenario: 无硬编码键名

- **WHEN** 扫描渲染与提示代码
- **THEN** 不出现键名字符串字面量
- **AND** 所有 chip 由注册表生成

### Requirement: 最终键位映射

注册表 SHALL 把下列 ActionID 绑定到所列物理键：`library.view_tracks` / `library.view_albums` / `library.view_artists` → `1` / `2` / `3`；`queue.open` → `4`；`queue.append` → `e`；`queue.insert_next` → `E`；`playlist.new` → `N`；`playlist.rename` → `F2`；`playback.next` → `n`；`lyrics.offset_earlier` / `lyrics.offset_later` → `[` / `]` 与 `,` / `.`。

#### Scenario: 新键生效

- **WHEN** 用户在曲目列表按 `4`
- **THEN** 队列面板打开
- **AND** 按 `Q` 无任何效果

#### Scenario: 入队键区分

- **WHEN** 用户选中曲目分别按 `e` 与 `E`
- **THEN** `e` 追加到队尾
- **AND** `E` 插到下一首播放

#### Scenario: 新建键不与下一曲冲突

- **WHEN** 用户在歌单列表层按 `N`
- **THEN** 进入新建歌单流程
- **AND** 按 `n` 触发下一曲而不新建歌单

### Requirement: 上下文栈解析

按键 SHALL 按 `input > confirm > overlay > fullscreen > settings > group > tracklist > global` 优先级由栈顶向下解析，首个绑定该键的 scope SHALL 消费该键。文本输入 scope SHALL NOT 接收单键动作或媒体 F 键；浮层 SHALL 为模态。

#### Scenario: 全屏帮助可关闭

- **WHEN** 用户在全屏视图按 `?`
- **THEN** 帮助浮层显示在当前视图之上
- **AND** 按 `Esc` 只关闭帮助浮层，全屏视图保持

#### Scenario: 输入框字符不被劫持

- **WHEN** 用户在歌单命名框或添加曲目搜索框输入 `q`、`j`、`k`
- **THEN** 字符写入输入框
- **AND** 不触发退出或导航动作

#### Scenario: 浮层模态

- **WHEN** 队列浮层打开且用户按 `n`
- **THEN** 不触发下一曲
- **AND** 队列浮层保持打开

### Requirement: 冲突检测

应用启动 SHALL 校验注册表：同一 scope 内一个物理键 SHALL 只属于一个 ActionID；禁用组合 SHALL NOT 出现在注册表；每个 ActionID SHALL 有中英描述。禁用组合指 `Alt+*`、`Ctrl+Shift+字母`、`Ctrl+I` / `Ctrl+M` / `Ctrl+[` / `Ctrl+Space` / `Ctrl+S` / `Ctrl+Q` / `Ctrl+Z` / `Ctrl+C`。校验失败 SHALL 阻止启动并报告具体冲突。

#### Scenario: 重复键阻止启动

- **WHEN** 注册表把同一 scope 的 `e` 同时绑定到两个 ActionID
- **THEN** 应用启动失败
- **AND** 错误指出冲突的 scope、键与两个 ActionID

#### Scenario: 文案缺失阻止启动

- **WHEN** 某 ActionID 缺少中文或英文描述
- **THEN** 应用启动失败
- **AND** 错误指出缺失的 ActionID 与语言

### Requirement: 四层键位提示

状态栏 SHALL 最多显示 6 个 chip，按 `?`、主操作、导航、播放、上下文动作、退出六槽固定排列；窄终端 SHALL 按上下文动作、播放、导航的顺序丢弃，`?`、主操作与退出 SHALL 始终保留。浮层 footer SHALL 显示 2-4 个 chip 且 `Esc` 永远最后。帮助浮层 SHALL 分五区，当前上下文区置首。README SHALL 由注册表生成。

#### Scenario: 窄终端丢弃顺序

- **WHEN** 终端宽度不足以显示全部 6 个 chip
- **THEN** 先丢弃上下文动作 chip，再丢弃播放 chip，最后丢弃导航 chip
- **AND** `?`、主操作与退出 chip 保持可见

#### Scenario: 页面不显示未绑定键

- **WHEN** 用户在歌单列表层查看状态栏与帮助浮层
- **THEN** 只显示该 scope 已绑定的键
- **AND** 不显示 `Space`、`e`、`x` 等当前 scope 未绑定的键

#### Scenario: 帮助浮层分区

- **WHEN** 用户打开帮助浮层
- **THEN** 五个分区按当前上下文、全局、曲库、歌单、媒体键排列
- **AND** 当前上下文区显示当前 scope 的全部绑定

#### Scenario: README 生成块一致

- **WHEN** 运行键位文档生成命令
- **THEN** README 生成块与注册表内容一致
- **AND** 生成块之外的段落不残留旧键

### Requirement: 固定分组顺序

所有键位提示 SHALL 按「退出/取消 → 主操作 → 导航 → 播放 → 当前对象 → 视图/面板 → 系统」分组排列；组内按 `ShortRank` 降序、ActionID 升序排列。同一 scope 的两次渲染 SHALL 产生相同顺序。

#### Scenario: 顺序稳定

- **WHEN** 同一 scope 连续渲染两次键位提示
- **THEN** 两次 chip 顺序完全一致

#### Scenario: 共享键位置一致

- **WHEN** 用户在曲库与歌单曲目层之间切换
- **THEN** `?`、主操作与退出 chip 保持在相同槽位
- **AND** 只有上下文动作槽随 scope 变化

### Requirement: 无兼容别名

旧键 `v`、`Q`、`z` 及旧语义 `r` 重命名、`n` 新建 SHALL NOT 绑定到任何 ActionID；按下 SHALL 静默无效果，且 SHALL NOT 出现在任何提示中。SHALL NOT 提供兼容设置或迁移路径。

#### Scenario: 旧键无效果

- **WHEN** 用户在曲目列表按 `v` 或 `Q`
- **THEN** 浏览层级与队列面板均不变化
- **AND** 不显示任何错误或提示

#### Scenario: 无兼容设置项

- **WHEN** 查看设置面板与持久化配置
- **THEN** 不存在启用旧键或键位兼容的开关
- **AND** 不存在键位迁移逻辑
