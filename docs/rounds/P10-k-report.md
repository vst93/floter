# P10-k 报告：register 的名字拼写——参考只认 `cmd`，另外三种是本版扩展

## 1. 本轮做了什么

任务书交棒「做 `register` 的位置参数拼写」，前提是「参考只读 query（`cmd`/`tool`/`program`）」。
本轮先核参考，**前提不成立**：参考只认 `cmd`，`tool`/`program`/位置参数三种都是本版多出来的拼写。
于是本轮把这一族一次收口（产品行为零改动）：

- 新增 `internal/shell/registername_test.go`（2 个测试函数，走完整 `HandleURL`）：
  四种拼写的真值表 + 「query 名压过位置名」。
- 改正四处**假注释**（「the old build's three spellings」——参考从来没有这三种）。
- `docs/extensions/README.zh-CN.md` 的 register 一节补一句：`tool`/`program`/位置参数是本版别名。
- `.pending-bugs.md` 新增拍板项：**「register 只留 `cmd`（对齐参考）」**——删是改交互，不擅自做。

## 2. 前提被推翻：参考只认 `cmd`

| 证据 | 行 | 内容 |
|---|---|---|
| `main:src-tauri/src/deep_link.rs` | `:472` | 循环里唯一的 register 名字键：`if action == "register" && key == "cmd"` |
| 同上 | `:491` | 其余键 → `return Err(Reject::UnknownParameter(key))`，所以 `tool`/`program` 被拒 |
| 同上 | `:505` | `command.ok_or(Reject::MissingCommand)?` —— 位置参数不进 `command`，`/rg` 被拒 |
| 同上 | `:588` | `canonical_argument` 只 `append_pair("cmd", value)`，从不拼 `tool=`/`program=` |
| `main:src/deep-link.ts` | `:42` | `DEEP_LINK_REGISTER_EXAMPLE = "floter://register?cmd=rg"` |

所以「三种拼写」是本版从 `a498523`（`feat(go): floter://register connects the program it names`）
起就写在提交信息与注释里的**假归属**——它说「The three spellings the old build accepted
(`cmd`, `tool`, `program`, or a positional path)」，但 old build 一个都没多认。

## 3. 真值表（本机实测，改前/改后行为相同）

| 链接 | 本仓 | 参考 | 一致？ |
|---|---|---|---|
| `floter://register?cmd=rg` | 接（`rg`） | 接（`rg`） | ✅ |
| `floter://register?tool=rg` | 接（`rg`） | `UnknownParameter` | ❌ 本版别名 |
| `floter://register?program=rg` | 接（`rg`） | `UnknownParameter` | ❌ 本版别名 |
| `floter://register/rg` | 接（`rg`） | `MissingCommand` | ❌ 本版位置别名 |
| `floter://register//usr/bin/rg` | 拒（`bareCommandName`） | `MissingCommand` | ✅（都拒，理由不同） |
| `floter://register/rg?cmd=fd` | 接（`fd`，query 优先） | 接（`fd`，忽略 path） | ✅（名字一致） |

最后两行是这张表的价值所在：位置别名**从不压过** query 名（`firstNonEmpty` 顺序，
`system.go:232-234`），所以「带 query 名的位置链接」两边答案一致；只有「纯位置链接」
才分叉。

## 4. 落地明细

### 4.1 新 pin（`internal/shell/registername_test.go`）

- `TestTheRegisterLinkTakesTheNameTheReferenceTakes`：表驱动四格（`cmd`/`tool`/`program`/
  位置参数），每格断言：探针收到的名字、`SurfaceSettings` + `PageIntegrations`、
  `registerTarget` 为空、notice == `RegisterNotFound(name)`。注释逐格写参考的拒绝码。
- `TestThePositionalSpellingNeverOverridesAQueryName`：`register/<pos>?<spelling>=<query>`
  三种 spelling 各一格，断言探针只收到 query 名。
- 用 `stubNotFinding`（包内新 helper，走既有 `stubLookTool` seam）记录探针收到什么名字，
  返回 not-found——不依赖本机搜索路径。

### 4.2 假注释改正（断言零改动）

