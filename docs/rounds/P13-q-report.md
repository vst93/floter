# P13-q 报告 —— 设置页动作面的第三档：按下

## 1 意图

设置页里「点一下就执行」的面，静息 → 悬停 → **按下** 三档必须都在；这一轮补的是
**按下**那一档：面沉到 `ControlPress`，几何按族的规矩收一档（胶囊 0.97 / 小格 0.92）。

## 2 为什么现在做（证据）

`docs/liquid-glass.md` §6 没有 `未开始` 行，按纪律转通用队列的**状态反馈**格。
`grep -n "Pressed()" internal/settingsui/*.go` 只命中 6 处，`grep -n "Hovered()"`
命中 19 处 —— 这一族只做了一半：

| 面 | 悬停 | 按下（本轮前） |
|---|---|---|
| `barButton`（`settingsui.go:5795`，`.extensions-action-button`） | 有 | 只有 `Scale(0.97)`，**没有面** |
| `extensionActionFace`（`rows.go:475`，**同一个 CSS 类**） | 有 | **无** |
| `extensionIconButton`（`rows.go:542`） | 有 | **无** |
| `paramIconButton`（`parameditor.go:260`） | 有 | **无** |
| `sessionIconButton`（`rows.go:790` 一带） | 有 | **无** |
| `headerButton`（`settingsui.go:1183`） | 有 | **无** |
| `sheetActionButton`（`pluginsheet.go:366`） | 有 | **无** |
| 启动器 `rowControl`（20u）/ 折叠卡设置格（28u） | 有 | **有**（`ControlPress` + `0.92`） |

同一个 CSS 类的两处实现不一致，同一枚 28u 小格在启动器上三档、在设置页两档。
按下时**没有任何反馈**（面与几何都不动），指针下去的那一下与悬停逐像素相同。

## 3 落地

| # | 位置 | 改成 |
|---|---|---|
| 1 | `rows.go` `extensionActionFace` | `pressed := !disabled && button.Pressed()`：面 `ControlPress`、墨 `TextStrong`、`Scale(0.97)` |
| 2 | `rows.go` `extensionIconButton` | 新 `case button.Pressed():`（在 hover 之前）：`ControlPress` + `Scale(0.92)` + `LitFace` |
| 3 | `rows.go` `sessionIconButton` | 同上（`danger` 臂保持 warning 墨） |
| 4 | `parameditor.go` `paramIconButton` | 按下臂在 hover 之前 |
| 5 | `settingsui.go` `headerButton` | 按下臂在 hover 之前（`.toolbar-button`） |
| 6 | `pluginsheet.go` `sheetActionButton` | `pressed` 时面 `ControlPress` + `Scale(0.97)`；`danger` 臂静息面透明，按下才有面（白线仍只在有面时画） |
| 7 | `settingsui.go` `barButton` | 补上 `ControlPress` 的面（几何 0.97 早就在） |

族的规矩写进注释一处引用一处：**胶囊 0.97**（`extensions.css:138-140` 的
`:active:not(:disabled) { transform: scale(0.97) }`）、**小格 0.92**（启动器同尺寸的
`rowControl` / 折叠卡设置格）。

**零改动红线**：只改 `Background` 与 `Scale`（渲染变换，布局盒不动）。高度链、
padding、圆角、`Border`、文案、i18n、点击回调、焦点顺序、键盘模型一个字节没动。

## 4 出图（自己复核过）

`/tmp/floter-P13-q-iconbutton-{light,dark}-{hover,press}.png`（200×120 探针，芯片在
(20,20)）。**这张图上看到的一处变化**：浅色档按住时那块 28u 方格的顶边与两侧从
`ControlHover`（128 地面上 114）退回 `ControlPress`（124），同时整块缩到 0.92，
于是「按住」读起来是把面**按回去**、把格子**收一档**，而不是继续亮着。

像素实测（探针地面 128）：

| 外观 | 静息 | 悬停 | 按下 |
|---|---|---|---|
| 浅色 | 128（透明） | 114 | **124** |
| 深色 | 128（透明） | 146 | **134** |

两种外观的按下档都是「朝静息面退一步」：浅色是更弱的黑洗（0.032 vs 悬停 0.105），
深色是更弱的白洗（0.05 vs 0.105）。所以 pin 断言的是**序**（rest → press → hover），
不是一个方向 —— 方向在两种底上相反。

## 5 门槛（实测）

```
gofmt -l cmd internal                空
go vet ./... / GOOS=windows|darwin    净
go test -count=1 ./...                全绿（0 FAIL）
PASS 2953 → 2963（+10）  SKIP 74（不变）  测试函数 2176 → 2180（+4）
GOOS=linux|windows|darwin go build    过
```

**V 0 / B 0 / S 0**：本轮没有改动任何旧 pin（按下态此前没有像素证据，静息与悬停的
pin 一个字节没动）。

## 6 mutation（3 条实跑，全部变红并逐字节还原）

