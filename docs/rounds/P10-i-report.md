# P10-i 报告：`args` 提示也有一条判据

规格 `docs/rounds/P10-i-spec.md`。方向：任务书「现在该做什么」交棒条目（`.pending-bugs.md`
P10-h「仍待」第 3 条）。

## 1. 先确认「本仓没有读者」

```
$ grep -rn '"args"\|args=' internal/shell/*.go
internal/shell/linkargs.go:31:     folded into an `args=` hint: registerFromLink takes the name and nothing
internal/shell/linkparams.go:48:		"args":    "args",
```

两处都不是读者：`linkargs.go:31` 是注释（终端行不折叠成 `args=`），`linkparams.go:48` 是表的行。
`registerFromLink`（`system.go:281-303`）只收 `name`。所以 `args` 今天**接受但丢弃**，
判据在数据上不可观测——但**在拒绝上可观测**：值不合参考的链接，本仓照常连接，参考整条拒绝。

## 2. 落地

### 2.1 `internal/shell/linkargshint.go`（新）

`linkArgsFit(raw string) bool`，逐条照 `validate_arguments`（`main:src-tauri/src/deep_link.rs:402-438`）：

| 规则 | 参考 | 本仓 |
|---|---|---|
| 空串 / 纯空白 → 接受 | `:404-406` `Ok(None)` | `linkArgsFit` 第一段 |
| 整串 > 256 字符 → 拒 | `:407-409` | `maxArgumentsChars = 256` |
| 按 **ASCII 空格**切，空 token 跳过 | `:411-415` | `strings.Split(value, " ")` + `TrimSpace` |
| 单 token > 64 → 拒 | `:416-418` | `maxArgumentChars = 64` |
| 前导 `-` → 拒 | `:419-421` | **不写分支**，见 §4 的幸存者 |
| 字符类 `[A-Za-z0-9._+=:,@%]` | `:422-424` + `is_inert_argument_char` `:440-443` | `inertArgumentChar` |
| token 数 > 16 → 拒 | `:427-429` | `maxArgumentTokens = 16` |

字符数用 `utf8.RuneCountInString`（参考数 `chars()`）。这条**今天不可观测**：字符类是 ASCII-only，
60 个汉字无论按字节还是按字符都被字符类拒。忠实翻译，注释写明。

### 2.2 `internal/shell/system.go:238-246`

register 分支里，`bareCommandName` 之后、`a.Open(SurfaceSettings)` 之前：

```go
if !linkArgsFit(parsed.Query().Get("args")) {
    a.refuseLink()
    return
}
```

顺序照参考：`parse_url` 先 `validate_command` 再 `validate_arguments`（`:496-501`）。位置照 P10-h 的
纪律：拒绝在开页之前。复用 `refuseLink` → `settings.deepLinkRejected`，**零新文案**（参考的
`InvalidArguments` 也不在 `is_silent` 名单里，`:225-228`）。

### 2.3 文档

- `docs/extensions/README.zh-CN.md:102-104`：补上形状（空格切 token、字符类、不许前导 `-`、
  单 token ≤ 64 / ≤ 16 个 / 整串 ≤ 256，不满足的整条拒绝）。承诺本身没变：`args` 仍「只作提示、
  从不执行」。
- `internal/shell/linkparams.go:22-30`：`args` 那段补一句「在表里不等于没判据，值有 `linkArgsFit`」。

### 2.4 修掉的活路径

| 链接 | 改前 | 改后 |
|---|---|---|
| `floter://register?cmd=rg&args=src%3Brm` | 打开集成页并真的去连 `rg` | 一句话拒绝，不切页 |
| `floter://register?cmd=rg&args=--hidden` | 同上 | 同上 |
| `floter://register?cmd=rg&args=` + 300 字符 | 同上 | 同上 |

## 3. 台账

