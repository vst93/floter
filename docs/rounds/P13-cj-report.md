# P13-cj 报告：选中行的副标题与标题同墨，而同一张卡的另一张列表只提一档

选题：P13-ci「仍待」第 1 条（通用队列 `状态反馈`）。规格
`docs/rounds/P13-cj-spec.md`，合约新增 **§6.53**。

## 一句话

**选中行把副标题提到和标题同墨，于是两行只剩字重一条腿；同一张卡里的启动器列表
在同一状态下只提一档。** 改的是这一档墨——`t.Text` → `tokens.TextSecondary`。

## 先量再改（实测）

量法：每个字带读**离行面最远的像素**（§6.51 判据 2：深色外观下最暗的是面，不是字），
x 取标题列 `54..500`，y 以行盒中线分成两条带。

| 面 | 标题 墨距 | 副标题 墨距 | 差 |
|---|---|---|---|
| 捕获输出 · 选中（改前） | 214 | **214** | **0** |
| 捕获输出 · 选中（改后） | 214 | **131** | 83 |
| 启动器 · 选中 | 211 | **131** | 80 |
| 捕获输出 · 静息 | 229 | 140 | 89 |
| 启动器 · 静息 | 226 | 140 | 86 |

深色：改前 **201 / 201（差 0）**，改后 **201 / 111（差 90）**，启动器选中
**194 / 111（差 83）**；静息两处 **138 / 138**。

**改后两张列表的选中行给副标题的是同一个档位**（131 / 111，逐级相同）——这不是
「调成相近」，是同一个 token 在同一个面上落同一个值。

## 两轮制：第一轮被自己的量尺否掉

第一轮照启动器**两个旋钮一起给**（`TextSecondary` + `FontWeight(600)`）。墨是对的
（131 / 111），但行高变了：**选中行 53px，静息行 54px**。这一页的行高是**内容撑的**
（没有 `MinHeight` 地板，启动器结果列表有 40u 地板），加一档字重就换一个字体面，
把这一行压短 1px——游标一移，整张列表跟着抖 1px。

第二轮**只改这一处**：去掉字重，只提墨。行高回到 **54 / 54**，墨距不变。
这条取舍写进 §6.53 判据 2，并用 pin 的第 ④ 臂钉住（加字重那版会红）。

## 改动

`internal/launcher/output.go`（`outputRow` 的副标题分支）：

```go
if row.Subtitle != "" {
    subtitle := ui.Text(c, row.Subtitle).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
    if chosen {
        subtitle.TextColor(tokens.TextSecondary)   // 旧值 t.Text（与标题同墨）
    }
}
```

**一个 token、一个字节**。字号、行高、padding、gap、字重、面、灯、游标、点击、
`outputText()`、`runOutputRow()` 全部不动。

## 台账

**V 0 / B 0 / S 0**：`grep` 过 `outputaccent_test.go` / `outputstyle_test.go` /
`outputframe_test.go`，没有 pin 断言过选中行副标题的墨；行为 pin 全部零改动。

## 门槛（实测）

```
gofmt -l cmd internal          → 空
go vet ./...                   → 净
GOOS=windows go vet ./...      → 净
GOOS=darwin  go vet ./...      → 净
go test -count=1 ./...         → 全绿
PASS 3254 → 3258 | FAIL 0 | SKIP 129 | 测试函数 2409 → 2411
三平台 build（linux / windows / darwin）→ 过
```

## 变异（3 条实跑，逐条变红并还原）

- **M1** 改回 `t.Text`：
  `the chosen row's subtitle is 0 levels from its title's ink (want ≥ 60)` +
  `the chosen row's subtitle ink is 83 levels from the selected result row's (214 vs 131)` → **双红**。
- **M2** 补 `FontWeight(600)`：
  `the chosen row is 53px tall and the plain row 54px` → **红**（深浅两套都红）。
- **M3** 静息副标题也提到 `TextSecondary`：
  `the plain rows' subtitles read 146 vs 140 levels from their own faces` → **红**。
- 三条均 `cp` 还原 + `md5sum -c` 逐字节核对（`internal/launcher/output.go: 成功`）+
  `touch` 后复跑绿。

## 出图（自己复核）

`/tmp/floter-P13-cj-outrow-{before,after}-{light,dark}.png`、
`/tmp/floter-P13-cj-listrow-{before,after}-{light,dark}.png`（opt-in：
`FLOTER_P13CJ_DUMP=1`，可选 `FLOTER_P13CJ_TAG` 区分前后）。

