# P13-eu 规格 · 只读行不答指针

**意图一句话**：一张设置卡上「报告一件事」的行（后端铸造的 id、程序自己选的值、块的标题）
此前与「有可按东西」的行答**同一个**指针——`cardRow` 无条件问 `Hovered()`/`Pressed()`，
于是只读行的面在指针底下抬得和它下面真能编辑的字段**逐字节相同**，按下去也塌，而按下去
什么都不发生。这一轮给 `rowFace` 加 `Inert`，六个只读构造点全部熄掉指针反馈，**只熄面**。

**为什么现在做**：`docs/liquid-glass.md` §6 差距表已无 `未开始` 行（P13-et），选题回到通用
队列的**状态反馈**。`Lens` 一族已结案（只剩 `control.SegmentedIndex` 一处「故意不做」），
这一族的另一半是**禁用 / 只读面**——P13-et「仍待」第 2 条点名要它，且此前**从未量过**。

## 证据（先量再改；720×2200 帧，深浅各一遍）

读数 = 行盒中段、行左内边距里的一小段（`lensMean`，3 行带），指针由 `Tester.Move` 落在行的
label 中心，120ms 面过渡跑完再读（`Advance(300ms)`）。

| # | 外观 | 行 | 静息面 | 指针下 | Δ |
|---|---|---|---|---|---|
| eu-1 | 浅 | **只读**（`fixedRow`，集成 id，`settingsui.go:5197`） | 229 | **205** | **−24 级**（三通道和 −72） |
| eu-2 | 浅 | 可编辑（`row`，Name，同一张卡下一行） | 229 | 205 | −24 级 |
| eu-3 | 深 | **只读**（同上） | 39 | **69** | **+30 级**（+90） |
| eu-4 | 深 | 可编辑（同上） | 39 | 69 | +30 级 |

eu-1 与 eu-2、eu-3 与 eu-4 **逐字节相同**。`mygo@v0.7.3/ui/input.go` 的 `Hovered()`/`Pressed()`
说的是「指针在不在这儿」，不是「这行能不能点」——两个问题此前被当成了同一个。

## 逐条落地项（行号为动手前实测）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| eu-a | `internal/settingsui/rows.go` `rowFace`（`Inert` 前是 `Muted` 的注释块） | 没有这个字段 | 新增 `Inert bool`，注释写明「报告一件事、没有可按的东西」+ §6.116 的实测数 |
| eu-b | `internal/settingsui/rows.go` `cardRow` 的 hover/press 分支（`:1567` 起） | `switch { case row.Pressed(): …; case row.Hovered(): … }` | 最前一支 `case face.Inert:` —— 面留在卡片平面上 |
| eu-c | `rows.go` `factRow`（`:1352`）/ `dataRow`（`:1410`）/ `labelRow`（`:1437`） | `rowFace{}` | `rowFace{Inert: true}` |
| eu-d | `settingsui.go` `keyValue`（`:4352`）/ `fixedRow`（`:4385`）/ `diagnosticsSection`（`:2410`） | `rowFace{}` | `rowFace{Inert: true}` |
| eu-e | 只读行的墨 / 盒子 / 分隔线 / `Selectable` | — | **一个字节不动**（只读是一种结构，不是一种墨，§6.10 / §6.84） |
| eu-f | 新 pin `TestAReadOnlyRowDoesNotAnswerThePointer` | — | 真页面：只读 id 行 hover / press 面**零位移**；对照：它下面可编辑行必须仍抬 ≥12 级 |
| eu-g | 新 pin `TestEveryReadOnlyRowFamilyIsInert` | — | 合成卡上五个只读构造器逐个量，全零位移；对照行仍抬 |
| eu-h | opt-in 出图 `TestDumpP13EUReadOnlyHoverPNGs`（`FLOTER_P13EU_DIR`/`FLOTER_P13EU_TAG`） | — | 深/浅 × {指针在只读行, 指针在可编辑行} 四张 |
| eu-i | `docs/liquid-glass.md` §6.115 第二张表 | 表头 4 列、数据行 5 格 → `TestTheLiquidGlassContractSupersedesTheRaycastOne` 在 HEAD 上就是**红**的 | 表头拆成 `现状 | 落成`，六行补 `已落地` |

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**全部不动**。本轮只改「行答不答指针」
这一件事：墨不动、盒不动、`Selectable` 的取值不动、分隔线不动。`Inert` 的零值 = 旧行为，
所以任何没显式标记的行（包括所有可编辑行、扩展行、会话行）几何与读数都不变。

## 门槛命令与 PASS 基线（实测，HEAD = `01e2bc35` P13-et）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3360（HEAD 实测；同一次 HEAD 有 1 FAIL）
GOOS=linux|windows|darwin go build ./...  # 过
```

**HEAD 的真实基线是 3360 PASS / 1 FAIL / 177 SKIP / 2532 测试函数**（在 `git worktree` 里
对 `01e2bc35` 实测，不是照抄上一轮报告）：那 1 个 FAIL 是 §6.115 表格式（eu-i）。
预期收尾：函数 2532 → **2535**（两条 pin + 一个 opt-in dump），PASS 3360 → **3363**
（+2 新 pin，+1 是 eu-i 修好的既有红），SKIP 177 → **178**。

## mutation 计划（≥2 条，实跑）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | `case face.Inert:` → `case false:`（整族的支路消失） | 两条 pin 全红：真页面 hover −72/+90、press 同样；五个构造器逐个红 |
| M2 | 只把 `dataRow` 的 `Inert: true` 去掉（族内单点回退） | 只有 `TestEveryReadOnlyRowFamilyIsInert` 的 `dataRow` 两行红；真页面那条仍绿 |
| M3（对照） | `case face.Inert: row = row.Background(face.Face)`（零值 `Face` = 不换面，语义等价） | 全绿——证明 pin 钉的是「面不动」这件事，不是某个写法的形状 |
