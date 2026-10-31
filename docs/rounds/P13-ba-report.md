# P13-ba 报告 · 行忙时那簇键的灯

**方向一句话**：扩展页的行忙状态此前是「一片熄了灯的控件中间还亮着几条冠线」——`actionButton`
把禁用那一格写死成 `false`，六处「建完再 `.Disabled(true)`」全部绕过了 P13-s 建好的禁用面。

- 规格 `docs/rounds/P13-ba-spec.md`；合约新增 `docs/liquid-glass.md` **§6.21**（7 行全部 `已落地`）。

## 为什么这轮让它更轻、更有空间感

z 阶梯的价值是**读起来一致**：同一页上「不可碰」必须只有一种画法。此前同一页有五种——
`DimFace`（27 枚）、框架平涂（四枚，P13-ai 之前）、accent 熄冠（`updateButton`）、
`ControlDim` 槽（`config.go`）、以及**行忙时那一簇仍然带灯的胶囊**。最后这一种恰好在最显眼
的位置：一行里 3-4 枚键被同一把锁按住，用户看到的是「都亮着，但只有中间那枚在转」。

## 落地（6 处调用点 + 1 条普查）

根因在 `internal/settingsui/rows.go`：`actionButton` → `extensionActionButton` →
`extensionActionFace(c, h, r, **false**)`。新增 `disabledActionButton`（同一枚胶囊、
`extensionActionFace(..., true)`、`.Disabled(true).Cursor(ui.CursorDefault)`），六处改成在
**建面之前**决定，而不是建完再 `.Disabled(true)`：

| 调用点 | 旧 | 新 |
|---|---|---|
| `transferRow` 导出 / 导入 | `.Disabled(busy)` | `busy` → `disabledActionButton`（只建一枚） |
| `rowAction`（行忙整簇） | `button = button.Disabled(true)` | `busy` → 直接返回 `disabledActionButton` |
| 行内 Run | `Tip(...).Disabled(true)` + `.Disabled(true)` | `switch`：不可运行 → `Tip(disabled)`；行忙 → `disabled`；否则 `actionButton` |
| `repairAction` | 建完 `.Disabled(true)` | 按 `rowBusyExceptRepair` 选面；**一枚** `tokens.Tip` 收口（保住 `tipglass_test.go` 的 10 处普查） |
| `detectedAction` | `.Disabled(disabled)` | `disabled` → `disabledActionButton` |

## 读数（`dimProbe`，320×160 harness，同一地面）

| 外观 | 地面 | 面 | 冠线 改前 → 改后 | 底对侧 | 可用胶囊对照 |
|---|---|---|---|---|---|
| 浅 | 247 | 238 | **240（+2）→ 238（0）** | 231（−7） | 236 / 230（+6） |
| 深 | 14 | 26 | **36（+10）→ 26（0）** | 23（−3） | 58 / 39（+19） |

**改前那枚冠线是「比可用胶囊还弱、但不是零」的一条**——最尴尬的一档：不像可用，也不像不可用。

出图（`FLOTER_P13BA_DUMP=1`，`before` 由 M1 注入取得）：
`/tmp/floter-P13-ba-locked-pill-{light,dark}-{before,after}.png`（每张从上到下：行忙胶囊 /
可用胶囊 / in-flight 胶囊）。**眼睛复核**：after 深色那张里第一枚与第二枚的差别只剩「面的
亮度 + 字的重」，第一枚顶上那条白线没有了；before 深色那张第一枚顶上有一条比第二枚还淡的白线。

## 门槛（全部实测）

- `gofmt -l cmd internal` → 空
- `go vet ./...` → 净；`GOOS=windows go vet ./...` → 净；`GOOS=darwin go vet ./...` → 净
- `go test -count=1 ./...` → 全绿；PASS **3117 → 3119**（+2）| FAIL 0 | SKIP **102 → 103**（+1，opt-in 出图）
- 测试函数：HEAD 实测 **2295** → **2298**（+3）。**更正**：P13-az 报的「2297」与 HEAD 实测
  2295 差 2，是那份报告的口径误差，本轮以实测记。
- `GOOS=linux/windows/darwin go build ./...` → 三个都过
- **V 0 / B 0 / S 0**（没有 pin 钉旧数值；`tipglass_test.go` 的 10 处普查在改成「一枚 Tip
  收口」之后原样通过）

## mutation（3 条实跑，全部变红并逐字节还原）

1. `disabledActionButton` 体退回 `a.actionButton(c, label).Disabled(true)`：
   ```
   disabledpill_test.go:40: the row-locked action pill (light): the disabled pill kept a lit edge:
       top inner row {240 240 241 255} is 2 levels over the face {238 238 239 255}
   disabledpill_test.go:40: the row-locked action pill (dark): ... {36 36 38 255} is 10 levels over {26 26 28 255}
   disabledpill_test.go:104: rows.go:514 disables an action pill after building it (...)
   ```
   （同一次注入也用来出 `before` 图。）`md5sum -c` 逐字节还原。
