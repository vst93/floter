# P13-ev 规格 · 堆叠行的三对，此前只有一条缝

**轮号**：P13-ev（上一轮 P13-eu，HEAD `90a7e77b`）
**选题**：通用队列 · **间距节奏**（`.pending-bugs.md` P13-eu「仍待」第 3 条：
`rowLineGap()` 是字面量 1px 而 `Space(1)` = 3.6——「行内两档墨之间的那条缝还没按出货尺量过；先量再改」）。
**表面**：设置页的 **堆叠行**（`internal/settingsui/rows.go:1701` `stackedRow`）。

## 1. 意图一句话

**同一个「标题 + 它的解释」对，在两种行里读成两条不同的缝**：控件在右边的行读 6px，
控件在下面的行读 9px；而后者还把「解释 → 控件」也塞进同一条 3.6px 的缝里，
于是那句解释读起来像控件的一部分。本轮只改这两个盒缝，不动墨、不动盒高、不动控件。

## 2. 为什么现在做（先量再改，证据是本轮实测）

| # | 位置 | 现状（本轮实测，860×1200，en） |
|---|---|---|
| 1 | `rows.go:1701` `stackedRow` | 列上一条 `Gap(t.Space(1))` = 3.6px 管**三对**：标题→解释、解释→控件（`rows.go:1715`） |
| 2 | `rows.go:427` `rowLineGap()` | trailing 行（`rows.go:1611` `names` 列）用 **1px**，是这一族自己的行内缝 |
| 3 | 实测：General 页「Language」行 | 标题盒 `263.80..281.80`、解释盒 `282.80..299.80` → 盒缝 **3.6**，墨缝 **9px** |
| 4 | 实测：General 页「Launch at startup」行 | 标题盒 `500.00..518.00`、解释盒 `519.00..536.00` → 盒缝 **1.0**，墨缝 **6px** |
| 5 | 实测：同一页的「解释 → 控件」 | 墨缝 **5px**（浅）/ **7px**（深），比「标题 → 解释」（9px）**还紧** |

第 3/4 行是同一个对（同一档墨、同一句关系）在两个行族里的两个读数；第 5 行更硬：
**一件可以按的面**比那句话的两行之间还近。§4.3 的规矩（间距属于两侧那一对，不属于区域）
在这里被一条 `Gap` 违了两次。

## 3. 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| ev-1 | `rows.go` `stackedRow` 的外列 | `Gap(t.Space(1))`，三对共用 | `Gap(t.Space(2))` —— 只留给「解释 → 控件」那一对 |
| ev-2 | 同处 | 标题与解释是列里的兄弟 | 收进一个 `ui.Column(c).FillWidth().Gap(a.rowLineGap())` —— 与 trailing 行同一条缝（1px；集成卡上随 `cardLineGap` 的 4px） |
| ev-3 | 墨 / 盒 / 控件 / 分隔线 / 行计数 | — | **一个字节不动**（这一轮只改两条缝） |
| ev-4 | 新 pin `TestAStackedRowSpacesItsTitleLikeAnyOtherRow` | — | 真页面上两个行族的「标题→解释」**墨缝**相等（±1px），且 trailing 那条在 4..9 之内（守探针本身） |
| ev-5 | 新 pin `TestAStackedRowsControlStandsApartFromItsSentence` | — | 「解释→控件」的墨缝比「标题→解释」至少大 2px |
| ev-6 | opt-in 出图 `TestDumpP13EVStackedGapPNGs` | — | `FLOTER_P13EV_DIR` / `FLOTER_P13EV_TAG`，深浅两张（General 页前两张卡） |
| ev-7 | `docs/liquid-glass.md` **§6.117** | — | 新小节：三对三条缝 + 旧值→新值 + pin 账 |

**旧值 → 新值（盒缝）**：标题→解释 `Space(1)`（3.6）→ `rowLineGap()`（1.0）；
解释→控件 `Space(1)`（3.6）→ `Space(2)`（7.2）。**净高度**：每个堆叠行 **+1.0px**
（−2.6 + 3.6），所以整页的节奏不动，只有缝的分档变了。

## 4. 行为契约零改动红线

- **高度链**：`height.go` 一个字不动；行高仍由内容与 44u 地板决定，堆叠行净 +1px 是缝的算术。
- **键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线**：全部不动（`stackedRow` 只画盒子）。
- 视觉轮只改墨与面——本轮更窄：**只改两条盒缝**，墨一个字节不动。

## 5. 门槛命令 + PASS 基线（HEAD `90a7e77b` 实测）

```
gofmt -l cmd internal                              # 空
go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...
GOOS=linux|windows|darwin go build ./...
go test -count=1 ./...                             # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"  # 基线 3363
go test -count=1 -v ./... | grep -c -- "--- SKIP"  # 基线 178
```

预期：PASS 3363 → **3365**（两条 pin），SKIP 178 → **179**（一个 opt-in dump），
测试函数 2535 → **2538**，FAIL 0。

## 6. mutation 计划

| # | 注入 | 应让谁红 |
|---|---|---|
| M1 | 外列 `Gap(t.Space(2))` → `Gap(t.Space(1))` | `TestAStackedRowsControlStandsApartFromItsSentence`（控件墨缝 9/11 → 5/7，与那句话 6px 同档） |
| M2 | 回到本轮的形状之前（三对一条 3.6 缝、无内列） | **两条**：标题→解释 9 vs trailing 6（+3），且控件比那句话还近（5 < 9） |
| M3（对照） | 内列 `Gap(a.rowLineGap())` → 字面量 `Gap(1)` | **全绿**——pin 钉的是读数，不是写法 |
