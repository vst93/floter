# P13-ea 报告 · 会话行的 56 地板（73-6 / 73-6c 结案）

**方向一句话**：设置页会话行穿上参考自己那条 **56px `min-height`**（`.session-manager__row`），
多出的 5.6px 全落在状态行**下方**——一行两行字终于不贴着盒底了。

## 1. 这轮为什么让它更轻/更有空间感

参考那条地板的注释是「a taller floor, because a session is two lines of text plus a footer of
metadata」：地板买的不是内容，是内容**下面**那口气。改前这一行的盒 = 内容（50.4），
状态行下沿离盒底 8.1；改后 56，同一处是 **13.8**（帧读数 8 → 12）。标题起点一个像素没动
（`Space(2.25)` 仍是 8.1），所以空间是加在「块之后」，不是把块整体压低。

## 2. 先量：P13-dz 的「处方无效」被证伪

同一张会话卡、同一取景（`720×760`、en、浅色），只改 `cardRow` 的地板：

| 注入 | 帧读数（`cardTop+1..cardBottom-2`） |
|---|---|
| `MinHeight(44)`（改前） | 49 |
| `MinHeight(50.4)`（= `Space(14)`，73-6 的处方） | **49**（不动） |
| `MinHeight(52)` | 51 |
| `MinHeight(56)` | **55** |
| `Height(44)` / `Height(50.4)` | 43 / 49 |
| `Height(60)` / `Height(70)` | 59 / 69 |

- 行盒**由行元素拥有**（`Height(60) → 59`、`Height(70) → 69`）。
- `Space(14) = 50.4` 恰好等于这一行的**内容高度** → 拿它打地板等于没打。
- 帧按整像素扫描，整体比布局盒少 1（`44 → 43`、`60 → 59`、`70 → 69`），50.2/50.4 都截到 49
  ——§6.95 看到的是这个截断。

## 3. 落地项（位置 → 现状 → 改成）

| 位置 | 现状 | 改成 |
|---|---|---|
| `internal/settingsui/rows.go:452` | 只有 `rowFloorPx = 44` | 新增 `const sessionRowFloorPx = 56`（字面 px，不是 `--u × 14`） |
| `internal/settingsui/rows.go:1476` `rowFace` | 没有行级地板 | 新增 `Floor float32` |
| `internal/settingsui/rows.go:1508` `cardRow` | 地板只来自 `max(Space(11), 44)` 与卡片首行的 `cardTitleFloor` | `if face.Floor > floor { floor = face.Floor }`（排在 `cardTitleFloor` 之后） |
| `internal/settingsui/settingsui.go:1686` | 会话行 `rowFace{}` | `rowFace{Floor: sessionRowFloorPx}` |

只动会话行：设置页其它行仍是 44，集成卡首行 66u 不变。

## 4. 台账（V / B / S）

- **V 2**：`sessionrowbox_test.go:106` 的盒读数 **49 → 55**（新版还把 `49` 换成
  `sessionRowFloorPx-1`，让常量与帧读数只有一个数字）；`sessionrowbox_test.go:139` 新增
  `TestTheSessionRowsFloorIsTheReferencesLiteral`（字面 56 / 不被 `Space(14)` 吃掉 / 那口气
  落在状态行下方）。
- **B 0**：高度链、键盘模型、会话管理、设置写回、i18n、命令接线零改动。
- **S 0**。
- 产品代码 4 处（3 处在 `rows.go`，1 处在 `settingsui.go`）＋1 个 opt-in 出图测试。
- 测试函数 **2493 → 2495**，PASS **3326 → 3327**，`--- FAIL` **0**，SKIP **159 → 160**
  （多出来的那一条就是本轮新增的 opt-in 出图 `TestDumpP13EASessionRows`，其余 159 条不变）。

