# P11-l 规格 · 同一族的第 4–7 份拷贝收口

**方向**：合约 §6 全「已落地」、上一轮「仍待」清单第 3 条（用户实测 4 条仍卡实机 → 按用户定的「三平台一致性 → 基础功能 → 像素细节」自查）。

---

## 1. 意图

P11-k 新增的 `internal/statefile` 已是状态文件落盘的**唯一实现**，但同族代码还有 4 处各自抄了一份：
- `internal/calculator/calculator.go:495` `saveLocked`
- `internal/extensions/config.go:135` （内联在函数里）
- `internal/extensions/config.go:683` `atomicWriteFile`
- `internal/extensions/repository.go:215` `writeRepository`
- `internal/terminalui/terminalui.go:523` `writeFileAtomically`

它们都无缺陷（全部 fsync + rename），只是冗余。本轮的目标是**让它们全部转发到 `statefile.Write`**，把"同一个问题只留一个答案"贯彻到底。

---

## 2. 落地（逐条核对行号与语义）

### 2.1 `internal/calculator/calculator.go:495` `saveLocked`

**现状**（495–535）：
```go
func (s *Store) saveLocked() error {
    if err := os.MkdirAll(s.paths.Root, 0o755); err != nil {
        return err
    }
    entries := s.entries
    if entries == nil {
        entries = []Entry{}
    }
    data, err := json.MarshalIndent(entries, "", "  ")
    if err != nil {
        return err
    }
    temporary, err := os.CreateTemp(s.paths.Root, ".index-*")
    if err != nil {
        return err
    }
    name := temporary.Name()
    defer os.Remove(name)
    // Write + Sync + Close + Chmod(0o600) + Rename + dir Sync...
}
```

**改成**：
```go
func (s *Store) saveLocked() error {
    data, err := json.MarshalIndent(s.entries, "", "  ")
    if err != nil {
        return err
    }
    return statefile.Write(s.paths.Index(), append(data, '\n'), 0o600)
}
```

**注意点**：
- `MkdirAll` 由 `statefile.Write` 内部处理
- `entries == nil` → `[]Entry{}` 的零值处理由 JSON marshal 自动完成
- `\n` 后缀保留
- 模式 `0o600` 不变
- `defer os.Remove` 不再需要（成功时才命名、删除由 `os.Rename` 负责原子性）

**调用点检查**：grep 确认调用位置（例如 tests / other funcs），确保改动前后行为一致。

---

### 2.2 `internal/extensions/config.go:135` 内联块

**现状**（约 117–153）：这段代码在某个函数内联写配置，模式 `0o600`，temp 前缀 `.config-*`，末尾带 `\n`。

**改成**：替换整个临时文件块为 `statefile.Write(path, data, 0o600)`。

（需读取完整上下文确认路径是什么、data 是否已含`\n`）

---

### 2.3 `internal/extensions/config.go:683` `atomicWriteFile`

**现状**（679–702）：
```go
func atomicWriteFile(path string, data []byte) error {
    dir := filepath.Dir(path)
    temporary, err := os.CreateTemp(dir, ".config-*")
    // ... Write+Sync+Close+Chmod(0o600)+Rename
}
```

**改成**：
```go
func atomicWriteFile(path string, data []byte) error {
    return statefile.Write(path, data, 0o600)
}
```

**注意点**：
- 原函数的 dir MkdirAll 逻辑隐含在调用点上，需检查 `statefile.Write` 是否自行 MkdirAll
- 不追加 `\n`（原文档说 `data` 已处理好）
- 模式 `0o600` 不变

---

### 2.4 `internal/extensions/repository.go:215` `writeRepository`

**现状**（200–240）：
```go
tmp, err := os.CreateTemp(dir, ".extension-repository-*.tmp")
// ... Write+Sync+Close+Chmod(0o644)+Rename+dir Sync
```

**改成**：
```go
return statefile.Write(path, data, 0o644)
```

**注意点**：
- 模式是 `0o644`（不同于其它 `0o600`），需准确传递
- `MkdirAll` 已在前面调过，`statefile.Write` 会重复调但不影响（幂等）

---

### 2.5 `internal/terminalui/terminalui.go:523` `writeFileAtomically`

**现状**（516–541）：
```go
func writeFileAtomically(path string, data []byte) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return err
    }
    temporary, err := os.CreateTemp(dir, ".snapshot-*")
    // ... Write+Sync+Close+Chmod(0o600)+Rename
}
```

**改成**：
```go
func writeFileAtomically(path string, data []byte) error {
    return statefile.Write(path, data, 0o600)
}
```

**注意点**：
- `data` 不带 `\n`（终端快照二进制）
- 模式 `0o600` 不变

---

## 3. V / B / S 台账预测

