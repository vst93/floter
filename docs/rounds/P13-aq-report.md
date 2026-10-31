# P13-aq 报告：禁用面只淡一道——同一条链上最后四枚熄灯

方向：**细节打磨 / 状态反馈的另一半（禁用面）**（`docs/liquid-glass.md` §6 无 `未开始` 行 →
通用队列）。规格：`docs/rounds/P13-aq-spec.md`。合约条目：§6.2b-6 收口 + 新增 §6.14。

## 一、这轮为什么让它更轻、更有空间感

「不可用」在这套材质里只有一句话：**面还在，光熄了**（§3：受光边是「这块面可以被碰」的记号）。
可仓里还有四枚同一个状态被画了两遍淡——框架给 `Disabled` 子树乘 0.5（`ui/paint.go:257-259`），
产品外面又叠一个 `.Opacity(0.x)`，有效 0.2~0.3。结果是面积在、字被洗掉：图标键的字/面对比
**138 → 29 级**（深 141 → 30），kill 药丸 **161 → 34**（深 135 → 29）。更糟的是那两枚 accent 面
的冠线写在禁用臂**之外**——一块按不动的面仍然点着灯，于是它读起来是「能碰但碰不到」，
而不是「不可用」。收成一道淡、把灯熄掉，同一个页面里的禁用态才和 P13-s 修过的胶囊同一种语法，
面与字的层级也不再被两次乘法搅拌成糊。

## 二、落地

| # | 文件 | 改动 |
|---|---|---|
| 1 | `internal/settingsui/rows.go`（`extensionIconButton`） | 删 `.Opacity(0.42)`；褪色只留框架的 0.5 |
| 2 | 同文件（`extensionPrimaryButton`） | 加 `disabled bool`：为真时 `.Disabled(true).Cursor(Default)` 且**不画** `LensRim` 冠线 |
| 3 | `internal/settingsui/config.go`（Save） | 删 `.Disabled(true).Opacity(0.4)`，改传 flag 给 #2 |
| 4 | `internal/settingsui/settingsui.go`（`updateButton`） | 删 `.Opacity(0.6)`；冠线移进 `default:` 臂（可用档才戴） |
| 5 | 同文件（run 表单的 accent 主按钮） | 改传 `len(issues) > 0`，不再自己 `.Disabled(true)` |
| 6 | `internal/settingsui/rows.go`（`sessionKillConfirm`） | `LitFace` 移进 `!disabled` 臂；删 `.Opacity(0.4)` |
| 7 | `internal/ci/disabledfade_contract_test.go`（新） | **链级**反向 census：同一方法链上 `Disabled(true)` + `Opacity(f<1)` 即红，豁免表空 |
| 8 | `internal/settingsui/disabledfade_test.go`（新） | 三条 pin × 深浅两套（字一道淡 / 冠线熄灯 / accent 面档） |
| 9 | `internal/settingsui/disabledfade_dump_test.go`（新） | opt-in 出图（四枚 × 禁用/可用 × 深浅） |
| 10 | `docs/liquid-glass.md` §6.14 | 17-1…17-5 逐行 + 两个反直觉 |

**没有动的东西**：`Disabled(true)` 的命中拒绝、`Cursor` 语义、`Scale`、`Transition`、
`Border`/`Background` 的颜色 token、几何（`MinHeight`/`Padding`/`Radius` 逐字段照抄）、
i18n 文案、键盘模型、高度链、会话管理、设置写回。

## 三、实测（同一把尺：`dimProbe`，litSum/3 = 亮度级）

| 位置 | 量 | 旧 | 新 |
|---|---|---|---|
| 图标键（透明面） | 字/面对比（浅 / 深） | **29 / 30**（启用的 0.21） | **69 / 70**（0.50） |
| 配置页 Save | 冠线行 − 面（浅） | **+10** | **0** |
| | 字/面对比（浅 / 深） | 43 / 40 | 43 / 40（这条路径上那 0.4 本来就没落到像素，见 §6.14 的「反直觉 ②」） |
| 更新 CTA | 面离地面（浅 / 深） | **50 / 54**（启用的 0.30） | **83 / 90**（0.50） |
| | 字/面对比 | 43 / 40 | 43 / 40 |
| kill 药丸 | 冠线行 − 面（浅 / 深） | **+1 / +4** | **0 / 0** |
| | 字/面对比（浅 / 深） | **34 / 29**（0.21） | **84 / 71**（0.52 / 0.53） |

