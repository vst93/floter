# P10-x 报告：重复内容的折叠——收藏的条目不能被删掉

## 1 本轮做了什么

方向：优先级 4（三平台一致性 → 基础功能 → 像素细节），落在「基础功能」。上一轮 P10-w
报告「仍待」第 7 条建议离开剪贴板族去核 `copyClipboardEntry` 的写回三臂；量完发现写回面
（`shell.go:2987-3035` + `systemClipboardWrite` 三条系统全走 `mygo.Clipboard`）**没有分歧**，
而顺着「复制回剪贴板」这条线往下走，落在同一族的另一处：**折叠**。

### 1.1 意图一句话

同一份内容再复制一次时，「折叠」只允许吞掉**没有收藏**的旧行；收藏过的旧行原地留下，
收藏标记跟着新行到顶部。

### 1.2 证据

- 修前 `internal/clipboard/clipboard.go:553-563`：

  ```go
  func (s *Store) insertLocked(entry Entry) {
      kept := make([]Entry, 0, len(s.entries)+1)
      kept = append(kept, entry)
      for _, existing := range s.entries {
          if existing.Kind == entry.Kind && existing.Hash == entry.Hash {
              continue        // ← 不看 existing.Favorite
          }
          kept = append(kept, existing)
      }
      s.entries = kept
  }
  ```

  命中同 kind + 同 hash 的旧行一律丢弃，且新行 `Favorite` 恒为 `false`。于是
  「收藏一条老内容 → 之后又复制到同样的内容」把那条收藏行**静默删掉**；而 `Favorite`
  在 UI 上是有入口的（`internal/launcher/filters.go:295-296` 的 favorites 过滤、
  `internal/launcher/fieldkeys.go:97` 的 `pendingFavorite`），用户钉住的东西凭空消失。
- 参考 `main:src-tauri/src/clipboard_history/store.rs:206-217` `fold_capture` 写得很死：
  `entries.retain(|entry| !matches(entry) || entry.favorite)`（收藏的旧行**留下**），
  再 `capture.favorite = carries_favorite`（新行**继承**标记），最后 `entries.insert(0, capture)`。
  参考自带四条测试就是这条契约的原文：store.rs:598 / 614 / 634 / 655。
- 三个入口共用这一个函数（`AddText` / `AddImageCaption` / `addFilesEntry`），所以这是
  「一族同源 bug」：文字、图片、文件列表三种条目一起错，一次修完。

## 2 落地

`internal/clipboard/clipboard.go`：

1. `insertLocked` 先扫一遍同 kind + 同 hash 的旧行，**任一条是收藏**就把新行 `Favorite` 置 `true`；
   折叠时**只丢非收藏的**同内容旧行，收藏的留在原位。
2. 签名改成 `func (s *Store) insertLocked(entry Entry) Entry`，返回真正落库的那个 `Entry`；
   三个调用点改成 `entry = s.insertLocked(entry)`。否则 `AddText` 等返回给调用方的
   `Entry.Favorite` 是 `false`，而库里那条是 `true`——同一个值两个答案。
3. 新 `internal/clipboard/fold_test.go`（5 个测试函数 + 3 个 subtest），逐条对参考的不变量：
   `TestFoldTopDuplicateIsANoOpRegardlessOfFavorite`、
   `TestFoldOlderDuplicateMovesToTopWithFreshTimestamp`、
   `TestFoldKeepsTheFavoritedDuplicateAndCarriesItsFlag`（修前红）、
   `TestFoldOnlyTouchesTheSameContent`、
   `TestFoldKeepsFavoritesForEveryKind/{text,image,files}`（修前红，三个入口各钉一次）。

**V/B/S 台账：V=0 / B=0 / S=0**——全部是新增断言，没有任何旧 pin 需要重定基线；既有测试
（含 P10-w 的文件列表身份 pin）一条未改、全绿。

## 3 门槛（全部实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
go test -count=1 ./...                                 → 全绿，36 行（34 ok + 2 no test files）
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2203   （基线 2195，+8）
                              grep -c -- "--- SKIP"    → 15     （=）
                              grep -c -- "--- FAIL"    → 0
