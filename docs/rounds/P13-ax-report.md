# P13-ax 报告：提示框戴上浮面（E2）—— 全仓最后一枚白片子

规格见 `docs/rounds/P13-ax-spec.md`，合约新增 **§6.20**（终端页 `已落地`，启动器 / 设置页 `未开始`）。

## 这轮为什么让它更轻、更有空间感
提示框此前是**唯一一个不在 z 阶梯上的面**：不透明的反色平涂 + 一层 8px 糊影 + 零条边，
深色档上它甚至是一枚近白的片子（本仓从不设 `ui.Theme.Inverse`，框架退回 `Text`）。
现在它与抽屉、对话框、便条同一套说法——`Float` 面 + 一条受光边 + 两层影——
它读成「浮在内容上的一张纸」，而不是一块贴在玻璃上的贴纸。

## 落地
1. **`internal/theme/theme.go` 新增 `Tokens.Tip`**（`ui.TooltipBase` + E2 材质）：`Float` 面、
   `RadiusSM` 8、1px `InputStroke` 发丝、顶边 `EdgeHighlight` 冠线、两层影
   `0 6px 18px 0.30` + `0 1px 2px 0.20`。影**不走** `ElevThrow`：`--elev-shadow-scale` 跟的是
   `main_opacity`（窗框自己的玻璃），提示不在这张框里。
2. **终端页两处接上**：`barButton` 的 `button.Tooltip(hint)`、会话标题的
   `Tooltip(identityTitle)`（P13-ae 定的「读被截断的会话全名」的唯一出口）。
3. **行为一个字节没动**：0.6s 停留、0.4s 热接力、Esc、锚点与 1.5u 间距、80u 上限、
   `Description` 的无障碍描述，全部沿用框架。

## 读数（实测，深/浅两套真 `Tokens`）
| 项 | 深色 | 浅色 |
|---|---|---|
| 地面（pin 的不透明地） | 20/21/24 | 247/247/249 |
| 提示面 = `FaceOver(ground, Float)` | 23/23/26（旧值：`Text` 近白 ≈229） | 250/250/252 |
| 面像素数（≥500 才算戴上了 float 面） | 通过 | 通过 |
| 冠线 | 门槛 ≥ +6 级，通过（M2 熄掉后顶行 19/20/23，比面低 4 级） | **不断言**：0.92 近白压 250 的近白面实测 −13 级（白压白），浅色靠发丝那一圈 |
| 圆角 | 5（`KeycapRadius` 档）→ **8**（`RadiusSM`） | 同 |
| 0.4s 时画面 | 一个 float 像素都没有 | 同 |

出图：`/tmp/floter-P13-ax-tip-dark.png`、`/tmp/floter-P13-ax-tip-light.png`（`FLOTER_P13AX_DUMP=1`）。
图上能看到的一处变化（深色）：提示从一块近白的片子变成一块**深面**，它的顶行比面亮、
而且它下面有一圈把地面压暗的影（改前只有一个 8px 的糊影、顶行与面同值）。

## V / B / S 台账
- **V 0**：这块面此前没有任何视觉 pin。
- **B 0**：`terminalui` 既有测试（`barchrome_test` / `view_test` / `drawermetrics_test`）零改动全绿；
  提示的出现条件、锚点、无障碍描述都是框架行为，没有触碰。
- **S 0**：没有删断言、没有加容差、没有 `t.Skip`（出图靠 env，不是 skip）。

## 门槛（实测）
```
gofmt -l cmd internal                      # 空
go vet ./... / GOOS=windows,darwin go vet  # 净
go test -count=1 ./...                     # 全绿
PASS 3100 → 3103 (+3) | FAIL 0 | SKIP 102（不变）| 测试函数 2288 → 2289 (+1)
GOOS=linux/windows/darwin go build ./...   # 过
```

## 变异（2 条实跑，2 红，逐字节还原）
1. `Background(t.Float)` → `Background(t.Theme.Text)`（退回框架的反色片子）：
   `tipglass_test.go:159: the tip's face is 0 pixels of FaceOver(ground, Float): the tip is not wearing the float face`
2. 删掉 `tip.InsetShadow(0, 1, 0, 0, t.EdgeHighlight)`：
   `tipglass_test.go:184: the tip's top edge is {19 20 23} against the face {23 23 26} (-4 levels): no lit edge`
   - 两条都 `cp` 还原 + `md5sum -c /tmp/p13ax.md5` 通过，`touch` 后复跑绿。

## 偷了什么（craft-loop 工艺密度菜单，逐条落成数值）
- **第 5 项「多层染色阴影」** → 两层，`0 6px 18px rgba(0,0,0,0.30)`（环境）+ `0 1px 2px rgba(0,0,0,0.20)`（接触），
  取代框架那唯一一层 `0 2px 8px 0.2`。
- **第 12 项「材质细节：内边框高光」** → 顶边 1px `EdgeHighlight`（深 0.20 / 浅 0.92）当冠线，
  外加 `InputStroke` 一圈发丝；圆角从 5 抬到梯子上的 8。
- **第 9 项「自定义滚动条」/ 第 1 项「氛围层」/ 第 2 项 `::selection`** → 本仓已由前几轮落地或按 §1 禁令不做
  （玻璃面不许自造透明、不许叠玻璃），这轮不动，写在这里是为了不假装偷了 5 项。

## preflight
- **A 反 AI 套路**：没有新文案（i18n 零改动）、没有 emoji、没有渐变白边、没有把手/圆点装饰。
  **过**。
- **B 设计读取一致性**：读取是「克制的功能型轻玻璃」，本轮只动一枚浮层的面与影，密度 6 未变；
  记忆点 = 提示的冠线是它唯一亮起来的一条边。**过**。
- **C 动效抽查**：这一轮**不动效**——提示的时钟是框架的 0.6s/0.4s，没有新增 transition，
  `prefers-reduced-motion` 不适用（无动画可关）。**不适用，已写明理由**。
- **D2 中文页**：无新增文案；`tokens.Body` 10.8px 与「正文 ≥ 14px」的张力**仍然不过**，
  已在 `.pending-bugs.md` 记为需拍板（全局字阶），本轮不擅自改。

## 仍待（下一轮候选）
1. **§6.20 的 20-2 / 20-3**：启动器 7 处、设置页 10 处还是框架的平涂提示（各一轮，机械迁移 `tokens.Tip`）。
   两处都在自动化车道里，**不需要拍板**。
2. **需拍板（沿用）**：`tokens.Body` 10.8px / `tokens.Caption` 11px 与 D2 的张力；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；终端弧 S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾（客户端不告而别）。
