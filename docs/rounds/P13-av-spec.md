# P13-av 规格 · 多选列表箱的 56px 视口：两端不该是平刀切

**这轮为什么让它更轻、更有空间感**：`docs/liquid-glass.md` §6 已经没有 `未开始` 行，选题回
通用队列的「满态/滚动收口」这一族。这一块是族里最后一块**两端都硬**的面——66px 的
`select[multiple]` 地板减掉 4px padding 与两条 1px 发丝，滚动区只有 56px，而一行选项是
20px 盒子 + 2px 缝：六项以上的列表必然有一行被 `.Clip()` 水平切断，两端什么都没有。
给一个「有边界的视口」补上它两头该有的退让，是空间感里最便宜也最有效的一格：内容不再是
被刀切开的，而是**知道自己装在一个盒子里**。

## 意图与证据

| 事实 | 读数 | 出处 |
|---|---|---|
| 视口高度 | 66 − 2×1（发丝）− 2×4（padding）= **56px** | `internal/settingsui/config.go` `configMultiselectBox` |
| 一行选项 | 20px 盒 + 2px 缝 | `configOptionRow` 的 `MinHeight(cardSpace(5))` |
| 改前两端 | 视口最后四行的墨展开 **111/111（浅）= 100%**、**86/86（深）= 100%**，框外一行立刻 0 | §6.18 原记（900×1400 Integrations 页） |
| 出问题的位置 | `.Clip()` 裁在 border box，内容在视口第一行/最后一行像素上被切断 | 与 §6.17 同一族 |

## 逐条落地项

行号按本轮实测（`git rev-parse HEAD` = `593c7888`）。

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `config.go` `configMultiselectBox` 的 `box.Children` | 只有 `ui.Scroll`，视口两端无退让 | 两端各一块 `glass.ScrollEdge`，边高 **3u(12px)** |
| 2 | 边的门 | — | `scroll.Y > 0` 画顶、`scroll.Y < scroll.MaxY` 画底；`scroll == nil` 时两边都不画 |
| 3 | 边的锚 | — | `Absolute().Top(padY)` / `.Bottom(padY)`——绝对定位从 **padding box** 起算，`Top(0)` 会落在箱子的内边距上 |
| 4 | 边的底色 | — | `theme.FaceOver` 压「窗口地 → 面板 `Sunken` → 卡 `Control` → 箱 `Control`」，**不透明** |

### 与 §6.18 原计划的偏差（本轮实测推翻）

- 原计划边高 **2u(8px)**，理由是「4u 要吃 56px 的 29%」。实测 2u 时最坏读数
  顶 **44/106（41%）**、底 **67/106（63%）**——下沿压不到目标。取 **3u(12px)**
  （与 §6.16 / §6.17 同高）后降到 **35/106（33%）** 与 **50/106（47%）**。
  12px 占 56px 的 21%，且小于一行 20px，所以最多只软化一行的边缘。
- 原计划没写锚点。P13-au 记的「顶边那一格一个像素都没画出来」根因就在这儿：
  `Top(0)`/`Bottom(0)` 把边画进了箱子的 4px 内边距里，下沿那 8px 整个落在空白上
  （实测下沿仍 106/106 = 100%）。

## 行为契约零改动红线

- 高度链：66px 地板与 56px 视口**逐像素不动**（pin：`TestTheMultiselectBoxGeometryIsUntouched`）。
- 滚动状态：`configListScrolls` 的 key 与生命周期不动；`TrackScroll` 零改动。
- 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：本轮零改动（只加两条绘制层）。
- `scroll == nil` 的调用点（`configform_test` 的空滚动探针）必须继续可用——不改测试。

## 门槛与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                  # 空
go vet ./...                           # 净
GOOS=windows go vet ./...              # 净
GOOS=darwin  go vet ./...              # 净
go test -count=1 ./...                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3079 → 3091
GOOS={linux,windows,darwin} go build ./...          # 三平台过
```

## 变异计划

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | 边的锚回到 `Top(0)` / `Bottom(0)`（落进 padding） | `TestTheMultiselectBoxFadesAtBothEdges`（下沿 75%） |
| M2 | 顶边的门改成 `scroll.Y >= 0`（常亮） | `TestTheMultiselectEdgesAreGatedOnTheOffset`（开头的 A/B 无差） |
| M3 | `configListEdgeFace` 直接返回 `tokens.Control`（薄罩当底色） | `TestTheListBoxEdgePaintsItsOwnFace`（Δ=−7 / +8 级） |
