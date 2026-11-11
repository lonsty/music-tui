# lyrics-playback Specification

## Purpose
定义歌词时间轴偏移与同步行为，让用户能校正与音频不完全对齐的歌词。偏移按曲目独立保存，纯文本歌词不参与偏移。

## Requirements

### Requirement: 歌词偏移调整

同步歌词下，`[` SHALL 将歌词提前 0.5 秒，`]` SHALL 延后 0.5 秒；范围限制在 ±30 秒；调整后当前高亮行立即更新。

#### Scenario: 延后

- **WHEN** 用户按 `]`
- **THEN** 歌词时间轴延后 500ms
- **AND** 当前高亮行按新偏移重新计算

#### Scenario: 边界

- **WHEN** 偏移已达 +30 秒再按 `]`
- **THEN** 偏移保持 +30 秒

### Requirement: 偏移持久化

每首曲目的偏移 SHALL 独立保存并在下次播放同一曲目时恢复。

#### Scenario: 恢复偏移

- **WHEN** 用户为曲目设置偏移后重启应用并播放同一曲目
- **THEN** 偏移值被恢复

#### Scenario: 不同曲目

- **WHEN** 用户切换到另一首曲目
- **THEN** 使用该曲目自己的偏移值

### Requirement: 浏览定位使用偏移

全屏歌词浏览中按 Enter 定位 SHALL 使用调整后的时间。

#### Scenario: 定位

- **WHEN** 偏移为 +2 秒且用户定位到某行按 Enter
- **THEN** 播放位置跳到该行时间加上偏移

### Requirement: 纯文本歌词忽略偏移

无时间戳的纯文本歌词 SHALL 忽略偏移按键，并保持进度驱动的滚动行为。

#### Scenario: 纯文本下按键

- **WHEN** 当前歌词无时间戳且用户按 `[` 或 `]`
- **THEN** 歌词显示不变且给出仅同步歌词支持偏移的提示
