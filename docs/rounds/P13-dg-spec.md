# P13-dg 规格：三个画面包的取景收口，先量「一把尺子还是两把」

**轮号**：P13-dg（第三十四轮，接 P13-df）。
**选题**：P13-df「仍待」第 1 条 —— `docs/liquid-glass.md` §6.75 写死的下一格。
**结论先行**：本轮**不改产品、不改测试**，把「一个包的尾巴」量成「两把尺子」，
并把原来的一格拆成三格（每包一格）。理由与实测都在报告里。

---

## 1. 意图

上一轮证明「夹具画的是框架库存主题，生产画的是设计主题」。本轮要回答的是
**这两套主题差的是不是同一个比例** —— 如果只差一个 0.9 的缩放，收口就是重钉数字；
如果不是，那么「按间距算出来的帧高」本身就是错的，收口必须先修帧高再重钉。

## 2. 证据（本轮实测，行号自己核过）

| 量 | 库存（夹具） | 设计（生产） | 比例 |
|---|---|---|---|
| `c.Theme().Spacing` | 4.0 | 3.6 | **0.90** |
| `c.Theme().FontSize` | 14.00 | 10.80 | **0.77** |
| `MeasureText("Ag", 正文)` | 22.0 | 17.0 | **0.77** |

- 生产装主题的位置：`internal/shell/shell.go:4262`（`c.SetTheme(surface.Tokens.Theme)`）。
- 三个包的主夹具：`internal/launcher/view_test.go:56`、`internal/terminalui/view_test.go:22`、
  `internal/settingsui/view_test.go:34`（`render` 有，`renderThemed` 只有 settingsui 有）。
- 启动器的帧高只按间距算：`internal/launcher/view_test.go:41` 的 `frameHeight`
  用 `Geometry{Spacing: spacing, Rows: ..., Held: rows}`，**没有传 Font/TitleLine/SubtitleLine**，
  而 `Geometry.Height()`（`internal/launcher/height.go`）在字段缺省时按 `font = 14` 兜底。

## 3. 逐条落地项（本轮实际做的）

1. **量启动器**：给 `render()` 装设计主题（`c.SetTheme(a.tokens(c).Theme)`）实跑，
   红 **21** 个测试函数，逐条抄下失败串 → 归类（报告 §3 的表）。
2. **量终端页**：给 `render()` 装 `tokensOf(a, c).Theme` 实跑，红 **≥5** 个函数
   （P13-df 记 8 个，本轮 grep 窗口截断，以 8 为准）。
3. **判定**：0.90 ≠ 0.77 ⇒ 「帧高按间距算」在设计尺上多出近一行；
   这一条把 21 条红劈成三类（数字漂 / 前提破 / 混排），并给出每类的处置手续。
4. **拆轮**：`§6.75` 的一格拆成 `P13-dg-l`（launcher）/ `P13-dg-t`（terminalui）/
   `P13-dg-s`（settingsui），每格一片，钉子台账逐条写 V/S，**不许**加容差、`t.Skip`、放宽断言。
5. **两条产品缺陷登记**（不进本轮）：终端抽屉的顶缝在设计间距下丢失；捕获输出页的
   列表在页头的呼吸带里起笔。两条都在 `.pending-bugs.md` 的「仍待」里写了复现串。

## 4. 行为契约零改动红线（本轮全部遵守）

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**
（`git status` 只有 `docs/` 与 `.pending-bugs.md`）。

## 5. 门槛

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净（+ windows / darwin）
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux|windows|darwin go build ./...
```

**PASS 基线（本轮实测，不是旧报告）**：3311 PASS / 0 FAIL / 148 SKIP / 2467 测试函数。
本轮结束后应当**逐项相同**（零代码改动）。

## 6. mutation 计划

**不适用**：本轮改动只有 `docs/` 与 `.pending-bugs.md`，没有任何可注入缺陷的代码路径
（照实写，不拿跑绿冒充变异）。变异留给 `P13-dg-l/-t/-s`：那三格各自的 V 钉都可以用
「把 want 退回库存值」当场打红，S 钉用「把窗口换回 640×360」打红。
