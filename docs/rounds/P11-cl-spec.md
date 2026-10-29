# P11-cl 规格：点一下行的那一帧，目录也只走一遍

## 意图

一个**绘制期**的指针边（行自己的 `Clicked()`）在同一次 draw 里把目录又完整匹配了一遍：
`a.activate(a.Results())`。本仓早有一条规则写在同一份代码里（`nextSelectableIn` 的注释：
「the draw clamps its choice against the slice it built, and asking for a fresh catalog there
was the draw's second full match」），键盘那一支已经照它改了，**指针那一支漏了**。

为什么现在做：`searchruns_test.go` 的 `TestADrawWalksTheCatalogOnce` 存在就是为了守「一帧一遍」，
但它的步骤表里没有任何一步是**点**。实测（720×488，五条最近启动）：

```
click a row   searches per draw: [1 1 … 1 2 1 … 1]   ← 第 22 个 draw 是 2
idle frame    searches per draw: [1]
click footer  searches per draw: [1]
```

——一次点击的帧里有一遍多出来的全量匹配，没有按键可以解释它。在大目录（Start Menu 上千条 + 拼音打分）
这一遍就是用户点一下时的可见延迟；与 P11-aw/ax 那一族「同一条规则漏一个调用点」是同一个形状。

## 落地项

### 1. `internal/launcher/launcher.go` · `row` 的 `Clicked` 分支（现状 `:2656`）

现状：

```go
if row.Clicked() && !removed {
    a.Selected = i
    a.pointerChose = true
    a.activate(a.Results())   // ← 绘制期的第二次全量匹配
}
```

应改成：读这一帧已经建好的那份切片。

- `App` 新增 `drawn []Item` 字段（注释写清它为什么存在、与 `nextSelectableIn` 同规则）。
- `View` 在 `clampSelection` 之后、`assignNumbers` 之前写 `a.drawn = results`（同一个
  `results` 就是列表建行用的那份）。
- `row` 的 `Clicked` 分支改成 `a.activate(a.drawn)`。

**只改这一次查询的来源**：选择、指针交接、行几何、动作执行、编号、高度链、键盘模型、
i18n、命令接线一个字节不动。

## 行为契约零改动红线

- 高度链（`height.go`）：不动。
- 键盘模型（↑↓/Tab/Esc/⌘↩/Super+↩）：不动。
- 会话管理、设置写回、i18n 文案、命令接线：不动。
- **视觉：零像素改动**（本轮的证据是「哪份切片被读」，不是墨）。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2763 → 2764（+1）
go test -count=1 -v ./... | grep -c -- "--- SKIP"   # 55（不变）
GOOS=linux/windows/darwin go build ./...  # 全过
```

基线（本轮开轮实测）：**PASS 2763 / FAIL 0 / SKIP 55 / 函数 2052**。
收轮实测：**PASS 2764 / FAIL 0 / SKIP 55 / 函数 2053**（+1 = 新 pin
`TestAClickRunsTheRowTheDrawBuilt`；`TestADrawWalksTheCatalogOnce` 新增的 `a click on a row`
步骤是**表里的一步**，不是子测试，所以不涨 PASS 计数——它涨的是覆盖面）。

## mutation 计划

- **M1**：`row` 的 `Clicked` 分支改回 `a.activate(a.Results())`
  → `TestADrawWalksTheCatalogOnce` 的 `a click on a row` 臂红（实测原串：
  `the a click on a row frame searched twice on 1 draws, but injected 0 keystrokes`）。
- **M2**：删掉 `a.drawn = results` 那一行（`drawn` 恒为 nil）
  → 新 pin `TestAClickRunsTheRowTheDrawBuilt` 红（实测原串：`the click ran [], want [Recent 02]`）。
  ——这条是为了证明「只数次数」不够：`drawn` 过期/为空时，计数 pin 仍绿而点击什么都不跑。
