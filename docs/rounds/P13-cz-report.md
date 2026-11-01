# P13-cz 报告 · 会话页的满态：列表与页脚的关系（先量，不改）

**结论先行**：三个数都落在 §4.3 的阶梯上（8px / 22px / 31px），**产品代码零改动**，
本轮交付的是**账**：两条 pin + 合约 §6.69。渲染像素一个都没变（没有改动画笔），
所以**不出前后对比图**——这正是本轮的正确答案（同 P11-cr 的判定）。

## 1. 为什么是它

- `docs/liquid-glass.md` §6 已无 `未开始` 行（这一支的表面队列是空的），选题回通用队列。
- 通用队列里「空态 / 满态」这一族：会话页的**空态**已经收过两轮（P13-aj 的三件东西、
  P13-ap 的出口按种类取间距），而**满态**那一半是空格子 —— P13-aq 的「仍待」第 4 条②
  原文点名的就是它：「空态 / 满态的另一半（会话多条时列表与页脚的关系）」。
- 它不需要拍板：只量、只钉，不改交互、不改文案、不改高度链。

## 2. 量法（可复现）

`internal/settingsui/sessionsfoot_test.go`：`render(a, 720, 760)` → 点侧栏 `Sessions` →
取一帧；在 `x=238` 这一列上（卡片内、避开前导 chip `≤230`、避开标签列 `≥242`）用
`blockAbove` 从「地面色」往上扫出第一块非地面色块，于是**卡片盒 / 组注盒 / 页脚胶囊盒**
三块各自独立读出。浅色档、默认 interface step 实测：

| 盒子 | 实测 | 来源 |
|---|---|---|
| 卡片 | y **177..318** | 面段 178..316 + 冠线 177 + 对侧两条 317/318 |
| 组注（`SessionsActive`） | y **327..349** | 字盒 |
| 页脚胶囊（`SessionsNew`） | y **371..401** = **31px** | 30u 的 `MinHeight` + 一对 1px 边 |

三段盒对盒的间距：

| 关系 | 实测 | 对应刻度 | 判 |
|---|---|---|---|
| 卡底 318 → 注顶 327 | **8px** | `sectionGroupPadded` 的 `Space(2)`（组内 gutter） | 同一块 ✔ |
| 注底 349 → 胶囊顶 371 | **22px** | `pageColumn` 的 `Space(5.5)`（页 gap） | 一个可以按的新东西 ✔ |

§4.3 那条阶梯（8px 同一块 < 12px 同一段的下一行 < 22px 一个新东西）与空态那一支
（P13-ap 的 0.5u / 3u）说的是同一句话：**间距按「这个东西是什么」取，不按「它在第几行」取**。
所以本轮**不改值**。

第二条 pin 回答「页脚是不是钉在视口底」：换成一枚会话之后，两个间隙**逐值相等**
（8 / 22），并且 3 行的卡片严格高于 1 行 —— 页脚跟着内容走，不是 fixed 底栏，也不读行数。

## 3. 台账

- **V 0**：没有视觉数值被重定基线（本轮不改值）。
- **B 0**：`go test -count=1 ./...` 全绿，既有 pin 一条没动。
- **S 0**：结构没变。
- 新增 2 条 pin（`sessionsfoot_test.go`），两条都在同一个 `render` 夹具上跑。

## 4. 门槛（实测）

| 项 | 实测 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 净 / 净 / 净 |
| `go test -count=1 ./...` | 全绿 |
| PASS | **3302**（基线 3300，+2） |
| FAIL | 0 |
| SKIP | 143（不变；新 pin 不是 opt-in 截图 dump） |
| 顶层测试函数（`^=== RUN   Test…$` 口径） | 2448（+2） |
| `GOOS=linux/windows/darwin go build ./...` | 过 / 过 / 过 |

## 5. mutation（2 条，实跑变红 + 逐字节还原）

**M1**：`internal/settingsui/rows.go` 的 `pageColumn`：`Space(5.5)` → `Space(4.5)`。
原始报错串：

```
--- FAIL: TestTheSessionsPageFooterRidesItsContent (0.73s)
    sessionsfoot_test.go:108: the page button stands 18px under the hint, want the page column's 22u gap (hint box ends at 349, pill at 367)
```

**M2**：同文件 `sectionGroupPadded`：`a.cardTheme(c).Space(2)` → `Space(3)`。

