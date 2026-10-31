# P13-ad 报告 · 指针的颜色过渡收成一条算术（`theme.ColorStep` = 120ms）

规格：`docs/rounds/P13-ad-spec.md`。方向：**细节打磨**（通用队列「状态反馈 / 一致性」，
不是新表面，§6 差距表仍然没有 `未开始` 行）。

**这轮为什么让它更轻**：轻不是「动画更快」，是**节拍只有一条**。指针扫过一张浮纸里的行
时，那张纸比它脚下的列表慢 25%（150ms vs 120ms），读起来就是两块不同年头的界面；
而合约上写着 90ms——一个在 `internal/` 与 `cmd/` 里**从未出现过**的数字。

## 一、先量后改（实测数字）

`grep -rn "Colors: true" --include=*.go internal/ cmd/`（非测试）：

| 事实 | 位置 |
|---|---|
| 颜色过渡共 **12 处**：11 处 `120 * time.Millisecond` + 1 处 `150 * time.Millisecond` | 见下表 |
| `rowTransition` 不是颜色过渡：`Colors: false`、`Enter: {Opacity: 0}`，是**行的入场** | `internal/launcher/launcher.go:966-970` |
| 合约写的 `90ms` 在代码里出现 **0 次** | 全仓 grep |
| 框架默认时长是 **200ms**（`Duration` 为零时），所以手写的 120ms 不是冗余 | mygo `ui/transition.go:19` |

`--dur-1` = 80ms（transform，框架按 render-time transform 播，不走这条时长）、
`--dur-2` = 120ms（颜色）、`--dur-3` = 160ms（浮层/带子入场）——参考 CSS 的三个步长，
本仓的 `launcher.go:3441-3444` 注释里早就记着，只是从来没落成一个常量。

## 二、落地

| # | 位置 | 改动 |
|---|---|---|
| 1 | `internal/theme/theme.go` | 新增 `const ColorStep = 120 * time.Millisecond`（`--dur-2`），注释写清另两步为什么**不是**它 |
| 2 | `internal/launcher/filters.go` | 1 处 → `theme.ColorStep`；`time` 导入换成 `theme` |
| 3 | `internal/launcher/launcher.go` | 2 处（行内 20u 控制格 / 字段行的设置键） |
| 4 | `internal/launcher/power.go` | 1 处（电源确认的行） |
| 5 | `internal/settingsui/parameditor.go` | 1 处；`time` 导入换 `theme` |
| 6 | `internal/settingsui/rows.go` | 3 处（两颗动作胶囊 + 裸便条键） |
| 7 | `internal/settingsui/settingsui.go` | 2 处（侧栏行 / 运行药丸） |
| 8 | `internal/settingsui/uninstall.go` | 2 处；**重定基线 150 → 120**（那 150ms 是组件自己的 CSS，注释里留了出处与回退条件） |
| 9 | `internal/ci/colorstep_contract_test.go` | **新增两条账**：常量是写死的 120ms（不许拿常量判常量）；任何 `Colors: true` 只许写 `theme.ColorStep`，且**位置表过期即红** |
| 10 | `docs/liquid-glass.md` §5 | 90ms → **120ms**，带 旧值→新值 与出处 |
| 11 | `docs/raycast-design.md` §4.8 | 把「`rowTransition` = 90ms 颜色过渡」拆成两行事实（入场 120ms `Colors: false` / 颜色过渡 `theme.ColorStep`） |

**行为契约零改动**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线一字节没动。
起止颜色、`LitFace` 的一对线、`ControlPress` 按下档、`Position` 过渡全部不变——**只改时间**。
`rowTransition`（120ms 入场）、toast 的 160ms 入场、`--dur-1` 的 80ms transform 都没碰。

## 三、门槛（全部实测）

```
gofmt -l cmd internal                        → 空
go vet ./...                                 → 净
GOOS=windows go vet ./... / GOOS=darwin …    → 净
go test -count=1 ./...                       → 全绿
PASS 3021 → 3023   FAIL 0 → 0   SKIP 85 → 86   测试函数 2227 → 2230
GOOS=linux/windows/darwin go build ./...     → 三平台过（vet 也过，含 _test.go）
```

新增的 2 个 PASS = 两条新账（`TestTheColorStepIsTheReferenceDurTwo`、
`TestEveryColorTransitionRidesOneStep`）；新增的 1 个 SKIP = `TestDumpP13adColorStep`
（opt-in 截图 dump，默认 `t.Skip`，只在 `FLOTER_P13AD_DUMP=1` 时出货）；
新增的 3 个测试函数就是这三条。

## 四、变异（3 条，全部实跑变红并逐字节还原）

