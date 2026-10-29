# P11-ce 规格：补全之后的光标停在行尾

## 1. 意图（一句话）

用户在命令的参数模式里按 Tab 补全一个参数之后，**下一个字符必须接在补全词后面**，
而不是插进补全词的中间。

## 2. 为什么现在做

选题顺序：用户实测反馈 F1–F4 已全部收完 → `docs/raycast-design.md` §6 全 `已落地`
（只剩拍板项）→ P11-cd「仍待」第 1–3 条全是拍板项/作废项 → 按用户优先级自己找
（界面质感与交互 → 基础功能 → 三平台一致性 → 像素细节）。

找到的是**输入框的交互行为**，实测复现（不是猜）：

```
$ go test -count=1 -run TestScratchCaretAfterTabCompletion ./internal/launcher/
    zz_scratch_test.go:18: after completion: "-mode "
    zz_scratch_test.go:21: after typing x: "-mxode "
```

即：`-m` + Tab 补全成 `-mode `，再按 `x`，字段变成 `-mxode `。命令行的下一个字符
落在词中间，只能靠手改。

## 3. 现状与根因（动手前读的代码，行号本轮实测）

- 补全的写入口只有一处：`internal/launcher/launcher.go:2117` `appendWord`。
  它被两条路径共用——Tab 键（`launcher.go:1755`，`case a.mode != nil && item.complete != ""`）
  与补全行自己的 `Run`（`search.go:573` `Run: func() { a.appendWord(value) }`，点击/Enter）。
- 字段是 `ui.TextInput(&a.Query)`（`launcher.go:1465`）。`a.Query` 是 **App 写的**，
  编辑器里另有一份缓冲与光标。框架对外部写入的文本只做夹取：
  `mygo@v0.7.3/ui/editor_history.go:59` 的 `ed.caret = min(ed.caret, ed.buf.n)`。
  光标**不跟着插入点走**，所以补全后行变长、光标留在旧位置（`-m` 的 2）。
- App 早就有处理这件事的字段：`pendingCaret`（`launcher.go:379` 注释、`launcher.go:1486`
  消费，`field.SetTextSelection(len(a.Query), len(a.Query))`）。`SetQuery`/`ResetQuery`/
  历史 ↑↓/模式进入/文件页都置了它——**唯独 `appendWord` 没有**。这是漏网，不是设计。
- 参考的行为：`main` 的输入是 DOM `<input>`，App 从外部写 `value` 时浏览器把光标放回末尾
  （React 受控输入的行为），所以补全之后接着打字是行的常态。

## 4. 逐条落地项

| 位置 | 现状 | 应改成 |
| --- | --- | --- |
| `internal/launcher/launcher.go:2138`（`appendWord` 尾部） | 写 `a.Query` 后只置 `Selected/chosenRow`，光标留在旧位置 | 加 `a.pendingCaret = true`，与同族五条路径一致；那一帧字段把选区放到 `len(a.Query)` |
| `internal/launcher/tabcaret_test.go`（新增） | 无 | 两条臂：Tab 键补全后键入 `x` → `-mode x`；补全行的 `Run` 后键入 `x` → `-mode x` |

**不动的红线**：补全出来的字符串本身（`trimmed + " " + word + " "` / `replaceLastWord`）、
`Selected/chosenRow` 的重置、`FocusSearch`、字段的墨与几何、高度链、键盘模型、i18n、
命令接线。只改「光标在哪」。

## 5. 门槛与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      → HEAD 2754 → 本轮 2755
go test -count=1 -v ./... | grep -c -- "--- SKIP"      → 55（未变）
func Test 计数                                          → 2048 → 2049
三平台 build                                            → 过
```

## 6. mutation 计划（实跑，见报告 §5）

1. **M1**：删掉 `appendWord` 里的 `a.pendingCaret = true` → 新 pin 两条臂都红
   （`-mxode ` 原样回来）。
2. **M2**：把字段消费 `pendingCaret` 时的 `SetTextSelection(len(a.Query), len(a.Query))`
   改成 `SetTextSelection(0, 0)` → 新 pin 红成 `x-mode `，证明这条 pin 量的是**光标**，
   不只是那个布尔字段。
