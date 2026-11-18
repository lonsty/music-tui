# lyrics-playback Specification

## Purpose
定义歌词时间轴偏移与同步行为，让用户能校正与音频不完全对齐的歌词。偏移按曲目独立保存，纯文本歌词不参与偏移。

## Requirements

### Requirement: 歌词偏移调整

同步歌词下，触发 `lyrics.offset_earlier` SHALL 将歌词提前 0.5 秒，触发 `lyrics.offset_later` SHALL 延后 0.5 秒；范围限制在 ±30 秒；调整后当前高亮行立即更新。全屏歌词视图中 `,` 与 `.` SHALL 分别为两者的等效键；列表视图 SHALL 使用 `[` 与 `]`。

#### Scenario: 延后

- **WHEN** 用户触发 `lyrics.offset_later`
- **THEN** 歌词时间轴延后 500ms
- **AND** 当前高亮行按新偏移重新计算

#### Scenario: 边界

- **WHEN** 偏移已达 +30 秒再触发 `lyrics.offset_later`
- **THEN** 偏移保持 +30 秒

#### Scenario: 全屏等效键

- **WHEN** 用户在全屏同步歌词视图按 `,` 或 `.`
- **THEN** 偏移分别按 `lyrics.offset_earlier` 与 `lyrics.offset_later` 调整
- **AND** 不打开设置面板

#### Scenario: 列表层等效键

- **WHEN** 用户在曲目列表按 `[` 或 `]`
- **THEN** 偏移分别按 `lyrics.offset_earlier` 与 `lyrics.offset_later` 调整
- **AND** 按 `,` 打开设置面板

### Requirement: 偏移持久化

每首曲目的偏移 SHALL 独立保存并在下次播放同一曲目时恢复。

#### Scenario: 恢复偏移

- **WHEN** 用户为曲目设置偏移后重启应用并播放同一曲目
- **THEN** 偏移值被恢复

#### Scenario: 不同曲目

- **WHEN** 用户切换到另一首曲目
- **THEN** 使用该曲目自己的偏移值

### Requirement: 浏览定位使用偏移

全屏歌词浏览中触发 `lyrics.seek_line` SHALL 使用调整后的时间定位。

#### Scenario: 定位

- **WHEN** 偏移为 +2 秒且用户定位到某行并触发 `lyrics.seek_line`
- **THEN** 播放位置跳到该行时间加上偏移

### Requirement: 纯文本歌词忽略偏移

无时间戳的纯文本歌词 SHALL 忽略所有偏移动作及其等效键，并保持进度驱动的滚动行为。

#### Scenario: 纯文本下按键

- **WHEN** 当前歌词无时间戳且用户触发偏移动作或按下等效键
- **THEN** 歌词显示不变且给出仅同步歌词支持偏移的提示
