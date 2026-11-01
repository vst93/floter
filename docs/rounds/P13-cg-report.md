# P13-cg 报告 · 一张全是便条的列表没有游标（只读面）

选题：`docs/rounds/P13-cf-report.md`「仍待 2」= 通用队列「状态反馈」的另一半
（**禁用 / 只读面**）。规格 `docs/rounds/P13-cg-spec.md`，合约新增 **§6.50**。

## 这轮为什么让它更轻、更有空间感

前 12 轮把「禁用」这一支的**材质**收干了（P13-aq / s / u / ar / ba / bb / cb / t：
受光边熄、只淡一道、面留在原地）。剩下的不是材质，是**只读面有没有游标**——
一份全是便条的列表（报告式插件）今天**整页没有一行是选中的**：方向键哑、Enter 哑，
屏幕上也没有一句话。这一轮给游标一个**只读档**：游标落在第一行，**面色到了、灯不点**。
于是同一个 z 平面上第一次分出两种「被选中」——门（面 + 受光边 + 700 字重）与便条
（只有面），**层级靠面差与一条灯的取舍表达，不靠新画框**，正是 §2 的读法。

## 改前实测（先量再改）

`kind:"status"` 的行不可运行 → `firstRunnable` 返回 **-1**；`drawOutputRow` 的选中臂
`i == Selected && row.Runnable()` 再把选中面吞掉。两处叠起来：

| 读数 | 改前 |
|---|---|
| `a.output.Selected`（3 行全便条） | **-1** |
| 第一行的面色（浅 / 深） | 253 / 23（= 静息行，**0 级差**） |
| 方向键 ↓ | 原地不动（跳行规则只停在可运行行） |
| Enter | 什么也不做 |
| 屏幕上有没有一句话说为什么 | 没有 |

## 改后实测（同一帧）

| 读数 | 改前 | 改后 |
|---|---|---|
| `Selected`（3 行全便条） | -1 | **0** |
| 第一行面色 · 浅 | 253（=静息） | **238**（+15） |
| 第一行面色 · 深 | 23（=静息） | **43**（+20） |
| 行顶四行是否比面色更亮（浅） | — | **否**（灯没点） |
| 门被选中时的受光边 | 亮 | **亮**（旧 pin 仍绿） |
| `outputText()`（复制取的行） | `""` | **`""`**（守卫保住旧行为） |
| 前后帧差分 | — | **28081** px，包围盒 `8,118..711,157` |

差分包围盒正好是第一行的 40u 行盒，**其余像素零变化**（720×488 全帧逐像素比）。
「这张图上看到的一处变化」：三行便条里只有第一行换了面，第二三行与页头、条子、
面板一个字节没动——游标是**一行**的事，不是一页的事。

## 实现（只改墨与面）

- `output.go` `firstRunnable` → **`initialOutputSelection`**：无可运行行时返回 `0`
  （空列表仍 -1）。注释写清「游标 ≠ 门」。
- `drawOutputRow`：`selected`（游标在这儿）与 `chosen`（被选中的门）分开；
  新增 `case selected:` → `Background(RowSelected)`，**不调 `LitFace`**。
- 标题 700 / 副标题提墨 / 行尾徽章三处从 `selected` 改读 `chosen`——一句便条不该因为
  被游标指到就变成标题。
- `outputText()` 加 `if !row.Runnable() { return "" }`：**保住**本轮之前的行为
  （那时 `Selected` 到不了便条），不是新语义。

## 台账（V / B / S）

- **V 0**：没有一条 pin 钉的是旧视觉值（旧的选中行 pin 全都仍然绿——门那一支一个像素没动）。
- **B 0**：`moveOutputSelection` 的跳行规则、`runOutputRow` 的 `!Runnable()` 提前返回、
  复制、编号族、折边、高度链**零改动**。
- **S 0**。
- 新 pin：`internal/launcher/notecursor_test.go` 三条 + 一条 opt-in 出图。

## 门槛（实测）

```
gofmt -l cmd internal                 → 空
go vet ./...                          → 净
GOOS=windows go vet ./...             → 净
GOOS=darwin  go vet ./...             → 净
go test -count=1 ./...                → 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS" → 3248   （基线 3241，+7）
SKIP 126 → 128（新增两条 opt-in dump 的 skip）
测试函数 2397 → 2404（+7，无减少）
GOOS=linux|windows|darwin go build ./... → 三条全过
```

## 变异（3 条，实跑变红并逐字节还原）

