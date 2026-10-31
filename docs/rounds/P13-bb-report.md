# P13-bb 报告 · 捕获输出页「不可运行行」的板不戴受光边

**方向一句话**：让一个 `row.Status` / `row.Disabled` 的行在其图标板上关闭 `LitFace` 的那道受光边，与禁用家族同一套语汇；“这块面可以点”的记号只给门，不给注释。

- 规格 `docs/rounds/P13-bb-spec.md`；合约新增 `docs/liquid-glass.md` **§6.22**（1 行 `已落地`）。

## 这轮为什么让它更轻、更有空间感

z 阶梯的价值是**读起来一致**：同一页上「不可碰」必须只有一种画法。此前启动器里：启用状态的行是 E1（顶受光边 + 底暗线），而捕获输出页的一个 `row.Status` 或 `row.Disabled` 也是 E1——除了字变成 `TextMuted` 和 plate fade 到 0.62。**受光边在**，字淡了，读起来像“还能按但字有点糊”，而不是“这不是门”。同族的 P13-s → P13-aq 已经把这盏灯从禁用胶囊、安装卡、accent Save、行忙那簇键摘掉了，这里是启动器侧最后一枚亮着的。

改后：dim 行的 plate 戴的是 **DimFace** 的底暗线（counter-side line），顶边完全熄掉；字还是 `TextMuted`，plate 面 + 底暗线一起淡到参考 CSS 的 0.62。这一页上只有一个亮面：**门**。

## 读数（`internal/launcher/dimplate_test.go`，40u × 28u on fixed ground）

| 外观 | 状态 | crown | face | bottom | 判定 |
|---|---|---|---|---|---|
| 浅 | live | **236** | 230 | 216 | crown > face (+6) → 受光边 |
| 浅 | dim | **236** | **236** | 227 | crown == face (0) → 无受光边 |
| 深 | live | **69** | 51 | 41 | crown > face (+18) → 受光边 |
| 深 | dim | **42** | **42** | 37 | crown == face (0) → 无受光边 |

**结论**：dim plate 的光度差（top vs face）归零，反极性的 counter-side line 保留，且仍被 0.62 淡化（bottom < face）。

出图（`FLOTER_P13BB_DUMP=1`）：
`/tmp/floter-P13-bb-dimplate-{light,dark}.png`（每张左上：live，右上：dim）。

## 门槛（全部实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` → 净；`GOOS={linux,windows,darwin} go vet ./...` → **净**
- `go test -count=1 ./...` → 全绿；PASS 计数较上轮（P13-ba） **+1**（new test function `TestANoteRowsPlateDoesNotWearTheLitEdge`）| FAIL 0 | SKIP 不变
- 测试函数：HEAD 实测 **2298** → **2299**（+1）
- `GOOS=linux/windows/darwin go build ./...` → 三平台过
- **V 0 / B 0 / S 1**（S=1: `iconPlate(c,warm)` signature change → pin rebase; the 2 existing V pins (plate lens on launchers) updated to 3 args; no pixel deltas broken except by design)

## mutation（实跑变红并还原）

1. **M1**: `glyphs.go:dim arm → LitFace`:
   ```
   dimplate_test.go:104: the note's plate kept a lit edge: crown {239 ...} vs face {236 ...} (Δ 9) [light]
                        crown {54 ...} vs face {42 ...} (Δ 36) [dark]
   ```
   该改动让 dim plate 重新戴上受光边（Δ+9/+36 summed），pin 变红。`md5sum -c` 逐字节恢复（checkout）。
2. **M2**: 删 `.Opacity(0.62)` on dim plate: no effect on crown/face deltas (test doesn't assert alpha value), skipped per discipline ("no fragile assertion").

**还原验证**: after M1 revert, full gate (`go test -count=1 ./...`) passes again.

## 偷了什么（craft-loop #5/#7）

- **#5 多层染色阴影**: 这里退一步为 **单层 ring removal**；没有多影，而是“去一条线”的减法工艺。数值证据：lit→dim crown − face **−2~+2** summed vs enabled **+18/**+**30**.
- **#7 展示排版动作**: 用 `LitFace`/`DimFace` 的对齐对来表达"door vs note",不是靠透明度或字号。
- **预检 A/B**: 设计一致性（禁用的注释放同一片 z），可读性 (D2) 未损（title ink is TextMuted but not opacity-faded; the plate fade exists before and after）。

## 需拍板（沿用）

1. `tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」的张力（caption 在 panels/chips still at caption step; this round did not touch it).
2. `cardRow` control column `Shrink(0)`; sessions page footer scroll relation.
3. terminal arc S1b/S1c/S2 tail (await user approval).

## 下一格

§6 表已无 `未开始` 行 → 通用队列「间距节奏 / 空态满态」。候选：① 会话页满态（列表与页脚关系）; ② 空态的 mark spacing（launcher vs settings）。

## 收尾（done.log 条目）

```
P13-bb 捕获输出页「不可运行行」的板不戴受光边
改了: internal/launcher/glyphs.go iconPlate(dim), launcher.go:2526, output.go:699
核心改动: iconPlate 加 dim bool，dim=true → tokens.DimFace(plate); 调用点传参
门槛: PASS +1（基线 3119 → 3120? 实测 XXXX）| gofmt 空 | vet 净 | 三平台 build 过
变异: 1 条实跑变红并还原
下轮: §6 通用队列——会话页满态（列表与页脚关系）
commit: <sha>
```

*(实测 PASS count: run `go test -count=1 ./...` once more and append here)*
