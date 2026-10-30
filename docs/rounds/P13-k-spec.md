# P13-k 规格：toast 顶部那条「渐变」量到底——它既不是自己的影，也不是兄弟的影

轮号：**P13-k**（`docs/liquid-glass.md` §6 在 P13-j 之后没有 `未开始` 行，按纪律走通用队列）

## 1. 意图

**一句话**：把 P13-j「仍待」第 2 条那个悬着的观察（「toast 的 `Float` 面半透明，E2 两层影
从面里透出来，pane 顶 14 行是 241→202 的纵向渐变」）量到底，判定它是产品缺陷、dump 台的
假象，还是两者各一半——然后**只按量到的事实动手**，不按上一轮的猜测动手。

为什么现在做：它是 P13-j 留下的唯一一条「已量到、未判定」的观察，且直接落在本轮总目标的
第二条验收轴上（「轻便液态玻璃」= 浮层的面该不该被背后的东西涂脏）。如果它是真的，那是
E2 材质的一处硬伤；如果是假的，那它会把后面几轮引到「给浮层加不透明度」这条错路上——
两种结果都必须先量。

## 2. 候选假设与判据

| # | 假设 | 判据（可量） |
|---|---|---|
| H1 | pane 的影从它自己的面里透出来 | 同一个 `Float` 面 + `theme.ElevThrow` 的**隔离渲染**，面内每一行必须等值（±1） |
| H2 | 上面那张 toast 的 throw 落在下面那张的面里 | 两张 toast 时，下面那张的第一个内行与它的「无 toast 参照帧」同一行相差 ≤ 4 级 |
| H3 | 渐变是**背景透过玻璃**（滚动边缘的不透明回放） | 无 toast 的参照帧在同一行上就已经有同样的渐变；且 pane 内 = `0.77×面 + 0.23×背景` 的合成值 |

## 3. 落地项

1. **量 H1**：`internal/shell` 里用 `ui.Tester` 画一个孤立的 `Float` 面（`tokens.Float` +
   `InputStroke` + `ElevThrow` + `EdgeHighlight`），逐行读 `R`。→ 结论进报告。
2. **量 H2**：`a.notices` 塞 1 张 / 2 张，逐行读缝与下面那张的面。→ 结论进报告。
3. **量 H3**：同一个 surface 再渲染一帧**没有 toast** 的，逐行读同一列。→ 结论进报告。
4. **产出尺子**：新增 `internal/shell/toastramp_test.go`（opt-in dump），把「1 张 / 2 张 ×
   深/浅」四张 PNG 写到 `/tmp/floter-P13-k-toast-stack-<appearance>-<n>.png`，并把每一行读数
   写进 `t.Logf`。下一轮要动滚动边缘时先拿它对比。
5. **不产出像素断言**：本轮量出的原因是「dump 台的窗口后面没有桌面」，`ui.Tester` 的底座
   与真机桌面不是同一个东西；在这种帧上钉一条「面内必须平」的 pin，钉住的是台子的黑底，
   不是产品。理由写进报告，不静默跳过。

## 4. 行为契约零改动红线（本轮）

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一行不碰**。
- toast 的入场（160ms translate/scale/opacity）、`ElevThrow`/`ElevThrowCompact` 的几何与
  墨、`Float` 面、`EdgeHighlight` 冠线：**一行不碰**（本轮没有判定它们有错）。
- `settingsui` 的滚动边缘：**不改**。量到的候选修法（回放「框体的半透明面」而不是不透明
  底色）在插件 API 上不可表达——`glass.ScrollEdge` 的 `Background` 是**一个不透明色**，
  它自己乘 0.85（`plugins/glass/edge.go:91` 的 `softEdge = 0.85`，`bg.Alpha(softEdge)`
  是**替换** alpha，不是相乘）。要改得先拍板，见 §7。

## 5. 门槛命令 + PASS 基线（本轮开工前实测）

```
gofmt -l cmd internal                      # 空
go vet ./...                               # 净
GOOS=windows go vet ./...                  # 净
GOOS=darwin  go vet ./...                  # 净
go test -count=1 ./...                     # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 2920
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## 6. mutation 计划（≥2 条）

- **M1**：把 `theme.ElevThrow` 里的 `Cast` 效果绕开（改成在 pane 的 `Children` 之后补一条
  `Shadow`），隔离渲染里 H1 的面内必须出现渐变 → 若不变，说明 H1 的判据不敏感，报告里写明。
- **M2**：在 `toastStackFrame` 里把「无 toast 参照帧」换成「有 toast 帧自己」，H3 的比对必须
  自相矛盾（同一行相等）→ 证明尺子真的在比两帧。
- **M3**：把 `ElevThrow` 的 `scale` 换成常数 1（丢掉 `ElevScale`），两张 toast 的缝里读数必须
  变化 → 证明 H2 的读数抓的是 throw 本身。

## 7. 需拍板（不在本轮动手）

- **滚动边缘回放不透明底色**：`settingsui.go` 的 `body` 传 `glass.ScrollEdge{Background: t.Background}`，
  而 `t.Background` 是不透明 `RGB(243,244,247)`（浅）/ `RGB(17,18,20)`（深）。窗口半透明时
  这条 16px 的带子回放的是一个**屏幕上任何地方都不存在的颜色**：深色壁纸 + 浅色外观时，
  设置页内容顶部会出现一条 91 级的亮带（dump 台实测 228 对 137）。修法只有两条，都要拍板：
  ①把带子的回放色改成「框体当前的合成色」（需要插件支持半透明回放）；②承认它只是滚动
  提示、把带子的高度改成「条高 + 10 DIP」并让它只在滚动时出现。
