# P13-dv 规格：捕获输出页的五条前提破（夹具取景收口第三格）

## 意图

`P13-du`「仍待」第 1 条 = `docs/liquid-glass.md` §6.86 表里「前提破（S）」那一行：
捕获输出页在**出货尺**下有五处夹具的**前提**不成立（不是数值漂移）。本轮把这五处换到
出货尺夹具，并用页面自己量出来的几何**重新建立前提**——不换断言强度、不加容差带。

为什么现在做：`internal/launcher` 的夹具债（§6.76 拆成 `dg-t` / `dg-l` / `dg-s`）逐面交割，
`dg-l` 的 21 条里 V 类已在前几轮做完（P13-dq/dr/ds/dt/du），只剩这一族 S 和后面的混排 B。
这是「启动器/输出页」这个表面的最后一批取景账。

## 先量（改前实测，`renderShipped` 出货尺：Spacing 3.6 / 帧高 439）

| # | 夹具 | 改前（库存尺/字面量） | 出货尺实测 |
|---|---|---|---|
| 1 | `headingfold_test.go:246` 编号族 | `len(numbers)==9` | **10**（`1..9` + 第十条 `0`） |
| 2 | `outputedges_test.go:66` 列表分支呼吸带 | `lastInkAbove(img, 118)`，带高 `16` | 条末墨 **83**，裁剪上沿 **100**，带 `86..99`（**14** 行 = `round(4×3.6)`） |
| 3 | `outputedges_test.go:105` 正文分支呼吸带 | 同上（118/16） | 裁剪上沿 **100**；条末墨 83/85（浅/深） |
| 4 | `outputedges_test.go:138` 视口下沿 | `466` | **419.2**（帧高 488 → 439） |
| 5 | `outputfold_test.go:161` 首行整行 | `visibleShare == 1` | 比例 **0.999999769**（`7.2 + 705.6` 的 float32 加减丢 1 ulp）；盒 `x=7.2..712.8 y=100.4..136.4` 完全在视口 `x=0..720 y=100.4..439` 内；静息墨 **219** |

第 3 条的根因不是尺子：正文分支**没有记裁剪盒**（`a.outputListRect` 只在列表分支被赋值），
所以夹具只能退回库存字面量 118。这是产品的记账缺口，不是取景问题。

## 落地项

1. **`internal/launcher/output.go:761`（产品，只补记账）**：正文分支的 `ui.Scroll` 取
   `scroll.Bounds()` 记进 `a.outputListRect`。`assignOutputNumbers` 对正文早退，编号族不受影响；
   两个分支现在都报同一个「内容从哪里开始」，呼吸带因此可以在两条分支上用同一条规则量。
2. **`outputedges_test.go:56` `outputAirBand(clipTop)`**：带的下沿 = 裁剪上沿，高 =
   `round(4 × tokens.Theme.Spacing)`（出货尺 14，库存尺 16）。删掉 118 与 16 两个字面量；
   夹具先读 `a.outputListRect.Y`，再从带的上沿往回找条末墨。
3. **`outputedges_test.go:138`**：视口下沿 pin `466 → 419.2`（`tenths` 读，注释写旧值→新值）。
4. **`outputfold_test.go:161`**：「整行在视口里」由比例改成**几何谓词**（四条边），
   因为比例在 float32 上天生会落在 1 的下一格；静息墨阈值 150 不动，实测 219。
5. **`headingfold_test.go:246`**：编号族 `9 → 10` 条（`1..9` + 第十条 `0`），注释写明
   旧结构 → 新结构；标题盒仍在自己的表里，不进编号族。
6. **`outputedges_test.go:269`**：新增 1 条 opt-in dump（`FLOTER_PNG=1`），
   出货尺下两条分支 × 深浅四张图，供报告肉眼复核。

## 行为契约零改动

- 高度链、`MaxY`、滚动手感、键盘模型、会话、设置写回、i18n、命令接线全部不动。
- 产品改动只有 `outputListRect` 的**记账**（正文分支新增一行赋值 + 字段注释）；
  没有任何墨、面、间距、动效变化。

## 门槛（本轮实测基线）

```
gofmt -l cmd internal                # 空
go vet ./...                         # 净（windows / darwin 同）
GOOS=linux/windows/darwin go build   # 全过
go test -count=1 ./...               # 绿
PASS 3325（基线 3325）/ FAIL 0 / SKIP 159（+1 opt-in dump，基线 158）
测试函数 2492（基线 2491，+1 opt-in dump）
```

## Mutation 计划（≥2 条，实跑）

- M1（产品记账）：正文分支 `outputListRect` 只留宽、丢掉 Y → 正文带夹具应红
  （`clip top y=0`）。
- M2（带的锚点）：`outputAirBand` 从 `clipTop-h` 挪到 `clipTop` → 列表与正文两支
  都应在带里读到墨。
- M3（编号族 pin）：把 `want 10` 退回 `9` → `10 entries, want 9` 应红。
