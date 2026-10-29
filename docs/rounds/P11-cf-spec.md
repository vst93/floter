# P11-cf 规格：App 写字段的**每一条**路径都带光标规则（一把尺子）

## 0. 意图（一句话）

上一轮（P11-ce）修的是「补全之后光标落在行尾」，但只给**一条**路径（`appendWord`）留了 pin；
其余六条写 `a.Query` 的路径（`SetQuery`、历史 ↑↓、命令模式的 argv、捕获输出页的 insert）
只有**读代码的复核**，没有断言——把那句 `a.pendingCaret = true` 删掉，全仓**一条测试都不会红**。
本轮把这一族钉成一把尺子：**行为表 + 源码契约**，并补上 P11-ce 自己列出的「补全的另一个分支
没有独立 pin」。

## 1. 为什么现在做

选题顺序：用户实测反馈 F1–F4 **已全部收尾**（F1 → P11-y，F2/F3 → P11-z，F4 → P11-bb/bj/bk/bo）
→ `docs/raycast-design.md` §6 **全 `已落地`**（`grep -n "未落地" ` 只剩 §4.2 一句历史注脚）
→ P11-ce「仍待」第 1 条是 15 条**拍板项**（不能擅自改）、第 2 条是「这一族已复核，没有第二个
漏网」（＝复核过、**没钉住**）、第 3 条正是“另一个分支没有独立 pin”、第 4 条是尺子盲点。
按用户定的优先级（界面质感与交互 → 基础功能 → 三平台 → 像素）取「交互」这一档里**唯一
不牵拍板**的一条：把上一轮刚修的那个用户可见 bug（下一个字符插进词中间）的**回归资产**补齐。

证据（本轮实测，`grep -n "a\.Query = " internal/launcher/*.go | grep -v _test`）：全包 23 处
写 `a.Query`，其中 **7 处写非空串**（`history.go:53/66/70`、`launcher.go:1084`、
`launcher.go:1899`、`launcher.go:2132/2134`、`output.go:418`）；`grep -rn pendingCaret
--include=*_test.go internal/` 只有 `tabcaret_test.go`（补全那条）与一处夹具赋值。

## 2. 逐条落地项（动手前已按当前 HEAD 的行号核对）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/launcher/querycaret_test.go`（新增） | 没有这个文件 | 行为表三臂：深链 `SetQuery` / 历史 ↑ / `Esc` 清空（空写入的对照臂），每臂都「外部写入 → 键入 `x` → 断言 `x` 落在**行尾**」 |
| 2 | 同文件 | 补全的**另一个分支**（`appendWord` 的 `trimmed != a.Query`，即补进一个新的词）没有 pin（P11-ce 仍待第 3 条） | 加一臂：命令模式里键入 `-f `（行尾带空格）→ Tab → 补成 `-f -f ` → 键入 `x` ⇒ `-f -f x` |
| 3 | 同文件 | 这一族的**答案只活在七个地方手写的同一行**（`a.pendingCaret = true`），新加一条写 `a.Query` 的路径没有任何东西会拦 | 源码契约 `TestEveryQueryWriteCarriesItsCaret`：扫 `internal/launcher` 的生产 `.go` 文件，凡是**写非空串**的 `a.Query = …`，其后 6 行内必须有 `a.pendingCaret = true`（写空串的那 16 处不需要：`min(caret, 0)` 已经是 0） |

## 3. 行为契约零改动红线

- 产品代码**一个字节不改**（本轮是尺子轮）。高度链、键盘模型、会话管理、设置写回、
  i18n 文案、命令接线：全部不动。
- 墨与面：不动，**不出图**（理由同 P11-ce：改的是「光标停在哪」，证据是字段里的字符串）。

## 4. 门槛命令 + PASS 基线（本轮实测值）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2755（P11-ce 实测）
GOOS=linux go build ./*/... && windows && darwin        # 三平台过
```

函数数基线 `2049`（`grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l`）；
SKIP 基线 `55`。

## 5. mutation 计划（≥2 条，说明每条会让哪个 pin 变红）

- **M1**：删掉 `SetQuery` 里的 `a.pendingCaret = true`（`launcher.go:1089`）。
  预期：行为表「深链」臂红（`xgit status`），源码契约红（`launcher.go:1084`）。
- **M2**：删掉 `history.go:54` 的那一行。预期：行为表「历史 ↑」臂红，源码契约红。
- **M3**（对照，预期**绿**）：把 `launcher.go:2137` 的 `a.pendingCaret` 挪到 `appendWord` 的
  分支之前——两个分支都覆盖到，行为不变。说明这把尺子钉的是「有没有」，不是「写在哪一行」。
