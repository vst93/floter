# P11-ao 报告 · 换一个设置页从顶上开始

## 1 意图

设置页换面板时，阅读位置回到这一页的顶上。参考如此（按页码取 key 的滚动容器），本仓
偏偏把五个页面塞进一个滚动器、只换内容不换偏移——从滚到一半的 General 点 Shortcuts，
新页的标题已经在视口上方。

## 2 证据（实测，先红后绿）

`internal/settingsui/settingsui.go:1426` 是唯一的滚动器
`ui.Scroll(c.Key("settings.body")).TrackScroll(&a.Body)`；`selectPage`
（`:1379`）换页时只写 `a.Page`。参考：

- `main:src/App.tsx:2930` `<main className="settings-content" ... key={settingsPage}>`
- `main:src/styles/settings.css:233-244` `.settings-content { ... overflow-y: auto }`

`key` 变 → 滚动容器重挂 → `scrollTop = 0`；同一个 key 不重挂 → 重复点当前页不跳顶。

修前实测（720×420，滚轮 240）：`Body.Y = 240`，点 Shortcuts 后 `a.Page = 2`，
`Body.Y` 仍 **240**；`Show / hide floter` 行 y=**51.4**，从没滚动过的新 App 直接打开
Shortcuts 时同一行 y=**132.9**。修后：`Body.Y = 0`，两处 y 相等。

## 3 落地

`selectPage` 在**页码真的变了**时把 `a.Body.Y` 归零；同一页重复点击不动（对应参考的
key 不变）。不新增 token，几何/文案/键盘模型零改动。

新增 `internal/settingsui/pagescroll_test.go` 两条 pin：

- `TestANewPageStartsAtTheTop`：滚 General → **点侧栏** Shortcuts → `Body.Y == 0`，且该页
  第一行的 y 与一个全新 App 直接打开同一页时**相等**（读矩形，不写字面数）。
- `TestClickingTheCurrentPageKeepsItsScroll`：同一页再 `selectPage` 不改变偏移。

## 4 台账

- **B**（行为）：本轮的产品改动就是行为本身；既有 pin 一条没改，必须全绿。
- V=0 / S=0：没有视觉数值或结构断言被重定基线。
- 函数数/ PASS 数：见下节实测。

## 5 门槛（实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿（0 FAIL）
--- PASS 计数              → 2376（基线 2374）
--- SKIP 计数              → 27
测试函数数                → 1941（基线 1939）
三平台 go build ./...        → 过
```

## 6 变异（实跑）

1. 删掉 `a.Body.Y = 0` → `TestANewPageStartsAtTheTop` 红：
   `the new page kept the old offset: Y = 240, want 0`；
   `the row sits at y=51.399998 after the page change, a fresh page draws it at y=132.9`。
   `md5sum -c` 还原成功，`touch` 后复跑绿。
2. 去掉 `page != a.Page` 条件（每次点击都归零）→ `TestClickingTheCurrentPageKeepsItsScroll` 红：
   `reopening the current page moved the scroll: Y = 0, was 240`。同样还原并复跑绿。

## 7 图

- `/tmp/floter-P11-ao-shortcuts-before.png`：从滚到底的 General 点进 Shortcuts，画面从
  `New command` 那一行开始，`Shortcuts` 页标题整条不在屏幕上。
- `/tmp/floter-P11-ao-shortcuts-after.png`：同一操作后从 `Shortcuts` 标题与一句副题开始，
  `Show / hide floter` 在标题下方第一段，滚动条回到顶部。
- 我实测看到的：两张图唯一的差别是**页面从哪一行开始**——before 少了标题与全局面板的
  说明，after 的布局与直接打开 Shortcuts 时逐行一致；没有新的颜色、边或底板。

## 8 仍待

1. 五条拍板项原样有效（caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1），另加 P11-an 的「终端便条投影跟不跟 `terminal_opacity`」。
2. 侦察结论（别当 TODO 重领）：同一设置面的**关闭再打开**是否也回顶，取决于参考的设置
   窗口生命周期（React 树是否卸载），本轮没证据，不要凭猜改。