**图上看到的一处变化**：浅色 `outrow` 那张的副标题带（`Sub A`）改前是 `#`
（R<120 的实心档，与上面标题同一档），改后整行降到 `+`（R<190）——笔画位置、
基线、行盒一个像素没动，只有墨退了一档；启动器 `listrow` 那张前后**逐字节相同**
（md5 一致），证明改的只是捕获输出页这一处。

## 用 skill 的证据（qiaomu-design · craft-loop + preflight）

**读了**：`SKILL.md`（路由表 → 「已有方向，只要打磨」车道）→
`references/user-preferences.md`（P-1 禁斜体 / P-5 可读性优先 / P-7 功能契约）→
`craft-loop.md` → `preflight.md`。**没有出四方向预览页**：用户已把方向给死为
「轻便液态玻璃」，自动车道无人可点选（P-2 的例外在本任务书里写明）。

**偷了什么**（逐条落到数值）：

1. `craft-loop.md` 工艺密度菜单 **第 6 项「编辑细节」**（字阶 rank）→ 落成
   **选中行副标题的墨距 131 / 111，与标题 214 / 201 差 83 / 90 级**，并且
   **跨列表同级**（启动器 131 / 111）。
2. 工艺密度 **第 12 项「材质细节」** → 判据「副标题是这块面上的第二行，不是第二块面」，
   用它**否掉**「给副标题加字重」：加字重换字体面，把选中行 54px 压到 53px，
   是在动面的几何而不是墨（第二轮只改这一处）。
3. `craft-loop.md` §三 **两轮制**：第一轮 131 + 600 → 量到 53/54 → 第二轮只改
   「去字重」→ 复核 54/54 且墨距不变。**没有推倒重做。**

**偏差（写明原因）**：工艺密度只落 2 项，不到菜单的 5 项——这一轮改的是**一行字的墨**，
不是一张新面；按 §四 的取舍顺序（两轮制 > DNA > 密度），预算放在量测与两轮复核上，
再凑就是 §四 说的「密度不等于吵闹」。§一 的 DNA 注入同样不适用：没有引入新的数值体系，
只把一个既有 token 换到另一个既有 token。

**preflight 逐条**：

- **第 0 步（偏好账本）过**：P-1（无斜体）——本轮零字形变化；P-5（可读性）——
  副标题改后仍是 131/111 的墨距，比静息行的 140/138 只低 9 级，不是「读不清」；
  P-7（功能契约）——选中行仍是「Enter 会跑的那一行」，加粗 700 与面、灯全在。
- **A（反 AI 套路）过**：无渐变、无发光、无斜体、无假数据、无新装饰；本轮只换一个颜色 token。
- **B（设计读取一致性）过**：与 §4「层级靠面差 + 字重，不靠画框」同向；§6.53 记录数值。
- **C（动效抽查）不适用**：本轮零动效，前后帧逐像素可复现（`listrow` 前后 md5 相同）。
- **D2（中文页）部分不适用**：副标题是插件给的字符串，排版栈（字号 / 行高 / token）未改。
  **仍需拍板**：`tokens.Caption` 11px 与 D2「正文 ≥ 14px」（沿用第 14 份证据，不擅自改）。

## 仍待（P13-cj）

1. **下一格（通用队列 · 状态反馈）**：捕获输出页**标题**用的是 `t.Text`，启动器结果
   列表标题是 `tokens.TextStrong`——两者在浅色下都是 27（同值），深色下都是 237，
   所以**这一处没有差**（量过，不作废）；真正还差一处的是**行尾的动作字**
   （`rowActionText` 的 `620` 对启动器同处的 `620`，已同级）。下一格改从
   「空态满态」里挑：捕获输出页的**分页/加载中**那一档（`output.go` 的 footer 三档）
   是否与列表的两张表在「进行中」上同一档。
2. **需拍板（沿用 + 第 14 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：启动器 `breathUnits`（4px）对捕获输出页 `Space(4)`（16px）；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；按下档那一对线；终端外观抽屉行要不要面；终端弧 S1b / S1c / S2 尾。
4. **抢救 stash 核对（本轮第十二次）**：顶部三条（`33e03d1f6a` / `afc38b247c` /
   `114b7d1585`）已判定与 HEAD 无未接续内容，**不要再试合**。
