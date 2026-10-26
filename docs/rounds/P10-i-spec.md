# P10-i 规格：`args` 提示也有一条判据——同一个链接不能在这里过、在参考里被拒

## 意图（一句话）

`floter://register?cmd=rg&args=…` 的 `args` 值今天**只过表、不过值**：P10-h 把 `args` 列进了
`linkParamSlots`（`internal/shell/linkparams.go:48`，键形合法），但没有任何一处读它、也没有任何一处
判它的形状；参考 `validate_arguments`（`main:src-tauri/src/deep_link.rs:402-438`，测试 `:1498-1513`）
对同一个链接是**整条拒绝**。本轮把参考的逐 token 判据接上，落到同一个 `refuseLink`，让「同一个链接
两边一个答案」。

## 为什么现在做（证据）

- 交棒条目就是这条（`.pending-bugs.md` P10-h「仍待」第 3 条 / 任务书「现在该做什么」）。
- 现状证据（本轮实测）：
  - `internal/shell/linkparams.go:41-51`：`register` 行有 `"args": "args"`，注释明写「accepted and
    dropped」，`linkQueryFits` 只看键与槽，不看值（`:66-90`）。
  - `grep -n "\"args\"\|args=" internal/shell/*.go`：除 `linkparams.go` 的表与注释外**零读者**；
    `registerFromLink`（`system.go:274-296`）只收 `name`。
  - 于是 `floter://register?cmd=rg&args=src%3Brm`（`;` 注入）、`args=--hidden`（前导 `-`）、
    `args=` + 300 字符，今天都**照常**打开集成页并连接；参考对这三种都是 `Reject::InvalidArguments`
    （`:407/416/421`），且 `InvalidArguments` 不在 `is_silent` 名单里（`:225-228`）→ 用户会看到一句话。
- 为什么不是「写明本版不读、钉住接受」：拒绝**不**与「无读者」冲突——参考拒的是值，不是读者；
  本仓一旦哪天有界面展示 `args`（README:102 已经承诺「只作为提示展示」），脏值就已经在仓库里了。
  而接受一条参考拒绝的链接，正是 P10-g/P10-h 已经两次判定为 bug 的形状。

## 落地项（位置 → 现状 → 应改成）

### 1. 新 `internal/shell/linkargshint.go`

- 现状：不存在。
- 改成：`linkArgsFit(raw string) bool` + `inertArgumentChar(r rune) bool` + 三个上限常量，
  逐条照参考 `validate_arguments`：
  1. 整串 `trim` 后为空 → **接受**（参考 `Ok(None)`：键可选，`args=` 等于「没有参数」）。
  2. 整串字符数 > `256` → 拒（`MAX_ARGUMENTS_TOTAL_CHARS`，`deep_link.rs:125`）。
  3. 按**ASCII 空格**切 token（`strings.Split(value, " ")`），逐 token `trim`，空 token 跳过。
  4. 单 token 字符数 > `64` → 拒（`MAX_ARGUMENT_CHARS`，`:119`）。
  5. token 以 `-` 开头 → 拒（前导 `-` 是给下游程序的一个选项，不是一个参数名）。
     **但这条分支在本仓不写**：`-` 根本不在第 6 条的字符类里，参考自己的 `:419-421` 分支同样
     不可达（mutation 实测删掉它全绿）。本仓只留决定性的那一条（第 6 条），注释写明参考的两处
     写法在这里收成一处——与 P10-g 对 `value.split('.')` 的处理一致。
  6. token 里每个字符必须是 ASCII 字母数字或 `. _ + = : , @ %`（`is_inert_argument_char`，`:440-443`）
     → 否则拒（`; | & $ \` < > ( ) { } [ ] * ? ! ~ \ " '` 与所有空白/控制字符都落在这里）。
  7. token 数 > `16` → 拒（`MAX_ARGUMENT_COUNT`，`:122`）。
  8. 字符数用 `utf8.RuneCountInString`，不是 `len`——参考数的是 `chars()`。这条**今天不可观测**
     （字符类是 ASCII-only，一个中日韩 token 无论按字节还是按字符都被第 6 条拒），但按参考读是
     忠实翻译；实测抓到了这一点：初版测试把 60 个汉字写成「接受」，红了（见报告 §4）。
