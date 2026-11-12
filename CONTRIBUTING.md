# 贡献指南

感谢参与 music-tui。本文只列提交前必须满足的条件与约定；功能细节见 `ARCHITECTURE.md` 与 `openspec/specs/`。

## 环境

- Go 1.24.2（`go.mod` 要求）。系统 Go 较旧时设置 `GOTOOLCHAIN=auto`，或直接使用 `make`（已注入）。
- CGO 必须启用：SQLite 驱动与音频后端依赖它。
- Linux 需要 ALSA 开发头文件：`sudo apt-get install -y libasound2-dev`。
- 8-bit 芯片模式需要可选的 `p2chip` 外部命令；开发其余功能无需安装。

## 提交前门禁

```bash
make fmt      # gofmt -l 必须无输出
make build
make vet
make test
```

网络与音频相关改动额外运行：

```bash
go test -race $(go list ./... | grep -v internal/audio)
```

`internal/audio` 的 oto 初始化竞态是上游已知问题，`-race` 排除该包。真实播放测试需要设置 `MUSIC_TUI_TEST_MP3` 指向一个本地音频文件；未设置或文件不存在时测试会跳过。

## 代码约定

- 渲染路径不得查询数据库；计数与元数据在处理消息时预加载。
- 修改 `filteredIdxs` 必须同步重建 `posByID`（以及新增的 ID 索引）。
- 所有用户可见文案必须同时写入 `internal/tui/i18n.go` 的中英文表；`TestTranslationKeysCoverage` 会检查缺失与空翻译。
- 物理键只在 `internal/tui/keymap.go` 注册表定义；改键后运行 `go generate ./...` 回填 `README.md` 的生成块，并重生成 `internal/tui/testdata/golden/*.golden`。
- 后台 `tea.Cmd` 不直接写 `App` 状态；结果通过消息回到 `Update`。
- 新增 Tab 只能追加 `tabID`，不能重排持久化数值。
- 新增音频格式需同时更新 `library/formats.go` 与 `audio/source.go`。

## 分支与提交

- 只向 feature 分支提交与推送；不直接推 `main` / `master`，不 force-push。
- 提交信息使用 `type: summary`（如 `fix:`, `perf:`, `docs:`），一行说清意图。
- 行为变更需要先有 OpenSpec change；纯修复可直接提交并补测试。

## 评审

- 每个 PR 必须包含验证命令与输出摘要。
- 结构性重构与行为变更分开提交，便于回滚。