```
--- FAIL: TestTheSessionsPageFooterRidesItsContent (0.67s)
    sessionsfoot_test.go:104: the hint stands 12px under the card, want the group's 8u gutter (card box 181..322, hint at 335)
```

两条都用 `cp` 覆盖回原文件后 `md5sum -c /tmp/p13cz.md5` → `internal/settingsui/rows.go: 成功`，
再 `touch` 测试文件复跑 → `ok floter/internal/settingsui 1.486s`（不吃缓存）。

## 6. 用了设计 skill（用户 2026-10-30 要求）

读的路径：`SKILL.md`（Router Rules）→ `references/user-preferences.md`（P-1 禁斜体、P-5
可读性优先、P-2 先看后选）→ 本轮表面按「已有方向，只要打磨」走 `craft-loop.md`。
**没有**走 Phase 2 的四方向预览，没有出 HTML 预览页（方向已由用户钉死为轻便液态玻璃，
且这条车道无人点选）；P-2 在这里被任务书显式压过，本报告如实记账。

### 偷了什么（逐条）

1. 从 `craft-loop.md` §一「参考 DNA 注入」偷的是**参考自己的盒子算术**：不是「参考看起来
   很讲究」，而是「一块面 = 面段 + 冠线 + 对侧两条」。落成 `blockAbove` 的读法——
   pin 读的是**盒**（318），不是面段（316），所以 8px 才等于 `Space(2)` 的承诺。
2. 从 §二工艺密度菜单取 **第 6 项「编辑细节」**（菜单原例 hairline / 大写字距标签）→
   本仓对应物是**卡片内部那两条 1px 发丝**（行分隔 208 与对侧 `LensEdge`）：它们正是
   `blockAbove` 把「一行」与「整张卡」分开的依据，落成 `cardFloor` 的 +2。
3. 同菜单取 **第 12 项「材质细节」**的边界：页脚胶囊的盒高落成 **31 = 30u + 一对 1px 边**
   ——E1 的一对线在**盒子算术里**，不在盒子外面（§3「顶亮一分、底暗一分」）。
4. 诚实记账：菜单 12 项本轮只落了 **2 项**，低于 ≥5 的密度要求。理由：这是一轮
   **只量不改**的收口轮（一个关系、三个数），新增 5 项签名动作必然越界到材质、动效与
   accent，违反「一轮只做一件事」与「视觉轮只改墨与面」。

### preflight 逐条结论

- **A 反 AI 套路**：本轮零像素改动 —— 无渐变文字 / 无纯黑 / 无外发光 / 无斜体 / 无三等分
  卡片 / 无左侧竖线装饰，全部照旧成立。**结论：过（且是被动过，改动面为空）**。
- **A2 进攻三步**：DNA 注入 ✔（第 1 条）；工艺密度 **2/5**（如实记在上面，不达标并说明
  原因）✖；两轮制 ——「量（第一轮）→ 判 → 只钉不修」成立 ✔。
- **B 设计读取一致性**：一页一个明暗主题（pin 在 store 默认外观上取景；量的是**盒**，
  几何与外观无关，颜色的比较只在同一帧内做）。**结论：过**。
- **C 动效抽查**：本轮不含动效面（没有 `transition`、没有入场、没有 reduced-motion 相关
  改动）。**结论：不适用（改动面不含动效）**。
- **D2 中文页**：pin 只读英文文案的既有键，中文页一个字符没动；但会话页的组注/行文案仍是
  `tokens.Caption`（11px），**低于 preflight 的「正文 ≥ 14px」**——本仓既有口径，已挂
  `.pending-bugs.md`「需用户拍板」第 26 份证据，**不许擅自改**。**结论：字号一项不过，
  原因是需拍板项，已挂账**。

## 7. 仍待

1. **下一格（先量）**：会话行的步距实测 **46~47px**，而参考的 `.session-manager__row` 是
   `min-height: 56px`（`settings.css:1240-1248`）。本仓 `rowValue` 没有 56u 的地板，行高由
   两行字加内边距自然撑出。要不要把地板补上属**布局/高度链**改动，先对参考 CSS 逐字段核
   一遍再定，**本轮不动**。
2. **需拍板（沿用）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」（第 26 份证据）。
3. **需拍板（沿用）**：终端页的会话抽屉 / 标签对失败只加字不换墨（`HeaderExited(code)`）；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
4. **记一笔（不改）**：`rowFace.Muted`（`.settings-row--muted` 的 0.62）当前零调用点。
5. **抢救 stash 核对（第二十八次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，本轮未再试合、也未静默丢弃。
