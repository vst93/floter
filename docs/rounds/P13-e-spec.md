# P13-e 规格：扩展页表面（`docs/liquid-glass.md` §6.4）——E1 的受光边落到集成行与动作胶囊

**轮号**：P13-e（视觉队列第 4 片；§6.1 = P12-b、§6.2 = P12-c、§6.3 = P13-d 已落地）
**表面**：扩展页（`internal/settingsui` 的 Integrations 面板）**一个表面，一轮**。
**为什么现在做**：`docs/liquid-glass.md` §6 是唯一权威选题队列，§6.4 是全表里最后一个
有 `未开始` 行的表面（4-1 / 4-3）。终端弧剩下的 S1b **改会话管理语义 = 需用户拍板**，
所以按纪律回到视觉队列。

**这轮为什么让它更轻 / 更有空间感**（每个视觉轮必答的一句话）：
这一页的「面」其实**已经在场**（P12-c 给 `cardGeometry` 加过 `LitFace`，分组卡是 E1），
但页面上唯一一块**被选中**的、属于某个具体工具的整行面——`floter://register` 链接命名
的那一行——仍然只是一层 `RowSelected` 平涂：它与脚下那张 E1 卡片只差一档 alpha，
读起来是「卡里又刷了一块漆」，不是「卡上放着一个选中的块」。给这一行补上 E1 的一对
线（顶 `LensRim` 受光边 + 底 `LensEdge` 对侧），这一页才第一次出现「底（页面）/
面（分组卡）/ 选中的面（命名行）」三级而不是两级半。同一轮把动作胶囊的冠线**量下来**
（它已经戴着 `LensRim`，但从来没人用像素钉过它是 E1 的线而不是 E2 的），
并把「破损行不加冠线」写死成反面 pin——**三种面各有各的线**，空间感才有算术来源。

---

## 落地项（位置 → 现状 → 应改成）

行号是本轮 `grep -n` 实测（动手前重核），不是旧报告里的漂移值。

### 4-1 集成行面（`internal/settingsui/settingsui.go:2431`）——`RowSelected` 一档 → E1

- 现状：`integrationRowFace` 的 `Highlighted` 臂返回
  `rowFace{Face: tokens.RowSelected, FaceSet: true}`（`settingsui.go:2438`）。
  `cardRow`（`rows.go:1004`）只做 `row.Background(face.Face)`——**一层平涂**，
  顶边与底边都没有线。
- 应改成：这层「面」站起来。给 `rowFace` 加一个 `Lit bool`（`rows.go:942` 的结构体），
  `cardRow` 在按压/悬停换面之后调用 `tokens.LitFace(row)`——与 §6.1 第 1-2 行（启动器
  选中块）、§6.2 第 2-2 行（侧栏选中页）、§6.3 第 3-8 行（空态键帽）**同一个函数**。
  `integrationRowFace` 的 `Highlighted` 臂置 `Lit: true`。
- **只给被选中的那一行**：`Broken` 臂（`RaisedWarm` + 警告色内环 + 警告前导条）**不加**。
  理由（§1 的不变量）：一个面的身份是**一条**受光边；破损行的那条线已经被警告环占了
  （`InsetShadow(0,0,0,1, warning)` 四边都在），再叠一条白冠线会画进同一个 1px 行、
  把状态信号洗淡。这不是「少做一步」，是「一个面只说一句话」。写进合约 §6.4 第 4-4 行。

### 4-3 扩展动作胶囊（`internal/settingsui/rows.go:475`）——**复核 + 像素 pin**

- 现状：`extensionActionFace` 用 `tokens.Lens(button)`（`rows.go:486`），`Lens` 的第一层
  就是 `LensRim`——P12-a 升代后它已经是 0.09，所以「受光边自动到位」这一条成立。
- 应改成：**产品代码零改动**。补一条像素 pin：胶囊顶边内侧的 1px 是 E1 的 `LensRim`，
  不是 E2 的 `EdgeHighlight`，且胶囊外**没有**投影（§2 判据 2）。
  这一条正是 §6.4 原行写的「只需复核」；没有像素证据的复核不算复核。
- 不改 `Lens` → `LitFace`：胶囊的第四层 `ControlEdge` 环与它自己的
  `Border(1, ControlEdge)` 画在相邻的两行 1px 上（`rows.go:483-486`），是**同一条边的
  两次上色**，不是「画框分层」的第二层——退掉它会让胶囊的边缘从 2px 掉到 1px，
  那是几何改动，超出「视觉轮只改墨与面」的红线。留一条观察在报告里。