## 5. 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                            → 净
GOOS=windows go vet ./...                               → 净
GOOS=darwin  go vet ./...                               → 净
go test -count=1 ./...                                  → 全绿，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"       → 3327（基线 3326）
go test -count=1 -v ./... | grep -c -- "--- SKIP"       → 160（基线 159；多的是本轮新增的 opt-in 出图）
GOOS=linux/windows/darwin go build ./...                → 三个都过
测试函数计数（`grep -rn "^func Test" internal cmd`）    → 2495（基线 2493）
```

## 6. 变异（3 条实跑变红并还原；`md5sum -c` 逐字节相同 + `touch` 复跑全绿）

- **M1** 会话行 `rowFace{Floor: sessionRowFloorPx}` → `rowFace{Floor: 50.4}`（把 73-6 的旧处方
  放回去）：
  `the session row is 49px tall (160..208), want 55 on the frame's own reading`
  ＋ `the state line ends 6px above the row's bottom, want 12`。
- **M2** 删掉 `cardRow` 的 `if face.Floor > floor { floor = face.Floor }`：同一对红串
  （行级地板失效 = 回到内容 50.4）。
- **M3** `sessionRowFloorPx` 56 → 50（改常量本身）：
  `the session row's floor is 50, want the reference's literal 56px` ＋
  `the session row's floor is 50, not above the u-metric Space(14) (50.4): a floor there draws
  nothing` ＋ 空格那条。
- 还原后 `md5sum -c /tmp/p13ea.md5` 三条全「成功」，`touch` 后复跑目标测试 `ok`。

（规格里写的 M3 是「把地板挂到卡片首行」，落地时发现那个形状在这张卡上等价、不会变红——
按「变异必须真红」的纪律换成了上面这条，记在这里。）

## 7. 出图

`FLOTER_P13EA_DUMP=1 go test -run TestDumpP13EASessionRows ./internal/settingsui/`
（`FLOTER_P13EA_TAG` 区分 before/after）：

- `/tmp/floter-P13-ea-sessions-light-{before,after}.png`
- `/tmp/floter-P13-ea-sessions-dark-{before,after}.png`

**这张图上看到的一处变化**（逐像素 diff 复核，dark 列 250）：卡片底边从 `350/351` 移到
`356/357`；**文字列逐字节相同**（标题与状态行没动），差的是**行右侧那对 28u 图标键**
（控件槽 `AlignItems(Center)`，随行盒下移约 3px）。也就是说这 6px 全部长在字的**下面**。

## 8. 偷了什么（qiaomu-design，本轮真的读了）

开工前读：`SKILL.md` → `references/user-preferences.md`（偏好账本）→ 按本轮表面（设置页的
间距节奏）取 `references/craft-loop.md` + `references/preflight.md`，另取
`design-systems/raycast/DESIGN.md` 当 DNA 供体（本仓视觉单一来源就是 Raycast 那一代）。

- **DNA 注入（`craft-loop.md` 一）**：从 `design-systems/raycast/DESIGN.md:227`
  偷「Pill buttons: 86px radius with 20px padding — **well above 44px minimum**」——
  即「触控目标要**明显高于** 44 这条下限」。落成：会话行地板 **56**（不是 44，也不是
  `Space(14)=50.4`），且这条地板**不随字阶缩放**（和 `rowFloorPx` 同一条规则：字面 px）。
- **工艺密度菜单（`craft-loop.md` 二）**：
  - **第 6 项「编辑细节」** → 落成 `sessionRowFloorPx = 56` 与「那口气在状态行下方」
    （帧读数 8 → 12、布局 8.1 → 13.8）。
  - **第 12 项「材质细节」** → 本轮**不适用**：这一格是盒高，面/受光边/投影一个字节没动
    （§6.13 起这一行就是 E1 面，不该动）。不硬凑。
  - 其余 10 项（氛围层、::selection、focus 环、签名交互、多层染色阴影、展示排版、流体字号、
    自定义滚动条、彩蛋、图形资产）在**一个自绘桌面应用的设置页行盒**上都没有落点，
    按 preflight「空态/状态齐全」那一类归到别的表面，本轮不碰。