| 文件 | 旧 | 新 |
|---|---|---|
| `internal/shell/system.go:222-231` | 「`cmd`, `tool` or `program` (the old build's three spellings)」 | 「`cmd` 是参考唯一拼写；`tool`/`program`/位置参数是本版别名，参考拒」 |
| `internal/shell/linkparams.go:46-48` | 「One name, three spellings」 | 补「只有 `cmd` 是参考的行；`tool`/`program` 是本版别名」 |
| `internal/shell/appkeys_test.go:224-229` | 「the three spellings the old build accepted」（循环里其实四个） | 「four spellings this build accepts；`cmd` 是参考的，其余三个参考拒」 |
| `internal/shell/linkparams_test.go:47-49` | 「the old build's second/third spelling」 | 「this build's alias; the reference refuses it (UnknownParameter)」 |

`linkparams_test.go` 只改 `why` 描述串，`want` 与断言未动。

### 4.3 README

`docs/extensions/README.zh-CN.md:112-115`：补一句「`cmd` 是规范写法；本版也接受 `tool`、
`program` 与位置参数（`floter://register/rg`）三种等价别名——它们是本版的扩展，参考实现只认
`cmd`」。承诺不变（只作提示、从不执行、一次性确认）。

### 4.4 拍板项（新增，不擅自实现）

`register` 只留 `cmd` 一种拼写（对齐参考）：删 `tool`/`program`/位置参数是**改交互**
（现有链接会失效），按纪律不擅自做。

## 5. 台账

**V=0**（无视觉数值）。**B=0**（产品行为、文案、接线零改动；现有 pin 断言全部照旧通过，
只改注释/描述串）。**S=0**（没有结构断言要改）。

## 6. 门槛（本轮实测）

```
gofmt -l cmd internal                    # 空
go vet ./...                             # 净
go test -count=1 ./...                   # 全绿（34 ok + 2 no test files = 36 包）
--- PASS                                 # 2110（基线 2104，+6 = 4 子测试 + 1 父 + 1）
--- SKIP                                 # 15（=）
--- FAIL                                 # 0
测试函数                                  # 1716（基线 1714，+2）
GOOS=linux/windows/darwin go build ./... # 三个都过
```

## 7. 变异（4 条实跑变红 + `md5sum -c` 逐字节还原 + `touch` 复跑）

改前 md5：`system.go 1200795a47c41185b6d2ab6f667640f8`、
`linkparams.go ff48c025a8753bd9f796fd55d199593a`。每条：注入 → 跑 → 抄原始报错 → `cp` 还原 →
`md5sum -c` 逐字节核对（两文件全「成功」）→ `touch` 后复跑全绿。

1. **M1 删掉 `firstNonEmpty` 的 `tool` 参数**（`system.go:233`）→ 红：
   `registername_test.go:51: floter://register?tool=p10k-tool opened surface 0, want the settings page (this build's alias (reference: UnknownParameter))`。
   只有 tool 那格红，其余三格绿——证明这条 pin 钉的是 tool 这条路径。
2. **M2 把 `firstNonEmpty` 的参数顺序改成 path 在前** → 红（三条）：
   `registername_test.go:82: floter://register/p10k-pos?cmd=p10k-query asked the probe for [p10k-pos], want p10k-query (the query name outranks the path)`
   （`tool=`/`program=` 两条同形）。证明「query 压过 path」不是恒真。
3. **M3 不再读 `parsed.Path`（删位置参数）** → 红：
   `registername_test.go:51: floter://register/p10k-pos opened surface 0, want the settings page (this build's positional alias (reference: MissingCommand))`。
   只有位置那格红——证明位置别名是**产品**在支持，不是测试自说自话。
4. **M4 删掉 `linkparams.go` 表的 `tool` 行** → 红（两处）：
   `linkparams_test.go:67: linkQueryFits("register", "tool=rg") = false, want true (this build's alias; the reference refuses it (UnknownParameter))`
   与 `registername_test.go:51: floter://register?tool=p10k-tool opened surface 0, want the settings page`。
   证明表与路由是同一件事的两半。

## 8. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 41 轮，本仓无可改代码。
2. 需拍板四条不变（P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. **新增拍板项**：register 只留 `cmd`（对齐参考）——删 `tool`/`program`/位置参数是改交互。
4. `open` 的参数分歧（参考零参数 / 本仓 `q`、`query`）已钉住（`linkparams_test.go` 三格 +
   `shell_test.go:651-658`）。要收需用户拍板。

## 9. 本轮非视觉

非视觉轮，无像素改动，故不出 PNG。
