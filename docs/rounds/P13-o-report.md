# P13-o 报告 · 凹槽的一对线：`LensInset` 的底边不是 `LensRim`

**轮号** P13-o ｜ **规格** `docs/rounds/P13-o-spec.md` ｜ **合约** `docs/liquid-glass.md` §1 的 0-9 行 / §3

## 这轮为什么让它更有空间感

深色档的凹槽一直读得出「底下有光」（底内线 +33），浅色档**从来没有**：槽面 232 上，
它自己的发丝线（`TrackEdge` 黑 0.08）先把那一行压到 213，再叠 0.09 的白只剩 217——
**比槽面暗 15 级**。于是浅色档的开关和分段槽是「两条暗线夹一块面」的画框，不是洞。
换成凹槽自己的受光边（`TrackRim` 浅色 0.70）之后，同一行变成 242（**+10**）：洞里第一次
有光，凹槽与它所在的那张纸之间第一次有了方向（上面暗、下面亮），而不是一圈等价的描边。

## 做了什么

1. `internal/theme/theme.go`：新增 `palette.trackRim` 与 `Tokens.TrackRim`（随 `LensScale`
   分级，「玻璃关掉」时归零）；浅色 **0.09 → 0.70**、深色 **0.09 不动**；`LensInset` 的
   底边线 `t.LensRim` → `t.TrackRim`（四层不变，只换一层）。
2. `internal/theme/theme_test.go`：`off` 档的归零清单加 `TrackRim`；玻璃档递增清单加
   `TrackRim`（它和 `LensRim` 走同一条 `--glass-lens-scale`）。
3. 新增 `internal/settingsui/groovepair_test.go`：2 条像素 pin（3 个真凹槽 × 2 套外观）
   + 1 条 opt-in dump。量的是**真组件**（`control.SegmentedIndex` / `a.switchControl`），
   不是自己画的参照框。
4. `docs/liquid-glass.md`：§1 上限表加 `TrackRim` 一行 + 「新上限一句话」补一句；§3 加
   `TrackRim` 一行 + 「凹槽的底边是另一条线」一段；§6.0 新增 **0-9**；§6.2 的 2-5 改判。

## 数字（实测，真设置页地面，真组件）

| 外观 | 凹槽 | 槽面 | 顶内线 | 底内线（旧 0.09） | 底内线（新） |
|---|---|---|---|---|---|
| light | 分段槽 | 232 | 184（−48） | 221（**−11**） | **242（+10）** |
| light | 关的开关 | 192 | 153（−39） | 184（−8） | **231（+39）** |
| light | 开的开关 | `6,74,166` | `5,59,132` | `28,85,161` | `181,199,224` |
| dark | 分段槽 | 36 | 29（−7） | 69（+33） | 69（+33）**不动** |
| dark | 关的开关 | 68 | 46（−22） | 96（+28） | 96（+28）**不动** |
| dark | 开的开关 | `143,183,255` | `84,104,142` | `157,189,250` | 同左**不动** |

浅色档的底内线 242 **仍在地面 247 之下**——凹槽接住光，但不闪；这条是第二个 pin。

## 被否掉 / 没做的

- **深色档不动**：它的底边是两条白线相加（环 0.075 白 + 受光边 0.09 白），实测 +33，
  本来就亮。深色缺的那一半是「面·对侧」，P13-n 已经补过。
- **没有把浅色 `LensRim` 一起抬**：P13-n 已实测，抬到 0.55 会把 accent CTA 的顶行洗成
  55% 的白（`TestThePrimaryCTAIsAnAccentButtonAtRadiusSM` 找不到强调色顶行）。这一轮
  只给凹槽一条自己的线，面那一条仍然是 0.09。
- **没动 `LensInset` 的层数**：仍是「环 / 顶暗 / 接触影 / 底亮」四层，只换底亮那层的 token。

## 出图（自己复核）

`/tmp/floter-P13-o-groove-{light,dark}.png`：两条 999px 的凹槽（中性 `Track` 与 accent）
× before（借 `LensRim` 0.09）/ after（`TrackRim`），画在真页面的地面上。

**这张图上看到的一处变化**（浅色档，中性槽，实测像素）：底边那一行 **217 → 242**，槽面是 232。
before 那一行比槽面**暗 15 级**——凹槽的下边界是一条暗线，和顶边（184）一起把槽围成画框；
after 那一行比槽面**亮 10 级**，并且仍在地面 247 之下：下边界变成「接到纸上的光」。
深色档上下两张逐像素相同（这轮不动深色），这也是核对「只改了浅色」的方式。

## 用了设计 skill（`qiaomu-design`，2026-10-28 装）

读了 `SKILL.md` → `references/user-preferences.md`（偏好账本，最高优先级，逐条对照）→ 本轮表面用得上的
三份：`craft-loop.md`（工艺密度菜单）、`apple-fluid-interfaces.md` §8（透明材质与深度）、
`preflight.md`（A / B / C / D2）。**没走 Phase 2 的四方向预览**：方向由用户定死（轻便液态玻璃），
自动车道没有人能点选。

**设计读取**：桌面命令面板应用的**开关与分段控件**，面向每天用键盘启动命令的高级用户，用
「低光泽 + 一条受光边」的轻便玻璃语言，倾向 Apple HIG 的材质/深度家族（`apple-fluid-interfaces.md` §8）
叠加本仓 `docs/raycast-design.md` 的行为契约。拨盘：`VARIANCE 3`（形状已被参考钉死，只剩材质）、
`MOTION 3`（这一轮不动效）、`DENSITY 6`（设置页的信息密度中高，凹槽是密度里最安静的那一层）。

