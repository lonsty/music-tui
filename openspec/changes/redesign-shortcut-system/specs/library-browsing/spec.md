# Spec Delta

## MODIFIED Requirements

### Requirement: 曲库浏览层级

本地曲库 SHALL 提供 Tracks、Albums、Artists 三种浏览层级，并允许在层级间下钻与返回；触发 `library.view_tracks`、`library.view_albums`、`library.view_artists` SHALL 分别直达对应层级。

#### Scenario: 从专辑进入曲目

- **WHEN** 用户在 Albums 层级选中专辑并触发 `library.drill_down`
- **THEN** 显示该专辑的曲目，按曲目编号排序
- **AND** 触发 `library.back` 返回 Albums 并保持原光标位置

#### Scenario: 切换浏览层级

- **WHEN** 用户在曲库视图触发 `library.view_albums`
- **THEN** 列表按 Albums 重建，光标置于首项

#### Scenario: 旧循环键无绑定

- **WHEN** 用户在曲库视图按 `v`
- **THEN** 浏览层级不变
- **AND** `v` 不出现在任何键位提示中

### Requirement: 过滤状态可见

搜索输入关闭后，生效的过滤条件 SHALL 在列表头部显示查询词与清除提示，直到被清除；清除提示 SHALL 由键位注册表生成。

#### Scenario: 播放后保留过滤

- **WHEN** 用户在搜索结果中触发 `playback.play` 并关闭输入框
- **THEN** 列表仍只显示匹配结果
- **AND** 头部显示查询词与 `library.clear_filter` 的键位提示

#### Scenario: 清除过滤

- **WHEN** 用户触发 `library.clear_filter`
- **THEN** 过滤清除并恢复完整列表
- **AND** 光标回到正在播放曲目（若存在）

### Requirement: 列表导航效率

曲目列表 SHALL 支持跳到正在播放曲目、按视口翻页、跳到首尾；这些动作 SHALL 由键位注册表解析。

#### Scenario: 跳到正在播放

- **WHEN** 当前有播放曲目且用户触发 `nav.jump_playing`
- **THEN** 光标定位到该曲目并进入可视区域

#### Scenario: 翻页

- **WHEN** 用户触发 `nav.page_down`
- **THEN** 光标下移一个视口高度且选中行保持可见

#### Scenario: 无播放曲目

- **WHEN** 没有正在播放的曲目且用户触发 `nav.jump_playing`
- **THEN** 状态不变并给出提示

### Requirement: 空结果提示

搜索无结果时 SHALL 显示包含当前查询词的空状态和清除提示；清除提示 SHALL 由键位注册表生成。

#### Scenario: 无匹配

- **WHEN** 查询无匹配曲目
- **THEN** 面板显示无匹配结果及查询词
- **AND** 提示 `library.clear_filter` 的绑定键
