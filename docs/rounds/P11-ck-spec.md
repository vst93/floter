# P11-ck Spec: Output insert path needs a behavioral caret arm

## Intent

The `insert` action in `runOutputRow()` (`output.go:417–419`) writes `a.Query = row.Action.Text`
and sets `a.pendingCaret = true`. The source contract (`TestEveryQueryWriteCarriesItsCaret`)
verifies the assignment exists, but no test exercises the actual caret landing through the UI.
P11-cf still待 item #2 calls this out directly.

## Why Now

P11-cf identified the gap: the only path among seven `a.Query` writes that has no behavioral
arm is `output.go:419`. All other paths (deep link, SetQuery, history recall, completion)
have behavioral assertions. A regression that silently breaks only the output insert (e.g. a
change to `leaveOutput` ordering or a new guard before the field) would not be caught by
`TestEveryQueryWriteCarriesItsCaret` alone.

## Current state

| Path | Source contract | Behavioral arm |
|---|---|---|
| Deep link (launcher.go:379) | ✓ | ✓ (TestEveryExternalWrite…) |
| SetQuery (launcher.go:1033) | ✓ | ✓ (same) |
| History recall (history.go:54,67,71) | ✓ | ✓ (same) |
| appendWord (launcher.go:2137) | ✓ | ✓ (TestACompletionLeaves…) |
| Calculator/files (calculator.go:66, files.go:64) | ✓ | ✓ (TestEveryExternalWrite… arms) |
| **output insert (output.go:419)** | **✓** | **MISSING** |

## What to build

A behavioral test `TestAnOutputInsertLeavesTheCaretAtTheEnd` that:

1. Sets up an App with an active output list containing a row with
   `Action = &extensions.RowAction{Type: "insert", Text: "git status --short"}`.
2. Renders the frame to get a `*ui.Tester`.
3. Runs `runOutputRow()` via Enter (the keyboard path) or calls it directly.
4. Verifies `a.Query == "git status --short"` (leaveOutput was called, then Query was written).
5. Types `x` on the next frame and verifies the result is `"git status --shortx"` —
   proving the caret was placed at the end by `pendingCaret`.

## Behavior contract zero-change red lines

- Height chain, keyboard model, session management, settings writeback: **untouched**.
- The only production file touched is none (test-only).
- i18n copy: no new strings.
- Command wiring: unchanged.

## Threshold commands

```
gofmt -l cmd internal
go vet ./...
GOOS=windows go vet ./...
GOOS=darwin go vet ./...
go test -count=1 ./...
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## Mutation plan

1. **M1**: In `output.go:419`, remove `a.pendingCaret = true` after the insert write.
   Pre-registered expectation: new test turns RED. **实测结果（报告 §2/§4）：行为臂绿、
   源码契约红** —— 输出页期间字段整行不在树里（`launcher.go:1293` 早退），回场时框架
   新建编辑器、光标天然在末尾（mygo `ui/textinput.go:36-40`）。这正是两把尺子互补。

2. **M2**: In `output.go:417`, swap the order: write `a.Query` **before** calling `leaveOutput()`.
   `leaveOutput()` clears `a.Query` to `""`, so the written text is lost.
   Expected: new test turns RED (Query is empty after Enter). — **实测：行为臂红**。
