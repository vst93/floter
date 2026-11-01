# P13-da 报告 · 会话行的高度：那道 56 的地板在生产上已经成立（先量，不改）

**结论先行**：P13-cz 留下的「行步距 46~47px」是**夹具**的读数；生产的会话行**恰好 56**，
就是参考 `.session-manager__row { min-height: 56px }` 那个数。补地板在生产上零像素变化，
所以本轮**不改画笔**，交付的是**账**（两条 pin + §6.70 + §6.69 的更正）。另：HEAD 上
`internal/ci` 是红的（P13-cz 在差距表里写了四个合法词之外的 `故意不改`），本轮顺手修绿。

## 1. 为什么是它

P13-cz 的「仍待」第 1 条原文：会话行步距实测 **46~47px**，参考 `.session-manager__row`
是 `min-height: 56px`，要不要补地板「先对参考 CSS 逐字段核一遍再定」。本轮就是把这句话
量清楚——它不需要拍板，只量、只钉。

## 2. 量法（可复现）

`sessionRowFrame`（`internal/settingsui/sessionrowbox_test.go`）：一帧 720×760，点侧栏
`Sessions`；在 `x=238` 一列（卡片内、避开前导 chip 的 `≤232`、避开标签列 `244`）从地面色
往上扫出卡片盒，行盒 = 卡片面（`cardTop+1 .. cardBottom-2`，`card()` 是上 1px + 下 2 条线）。

| 夹具 | 卡片盒 | 行盒 |
|---|---|---|
| 只有状态行（生产形状：`Created` 非零） | 177..235 | **178..233 = 56** |
| 带 cwd（三行字） | 177..251 | 178..249 = 72 |
| 只有标题（P13-cz 的夹具） | 177..224 | 178..222 = 45 |

行内（同一帧，`tt.Find` 读文字盒）：标题 `y=187` = 行顶 + **9**（行自己的 `Space(2.25)`）、
`h=23`；状态行 `y=211` = 标题底 210 + **1**（`rowLineGap`）、`h=15`；带 cwd 时
cwd `y=211 h=15`、meta `y=227 h=15` —— 同高、同列（x=244）、隔 1px。

结论：两行字 = 23 + 1 + 15 = 39，加行自己的 9+9 内边距与盒算术取整 → **56**。参考那句
`min-height: 56px` 的注释写着「a taller floor, because a session is two lines of text plus
a footer of metadata」；本仓的行**正好**落在这个数上，不靠常量，靠内容。

## 3. 台账

- **V 0**：没有视觉数值被重定基线（本轮零生产代码改动）。
- **B 0**：`go test -count=1 ./...` 全绿，既有 pin 一条没动；新 pin 两条。
- **S 0**：结构没变。
- **顺手修红（文档，非产品）**：§6.69 差距表里三处 `故意不改` → `故意不做`（四个合法词是
  未开始 / 已落地 / 需拍板 / 故意不做）；§6.22 第 22-1 行的 `dim = row.Status || row.Disabled`
  内嵌 `||` 在朴素 `Split` 下把状态列挤到下一格（解析器报 313 行 / 312 有状态），改写成文字。

## 4. 门槛（实测）

| 项 | 实测 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 净 / 净 / 净 |
| `go test -count=1 ./...` | **全绿**（修红前 `internal/ci` 1 条红） |
| PASS | **3304**（基线 3302，+2 新 pin；修红那条恢复成过的状态） |
| FAIL | 0（修红前 1） |
| SKIP | 143（不变） |
| 顶层测试函数（`^=== RUN   Test…$` 口径） | **2450**（+2） |
| `GOOS=linux/windows/darwin go build ./...` | 过 / 过 / 过 |

## 5. mutation（2 条，实跑变红 + 逐字节还原）

**M1**：`internal/settingsui/rows.go` 的行内边距 `Padding(t.Space(2.25), inset-1)` →
`Padding(t.Space(1), inset-1)`。

```
--- FAIL: TestTheSessionRowMeetsTheReferencesFloor (0.28s)
    sessionrowbox_test.go:88: the session row is 46px tall (178..223), want the reference's 56u floor
    sessionrowbox_test.go:91: the title starts 4px inside the row, want the row's own 2.25u padding (9)
    sessionrowbox_test.go:102: with a cwd the title starts 4px inside the row, want the same 9: the padding owns the top, the floor owns the rest
```

**M2**：同文件 `rowLineGap` 的 `return 1` → `return 8`。

```
--- FAIL: TestTheSessionRowsStateLineIsOneRegisterWithItsBlock (0.40s)
    sessionrowbox_test.go:130: the state line stands 8px under the cwd line, want the row's own 1px line gap
```

