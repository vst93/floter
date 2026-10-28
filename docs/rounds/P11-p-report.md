# P11-p 报告：测试套件在三个平台上都得能编译

## §0 方向

用户实测四条仍卡实机复测、合约 `docs/raycast-design.md` §6 全「已落地」、上一轮（P11-o）
「仍待」第 5 条明写「`GOOS=windows go vet ./internal/extensions/` 报 `undefined: testPlatforms`
……是下一轮候选（低优先）」→ 按用户定的「三平台一致性 → 基础功能 → 像素细节」，
本轮落在**三平台一致性**。规格 `docs/rounds/P11-p-spec.md`。

选它的理由不是「低优先」而是它的后果：`.github/workflows/go.yml:20` 的 matrix 含
`windows-latest`，:45 跑 `go vet ./...`、:51 跑 `go test -count=1 ./...`。`go vet` 也编译
`_test.go`，所以那两个包让**整个 windows 那一格在跑到第一条断言之前就失败**——Windows 上
执行过的测试数是 0，而 Linux 门禁永远看不见：gated 文件在它被 gate 掉的平台上根本不参与编译。

## §1 意图

一个测试辅助属于**它的包**，不属于它恰好方便所在的平台。两处把它关进了
`//go:build !windows`，使用者却是无标签文件：

1. `internal/apps/apps_test.go:1` 是 `//go:build !windows`（它测 `.desktop` 与 POSIX 路径），
   `icnsEntry`（:543）、`icnsContainer`（:550）、`pngBytes`（:572）、`writeBytes`（:582）
   都定义在里面；无标签的 `internal/apps/icnsfallback_test.go` 在 :28/:80/:98/:99/:121/:132
   用它们 → Windows 上 `undefined: pngBytes`。
2. `internal/extensions/custom_test.go:1` 是 `//go:build !windows`（它的夹具是 shell 脚本），
   `testPlatforms` 定义在 :18；无标签的 `internal/extensions/commandword_test.go:52` 用它
   → Windows 上 `undefined: testPlatforms`。

## §2 落地（实测行号，HEAD `ef6866b`）

- 新增 `internal/apps/testfixtures_test.go`（**无 build tag**）：把 `icnsEntry`、
  `icnsContainer`、`pngBytes`、`writeBytes` 整体搬过去，文件头注释写明为什么不能跟着
  `apps_test.go` 一起 gated，并指名守着它的 pin。`apps_test.go` 只删这四段 + 三个随之
  空掉的 import（`encoding/binary`、`image`、`image/png`），其余零改动。
- 新增 `internal/extensions/testplatforms_test.go`（**无 build tag**）：搬 `testPlatforms`
  与其注释；`custom_test.go` 删掉它并去掉只为它存在的 `runtime` import。
- 新增 `internal/ci/crossplatform_testfiles_test.go` 的
  `TestTestHelpersAreVisibleOnEveryPlatform`（pin）：对 `internal/`、`cmd/` 下每个含
  `_test.go` 的目录，用 `go/build.Context{GOOS, GOARCH: amd64, CgoEnabled: false}.MatchFile`
  求出 linux/darwin/windows 三份「这个平台会编译哪些文件」（文件名后缀与 `//go:build`
  两种约束都由它判），`go/parser` 收集每个文件声明的顶层名字，再扫该平台会编译的
  `_test.go` 里的标识符：撞上「三平台里有、这个平台没有」的名字就红，报
  `file:line: <名字> does not exist on <goos>`。
  两处精度处理：`boundNames` 扣掉文件自己绑定的名字（`:=`、`var`、参数、标签）与语法位置
  （选择子的字段、结构体字段、复合字面量的键、import 别名）——否则
  `startmenufilter_test.go:166` 的局部 `write := func(...)` 会被误读成 gated 的 `write` 辅助；
  名字按目录跨包名合并（`foo_test` 引用 `foo` 不算缺陷）。检查方向是「这个名字在本平台
  哪儿都没有」，正好等于编译错误本身。

## §3 V/B/S 台账