两条反直觉（写进 §6.14，免得下轮重猜）：

1. 「字/面对比减半」只对**透明面**成立（0.21 → 0.50）。字与面同在一条淡里的 accent 面，
   一次淡只把对比压到 **0.26**——两者一起向地面淡，差值不是线性的。所以 17-2 的判据只能是
   「冠线熄灯 + 面档比例」，不能拿 0.5 当门槛（写错的版本在本地被自己的测量否掉了）。
2. `Opacity` 在 `extensionPrimaryButton` 的 accent 填充上**漏掉**了面与字（面 83/165、字 43/168
   与只有 0.5 时逐级相同），在 `updateButton` 上**没有漏**（0.30）。同一种写法两种结果——
   这一族的判据只能是像素，不能只看源码。

## 四、出图与整帧 diff

`FLOTER_P13AQ_DUMP=1 go test -run TestDumpP13AQDisabledFade ./internal/settingsui/` 写出
`/tmp/floter-P13-aq-disabled-{light,dark}-{before,after}.png`（四枚 × 禁用/可用、深浅两套；
before 由重新注入四处旧写法后拍）。PIL 逐像素比：

```
light (400, 420) diff px 11086 bbox (20, 27, 340, 291)
dark  (400, 420) diff px 11078 bbox (20, 27, 340, 291)
```

**这张图上看到的一处变化**：diff 的 bbox `y ≤ 291` 恰好是**禁用那四行**，下面四行可用的孪生
零位移——杀的只是「不可用」这一档，可用档连一个像素都没动。禁用行里最先跳出来的是图标键的
字形从「几乎看不见的浅灰」回到能读，其次 Save 的顶边白线消失，kill 药丸的暖色面暗下去之后
再没有一圈亮边。

## 五、V / B / S 台账

- **V 0**：没有一条钉旧数值的断言。新加的三条 pin 是本轮新账。
- **B 0**：`go test -count=1 ./...` 全绿，行为/接线测试零改动。
- **S 0**：唯一的接口变化是 `extensionPrimaryButton` 多一个 `bool`，两个生产调用点 + 一个测试
  调用点跟着改（编译期强制，没有宽容差）。`internal/settingsui/extensionpanelcss_test.go:618`
  只补了参数，断言一条没动。

## 六、门槛（实测，HEAD `02aaaab7` → 本轮）

```
gofmt -l cmd internal                                # 空
go vet ./...                                         # 净
GOOS=windows go vet ./...                            # 净
GOOS=darwin  go vet ./...                            # 净
go test -count=1 ./...                               # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"    # 3056 → 3060
go test -count=1 -v ./... | grep -c -- "--- SKIP"    # 97 → 98（+1 opt-in 出图）
测试函数（同一把尺：只数 cmd/ + internal/）          # 2263 → 2268
GOOS=linux|windows|darwin go build ./...             # 三个都过
```

## 七、变异（4 条实跑，全部变红）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 图标键加回 `.Opacity(0.42)` | **红**（pin + census 双红）：`light: disabled ink 29 levels, enabled 138 (0.21 of it)` / `internal/settingsui/rows.go:627: this chain both disables the element and fades it by hand` |
| M2 | 主按钮冠线改回无条件 | **红**：`the configuration sheet's Save (light): disabled crown +10 levels over the face` / `the disabled CTA kept a lit edge: top inner row {141 171 213 255} is 10 levels over the face {126 160 207 255}` |
| M3 | kill 药丸 `LitFace` 改回臂外 | **红**：`light: disabled crown +2 levels over the face` / `the disabled kill pill (light): top inner row {246 241 237 255} is 2 levels over the face {245 239 234 255} — the light is still on` |
| M4 | 更新 CTA 加回 `.Opacity(0.6)` | **红**（pin + census）：`the disabled fill sits 50 levels off the ground, 0.30 of the enabled 165 — the fill is faded twice; want one fade (≥0.40)` / `internal/settingsui/settingsui.go:3897: this chain both disables…` |

