# Spec Delta

## MODIFIED Requirements

### Requirement: 入队操作

选中曲目时，触发 `queue.insert_next` SHALL 将其插入当前曲目之后；触发 `queue.append` SHALL 追加到队尾；已存在于队列中的曲目再次入队 SHALL 被忽略。

#### Scenario: 下一首播放

- **WHEN** 用户选中曲目并触发 `queue.insert_next`
- **THEN** 该曲目成为队列首项
- **AND** 下一次切歌时首先播放它

#### Scenario: 重复入队

- **WHEN** 曲目已在队列中且用户再次触发 `queue.insert_next` 或 `queue.append`
- **THEN** 队列保持不变并提示已在队列中

### Requirement: 队列消费顺序

自然播完或触发 `playback.next` 时，若队列非空，SHALL 先播放队首并从队列移除；队列为空时 SHALL 按当前播放模式继续。

#### Scenario: 队列优先

- **WHEN** 当前曲目结束且队列非空
- **THEN** 播放队首曲目
- **AND** 该曲目从队列移除

#### Scenario: 队列耗尽

- **WHEN** 当前曲目结束且队列为空
- **THEN** 按当前播放模式选择下一首

### Requirement: 队列面板

触发 `queue.open` SHALL 打开队列面板，显示顺序与当前位置；支持移除选中项与清空；触发 `queue.close` SHALL 关闭面板。

#### Scenario: 移除队列项

- **WHEN** 队列面板打开且用户触发 `queue.remove`
- **THEN** 选中项从队列移除，光标保持有效

#### Scenario: 清空队列

- **WHEN** 用户触发 `queue.clear`
- **THEN** 队列清空，当前播放不受影响

#### Scenario: 关闭面板

- **WHEN** 用户触发 `queue.close`
- **THEN** 面板关闭且队列内容保持不变
