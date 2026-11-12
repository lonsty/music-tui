# Spec Delta

## ADDED Requirements

### Requirement: 歌单重命名

触发 `playlist.rename` SHALL 让选中的歌单进入重命名输入态；旧重命名键 `r` SHALL NOT 触发重命名。

#### Scenario: 重命名歌单

- **WHEN** 用户在歌单列表层选中歌单并触发 `playlist.rename`
- **THEN** 进入重命名输入态且原名称可编辑
- **AND** 提交后列表显示新名称

#### Scenario: 旧键不再重命名

- **WHEN** 用户在歌单列表层按 `r`
- **THEN** 不进入重命名输入态
- **AND** 歌单名称保持不变

### Requirement: 喜欢操作目标一致

触发 `library.like` SHALL 作用于当前光标选中的曲目；SHALL NOT 因 scope 不同而改作用于正在播放的曲目。歌单列表层选中对象是歌单而非曲目，该 scope SHALL NOT 绑定 `library.like`。

#### Scenario: 歌单列表层不绑定喜欢动作

- **WHEN** 用户在歌单列表层按 `L`
- **THEN** 喜欢状态不变
- **AND** `L` 不出现在该页面的键位提示中

## MODIFIED Requirements

### Requirement: 歌单内顺序调整

歌单曲目层级中，触发 `playlist.move_down` / `playlist.move_up` SHALL 将选中曲目下移 / 上移一位并立即持久化；首项上移与末项下移 SHALL 无效果。

#### Scenario: 上移

- **WHEN** 用户在歌单曲目列表触发 `playlist.move_up`
- **THEN** 选中曲目与其前一项交换位置
- **AND** 重启应用后顺序保持

#### Scenario: 边界

- **WHEN** 选中首项并触发 `playlist.move_up`
- **THEN** 顺序不变且不报错

### Requirement: 撤销移除

触发 `playlist.remove_track` 移除曲目后，5 秒内触发 `playlist.undo` SHALL 将最近移除的曲目恢复到原位置。

#### Scenario: 撤销

- **WHEN** 用户移除一首曲目并在 5 秒内触发 `playlist.undo`
- **THEN** 该曲目回到原位置并显示恢复提示

#### Scenario: 超时

- **WHEN** 超过 5 秒后触发 `playlist.undo`
- **THEN** 不再恢复并提示撤销已过期

### Requirement: 从选择浮层新建歌单

「添加到歌单」浮层中触发 `playlist.new` SHALL 进入新建流程，创建后自动把当前曲目加入新歌单。

#### Scenario: 新建并加入

- **WHEN** 用户在浮层触发 `playlist.new` 并输入名称回车
- **THEN** 新歌单创建
- **AND** 当前曲目被加入该歌单
- **AND** 浮层关闭并显示成功提示
