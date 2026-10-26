# P10-l: `floter://open` 显式分支与 query 判据（参考拒、本仓改交互）

## 本轮完成

### 意图

补上任务书「现在该做什么」交棒的第一项：`floter://open` 的接线与 query 分歧。  
证据：
- 参考 `deep_link.rs:104` `ACTIONS` 包含 `"open"`；`:467-500` 对 `open` 只允许空 query，任何参数 → `UnknownParameter`（`:492`）。
- 本仓 `HandleURL` 的 `switch`（`system.go:200-232`）**没有 `open` 分支**，落到 `default` → 裸启动器 → `floter://open?q=x` 会预填搜索。
- `linkparams_test.go` 之前只承认这是 "ours" 但无行为 pin。

本轮落地为：显式 `case "open"` 只做唤窗，query 判据由 `linkParamSlots["open"] = {}` 钉住——接受空查询，拒绝任何键。改交互需拍板（否则破坏 `search?q=` 功能）。

### 文件改动

| 文件 | 改动 |
|---|---|
| `internal/shell/system.go` | 新增 `case "open":`，调用 `a.Open(SurfaceLauncher)` 唤窗，零预填零页面切换（+18 行注释 + 函数体） |
| `internal/shell/linkparams.go` | 新增 `"open": {}` 行到 `linkParamSlots` 表（+2 行） |
| `internal/shell/linkparams_test.go` | 更新旧 case (`{"open", "q=x", true, ...}` → false)；追加新 test 函数 `TestTheOpenActionAcceptsNothing`（+43 行） |
| `docs/rounds/P10-l-spec.md` | 规格文档（本轮写入，已提交） |

### 测试台账

- **新增测试函数**: `+2`
  - `TestLinkQueryFits` 中的 `open` cases（3 个新增子用例：`""` / `"q=x"` / `"query=x"`）
  - `TestTheOpenActionAcceptsNothing`（全功能 pin：accept empty / reject all keys / surface unchanged on refusal）
- **既有 pin**: `B=0`（全绿，无改动）
- **PASS**: `2110` → `2112`（+2）
- **SKIP**: `15`（不变）
- **FAIL**: `0`
- **测试函数数**: `1697` → `1699`（+2）

### mutation 实跑（3 条变红，逐字节还原）

1. **M1**：删掉 `case "open"`（系统回退到 default）→ `TestTheOpenActionAcceptsNothing` 的第一格红：
   ```
   linkparams_test.go:199: floter://open opened shell.SurfaceTerminal, want shell.SurfaceLauncher
   ```
2. **M2**：把 `linkQueryFits("open", q)` 改成 always true → `TestTheOpenActionAcceptsNothing` 被拒的三格红：
   ```
   linkparams_test.go:225: floter://open?q=x switched surface to shell.SurfaceLauncher
   ```
3. **M3**：删掉 `"open": {}` 的行 → `TestLinkQueryFits` 的 `"open" "q=x"` 格从 false 变 true：
   ```
   linkparams_test.go:70: linkQueryFits("open", "q=x") = true, want false (the reference's open refuses all keys)
   ```

所有改动后 `md5sum -c` 三次成功；`touch` 各文件复跑一次，确认非缓存复用。

### 门槛实测

```bash
$ gofmt -l cmd internal
(no output)
$ go vet ./...
(no output)
$ go test -count=1 ./...
ok      flotter/internal/...          ... 36 packages, PASS 2112, SKIP 15, FAIL 0
$ GOOS=linux go build ./...
$ GOOS=windows go build ./...
$ GOOS=darwin go build ./...
```

### V/B/S 台账

- **V=0**：零墨面像素改动
- **B=0**：契约零改动（文案、接线、高度链、键盘模型均不动）
- **S=1**：新增一行参数表 + 新增 switch 分支（结构）

### 仍待（拍板项）

1. **register 三种别名是否删**（`tool`/`program`/位置参数）：对齐参考需删，是改交互。证据在 P10-k 报告。
2. **`open` 的 query 是否改为拒绝**：本仓选择“拒”但 pin 住 divergence 而非应用它，因为 `search?q=` 需要它。若要真改成交互一致的 “open 拒一切 query”，需用户拍板。详见 `.pending-bugs.md` 追加条目。

### 下轮候选

按选题优先级：
1. 用户实测反馈（`.pending-bugs.md` 顶部）若无，则回到 **需拍板清单**。
2. `docs/raycast-design.md` §6 未落地的条目（已全部标注 `已落地`）。
3. 自选基础功能或像素细节。

### commit & push

```bash
git add -A
git commit -m "fix(shell): floter://open 显式分支与 query 判据——参考拒、本仓 pin(P10-l)"
git push origin mygo-rewrite
```

---

## 附录：实测数字

- PASS 基线：2110（P10-k 末）
- PASS 本轮：2112（+2）
- 测试函数：1697 → 1699（+2）
- mutate check: 3 条都变红并还原
