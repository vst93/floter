# P13-ag 规格：对比度模式里的受光边不许被压到看不见

`docs/liquid-glass.md` §6 没有 `未开始` 行，按纪律走通用队列的「状态反馈 / 材质一致性」。
这一轮量的是 **E1 的受光边在四档材质 × 两种对比度下是不是同一条线**。

> **先量，再改（本轮的转折）**：规格最初的假设是「`frosted` 档把受光边缩到 8~10 级，
> 等于没有」——量下来 10 级是**安静**，不是消失，而且把地板抬到普通档会撞红两条
> 「各档读起来不一样」的既有 pin（`launcher/plate_lens_test.go`、
> `settingsui/lens_test.go`）。真正压到 1x 屏之下的是**对比度模式**那一臂（6 级）。
> 所以本轮只改那一臂，普通档位一个数不动。这条转折写在这里，不藏。

## 意图（一句话）

「增加对比度」的用户在深色面上拿到的受光边只有 **6 级**——比 P12-a 之前那条 0.04 还低；
一个要求「把边给我看清楚」的系统设置，反而把边抹掉了。

## 证据（先量的数）

1. `internal/theme/theme.go` 的 `LensRim: scaleAlpha(p.lensRim, spec.LensScale)`；
   `internal/glassmap/glassmap.go:105-109` 的 `LensScale` 是 0.5 / 1 / 1.6，
   对比度模式再乘 `ContrastLensScale = 0.6`（`glassmap.go:330`）。
2. 于是同一个 token（0.09）的实际墨：

   | 档 | factor | 深色面 65 上的抬升（实测像素） |
   |---|---|---|
   | `regular` | 1.0 | 0.09 → **+19 级** |
   | `frosted` | 0.5 | 0.045 → **+10 级** |
   | `regular` + 对比度 | 0.6 | 0.054 → **+12 级** |
   | `frosted` + 对比度 | 0.3 | 0.027 → **+6 级** ← 缺陷 |

3. 合约自己把两半分开写过：`glassmap.go:164` 的注说被压低的是 **sheen**（装饰性折射），
   而 `docs/liquid-glass.md` §3 说受光边是 E1 的**结构线**。对比度模式那道 0.6 是把
   sheen 的话也用在了结构线上。

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/theme/theme.go` 新增 `litEdge(c, factor, contrast)` | 受光边在对比度模式下被两级缩到 0.027 | 对比度模式：`scaleAlpha(c, max(factor, LitEdgeFloor))`，`LitEdgeFloor = 0.75`；**非对比度原样** `scaleAlpha(c, factor)` |
| 2 | 同上，`factor <= 0` 守卫 | — | `off` 档（factor 0）仍然 0：地板不许复活被停用的一档删掉的线 |
| 3 | pin `internal/settingsui/rimstop_test.go` | — | 深/浅 × `{frosted, regular}` 的对比度臂 ≥ 地板（深 12 级 / 浅 2 级）；普通档不让薄档比厚档更亮；深色普通 `regular` ≥ 12 |
| 4 | `docs/liquid-glass.md` §1 表、§3 表 + 注、§6.0 第 0-10 行 | 只说「`LensRim ≤ 0.09`」 | 写明「受光边有地板，没有天花板」+ 实测三行表 |

## 行为契约零改动红线

只改 `theme` 的一个传递函数 + pin + 文档。不碰高度链、键盘模型、会话管理、设置写回、
i18n 文案、命令接线。`lensRim` 的 token 值、`LensSheen` / `LensEdge` / `TrackRim` /
`EdgeHighlight` 一个数都不动；**普通（非对比度）档位的渲染结果逐像素不变**。

## 门槛（基线，上轮实测）

- `gofmt -l cmd internal` 空；`go vet ./...` 净；`GOOS=windows/darwin go vet ./...` 净
- `go test -count=1 ./...` 全绿；PASS **3032** → 只许涨；测试函数 **2238** → 只许涨
- 三平台 `go build ./...`

## mutation 计划（≥2 条，实测见报告）

1. `litEdge` 不看 `contrast`（退回 `scaleAlpha(c, factor)`）→ 新 pin 的四条对比度臂红
   （深色 6 < 12、浅色 1 < 2）。
2. 删掉 `factor <= 0` 守卫 → `theme.TestLensScalesWithTheGlassStop` 的「off kept a lens」红。
3. `LitEdgeFloor` 改回 1.0 → 新 pin 的「薄档仍比厚档安静」臂红（`frosted` 与 `regular` 同值）。
