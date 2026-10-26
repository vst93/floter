# P10-k 规格：register 的名字拼写——参考只认 `cmd`，另外三种是本版扩展

## 意图（一句话）

任务书交棒「做 `register` 的位置参数拼写」，前提是「参考只读 query（`cmd`/`tool`/`program`）」。
本轮先核参考，发现**前提不成立**：参考的 `parse_url` 只认 `cmd`（`main:src-tauri/src/deep_link.rs:472`），
`canonical_argument` 也只拼 `cmd`（`:588`）。于是 `tool`、`program`、位置参数 `/rg` **三种都是本版
多出来的拼写**，参考分别给 `UnknownParameter` / `UnknownParameter` / `MissingCommand`。本轮把这
一族一次收口：钉住四种拼写的行为、改正「old build's three spellings」这三处假注释、把三种扩展
拼写写进 README、并把「删成参考那一种」记进拍板表（删=改交互，不擅自做）。

## 为什么现在做（证据，行号本轮实测）

- 交棒条目：任务书「现在该做什么」→ `.pending-bugs.md` P10-j「仍待」/ 任务书「下一轮第一项」。
- 参考只认一个名字：
  - `git show main:src-tauri/src/deep_link.rs | sed -n '467,492p'`：循环里只有
    `if action == "register" && key == "cmd" && command.is_none()`；其余键落到
    `return Err(Reject::UnknownParameter(key))`（`:491`）。`tool` / `program` 走的就是这一条。
  - 同文件 `:588` `canonical_argument`：`url.query_pairs_mut().append_pair("cmd", value)`，
    从不拼 `tool=` / `program=`。
  - 同文件 `:505`：`command.ok_or(Reject::MissingCommand)?` —— 位置参数 `/rg` 不进 `command`，
    所以 `floter://register/rg` 在参考是 `MissingCommand`。
  - TS 前端也只写 `cmd`：`main:src/deep-link.ts:42` `DEEP_LINK_REGISTER_EXAMPLE = "floter://register?cmd=rg"`。
- 本仓的现状（实测行号）：
  - `internal/shell/system.go:232-234`：`bareCommandName(firstNonEmpty(cmd, tool, program,
    strings.TrimPrefix(parsed.Path, "/")))` —— 四个来源，query 优先于 path（`firstNonEmpty` 顺序）。
  - `internal/shell/linkparams.go:46-49`：表里 `cmd`/`tool`/`program` 三个键共用 `name` 槽
    （注释写「One name, three spellings」，但没说只有 `cmd` 是参考认的）。
  - `internal/shell/system.go:222-224` 注释：「`cmd`, `tool` or `program` (the old build's three
    spellings)」—— **假**，参考只认 `cmd`。
  - `internal/shell/appkeys_test.go:224-226` 注释：「the three spellings the old build accepted
    all reach it」—— **假**，且循环里其实有四个。
  - `internal/shell/linkparams_test.go:47-49` 的 `why`：「the old build's second/third spelling」—— **假**。
  - 三种扩展拼写**用户文档一条都没写**：`docs/extensions/README.zh-CN.md:112` 只写 `?cmd=rg`。
- 可观测的分歧（本机实测，见报告 §3）：
  - `floter://register?tool=rg` → 本仓接（`rg`），参考 `UnknownParameter`。
  - `floter://register?program=rg` → 本仓接，参考 `UnknownParameter`。
  - `floter://register/rg` → 本仓接，参考 `MissingCommand`。
  - `floter://register//usr/bin/rg` → 两边都拒（本仓 `bareCommandName`，参考 `MissingCommand`）。
  - `floter://register/rg?cmd=fd` → 两边都按 `fd`（本仓 query 优先；参考忽略 path）——**一致**。

## 落地项（位置 → 现状 → 应改成）

### 1. 新 `internal/shell/registername_test.go`（真值表 pin，产品零改动）

- 现状：四种拼写只有 `appkeys_test.go:228-233` 用「抽取」钉过（`firstNonEmpty` 的复算），
  **没有一条走完整 `HandleURL`**；`tool`/`program` 的「参考拒」没有任何注释记录。