- 注释里写清：这个值本仓**仍然没人读**（`linkparams.go:22-26`），判据存在的理由就是参考写下的那条
  ——「别让一条敌意链接夹带一个后来的读者可能会执行的东西」，以及「判据今天不可观测的那一半」。

### 2. `internal/shell/system.go` 的 register 分支（`:219-239`）

- 现状：`bareCommandName` 之后直接 `a.Open(SurfaceSettings)` + `registerFromLink(name)`。
- 改成：在名字判据之后、`a.Open` 之前插一道 `if !linkArgsFit(parsed.Query().Get("args")) { a.refuseLink(); return }`。
  - 顺序照参考：`parse_url` 先 `validate_command` 再 `validate_arguments`（`:496-503`），所以名字的
    拒绝句先于参数的拒绝句。
  - 拒绝在 `a.Open` 之前：被拒的链接不切页（与 P10-h 的闸门同一个位置纪律）。
  - 复用 `refuseLink` → `settings.deepLinkRejected`，**零新文案**（参考的 `InvalidArguments` 也不是静默的）。

### 3. `docs/extensions/README.zh-CN.md:102`

- 现状：「可选的 `args` 只作为提示展示、宿主从不执行」——没写形状。
- 改成：补一句形状（空格切 token、只允许字母数字与 `. _ + = : , @ %`、不许前导 `-`、单 token ≤ 64、
  ≤ 16 个、整串 ≤ 256）。**不是**静默改承诺：`args` 依旧「只作提示、从不执行」，本轮只是把参考的
  形状写出来（代码现在也这么判）。

## 行为契约零改动红线

- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**（无新文案、无新视图、
  无新接线；拒绝复用既有 `settings.deepLinkRejected`）。
- 本仓自有主机（`settings`/`terminal`/`search`/裸启动器）不接这张表也不接这条判据：它们不是参考动作
  （`linkparams.go:33-37`）。
- 终端拼写 `floter register rg --hidden src` 仍然**丢**掉名字后的词（`linkargs.go:29-33`），
  本轮不把终端行折叠成 `args=`：那会凭空造出一个读者没有的参数。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2096（本轮基线）
GOOS=linux/windows/darwin go build ./...               # 三个都过
```

基线（本轮实跑，不是旧报告数字）：**2096 PASS** / **36 包** / **15 SKIP** / **0 FAIL** /
**1706 测试函数**。

## 台账预判

- **V=0**（无视觉数值）。
- **B=0**（文案、接线、高度链零改动）。
- **S=1**：`internal/shell/linkparams_test.go:197` 的 `args=--hidden` 用例——它本来钉的是「表内键
  不被拒」，但 `--hidden` 现在会被**值判据**拒（前导 `-`），换成合法值 `args=src`，注释指向
  `linkargshint.go`；「前导 `-` 被拒」由新测试钉住。

## mutation 计划（≥2 条，每条说清哪个 pin 变红）

0. （**已实跑的幸存者**）删掉参考式的 `strings.HasPrefix(token, "-")` → 全绿：`-` 不在字符类里，
   前导 `-` 由字符判据拒。所以落地时这条分支没有进仓（见落地项 5）。
1. 把 `-` 加进 `inertArgumentChar` 的字符类 → `-hidden` / `--version=1.2.3` 两格变红
   （`args=--hidden` 会被接受）。
2. 把 `inertArgumentChar` 的字符类放宽成「非空白即接受」→ `src;rm`、`|cat`、`$HOME`、`*`、`~`、
   `"quoted"` 六格变红。
3. 把 `maxArgumentTokens` 从 16 改成 17 → 17-token 用例变红。
4. 把 `maxArgumentsChars` 的总长检查删掉 → 257 字符用例变红。
5. 把 `system.go` 里的调用点删掉 → `TestTheRegisterLinkRefusesAnArgumentsHintThatIsNotInert` 变红
   （这条专门证明判据**真的接在活路径上**，不是只测了纯函数）。