测试函数数（grep -c '^func Test'）                     → 1776   （基线 1771，+5）
GOOS=linux/windows/darwin go build ./...               → 三平台均过
```

本轮非视觉，不出 PNG（剪贴板历史是列表/排序，没有可看的墨与面）。

## 4 mutation（3 条，全部实跑）

每条：注入 → 跑 `-run TestFold` → 看到红 → 从备份还原 → `md5sum -c` 逐字节核对
（输出「成功」）→ `touch` 再复跑（避开测试缓存）。原始报错串：

**M1 · 折叠不看 Favorite**（`existing.Hash == entry.Hash && !existing.Favorite` → 去掉尾部条件）

```
--- FAIL: TestFoldKeepsTheFavoritedDuplicateAndCarriesItsFlag (0.00s)
    fold_test.go:135: 2 entries, want 3 (fresh + world + the starred one): [...]
--- FAIL: TestFoldKeepsFavoritesForEveryKind (0.01s)
    --- FAIL: TestFoldKeepsFavoritesForEveryKind/text (0.00s)
        fold_test.go:250: the favorited text entry was folded away: [...]
    --- FAIL: TestFoldKeepsFavoritesForEveryKind/image (0.00s)
        fold_test.go:250: the favorited image entry was folded away: [...]
    --- FAIL: TestFoldKeepsFavoritesForEveryKind/files (0.00s)
```

**M2 · 新行不继承标记**（`entry.Favorite = true` → `entry.Favorite = entry.Favorite`）

```
--- FAIL: TestFoldKeepsTheFavoritedDuplicateAndCarriesItsFlag (0.00s)
    fold_test.go:131: the returned entry did not carry the flag the history has
    fold_test.go:138: top = {... Favorite:false ...}, want the fresh copy carrying the flag
--- FAIL: TestFoldKeepsFavoritesForEveryKind/text
        fold_test.go:245: the fresh text entry did not inherit the flag
--- FAIL: TestFoldKeepsFavoritesForEveryKind/image
        fold_test.go:245: the fresh image entry did not inherit the flag
```

**M3 · 折叠忽略 hash**（`existing.Hash == entry.Hash && !existing.Favorite` → `!existing.Favorite`，
即任何同 kind 旧行都被吞）

```
--- FAIL: TestFoldOlderDuplicateMovesToTopWithFreshTimestamp (0.00s)
    fold_test.go:93: 1 entries, want 2: [...]
--- FAIL: TestFoldKeepsTheFavoritedDuplicateAndCarriesItsFlag (0.00s)
    fold_test.go:135: 2 entries, want 3 (fresh + world + the starred one): [...]
--- FAIL: TestFoldKeepsFavoritesForEveryKind/text
        fold_test.go:256: 2 text entries, want 3
```

还原后 `go test -count=1 ./internal/clipboard/` → `ok`。

## 5 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 53 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表）：**#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）；register 只留 `cmd`、
   `open` 是否拒一切 query、未知宿主是否静默忽略。
3. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
4. icns 转换落盘缓存（P10-t）、SVG/XPM 图标（P10-s）仍是拍板项。
5. 剪贴板族**未动**两处：轮询间隔 `clipboardPoll=600ms` vs 参考 `POLL_INTERVAL_MS=900`
   （行为节奏，改它要用户拍板）；图片上限的测量点已核**同构**（参考 monitor.rs:147 判编码后
   PNG 字节，本仓 `AddImageCaption` 判的也是存盘的 PNG 字节），可从待办里划掉。
6. **本轮新发现、未动**：写回面与折叠的交界。`copyClipboardEntry` 写回文字时置 `selfCopied`，
   `captureClipboard`（`system.go:504`）据此跳过——于是**文字**条目复制回来不会像参考那样
   被 `fold_capture` 提到顶部；而**图片/文件**条目没有这个抑制，走监听器 → 折叠 → 会提到顶部。
   三种 kind 三条答案。要收口就得先决定「复制回来的条目该不该置顶」（参考：该），
   且会让 `shell_test.go:1382/1411`、`capture_test.go:197`、`detachlist_test.go:309`
   四处既有 pin 重定基线——属于交互行为变更，先记在这里，不擅自改。
7. 下一轮建议：离开剪贴板族，回到「三平台一致性」表面挑一处（例如 `internal/browser` /
   `internal/terminal` 里某条只在单平台成立的分支）。
