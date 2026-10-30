# P12-c 规格：设置页表面——E1/E2 的受光边落到这一张表面

**意图一句话**：把 `docs/liquid-glass.md` §6.2 的 8 行（2-1 ~ 2-8）在设置页面上收口——
分组卡、字段、侧栏选中页站起来（E1），卸载对话框与保存告警拿到 E2 的冠线。

**为什么现在做**：`§6` 是这一支的选题队列，第一个 `未开始` 的表面就是它（P12-b 的「仍待」第 1 条）。
证据（当前 HEAD `19be40c9` 实测）：

- `internal/settingsui/rows.go:303-306`：分组卡 `Border(1, HairlineSoft)` + `Background(Control)`，
  **没有顶边**；`internal/settingsui/raycast_test.go:405-408` 还钉着「top inner row 就是 face」。
- `internal/settingsui/textinput.go:74-77`：`fieldFace` 只有面 + `ControlEdge`，没有受光边。
- `internal/settingsui/pluginsheet.go:330-333`：弹层字段槽同上。
- `internal/settingsui/settingsui.go:1314-1322`：侧栏选中页 `RowSelected` 一块平涂；
  `raycast_test.go:172-178` 钉着「top inner edge 就是 fill」。
- `internal/settingsui/uninstall.go:169-170`：E2 浮层用的是 **E1 的** `LensRim` 冠线（0.09），
  `EdgeHighlight`（深 0.20）从来没上过这条浮层。
- `internal/settingsui/savealert.go:99-103`：`Float` 面，**无冠线、无影**。
- `internal/settingsui/rows.go:611`：开关 `LensInset` 凹槽——`LensEdge` 已随 P12-a 抬到 0.06，
  这一行只需**守住「凹槽不许有冠线」**。

## 落地项（位置 → 现状 → 应改成）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 2-1 | `settingsui.go:1249` 内容页 `Sunken` | 底 + `contentStep` | **不改**：它是底（E0），底不画面、不画线 |
| 2-2 | `settingsui.go` `sidebarRow` 选中臂 | `RowSelected` 平涂 | E1：`tokens.LitFace(row)` |
| 2-3 | `rows.go` `cardGeometry` | `Control` + `HairlineSoft`，无顶边 | E1：`tokens.LitFace(card)`（边仍是弱发丝，只加冠/对侧） |
| 2-4 | `textinput.go` `fieldFace`、`pluginsheet.go` `sheetFieldSlot` | 面 + `ControlEdge` | E1：两处同一个 `tokens.LitFace` |
| 2-5 | `rows.go` `switchControl` / `segmentedControl` | `LensInset` 凹槽 | **不改代码**：`LensEdge` 已 0.06（P12-a）；加一条**反面 pin**「凹槽不许有冠线」 |
| 2-6 | `rows.go` `extensionPrimaryButton` | 已有 `LensRim` 顶边 | **不改**；加 pin：它戴的是 E1 冠线（`LensRim`），不是 E2 的 `EdgeHighlight` |
| 2-7 | `uninstall.go` 卸载对话框面板 | `LensRim` 冠线（E1 的线用在 E2 上） | E2：`EdgeHighlight` 冠线（两层影 `ElevThrow` 已在） |
| 2-8 | `savealert.go` 保存告警 | `Float` 面，无冠线 | E2 冠线 `EdgeHighlight`；**不加影**（见下） |
| 2-9 | 状态 chip / 权限药丸 | — | `故意不做`（太薄，加线成噪点，P12-a 已定） |

**2-8 为什么只加冠线不加影**：这条带画在页面列里、随内容一起滚（`body()` 的 `Scroll` 内容），
不是浮层；`ElevThrow` 的 6.5u 宽投影会在滚动视口/列的内边距上被裁成一条硬边。
把它变成真浮层是**交互/布局改动**（不在这支的红线内），已写进 `.pending-bugs.md` 的「需用户拍板」。

## 红线（零改动）

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线、圆角/间距刻度（§4）。
只改墨与面：本轮**只加 1px 内线**，不新增面阶、不动半径、不动高度。

## 门槛（基线用实测当前值）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows / darwin go vet ./...   # 净
go test -count=1 ./...           # 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 2819，只许涨
GOOS={linux,windows,darwin} go build ./...
```

## mutation 计划（≥2 条，每条说明撞红哪个 pin）

1. `sidebarRow` 删掉 `LitFace` → `TestTheChosenPageStandsOnOneLitEdge` 红。
2. `cardGeometry` 删掉 `LitFace` → `TestTheGroupedCardIsAnE1Pane` 红。
3. `uninstall.go` 把 `EdgeHighlight` 换回 `LensRim` → `TestTheUninstallDialogWearsTheE2Crown` 红。
4. `savealert.go` 删掉冠线 → `TestTheSaveAlertWearsTheE2Crown` 红。
5. `switchControl` 的 `LensInset` 换成 `LitFace` → `TestTheRecessedGrooveHasNoCrown` 红（反面）。
