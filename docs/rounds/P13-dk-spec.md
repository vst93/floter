# P13-dk 规格：更新行的状态字，两种答案是同一档灰

**意图一句话**：About 页更新行的那条状态字有四种答案，此前共用一档副标题墨；让
「有新版」成为事实、让「失败」带上色相，而**一个盒子都不动**。

**为什么现在做**：P13-dj 的「仍待」第 1 条 + 任务书「下一格从状态反馈的另一半（禁用 / 只读面）
挑，先量再改」。§6.10 的 13-5 把这一族（`PermissionBoundary` / `tool.Path` / `updateRow` 的
只读臂）留成「值已在 emphasis 档，无需对调」——`updateRow` 的**标签**确实在 emphasis，但它
下面那行状态字此前**没人量过**，而它是这一页唯一会变的一句话。

## 先量（改前实测，`inkPixel` 的 luma 跨度，1 级 = 1 luma；`updatePage` 760×1400）

| 状态 | 状态字 | 浅 | 深 | 墨 |
|---|---|---|---|---|
| `up-to-date` | Up to date | **101** | **94** | `t.TextMuted` |
| `available` | Latest version: v1.4.0 | **101** | **94** | 同上 |
| `failed` | Update failed, retry? | **101** | **94** | 同上 |
| `disabled` | This build has no update feed | **101** | **94** | 同上 |

四种**不同的意思**，像素上一模一样。同一行的标签 `Current version: v1.2.3` 是 202 / 198
（emphasis / `TextStrong`）—— 这一行上最响的字是**不变的**版本号，最安静的是**会变的**答案。

## 落地项（实为 80-1…80-7，见合约 §6.80）

1. `internal/settingsui/rows.go`：新增 `rowInk`（三档）+ `rowLine.Ink`；普通单行路径按档取墨，
   `Segments` 路径不动；新增 `rowState`（`rowAligned` + 一条带 `Ink` 的 `rowLine`）。
2. `internal/settingsui/settingsui.go:4054` `updateRow`：`switch` 里给 `Failed` 打
   `rowInkWarning`、给 `Available` 打 `rowInkFact`，其余保持 `rowInkQuiet`；调用点 `a.row` →
   `a.rowState`。
3. 新增 `internal/settingsui/updatestateink_test.go`：一条像素 pin（四状态 × 深浅）+ opt-in 出图。
4. `docs/liquid-glass.md` 新增 §6.80。

## 改后实测（同一夹具、同一读法）

| 状态 | 状态字 | 浅 | 深 | 像素（浅 / 深） |
|---|---|---|---|---|
| up-to-date | Up to date | 101 | 94 | `[128 128 135]` / `[133 133 141]` |
| disabled | This build has no update feed | 101 | 94 | 同 |
| available | Latest version: v1.4.0 | **122** | **115** | `[107 107 112]` / `[154 154 160]` |
| failed | Update failed, retry? | **162** | **131** | `[156 31 24]` / `[216 153 142]` |

盒子：四种状态的版本行 18px / x=200.4 / y=366.4、状态行 17px / x=200.4 / y=385.4 —— 逐状态相同
（≤0.5px）。版本行的墨四种状态都是 202 / 198。

## 行为契约零改动红线

高度链（`cardRow` 地板、`rowLineGap` 1px）、键盘模型、会话管理、设置写回、i18n 文案、
命令接线：**零改动**。这一轮只改「一行字的墨」。

## 门槛命令与 PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                     → 空
go vet ./... / windows / darwin           → 净
go test -count=1 ./...                    → 全绿
--- PASS 计数                             → 3315（基线 3314）
--- SKIP 计数                             → 151（基线 150，+1 = 本轮 opt-in dump）
测试函数                                  → 2474（基线 2472）
GOOS=linux/windows/darwin go build ./...  → BUILD-OK
```

## mutation 计划

- **M1**：删掉 `Failed` 那一支的 `ink = rowInkWarning`（状态字回到副标题墨）。
  预期变红：色相那条（`the failed state's ink is [128 128 135] … neutral`）× 深浅，
  以及阶梯那条（`the failed state resolves 101 against the available state's 122 (-21)`）× 深浅。
- **M2**：删掉 `Available` 那一支的 `ink = rowInkFact`。
  预期变红：`the available state resolves 101 against the quiet rung's 101 (+0)` × 深浅。
- 还原后用 `md5sum` 逐字节核对，`touch` 再复跑一次（避免测试缓存）。
