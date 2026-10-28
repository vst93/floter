# P11-p 规格：测试套件在三个平台上都得能编译

## §0 意图（一句话）

`internal/apps` 与 `internal/extensions` 的测试包在 `GOOS=windows` 下**编译不过**——
两个测试辅助被关在 `//go:build !windows` 文件里、使用者却是无标签文件——于是
`.github/workflows/go.yml` 的 `windows-latest` 那一格 `go vet ./...` / `go test ./...`
必红，Windows 上的测试覆盖率是 0。本轮把「测试辅助属于整个包，不属于某个平台」钉住。

为什么现在做（证据，HEAD `ef6866b`）：

```
$ GOOS=windows go vet ./...
# floter/internal/apps
vet: internal/apps/icnsfallback_test.go:28:9: undefined: pngBytes
# floter/internal/extensions
vet: internal/extensions/commandword_test.go:52:57: undefined: testPlatforms
$ GOOS=darwin go vet ./...      # 干净
```

- `internal/apps/apps_test.go:1` 是 `//go:build !windows`；`icnsEntry`（:543）、
  `icnsContainer`（:550）、`pngBytes`（:572）、`writeBytes`（:582）都定义在里面。
- `internal/apps/icnsfallback_test.go:1` **没有** build tag，:28/:80/:98/:99/:121/:132
  都在用这三个辅助 → Windows 上 `pngBytes`/`icnsContainer`/`writeBytes` 不存在。
- `internal/extensions/custom_test.go:1` 是 `//go:build !windows`；`testPlatforms`
  定义在 :18（`runtime.GOOS`）。`internal/extensions/commandword_test.go:1` **没有**
  build tag，:52 用它构造 `CustomRequest` → Windows 上不存在。
- `.github/workflows/go.yml:20` 的 matrix 含 `windows-latest`，:45 `go vet ./...`、
  :51 `go test -count=1 ./...`：这两格在 Windows 上从来跑不到测试，只跑出编译错误。

上一轮（P11-o）「仍待」第 5 条就是这条，被标为「下一轮候选（低优先）」；
用户实测四条仍卡实机复测、合约 §6 全「已落地」→ 本轮落在**三平台一致性**。

## §1 逐条落地项

### 落地项 1：`internal/apps` 的 icns 测试夹具搬到无标签文件

- 位置：`internal/apps/apps_test.go:543-586`（`icnsEntry` 类型 + `icnsContainer`、
  `pngBytes`、`writeBytes` 三个辅助）。
- 现状：都在 `//go:build !windows` 文件里，而无标签的 `icnsfallback_test.go` 用它们。
- 应改成：新建 `internal/apps/testfixtures_test.go`（**无 build tag**），把这四段整体
  搬过去；`apps_test.go` 只删这四段、其余一字不动。搬完 `apps_test.go` 里
  `encoding/binary`、`image`、`image/png` 是否还有使用者由编译器判定（不留空 import）。

### 落地项 2：`internal/extensions` 的 `testPlatforms` 搬到无标签文件

- 位置：`internal/extensions/custom_test.go:15-18`（注释 + `testPlatforms`），
  `custom_test.go:10` 的 `runtime` import 只为它存在。
- 现状：定义在 `//go:build !windows` 文件里，无标签的 `commandword_test.go:52` 用。
- 应改成：新建 `internal/extensions/testplatforms_test.go`（**无 build tag**）放
  `testPlatforms`（注释一起搬）；`custom_test.go` 删掉它并去掉 `runtime` import。

### 落地项 3：pin —— 测试辅助不许只活在某个平台上

- 新增 `internal/ci/crossplatform_testfiles_test.go`：
  `TestTestHelpersAreVisibleOnEveryPlatform`。
- 做法（纯 `go/build` + `go/parser`，不跑工具链）：对 `internal/`、`cmd/` 下每个含
  `_test.go` 的目录，用 `build.Context{GOOS: goos, GOARCH: "amd64", CgoEnabled: false}`
  的 `MatchFile` 求出 linux/darwin/windows 三份「这个 GOOS 下会编译的文件集」；
  对每个 GOOS：
  1. 收集该 GOOS 下**所有**文件的顶层声明名（含非测试文件），得到 `available`；
  2. 收集**只在该 GOOS 下缺席**的文件里声明的名字，减去 `available` 与 Go 预声明名
     → `missing`；
  3. 扫该 GOOS 下会编译的 `_test.go` 的 `ast.Ident`，撞上 `missing` 就红，
     报「`file:line` 引用了只在 <另一平台> 上声明的 `name`」。
- 为什么这条 pin 现在必须存在：这一族的缺陷**本机 linux 门禁全绿**（gated 文件不参与
  编译，谁都没意见），只有换 GOOS 才看得见。回归时人工不会去跑 `GOOS=windows go vet`。

## §2 行为契约零改动红线

- **只动 `_test.go` 与新增 `internal/ci` 的 pin**：`cmd/` 与 `internal/` 的**产品文件
  零改动**（`git diff --stat` 里不许出现非 `_test.go` 的既有文件）。
- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**全部不碰**。
- 测试函数数量**只许涨**（新增 1 条 pin）；既有断言**零改动**；不加 `t.Skip`。
- 非视觉轮：不出图（无墨与面的变化）。

## §3 门槛命令 + 本轮 PASS 基线（实测，HEAD `ef6866b`）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 本轮起必须净（现状：2 个包编译错误）
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2273（基线，本轮只许涨）
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

实测基线：PASS **2273** / 38 行（36 ok + 2 no test files）/ SKIP **15** / FAIL **0** /
测试函数 **1844**（`grep -rhn '^func Test' internal/ cmd/ --include=*_test.go`）。

## §4 mutation 计划（≥2 条，实跑变红后逐字节还原）

1. 把 `pngBytes` 从 `testfixtures_test.go` 搬回 `apps_test.go`（恢复原状）→
   `GOOS=windows go vet ./internal/apps/` 应重新报 `undefined: pngBytes`，
   且新 pin `TestTestHelpersAreVisibleOnEveryPlatform` 变红（引用
   `icnsfallback_test.go:28`）。
2. 把 `testPlatforms` 搬回 `custom_test.go` → 同上，`GOOS=windows go vet
   ./internal/extensions/` 报 `undefined: testPlatforms`，pin 变红。
3. 给 `internal/apps/testfixtures_test.go` 加上 `//go:build !windows`（只挪标签、不挪
   函数）→ pin 必须红在同一个 `missing` 名字上，证明 pin 认的是「谁在哪个 GOOS 下缺席」，
   不是文件名。

每条都要：注入 → 看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑一次（避开缓存）。
