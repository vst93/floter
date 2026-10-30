# P13-o 规格 · 凹槽的一对线（`LensInset`）

**轮号** P13-o ｜ 队列：`.pending-bugs.md` 的 P13-n「仍待」第 2 条（`docs/liquid-glass.md` §6 没有
`未开始` 行 → 通用队列「层级一致性」）｜ 表面：**开关 / 分段控件的轨道**（一个材质规则，三个调用点）

## 意图（一句话）

凹槽的受光边在**浅色档不存在**：0.09 的白被同一条边上的 0.08 黑压过去，于是「洞里积光」读成
「两条暗线的框」——把浅色档凹槽底边抬到近白，让凹槽第一次真的是**凹**的。

为什么现在做：P13-n 把深色档的面·对侧抬到 0.20 之后，深色凹槽的一对线（顶 −8 / 底 +21）都读得出来，
而浅色档**从来没有人量过**。参考实现（`floter-main/src/styles/base.css:593-597` 的
`--glass-lens-stack-inset`，浅色 `--glass-lens-rim-base: 0.9` / 深色 `0.15`，`:899`）本来就是
「浅色近白」，是本仓 P8 那一代把整族光泽压到 0.04 时一起压掉的——`LensInset` 的注释至今还写着
「The light pools along the bottom edge of the groove」，而浅色档它并没有。

## 证据（实测，真设置页地面）

| 外观 | 地面 | 轨道面 | 顶内线 | 底内线 | 判读 |
|---|---|---|---|---|---|
| light | 247 | 232 | 197（**−35**） | 221（**−11**） | 顶暗、底也暗 = 画框，不是凹槽 |
| dark | 14 | 36 | 28（−8） | 57（**+21**） | 顶暗、底亮 = 凹槽（P13-n 之后） |

根因（`internal/theme/theme.go:1105-1110` + `:684-688`）：`LensInset` 的底边线是 `LensRim`（浅色
白 0.09），而它下面还压着 `TrackEdge`（浅色黑 0.08）。两条都是半透明、叠加之后
232 → 213.4（环）→ **217**（rim），净 −15；rim 只有 +2 的量级，赢不了环的 −19。
深色档没有这个问题，因为它的环是**白** 0.075（`theme.go:787`），两条白线相加。

## 落地项

1. `internal/theme/theme.go`
   - `palette` 新增 `trackRim ui.Color`（挨着 `trackEdge`，`theme.go:917` 那一行）；
   - 浅色 `trackRim: ui.RGBA(255, 255, 255, 0.70)`（旧值 = 借用 `lensRim` 0.09）；
   - 深色 `trackRim: ui.RGBA(255, 255, 255, 0.09)`（**不动**：深色凹槽的底边实测 +21，已经读得出）；
   - `Tokens` 新增 `TrackRim ui.Color`（`theme.go:186` 的 `TrackEdge` 旁边），
     `:580` 处按 `scaleAlpha(p.trackRim, spec.LensScale)` 装配（「玻璃关掉」时和透镜族一起归零）；
   - `LensInset` 的底边线 `t.LensRim` → `t.TrackRim`（`theme.go:1109`），注释写 旧值 → 新值。
2. `internal/theme/theme_test.go`：`off` 档的归零清单加 `TrackRim`；`TestLensScalesWithTheGlassStop`
   的递增清单加 `TrackRim`（它随 `LensScale` 分级，和 `LensRim` 同一条链）。
3. 新增 `internal/settingsui/groovepair_test.go`：在**真页面的地面**上量真组件
   （`a.switchControl` 与 `control.SegmentedIndex`），三条 pin + 一条 opt-in dump：
   - 浅色：底内线比轨道面亮 **≥ 6 级**，且**不高于地面**（凹槽不许比它被切进去的那张纸更亮）；
   - 深色：底内线比轨道面亮 **≥ 6 级**（守住 P13-n 之后的 +21，别被这轮改坏）；
   - 两套外观：顶内线比轨道面**暗**（`LensInset` 仍是从上往下看进去的凹槽）。
4. `docs/liquid-glass.md`：§1 上限表新增 `TrackRim` 一行（旧值 → 新值）、§3 新增 `TrackRim` 一行、
   §6.0 新增 **0-9**、§6.2 的 2-5 补一句「凹槽的底边是另一条线」。

## 行为契约零改动红线

只改墨与面：高度链（`rowUnits` / `MinHeight`）、键盘模型、会话管理、设置写回、i18n 文案、
命令接线**一行不动**。`LensInset` 的层数仍是四层（环 / 顶暗 / 接触影 / 底亮），只换底亮那一层的
token；`control.SegmentedIndex` 的选中段（`Lens` + accent 环，§6.2 的 9-11）**不动**。

## 门槛（基线 = P13-n 实测）

```
gofmt -l cmd internal            # 空
go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...
go test -count=1 ./...           # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # ≥ 2944，只许涨
GOOS=linux/windows/darwin go build ./...
```

基线：PASS **2944** / FAIL **0** / SKIP **72** / 测试函数 **2167**。

## 变异计划（每条要能让哪个 pin 变红）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | 浅色 `trackRim` 回 0.09（= 借 `lensRim`） | 新的浅色底内线 pin（−11 < +6） |
| M2 | 浅色 `trackRim` → 1.0 | 「不高于地面」那条（凹槽比纸亮 = 类玻璃边） |
| M3 | `LensInset` 底边线换回 `t.LensEdge`（没有受光边） | 两套外观的底内线 pin |
| M4 | `LensInset` 里 `TrackEdge` 与 `TrackRim` 换序（环画在最后） | 浅色底内线 pin（环压住近白线） |
