# P11-bi 规格 · 编号家族对**每一条能滚的列表**都读视口，不只读「超过十项」的列表

## 意图

用户的 `Super+N` 快捷键必须只对**读得全**的可见行生效。上一轮 P11-bb 把判据从
「框架的 `List.Visible()`（任何一像素在框里）」换成了「行盒 ∩ 列表视口 ≥ 70%」，
但**只在 `resultCount > maxResultSlots`（结果数 > 10）时才读视口**。这条闸门在本仓是错的：
窗口把搜索字段的浮带（60u）算在列表上方，所以**恰好十项**的列表也能滚，
`resultCount` 又只数非 action-bar 行，导致「10 个应用 + 1 条 action bar」也是 `resultCount == 10`。
这些列表滚起来后，`1` 仍钉在被字段带盖住的第一行（实测 720×488：`share 0.53`），
`Super+1` 跑的是屏幕外的行——正是用户原话「超出屏幕的项也显示快捷键」。

## 证据（本轮实测，`internal/launcher`）

- `launcher.go:619`：`if resultCount > maxResultSlots {` 是唯一的视口入口。
- 探针（10 个 `App %02d`、`Query="app"`、720×488）：滚动 6 步后
  `row0={X:8 Y:41 W:704 H:40}`、`listRect={X:0 Y:60 W:720 H:383}`、
  `share=0.53`，而 `numbers=map[0:1 1:2 … 9:0]`——`1` 还在 row 0 上。
- P11-bb 的 pin `TestTheNumberedFamilyStartsOnARowYouCanRead` 用 30 个结果，
  正好绕过这条闸门，所以没抓到。

## 落地项

1. `internal/launcher/launcher.go` 的 `assignNumbers`
   - 现状：`first, last := 0, len(results)-1`，仅当 `resultCount > maxResultSlots`
     才用 `numberedViewport`，否则按行序编号。
   - 改成：**只要上一帧留下了几何（`numberedViewport` 返回 ok）就读视口**；
     没有几何（首帧）回落到 `0…len-1`（循环本来就把家族封顶在 10 枚）。
   - 删掉 `resultCount` 变量与它的计数循环（不再有闸门），
     同步改写函数上方的注释，说明「本仓的十项列表也能滚」这条本仓事实。
2. 新增 pin `TestATenRowListRenumbersWhenItScrolls`（`numberband_test.go`）：
   10 个结果、滚动到行的 `share < 0.7` 后，**没有**任何 < 0.7 可见的行携带编号，
   且带 `1` 的行 `share ≥ 0.7`。把闸门加回去这条立刻红。
3. 视觉 dump `TestATenRowListDump`（opt-in `FLOTER_PNG=1`）：
   `/tmp/floter-P11-bi-tenrow-<dark|light>.png`，修前用隔离 worktree 出对照帧。

## 行为契约零改动红线

- 高度链（`height.go`）、键盘模型（`Super+N` 的解析与派发）、会话、设置写回、
  i18n 文案、命令接线**一律不动**。本轮只改「哪一行领号」这一个判据。
- 编号家族的形状（`1..9, 0`、action-bar 不领号、bare terminal 不领号、
  封顶 10 枚）不变。

## 门槛与 PASS 基线

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 只许涨（基线 2443）
GOOS=linux/windows/darwin go build ./...            # 过
```

## mutation 计划

1. 把 `if resultCount > maxResultSlots {` 那条闸门加回（恢复 `first=0,last=len-1` 除非超过十项）：
   → `TestATenRowListRenumbersWhenItScrolls` 红。
2. 把 `numberedViewport` 的 `share >= numberVisibleShare` 改成 `share > 0`：
   → P11-bb 的 `TestTheNumberedFamilyStartsOnARowYouCanRead` 与新的十项 pin 都红。
