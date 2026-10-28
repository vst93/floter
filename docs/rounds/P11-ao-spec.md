# P11-ao 规格 · 换一个设置页要从顶上开始（设置页的滚动归这一页所有）

## 意图

设置页换一块面板时，阅读位置应回到这一页的顶上；现在整个设置面只有**一个**滚动状态，
从长页（General）滚到一半再点 Shortcuts/About，新页面**带着上一页的偏移打开**——标题被
滚出可视区，用户看到的是半截列表。这一条属「交互逻辑」，不需要任何拍板。

## 为什么现在做

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 只剩
`⌘K`＝拍板）→ 上一轮「仍待」第 1 条（终端投影跟不跟 `terminal_opacity`＝拍板）→
**界面质感与交互**。P11-ak 把这一档里四条「凭猜会改错」的实测否掉了，P11-an 修了投影
字面值；本轮在这一档里找到的是**代码与参考各说一套**的一条：

- 参考把页面内容挂在一个按页码取 key 的滚动容器上：
  `main:src/App.tsx:2930` `<main className="settings-content" ... key={settingsPage}>`，
  而 `.settings-content` 自己就是滚动容器（`main:src/styles/settings.css:233-244`，
  `overflow-y: auto`）。`key` 一变整棵子树重挂 → 新页面的 `scrollTop` 是 0。
- 本仓五个页面共用一个滚动器：`internal/settingsui/settingsui.go:1426`
  `ui.Scroll(c.Key("settings.body")).TrackScroll(&a.Body)`；`selectPage`
  （`internal/settingsui/settingsui.go:1379`）只换 `a.Page`，**从不碰 `a.Body`**。
- 实测（`renderIn` 720×420，滚轮 240）：General 的 `a.Body.Y = 240`；点 Shortcuts 后
  `a.Page = 2` 但 `a.Body.Y` 仍是 **240**，内容区第一行 `Show / hide floter` 被顶到
  y=51（整页标题在视口上方），About 页则被夹到 `Y = 6.1`（顶部的滚动留白也没了）。

## 逐条落地

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/settingsui/settingsui.go:1379` `selectPage` | 换页只写 `a.Page`，滚动偏移跨页保留 | 页码**真的变了**时把 `a.Body.Y` 归零（同一个页码重复点击不动，等于参考的 key 没变、不重挂） |

不新增 token、不改几何、不改文案、不改键盘模型。

## 零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一个字节不动**。
本轮的「面」是行为，不是像素。

## 门槛（本轮实测基线）

```
gofmt -l cmd internal        空
go vet ./...（含 windows / darwin）  净
go test -count=1 ./...       全绿
--- PASS 计数                本轮实测
三平台 build                 过
```

## 验收（修前红 / 修后绿）

新增 `internal/settingsui/pagescroll_test.go` 的 `TestANewPageStartsAtTheTop`：

1. General 上滚轮 240，断言 `a.Body.Y > 0`（前提：确实滚动了）；
2. 点侧栏 `Shortcuts`（真实 UI 路径，不是直接调 `selectPage`），帧；
3. 断言 `a.Body.Y == 0`，且 `Show / hide floter` 这一行的 y 与**一个从没滚动过的新 App**
   直接打开 Shortcuts 时**相等**（读矩形，不读字面数）。

修前第 3 步红（`Body.Y = 240`、行 y=51.4 vs 132.2），修后绿。

## mutation 计划

1. 把 `a.Body.Y = 0` 删掉 → `TestANewPageStartsAtTheTop` 必须红（`Body.Y = 240`）。
2. 去掉 `page != a.Page` 的条件（每次点击都归零）→ 新 pin 仍绿，但「点当前页不跳顶」
   是参考的 key 语义；用它去改 `TestClickingTheCurrentPageKeepsItsScroll`（本轮新增的第二条
   pin）——删除条件会让它红。