- **V=0**（无视觉数值；本轮没碰墨与面，没出图——不是视觉轮）。
- **B=0**（文案、接线、高度链、键盘、会话、设置写回零改动；`refuseLink` 复用既有句）。
- **S=1**：`internal/shell/linkparams_test.go` 两处 `args=--hidden` → `args=src`。它们钉的是
  「表内键不被表拒」，而 `--hidden` 现在被**值判据**拒；注释写明重定基线的理由。「前导 `-` 被拒」
  改由 `linkargshint_test.go` 钉。

## 4. 变异（5 条实跑，含 1 条幸存者）

命令模板：改文件 → `go test -count=1 -run <pin>` → 抄原始报错 → 还原 → `md5sum -c` 逐字节核对 →
`touch` 再跑一次（避免缓存）。

**M0（幸存者，已实跑）** 删掉参考式的 `strings.HasPrefix(token, "-")` → **全绿**。
原因：`-` 不在字符类里，前导 `-` 由字符判据拒；参考自己的 `:419-421` 分支同样不可达。
处置：这条分支**没有进仓**，注释写明两处写法收成一处（与 P10-g 对 `value.split('.')` 一致）。

**M1** 把 `-` 加进字符类 → 红：

```
--- FAIL: TestTheArgumentsHintHasTheReferencesTokenRules (0.00s)
    linkargshint_test.go:68: linkArgsFit("-hidden") = true, want false (a leading dash)
    linkargshint_test.go:68: linkArgsFit("--version=1.2.3") = true, want false (a leading dash, even when the rest is inert)
--- FAIL: TestTheRegisterLinkRefusesAnArgumentsHintThatIsNotInert (0.00s)
    linkargshint_test.go:93: args=--hidden switched surface to 1
    linkargshint_test.go:96: args=--hidden reads "", want the refusal "That link could not be opened"
```

**M2** 字符类放宽成「非空格即接受」→ 红（7 格，抄前 3 条）：

```
    linkargshint_test.go:69: linkArgsFit("日日…日") = true, want false (the character class is ASCII-only, however few runes)
    linkargshint_test.go:69: linkArgsFit("src;rm") = true, want false (a semicolon ends the token for a shell)
    linkargshint_test.go:69: linkArgsFit("|cat") = true, want false (a pipe)
```

**M3** `maxArgumentTokens` 16 → 17 → 红：

```
    linkargshint_test.go:69: linkArgsFit("a a a a a a a a a a a a a a a a a") = true, want false (one token over the count bound)
```

**M4** 删掉整串长度判据 → 红（五个 51 字符 token = 259 字符，其余三条界限都够不着）：

```
    linkargshint_test.go:69: linkArgsFit("aaa…a aaa…a aaa…a aaa…a aaa…a") = true, want false (over the total bound)
```

**M5** 删掉 `system.go` 的调用点 → 红（证明判据接在活路径上，不是只测纯函数）：

```
--- FAIL: TestTheRegisterLinkRefusesAnArgumentsHintThatIsNotInert (0.01s)
    linkargshint_test.go:94: args=src%3Brm switched surface to 1
    linkargshint_test.go:97: args=src%3Brm reads "", want the refusal "That link could not be opened"
```

每条都 `md5sum -c` 成功、`touch` 后复跑 `ok`。

## 5. 门槛（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2099（基线 2096，+3）
GOOS=linux/windows/darwin go build ./...               # 三个都过
```

- **2099 PASS** / **36 包**（34 个有测试，2 个 no test files）/ **15 SKIP**（=）/
  **0 FAIL** / **1709 测试函数**（+3）。
- 新增 3 个测试函数：`TestTheArgumentsHintHasTheReferencesTokenRules`、
  `TestTheRegisterLinkRefusesAnArgumentsHintThatIsNotInert`、`TestAnInertArgumentsHintChangesNothing`。
  无新 `t.Skip`。

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 39 轮，本仓无可改代码。
2. 需拍板四条不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. 参考的 `args` **渲染面**（review surface 上的提示）本仓没有；README 说「只作为提示展示」，
   本仓只做到「接受、判形、丢弃」。要不要真展示需拍板（会新增视图）。
4. `open` 的参数分歧（参考零参数 / 本仓 `q`、`query`）已钉住，要收需用户拍板。
