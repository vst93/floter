# P10-l: `floter://open` 显式分支与 query 判据（参考拒、本仓改交互）

## 意图

交棒条目是「任务书」第一优先级：`floter://open` 的接线与 query 分歧。  
证据：
- 参考 `deep_link.rs:104` `ACTIONS` 包含 `"open"`；`:467-500` 对 `open` 只允许空 query，任何参数 → `UnknownParameter`（`:492`）。
- 本仓 `HandleURL` 的 `switch`（`system.go:200-232`）**没有 `open` 分支**，落到 `default` → 裸启动器 → `floter://open?q=x` 会预填搜索，而非被拒绝。
- `linkparams_test.go:63` 写 `{"open", "q=x", true, "open is ours; the reference's open takes nothing"}` —— 承认是 ours，但**只有表放行一格**，没有行为 pin（是否预填）的测试函数。

本轮目标：**补 `open` 显式分支并钉住「参考拒 query / 本仓拒 query（改交互）」**，不是把 `open?q=` 做成预填搜索。改交互需在 `.pending-bugs.md` 拍板项中记录。

## 落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `system.go:200-232` | switch 无 `open` 分支，落到 default | 加 `case "open":`，不做预填，直接返回或打点，零页面切换 |
| `linkparams.go` | `linkParamSlots["open"]` 未定义 | 新增 `openSlots = []string{}`，`linkQueryFits("open", q)` 要求空查询 |
| 新测试 | 缺 `HandleURL("floter://open?…" )` 的行为 pin | `TestTheOpenActionAcceptsNothing`：接受 `floter://open`；拒 `floter://open?q=x`、`&query=x`；被拒时 surface 不变、弹出拒绝文案 |
| `.pending-bugs.md` | 已有拍板项表 | 追加「register 三种别名是否删」、新增「`open` 的 query 是否改为拒绝」两条 |

## 行为契约红线

- **不改动**：高度链、键盘模型、会话管理、设置写回、i18n 文案（复用现有 `settings.deepLinkRejected`）、命令接线（`open` 不调用 `a.Open(SurfaceXxx)`）。
- **视觉零改动**：非视觉轮，不出图。
- **测试函数不减**：+2 函数（`linkparams_test.go:1` + `shell_test.go:1`），PASS 计数 +2。

## 门槛命令 + PASS 基线

```bash
gofmt -l cmd internal                                   # 必须空
go vet ./...                                             # 净
go test -count=1 ./...                                   # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 实测数字（基线 2110）
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## mutation 计划

≥2 条，每条说明会让哪个 pin 变红：

1. **M1**：删掉 `case "open"` 或在 `linkQueryFits` 放行 `open` 的任意 query → `TestTheOpenActionAcceptsNothing` 的红（`open?q=x` 本该拒）。
2. **M2**：把 `case "open"` 写成 `a.Open(SurfaceLauncher)` 并预填 query → `TestTheOpenActionAcceptsNothing` 的另一半红（surface 不该切换、query 不该被读）。
3. **M3**：把 `linkQueryFits("open", "")` 改成 false → `TestLinkQueryFits` 的那格红（`open` 应该接受空查询）。

## 进度标记

- V/B/S：预计 B=0（产品行为改交互，需拍板）/ V=0（零墨面）/ S=1（新增分支 + 判据表行）。
- PASS 基线：2110 → 2112（+2）。
- SKIP：15（零新增）。
