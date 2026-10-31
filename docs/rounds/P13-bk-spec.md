# P13-bk 规格 · 字段行尾部那一格，在三页上说错了话

## 意图（一句话）

启动器字段行右端那一格提示（`copy.CommandModeHint`，「Enter runs it · Tab completes ·
Esc cancels」）本该只属于**扩展命令模式**，却同时挂在剪贴板页与浏览器页上：那两页没有命令
可跑，而它们自己的 chips 行同一帧里写着「Tab to switch」——**同一个窗口对同一个键给出两个
相反的说法**。命令模式的空态上，这句话还被画了**两遍**（页面陈述 + 字段尾部）。

## 为什么现在做（证据，HEAD `f7d6a688` 实测）

`ui.Tester` 出图后逐像素量（地面 浅 253 / 深 23，峰值对比）：

| 页 | chips 行的提示 | 字段行尾部的提示 |
|---|---|---|
| 剪贴板（空） | `Tab to switch · Ctrl+D to favorite` rect `{325 60 379 20}` ink **144 / 112** | `Enter runs it · Tab completes · Esc cancels` rect `{390 17 278 22}` ink **116 / 164** |
| 浏览器（空） | `Tab to switch` rect `{248 60 456 20}` ink **144 / 112** | 同上，rect `{390 17 278 22}` ink **116 / 164** |
| 命令模式（无匹配） | — | 同一句话出现 **2 次**（`tt.Texts()` 计数 = 2）：一次是页面陈述（`Emphasis` + `TextStrong`），一次是字段尾部（`FontSize` + `TextMuted`） |
| 计算器 / 文件 / 普通页 | — | `Type to search` rect `{574 17 94 22}` ink **116 / 164** |

**i18n 自己的契约**：`internal/i18n/i18n.go:117-119` 写着
「CommandModeHint is the field's hint **while an extension command's arguments are typed**」。
剪贴板页没有命令、也没有参数；按这份契约，那一格不该出现。而 `internal/launcher/launcher.go:1569`
的条件是 `a.mode != nil || a.clipboard || a.browser`——多出来的两个成员是**移植时顺手并进来的**。

**同一族两个答案**：五个插件模式里，计算器与文件页走 `default` 分支（空查询 → `copy.Hint`，
有查询 → ✕ 清除键），剪贴板与浏览器走命令分支。同一族的五页，两种说法，其中一种是错的。

## 落地项

| # | 位置（HEAD 行号，动手前复核） | 现状 | 应改成 |
|---|---|---|---|
| 21-1 | `internal/launcher/launcher.go:1569` 字段行的 `switch` | `case a.mode != nil \|\| a.clipboard \|\| a.browser:` → 三页都画 `CommandModeHint` | `case a.mode != nil:`——只有命令模式有「Enter runs it / Tab completes」这两句话的事实。剪贴板与浏览器落进 `default`，与计算器/文件页同一答案（`copy.Hint`，有查询时是 ✕） |
| 21-2 | 同文件 `:1569-1575` 的命令模式臂 | 命令模式无匹配时，页面陈述（`emptyMessage` 在 `a.mode != nil` 返回**同一个** `CommandModeHint`）与字段尾部是同一句话 | 页面已经说过的那一句，字段尾部不再重复：尾部只在「页面没在说这句话」时画（页面陈述的条件 = `a.output == nil && !a.configOpen() && len(results) == 0`） |
| 21-3 | 同文件 `:1360` 空态臂 | `} else if len(results) == 0 {` | 提到同一个局部量（`quietPage`），两处共用一个判据，不许两处各写一遍（21-2 要读的就是它） |

**不改**：`copy.CommandModeHint` 的文案（i18n 红线）、`emptyMessage` 在命令模式返回它的决定、
chips 行的提示、键盘模型、高度链（字段行是 `Absolute().Height(fieldRow)`，尾部只是它的一个
`Shrink(0)` 孩子，`Grow(1)` 的字段吃掉腾出的宽度）、`copy.Hint` 的文案。

**一个如实的副作用（写在这里，不藏在数字里）**：剪贴板页与浏览器页在**有查询**时，字段尾部
从「那句假话」变成 ✕ 清除键——与计算器/文件页同一个控件。这不是新功能，是同一族的第五页
终于和另外三页长得一样。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                       # 空
go vet ./...                                # 净
GOOS=windows go vet ./...                   # 净
GOOS=darwin  go vet ./...                   # 净
go test -count=1 ./...                      # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3168，只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

测试函数基线 **2337**，SKIP 基线 **111**（其中 1 条是本轮的 opt-in 出图）。

## mutation 计划

| # | 注入 | 应红的 pin |
|---|---|---|
| M1 | 把 `case a.mode != nil:` 改回 `case a.mode != nil \|\| a.clipboard \|\| a.browser:` | 「剪贴板/浏览器页的尾部不是命令提示」两条 |
| M2 | 删掉 21-2 的重复守卫（命令模式无条件画尾部） | 「命令模式的这句话只说一遍」 |
| M3 | 把尾部墨从 `t.TextMuted` 改成 `tokens.TextTertiary` | 「五个插件模式的尾部是同一档墨」（像素级） |
| M4 | 把 `quietPage` 的 `!a.configOpen()` 去掉 | 配置浮层打开时的重复守卫（结构 pin + 文本计数） |

## 红线自查

- 高度链：字段行高度、窗口高度、`Geometry` 一个数都不动（尾部是 `Shrink(0)`，字段 `Grow(1)`）。
- 键盘模型：不动。`Tab`/`Enter`/`Esc` 的接线在 `fieldkeys.go`，本轮零改动。
- 会话管理 / 设置写回 / 命令接线：零改动。
- i18n 文案：零改动（只换**用哪一条**，不新增、不改字）。