- **M1** 删掉 `extensionIconButton` 的 `case button.Pressed():` 臂 →
  `TestPressedIconButtonSitsBetweenRestAndHover` 红：
  `[light] the chip under the pointer is {114 …} whether or not the button is down (rest {128 …}, hover {114 …})`
- **M2** 把该臂的 `Scale(0.92, 0.92)` 改成 `Scale(1, 1)` →
  `TestPressedIconButtonSinksItsBox` 红：
  `[light] the pressed chip's painted box is 1.00 of the hovered one ({20 20 27 27} → {20 20 27 27})`
- **M3** 删掉 `extensionActionFace` 的按下臂（面 + 几何两段） →
  `TestPressedPillSitsBetweenRestAndHover` 红：
  `[light] the pill under the pointer is {114 …} whether or not the button is down (rest {128 …}, hover {114 …})`

三条都 `cp` 备份 → 注入 → 看到红 → 还原 → `md5sum -c` 成功。

## 7 用了 qiaomu-design（偷了什么）

读：`SKILL.md` → `references/user-preferences.md`（P-5 可读性优先、P-14 不常驻系统
自言自语、P-1 禁斜体）→ `craft-loop.md` + `preflight.md`（本轮表面是自绘桌面 UI，
走「已有方向，只要打磨」那条车道，不出四方向预览、不等点选）。

- **craft-loop 工艺密度第 4 项（签名交互）**：把「按下」做成这一族**唯一**的几何动作，
  数值从参考自己的 CSS 取 —— 胶囊 `scale(0.97)`、小格 `scale(0.92)`（与启动器同尺寸
  的小格一致），没有自创数值。
- **craft-loop 工艺密度第 12 项（材质细节）**：按下不是换色而是**换一档材质**——
  `--glass-control-press`（浅色黑 0.032 / 深色白 0.05），面从悬停档退回静息档一侧，
  同时仍戴 E1 的一对面（`LitFace`），所以按下时那枚格子还是「一块有受光边的面」，
  不是一块被涂黑的方块。
- **craft-loop 第三节「两轮制」**：第一轮 7 处改完 → 出图 → 评审发现**两处要修**：
  ①浅色档的按下比悬停更**亮**（`ControlPress` 0.032 < hover 0.105），原来的断言
  「按下更暗」是错的，pin 改成断言**序**；②胶囊的 `0.97` 在 20u 宽的探针上只有
  0.6px，落在圆角抗锯齿里量不出来，于是几何交给小格的 0.92 钉，胶囊只钉面 —— 两处
  都写进了注释与 pin 的说明里。

## 8 preflight（逐条）

- **A 反套路**：无渐变白边、无霓虹外发光、无大标题渐变（自绘 UI，本轮只动两块纯色
  面与一次 `Scale`）；无斜体（P-1）；没有新增文案，P-14 不受影响。**过。**
- **B 设计读取一致性**：一页一个主题（深浅两套各跑一遍 pin）；记忆点 = 「按住时格子
  收一档」；动效气质与材质一致（只有颜色 120ms + 一次 transform）。**过。**
- **C 动效抽查**：无 `transition: all`（只有 `Colors: true`）；时长 120ms ≤ 300ms；
  **「按压有 `:active` 反馈」这一条此前在设置页是不成立的，本轮补上**；只动
  background 与 transform；高频键盘操作没有加动画。**过。**
- **D2 中文页**：本轮没有改任何中文文案（红线），故中西文空格、全角标点、行高等
  既有条目不受影响；行高与字号一个字节没动。**过（不适用项已注明）。**
- **E 三秒测试**：这一轮不新增视觉元素，只让已有的面在按住时给出反馈 —— 不争夺注意力。**过。**

## 9 仍待（追加在 P13-p 之后）

1. **`savealert.go` 的 ✕（24u）与 `settingsui.go:5812` 之外的两处 hover 面**（
   `runOutputToggle` 的整行折叠按钮、`custom.go` 的录制面）**没有按下档**：前者是
   「一行字 + chevron」的文本控件（按下缩放会读成抖动），后者是录制中的 accent 面
   （§6.2b 9-11 判「一个面只说一句话」）。**故意不做**，理由写在上面这句里。
2. **`tokens.ControlPress` 在浅色档比 `Control` 更「亮」**（黑 0.032 对黑 0.018）：
   参考的梯子本来如此，但它意味着浅色档的「按下」不是「沉到静息面之下」，而是
   「比悬停弱、比静息强」。要不要把浅色的 `ControlPress` 调到 0.014 以下，属于
   **改 token 基线**，需拍板（本轮不改，pin 只断言序）。
3. **P13-p 第 1 条原样有效**：材质 census 已换成扫函数体，不必再扫这一族。
4. **P13-p 第 2 条原样有效**：设置页「间距节奏」量过 General 页没找到破口，要做得先有
   更强的判据。
5. **P13-k 第 2 条 / P13-k 第 1 条 / P13-c 四条 / P13-h 第 9 条**原样有效（均需拍板）。
