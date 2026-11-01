# P13-ck 报告 · 分页脚注：拉取结束的那一帧，词横移 7.5px

方向一句话：**这一行的两种状态不是两段文字，是同一枚环的两种姿态——环不该消失，该停下。**

## 1. 先量：三条候选，量倒两条

选题来自 P13-cj「仍待」第 1 条。开工先量，量的是页面画出来的像素（`ui.Tester`，
装 app 主题，与 `shell.View` 同一条生产路径，窗口 720×488，深浅各一遍）。

| 候选 | 量到的数 | 判定 |
|---|---|---|
| ① 分页行被高度链挤压 | 行带 468.2→488 = **19.8px**，换窗口 440 / 488 / 520 三次恒为 19.8 | **否掉**：正好是 `MinHeight(Space(5.5))` = 5.5 × 3.6 = 19.8，不是挤压 |
| ② 这一页有两个地板（到底了 118 / 图例 109，浅 9 级；深 118/89，29 级） | 浅色看着像档差塌了 | **否掉**：两处不在同一条带上，图例是 chrome（§6.26 有意坐最低档），到底了是内容 |
| ③ 在飞比结局还响（139 / 137，差 2） | 两者都是 `TextSecondary` | **否掉**：各是各自那条带的上限（页脚 y=470 / 条子 y=71），不需要互相分辨 |

量到第四件事才立住：**两档画的内容不同，而这一行是居中布局**。

| | 词盒 x | 内容左沿 | 环 |
|---|---|---|---|
| 在飞 | **331.0** | 313.0 | 313–325，峰值 **139**（活动象限） |
| 到底了 | **323.5** | 323.5 | 无 |

词走 **7.5px**，左沿走 10.5px，环整枚消失——这一页唯一一条「换状态时整行都在动」的带。

## 2. 改了什么

`internal/launcher/output.go`：

- `:341` `if state == footerLoading { a.footerSpinner(c) }` →
  `if state == footerLoading || state == footerEnd { a.footerSpinner(c, state == footerLoading) }`
- `:414` `footerSpinner` 收一个 `live bool`；`:422` 新增 `!live` 分支：**只画 hairline
  一整圈**（闭合），不画活动象限、不 `Transform`、不 `AnimationFrameRate`，然后 `return`。
- `live` 分支逐字节不动（见 §4 的逐字节证据）。

墨、字号、行高、`Gap`、`Padding`、环的尺寸与描边、`footerMore` 的空行、`footerWord`
的三臂、`outputFooterState` 的状态机：**一个字节没动**。高度链 / 键盘模型 / 会话管理 /
设置写回 / i18n / 命令接线：零改动。

## 3. 判据（写进合约 §6.54）

1. 状态切换不许让这一行的内容搬家：词漂 **7.5 → 1.5px**。
2. 残差 1.5px 是布局的算术：两个词宽 76 / 73，一半 = 1.5；环的盒子买回另外 6px。
   pin 的天花板因此是 **2px**，不是 0。
3. 到底了那一圈是闭合轨道：峰值 **22**（浅 / 深同值），比它自己的词（118）低 **96 级**；
   在飞仍是 **139**（与词同级，P13-bj 不动）。

## 4. 门槛（实测）

```
gofmt -l cmd internal                # 空
go vet ./...                         # 净
GOOS=windows go vet ./...            # 净
GOOS=darwin  go vet ./...            # 净
go test -count=1 ./...               # 全绿
PASS 3257 → 3260 | FAIL 0 | SKIP 129 → 130 | 测试函数 2410 → 2414
GOOS=linux/windows/darwin go build ./...   # 三平台过
```

基线按 HEAD `1f266f5b` **实测重记**（`git grep -c '^func Test' HEAD` = 2410）：上轮报告
记的 3258 / 2411 各高 1，本轮以实测为准。新增 3 条 pin + 1 条 opt-in dump。

## 5. 变异（3 条实跑，`md5sum -c` 还原 + `touch` 复跑绿）