| 类别 | 数量 | 说明 |
|---|---|---|
| V（视觉数值） | 0 | 非视觉轮 |
| B（行为/度量/接线） | ~5 | 所有改动的断言都应保持绿；若有红则是行为回归（应该改产品而不是测试） |
| S（结构变了） | 0–2 | 若某测试钉的是临时文件前缀（如 `.config-*` vs `.floter-state-*`），则需重定基线 |

**判据**：除了 calculator 可能钉了 `.index-*` 外，其余都不该有 pin 变动——它们的调用点不关心临时文件名，只关心最终结果正确。

---

## 4. 门槛命令（本轮必须全过）

```bash
gofmt -l cmd internal                          # 必须空
go vet ./...                                    # 必须净
go test -count=1 ./...                          # 全绿
go test -count=1 ./... | grep -c -- "--- PASS"  # 计数只涨不跌
GOOS=linux go build ./... && \
GOOS=windows go build ./... && \
GOOS=darwin go build ./...                      # 三平台 build 过
```

**PASS 基线**：当前 2262，期望 ≥ 2262（新增的转发函数可能有新测试或不增加）。

---

## 5. mutation 计划（≥2 条）

每条 mutation 都要让某个 pin 变红，并记录原始错误信息。

### Mutation 1：删掉 `statefile.Write` 内的 `syncFile`

编辑 `internal/statefile/statefile.go`，注释掉或删掉 flush 那一步。

**预期结果**：
- `calculator` 或 `extensions` 或 `terminalui` 的写失败回滚测试会变红
- 或者出现 "data loss simulation" 类的 pin 变红

### Mutation 2：把 `renameFile` 换成就地写

编辑 `internal/statefile/statefile.go`，把 `os.Rename(name, path)` 换成 `os.WriteFile(path, data, mode)`。

**预期结果**：
- "rename failure leaves target untouched" 类测试变红
- 证明原子性的必要性

### Mutation 3：篡改任意一个调用点

临时把 `calculator.saveLocked()` 改回手写 temp 块（不调 `statefile.Write`）。

**预期结果**：
- 如果本次轮做了"源码扫描 pin"（契约测试），它会变红
- 否则这一条可能不会直接暴露 —— 可以列为下轮候选

---

## 6. 报告与提交格式

**commit message**：
```
refactor(statefile): 同一族第 4–7 份拷贝收口——calculator/extensions/terminalui 转发 statefile.Write (P11-l)

- calculator.saveLocked → statefile.Write (补上 MkdirAll)
- extensions/config.go 两处 → statefile.Write (.config-* 前缀统一)
- extensions/repository.go → statefile.Write (模式 0o644)
- terminalui/writeFileAtomically → statefile.Write (.snapshot-* 前缀统一)

台账：V=0 / B≈5 / S=0–2
门槛：gofmt 空 | vet 净 | PASS 226X（基线 2262）· SKIP 15 · FAIL 0 | 三平台 build 过
mutation：2–3 条实跑变红并还原
```

---

## 7. 收尾日志格式（~/.hermes/state/iteration/floter-done.log）

```
P11-l 同一族第 4–7 份拷贝收口
改了：calculator.go saveLocked、config.go x2、repository.go writeFileAtomically、terminalui.go writeFileAtomically
核心改动：5 处转发 statefile.Write，删除冗余 temp+fsync+rename 代码
门槛：PASS 226X（基线 2262）· gofmt 空 · vet 净 · 三平台 build 过
变异：2–3 条实跑变红并还原
下轮：① 用户实测四条第 67 轮 ② 源码扫描 pin（确保 internal/ 不再出现 os.CreateTemp）
commit: <sha>
```

---

## 8. 风险与取舍

1. **行为回归风险低**：这 4 处原本就正确实现了 fsync+rename，改成同一实现后行为应完全一致。
2. **测试 pin 可能变化**：若有测试钉的是临时文件前缀（如 `.config-*`），需要改断言为 `.floter-state-*`，属 S 类。
3. **没有视觉变更**：输出逐字节相同（除可能的文件名变化），不需要截图验证。
4. **代码缩减**：预计删除约 100–120 行重复代码，提高可维护性。

---

## 9. 成功标准

- [ ] 所有 5 处转发完成，原有临时文件实现全部删除
- [ ] gofmt、vet、test、build 全过
- [ ] PASS 计数不低于 2262
- [ ] mutation 至少 2 条实跑变红
- [ ] 报告 + done.log 写入仓库与日志
- [ ] git commit + push origin mygo-rewrite

---

**备注**：此轮完成后，下一步可以是：
1. 继续做"源码扫描 pin"契约测试（确保 `internal/` 内除 `statefile` 外不再有 `os.CreateTemp`）
2. 回到用户实测 4 条继续跟进（虽然本仓无可改代码，但仍需记录复测结果）
3. 或转向其他拍板项（快捷键词序、`Esc`显示等）
