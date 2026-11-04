# P13-fx 规格：满态第三格 —— 集成页 24 张集成卡

## 意图

把「满态」（列表比窗口长）这一族收口：会话页（§6.141 / P13-fv）、命令列表
（§6.142 / P13-fw）之后，第三格是集成页的**已装集成列表**——装二十几个扩展的机器
上它才是这一页最常见的样子，而此前这一页只量过一张卡。

为什么现在做：P13-fw 报告「仍待」第 1 条给出了两条路——`140-5` **需拍板**，
替代路径 = 满态第三格。本轮走替代路径。

**不是**新功能弧，**不是**换设计语言：零像素改动，把「已经能用的表面」上的
节奏与缝隙量成 pin。

## 现状（动手前核过的行号）

- `internal/settingsui/settingsui.go:1838` `integrations()`：hint → 各分组 →
  `extensionSection(…, len(integrations), …)` 里 `for … a.integrationRow`。
- `internal/settingsui/settingsui.go:2707` `integrationRow()` → `a.cardGeometry(...)`
  → `a.cardRow(...)`：每张卡是**独立的一张卡**（各有自己的 1px 边 + 受光冠）。
- `internal/settingsui/rows.go:262` `extensionSection` → `sectionGroup` →
  `sectionGroupPadded` 的 `Gap(Space(2))` = 8：卡与卡之间的那一段。
- `internal/settingsui/rows.go:269-274` 组的计数胶囊（`MinWidth` 6.25u）。
- `internal/settingsui/settingsui.go:1614 / 1626` 页面滚动两端 `glass.ScrollEdge`。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
| --- | --- | --- | --- |
| 143-1 | 24 张卡的 `Find` y | 无 pin | 节奏 = 187.00 逐值（23 个间隔） |
| 143-2 | 卡与卡之间（中轴） | 无 pin | 8 行页面的面，且被两张卡各自的边夹住 |
| 143-3 | 滚到 `MaxY` 的下沿 | 无 pin | 最后一张卡下边仍在；脚下整条带子 = 页面的面 |
| 143-4 | 组的计数 `24` | 无 pin | 与标题同一中心线、在右、不被切 |

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线 **一律不动**；
本轮只加测试文件（`fullstateintegrations_test.go`），生产代码零改动。

## 门槛命令 + PASS 基线（实测当前值）

```
gofmt -l cmd internal                  # 空
go vet ./...  / GOOS=windows / GOOS=darwin   # 净
go test -count=1 ./...                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3434（P13-fw 实测基线）
```

## mutation 计划

- **M1**：`rows.go:103` 的 `Gap(Space(2))` → `Space(0)`。会让 143-1 的节奏变 179.00
  与 143-2 的「8 行页面面」变 0 行（两臂各 23 处红）。
- **M2**：`rows.go` 把组的计数画成 `count+1`。会让 143-4 找不到 `24`（两臂各 1 处红）。
- （另跑一条**预期不红**的 M2'：下沿软边 `Background: face` → `t.Background`，
  用来证明 143-3 是边界不是软边配色的守卫——见报告。）
