# P13-et 规格 · 动作栏那一行的 42u 地板从来没落到盒子上（它是陷阱）

**意图一句话**：`rowBody` 里那句 `floor = t.Space(10.5)`（42u）写在 `MinHeight(floor)` **之后**，
一个字节都没生效；把它「修好」反而会让选中的胶囊贴着窗口最下沿。这一轮删掉它、把带子的
45u 记成**三段**（呼吸 3u + 行 40u + 卡 tail 2u），并用 pin 把这两件事钉死。

**为什么现在做**：`docs/liquid-glass.md` §6 的差距表已无 `未开始` 行，选题回到通用队列的
「边界情况」。这条是**同族最后一处**「注释与代码互相矛盾、且矛盾的方向正好是坏的」的位置：
`height.go:46-52` 的注释说带子是「42u 地板 + 3u 呼吸」，而 `launcher.go:2471` 那句地板是死码，
真正的行是 `rowUnits` = 40u，剩下的 2u 是 shell 给启动器面单独设的 `padBottom`
（`internal/shell/shell.go:4486-4489`，`t.Space(launcher.TailUnits)`）。

## 证据（出货尺 3.6px/`Space(1)`，720×439 帧，深浅同值）

| # | 量的是什么 | 值 |
|---|---|---|
| et-1 | `Space(actionBarUnits)` = `Space(11.25)`（带子） | **40.5** |
| et-2 | `Space(actionBarBreathUnits)` = `Space(0.75)`（上呼吸） | **2.7** |
| et-3 | `Space(rowUnits)` = `Space(10)`（行本身，实测 `rowRects[12].H`） | **36.0** |
| et-4 | `Space(TailUnits)` = `Space(0.5)`（卡片 tail = shell 的 `padBottom`） | **1.8** |
| et-5 | et-2 + et-3 + et-4 | **40.5** = et-1（逐位相等） |
| et-6 | 满态：12 个匹配 + 动作栏，charged | **478** |
| et-7 | 动作栏行的盒子（`rowRects[12]`） | `Y=440.2 H=36 bottom=476.2` |
| et-8 | et-7 的底边 → 窗口下沿（et-6 − 476.2） | **1.8** = et-4 |
| et-9 | 若地板真的成为 42u：行高 | **37.8**（`Space(10.5)`） |
| et-10 | 若 et-9 成立：行底 = 440.2 + 37.8 | **478.0** = 窗口下沿，et-8 → **0** |

et-10 就是陷阱的方向：不是「行变高一点」而是「选中的胶囊贴住窗口最下沿」——
§2 的 elevation 阶梯要求浮层内部的行与卡片边之间始终留一条呼吸，贴边读成「被切开」。

## 逐条落地项

| # | 位置（本轮实测行号） | 现状 | 应改成 |
|---|---|---|---|
| et-a | `internal/launcher/launcher.go:2471` | `floor = t.Space(10.5)`，赋值在 `row := …MinHeight(floor)`（`:2440`）之后 → 死码 | **删掉**，并把「为什么不能要 42u」写成注释（含 et-9/et-10 的数） |
| et-b | `internal/launcher/launcher.go:2465-2470` 的注释 | 说带子是「42u 地板 + 3u 呼吸 = 45u」 | 改说三段：呼吸 + `rowUnits` + 卡 tail |
| et-c | `internal/launcher/launcher.go:2472` | `row.Margin(t.Space(0.75), 0, 0, 0)` 裸字面量 | `t.Space(actionBarBreathUnits)` |
| et-d | `internal/launcher/height.go:46-52` | `actionBarUnits` 注释同样说 42u + 3u | 改写为三段账，并指向 shell 的 `padBottom` |
| et-e | `internal/launcher/height.go` 常量区 | 只有 `actionBarUnits` | 新增 `actionBarBreathUnits = 0.75`（3u），与 `actionBarUnits` 并排 |
| et-f | 新 pin `TestTheActionBarBandIsBreathRowAndTail` | — | 三段之和 == `actionBarUnits`（纯数，任一格被改就红） |
| et-g | 新 pin `TestTheActionBarKeepsTheCardsTailUnderIt` | — | 渲染：行盒高 == `Space(rowUnits)`；行顶 == 带顶 + `Space(actionBarBreathUnits)`；行底 → 窗口下沿 == `Space(TailUnits)` |
| et-h | opt-in `TestDumpP13ETBarPNGs` | — | light / dark 各一张满态帧，`FLOTER_DUMP_P13ET=1` |

## 行为契约零改动红线

- 高度链：`actionBarUnits` = 11.25 **不动**；窗口 charged 高度一像素不动。
- 键盘模型 / 会话管理 / 设置写回 / i18n / 命令接线：**不碰**。
- 视觉：本轮**零像素改动**（`Space(0.75)` 与旧字面量 0.75 同值），前后帧必须逐字节相同；
  这是刻意的——本轮的交付是「删掉一句会误导下一次改动的死码 + 钉住它守的那条呼吸」。

## 门槛命令与 PASS 基线（实测，P13-es 收尾值）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3356（基线）
GOOS=linux|windows|darwin go build ./...  # 过
```

预期：测试函数 2527 → **2529**（两条 pin；opt-in dump 也算一个 → 2530），
PASS 3356 → **3359**，SKIP 175 → **176**。

## mutation 计划（≥2 条，实跑）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | 把 `floor = t.Space(10.5)` 加回去，并真的落到盒子上（改成在 `MinHeight` 之前赋值：`floor := t.Space(10.5)` 给 `isBar` 分支用） | `TestTheActionBarKeepsTheCardsTailUnderIt`（行高 36 → 37.8、行底 → 窗口下沿的间距 1.8 → 0） |
| M2 | `actionBarBreathUnits` 0.75 → 1.5（呼吸翻倍，带子不变） | `TestTheActionBarBandIsBreathRowAndTail`（2.7+36+1.8 ≠ 40.5 → 5.4+36+1.8 = 43.2） |
| M3（对照） | 把 `actionBarBreathUnits` 的**字面量**换回 0.75 的等价写法（`Space(0.75)`） | 全绿（证明 et-c 是纯可读性，不改变几何） |
