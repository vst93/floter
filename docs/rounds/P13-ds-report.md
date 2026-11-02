# P13-ds 报告：状态带 `30u → 27`、编号族视口沿 `60 → 54`（pin 挪到 app 真在画的那个刻度）

**为什么让它更轻 / 更有空间感**：本轮一次像素都没动（产品零像素），它做的是**把尺子摆正**——
编号族的视口沿与底部状态带的两个数一直量着框架库存主题（1u = 4px）的帧，而 app 出货画的是
3.6。绿着的 pin 与真机差 6px 和 3px，于是「改一个像素」与「改一整个刻度」在测试里读不出差别；
这一支（`§6.86` 的夹具债）没清完之前，任何间距/层级打磨都缺一把可信的尺子。

## 1. 证据（动手前实测）

| 位置 | 现状 | 现在 |
|---|---|---|
| `numberband_test.go:46` | `render`（库存尺帧 `frameHeight(10)` = **488**） | `renderShipped`（出货尺 **439**） |
| `numberband_test.go:91` | `LightTheme().Space(14+1)` = `4.0 × 15` = **60** | `designMetrics()` 的 `3.6 × 15` = **54** |
| `numberband_test.go:159` | `render` | `renderShipped` |
| `numberband_test.go:188` | `charged - up != 30`（30u 写死成 30px） | `30 × designUnit()` = **27** |
| dump（opt-in） | `renderIn` + `/tmp/floter-P11-*` | `renderShipped` + `/tmp/floter-P13-ds-*` |

产品侧真值：`launcher.go:3349` 的 `FeedbackBand()` = `Space(7.5)` → 3.6 × 7.5 = **27**；
`height.go` 的 `chrome` 里 `fieldRowUnits + breathUnits` = 15 spaces → **54**。

## 2. 落地

1. 两条 pin 的取景换成 `renderShipped`，数字换成「参考刻度 × `designUnit()`」，比较走 `tenths()`
   （十分之一像素的整数比较，**没有容差带**）。
2. **顺手收掉第二份拷贝（零像素、零行为）**：`height.go` 里状态带原本手写两次 `7.5 * spacing`
   （状态注行 `:318`、底部带 `:353`），而同名常量 `launcherStatusUnits = 7.5` 只喂了视图。
   变异 M3 第一次**静默**就是被这个撞出来的（改常量 8.0 时窗口高度不变）。两处改成 `launcherStatusUnits`
   之后常量真的同时驱动「画」和「算」——这正是 `§6.86` 那类「同一判据两份手写拷贝」的最后一个。
3. dump 取景同步（opt-in，不进 PASS 基线）。

**台账**：V 2（两条 pin，旧值 60/30 → 新值 54/27）/ B 0 / S 0（断言 2 处）。产品代码 2 行
（同一常量替换同一字面量，像素零变化）。测试函数数 **2489**（`grep -c "^func Test"`，不变）。

## 3. 变异（实跑，原始报错串照抄）

- **M1+M2（一次跑，两条 pin 各自红）**：期望改回 `30` 与 `LightTheme().Space(15)`：
  ```
  numberband_test.go:100: the family's viewport starts at y=54.0, want the field's band 60.0 (15 spaces at the shipped step)
  numberband_test.go:203: holding the modifier grew the window by 27.0, want the reference's 30u status band (30.0 at the shipped step)
  ```
  → 两条都真的钉在出货尺上，不是「看起来对的小数」。
- **M3（产品回归）**：`launcherStatusUnits` 7.5 → 8.0：
  ```
  numberband_test.go:202: holding the modifier grew the window by 29.0, want the reference's 30u status band (27.0 at the shipped step)
  ```
  **第一次跑是静默的**（那时计费还读字面量 `7.5`）——这条静默就是 §2 第 2 条的依据，不是巧合。
- 还原后 `md5sum -c /tmp/P13-ds.md5` 两条 **成功**，`touch` 两个文件再复跑一次：`ok`。

## 4. 门槛（实测）

```
gofmt -l cmd internal                       空
go vet ./...                                净
GOOS=windows / darwin go vet ./...          净
go test -count=1 ./...                      全绿，FAIL 0
go test -count=1 -v ./... | grep -c "--- PASS"   3325（基线 3325，不变；本轮不新增测试）
SKIP 156（沿用），顶层测试函数 2482（`=== RUN   Test` 无子测试）
GOOS=linux / windows / darwin go build ./... 全过
```

## 5. 出图（自己复核）

`/tmp/floter-P13-ds-{numbers,tenrow,heldhint}-{light,dark}.png`。图上读到的那一处：
`heldhint-light` 从 **720×488** 变成 **720×439**，卡片下沿从 y=482 移到 y=433 —— 少掉的 49px
就是这批 pin 一直在算、app 从来没申请过的那半截帧；`numbers-light` 日志里首行盒子
`{Y:41.99 H:36}`（40u × 0.9 = 36），视口 `{Y:54}`（15 spaces × 3.6）。

## 6. 用了设计 skill（`qiaomu-design`，「已有方向，只要打磨」车道）

读：`SKILL.md` → `references/user-preferences.md`（P-1 禁斜体 / P-5 可读性优先 / P-7 功能契约不得被创意吃掉）
→ `references/craft-loop.md` §二 工艺密度菜单 → `references/preflight.md` A / B / C / D2。

**偷了什么**（逐条落成具体数值）：

1. `craft-loop.md` 菜单**第 6 项「编辑细节（hairline / 编号）」** → 状态带与视口沿都记成
   **一个参考刻度的整数倍**：`30u × 0.9 = 27.0`、`15 spaces × 3.6 = 54.0`，比较用 `tenths()`
   （270 / 540），**没有容差带**——hairline 的判据是「一条线」，不是「差不多一条线」。
2. `craft-loop.md` 菜单**第 12 项「材质细节（内边框高光 / 带边界）」** → 本轮不动像素，但把它落成
   下一格的判据：带的地板与视口沿现在有可信的尺子，`§6.s` 系列的「受光边只给浮层」才谈得上量。
3. 其余 10 项（氛围层、focus 环、流体字号、彩蛋…）**故意不做**：本轮是 Go 自绘的测试尺子，
   没有 DOM、没有字号 clamp、没有页面级氛围层；`craft-loop` 的「≥5 项」是页面级门槛，
   硬凑会让报告失真。

**preflight 逐条**：
- **A 反套路**：通过。本轮零新视觉元素（无渐变白边、无 glass 叠 glass、无 emoji、无斜体）。
- **B 设计读取一致性**：通过，而且这正是本轮的全部内容——图片、pin、产品计费三者现在读同一把尺子。
- **C 动效抽查**：不适用/通过。没有新增过渡；`renderShipped` 的深色切换仍走 `Advance(500ms)` 等帧。
- **D2 中文页（正文 ≥ 14px）**：**仍未过** —— `tokens.Caption` 9.9px / 正文 10.8px（第 44 份证据），
  这是「需拍板」项，不在本轮擅自改。

## 7. 仍待

1. `P13-dg-l-1b` 剩下 4 条单位算术，各占一轮：历史行 pin 控件 `20u → 18`（`powerfaces_test.go`）、
   空页带顶 `60 → 54`（`emptystate_test.go`）、过滤条带（`filterband_test.go`）、
   取整 2 条（`int()` → `math.Round`）。
2. `P13-dg-l-2` = 5 条前提破 S；`-l-3` = 2 条混排 B；`-l-4`（产品墨）= `outputsubtitle` 的 6 级。
3. **需拍板（第 44 份证据）**：`tokens.Caption` 9.9 / 正文 10.8 与 preflight D2「≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
