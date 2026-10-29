# P11-cn 规格：本帧的动作读本帧那张表（键盘 / 星标键 / 星标点击）

## 意图

`View` 每帧只走一遍目录（P11-bq 起的规矩，`App.searchRuns` 有 pin 守着）。P11-cl 修掉了
**指针点击行**那一支在绘制期又走一遍的漏网。本轮把**同一族剩下的三支**收口：它们都在
一帧之内第二次调用 `a.Results()`，而那一遍全量匹配（`Catalog()` 重建 + 每条打分 + 拼音）
买到的信息和本帧已经建好的那张表完全一样。

## 为什么现在做（证据：本轮实测，不是旧报告）

探针（临时文件，已删）在 `720×488`、剪贴板模式两行上逐帧数 `a.searchRuns`：

```
arrow down         draws=61 over1=1  [2 1 1 1 …]
arrow up           draws=61 over1=1  [2 1 1 1 …]
cmd-D favorite     draws=80 over1=1  [2 1 1 1 …]
cmd-Backspace arm  draws=80 over1=0  [1 1 1 1 …]
star click         draws=62 over1=1  [1 … 1 2 1 …]
typed Tab          draws=61 over1=0
typed cmd-1        draws=5  over1=0
```

三次「某一帧某一次 draw 搜索 2 次」都是**用户按一下键/点一下星**的那一帧，且都在
**绘制中**（用户已经按下去、窗口正在画）。三个调用点（HEAD `142e8e82`）：

| 位置 | 现状 | 每一帧的第二次走 |
|---|---|---|
| `launcher.go:2837` `move` | `a.nextSelectable(...)` → `a.Results()`（`:2856`） | ↓ / ↑ |
| `launcher.go:1790` `modeShortcuts` | `toggleCalculatorFavorite`/`toggleClipFavorite` → `selectedClip()`/`a.Results()`（`:1882`） | ⌘D |
| `launcher.go:2613` 行尾星标 `Clicked` | `a.toggleFavorite(item)` → `toggleClipFavorite` → `selectedClip()` → `a.Results()` | 点星 |

**同一条规则已经写在代码里**：`nextSelectableIn` 的注释——「the draw clamps its choice
against the slice it built, and asking for a fresh catalog there was the draw's second full
match」；`View` 的 `a.drawn`（P11-cl）就是那张表。键盘那支当时改的是 `nextSelectableIn`
**本身**，但 `move` 仍然走 `a.nextSelectable`（它自己再 `a.Results()` 一遍），所以键盘
那一支**并没有真的改到**——P11-cl 的计数 pin 只覆盖点击臂，没有覆盖方向键，这就是它没被
抓住的原因。

## 逐条落地项

1. **`launcher.go:2837` `move`**
   - 现状：`a.Selected = a.nextSelectable(a.Selected, delta)`；`nextSelectable` 每次
     `a.Results()`。
   - 应改成：`a.Selected = nextSelectableIn(a.drawn, a.Selected, delta)`。
   - 连带：`nextSelectable`（方法）失去唯一的生产调用点 → 删掉；`filters_test.go:136`
     改读 `nextSelectableIn(results, …)`（断言一字不改：箭头不落在分组头）。
2. **`launcher.go:1790` `modeShortcuts`（⌘D 星标键）**
   - 现状：无参 toggle，内部 `selectedClip()`/`a.Results()`。
   - 应改成：先取 `item, ok := a.selectedHistoryRow()`（读本帧的表，见第 3 条），再
     `toggleCalculatorFavorite(item)` / `toggleClipFavorite(*item.clip)`。
3. **`launcher.go:1881` `selectedClip` / `fieldkeys.go:320` `selectedHistoryRow`**
   - 现状：两者都 `results := a.Results()`。
   - 应改成：读 `a.drawn`——「用户在屏幕上看着的那一行」。删除键（`historyDeleteClaimable`）
     与 ⌘D 都靠这条判据，两者的语义本来就是「屏幕上选中的那一行」。
4. **`launcher.go:2613` 行尾星标点击**
   - 现状：`a.toggleFavorite(item)` → 无参 toggle → `selectedClip()` → 再走一遍目录。
   - 应改成：`toggleFavorite(item)` 把**这一行自己的 Item** 交下去
     （`toggleClipFavorite(entry)` / `toggleCalculatorFavorite(item)`），点击那一支一次
     目录都不再走。
5. **`a.drawn` 的注释**：从「指针点击用的那一份」升级成「本帧画出来的那张表，本帧自己的
   动作也读它」。

**行为契约零改动红线**：高度链、行几何、键盘模型（哪些键做什么）、会话管理、设置写回、
i18n 文案、命令接线——一个字节不动。改的只是**动作从哪张表里取那一行**。V=0 / S=1
（`nextSelectable` 方法删除，测试改读它的 `In` 版本）/ B=0。

**一处刻意的语义收窄（写进报告）**：星标键与删除键现在作用于**上一帧画出来的**那张表，
而不是「按下那一刻重新匹配出来的」表。两表在正常使用下逐条相同（本轮用探针比过 12 帧
× 8 种状态：`frame == fresh`），只在剪贴板监听在两帧之间插入新条目时才会不同；此时
「用户看着的那一行」才是对的答案——与删除族早已定下的「按 id 而不是按索引」同一条规矩。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2764 → 2764+（只许涨）
三平台 go build ./...
```

基线：**PASS 2764 / FAIL 0 / SKIP 55 / 2055 测试函数**（HEAD `142e8e82` 实测）。

## mutation 计划（≥2 条，实跑）

- **M1**：`move` 改回 `a.nextSelectable(...)`（即每帧第二遍全量匹配）→ 新 pin 的
  「the down arrow」臂红（该帧某一次 draw 搜索 2 次）。
- **M2**：`selectedHistoryRow` 改回 `a.Results()` → 「the star's key」（⌘D）臂红。
- **M3**：行尾星标点击改回绘制期重取（`a.toggleFavorite(a.Results()[i])`）→ 「the star's
  click」臂红。
- 每条注入 → 看红 → `md5sum -c` 逐字节还原 → `touch` 再复跑一次绿。
