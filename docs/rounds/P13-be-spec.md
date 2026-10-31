# P13-be 规格 · 插件窗口的「一句话」：正文陈述在 chrome 之下

## 意图

捕获输出页（`shell/detach.go` 的 detached 窗口）**正文里唯一的那句话**，比它头顶的
chrome 还轻——同一个窗口里没有第二个层级。这一轮把状态行的**寄存器**分成三档，
让「这一页说了什么」第一次比「这一页叫什么」重。

为什么现在做：P13-bd 收尾时把这一格列为下一候选（「它是状态行不是空态，该不该有
陈述档要先量」）。本轮先量，再改。

## 证据（先量，实测 720×460 静息，`shell/detach.go` 的 `detachedBodyView`）

窗口自己的地面（`tokens.Opaque`）：浅 **246** / 深 **20**。

| 位置 | 现状墨 | 浅 | 深 | 对地面 |
|---|---|---|---|---|
| bar 标题（`detached.title`，Caption + `t.TextMuted`） | 静息 | **131** | **130** | 115 / 110 |
| 正文 · running | `Caption` + `t.TextMuted` | **131** | **130** | 115 / 110 |
| 正文 · empty | 同上 | **130** | **131** | 116 / 111 |
| 正文 · idle | 同上 | **128** | **133** | 118 / 113 |
| 正文 · failed | `Caption` + `t.Warning` | 69 | 164 | 177 / 144 |
| 列表里的 status 行 | `Caption` + `t.TextMuted` | **131** | **130** | 115 / 110 |

**两件事**：
1. 正文的四条状态行与 bar 标题**逐级相同**（浅 128–131 对 131，深 130–133 对 130）。
   窗口读起来是「一条 bar + 一条同色的灰线」，没有第二档——这正是用户说的「平」。
2. 四条状态行里 `idle` / `empty` 是**结论**（这页为什么空），`running` 是**进行**，
   `failed` 是**故障**；三种语义同一个字号、同一种墨（只有 failed 换了色相）。

候选墨（同一地面、同一 Caption 步实测）：`TextStrong` 30 / 234（216 / 214）、
`TextSecondary` 109 / 152（137 / 132）、`TextMuted` 128 / 133（118 / 113）。

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `shell/detach.go` `detachedStatus`（一个 `failed bool` 管四臂） | `Caption` + `t.TextMuted`（failed 换 `t.Warning`） | 拆成两个角色：`detachedStatement`（正文三档，见 2）与 `detachedListNote`（列表 status 行，**不动**） |
| 2 | `detachedStatement` 三档 | — | 陈述（idle / empty）：`tokens.Emphasis` + 580 + `tokens.TextStrong`（§4.7 的陈述档，与 `settingsui.emptyState` 同一条）；进行（running）：`tokens.Emphasis` + 500 + `tokens.TextSecondary`（一档之下）；故障（failed）：`tokens.Emphasis` + 580 + `t.Warning`（色相不动，只抬到陈述的字阶） |
| 3 | `detachedListNote`（`detachedBodyView` 的 list 臂、`detachedList` 的 status 行） | 与陈述同墨 | **不动**：它说的是**列表**（从属），不是这一页；列表的盘子（`tokens.Control`）才是那一档的主体 |
| 4 | bar 标题 | `Caption` + `t.TextMuted` | **不动**：chrome 本来就该是最轻的一件（参考 `plugin-window__title` 的 caption/muted） |

**重定基线**：参考的 `.plugin-window__status` 是 `--text-caption` + `--text-muted`
（`launcher.css:1783-1789`）。本仓按 §4.7 把**这一页的结论**抬到陈述档，
参考值只保留在「列表 status 行」这一从属角色上。旧值 → 新值：正文状态行
`Caption`/`TextMuted`（115 / 110）→ `Emphasis`/`TextStrong`（216 / 214）。

## 行为契约零改动红线

高度链（窗口本体是 `Fill()` 的 scroll，字阶变化不改窗口高度）、键盘模型、会话管理、
设置写回、i18n 文案、命令接线**全部零改动**。本轮只改**墨与字阶**，不改几何常量、
不改文案、不增删控件。

## 门槛与 PASS 基线（本轮实测）

- `gofmt -l cmd internal` 空 / `go vet ./...` 净 / Windows+darwin vet 净 / 三平台 build 过
- `go test -count=1 ./...` 全绿；PASS 基线 **3142**、测试函数 **2309**、SKIP **105**（P13-bd 实测）
- 出图：`/tmp/floter-P13-be-pluginwindow-{light,dark}-{running,failed,empty,idle,liststatus}-{before,after}.png`

## mutation 计划

1. `detachedStatement` 的陈述臂退回 `t.TextMuted` → 「正文陈述比 bar 标题重」那条红。
2. `running` 臂抬到陈述档 → 「进行比陈述轻一档」那条红。
3. `failed` 臂换 `tokens.TextStrong` → 「故障保留语义色相」那条红（量 `r−b`）。
4. 列表 status 行跟着抬到陈述档 → 「列表的注是这一页的从属」那条红。