- **M1 · 游标退回旧形状**（`initialOutputSelection` 兜底 `return 0` → `return -1`）：
  ```
  notecursor_test.go:69: a list of 3 notes put its cursor at -1, want 0 (no row on the page is chosen)
  notecursor_test.go:69: ... (dark 同)
  notecursor_test.go:132: the down arrow moved the cursor to -1 on a list where nothing can run
  ```
- **M2 · 便条游标点了灯**（便条那一支补 `tokens.LitFace(...)`）：
  ```
  notecursor_test.go:108: the note cursor's top edge row 0 is {242 242 243 255}, brighter than
  its face {238 238 239 255}: a note is not a door (liquid-glass.md §3)
  ```
  （深色臂同时红；门那一臂仍绿——它本来就该亮。）
- **M3 · 复制跟着游标走**（删掉 `outputText()` 的 `!row.Runnable()` 守卫）：
  ```
  notecursor_test.go:127: an all-notes list copies "Note 0", want nothing
  ```

每条：注入 → 抄下报错串 → `cp` 还原 → `md5sum -c` 逐字节核对（成功）→ `touch` 复跑绿。

## 偷了什么（qiaomu-design · craft-loop.md + preflight.md）

**从 `craft-loop.md` 的工艺密度菜单取的项，落成的具体数值：**

1. **第 12 项「材质细节」（内边框高光 / glass 折射边）**——落成「**灯是权限，面是状态**」：
   便条游标 `RowSelected`（浅 253→238，深 23→43）但 `LensRim` 不画（行顶四行无一行更亮）。
   同一个 token、同一条面差，差的是那一条 1px 的受光边。
2. **第 5 项「多层染色阴影」的反面用法**——落成「**把第二层拿掉**」：`LitFace` 是
   顶 `LensRim` + 底 `LensEdge` 的一对；只读游标把这一对整体留空，而不是换成另一对。
   一个应用里「被选中」只有一条材质规则，两种权限用**有/无**区分，不用两套。
3. **第 6 项「编辑细节：hairline / 大写字距标签」**——落成**字重只在门上**：
   700 与 `t.Text` 只给 `selected && row.Runnable()`，便条被选中时仍是它自己的
   `TextMuted` + 原字重（一句便条不该因为被游标指到就变成标题）。
4. **`craft-loop.md` 的「先读现物，再动手」**——落成「**先量再改**」那一节：本轮先量到
   `Selected = -1`、面色对静息行 **0 级差**，才动手；不是先想一个好看的做法。
5. **`user-preferences.md` P-5（可读性优先于气质）**——落成「游标必须看得见」的硬门槛：
   `channelGap(face, rest) >= 8`（实测 15 / 20），低于 8 就是「没人看得见的游标」，
   直接判红，不靠审美拍板。

**preflight 抽查（A / B / C / D2）**：

- **A（反 AI 套路）**：过。零装饰性产物——没有渐变、没有 glow、没有新阴影；唯一新增的
  视觉是**一个已有的 token 用在了一个新状态上**。文案零新增（本轮不加一个字）。
- **B（设计读取一致性）**：过。只读 `docs/liquid-glass.md` §3（受光边是「可碰」的记号）
  与 §2（层级靠面差）；两条臂用的是**同一个** `RowSelected` 与**同一个** `LitFace`，
  没有为便条发明第二套材质。
- **C（动效抽查）**：过（本轮无动效改动）。游标的出现是一次重绘，不是 transition；
  行仍然只有 `rowTransition` 的颜色过渡，没有新增 tween。
- **D2（中文页正文 ≥ 14px）**：**不适用且仍是未决项**——本轮没有新增任何中文文案，
  也没动字号；`tokens.Caption` 11px 与 D2 的冲突沿用第 11 份证据，仍在
  `.pending-bugs.md` 的「需用户拍板」里，不静默跳过。

## 仍待（写进 `.pending-bugs.md`）

1. **下一格（通用队列）**：`docs/liquid-glass.md` §6 已无 `未开始` 行 → 回「细节打磨」
   通用队列。候选：① 参数编辑器（`parameditor.go:89`）的空态是**唯一**没走
   `emptyState` 的一处（caption 一行字），其余三处空态都是 emphasis + `TextStrong`；
   ② 便条行的**图标板**仍叠着 `DimFace` + `Opacity(0.62)` 两道（`output.go:873`），
   本轮**没量**它够不够 8 级——先量再决定。
2. **需拍板（沿用 + 第 11 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：启动器 `breathUnits`（4px）对捕获输出页 `Space(4)`（16px）；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；按下档要不要保留那一对线（`pointerFaceAllow` 两条）；
   终端外观抽屉行要不要面；终端弧 S1b / S1c / S2 尾。
4. **抢救 stash 核对（本轮第九次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，**不要再试合**。
