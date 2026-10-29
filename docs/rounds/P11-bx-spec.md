# P11-bx 规格：设置页的临时状态也进「字压字 / 出窗」审计

## 意图

设置页的**非静止状态**（动作在飞、录制被拒、录制中、集成列表为空）此前从没进过尺子；
它们由同一批行构造器画，但**词更长、盒子一样大**，所以「静止页干净」推不出「它也干净」。
本轮把那四个状态挂进 `auditSettingsFrame`，并把「这条臂真的改变了画面」写成断言，
让它成为常驻 pin 而不是一次性探针（P11-bu 的规矩）。

为什么现在做：用户实测反馈 F1–F4 已全部收完 → 合约 `docs/raycast-design.md` §6 全 `已落地`
（只剩拍板项）→ 上一轮（P11-bw）「仍待」第 1/2 条是拍板项、第 3 条「尺子的第四处盲点」卡在
框架不导出裁剪前的几何（`ui.Tester` 只有 `Find`/`Texts`/`Image`，没有场景树，已核 v0.7.3）→
按用户定的下一档（界面质感与交互 → 基础功能 → 三平台一致性 → 像素细节）落在**界面质感**里
一件不需要拍板的事：把没量过的状态量一遍。

## 逐条落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/settingsui/textframe_test.go:100` | `TestTheSettingsPagesPaintNoTextOverText` 只跑五个页面的**静止**帧 | 新增同族 pin `inlineframe_test.go`，四个临时状态各一臂 |
| `internal/settingsui/settingsui.go:3555` 拒绝句 / `savealert.go:156` | 拒绝录音的两句话从没进过任何审计 | 进审计（Shortcuts 页臂） |
| `internal/settingsui/settingsui.go:3667` 录制中 | 录制面板占着键帽的位置，从没进过审计 | 进审计 |
| `internal/settingsui/settingsui.go:2702` 重检在飞 | `busyActionButton` 用了 spinner + 词，从没进过审计 | 进审计（Integrations 页臂） |
| 空集成列表 | 空态那句话从没进过审计 | 进审计 |

**行为契约零改动红线**：产品代码零改动（`settingsui.go` / `rows.go` / `savealert.go`
逐字节不变，md5 见报告 §3）。高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线一律不碰。

## 每条臂的自证（没有它，改个字段名就退化成「画静止页并绿」）

1. 该臂的帧必须与**同页同外观同尺寸的静止帧**有像素差；差 0 → 红。
2. 状态自带句子的臂，那句必须在 `Texts()` 里（busy 臂的 pill 词与静止态相同，靠第 1 条）。

## 门槛命令与 PASS 基线（本轮实测，2026-10-29）

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净
GOOS=windows go vet ./...             # 净
GOOS=darwin  go vet ./...             # 净
go test -count=1 ./...                # 全绿，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"    # 2595（基线 2562）
go test -count=1 -v ./... | grep -c -- "--- SKIP"    # 54（不变）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l   # 2029（基线 2028）
GOOS=linux/windows/darwin go build ./...                             # 三平台过
```

## mutation 计划（≥2 条；每条说明它让哪个 pin 变红）

1. **M1（自证臂本身）**：把审计尺寸表改成含 `640×520` → Integrations 页的行在这么小的窗口里
   落在折叠线以下，busy 臂画出与静止帧**逐字节相同**的帧 → 4 个子测试红（en/zh × light/dark）。
   预期红。
2. **M2（产品）**：`savealert.go:157` 的 `if a.rejectedShortcut == ""` 取反（拒绝句永不画）→
   预期 `want` 断言红。**实测绿**（见报告 §4.2：同一句话在行下也画了一遍）。
3. **M3（产品）**：`rows.go:594` busy pill 的 `Gap(1.25)` → `Gap(-3)`（spinner 压到词上）→
   预期字压字红。**实测绿**（尺子只量字盒，图标与字互压它看不见）。
4. **M4（产品）**：拒绝句字号 ×6 → 预期出窗红。**实测绿**（`r.W >= w-1` 的整宽文本被尺子跳过）。
