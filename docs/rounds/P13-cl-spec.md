# P13-cl 规格 · 重新检查键一按下，它的词就横移 15px

选题：P13-ck「仍待」第 1 条之外的第一件**不需要拍板**的事（通用队列 `状态反馈`）。
P13-ck 的下一格候选（`Integration.state` 把 `Enabled` 读成「运行中」，`i18n/copy.go:1228`）
是 i18n 红线，本轮只把它量清楚、写进「需用户拍板」，不动文案。

## 意图

**一个键按下之后，它自己不该搬家。** 集成页的重连 / 重新检查键是本仓唯一带「在飞」面的
行内控制：静息时是**一行字**（`rows.go:507` `actionButton` → `extensionActionButton`），
在飞时是**环 + 一行字**（`rows.go:800` `busyActionButton`，`actionSpinner` 2.5u = 10px、
`Gap(Space(1.25))` = 5px）。多出来的这 15px 是**凭空长出来的**，不是从某个槽里换出来的：
参考里 LoaderCircle 是**顶替**那枚已有的 Wrench / RefreshCw（`ExtensionRow.tsx:305,325,364`），
1:1 换，所以参考那边不会动；这一版把静息的 glyph 丢了，于是环是**加**上去的。

这与 P13-ck 是同一条规矩的两个实例：那一轮修的是「两档画两套内容、而这一行居中」，
这一轮修的是「两档画两套内容、而这一行左对齐」——**内容变了，位置就得变**，除非槽先留好。

## 现状（先量，实测）

| 状态 | 内容 | 词盒 x | 那一格最左墨列 | 胶囊左沿 |
|---|---|---|---|---|
| 静息 `actionButton` | 词 | 552.0 | **没有墨** | L0 |
| 在飞 `busyActionButton` | 环 10px + 5px + 词 | 552.0 | 545 | L0 + 15 |
| 另一操作在飞 `disabledActionButton` | 词 | 552.0 | 没有墨 | L0 |

**量出来的形状与预想不同，照实记**：胶囊是这一行最右的控制、**右沿钉住**，所以多出来的
15px 不是把词往右推，而是把**左沿**往左推 15px——词盒 x 两臂都是 552.0。手指按着的那条
边因此是「会动的那条边」。实测值由 `repairjump_test.go` 打印，深浅各一遍。

## 落地项

1. `internal/settingsui/rows.go` `actionSpinner`：环 **2.5u (10px) → 3.5u (14px)**。
   理由不是「大一点好看」，是**槽要对齐**：静息 pill 的 glyph 是 `Size(Space(3.5))`
   （`extensionGlyphActionButton`），在飞的环必须占同一个盒子，否则即使补了 glyph 也还差 4px。
   `busySwitchControl` 的轨道是 9u×5u（36×20px），14px 的环在里面仍居中，不越界。
2. `internal/settingsui/rows.go` `repairAction`：静息臂与「另一操作在飞」臂都改成
   **戴 glyph 的 pill**（`extensionGlyphActionButton(c, "refresh", label, Space(7.5),
   Space(3.75), Body, disabled)`）——这就是参考里那枚 RefreshCw：两臂的语义都是「再跑一次
   搜索」（`repairTooltip` 的注释：`reconnectUnboundHint` 说的是搜索会重跑），所以一个 glyph
   对两臂都成立。
   `disabled` 在**建造时**传入（P13-ba 的规矩：先建后 `.Disabled` 会留下亮着的冠线），
   另外补 `.Disabled(true).Cursor(ui.CursorDefault)`——`extensionGlyphActionButton` 只把
   `disabled` 传给材质，不设框架的禁用位，所以这一行必须自己补。
3. **不动的**：`actionButton` / `disabledActionButton` 的其它 ~10 个调用点（它们没有在飞臂，
   加 glyph 就是给每一个 pill 添墨）；`extensionActionFace`；pill 的高度 / 圆角 / 字号 /
   padding / gap；`repairBusy` 的生命周期与 `rowBusyExceptRepair` 的语义。

## 行为契约零改动红线（逐条对照）

- 高度链：pill 的 `MinHeight(Space(7.5))` 不动；行高由 `cardRow` 的地板定，不随 pill 内容变。
- 键盘模型 / 焦点：`ui.ButtonBase` 不变，`Disabled` 位与 P13-ba 一致（原本也在）。
- 会话管理 / 设置写回 / 命令接线：`RepairIntegration` / `ReconnectIntegration` 的调用与
  `done` 回调一字不改。
- i18n 文案：**零改动**（`IntegrationsRecheck` / `IntegrationsReconnect` 原样）。

## 门槛命令 + PASS 基线（HEAD `60850c01` 实测）

```
gofmt -l cmd internal                     # 必须空
go vet ./...                              # 必须净
GOOS=windows go vet ./...                 # 必须净
GOOS=darwin  go vet ./...                 # 必须净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3260
GOOS=linux/windows/darwin go build ./...  # 必须过
```

基线：**3260 PASS / 0 FAIL / 130 SKIP / 2414 测试函数**。

## mutation 计划

- **M1**：`repairAction` 的静息臂退回 `a.actionButton(c, label)`（去掉 glyph 槽）
  → ① 臂红（词盒 x 差 15px）。
- **M2**：`actionSpinner` 退回 `Size(t.Space(2.5))`（环比槽小 4px）
  → ① 臂红（差 4px）。
- **M3**：`repairAction` 静息臂戴 glyph 但 `.Disabled` 挪到建造之后
  （`button.Disabled(true)` 在 `extensionGlyphActionButton` 之后对 disabled 臂重复一次）
  → 该臂行为不变，预期**不红**——这条是**反向对照**，用来证明 M1/M2 的红确实来自几何，
  不是来自「按钮被禁用」这类副作用。跑完照实写进报告。
