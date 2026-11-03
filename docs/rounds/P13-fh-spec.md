# P13-fh 规格：集成页结果格最长的那句话，量在它真正会被画的那个宽度上

## 意图

P13-fg「下一格」第 1 条留下的问题：`Result` 格里最长的答案词 `Succeeded with warnings`（en）
会不会被 `metadataCell` 的 `SingleLine()` 切掉，切在哪，要不要让标签先让位。

**这一轮先量**。量完的结论决定改不改——如果这个宽度在应用里根本不会出现，那就不该为它
加换行规则（多一行 = 多一层高度，正好是「空间感」的反面）。

## 为什么现在做

- 证据链：`docs/liquid-glass.md` §6.128（P13-fg）末尾「下一格（首选）」，`docs/rounds/P13-fg-report.md`
  的「仍待」第 1 条。
- 代码：`internal/settingsui/settingsui.go:6133-6140`（`metadataCell`，值那一行是 `.SingleLine()`），
  调用点在 `:6084-6087`（`freshnessSection` 的两格）。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/freshnessanswer_test.go`（新） | 无 | 新 pin：在 `launcher.InputWindowWidth`（= 720，设置面板的唯一生产宽）那一帧上，en/zh × 深浅 × 五个结果词的**墨跨**必须等于它在 1600u 下的自然宽；格盒不窄于自然宽 |
| 2 | 同上 | 无 | 新 pin：最长答案的余量 ≥ 自身宽度的 1/4；标签与值同列、值不压标签 |
| 3 | `freshnessSection` / `metadataCell` 的几何 | 两列 1fr、`GapX(Space(3))`、`SingleLine()` | **一字节不动** |
| 4 | 窄窗换行 / 两格堆叠 / 标签让位 | 无 | **不做**（不可达，见下表） |
| 5 | 把面板改成可缩放 / 允许更长的结果词 | 现状 | `需拍板`（窗口模型 / i18n 文案属行为契约） |

## 先量的表（实测，`TestProbeP13FHResultCell`，探针已删）

| 面板宽 | 结果格盒宽 | 墨列 | 句子 |
|---|---|---|---|
| 1600u | 683.5u | 129 | 整句 |
| 860u | 313.5u | 129 | 整句 |
| 720u | 243.5u | 129 | 整句（余 114.5u） |
| 560u | 163.5u | 129 | 被切 |
| 460u | 113.5u | 98 | 被切 |

不可达的两条依据：`internal/shell/shell.go:4743-4745`（`SurfaceSettings` 一律 720 宽、不开 resize）、
`internal/i18n/i18n.go:1303-1310`（只有 en / zh，zh 那句更短）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。本轮只加测试文件。

## 门槛命令与 PASS 基线（实测当前值）

```
gofmt -l cmd internal                                   # 空
go vet ./...                                            # 净
GOOS=windows go vet ./...                               # 净
GOOS=darwin  go vet ./...                               # 净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 3406（基线 3404）
GOOS=linux/windows/darwin go build ./...                # 过
```

## mutation 计划

1. **M1**：`internal/i18n/copy.go` 的 en `FreshnessResultDegraded` 加长到 247u（> 243.5u）→
   两条 pin 必须红（格盒不足 + 余量为负）。
2. **M2**：`metadataCell` 的值字号 `t.FontSize` → `t.FontSize*2`（自然宽 252u）→ 两条 pin 必须红。

还原后 `md5sum -c` 逐字节核对 + `touch` 复跑（避免测试缓存复用）。