2. `rowAction` 退回「建完再 `.Disabled(true)`」的**多行**写法：
   ```
   disabledpill_test.go:104: settingsui.go:2773 disables an action pill after building it (button = button.Disabled(true))
   ```
   这条证明普查的 3 行窗口能抓到多行写法，不是只抓同行。还原后 `md5sum -c` 通过。
3. 把 `DimFace` 那一行换成 `_ = tokens`（熄灯但**连底对侧也没了**）：
   ```
   disabledpill_test.go:40: ... (light): the disabled pill lost its counter-side: bottom inner row {238 238 239 255} is only 0 levels under the face {238 238 239 255}
   disabledpill_test.go:40: ... (dark): ... {26 26 28 255} is only 0 levels under ...
   ```
   还原后 `md5sum -c` 通过。**另记一条没做成 pin 的**：把 `.Cursor(ui.CursorDefault)` 去掉，
   引脚**不红**——框架对禁用子树本来就拒绝指针，而这套产品里可用的胶囊也读 `CursorDefault`
   （`ButtonBase` 不写 Pointer），所以「光标」在这条链上**没有两侧可夹**。按纪律不造脆弱断言，
   那条探针整段撤掉，理由写在这里。

## 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

- **第 5 项「多层染色阴影」**：不在本轮（这一族是 E1、无影，§2 判据 2）——显式不做，理由在合约里。
- **第 12 项「材质细节」**：取「内边框高光 / 玻璃折射边」。落成具体数值：禁用胶囊**只留底对侧**
  （浅 `LensEdge` 0.06 在 0.5 淡下实测 −7 级、深 −3 级），**顶受光边 alpha = 0**（改前浅 +2 / 深 +10）。
  这不是「调淡」，是把「受光」这一位整个删掉——`DimFace` 与 `LitFace` 是两种材质，不是两档强度。
- **第 7 项「编辑细节」**：取「同一语义必须同一种画法」。落成：六处调用点全部收敛到**一个函数**
  `disabledActionButton`，并加一条普查 pin 让第七处不能再手写出来。
- 未取项说明：第 2/3/4/9/10 项是页面级签名动作（选中色、focus 环、磁吸、滚动条、彩蛋），
  这一族是**自绘桌面客户端的单个状态**，套上去就是「密度 ≠ 吵闹」的反例。

## preflight 逐条（打磨模式，A / B / C / D2）

- **A 反 AI 套路**：无渐变描边冒充高光（本轮反而**删掉**一条受光边）；无纯黑/外发光/渐变文字；
  无三等分卡片；无「革命性/无缝/赋能」；感叹号 0。
- **B 设计读取一致性**：读取 =「桌面启动器的设置页，面向高级用户，用轻便液态玻璃的克制语言，
  倾向 E1/E2 两档材质」。本轮产出与读取一致——**只减不加**，没有引入任何新的面档。
  记忆点一句话：同一页上「不可碰」只有一种画法。
- **C 动效抽查**：本轮不动效。禁用面**本来就没有**颜色过渡以外的动效（`Transition{Colors, 120ms}`
  仍在）；行忙那一枚的 ring 仍然转（`actionSpinner`，0.9s/圈），reduced-motion 下静止——未动。
- **D2 中文页**：本轮没有新增/改动任何用户可见文案（`copy.*` 一个字段没动），行高与字号沿用
  `tokens.Body`。**未过项（沿用，需拍板）**：`tokens.Body` 10.8px / `tokens.Caption` 11px 低于
  D2 的「正文 ≥ 14px」；这条是全仓字阶的账，`.pending-bugs.md` 里已挂了多轮，本轮不擅自改。
- **D 工程抽查**：图标按钮有 label（`ButtonBase(c).Label(...)` 仍是原样）；禁用元素的指针与点击
  拒绝由框架 `flagDisabled` 负责，本轮只换材质；无新增滚动区/弹窗。

## 仍待（P13-ba）

1. **需拍板（沿用，P13-az 新记）**：`ui/tooltip.go:133` 的 `s.tip && s.flags&flagDisabled == 0`
   ——禁用元素不弹提示。本轮**又给这条加了 5 个受影响的点**（行忙时 3 枚 + Run 不可用的理由 +
   检测到工具行的空态），`copy.RunActionUnavailable` 依旧永远不出现。要改就得动框架悬停语义。
2. **需拍板（沿用）**：`tokens.Body` 10.8px / `tokens.Caption` 11px 与 preflight D2 的张力；
   `cardRow` 控制槽 `Shrink(0)`；多选箱宽度；终端弧 S1b / S1c / S2 尾。
3. **下一格**：§6 表仍无 `未开始` 行 → 通用队列。候选：① `settingsui` 里还剩**一处**同类
   「后置禁用」的形状——`extensionIconButton` 的 `disabled` 臂是**先画静息面再熄灯**（不走
   `DimFace`，因为它没有受光边），复核它是否需要；② 空态满态里「空态 + 搜索无结果」两种
   文案共用一个 `emptyState` 的间距账；③ `shell` 侧的禁用面普查（本轮的普查只扫
   `internal/settingsui`，`internal/shell` / `internal/launcher` 尚未建同一条）。