### 偷了什么（逐条：从哪份参考的哪一项 → 落成哪个具体数值）

| 来源 | 偷的项 | 落成的值 |
|---|---|---|
| `craft-loop.md` 工艺密度菜单 **第 12 项**（材质细节：内边框高光 / glass 折射边） | 「一条 1px 的内边框高光」这一项 | `TrackRim` 浅色 **0.70** / 深色 **0.09**，落在 `LensInset` 的底边；实测底内线 **217 → 242**（槽面 232） |
| `craft-loop.md` 工艺密度菜单 **第 6 项**（编辑细节：hairline） | 「发丝线要服从层级，不是自己当主角」 | `TrackEdge` 环保留（浅 0.08 黑 / 深 0.075 白），但明确它**不许赢过**受光边：浅色 0.08 黑 vs 0.70 白，净 +10 |
| `craft-loop.md` 工艺密度菜单 **第 5 项**（多层染色阴影） | 反用：菜单要「2-3 层、拒绝单层」，本仓凹槽已经是四层 —— 层数不是目的，**方向**才是 | 只换一层 token（不加第五层、不加模糊）：四层结构一个字节没动 |
| `apple-fluid-interfaces.md` §8 第 3 条「不要把浅色 glass 叠在浅色 glass 上」 | 凹槽的受光边不许亮过它被切进去的那张纸 | 浅色底内线 **242 < 地面 247**（`TestTheGroovesLitEdgeStaysUnderTheSheet` 守住） |
| `apple-fluid-interfaces.md` §8 第 1 条「半透明只给浮动功能层」 | 组件层不许自造透明 | 本轮只改 token 的一行 alpha，窗口层的 `main_opacity` / `glass_step` 链**一行未动** |
| `floter-main/src/styles/base.css:899`（参考实现自己的值 `--glass-lens-rim-base: 0.9`） | 浅色凹槽的受光边本来是近白，是本仓 P8 压到 0.04 时一起压掉的 | 取 **0.70**（比参考轻 0.2：本仓浅色的槽面比参考更亮，0.9 会把那一行顶到 255） |

**工艺密度只记 3 项，不是 5 项**——理由写在这里：这一轮只改**一条 1px 线**，凑到 5 项就得往凹槽里加
第二层光、加模糊、加渐变，而这三样正是 `docs/liquid-glass.md` §1 禁令 1/3 点名的「冒充玻璃」。
菜单的密度要求是针对「整页产出」的，这一轮的产出是「一个材质规则」；凑数会直接违反本仓合约。

### `preflight.md` 逐条结论

- **A（反 AI 套路）**：过。无紫蓝渐变（本轮只动中性白/黑）、无 `#000000` 纯黑（深色地面实测
  `14,14,16`）、无外发光、无大标题渐变字、无左侧竖线（P-38）、无斜体（自绘字体不设 italic）。
  **不适用**：Hero 构图 / 三等分卡片 / Bento / 眉批 / 字体选择 / 假数据文案 —— 这是自绘桌面应用里
  的一个控件面，没有 Hero、没有 webfont、没有卡片海、没有生成文案。
- **B（设计读取一致性）**：过。记忆点一句话：**「凹槽的下边界从一条暗线变成一条接住光的线」**；
  每张图只有一个明暗主题（浅色图不含深色块）；本轮零动效，不存在气质冲突。
- **C（动效抽查）**：不适用（本轮没有任何动效改动：无时长、无曲线、无 transform）。唯一相关的一条：
  两条 pin 与两张 dump 都显式 `ReduceMotion: true`，所以量到的都是静息帧，不是过渡中间态。
- **D2（中文页）**：不适用（本轮**零新增中文字符串**，i18n 文案与设置写回一行未动；报告与合约里的
  中文是 markdown 文档，不受 UI 字号/行高规则约束）。数字与单位之间的空格已遵守（`0.70`、`217 → 242`）。

## 变异（4 条实跑，全部变红并逐字节还原）

| # | 注入 | 原始报错串（截断） |
|---|---|---|
| M1 | 浅色 `trackRim` → 0.09（= 借 `LensRim`） | `[light] the segmented track's groove has no lit bottom ({232 232 233} against its own fill {232 232 233})` |
| M2 | 浅色 `trackRim` → 1.0 | `the segmented track's lit edge ({255 255 255}) is brighter than the sheet it is cut into ({247 247 248})` |
| M3 | `LensInset` 的底边线 → `t.LensEdge`（只改这一个函数体，`assert body.count('t.TrackRim')==1`） | `[light] the switch at rest's groove has no lit bottom ({192 192 193} against its own fill {192 192 193})`（深色同样红） |
| M4 | `LensInset` 的顶暗线换成受光边（凹槽长出冠线） | `[light] the switch's groove has a lit crown ({65 112 175} over its own track {6 74 166})`（既有 pin `TestTheRecessedGrooveHasNoCrown` 抓到） |

每条都 `cp` 备份 → `sed`/python 注入 → `touch` 后复跑（避开测试缓存）→ `cp` 还原 → `md5sum -c`
逐字节核对（三次都是 `internal/theme/theme.go: 成功`）。

**抓不到的那一条（如实记）**：把 `LensInset` 里的 `TrackEdge` 与 `TrackRim` 换序（环画在最后）
**没有变红**——两层都是半透明，合成结果只差 ±2 级（实测 124 vs 122）。所以「后加的画在上面」这个
顺序在本仓只是注释层面的约定，像素 pin 不会守它；`theme.go` 的注释按 `ui.InsetShadow` 的语义写
（后加者在上），但**不要指望测试抓这个**。
