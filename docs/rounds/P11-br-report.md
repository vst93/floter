# P11-br 报告 · 升级 mygo v0.7.0 → v0.7.3（七条 ui 行为变化逐条定性，两条有路径的给出护栏）

规格 `docs/rounds/P11-br-spec.md`。基线 HEAD `6afcc38`（P11-bq），v0.7.0 实测
**PASS 2460 / SKIP 54 / 函数 2014 / 42 包**。

## 1. 为什么是这一件

选题顺序前三档全堵，逐条写清（这是本轮的依据，不是感觉）：

1. **用户实测反馈**：F1–F4 全部收完（P11-bb/bj/bk/bo、P11-bc/bd/bg/bq）。
2. **合约未落地条目**：`docs/raycast-design.md` §6 全 `已落地`，余两条明写「本版不做 / 需拍板」。
3. **上一轮「仍待」**：P11-bq §9 三条——(a) 老拍板项要用户点头；(b) 设置页 `actionSpinner`
   在 P11-bp §2.1 已实测两条出路都封死：框架到 v0.7.3 仍然只给
   `Painter.Icon(s, r, c)`（`ui/svg.go:91` → `p.drawIcon(s, r, c, 0)`，角度只能是元素字段），
   另一条是把 LoaderCircle 重画成矢量弧（等于给一枚图标再造一份几何，属拍板项）；
   (c) shell 两处入场「别删」。

于是按第 4 档「基础功能」自选：`go.mod:5` 钉的还是 `mygo v0.7.0`，而模块缓存里已经有
`v0.7.1`、`v0.7.3`。框架的 `ui` 包就是本仓的窗口 / 绘制 / 输入 / 无障碍引擎，
**它改输入路由，用户的手感就改**，所以升级不是「顺手做做」，是要逐条定性的事。

## 2. 七条行为变化逐条定性（`diff -ru ui/` 2134 行）

导出符号**零删除**：

```
grep -E "^-func [A-Z]|^-func \([a-z] \*?[A-Za-z]+\) [A-Z]|^-type [A-Z]|^-const|^-var" /tmp/u.diff
→ 无输出
```

新增的不在用的：`PopoverMenuButton`、`Video`、`ImageSize`、`DecodeImage`、
`richtext.Document.IdentifyParagraphs/VisibleParagraphs`、`richtext.TextLayout.Lines`。

真正会改行为的有七条，逐条查了**本仓有没有路径**：

| # | 变化 | 位置 | 本仓路径（实测） | 结论 |
|---|---|---|---|---|
| 1 | 一个元素一帧只吃一个键；剩下的等下一帧，而 `len(rt.keys) > 0` 会自己请帧 | `ui/input.go` `routeKeys`、`ui/runtime.go` `runFrame` | **有**：全部键盘输入 | 结果不变（键不丢，burst 时晚一帧）；既有 2460 条 pin 覆盖 |
| 2 | 编辑命令在焦点不在编辑器上时走 `commandKey(ev, nil)` | `ui/input.go` `editKey`/`commands` 路径 | **有**（间接） | 三处 `HandleInput` 都先判 `ev.Kind != ui.InputKeyDown` → `false`（`settingsui/textinput.go:47`、`settingsui/custom.go:84`、`settingsui/settingsui.go:3627`），编辑命令不经它们 |
| 3 | 滚轮从最内层往外逐个 `HandleInput`；滚动容器只在没人接时才动 | `ui/input.go` `scroll` | **零**：`grep -rn "InputScroll" internal/ cmd/` 零命中 | 新 pin 钉住这个零路径 |
| 4 | 无障碍名字：带 `Label` 的后代给 label，不再给它的文字；`innerText` 也跳过隐藏元素 | `ui/access.go` | 有（只改 VoiceOver 读法，不改墨） | 无 pin 需要改 |
| 5 | 菜单按钮的菜单可以排到下一帧再开（`rt.menu.later`） | `ui/runtime.go`、`ui/input.go` `openMenuButton` | 有（`ui.Menu` 用法未变） | 既有 pin 覆盖 |
| 6 | 窗口 `scene.Background` 从根元素的不透明底色取（平台背景） | `ui/paint.go:65` | 零：本仓自画根底 | 无 |
| 7 | `Font.LineHeight` 新字段，`Metrics` 显式忽略它 | `ui/glyphs.go` | 零：本仓不设该字段（行高自算） | 无 |

第 1 条为什么不是缺陷而是「晚一帧」：v0.7.3 的 `routeKeys` 把没排上的键**留在 `rt.keys`**，
而 `runFrame` 结尾的 `if len(rt.retried) > 0 || len(rt.keys) > 0` 会 `host.requestFrame()`。
v0.7.0 里这个条件是 `len(rt.retried) > 0`——正是这条新分支让「等下一帧」不会变成「永远等」。

## 3. 落地

