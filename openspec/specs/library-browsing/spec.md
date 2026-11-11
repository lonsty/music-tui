# library-browsing Specification

## Purpose
定义本地曲库的浏览层级、搜索语义、列表导航与渲染约束，使用户能在大型曲库中快速定位并开始播放。搜索始终作用于曲目，分组视图只改变浏览维度。

## Requirements

### Requirement: 曲库浏览层级

本地曲库 SHALL 提供 Tracks、Albums、Artists 三种浏览层级，并允许在层级间下钻与返回。

#### Scenario: 从专辑进入曲目

- **WHEN** 用户在 Albums 层级选中专辑并按 Enter
- **THEN** 显示该专辑的曲目，按曲目编号排序
- **AND** 按 Esc 返回 Albums 并保持原光标位置

#### Scenario: 切换浏览层级

- **WHEN** 用户在曲库视图切换浏览模式
- **THEN** 列表立即按新模式重建，光标置于首项

### Requirement: 搜索语义统一

搜索 SHALL 大小写不敏感，默认匹配标题、艺术家、专辑；SHALL 支持 `s:`、`a:`、`t:`、`f:` 前缀限定字段；本地库与“添加到歌单”面板 SHALL 使用相同语义。

#### Scenario: 默认搜索

- **WHEN** 用户输入 `blue`
- **THEN** 标题、艺术家或专辑包含 `blue` 的曲目都会出现

#### Scenario: 前缀搜索

- **WHEN** 用户输入 `a:kind`
- **THEN** 仅专辑名包含 `kind` 的曲目出现

#### Scenario: 添加面板同语义

- **WHEN** 用户在添加曲目面板输入相同查询
- **THEN** 结果集合与本地库搜索一致

### Requirement: 过滤状态可见

搜索输入关闭后，生效的过滤条件 SHALL 在列表头部显示查询词与清除提示，直到被清除。

#### Scenario: 播放后保留过滤

- **WHEN** 用户在搜索结果中按 Enter 播放并关闭输入框
- **THEN** 列表仍只显示匹配结果
- **AND** 头部显示查询词与“Esc 清除”

#### Scenario: 清除过滤

- **WHEN** 用户按 Esc
- **THEN** 过滤清除并恢复完整列表
- **AND** 光标回到正在播放曲目（若存在）

### Requirement: 列表导航效率

曲目列表 SHALL 支持跳到正在播放曲目、按视口翻页、跳到首尾。

#### Scenario: 跳到正在播放

- **WHEN** 当前有播放曲目且用户按 `c`
- **THEN** 光标定位到该曲目并进入可视区域

#### Scenario: 翻页

- **WHEN** 用户按 PageDown 或 Ctrl+D
- **THEN** 光标下移一个视口高度且选中行保持可见

#### Scenario: 无播放曲目

- **WHEN** 没有正在播放的曲目且用户按 `c`
- **THEN** 状态不变并给出提示

### Requirement: 渲染不查询数据库

列表与浮层的单帧渲染 SHALL NOT 发起数据库查询；所有计数与元数据 SHALL 在消息处理阶段预先加载。

#### Scenario: 浏览歌单列表

- **WHEN** 用户在歌单列表连续移动光标
- **THEN** 渲染过程不产生数据库查询
- **AND** 每行计数保持正确

### Requirement: 空结果提示

搜索无结果时 SHALL 显示包含当前查询词的空状态和清除提示。

#### Scenario: 无匹配

- **WHEN** 查询无匹配曲目
- **THEN** 面板显示无匹配结果及查询词
- **AND** 提示按 Esc 清除