两条都用 `cp` 覆盖回原文件后 `md5sum -c /tmp/p13da.md5` → `internal/settingsui/rows.go: 成功`，
再 `touch` 测试文件复跑 → `ok floter/internal/settingsui 174.194s`（不吃缓存）。

## 6. 用了设计 skill（用户 2026-10-30 要求）

读的路径：`SKILL.md`（Router Rules）→ `references/user-preferences.md`（P-5 可读性优先、
P-2 先看后选）→ 本轮按「已有方向，只要打磨」走 `craft-loop.md` + 收尾的 `preflight.md`。
**没有**走 Phase 2 的四方向预览，没有 HTML 预览页（方向已由用户钉死为轻便液态玻璃，
这条自动车道无人点选；P-2 在这里被任务书显式压过，如实记账）。

### 偷了什么（逐条）

1. 从 `craft-loop.md` §一「参考 DNA 注入」偷的是**参考自己的盒子算术**：不是「参考看起来很
   讲究」，而是「一个行盒 = 内容 + 行自己的内边距」，且**地板不参与盒子算术**——所以
   「56 到底是地板还是内容」是一个能被量出来的问题，而不是读注释。落成 `sessionRowFrame`
   的 `cardTop+1 / cardBottom-2` 读法。
2. 从 §二工艺密度菜单取 **第 11 项「状态/回归细节」**（菜单原例「一个状态的字号要跟它收尾的
   那一行同级」）→ 落成 `TestTheSessionRowsStateLineIsOneRegisterWithItsBlock`
   （cwd 与状态行同高 15、同列 244、隔 1px），与 P13-cy 在集成卡上做的是同一件事。
3. 同菜单取 **第 6 项「编辑细节」**（hairline）→ 落成「行与行之间那道 1px 分隔线不是我量的
   边界」这一条判断：读盒用的是卡片自己的边框与行内边距，分隔线只在多行时出现，不能当尺子。
4. 诚实记账：12 项菜单本轮落 **2 项**，低于 ≥5 的密度要求。理由与 P13-cz 相同——这是一轮
   **只量不改**的收口轮（一个数、一个关系），硬凑 5 项签名动作必然越界到材质与 accent，
   违反「一轮只做一件事」。

### preflight 逐条结论

- **A 反 AI 套路**：本轮零像素改动——无渐变文字 / 无纯黑 / 无外发光 / 无斜体 / 无三等分卡片 /
  无左侧竖线装饰。**结论：过（被动过，改动面为空）**。
- **A2 进攻三步**：DNA 注入 ✔；工艺密度 **2/5**（如实记在上面，不达标并说明原因）✖；
  两轮制「量 → 判 → 只钉不修」✔。
- **B 设计读取一致性**：pin 在 store 默认外观上取景，量的是**盒**（几何与外观无关），
  颜色只在同一帧内比较。**结论：过**。
- **C 动效抽查**：本轮不含动效面。**结论：不适用**。
- **D2 中文页**：pin 只读英文文案的既有键，中文页一个字符没动；会话页状态行仍是
  `tokens.Caption`（11px），低于 preflight「正文 ≥ 14px」——本仓既有口径，已挂
  `.pending-bugs.md`「需用户拍板」第 27 份证据，**不许擅自改**。**结论：字号一项不过，
  原因是需拍板项，已挂账**。

## 7. 仍待

1. **需拍板（新）**：同一张列表里，报了 cwd 的行 72、没报的行 56（差 16px），节拍跟着
   「这条 shell 报没报 OSC 7」跳。参考永远画那行（`session.cwd || "~"`）；本仓的
   `TestTheSessionCwdLineIsTheBlocksOnlyMonoLine` 反过来钉了「没报就不画」。两条路都改视觉
   语义，**先拍板**。
2. **需拍板（沿用）**：`tokens.Caption` 11px 与 preflight D2（第 27 份证据）。
3. **需拍板（沿用）**：扩展页空态只有一行结论没有出路；会话页空态说错对象；终端页会话抽屉 /
   标签对失败只加字不换墨；终端外观抽屉行要不要面；`ui/tooltip.go:133` 禁用元素不弹提示；
   `cardRow` 控制槽 `Shrink(0)`；多选箱宽度；路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
4. **纠正旧账**：`rowFace.Muted` 不是零调用点——`internal/settingsui/custom.go:256`
   （`face.Muted = !complete`）就是它的调用点。
5. **不出图**：本轮画笔零改动，前后两帧逐像素相同（同 P13-cz / P11-cr 的判定）——图会是一张
   「改前 = 改后」的假对比。
