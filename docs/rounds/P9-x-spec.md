# P9-x Spec: 统一“一个文件算不算程序”的判据

## 本轮意图

**统一 `internal/inventory/exceutableFile` 与 `internal/tools/isExecutable` 两处实现 → 抽取为单一源头。**

证据：P9-w “仍待”第 4 条 + `.pending-bugs.md` P9-t 结尾已明确：“两个地方仍是两份实现（P9-t 只统一了 tools 内部）”。用户优先级「三平台一致性 → 基础功能 → 像素细节」。

---

## 落地项（位置 → 现状 → 应改成）

### A. 现状读取（先读后改，确认现行行号）

| # | 文件 | 行号 | 现状行为 |
|---|---|---|---|
| 1 | `internal/inventory/*.go` | ? | `executableFile(path string)` / `var executableFile = isExecutable`（来自同一包内的 `isExecutable`） |
| 2 | `internal/tools/*.go` | ? | `isExecutable(path string)` 函数，含平台分支逻辑（Windows/Unix） |
| 3 | 参考 Tauri | `src-tauri/src/extensions/inventory.rs` | 单一定义（Rust 的 metadata+权限位） |

**动作**：先用 `grep -n` 定位两处函数的真实行号与实现细节。

### B. 抽取单一源头

- 新建 `internal/progtype/progtype.go`：定义 `IsExecutable(path string) bool`
- 两调用处改为导入 `progtype` 并调用它
- 保持行为零改动（先把现有两套完全一样的逻辑迁到一处），后续再决定是否需要差异

### C. 行为契约红线（绝对不动）

- **高度链**、**键盘模型**、**会话管理**
- **设置写回**、**i18n 文案**、**命令接线**
- **视觉**（墨、面、阴影、半径）→ 非视觉轮，零像素改动

### D. 门槛命令 + PASS 基线（实测当前值）

```bash
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿（预期基线 2037 PASS）
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 计数只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...  # 三平台过
```

**测试函数数量不许减少**；若因抽离导致旧 pin 消失，需在新包补等价 pin。

---

## mutation 计划（≥2 条）

M1 `IsExecutable` 恒返回 `true` → 破坏 `inventory.Discover` 的去重/筛选，若干条目不应出现却出现。

M2 `IsExecutable` 恒返回 `false` → 所有候选都不可执行，工具列表为空或只剩目录条目。

**要求**：变异后看到红 → md5sum -c 逐字节还原 → `touch` 复跑一次（避免缓存）。

---

## V/B/S 台账

这是**B 类（行为/接线）**：测试零改动必须全绿；红了是产品回归，改产品不改测试。

若发现第三套实现（如 `main` 分支的 Tauri 版）与现有两套不同 → 记录进 `.pending-bugs.md` 的“需拍板”，**不要擅自实现**。

---

## 为何选这一轮

1. P9-w“仍待”第 4 条点名；用户实测四条卡在实机/拍板；合约 §6 全部标注“已落地”。
2. 符合优先级「三平台一致性 → 基础功能」：两处重复实现本身就是不一致的风险源。
3. 代码层可控、无需实机拍板，一轮内可交付闭环。

---

## 成功标准

- `gofmt/vet/build/test` 全部通过
- PASS 计数不减少（目标 2037+）
- `inventory` 与 `tools` 均导入 `progtype.IsExecutable`
- mutation 至少 2 条实跑变红并还原
- 无新增 t.Skip（除非 opt-in 截图 dump）
