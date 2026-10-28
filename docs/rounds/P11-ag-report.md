# P11-ag 报告：空态的那句话必须在屏幕上

选题顺序：用户 2026-10-28 实测反馈（**界面质感与交互逻辑 + bug 优先**）→ 合约未落地条目
（§6 全表 `已落地`，只剩 `⌘K` 动作面＝拍板）→ 上一轮「仍待」第 1 条（同上）→
**界面质感与交互**。本轮取其中的一条真 bug：**空态句子被字段块整块盖住**，一个查询什么都
没匹配到时窗口是空白的。规格 `docs/rounds/P11-ag-spec.md`，合约 `docs/raycast-design.md`
§6.13。

## 1 缺口（根因）

`internal/launcher/launcher.go` 的空态分支（改前 `:1187`）：

```go
ui.Column(c).FillWidth().Padding(t.Space(3)).Center().Children(func() {
    ui.Text(c, a.emptyMessage(copy)).FontSize(tk.Emphasis).TextColor(tk.TextStrong).SingleLine()
})
```

面板（`ui.Box(c).Fill()`）是**整张卡**的高度，而字段行是浮在它上面的绝对定位块
（`launcher.fieldRow`：`.Absolute().Top(0).Height(fieldRow)`，`fieldRowUnits = 14` 即 56u，
`internal/launcher/height.go:23`）。空态这条列既没有 `Grow` 也没有顶部内缩，`Center()` 因此
把句子放到了**卡片顶边**——正是字段块的位置。

实测（`ui.Tester`，720×488，浅色，查询 `zzzz`）：

- 句子矩形 `{X:334 Y:12 W:52 H:18}`；字段带 y 0–56。整句在字段的底下面。
- 整帧非底像素 **562** 个，全部来自字段自己的字（放大镜/查询/按钮）；句子矩形内**零墨**。
- 图：改前的空白帧与本轮两张图同为 `/tmp/floter-P11-ag-empty-*.png`（改后）。

为什么既有 pin 没发现：`TestNoResultsShowsTheEmptyMessage` 与 `TestLauncherShowsTheEmptyState`
读的是 `tt.HasText` —— 构建出来的文本树里那句话一直在，**可见性**从来没被量过。
`internal/launcher/view_test.go:174` 甚至专门钉了「空页不画动作条」，语义是对的，像素没有。

可达性：普通页的「无匹配」是设计内的页面（不画动作条，页面自己说）；插件模式的空态更常见
——剪贴板为空、浏览器没配置、计算器没有历史，这些页面既没有列表也没有页脚，整页只剩这句
看不见的话。

## 2 改法

一条规则：**空态与列表读同一条带**。

```go
ui.Column(c).FillWidth().Grow(1).Center().
    Padding(listEdge, t.Space(3), t.Space(3), t.Space(3)).
    Children(func() { ... })
```

- 上边距 = `listEdge`（`fieldRow + breath + 模式的 chips band`），和列表自己的
  `Padding(listEdge, …)` 同一个数；`Grow(1)` 让这条列吃掉面板剩下的高度，`Center()` 于是在
  **列表会占据的那条带**里居中，而不是在卡片顶边。
- 墨与字阶零改动：仍是 §4.7 的 `Emphasis` + `TextStrong`，无底板、无图标、无 accent；
  高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线零改动。

## 3 台账

- **V=0 / B=0 / S=0**：没有旧 pin 钉过空态这块列的几何（`HasText` 类 pin 一个都不读位置，
  改后仍绿）。新增 pin（`internal/launcher/emptystate_test.go`）：
  - `TestTheEmptyPageSaysSoUnderTheField`：句子矩形里**墨量 > 0**（被盖住时为 0），
    且中心落在「字段带底 → 面板底 −3u 内缩」的中点 ±6px、矩形顶 ≥ 字段带底。
  - `TestDumpP11AGEmptyPagePNGs`（opt-in `FLOTER_PNG=1`，+1 SKIP）：深/浅两帧。
- 测试函数 1922 → **1924**；PASS **2357 → 2358**（+1 新 pin，另一条是 SKIP）；SKIP 23 → 24。

## 4 mutation（实跑，还原后 `md5sum -c` 逐字节核对 + `touch` 复跑绿）

- **M1**（回到改前的列：去掉 `Grow(1)`、上边距回 3u）——
  ```
  emptystate_test.go:58: the empty page's sentence carries no ink at {X:334 Y:12 W:52 H:18}:
  it is painted under the field block, so the window reads as blank
  ```
  红在**墨量**那一条：证明句子确实被字段盖住，不是「位置偏了一点」。
- **M2**（保留 `Grow(1)`，去掉 `listEdge` 内缩）——
  ```
  emptystate_test.go:68: the sentence is centred at y=244, want 268 (the band under the field)
  ```
  红在**居中**那一条：证明 pin 钉的是「带里居中」，不是「随便可见」。
- 还原：`md5sum -c /tmp/p11ag-md5.txt` → `成功`，`touch` 后复跑绿。

## 5 门槛（实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿（0 FAIL）
--- PASS 计数              → 2358（基线 2357）
--- SKIP 计数              → 24
三平台 go build ./...        → 过
```

## 6 图上实测（我自己在两张图里看到的变化）

- `/tmp/floter-P11-ag-empty-light.png`：查询 `zzzz` 的页面中央出现 `No results`
  （y≈268，整张卡的水平中线）——改前同一帧只有顶上的字段，下面一片空白。
- `/tmp/floter-P11-ag-empty-dark.png`：同一句话在深色面板中央是浅色字，位置相同；
  深色下也没有新的底板或分隔线（空态仍是纯文字）。

## 7 仍待

1. **`⌘K` 动作面**仍是合约里唯一未落地的条目（§4.6）：页脚画着键帽但 chord 没绑定。
   要新增表面 + 新 chord → **需拍板**（原样有效）。
2. 拍板清单原样有效（P11-aa 的默认浅色安静档、P11-z 的搜索框 caret、P11-y 的滚动缓冲、
   P9-g 表那一批、P11-ae 的 `CanvasAlpha`、P11-af 的浮条拖拽手柄大小）。
3. 侦察结论（别当 TODO 重领）：`TestNoResultsShowsTheEmptyMessage` 一类 `HasText` pin 保留
   ——它们钉文案，本轮新增的 pin 钉可见性，两者不重复。终端页与设置页的空态分别在面板/
   内容区里居中，实测可见（`/tmp/floter-probe-terminal-*.png`、`-settings-sessions-*.png`），
   不受这条 bug 影响。
