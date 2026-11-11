# Spec Delta

## Purpose

定义临时播放队列的入队、消费顺序、面板操作与生命周期，让用户在不破坏当前播放列表的情况下临时插入想听的曲目。

## ADDED Requirements

### Requirement: 入队操作

选中曲目时，`e` SHALL 将其插入当前曲目之后；`z` SHALL 追加到队尾；已存在于队列中的曲目再次入队 SHALL 被忽略。

#### Scenario: 下一首播放

- **WHEN** 用户选中曲目按 `e`
- **THEN** 该曲目成为队列首项
- **AND** 下一次切歌时首先播放它

#### Scenario: 重复入队

- **WHEN** 曲目已在队列中且用户再次按 `e` 或 `z`
- **THEN** 队列保持不变并提示已在队列中

### Requirement: 队列消费顺序

自然播完或按 `n` 时，若队列非空，SHALL 先播放队首并从队列移除；队列为空时 SHALL 按当前播放模式继续。

#### Scenario: 队列优先

- **WHEN** 当前曲目结束且队列非空
- **THEN** 播放队首曲目
- **AND** 该曲目从队列移除

#### Scenario: 队列耗尽

- **WHEN** 当前曲目结束且队列为空
- **THEN** 按当前播放模式选择下一首

### Requirement: 队列面板

`Q` SHALL 打开队列面板，显示顺序与当前位置；支持移除选中项与清空；Esc 关闭。

#### Scenario: 移除队列项

- **WHEN** 队列面板打开且用户按 `x`
- **THEN** 选中项从队列移除，光标保持有效

#### Scenario: 清空队列

- **WHEN** 用户按 `C`
- **THEN** 队列清空，当前播放不受影响

### Requirement: 队列生命周期

队列 SHALL 在切歌、切换 Tab、打开浮层时保留；SHALL NOT 写入数据库，退出应用后清空；手动从曲库选曲 SHALL NOT 清空队列。

#### Scenario: 跨曲目保留

- **WHEN** 队列非空时用户手动播放另一首曲目
- **THEN** 队列内容保持不变

#### Scenario: 退出不持久化

- **WHEN** 用户退出并重新启动应用
- **THEN** 队列为空