| # | 注入 | 结果 |
|---|---|---|
| M1 | `theme.ColorStep` 改成 `150 * time.Millisecond` | `--- FAIL: TestTheColorStepIsTheReferenceDurTwo` — `theme.ColorStep is not the reference's --dur-2 (120ms) spelled out; …` |
| M2 | `settingsui/uninstall.go` 退回字面 `150 * time.Millisecond` | `--- FAIL: TestEveryColorTransitionRidesOneStep` — `internal/settingsui/uninstall.go:438: a colour transition with its own duration — the pointer has one step (theme.ColorStep)` |
| M3 | 删掉 `launcher/power.go` 那条 `Transition` | `--- FAIL: TestEveryColorTransitionRidesOneStep` — `internal/launcher/power.go: 0 colour transitions, want 1 — the table and the code disagree` |

还原：`md5sum -c /tmp/m.md5` → `theme.go: 成功` / `uninstall.go: 成功` / `power.go: 成功`，
之后 `touch` 复跑（上面三次 `-count=1` 均为冷跑，无缓存复用）。

## 五、出图

`FLOTER_P13AD_DUMP=1 go test -count=1 -run TestDumpP13adColorStep ./internal/settingsui/` →
`/tmp/floter-P13-ad-uninstallcard-dark.png`、`/tmp/floter-P13-ad-uninstallcard-light.png`
（500×200，深 402 色 / 浅 385 色）。

**这张图上看到的一处变化：没有变化，而这正是结论。** 穿过卡体的那一列（x=250）：
深色 `ground 24,25,29 → 冠线 63,63,66 → 面 48,48,52`；浅色
`ground 247,247,249 → 边 216,216,217 → 冠线 232,232,233 → 面 230,230,231`。
浅色那 +2 级正是 §6.1 记下的「白压白」——这一轮一字节没动它，靠的是对侧那条 216 分家。
**诚实说明**：`ui.Tester` 的 `Frame()` 会 `settle()` 到过渡跑完，所以**拿不到中间帧**，
时长改动无法在静态 PNG 里表现；本轮的证据是上面两条账 + 三条变异，不是图。

## 六、偷了什么（qiaomu-design）

读的：`SKILL.md` 的「已有方向，只要打磨」车道 → `references/user-preferences.md`
（P-1 禁斜体 / P-5 可读性优先 / P-38 禁左侧竖线 / P-39 并列工作区隔离，本轮均不涉文案与几何）
→ `references/craft-loop.md` 的工艺密度菜单。

| 菜单项 | 落成什么 |
|---|---|
| **第 4 项「签名交互」**（单页签名交互 ≤1） | 落成 **`theme.ColorStep = 120 * time.Millisecond`**：整个应用里指针的每一个答案（面抬起 / 按下 / 离开）都是同一条 120ms 的节拍；`--dur-1`（80ms transform）与 `--dur-3`（160ms 浮层入场）被**显式**排除，写进常量注释与 §5 的表格。签名交互只有这一个，没有再加第二个。 |
| **第 12 项「材质细节：内边框高光」** | 本轮**核实不动**：浅色档冠线 232 对面 230 = **+2 级**，深色 63 对面 48 = **+15 级**——浅色靠的是对侧边 216（−14 级）分家。这条已有账（§6.1），本轮只补了实测数字。 |
| 第 5 项「多层染色阴影」 | 不涉及（E2 的两层影 P12-a 已落地，本轮不碰）。 |

**没做**：Phase 2 的四方向预览（用户方向已给死：轻便液态玻璃，自动车道不停下来等点选）、
HTML 预览页（同上）。

## 七、preflight 逐条

- **A 反 AI 套路**：过。没有玻璃叠玻璃、没有渐变白边、没有模糊冒充；本轮甚至不新增任何面。
- **B 设计读取一致性**：过。`theme.ColorStep` 是**唯一**入口，CI 扫全仓（`internal/` + `cmd/`），
  唯一例外是 `internal/theme` 里那条定义；位置表 7 个文件 12 处与代码逐一对上（M3 证明它会过期报警）。
- **C 动效抽查**：过。90ms（文档，从未实现）→ 120ms（实测、`--dur-2`）；150ms（1 处离群）→ 120ms；
  80ms transform 与 160ms 入场**未受影响**（它们不是颜色过渡）。不做弹跳、不做整卡 pose transform。
- **D2 中文页**：不适用——本轮零文案改动（i18n 是行为契约红线）。文案密度、斜体、装饰字体均未触碰。

## 八、仍待

1. `ui.Tester` 的 `settle()` 让「过渡时长」无法用像素证明（本轮已记）。要真验时长，得给
   tester 一个可控时钟（改框架或加一个本仓的 headless 时钟）——**需拍板**（会动测试基础设施）。
2. P13-ac 的「仍待」1（顶端 100% 档靠两层影分家）与 3（历史条目）原样有效。
3. 本轮没有解决、也不该由视觉轮解决的一条：`settingsui/uninstall.go` 的组件行**不再**照抄
   参考 CSS 的 150ms。若将来要回退，必须同时改 `theme.ColorStep` 与 §5，否则 CI 账会红。
