# P11-bi 报告 · 编号家族对**每一条能滚的列表**都读视口

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完；`Super+N` 编号渲染由 P11-bb
收过一轮）→ 用户本轮**再次报的同一条**：搜索列表的 `Super+1…0` 在滚动后仍钉在屏幕外的行上
→ 界面质感与交互逻辑（用户 2026-10-28 定的优先档）。规格 `docs/rounds/P11-bi-spec.md`。

## 1. 根因（P11-bb 的闸门把「恰好十项」漏掉了）

P11-bb 把「行是否读得全」从框架的 `List.Visible()`（任何一像素）换成了
「行盒 ∩ 列表视口 ≥ 70%」，但**只在 `resultCount > maxResultSlots`（> 10）时才读视口**：

```go
// 旧（P11-bb）
first, last := 0, len(results)-1
if resultCount > maxResultSlots {
    first, last = 0, maxResultSlots-1
    if visibleFirst, visibleLast, ok := a.numberedViewport(len(results)); ok { … }
}
```

`resultCount` 只数非 action-bar / 非 bare-terminal 行。本仓的窗口把搜索字段的浮带
（`fieldRowUnits + breathUnits` = 60u）算在列表上方，所以**恰好十项**的列表也能滚 17–19u；
而一条普通应用查询还会带一条 action bar 行，于是 `len(results) == 11`、`resultCount == 10`，
闸门 `10 > 10` 为假——**视口一次都没读**。

实测（720×488，10 个 `App %02d`、`Query="app"`，滚到顶行 53% 在带子底下）：

| 修前 | 修后 |
|------|------|
| `numbers = map[0:1 1:2 2:3 … 9:0]`，row 0 `share 0.53` 仍领 `1` | `numbers = map[1:1 2:2 … 9:9]`，row 0 无号 |
| `Super+1` 跑屏幕外那一行 | `Super+1` 跑第一行读得全的行 |

这正是用户原话「超出屏幕的项也显示快捷键」。

## 2. 落地

`internal/launcher/launcher.go` 的 `assignNumbers`：

- 删掉 `resultCount` 计数与 `resultCount > maxResultSlots` 闸门；
- 只要上一帧留下几何（`numberedViewport` 返回 ok）就读视口，否则回落到
  `0…len-1`（循环的 `count > maxResultSlots` 本来就把家族封顶在 10 枚）；
- 函数上方的注释改写：说明本仓「十项列表也能滚」这条本仓事实，并保留
  「首帧无几何时回落整段，避免 mid-walk 只剩一枚徽章」这条旧教训。

**形状零改动**：`1..9, 0`、action-bar 不领号、bare terminal 不领号、封顶 10 枚、
70% 阈值（P11-bb 的用户阈值）、上一帧几何（一帧滞后）全部原样。

## 3. 台账（V/B/S）

- **V = 0**：没有旧 pin 钉过「十项列表按行序编号」。
- **B = 0 / S = 0**：高度链、键盘模型、会话、设置写回、i18n、命令接线零改动。
- 既有测试**零改动**全绿。新增 pin `TestATenRowListRenumbersWhenItScrolls`
  与 opt-in dump `TestATenRowListDump`。

## 4. 变异（2 条实跑变红 + `md5sum -c` 还原 + `touch` 复跑绿）

| # | 注入 | 红在哪（原始报错串） |
|---|------|----------------------|
| M1 | 把 P11-bb 的 `resultCount > maxResultSlots` 闸门原样加回 | `row 0 is numbered 1 while only 0.53 of it is on screen` / `row 0 carries 1 while only 0.53 of it is on screen` |
| M2 | `first < 0 && share >= numberVisibleShare` → `share > 0` | `row 0 carries 1 while only 0.40 of it is on screen` + P11-bb 的 `TestTheNumberedFamilyStartsOnARowYouCanRead` 同时红 |

每条都 `cp` 还原后 `md5sum -c /tmp/p11bi.md5` 逐字节核对（`026ff53b…`），
`touch` 测试文件再跑一次绿。

## 5. 视觉证据（自己复核了像素）

- `/tmp/floter-P11-bi-tenrow-{light,dark}-before.png`（注入旧闸门，隔离重建）
- `/tmp/floter-P11-bi-tenrow-{light,dark}-fixed.png`

`compare -metric AE`：深浅两外观各 **1047 px** 差异；差异包围盒
`49x369+655+56`——正是右侧徽章列、跨 9 行（x 655…704，y 56…425）。
**我在图上量到的**：修前 row 0 的徽章列有一枚 `1`（落在 y≈61 的带上沿，
已被字段带压掉一半），修后那一列从 row 1 起才出现第一枚 `1`；其余徽章整体上移一行。

## 6. 门槛（实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # FAIL 0
PASS 2444（基线 2443）| SKIP 45（44 + 1 条 opt-in dump）| 测试函数 1991（1989 + 2）
GOOS=linux/windows/darwin go build ./...   # 过
```

## 7. 仍待（P11-bi）

1. 老拍板项原样有效：搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 的便条投影 / P11-bd 的「未揭示时窗口拖拽手柄接不接指针」/
   P11-be 的 `Permissions` 行「标签与胶囊同一个词」/ P11-bf 的 `configDiscardBar` 12u margin /
   P11-bg 的顶边失败带覆盖画布顶部两行。
2. **同族但不同表面（本轮未做）**：捕获输出页的 `assignOutputNumbers`
   （`internal/launcher/output.go:826`）仍用 `OutputList.Visible()`（任何一像素）+
   `len(rows) > maxResultSlots` 闸门。它没有搜索字段那条浮带，但「行只露 1px 也领号」
   的老判据还在。用户报的是**搜索视图**，本轮不动它；要修得先给它一套和
   `rowRects`/`listRect` 同形的几何。
3. **侦察结论（量过，别当 TODO）**：`numberedViewport` 读上一帧几何，所以快速滚动时
   编号比行慢一帧；70% 阈值就是为这个滞后留的余量。改成同帧需要框架暴露行矩形。
