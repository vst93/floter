# P13-ac 规格：浮层那一档在透明度滑块的两端还站得住吗（先量）

## 意图

`docs/liquid-glass.md` §6 已无 `未开始` 行，按纪律走通用队列的「层级 / 空间感」。本轮对象是
任务书「现在该做什么 / 先量再改的两个数」的**第 1 条**，原文：

> `Float` 档固定 +0.10。默认 47% 框上浮 0.57 对面 0.10 足够；但透明度拉到 10% 时浮 0.20
> 对面 0.10 只差 0.10 —— 可能要按框的 alpha 缩放面档。

这是一条**假设**，没有数字。它猜的方向是「低透明度端太弱」。本轮把它量到底：把
`glassmap.Spec.FloatAlpha()` / `ContentAlpha()` 与 `TintAlpha` 的差，沿滑块 10→100 逐档算出来
（`internal/glassmap/glassmap.go:225-231` 的 `floatStep = 0.1`、`:371` 的 `contentStep = 0.09`、
`:138` 的 `SolidTop = 0.98`），再回答「要不要按 alpha 缩放面档」。

## 现状（动手前读的代码，行号已核）

- `internal/glassmap/glassmap.go:396-406` `FloatAlpha()`：`min(1, TintAlpha+floatStep)`。
- `internal/glassmap/glassmap.go:392-396` `ContentAlpha()`：`overFrame(TintAlpha, min(1, TintAlpha+contentStep))`。
- `internal/glassmap/glassmap.go:263-264`：`target = min(SolidTop, opacity/100)`，`SolidTop = 0.98`。
- `internal/theme/theme.go:580`：`Float: floatFill.Alpha(floatAlpha)`，色 `#18191D` / `#FCFCFE`。

## 量出来的数（`SpecForPreferencesOS("linux", "regular", o, false)`，无平台 veil）

| 滑块 | TintAlpha（底） | FloatAlpha（浮） | 浮 − 底 | 相对 |
|---|---|---|---|---|
| 10% | 0.10 | 0.20 | **0.10** | **2.00×** |
| 25% | 0.25 | 0.35 | 0.10 | 1.40× |
| 47%（默认） | 0.47 | 0.57 | 0.10 | 1.21× |
| 70% | 0.70 | 0.80 | 0.10 | 1.14× |
| 90% | 0.90 | 1.00 | 0.10 | 1.11× |
| 100% | 0.98 | **1.00** | **0.02** | 1.02× |

两条结论：

1. **假设的反方向才是真的。** alpha 差是**常数** 0.10，所以 alpha 的**相对**抬升在低端最大
   （10% 时浮层是底的两倍），而浮层颜色相对桌面是近黑/近白（`#18191D` / `#FCFCFE`），
   框越薄、桌面透得越多，**像素**差越大。10% 端不需要缩放，缩放反而会把两档拉平。
2. **真正塌掉的是顶端。** `SolidTop = 0.98` 把底封在 0.98，浮被 `min(1, …)` 封在 1.00，
   差从 0.10 掉到 **0.02**——滑块推到头，E2 的 alpha 阶梯几乎不存在。

## 落成

**不改产品像素**（V 0）。理由写进 `docs/liquid-glass.md` §2：顶端的浮层不是靠 alpha 分家的，
它靠 §2 判据 3 的两层影 + `EdgeHighlight` 顶边；而把底从 0.98 压到 0.88 会改掉
`main_opacity` 的承诺（「滑块就是框自己的 alpha」），那是行为契约，不是墨。

**加 pin**（两处，都是「先量再改」留下的证据链）：

- `internal/glassmap/floatladder_test.go`：沿滑块六个档位钉「浮严格高于底」「面不低于底」
  「默认档差 = 0.10」「两端差 = 0.10 / 0.02」，并把上表抄进注释。
- `internal/theme/theme_test.go` 追加一条：**把滑块推到两端**，`TestTheThreeElevationRungsAreVisibleAndOrdered`
  的四条判据仍然成立（现有那条只量默认 47%）——这是任务书假设 #1 的直接反面：面档不随 alpha 缩放。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**一字未动**。
`glassmap.Spec` 的语义（滑块即 alpha 真值）不动。

## 门槛

- `gofmt -l cmd internal` 空
- `go vet ./...`、`GOOS=windows go vet ./...`、`GOOS=darwin go vet ./...` 净
- `go test -count=1 ./...` 全绿，PASS 计数只许涨
- 三平台 `go build ./...`

## mutation 计划（≥2，实跑）

- M1：把 `floatStep` 0.1 → 0.03。期望：新 pin 的「默认档差 = 0.10」与 theme 那条的
  `Float > Control` 至少一条变红。
- M2：把 `FloatAlpha()` 的 `min(1, …)` 换成 `TintAlpha`（浮 = 底）。期望：新 pin 的
  「浮严格高于底」在每一档都红。
- M3：给 `TintAlpha` 加 0.0 …不改；改为把 `SolidTop` 0.98 → 1.0。期望：顶端两档相等，
  「浮严格高于底」在 100% 那档红。

每条都要 `md5sum -c` 逐字节还原，并 `touch` 后复跑（避免测试缓存）。