### 4-2 包 chip（`internal/settingsui/rows.go:1098`）——保持 `故意不做`

已在 §6.4 判过（与 §6.2 第 2-9 行同一条：30u 的方块加冠线就成噪点）。本轮不动，
但把 `Lit` 的新字段在 pin 里反着钉住（chip 不是 `cardRow` 的 `rowFace`，天然碰不到）。

---

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一个字节不动**：
本轮只加两条 `InsetShadow` 内线，不改 `Background`/`Border`/`Padding`/`MinHeight`
的任何数值，也不改 `rowFace` 的既有字段语义（`Lit` 是**新增**字段，零值 = 旧行为）。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                   # 必须空
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 必须净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 只许涨（基线 2878）
GOOS=linux|windows|darwin go build ./...                # 三平台过
```
基线：HEAD = `fab984a8`（P13-d），**PASS 2878 / FAIL 0 / SKIP 62 / 2135 测试函数**。

## V / B / S 计划

- **V 1**：`settingsui/extensionrowface_test.go` 的 `TestTheRowFacePaintsItsPaneAndKeyline`
  里 register 行的那条断言。旧：「顶边与自己的面同色（没有 keyline）」。新：顶边
  **亮于**自己的面（受光边）且**中性**（`B - R` 不超过一个噪声级，不是警告色）。
  这正是本轮的设计意图，注释写 `P13-e 重定基线：旧值 → 新值`。
  同一断言的后半（左缘 `register.bar` 与面同色 = 没有前导规则条）**不动**。
- **B**：其余全部零改动。`TestTheRegisterRowIsTheNeutralSelectedRung`（读的是行中间
  的平面，不读顶边）、`TestTheLiveIntegrationRowsLeadWithTheChip`、
  `TestTheSettingsCardControlsCarryTheStopLens`（`Lens` 未动）、
  `shell/glasscensus_test.go`（`.extensions-action-button` 的 marker `tokens.Lens(button)`
  还在）都必须原样通过。
- **S 0**。

## mutation 计划（≥2，每条说明会让哪个 pin 变红）

1. **M1 把受光边从命名行拿掉**：`integrationRowFace` 的 `Lit: true` 删掉 →
   `TestTheRegisterRowStandsOnOneLitEdge`（新）红：顶边与面同色。
2. **M2 把冠线换成 E2 的线**：`cardRow` 里 `tokens.LitFace(row)` → 手写
   `row.InsetShadow(0,1,0,0, tokens.EdgeHighlight)`（只换顶边） →
   新 pin 的「最近候选是 `LensRim`」红。
3. **M3 给破损行也加冠线**：`Broken` 臂置 `Lit: true` →
   `TestTheIntegrationRowFaceFollowsItsState` 里新加的「破损行不 Lit」断言红。
4. **M4 把胶囊的冠线去掉**：`rows.go:486` 的 `tokens.Lens(button)` 删掉 →
   `TestTheExtensionActionPillWearsTheE1Crown`（新）红：顶边与面同色。

## 出图

`/tmp/floter-P13-e-integrations-{dark,light}-{before,after}.png`（Integrations 页，
带一个 `Highlighted` 集成与一个普通集成）与
`/tmp/floter-P13-e-pill-{dark,light}-{before,after}.png`（动作胶囊特写）。
opt-in dump（`FLOTER_PNG=1` + `FLOTER_PNG_TAG`），与 P12-c / P13-d 同一机制。

## skill 用法定死（qiaomu-design，走「已有方向只要打磨」车道）

`SKILL.md` → `references/user-preferences.md`（P-1 无斜体 / P-5 可读性 / P-15 层级
融合 / P-38 无左侧竖线装饰）→ 本轮按材质+空间挑 `references/craft-loop.md`（工艺密度
菜单 #5 多层阴影、#12 材质细节内边框高光）+ `references/preflight.md`（A/B/C/D2 逐条）。
**不走** Phase 2 四方向预览（方向由用户给死，自动车道无人点选，不许出 HTML 预览页、
不许停下等）。报告必须有「偷了什么」一节，逐条写「菜单第 N 项 → 落成哪个数值」。