| # | 注入 | 原始报错串 |
|---|---|---|
| M1 | 到底了不建环（退回只 `footerLoading`） | `the word moves 7.5px between the live and the ended footer (x 331.0 → 323.5)` + `the settled ring peaks at 0 over the plane, want ≥ 12` |
| M2 | 静止分支仍画活动象限 | `the settled ring peaks at 139 and its word at 118 (gap -21, want ≥ 12)` |
| M3 | 在飞也画静止环 | `the live ring peaks at 22, its word at 139 — the quarter lost the live ink` + P13-bj 的 `the ring peaks at 22, its word at 139 — the pair is not one ink` |

## 6. 出图（自己复核过）

`FLOTER_P13CK_DUMP=1` → `/tmp/floter-P13-ck-footer-{loading,end}-{light,dark}-{before,after}.png`，
`more` 档只有 before（该档不画环也不画字，本轮不碰）。

- **`loading` 两套外观前后 md5 逐字节相同**（`807fff89…` / `8bcfd571…`）——活的那一支没动。
- `end` 前后差分：**519 个像素**，包围盒 `(314,472)-(405,484)`——全部落在页脚行自己的内容带里，
  这一页别的像素一个没动。
- 图上看到的一处变化：到底了那一帧，词右移 9px 回到「环在时」的位置，词左边多出一枚
  **很淡的闭合细圈**（22 级，比词低 96 级），不再是「一行字孤零零地换了个位置」。

## 7. 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

- **第 4 项「签名交互：一个足矣」** → 落成「环的静止态」：到底了不删环，只**停转**
  （1.5px 描边 + hairline 22 级 + 零帧申请）。状态切换因此有动作，元素不搬家。
- **第 12 项「材质细节：内边框高光 / glass 折射边」** → 落成「轨道两档都在」：这一行的
  「受光边」不随状态消失，22 级恒在，于是行的左沿有一个恒定的锚。
- **第 5 项（多层染色阴影）本轮不用**：页脚是列表内的元素，§3 明写「面板内元素不用阴影」。

## 8. preflight 逐条

- **A（反 AI 套路）**：无渐变/发光/纯黑/斜体；无三等分卡片；文案零新增（本轮一个字的
  文案都没加）；无左侧竖线装饰（P-38 ✓）。**过。**
- **B（设计读取一致性）**：一页一个明暗主题 ✓；记忆点 = 「一圈合上 = 到底了」；
  动效气质（停转而非消失）与 §5「高频不动画、状态有位移」一致。**过。**
- **C（动效抽查）**：新增的「静止」是**取消**动画而非新增（`AnimationFrameRate` 只在
  `live` 时调，到底了不申请帧 → 这一行不再重绘）✓；无 `transition: all` 等价物；
  无 scale/fade 入场；`ReduceMotion` 路径不动（静止分支根本不读它）。**过。**
- **D2（中文页）**：本轮不新增中文文案，`正在加载…` / `没有更多结果` 原样；
  行高（15px 字盒 / 19.8px 行带）、字重（500 / 400）不变。**过**（`tokens.Caption`
  11px 与「正文 ≥ 14px」的冲突是**沿用**的需拍板项，见「仍待」，不是本轮引入）。

## 9. 仍待（P13-ck）

1. **下一格（状态反馈）**：`docs/liquid-glass.md` §6 无 `未开始` 行，回通用队列。
   候选：设置页 `Integration.state` 把 `Enabled` 读成「运行中」（`copy.go:1228`
   `IntegrationsRunning` = 运行中）——一个**开关状态**被说成一个**进行中的动作**，
   与页脚「正在加载…」是同一个词的两种含义。**先量，再决定改词还是改态**（i18n 文案
   属于红线，改文案要先写进「需用户拍板」）。
2. **需拍板（沿用 + 第 15 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：启动器 `breathUnits`（4px）对捕获输出页 `Space(4)`（16px）；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；按下档那一对线；终端外观抽屉行要不要面；终端弧 S1b / S1c / S2 尾。
4. **抢救 stash 核对（本轮第十三次）**：顶部三条（`33e03d1f6a` / `afc38b247c` /
   `114b7d1585`）已判定与 HEAD 无未接续内容，**不要再试合**。