- **两轮制**：第一轮 = 注入地板 + 量；评审（工程师视角）挑出「帧读数比布局盒少 1，pin 里写
  55 会被下一个人当成 55px 行」→ 第二轮把 55 的来源写进注释与 §6.96 的表，并让 pin 用
  `sessionRowFloorPx-1` 而不是硬写 55。

## 9. preflight 逐条结论

**A. 反套路（AI Tells）**

- 无紫蓝渐变 / 纯黑 / 霓虹外发光 / 渐变文字 —— 过（这轮只改一个盒高）。
- 字体不是 Inter/Roboto/系统默认 —— **故意不适用**：本仓是可执行的桌面应用，字体由
  `tokens` + 打包的参考字体决定，不是 web 字体选择（同 §6.x 历轮结论）。
- 全页无斜体 —— 过（`noitalic_test.go` 是本仓的常驻 pin）。
- Hero 居中构图 / 三等分卡片 / John Doe / Acme / 「革命性」式文案 —— 过（无营销页）。
- **A 的「无左侧竖线装饰」** —— 本仓的 `cardRow` 前导规则条是参考 `.extension-row::before`
  的规格，是**已定的设计语言**（§5 禁止项只禁「hover 时抬亮」），保留。

**B. 设计读取一致性**

- 读取：**设置页会话行，面向会用多个终端会话的开发者，用轻便液态玻璃（Raycast 系）语言**。
  VARIANCE 4 / MOTION 2 / DENSITY 6 —— 一轮只调一个盒高，三个拨盘都不动，产出与声明一致。
- 一页一主题 —— 过（light/dark 各出一张图，不混）。
- 记忆点一句话：**两行字下面那口气**（56 地板）。
- 动效与气质一致 —— 过（无新增动效；行 hover/press 仍是原来那套面色）。

**C. 动效抽查** —— 本轮**没有新增动效**：没有 `transition: all`、没有 `scale(0)`、
没碰 `width/height/padding` 的动画（盒高是静态值）。`prefers-reduced-motion` 兜底与本轮无关，
沿用既有 motion 包。**这条不是「跳过」，是「没有对象」。**

**D2. 中文页追加项（未过，需拍板）**

- 中文行高 ≥ 1.5 / 中西文空格 —— 过（i18n 的字串带空格，`chinese-typography` 那套在
  §6.x 已经落过）。
- **正文 ≥ 14px —— 仍未过**：`tokens.Caption` 9.9 / 正文 10.8（第 50 份证据，本轮无新增）。
  这是一个**全局字阶**决定，不是一行能改的，已挂在 `.pending-bugs.md` 的「需拍板」里。
- 暗色正文对比度 ≥ 7:1 —— 沿用 §6.x 的墨表结论（这一轮没动墨）。

**E. 三秒测试** —— 眯眼看会话卡：三行可读信息（标题 / 状态 / 时间）、两个图标键、一条
「New session」。争夺注意力的元素 ≤ 1（行标题）。**不像 AI 做的**：没有装饰性光斑/玻璃边/
网格线，盒子只由字阶 + 面差 + 发丝线分层。

## 10. 仍待（P13-ea）

1. **`§6.86` 的取景债现在只剩取景族**（`de-2/3/5/6/9/11`、`73-5`），都卡在「计费与绘制
   不是同一套字号」上；下一格建议 `P13-dg-t`（`internal/terminalui` 的夹具取景收口），
   只许 V 或 S。
2. §6 差距表**再次没有 `未开始` 行**（73-6 / 73-6c 本轮销掉），选题回通用队列：
   间距节奏 / 状态反馈的禁用·只读面 / 空态满态 / 文案密度 / 边界情况。
3. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2「≥ 14px」。
4. 抢救 stash 核对（第五十四次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