四条还原后 `cmp` 逐字节核对 `rows.go` / `settingsui.go` 与备份相同（`RESTORED-BYTE-IDENTICAL`）。

## 八、偷了什么（qiaomu-design）

本轮读的是 `user-preferences.md`（偏好账本，最高优先级）+ `preflight.md` 的门禁结构
（A / A4.1 / B / C / D2 / E），走「已有方向、只要打磨」那条车道，**没有** Phase 2 的四方向
预览（任务书禁止在自动驾驶道上出 HTML 预览页，也不许停下等点选）。

- **`user-preferences.md` P-5「可读性优先于气质」** → 落成具体数值：禁用态的字必须 ≥ 启用态的
  **0.40** 对比（`oneFadeRatio`）。1266 小时的项目里，这条账第一次让「禁用」有了可读性下限，
  而不是一句 `.4`。
- **`preflight.md` A4.1（功能型 UI 的状态语义不许混淆）** → 落成 `disabled bool` 参数与
  「冠线只属于可用档」这一条：状态语义从「一处一处记得写」变成函数的一个入参。
- **`craft-loop.md` 第三节「两轮制」**（P13-ap 起一直用的动作）→ 本轮第一轮先**只量不改**
  （写了一个临时探针把 8 个读数打出来），量出「字/面对比减半只对透明面成立」之后，第二轮才改
  判据与代码；写错的断言（拿 0.5 当门槛、把 update CTA 的冠线当判据）都是被第一轮的数否掉的。

## 九、preflight 逐条结论

- **A（反 AI 套路）**：过。这一轮只删东西（两道淡、一圈灯），没有新增渐变/发光/斜体/侧竖线。
- **B（设计读取一致性）**：过。读取 = 「轻便液态玻璃，禁用 = 面在、光熄」，产出与 §6.2b-6 / §6.14
  写下的判据一一对应，深浅两套同值同规则。
- **C（动效抽查）**：过。零动效变更；`Scale` 与 `theme.ColorStep` 一像素没动。
- **D2（中文页）**：**部分不过，如实记录**：本轮没有碰任何 11px 的 `tokens.Caption` 文案，
  但被改的控件里的中文标签仍在 `tokens.Body` 一档；`tokens.Caption` 11px 与「正文 ≥ 14px」的
  张力是 P13-ak 起就记着的「需拍板」，本轮没有扩大它的使用面。
- **E（三秒测试）**：过。眯眼看会话页：kill 药丸禁用时不再有亮边，和旁边的灰盘按钮同属
  「按不动的一族」；extensions 工具栏的图标键禁用时字形仍能读出。

## 十、仍待（P13-aq）

1. **同族未收口的一处**：`uninstall.go:442` 的 `card.Opacity(0.6)`（fixed 组件的卡片自身不是
   `Disabled`，褪色落在整棵子树上，里面还套着一个 `Disabled` 的勾选框）。要连带回答「根上禁用
   还是逐件禁用」，属另一片；已记进 `.pending-bugs.md`。
2. **需拍板（沿用）**：`cardRow` 控制槽 `Shrink(0)`；`tokens.Caption` 11px 与 preflight D2 的张力；
   终端弧 S1b / S1c / S2 尾。
3. 下一轮候选：① `uninstall` 的 fixed 卡片（上面第 1 条）；② 空态的**满态**那一半；③
   `ContentAlpha()` 沿透明度滑块两端（仍未量）。
4. 抢救 stash：`stash@{0}`（`afc38b247c`）与 `stash@{1}`（`114b7d1585`）在 P13-ap 已核过，本轮
   未再试合、也未静默丢弃。
