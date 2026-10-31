# P13-aw 规格：插件配置浮层的两端——也是「滚不动」的那一处

## 意图

把 `pluginsheet.go` 里 `plugin.config` 那个 scroller 的两条边收口（「滚动收口」族的最后一块面），
并修掉它顺带照出来的一个更硬的问题：**这块浮层的 scroller 在 app 里根本没有视口，`MaxY` 恒为 0**
——schema 比窗口高时，末尾几个字段被窗口下沿切掉而且**滚不到**。

为什么现在做：`docs/liquid-glass.md` §6 已无 `未开始` 行，P13-at / P13-av 的「仍待」第 2 条
逐字点名 `pluginsheet.go:64`（浮层，§6.2 管辖），且它明确写着「同族最后一块，独立一轮」。
一行改动能同时对上总目标的两条轴：**空间感**（内容在带子两端退场，而不是被刀切）+
**细节打磨**（被切掉的尾巴第一次能被滚到）。

## 落地项（动手前逐条核过行号）

| # | 位置 | 现状（改前） | 改成 |
|---|---|---|---|
| 1 | `internal/settingsui/pluginsheet.go` `PluginSheet` | `ui.Scroll(...).FillWidth()` 直接作为调用方的子元素 | 包一层 `ui.Column(c).Fill()`，`Scroll` 改 `.Fill()`（与 `settingsui.go` 的 `body` 同一个写法：§6.16 那块面板的 scroller 就是这么拿视口的） |
| 2 | 同上 | 两条边都没有 | `Absolute().Top(0)` / `Bottom(0)` 两块 `Height(3u)` 的 `PassThrough` 盒子，材质 `glass.ScrollEdge{Background: sheetEdgeFace(...)}` |
| 3 | 新增 `internal/settingsui/sheetedge.go` `sheetEdgeFace` | — | `theme.FaceOver(t.Background, tokens.Sunken)`：浮层的面（`Sunken` 压窗口地）压成**不透明**。`ui.Color.Alpha` 是**乘**，把 0.17 的 `Sunken` 直接交给 `ScrollEdge` 会把 0.85 乘成 0.14（§6.17 / §6.18 的同一条） |
| 4 | `internal/launcher/config.go` `configBody` | `ui.Box(c).FillWidth()`——内容高度的父盒不给 scroller 可用高度 | `ui.Box(c).Fill()`。窗口高度链仍由 `Geometry.Height` 按 `Schema.Height` 算，一行没动 |
| 5 | `internal/settingsui/config.go` `configMultiselectBox` 注释 | 写着「① 边高只取 2u(8px)」，而代码是 `cardSpace(t, 3)`——P13-av 实测把 2u 推翻了，注释没跟上 | 改成 3u 并把实测理由写清（2u 时底边 63% 压不住） |

## 门的取舍（与 §6.16 / §6.17 / §6.18 的差别）

**用几何自门控，不用 `ScrollState.Y`**（§6.16 那条机制）：这块浮层两端本来就有留白——
顶边压在 scroller 的 1u 顶留白 + 标题行的行盒上身（实测最外 12 行在这块面上是空白），
底边的 3u(12px) 底留白与边高**逐像素相等**。所以停在开头、滚到底时正在读的那一行都不会被淡，
也不必读上一帧的偏移。§6.17 / §6.18 用 `Y` 门是因为那两处 padding 不许长。

## 行为契约零改动红线

高度链（窗口高度仍按 `Schema.Height`）/ 键盘模型（`configKeys`、字段自己持键盘）/ 会话管理 /
设置写回（`Change` / `Arm` / `Run` / `Unarm` 四路回调）/ i18n 文案 / 命令接线：**一律不碰**。
视觉轮只改墨与面。

## 门槛命令与 PASS 基线（改前实测）

```
gofmt -l cmd internal                      # 空
go vet ./... ; GOOS=windows/darwin go vet  # 净
go test -count=1 ./...                     # 全绿
PASS 3091 | FAIL 0 | SKIP 101 | 测试函数 2284
三平台 go build ./...                      # 过
```

## 门槛（改后目标）

PASS ≥ 3091、FAIL 0、SKIP 只许因新增的 opt-in 出图而 +1；测试函数不许减少。

## mutation 计划（每条都要看到指定的 pin 变红）

1. **M1**：wrapper 退回 `.FillWidth()` → `MaxY` 回到 0，三条 pin 全红（「does not overflow」）。
2. **M2**：两条边都不画 → `TestThePluginSheetFadesAtBothEdges` 读到 100%（228/228）。
3. **M3**：`face := tokens.Sunken`（薄罩直接交给 `ScrollEdge`）→ 边淡成 0.14，读到 90% / 87%。
4. **M4**：底留白 3u → 1u → `TestThePluginSheetEdgesSitOnBlankAtBothEnds` 的页尾 26 / 28 红。
