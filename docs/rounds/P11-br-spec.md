# P11-br 规格 · 升级 mygo v0.7.0 → v0.7.3（七处 ui 行为变化逐条定性，唯一有路径的两条给出护栏）

## 0. 意图（一句话）

把框架从 v0.7.0 抬到已经在模块缓存里躺着的 v0.7.3，并把 `ui` 包里**每一处会改用户手感**的
行为变化逐条定性到「本仓零路径 / 有路径且被既有 pin 覆盖 / 有路径需要新护栏」，产品代码零改动。

## 1. 为什么是这一件

选题顺序的第 1–3 档都空：

- **用户实测反馈**：F1–F4 全收完（`docs/rounds/P11-az…P11-bm`）。
- **合约未落地条目**：`docs/raycast-design.md` §6 全 `已落地`，只剩拍板项。
- **上一轮「仍待」**：P11-bq §9 第 1 条是老拍板项（要用户点头）；第 2 条（设置页
  `actionSpinner` 构建期转）在 P11-bp §2.1 已经实测过两条出路都被封死——一条要框架给
  「绘制期旋转图标」（v0.7.3 仍然没有：`ui/paint.go` 的 `Painter.Icon` 依旧
  `p.drawIcon(s, r, c, 0)`，角度只能由元素字段给），另一条是把 LoaderCircle 重画成矢量弧
  （给一枚图标再造一份几何，属拍板项）；第 3 条明写「别删」。三档全堵。

于是按「基础功能」自己挑，挑的是**框架落后三个版本**这件事。证据不是感觉：
`go.mod:5` 钉的是 `github.com/egoist/mygo v0.7.0`，而本地模块缓存里已经有 `v0.7.1`、`v0.7.3`
（`/home/tar/go/pkg/mod/github.com/egoist/mygo@v0.7.3`，`go 1.27.3`，直接依赖与 v0.7.0 逐条相同）。
框架的 `ui` 包就是本仓的窗口/绘制/输入/无障碍引擎——它的输入路由改了，用户的手感就改。

## 2. 量出来的变化（v0.7.0 → v0.7.3，`diff -ru ui/` 2134 行）

导出符号**零删除**（`grep -E "^-func [A-Z]|^-type [A-Z]" /tmp/u.diff` 无输出），新增
`PopoverMenuButton`、`Video`、`ImageSize`、`DecodeImage`、`richtext` 的
`Document.IdentifyParagraphs`/`VisibleParagraphs`/`TextLayout.Lines`。真正要定性的行为变化七条：

| # | 变化（v0.7.3） | 位置 | 本仓路径 | 结论 |
|---|---|---|---|---|
| 1 | 一个元素一帧只吃一个键：第二个键等下一帧，且 `len(rt.keys) > 0` 会自己请帧 | `ui/input.go` `routeKeys`、`ui/runtime.go` | **有**：所有键盘输入 | 结果不变（键不丢，晚一帧），由既有 2460 条 pin 覆盖 |
| 2 | 编辑命令（Copy/Cut/Paste/SelectAll）在焦点不在编辑器上时走 `commandKey(ev, nil)` | `ui/input.go` | **有**（间接） | 三处 `HandleInput` 都只看 `InputKeyDown`，编辑命令不经它们的判据 |
| 3 | 滚轮从最内层往外逐个 `HandleInput`，滚动容器只在没人接时才动 | `ui/input.go` `scroll` | **零**：`grep -rn "InputScroll" internal/ cmd/` 零命中 | 新 pin 钉住这个零路径（见 §4） |
| 4 | 无障碍名字：有 `Label` 的后代给 label，不再给它的文字 | `ui/access.go` `innerText` | 有（只影响 VoiceOver 读法，不影响墨） | 无 pin 需要改 |
| 5 | 菜单按钮的菜单可以在下一帧再开（`rt.menu.later`） | `ui/runtime.go` | 有（`ui.Menu` 用法不变） | 既有 pin 覆盖 |
| 6 | 窗口 `scene.Background` 从根元素的不透明底色取 | `ui/paint.go` | 零：本仓自己画根底 | 无 |
| 7 | `Font.LineHeight` 新字段；`Metrics` 忽略它 | `ui/glyphs.go` | 零：本仓不设该字段（改用自算的行高） | 无 |

## 3. 落地项

1. `go.mod`：`github.com/egoist/mygo v0.7.0` → `v0.7.3`（`go 1.27.3` 不变，间接依赖逐条相同）。
   `go.sum` 由 `go get` 写。
2. **产品代码零改动**：改完 `go build ./...`、`go vet ./...` 直接过（实测见报告 §5）。
3. 规格 + 报告进仓库；`.pending-bugs.md` 顶部加本轮小节。

## 4. 红线（零改动）

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一律不动**。本轮不碰任何
  `internal/` 下的产品文件。
- 墨也不动（非视觉轮，不出图）。

## 5. 门槛命令 + PASS 基线（实测值）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 基线 2460（P11-bq 实测），本轮 +2 条 pin
GOOS=linux/windows/darwin go build ./...               → 三份过
```

## 6. mutation 计划（两条，说明各自会让哪个 pin 变红）

- **M1**：把 `go.mod` 的 `mygo v0.7.3` 改回 `v0.7.0` → `TestTheFrameworkIsPinnedAtOrAboveTheAuditedVersion`
  必须红（报错带两个版本号）。还原用 `md5sum -c`。
- **M2**：在任意产品文件里加一行引用 `ui.InputScroll`（如 `internal/launcher/search.go` 里
  `_ = ui.InputScroll`）→ `TestTheScrollHandlersStayOutOfTheWheelPath` 必须红（报出文件与行号）。
  还原后 `md5sum -c` 逐字节核对，`touch` 再复跑一次。
