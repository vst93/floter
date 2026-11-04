# P13-ga 规格：键盘焦点的环 —— 19 个可聚焦站点，环到底长在哪

**意图一句话**：P13-fi 把「禁用」的**指针**那一半数完了（14 个 `Disabled(...)`、一套光标说法），
但同一个「状态反馈」家族里的另一半从来没数过——`Focusable()` 在 `internal/` 里有 **20 处**，
没有一处自己画环，环全靠框架 `ui/paint.go:385` 的 `p.FocusRing(box, e.radius)`。这一轮把
「键盘走到哪里看得见吗」量到底：环画在盒子**内**还是**外**、什么颜色、谁能让它消失。

**为什么现在做**（任务书「下一轮 = 细节打磨 · 禁用 / 只读面」的接续）：任务书给的行号
`internal/settingsui/rows.go:568-588` 那枚「建时就禁用」的胶囊，P13-fi 已经量完并落了 pin
（14 个站点、`docs/liquid-glass.md` §6.130）——「禁用面」这一格**空表**了。「状态反馈」剩下
唯一没被数过的就是键盘焦点。工艺密度菜单第 3 项（`references/craft-loop.md`）甚至指名道姓：
「品牌 focus 环：focus-visible 用品牌色 + 偏移，**不用浏览器默认蓝**」——本仓是不是这样，没人验过。

## 1. 落地项（动手前逐条核过行号）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| ga-1 | `internal/settingsui/focusring_test.go`（新文件） | 环的几何无 pin | `TestTheKeyboardRingRidesOutsideTheBox`：浅/深两臂，Tab 上焦，量四条边的 2px 带、盒内（内缩 2）、3u 带外三件事 |
| ga-2 | 同上 | 环的颜色无 pin（靠 `theme.go:480` 一行 `t.Focus = p.focus` 悬着） | 断言从**画面**读色：B−R ≥ 60（不是从结构体读） |
| ga-3 | 同上 | 谁把环摘掉无 pin | `TestNoOneTakesTheKeyboardRingAway`：12 个文件 → 20 个站点的普查表；`FocusRing(false)` 必须在同函数里手画环 |
| ga-4 | 同上 | 图 | opt-in dump `FLOTER_P13GA_DUMP=1` → `/tmp/floter-P13-ga-focusring-{light,dark}.png` |

**产品代码零改动**：视觉已经是对的（§2），这一轮只把它钉住。

## 2. 先量（探针跑完即删/并入 pin，数字抄在这里）

`lensProbe`（`internal/settingsui/lensface_test.go:45`）给一块 128 中灰的地面，
上面一枚 `Focusable()` 的 120×40 控制（`Size(120,40).Radius(6)`，放在 (20,20)，
避免 1px 带落在半覆盖行上）。`tt.Key(0, ui.KeyTab)` 给键盘焦点（`ringShown` 要
`rt.focusVisible`，只有键盘驱动才置位，`ui/inline.go:223-226`）。

- 水平剖面（浅，cy=40）：x=14/15/16 = 128/128/128 → **x=17/18 = (28,84,159)** → x=19..23 = 128 灰。
- 垂直剖面（浅，cx=80）：y=17/18 = (28,84,159)，其余同值。
- 所以带在 **`[X-3, X-1)` 与 `[Y-3, Y-1)`**、右侧 `[X+W+1, X+W+3)`、下侧镜像；
  盒沿（X、Y）本身**不是**环。
- 颜色：浅 `(28,84,159)` = 调色板 `RGBA(6,74,166,0.82)` 压在 128 上的合成值
  （期望 28.1 / 83.7 / 159.2）；深 `(123,158,227)` = `RGBA(122,167,255,0.78)`
  （期望 123.3 / 158.4 / 227.1）。**逐通道一位不差** → 环用的是调色板的 focus，
  不是框架 stock 的 `RGBA(37,99,235,0.55)` / `RGBA(96,165,250,0.6)`（`ui/theme.go:123/:150`）。
- 一条量出来的事实：**圆角弧会侵入盒内 2 个像素，但只在四个角顶点**。
  `FocusRing` 把 rect 长 3、半径 +4（6→9），弧心 (X+6, Y+6)、半径 9，
  恰好过 (X, Y)——所以 (20,20) 是环色。盒内判据必须内缩 2 才干净。
- 普查：12 个文件 **20** 处 `Focusable()`，**0** 处 `FocusRing(false)`。

## 3. 门槛

```
gofmt -l cmd internal                     # 空
go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...   # 净
GOOS={linux,windows,darwin} go build ./...                                # 过
go test -count=1 ./...                    # 全绿，PASS 3453（基线 3449）
```

## 4. mutation 计划（≥2，每条说明让哪个 pin 变红）

- **M1** `internal/theme/theme.go:757` `focus: RGBA(6,74,166,0.82)` → 灰 `RGBA(128,128,128,0.82)`
  → 让 ga-1 的「带是蓝的」8 行全红（B−R = 0）。证明颜色判据读的是调色板，不是结构体。
- **M2** `internal/settingsui/rows.go:1807` 那行 `Focusable()` 后面加 `.FocusRing(false)`
  → 让 ga-3 红（`rows.go:1: FocusRing(false) with no hand-drawn ring`）。
  证明普查是真扫 AST，不是一张摆设的表。
