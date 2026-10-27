# P10-x 规格：重复内容的折叠——收藏的条目不能被删掉，新条目继承收藏标记

## 意图

一句话：**同一份内容再复制一次时，「折叠」只允许吞掉没有收藏的旧行；收藏过的旧行原地留下，收藏标记跟着新行到顶部。**

为什么现在做（证据）：

- `internal/clipboard/clipboard.go:553-563` `insertLocked` 的判据是
  `existing.Kind == entry.Kind && existing.Hash == entry.Hash` → 命中就 `continue`（丢弃）。
  **它不看 `existing.Favorite`**。于是「收藏一条老内容 → 之后又复制到同样的内容」会把那条
  收藏行从历史里**静默删掉**：`Favorite` 字段（`clipboard.go:85`）在 UI 上是有入口的
  （`internal/launcher/filters.go:295-296` 的 favorites 过滤、`internal/launcher/fieldkeys.go:97`
  的 `pendingFavorite`），用户钉住的东西凭空消失。
- 参考 `main:src-tauri/src/clipboard_history/store.rs:206-217` `fold_capture` 写得很死：
  `entries.retain(|entry| !matches(entry) || entry.favorite)`（收藏的旧行**留下**），
  然后 `capture.favorite = carries_favorite`（新行**继承**标记）再 `insert(0, capture)`。
  参考自带的四条测试就是这条契约的原文：`duplicate_at_top_is_a_no_op_regardless_of_favorite_state`
  (store.rs:598)、`duplicate_older_elsewhere_moves_to_top_with_fresh_timestamp` (store.rs:614)、
  `favorited_duplicate_survives_and_carries_its_flag_to_the_top` (store.rs:634)、
  `a_hash_match_only_touches_entries_of_the_same_content` (store.rs:655)。
- 本仓这条路径是**三个入口共用的一个函数**（`AddText` / `AddImageCaption` / `addFilesEntry`
  都只调 `insertLocked`），所以这是「一族同源 bug」：文字、图片、文件列表三种条目一起错，
  一次修完。

## 逐条落地项

1. `internal/clipboard/clipboard.go` `insertLocked`（现 553-563 行）
   - 现状：命中同 kind + 同 hash 的旧行一律丢弃，新行 `Favorite` 恒为 `false`。
   - 应改成：先扫一遍同 kind + 同 hash 的旧行，**任一条是收藏**就把新行的 `Favorite` 置 `true`；
     再折叠时**只丢非收藏的**同内容旧行，收藏的留在原位。
2. 同一函数改成返回「真正落库的那个 `Entry`」（`func (s *Store) insertLocked(entry Entry) Entry`），
   三个调用点改成 `entry = s.insertLocked(entry)`：否则 `AddText` 等返回给调用方的 `Entry`
   的 `Favorite` 会是 `false`，而库里那条是 `true`——同一个值两个答案。
3. 新增测试（`internal/clipboard/clipboard_test.go`），逐条对参考的四条不变量：
   - 顶部已是同内容：无论收藏与否都是 no-op（旧行的 id / 时间戳 / 收藏都不动）；
   - 同内容的老行在中间：搬到顶部、时间戳刷新、旧的非收藏行消失；
   - 同内容的老行是**收藏**：它原地留下，新行到顶且 `Favorite == true`（**修前红**）；
   - 命中只碰同内容：不同 hash 的行一条不动；
   - 三种 kind 各走一遍第 3 条（`AddText` / `AddImageCaption` / `AddFiles`）——共用
     `insertLocked`，但三个入口的 hash 来源不同，各自钉一次。

## 行为契约零改动红线

- 不改：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线、剪贴板写入面
  （`copyClipboardEntry` 三臂）、监听器（`captureClipboard` / `selfCopied`）。
- 只改「同内容折叠时旧行怎么处理」这一条判据；`pruneLocked` 的收藏豁免、容量上限、
  30 天窗口一律不动。

## 门槛命令 + PASS 基线

基线（HEAD `0a37994` 实测，P10-w 报告）：`--- PASS` **2195** / `--- SKIP` 15 / `--- FAIL` 0 /
测试函数 **1771** / `go test` 行数 36（34 ok + 2 no test files）。

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # ≥ 2195，只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

本轮非视觉，不出 PNG。

## mutation 计划

1. 把 `insertLocked` 的折叠条件改回「不看 Favorite」（`... && !existing.Favorite` 删掉）→
   `TestFoldKeepsTheFavoritedDuplicateAndCarriesItsFlag` 与
   `TestFoldKeepsFavoritesForEveryKind/{text,image,files}` 必须变红。
2. 把 `entry.Favorite = true` 改成 `entry.Favorite = entry.Favorite` → 「新行继承标记」断言必须变红。
3. 把折叠条件里的 `existing.Hash == entry.Hash &&` 删掉（任何同 kind 的旧行都被吞）→
   `TestFoldOlderDuplicateMovesToTopWithFreshTimestamp` 与 `TestFoldKeepsFavoritesForEveryKind`
   的条目数断言必须变红。

## 实测（本轮填回）

- 门槛：gofmt 空 · vet 净 · `--- PASS` **2203**（基线 2195，+8 = 5 个新测试函数 + 3 个 subtest）·
  `--- SKIP` 15（=）· `--- FAIL` 0 · 测试函数 **1776**（+5）· `go test` 行数 36（=）·
  linux/windows/darwin `go build ./...` 均过。
- 变异 3 条全部实跑变红，`md5sum -c` 逐字节还原后 `touch` 复跑 package 绿。
  台账 **V=0 / B=0 / S=0**（新测试全是新增，没有任何旧断言需要重定基线）。