- **V = 0**：没有旧视觉数值。
- **B = 0**：既有断言零改动。本轮只搬测试夹具、加一条 pin；产品文件（`cmd/` 与
  `internal/` 下的非 `_test.go`）**一个字节都没动**——`git diff --stat` 只有
  `internal/apps/apps_test.go` 与 `internal/extensions/custom_test.go` 两处删除。
- **S = 1**：测试夹具的归属结构变了（从平台文件 → 包文件），无既有断言被改。

## §4 门槛（实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=darwin  go vet ./...                              # 净
GOOS=windows go vet ./...                              # 净（本轮之前：2 个包编译错误）
go test -count=1 ./...                                 # exit 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2274（基线 2273）
GOOS=linux|windows|darwin go build ./...               # 三个都过
```

PASS **2274**（基线 2273）· 38 行（36 ok + 2 no test files）· SKIP **15**（不变）·
FAIL **0** · 测试函数 **1845**（基线 1844，`grep -rhn '^func Test' internal/ cmd/ --include=*_test.go`）。
非视觉轮：不出图（墨与面零变化）。

## §5 mutation（3 条实跑变红 + `md5sum -c` 逐字节还原 + `touch` 复跑）

md5 基线：
```
17248778420747228fe02d8b89308269  internal/apps/testfixtures_test.go
2f1dda7a416e2563a22268861e949dfa  internal/extensions/testplatforms_test.go
9f09b9ff509d9b8e838bc894794fca9d  internal/ci/crossplatform_testfiles_test.go
219a24b5390c73ab2674a4ea570f2150  internal/apps/apps_test.go
bf8e64a78f324bb6e7cac705dacff4cf  internal/extensions/custom_test.go
```

1. **给两个新夹具文件加回 `//go:build !windows`**（只挪标签、不挪函数）→
   ```
   vet: internal/apps/icnsfallback_test.go:28:9: undefined: pngBytes
   vet: internal/extensions/commandword_test.go:52:57: undefined: testPlatforms
   --- FAIL: TestTestHelpersAreVisibleOnEveryPlatform
       ../../internal/apps/icnsfallback_test.go:28:9: pngBytes does not exist on windows — it is declared only in a file windows does not compile (helper belongs to the package, not to one platform)
       ../../internal/apps/icnsfallback_test.go:67:22: icnsContainer does not exist on windows — …
       ../../internal/extensions/commandword_test.go:52:57: testPlatforms does not exist on windows — …
   ```
2. **只把 `pngBytes` 一个函数搬回 `apps_test.go`**（文件名不变、标签不动，只挪声明）→
   ```
   vet: internal/apps/icnsfallback_test.go:28:9: undefined: pngBytes
   --- FAIL: TestTestHelpersAreVisibleOnEveryPlatform
       ../../internal/apps/icnsfallback_test.go:28:9: pngBytes does not exist on windows — …
   ```
   这一条证明 pin 认的是「谁在哪个平台缺席」，不是「哪个文件名带标签」。
3. **还原后 `touch` 复跑**（避开测试缓存）：`internal/apps` ok / `internal/ci` ok /
   `internal/extensions` ok，`md5sum -c` 五行全「成功」。

## §6 仍待

1. 用户实机复测四条，连续第 71 轮。
2. 需拍板清单不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器
   `loadLocked` + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS 快捷键词序
   + `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`）。
3. **本轮只保证「Windows 上测试包能编译」**，不保证 Windows 上测试**跑得过**：本机是 Linux，
   没有 windows runner。真正跑一遍要看 CI 的 windows-latest 那一格（或
   `workflow_dispatch` 的 `windows-smoke`）。若那格红了，红的是运行时断言，不是编译。
4. 测试编译这一族已收口：`GOOS=linux|darwin|windows go vet ./...` 三份都净，
   有 `TestTestHelpersAreVisibleOnEveryPlatform` 守着。**别在里面再找小修。**
5. 下一处候选：`internal/inputtext`（205 行 / 7 条）的编辑行为三平台是否同一套
   （上一轮与本轮都点过名，尚未动手）。
