# P11-ar 规格 · 一份新输出从它自己的开头开始

**轮号**：P11-ar（上一轮 P11-aq，HEAD `d91ece0`）

## 意图

给一次新的捕获输出打开时，把输出的滚动偏移与输出列表的位置**归零**，让它从自己
的第一行开始——而不是沿用上一次输出的偏移。这是 P11-ao（换设置页回顶）在启动器侧
的同一条交互规则：**一份新内容从它自己的开头开始。**

## 为什么现在做（证据，本轮实测）

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 只剩 `⌘K`
动作面＝拍板）→ 上一轮「仍待」第 1 条（`Absolute()` 逐一量）→ **界面质感与交互**（用户定：
界面/质感/交互优先）。上一轮「仍待」第 1 条剩余项本轮逐条否掉（见报告 §5），于是按用户
定的优先级在「交互逻辑」这一档里实测到这条真 bug：

- **现状（`internal/launcher/launcher.go` 附近字段声明）**：
  `Output ui.ScrollState`（约 `:464`）与 `OutputList ui.ListState`（约 `:500`）都是
  **App 级**状态——它们是滚动容器跨帧保留偏移所必需的。
- **触发点**：`internal/launcher/output.go` 的 `showOutput`（约 `:107`）每次把
  `a.output = &view` 换成**一份新的视图**，但从不碰 `a.Output` / `a.OutputList`。
- **实测（本轮诊断脚本，已删除）**：跑一份 300 行的文本捕获，滚轮滚到 `Output.Y=400`，
  `Esc` 关掉，再跑**另一份 300 行**的捕获——新页面打开在 `Output.Y=400`，用户看到的
  是它的第 ~15 行起步，而不是第一行。
- **为什么不是「故意的」**：字段上那句注释 `Output keeps its scroll offset` 描述的是
  **跨帧**要求（同一份输出在下一帧仍要保留偏移，否则每帧回顶），不是「跨视图」决定。
  `loadMoreOutputPage`（`output.go:179`）是**同一视图**续页，不经过 `showOutput`，
  所以本轮的归零不会打断分页。

## 逐条落地项

1. **`internal/launcher/output.go` · `showOutput`**
   - 现状：`view := a.OutputFor(...)`; `view.Args = args`; `a.output = &view`;
     `a.Selected, a.chosenRow = 0, -1`。滚动/列表状态原样沿用上一份输出。
   - 应改成：在换上 `a.output` 之前，把 `a.Output = ui.ScrollState{}` 与
     `a.OutputList = ui.ListState{}` 重置一次，再赋 `a.output`。注释写清「新捕获从它
     自己的开头开始」（引 P11-ao 的同一条规则）。**只**在 `showOutput` 里重置，
     逐帧的重建路径一动不碰（否则每帧回顶）。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。
- 只改「一份新输出第一帧的滚动状态」这**一个**行为。滚轮、触控板、`ScrollIntoView`、
  分页续页、`Esc` 关闭路径全部不动。
- 视觉：墨与面零改动（这是行为轮，不出产品像素图；但要在报告里给出「改前/改后第一帧
  的顶行是哪一行」的实测读数）。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨（基线 2381）
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

- 基线（HEAD `d91ece0` 本轮实测）：**PASS 2381** / FAIL 0 / SKIP 29 / 测试函数 1946。

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

- **M1**：把 `showOutput` 里的 `a.Output = ui.ScrollState{}` 删掉（保留列表那条）——
  `TestAFreshOutputStartsAtItsBeginning` 的文本臂必须变红（新文本页沿用旧 `Y`）。
- **M2**：把 `a.OutputList = ui.ListState{}` 删掉（保留文本那条）——
  同一条 pin 的列表臂必须变红（新列表页沿用旧可见行）。
- **M3**（可选）：把重置移到 `View`（每帧执行）——既有的滚轮 pin
  `TestTheOutputPageScrollsWithTheWheel` 必须变红（滚轮滚不动，每帧被归零）。

每条：注入 → 看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 再复跑一次。

## 新 pin

- `internal/launcher/outputframe_test.go`（或新文件）新增
  `TestAFreshOutputStartsAtItsBeginning`：
  - 文本臂：长文本页滚到 `Y>0` → `Esc` → 再开一份长文本页 → `a.Output.Y == 0`
    且首行是第一行内容。
  - 列表臂：长列表页滚到可见首行 > 0 → `Esc` → 再开一份长列表页 →
    `a.OutputList.Visible()` 的 `first == 0`。
  - 反面：**同一**视图内滚轮仍生效（引用既有 `TestTheOutputPageScrollsWithTheWheel`
    已覆盖，本 pin 不重复）。
