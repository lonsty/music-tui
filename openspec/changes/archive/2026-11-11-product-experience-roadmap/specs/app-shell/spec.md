# Spec Delta

## Purpose

定义应用外壳的稳定行为：顶层导航只暴露可用功能，启动时自动同步曲库，终端输入与鼠标策略、状态反馈、本地化以及设置面板的可见性保持一致。

## ADDED Requirements

### Requirement: 顶层导航仅包含可用功能

系统 SHALL 仅提供已实现的顶层 Tab（Library、Playlists），SHALL NOT 展示占位或“即将推出”页面。

#### Scenario: 循环切换 Tab

- **WHEN** 用户在任意主视图按 Tab
- **THEN** 焦点只在 Library 与 Playlists 之间循环
- **AND** 不出现 Online 或占位页面

### Requirement: 启动自动同步曲库

应用 SHALL 在启动后于后台对当前音乐目录执行增量同步，期间界面保持可交互并显示扫描状态。

#### Scenario: 正常启动

- **WHEN** 应用启动且已配置音乐目录
- **THEN** 后台开始增量同步
- **AND** 状态栏显示扫描中，界面仍可导航与播放

#### Scenario: 同步失败

- **WHEN** 增量同步返回错误
- **THEN** 状态栏显示本地化错误
- **AND** 保持上一次成功加载的曲库可用

### Requirement: 全局重新扫描

在非文本输入状态下按 Ctrl+R SHALL 立即触发对当前音乐目录的增量同步。

#### Scenario: 主视图重新扫描

- **WHEN** 用户在曲库或歌单主视图按 Ctrl+R
- **THEN** 开始同步并显示扫描状态
- **AND** 完成后报告当前曲目数量

### Requirement: 不捕获鼠标

应用 SHALL NOT 启用终端鼠标事件捕获。

#### Scenario: 选择文本

- **WHEN** 用户在界面中拖选文本
- **THEN** 终端执行原生文本选择，而不是把事件交给应用

### Requirement: 音量显示语义

扬声器图标旁的百分比 SHALL 以 1.0 音量为 100%；高于 1.0 时 SHALL 仍显示 100% 并附带增益标记，而不是显示超过 100 的百分比。

#### Scenario: 默认音量

- **WHEN** 音量值为 1.0
- **THEN** 控件显示 100%

#### Scenario: 增益音量

- **WHEN** 音量值为 1.5
- **THEN** 控件显示 100% 并带增益标记

### Requirement: 界面文案本地化

所有用户可见的状态、错误、确认与提示文案 SHALL 来自当前语言资源，并在切换语言后立即生效。

#### Scenario: 中文界面错误

- **WHEN** 界面语言为中文且发生同步失败
- **THEN** 用户看到中文错误提示，其中不含英文句子

### Requirement: 删除歌单确认一致

删除歌单确认浮层展示的按键提示 SHALL 与实际行为一致：Enter 确认删除，Esc 取消。

#### Scenario: 取消删除

- **WHEN** 确认浮层显示且用户按 Esc
- **THEN** 歌单保留且浮层关闭

#### Scenario: 确认删除

- **WHEN** 用户按 Enter
- **THEN** 歌单及其关联被删除
