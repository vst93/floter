# P10-m: 启动器宿主族——预填只属于 `search` 与裸启动器两种拼写

## 意图

`HandleURL` 的 `default` 分支（`system.go:264-275`）把**任何**它不认识的宿主都当成启动器，
并且只要查询串里有 `q`/`query` 就预填搜索框。本仓自己写下的契约是相反的：
「未知/非法 URL 回落启动器且**不动搜索词**」（`docs/mygo-rewrite-plan.md:226`）。
参考那边更彻底：动作不在 `ACTIONS` 里就是 `Reject::UnknownAction`，**静默**
（`deep_link.rs:104`、`:223-228`、`:729-734`），连 query 循环都到不了。

实测（本轮开工前，`newApp` + `HandleURL`）：

| 链接 | 现状 | 契约 |
|---|---|---|
| `floter://bogus?q=hello` | `Query = "hello"` | 不动搜索词 |
| `floter:connect?q=hello`（无 authority） | `Query = "hello"` | 不动搜索词（参考这里是 `MalformedUrl`） |
| `floter://search?q=json+viewer` | `Query = "json viewer"` | 预填 |
| `floter://?q=notes` | `Query = "notes"` | 预填 |
| `FLOTER://SEARCH?q=x` | `Query = "x"` | 预填（宿主大小写不敏感，与 `action` 的 `ToLower` 一致） |

`net/url` 对本 scheme **不**小写宿主（实测 `FLOTER://SEARCH` → `Host = "SEARCH"`），
所以「宿主是不是我们的」这条判据必须自己 `ToLower`，否则大写拼写会掉档。

## 落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/shell/system.go:264-275` | `default:` 里直接读 `q`/`query` 并 `SetQuery`，不看宿主 | 改调 `launcherPrefill(parsed)`（新文件 `linkhost.go`），为空则不碰搜索框 |
| `internal/shell/system.go:266-268` | 注释「The query rides either the host (floter://?q=x) or the path (floter://search?q=x)」——事实错误：两者都是 **query 串**，变的是**宿主** | 改成「宿主是 `search` 或空（裸启动器）时预填；`q` 先于 `query`；空值等于没写」 |
| 新 `internal/shell/linkhost.go` | — | `launcherPrefill(*url.URL) string` + `launcherHost(*url.URL) bool`：一个地方回答「这条链接预填什么」 |
| 新 `internal/shell/linkhost_test.go` | — | 2 个测试函数：纯函数真值表 + 走 `HandleURL` 的行为表 |

判据（写进 `launcherHost`）：

1. `Opaque != ""` → 不是宿主形态（`floter:connect`），不预填。
2. 宿主（小写后）是 `search` → 预填。
3. 宿主为空 **且** 路径为空或 `/` → 预填（裸启动器 `floter://?q=`、`floter:///?q=`）。
4. 其余一律不预填：`floter://bogus`、`floter:///search`（路径形态不是宿主形态）、`floter://open`
   （`open` 自己的 query 判据在 `linkparams.go` 里先拒，到不了这里）。

值的选择顺序照现状保留：`q` 非空就用 `q`，否则用 `query`；两者都空（或值为空串）则不预填、
**不**清空用户已有的搜索词（一次坏链接不该清掉用户正在打的字）。

## 行为契约红线

- **不改**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线。零新文案。
- **不改**：未知宿主仍然「回落启动器」（`docs/mygo-rewrite-plan.md:226` 的既有契约，
  参考是静默忽略；这条分歧保留，只在 pin 的注释里写明）。
- **不改**：被拒链接的 `refuseLink` 路径（P9-m / P10-l 已钉）。
- 非视觉轮，不出图。

## 门槛命令 + PASS 基线

```bash
gofmt -l cmd internal                                   # 必须空
go vet ./...                                            # 净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 基线 2112（P10-l 末，本轮实测）
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## mutation 计划（≥2，每条说明让哪个 pin 变红）

1. **M1**：`launcherHost` 去掉 `Opaque != ""` 那道闸 → `floter:connect?q=hello` 那格红
   （行为表里「无 authority 不预填」）。
2. **M2**：`launcherHost` 只认 `host == "search"`（删掉空宿主分支）→ `floter://?q=notes` 那格红。
3. **M3**：`launcherHost` 不 `ToLower` → `FLOTER://SEARCH?q=x` 那格红。
4. **M4**：`launcherPrefill` 把 `q` 与 `query` 的次序对调 → 优先级那格红。

## V/B/S 台账（预计）

- **V=0**：零墨面改动。
- **B=0**：契约零改动（本次是把产品对齐到自己写下的契约，不新增文案/不改键盘与高度）。
- **S=1**：`default` 分支的预填从内联改成一次纯函数调用（结构）。