1. `go.mod:5`：`github.com/egoist/mygo v0.7.0` → `v0.7.3`；`go.sum` 由 `go get` 写，
   `go mod tidy` 再把 v0.7.0 的两行哈希摘掉（`go.sum` 净 +2/−2，仍是两行）。
   `go 1.27.3` 不变，**直接依赖逐条相同**（两版 `go.mod` 的 `require` 块只有版本号那一行不同）。
2. **产品代码零改动**：`go build ./...`、`go vet ./...` 在改完 `go.mod` 后直接过，
   `go test ./...` 一次全绿（见 §5）。
3. 新 pin `internal/ci/mygo_contract_test.go`（+2 函数）：
   - `TestTheFrameworkIsPinnedAtOrAboveTheAuditedVersion`：读 `go.mod`，把
     `github.com/egoist/mygo` 的版本与 `auditedFramework = "v0.7.3"` 比；低于就红，
     报错带两个版本号（M1 实测）。文件头把上面这张表作为「为什么是这个版本」的台账，
     并**同时认两种拼写**（`require x vY` 单行 / `require ( … )` 块内一行）——
     第一版只认后者，`go.mod` 是单行写法所以自己先红了一次（已修，见 §7）。
   - `TestTheScrollHandlersStayOutOfTheWheelPath`：扫 `internal/`、`cmd/` 的 `.go`
     产品文件（跳过 `_test.go`），出现 `InputScroll` 就红，报出 `file:line` 与整行。
     它不是「禁止滚动」，是「第一次有人要接滚轮时，把第 3 条审计重做一遍」（M2 实测）。

## 4. V / B / S 台账

- **V = 0**：没有一个像素数值需要重定基线，墨一个字节没动（非视觉轮，不出图）。
- **B = 0**：既有测试**零改动**全绿；没有产品行为变化（升级本身的行为变化在 §2 定性）。
- **S = 0**：没有测试因为结构变化改断言。

## 5. 门槛（本轮实测）

```
gofmt -l cmd internal                              → 空
go vet ./...                                       → 净
GOOS=windows go vet ./...                          → 净
GOOS=darwin  go vet ./...                          → 净
go test -count=1 ./...                             → 42 包 ok / 0 FAIL / EXIT=0
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 2462（基线 2460，+2 条 pin）
... | grep -c -- "--- SKIP"                        → 54（与基线同）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 2016（基线 2014，+2）
GOOS=linux / windows / darwin go build ./...       → 三份过
```

## 6. mutation（2 条实跑，注入 → 红 → 还原 → `md5sum -c` → `touch` 复跑绿）

| # | 注入 | 实测红（照抄） |
|---|---|---|
| M1 | `go.mod` 的 `require github.com/egoist/mygo v0.7.3` → `v0.7.0` | `mygo is pinned at v0.7.0, below the audited v0.7.3: raise the pin, or re-audit ui/ and raise auditedFramework` |
| M2 | `internal/launcher/search.go` 末尾加 `var _ = ui.InputScroll` | `internal/launcher/search.go:1162 asks for the wheel: var _ = ui.InputScroll` + 「`HandleInput` sees `InputScroll` before the scroll containers do」 |

两条都 `md5sum -c /tmp/p11br.md5` 逐字节还原（`go.mod: 成功`、`internal/launcher/search.go: 成功`），
`touch internal/ci/mygo_contract_test.go` 后复跑 `ok floter/internal/ci 0.422s`。

## 7. 本轮唯一一次「自己先红」

第一版 pin 的 `go.mod` 解析只认 `require ( … )` 块里的那一行（`len(fields) == 2`），
而本仓的 mygo 是单行 `require github.com/egoist/mygo v0.7.3`（3 个字段）→ 新 pin 自己红：

```
go.mod no longer requires github.com/egoist/mygo: the audit in docs/rounds/P11-br-report.md assumes it does
```

改成「先剥掉可选的 `require` 前缀，再比模块路径」后绿。照实记：这条 pin 修前是**误报**，
不是产品问题。

## 8. 仍待（P11-br）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行）。
2. 设置页 `actionSpinner` 仍然构建期转；升级后**仍然没有**框架侧的「绘制期旋转图标」
   （`ui/svg.go:91` 还是 `drawIcon(s, r, c, 0)`）——这条现在是**明确的上游缺口**，
   别再在框架里翻；要么等上游、要么拍板矢量弧。
3. `v0.7.1`/`v0.7.3` 新增的能力（`PopoverMenuButton`、`Video`、`ImageSize`/`DecodeImage`、
   `richtext` 三枚新 API、`Font.LineHeight`）本仓一个都没用——不是待办，是「有需要时再领」。
4. 下一个可做的非拍板候选：v0.7.0 起就有的 `Element.Cache`（P10-p 报告 §2.4 明写留给性能轮）
   与 `#753` 脏矩形，值得先量一页（设置页滚动时的重绘面积）再决定。