- 改成两条测试函数（都走 `HandleURL`，用 `stubLookTool` 记录探针收到的名字，返回 not-found）：
  1. `TestTheRegisterLinkTakesTheNameTheReferenceTakes`：表驱动
     `cmd` / `tool` / `program` / 位置参数 / 「位置参数 + `cmd`（query 胜出）」五格，
     每格断言：探针被问的名字 == 期望、落 `SurfaceSettings` + `PageIntegrations`、
     notice == `RegisterNotFound(name)`。注释逐格写参考的答案（接 / `UnknownParameter` /
     `UnknownParameter` / `MissingCommand` / 接 `cmd` 那个）。
  2. `TestThePositionalSpellingNeverOverridesAQueryName`：`floter://register/<a>?cmd=<b>`
     断言探针只收到 `<b>`（`firstNonEmpty` 的顺序），把「path 只是兜底」钉死。
- **不重复** `linkname_test.go:87` 的 `TestARegisterLinkRefusesAPath`（位置参数带路径被拒）——
  注释互指即可。

### 2. 改正三处假注释（产品行为零改动）

- `internal/shell/system.go:222-229`：把「the old build's three spellings」改成
  「`cmd` 是参考唯一认的拼写；`tool`/`program`/位置参数是本版的别名（参考拒，钉在
  `registername_test.go`）」。
- `internal/shell/linkparams.go:46`：注释补一句「只有 `cmd` 是参考的行；另外两个是本版别名」。
- `internal/shell/appkeys_test.go:224-226`：改成「`cmd` 是参考的拼写；其余三个是本版别名，
  参考拒（见 `registername_test.go`）」，并让「four spellings」与循环里的四条对上。
- `internal/shell/linkparams_test.go:47-49` 的 `why`：把「the old build's …」改成
  「this build's alias; the reference refuses it (`UnknownParameter`)」。**断言不动**（只是描述串）。

### 3. `docs/extensions/README.zh-CN.md` 的 register 一节

- 现状：只写 `?cmd=rg`（`:112`）与终端拼写，没提 `tool`/`program`/位置参数。
- 改成：在「`cmd` 是规范写法」后补一句——`floter://register` 也接受 `tool`、`program`
  与位置参数（`floter://register/rg`）三种等价别名；这三种是本版扩展，参考实现只认 `cmd`。
  承诺不变：仍「只作提示、从不执行」，仍走一次性确认。

### 4. `.pending-bugs.md` 拍板表（新增一条，不擅自实现）

- 新增拍板项：**「register 只留 `cmd` 一种拼写（对齐参考）」** —— 删 `tool`/`program`/位置参数
  是**改交互**（现有链接会失效），按纪律不擅自做；要用户拍板。附证据：参考
  `deep_link.rs:472` 只认 `cmd`，`:505` 位置参数是 `MissingCommand`。

## 行为契约零改动红线

- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**（无新文案、无新视图、
  无新接线；拒绝复用既有 `settings.deepLinkRejected`，接受复用既有 `RegisterNotFound`）。
- `HandleURL` 的四来源抽取（`system.go:232-234`）：**零改动**（本轮只钉它、不改它）。
- `linkparams.go` 的表、`bareCommandName`、`linkArgsFit`、`linkManifestValue`：**零改动**。
- 终端拼写 `floter register rg` → `?cmd=rg`（`linkargs.go:88-92`）：**零改动**。

## 门槛命令与 PASS 基线（本轮实测，不是旧报告数字）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2104（本轮基线）
GOOS=linux/windows/darwin go build ./...               # 三个都过
```

基线（本轮实跑）：**2104 PASS** / **36 包** / **15 SKIP** / **0 FAIL** / **1714 测试函数**。

## 台账预判

- **V=0**（无视觉数值）。
- **B=0**（产品行为、文案、接线零改动；现有 pin 断言全部照旧通过——只改注释/描述串）。
- **S=0**（没有结构断言要改；`linkparams_test.go` 只改 `why` 字符串，不改 `want`）。

## mutation 计划（≥2 条，每条说清哪个 pin 变红）

1. 把 `system.go:232-234` 的 `parsed.Query().Get("tool")` 删掉（只留 cmd/program/path）→
   `tool=` 那格变红（探针收到的名字变成空 / 整条被拒）。
2. 把 `firstNonEmpty` 的参数顺序改成「path 在前」→ `TestThePositionalSpellingNeverOverridesAQueryName`
   与真值表最后一格变红（探针收到位置名而不是 query 名）。
3. 把 `bareCommandName` 换成宽松判据（原样收下）→ `linkname_test.go` 的
   `TestARegisterLinkRefusesAPath` 与本轮真值表的「位置参数带路径」格变红。
4. 把 `linkparams.go` 的 `tool` 行删掉 → `linkparams_test.go` 的 `tool=rg` 格变红
   （表外键 → 整条拒绝），同时本轮真值表 `tool=` 格变红。
