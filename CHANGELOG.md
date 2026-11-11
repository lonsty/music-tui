# Changelog

本文件遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 格式，版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added

- 渲染 golden 测试与 10k 曲库基准，锁定行为并暴露热路径回归。
- `internal/library`、`internal/store` 的关键路径单测。
- MIT `LICENSE`、CI 工作流、`Makefile`、`CONTRIBUTING.md`。
- 歌单曲目排序索引与曲库排序表达式索引。
- 扫描取消与防重入。

### Changed

- 过滤、队列查找、歌词布局与渲染分配优化，输出保持不变。
- 曲库同步改为单事务 prepared upsert，分批提交。
- i18n 与图标集全局状态改为原子快照。
- 扫描与持久化错误不再静默丢弃。
- 超大文件按职责拆分，`App` 状态组拆出 `OverlayState`。

### Removed

- **BREAKING**：删除运行时未使用的 `internal/provider`、`internal/provider/local` 包，以及 `library.ScanDir`、`audio.Player.PlayAt`、`store.Store.DeleteTrack`、`TrackListContext.ExtraHints`。仓库以应用形式发布，不承诺外部导入兼容。

### Fixed

- 修复 `applyFormatPreference` 在 lossless-only / MP3-only 偏好下原地过滤、污染曲库顺序的问题。
- 修复 `tea.Cmd` 闭包直接写 `App` 字段导致的数据竞争。
- 修复 `internal/audio` 测试硬编码本机路径、无音频设备时 panic 的问题。
