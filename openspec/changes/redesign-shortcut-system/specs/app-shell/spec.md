# Spec Delta

## MODIFIED Requirements

### Requirement: 全局重新扫描

触发 `global.reload` SHALL 在除搜索框与命名输入框外的任意 scope 立即触发对当前音乐目录的增量同步；设置编辑态 SHALL 先保存目录再触发。该绑定 SHALL 来自键位注册表。

#### Scenario: 主视图重新扫描

- **WHEN** 用户在曲库或歌单主视图触发 `global.reload`
- **THEN** 开始同步并显示扫描状态
- **AND** 完成后报告当前曲目数量

#### Scenario: 浮层内重新扫描

- **WHEN** 用户在帮助、信息、队列或设置浮层触发 `global.reload`
- **THEN** 开始同步并显示扫描状态
- **AND** 浮层保持打开

#### Scenario: 设置编辑态重扫

- **WHEN** 用户在设置目录编辑态触发 `global.reload`
- **THEN** 目录先保存再开始同步

#### Scenario: 搜索输入不重扫

- **WHEN** 搜索框或歌单命名框处于输入态且用户按下 `global.reload` 的绑定键
- **THEN** 不触发同步
- **AND** 按键不产生应用动作

### Requirement: 界面文案本地化

所有用户可见的状态、错误、确认、提示文案与键位 chip SHALL 来自当前语言资源，并在切换语言后立即生效。键位名称 SHALL 来自键位注册表，SHALL NOT 内嵌于文案正文。

#### Scenario: 中文界面错误

- **WHEN** 界面语言为中文且发生同步失败
- **THEN** 用户看到中文错误提示，其中不含英文句子

#### Scenario: 中文界面键位提示

- **WHEN** 界面语言为中文
- **THEN** 状态栏、浮层 footer 与帮助浮层的键位描述均为中文
- **AND** 不出现未翻译的原始文案 key

#### Scenario: 切换语言立即生效

- **WHEN** 用户在设置面板切换语言
- **THEN** 所有键位描述立即按新语言显示
- **AND** 键名保持一致
