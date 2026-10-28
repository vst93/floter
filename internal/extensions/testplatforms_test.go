package extensions

import "runtime"

// 这个辅助**不属于某个平台**：`custom_test.go` 是 `//go:build !windows` 的
// （它的夹具是一个 shell 脚本），但无标签的 `commandword_test.go` 用它构造
// `CustomRequest`（一个连接至少要声明一个平台）。放在 gated 文件里，整个测试包
// 在 Windows 上就编译不过（`GOOS=windows go vet ./...` 报
// `undefined: testPlatforms`），`.github/workflows/go.yml` 的 windows-latest
// 那一格于是永远跑不到测试。由 `internal/ci` 的
// `TestTestHelpersAreVisibleOnEveryPlatform` 守着。

// testPlatforms is a valid non-empty platform list on the machine the suite
// runs on: a connection must name at least one platform, and the host only
// loads an integration that supports this machine.
func testPlatforms() []string { return []string{runtime.GOOS} }
