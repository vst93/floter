# P13-cc 规格：字段材质只有一处决定（三个绕开 `fieldFace` 的字段面收进来）

## 意图

把「设置页 / 弹层里一块**能打字的槽**长什么样」这件事，收成**一个**函数说了算：
`App.fieldFace`（外圈边作为参数）。今天有**三处**字段面自己拼材质、绕开它，
所以「字段只有一种面」这句话只是注释里的说法，不是代码里的事实。

为什么现在做：这是 P13-cb「仍待」第 1 条（上一轮自己量出来的那一格）。
下一次谁去动受光边的定值（`LensRim`/`LensEdge`）或字段的面档，漏掉这三处 =
同一个面板里出现两种字段材质——那正是用户说的「平、糊、旧」的来源
（层级不由面差说，而由谁记得改谁说）。

## 证据（HEAD = c015638c，行号已核）

| 位置 | 现状 | 与 `fieldFace` 的差 |
|---|---|---|
| `internal/settingsui/textinput.go:183` | `aliasField` 自己写 `Background(tokens.Control).Border(1, tokens.Hairline)` 再 `tokens.LitFace(...)` | 唯一合法差：外圈是 `Hairline`（参考的 `border: 1px solid var(--hairline)`） |
| `internal/settingsui/pluginsheet.go:359-364` | `sheetFieldSlot` 自己写 `Border(1, tokens.ControlEdge).Background(tokens.Control)` 再 `tokens.LitFace(...)` | 零差——纯复制 |
| `internal/settingsui/config.go:871-886` | `configMultiselectBox` 自己写 `Border(1, tokens.ControlEdge)` + `Background(tokens.Control)`，再 `tokens.LitFace(box)` | 零差——纯复制 |

`fieldFace`（`textinput.go:83-107`）已经是唯一带**状态**的那一处（`lit bool`，
P13-cb 的 `DimFace` 臂）。三处绕行者全都没有状态臂，将来给多选框加锁定
（`configMultiselectBox` 已经有 `locked` 参数、`lockedControl` 已经包着它）时
必然再漂一次。

## 落地项

1. `textinput.go`：拆出 `fieldFaceEdge(c, e, lit, edge)`——面 `Control`、外圈 `edge`、
   `lit` 决定 `LitFace`/`DimFace`；`fieldFace` 变成传 `tokens.ControlEdge` 的壳。
   **`lit` 的语义、`DimFace` 的算术一个字节不动**（P13-cb 的成果不重开）。
2. `textinput.go` `aliasField`：几何（`MinHeight(Caption+2*padY)`、`Padding`、`Radius`、
   `FontSize(Caption)`）一行不动，材质改走 `fieldFaceEdge(..., true, tokens.Hairline)`
   ——**外圈仍是 `Hairline`**（参考的边框选择，P13-m 已定），不借这次收口改墨。
3. `pluginsheet.go` `sheetFieldSlot`：删掉自己那行 `Border`/`Background`，改
   `a.fieldFace(c, slot, true)`。几何（28u、`RadiusSM`、8u 侧 padding）不动。
4. `config.go` `configMultiselectBox`：删掉自己那行 `Border`/`Background` 与行尾的
   `tokens.LitFace`，在 `Clip()` 之后改 `a.fieldFace(c, box, true)`（`LitFace` 原本就
   在 `Clip()` 之后，内阴影顺序不变）。66u 视口、`border` 记账、`ScrollEdge` 不动。
5. 新 pin `internal/settingsui/fieldcensus_test.go`：
   - **A**：字段族六个绘制函数（`textInputLit`/`extensionFieldLit`/`configSelect`/
     `configMultiselectBox`/`sheetFieldSlot`/`aliasField`）的函数体里必须出现
     `a.fieldFace(` 或 `a.fieldFaceEdge(`，且**不许**出现 `tokens.LitFace`/`tokens.DimFace`
     （AST 扫函数体，不是单行 grep——P13-i/l 的教训）。
   - **B**：`fieldFaceEdge` 是本包里唯一同时调用 `LitFace` 与 `DimFace` 的**字段**函数
     （白名单：`rows.go`/`uninstall.go` 那两处非字段的双臂各自带理由）。
   - **C**：`aliasField` 的外圈参数是 `tokens.Hairline`（像素上：边框列与 `ControlEdge`
     不同值才算通过——若两者实测同色则退成源码断言并写明）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。
本轮只改「谁拼这块面的材质」，不改任何墨的数值、任何盒子、任何状态。

## 门槛

```
gofmt -l cmd internal          # 空
go vet ./...                   # 净
GOOS=windows go vet ./...      # 净
GOOS=darwin  go vet ./...      # 净
go test -count=1 ./...         # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # ≥ 3228（HEAD 实测基线）
三平台 go build ./...
```

## mutation 计划

- **M1**：`fieldFaceEdge` 的 `lit==false` 臂退回 `LitFace` → `lockedfield_test.go` 与
  `lockedform_test.go` 的产品路径断言应变红（`DimFace` 熄灯那条）。
- **M2**：`aliasField` 改传 `tokens.ControlEdge`（把合法差抹掉）→ A 臂源码 pin 与
  （若可测）C 臂像素 pin 应变红。
- **M3**（若时间允许）：`configMultiselectBox` 退回自己拼材质并去掉 `LitFace` →
  `TestTheMultiselectBoxIsAFaceNotARing` 变红。
