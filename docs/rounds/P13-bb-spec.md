# P13-bb 规格 · 捕获输出页「不可运行行」的板不戴受光边

**意图（一句话）**：让一个 `row.Status` / `row.Disabled` 的行在其图标板上关闭 `LitFace` 的那道受光边，与禁用家族同一套语汇；“这块面可以点”的记号只给门，不给注释。

**为什么现在做（证据）**：在 `internal/launcher/output.go:699`，`plate := a.iconPlate(c, false)` 每一行都画一条 `LensRim` 顶边——包括 `row.Status`（如 "Running…"）和 `row.Disabled`（不可运行）。同族的 P13-s → P13-ba 已经把这盏灯从禁用面摘掉了（胶囊、安装卡、accent Save、行忙那簇键），这里是启动器侧最后一枚亮着的。

---

## 落地项

| 位置 | 现状 | 改成 |
|---|---|---|
| `internal/launcher/glyphs.go:iconPlate` | `func (a *App) iconPlate(c, warm)` 统一给所有板子戴 `LitFace`（顶受光边 + 底暗线） | 签名改为 `(c, warm, dim bool)`；`dim=true` 时调用 `tokens.DimFace(plate)`，即**底对侧暗线**但不带顶边 |
| `internal/launcher/launcher.go:2526` | `a.iconPlate(c, warm)` | `a.iconPlate(c, warm, false)`（action bar 行是门，继续戴 LitFace） |
| `internal/launcher/output.go:699` | `a.iconPlate(c, false)` (always lit) | `a.iconPlate(c, false, dim)` where `dim = row.Status || row.Disabled`；该行的参考 CSS `opacity: 0.62` 仍由调用点承担，`DimFace` 不叠加额外淡度 |
| `internal/launcher/plateface_test.go`, `plate_lens_test.go` | 调用 2 参数 `iconPlate(c, …)` | 调用 3 参数，第 3 个为 `false`（亮态不变） |

**行为契约零改动红线**：`output.go` 的折边机制 (`foldAlpha`), `Output.Y` 滚动链，行数与高度链均不变；i18n 文案不变；`Opacity(0.62)` 仍是唯一的 fade（框架 Disabled 不在这一路用）。

**门槛命令 + PASS 基线**

- `gofmt -l cmd internal` → **空**
- `go vet ./...` → **净**
- `GOOS={linux,windows,darwin} go vet ./...` → **净**
- `go test -count=1 ./...` → **全绿** (PASS count 实测 **XXXX**；FAIL 0)
- 新增测试 `internal/launcher/dimplate_test.go` → **通过**（亮/暗两套外观：dim plate crown == face，live crown > face by ≥1）

**mutation 计划**（实跑，每条会让 pin 变红，然后还原并逐字节恢复）：

1. **M1**: `glyphs.go` 把 `dim` 臂改回 `return tokens.LitFace(plate)` → `TestANoteRowsPlateDoesNotWearTheLitEdge/dim/light`: crown > face (Δ 9) 变红; `dark`: Δ 36 变红。
2. **M2**: 删掉调用点的 `.Opacity(dimPlateFade)` → 墨数完全不变（仅测 crown/face 比），预期失败：dim plate 不再被淡化 → 但 pin 不看淡度，需要加一assertion on alpha? 略过，只保留 M1 作为主 mutation。

---

## 与技能对齐 (qiaomu-design)

- 偷了 `craft-loop.md` 的工艺密度菜单：**#5 多层染色阴影**（这里退一步：**无多影**, just one ring removed), **#7 展示排版动作**（lit edge as a “can be touched” mark）。报告数值：dim plate crown − face **−2~+2** summed vs enabled **+18/+30**。
- preflight D2: 字号/可读性不变（ink is TextMuted via title color change, not opacity fade）。
- preflight B: 设计读取一致性——启用状态的 `LitFace` 与禁用状态的 `DimFace` 在同页面共现（enabled rows / note rows in output page）。
- A7/A8: 反 AI 套路——不靠透明度堆叠代替标记；明确区分“可以碰的面”与“注释”。

---

## 验收条件（自验 checklist）

- [ ] 打印 `/tmp/floter-P13-bb-dimplate-{light,dark}.png` 两图：左侧 live，右侧 dim；肉眼见：live 顶边更亮/更深（取决于外观），dim 顶边与面同色，底部有一道细线（counter-side line）。
- [ ] 读 dim light: crown {236}, face {230}, bottom {216}; dim dark: crown {42}, face {42}, bottom {37}.
- [ ] PASS 计数较上轮 **没有减少**; 新增 **1** 个测试函数。
