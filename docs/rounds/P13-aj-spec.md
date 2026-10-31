# P13-aj 规格：空态的间距属于成对的两件东西（墨上 12/12 → 8/12）

**意图一句话**：空态区域里「标记 → 句子 → 理由」三件东西在墨上量出 **12px / 12px**，
把 22u 的标记读成一个平级的邻居；把它与它引入的那句话收成一块（4u），三件东西才有领属。

**为什么现在做**：`docs/liquid-glass.md` §6 已无 `未开始` 行，按纪律走通用队列的
「间距节奏 / 空态满态」。候选队列里 P13-ai 记的第 ② 条就是「空态图标的第一档墨在深浅两套
下的级数」——本轮先量，量完发现**墨是对的、间隙是平的**，于是改间隙。

## 现状（实测，动手前）

`internal/settingsui/empty.go:41-43`：`ui.Column(c).FillWidth().MinHeight(floor).Gap(t.Space(2))`
——一个数用在每一对部件之间。渲染会话页空态（720×620，深/浅各一帧），在内容列
（`x=188..680`、`y=120..580`）里按 luma 检测墨带（相对区域中位面 ±12 级、每行 ≥3 像素，
≤3 行的悬挂部合并）：

| 帧 | 墨带（y 区间） | 墨的最强级数 |
|---|---|---|
| dark | 标记 `165-182` / 句子 `195-207` / 理由 `220-227` | 139 / 222 / 94 |
| light | 同上逐像素一致 | 139 / 219 / 102 |

⇒ 标记→句子 **12px**，句子→理由 **12px**（差值 0）。三档墨的阶梯本来是对的
（句子 ≫ 标记 ≫ 理由），**只有间隙这一条要改**。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/empty.go`（`emptyState`） | 图标与标题直接挂在区域的统一 8u 上 | 图标 + 标题合成一块 `head := ui.Column(c).Center().Gap(t.Space(1))`（4u）；理由、动作仍挂在区域 8u 上 |
| 2 | 同上，注释 | 无「为什么这一对更近」 | 写清 §4.3：间隙属于成对的两件东西；旧值 12px → 新值 8px |
| 3 | `internal/settingsui/emptyink_test.go`（新） | 空态只有盒子账（`empty_test.go` 的 8u） | 墨带检测 + 三条账 × 深浅两套：①标记→句子比句子→理由至少近 2px；②标记下 ≥6px 空气；③句子−标记 ≥20 级、标记−理由 ≥20 级。附 opt-in 出图 `FLOTER_P13AJ_DUMP=1` |
| 4 | `docs/liquid-glass.md` | §4 只说刻度，不说「一对可以比邻居近」 | 新增 **§4.3**（规则 + 旧值→新值表 + pin）；§6 新增 **§6.9**（12-1..12-4 行，全部 `已落地`） |

## 行为契约零改动红线

只动 `emptyState` 的**内部结构**（多一层 `head` 列）与它的**垂直间隙**：

- 高度链：区域仍是 `MinHeight(floor)`（92u / 页体 160u），实测 `empty_test.go` 的两条账照旧通过。
- 水平度量、行高、`Padding(24u)`、理由的 `MaxWidth(320u)`：一个数没动。
- 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**。
- 四条调用点（`settingsui.go:1527`、`settingsui.go:1737`、`custom.go:50` 及其自身的测试）自动生效。

## 门槛命令与 PASS 基线（动手前实测）

```
gofmt -l cmd internal                        → 空
go vet ./... / GOOS=windows|darwin go vet    → 净
go test -count=1 ./...                       → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 基线 3043
go test -count=1 -v ./... | grep -c -- "--- SKIP"   → 基线 90
测试函数总数（grep -c "^func Test"）          → 基线 2245
```

## mutation 计划（每条会让哪个 pin 变红）

1. `head` 的 `Gap(Space(1))` 换回 `Space(2)`（恢复均匀 8u）→ `TestTheEmptyRegionHangsItsMarkOffItsSentence`
   的第一条账（近 ≥2px）红：实测 **12px / 12px**。
2. 标记的墨 `TextSecondary` → `TextTertiary` → 同测试的第三条账（标记 − 理由 ≥20 级）红：
   实测 深 95 vs 94 / 浅 104 vs 102。
3. 理由的墨 `TextTertiary` → `TextSecondary` → 同一条账红：实测 139 vs 137（理由比标记还亮）。

每条跑完用 `cp` 还原 + `md5sum -c` 逐字节核对。
