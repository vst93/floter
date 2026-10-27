# P10-m: 启动器宿主族——预填只属于 `search` 与裸启动器两种拼写

## 1. 意图与证据

`HandleURL` 的 `default` 分支把**任何**它不认识的宿主都当启动器，并且只要查询串里有
`q`/`query` 就预填搜索框（改前 `system.go:264-275`）。本仓自己写下的契约是相反的：
「未知/非法 URL 回落启动器且**不动搜索词**」（`docs/mygo-rewrite-plan.md:226`）。

开工前实测（`newApp` + `HandleURL`，`a.Launcher.Query` 读回）：

| 链接 | 改前 | 改后 |
|---|---|---|
| `floter://bogus?q=hello` | `hello` | 不动（保持用户原有词） |
| `floter:connect?q=hello`（无 authority） | `hello` | 不动 |
| `floter:///search?q=hello`（路径形态） | `hello` | 不动 |
| `floter://search?q=json+viewer` | `json viewer` | `json viewer`（不变） |
| `floter://?q=notes` | `notes` | `notes`（不变） |
| `FLOTER://SEARCH?q=x` | `x` | `x`（不变） |

参考一侧：`open`/`connect`/`register` 之外的动作是 `Reject::UnknownAction`
（`deep_link.rs:104`、`:467-471`），它在 `is_silent` 名单里（`:223-228`），`dispatch_url`
直接 return（`:729-734`）——query 循环（`:467-492`）根本到不了，所以「未知动作不读任何参数」
在参考那边是**结构性**的，不是特例。

顺带实测到的一条：`net/url` 只对 special scheme 折宿主大小写，`FLOTER://SEARCH` 解出
`Host = "SEARCH"`。改前之所以「大写也能预填」是因为**根本不看宿主**；补上宿主判据就必须自己
`ToLower`，否则大写拼写会掉档（M3 变异把这条钉住）。

## 2. 落地

| 文件 | 改动 |
|---|---|
| `internal/shell/linkhost.go`（新） | `launcherPrefill(*url.URL) string` + `launcherHost(*url.URL) bool`：一个地方回答「这条链接预填什么」 |
| `internal/shell/system.go` | `default:` 的预填从内联的 `q`/`query` 读取改成一次 `launcherPrefill(parsed)` 调用；那句事实错误的注释（「query rides either the host or the path」）换成真正的规则 |
| `internal/shell/linkhost_test.go`（新） | 2 个测试函数：纯函数真值表（13 格）+ 走 `HandleURL` 的行为表（7 格） |
| `docs/rounds/P10-m-spec.md` | 本轮规格（已随提交进仓） |

判据（`launcherHost`）：

1. `Opaque != ""` → 不是宿主形态（`floter:connect`），不预填；它仍回落启动器（本仓契约），
   参考那边是 `MalformedUrl` 的可见拒绝——这条分歧**原样保留**，只写进注释。
2. 宿主（小写后）`search` → 预填。
3. 宿主为空且路径为空或 `/` → 预填（裸启动器两种拼写）。
4. 其余不预填：`bogus`、`launcher`、`/search` 路径形态。

值的选择顺序不变：`q` 非空用 `q`，否则 `query`；空值等于没写（不清掉用户正在打的字）。

## 3. 台账

- **V=0**：零墨面改动，非视觉轮，不出图。
- **B=0**：文案、键盘、高度链、会话、设置写回、命令接线零改动；无新 i18n 键。
- **S=1**：`default` 分支的预填从内联代码收成一次纯函数调用（+1 文件）。

## 4. 门槛实测（全部本轮实跑）

```
$ gofmt -l cmd internal
(空)
$ go vet ./...
(净)
$ go test -count=1 ./...
ok   ... 36 包全绿
$ go test -count=1 -v ./... | grep -c -- "--- PASS"
2113            # 基线 2111（同一台机、HEAD 181b783 上 git stash 实测）
$ go test -count=1 -v ./... | grep -c -- "--- SKIP"
15              # = 基线
$ go test -count=1 -v ./... | grep -c -- "--- FAIL"
0
$ GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
(通过)
```

测试函数：**1717 → 1719**（+2，`grep -c "^func Test"` 实测；基线数字在 P10-l 报告里记成
1699，与实际不符，本轮以自己量到的 1717 为准）。

## 5. mutation（5 条实跑变红 + 逐字节还原）

| # | 注入的缺陷 | 变红的 pin（原始报错串） |
|---|---|---|
| M0 | `system.go` 的 `default` 分支退回内联读 `q`/`query` | `linkhost_test.go:73: "floter://bogus?q=hello" left the query at "hello", want "keep"`（另两条同族：`floter:connect?q=hello`、`floter:///search?q=hello`） |
| M1 | 删掉 `launcherHost` 的 `Opaque != ""` 闸 | `linkhost_test.go:38: launcherPrefill("floter:connect?q=hello") = "hello", want ""` + `linkhost_test.go:73: "floter:connect?q=hello" left the query at "hello", want "keep"` |
| M2 | 删掉空宿主分支（只留 `search`） | `linkhost_test.go:38: launcherPrefill("floter://?q=notes") = "", want "notes"`（`floter:///?q=notes`、`FLOTER://?q=x` 同红）+ 行为表 `"floter://?q=notes" left the query at "keep", want "notes"` |
| M3 | 宿主不折大小写 | `linkhost_test.go:38: launcherPrefill("FLOTER://SEARCH?q=x") = "", want "x"` + 行为表同格红 |
| M4 | `q`/`query` 次序对调 | `linkhost_test.go:38: launcherPrefill("floter://search?q=a&query=b") = "b", want "a"` |

每条注入后 `touch` 目标文件再跑（避免测试缓存），还原后 `md5sum -c /tmp/p10m.md5` 逐字节核对成功。
M3 第一次写成删掉 `strings` 引用，结果是编译错而不是红 pin —— 按纪律重写成「保留 import、
只去掉折大小写」的形态，才作为证据记在这里。

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 42 轮，本仓无可改代码。
2. 需拍板不变：register 只留 `cmd`（对齐参考＝删三种别名）、`open` 是否拒一切 query、
   独立 review surface、`ShortcutTargetName` 无扩展名返回 `""`、**#692 日语 locale**、
   **#642 `MYGO_LDFLAGS`**。
3. 下一轮候选：**未知宿主仍然回落启动器**（参考是静默忽略、连窗口都不抬）——改它属改交互，
   要拍板；不拍板就继续按「三平台一致性 → 基础功能 → 像素细节」找。
