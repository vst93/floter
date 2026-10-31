# P13-am 规格：终端浮条的五枚键补上第三档（面沉一档，几何留字给框架）

**意图**：`terminalui` 浮条上的五个 28u 键（设置 / 便条 / 新命令 / 系统终端 / 关闭）此前只有
静息与悬停两档，指针**按下去**的那一帧与悬停帧逐像素相同——键不沉、不可信。本轮给它们补上
`docs/liquid-glass.md` §5 的第三档：面往浮条自己的面退一格，`LitFace` 的一对面保留。

**为什么现在做**：这是 `docs/liquid-glass.md` §6.7b 第 **7-9** 行——上一轮（P13-al）把同一个洞
在 `internal/shell` 的四个键上补完，并把这枚记成「故意不做（本轮）+ 下一轮候选」，理由写死为：
它的墨来自**画布**（`canvasInks` / `barInks`），板子里没有「按下的洗色」这一档，直接借
`ControlPress` 会在命名调色板的深色画布上压出一块窗口灰。本轮补的正是 `barInks` 的第六档。

## 现状（动手前实测，非旧报告行号）

- `internal/terminalui/terminalui.go:1066` `barInks` 五档：`Ink / Muted / Tertiary / Wash / Face`。
- `:1085` `windowBarInks`：`Wash = f.SurfaceHover`（窗口自己的控制阶梯）。
- `:1125` 命名调色板分支：`Wash = ink.Alpha(0.1)`。
- `:1417` `barButton` 只有 `if button.Hovered()` 一臂（`Background(inks.Wash)` + `LitFace`）。
- 实测（本轮新 pin 的原始输出，四套墨源 × 五枚键）：

  | 墨源 | 浮条面 | 悬停面 | 按下面 |
  |---|---|---|---|
  | inherit-light | 247,247,248 | 221,221,222 | **221,221,222** |
  | inherit-dark | 14,14,16 | 48,48,50 | **48,48,50** |
  | ink-lightwindow | 11,13,16 | 33,36,38 | **33,36,38** |
  | paper-darkwindow | 244,239,228 | 223,219,209 | **223,219,209** |

  四套全部逐像素相同：这不是墨的深浅问题，是**这一档根本不存在**。

## 落地项

1. `barInks` 加第六档 `Press`（`:1066`）。两套墨源各自的取值：
   - `windowBarInks`（`:1085`）：`Press = f.SurfacePressed`——浮条浮在窗口自己的画布上时，
     它的两块面**就是**控制阶梯的两档，不需要派生。
   - 命名调色板（`:1125`）：`Press = ink.Alpha(barPressAlpha)`，`barPressAlpha = 0.035`。
     0.035 不是新数：控制阶梯自己的比例是 `0.032/0.105 = 0.305`（浅）与 `0.05/0.14 = 0.357`（深），
     浮条洗色是 0.10，退一档取这个带子的上沿 0.035——因为浮条的洗色本来就比设置页薄，
     0.035 是调色板墨在**最暗的画布**（`contrast` / `forest`）上还能画出 ~8 级的那一档。
2. `barButton`（`:1417`）改 `switch`：`case button.Pressed()` 先于 `case button.Hovered()`，
   按下臂 `LitFace(Background(inks.Press))`。
3. **不跟族里的 `Scale(0.92)`**（见「取舍」）。

## 取舍：为什么这一页不做几何收档

族里（P13-q 设置页 / P13-al shell）按下是「面沉 + 盒子收 0.92」。本轮实测把它带到这一页会
**丢掉一整个点击**：浮条上的键在抽屉滑入的那两帧里只有 **2 DIP** 宽（实测盒子
`{858 314 2 28}`，`drawer_test.go` 的两条 pin 点的就是那一帧）；`Scale` 绕盒子中心缩，
2 DIP 的键左边缘往里让 0.08 DIP，正好把落在中心线上的点击点挤出命中框，释放时
`rt.contains` 为假。实测后果：`TestTerminalDrawerClosesFromItself` / `TestTerminalDrawerGearToggles`
两条 **B** pin 变红（`the drawer's own close did not close it` / `the flipped gear did not close the drawer`）。

按纪律「B 红了是产品回归，改产品不改测试」：这一页只收面，几何写成
`liquid-glass.md` §6.7b 第 **7-10** 行的「需拍板」（命中的账属于框架：盒子收缩时命中框跟着动）。

## 行为契约零改动红线

高度链（`barHeight`）、揭示 / 命中（`live` / `PassThrough`）、键盘模型、会话管理、i18n 文案、
命令接线：**零改动**。只改墨与面。

## 门槛与基线（本轮实测）

```
gofmt -l cmd internal                                  必须空
go vet ./...  /  GOOS=windows go vet ./...  /  GOOS=darwin go vet ./...   必须净
go test -count=1 ./...                                 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      3047 → 只许涨
GOOS=linux|windows|darwin go build ./...               必须过
```

基线（HEAD `84c13722` 实测）：**PASS 3047 / FAIL 0 / SKIP 93 / 测试函数 2252**。

## mutation 计划（≥2，每条说明会让哪个 pin 变红）

- **M1** 按下臂换成 `inks.Wash` → ② 「按下面 ≠ 悬停面」红。
- **M2** `barPressAlpha` 0.035 → 0.2（越过悬停洗色）→ ③ 「按下面在两档之间」红。
- **M3** `windowBarInks` 的 `Press: f.SurfacePressed` → `f.SurfaceHover` → inherit 两套的 ② 红。

## 出图

`/tmp/floter-P13-am-<case>-<hover|press>.png`，四套墨源 × 悬停/按下 = 8 帧（opt-in `FLOTER_PNG=1`）。
