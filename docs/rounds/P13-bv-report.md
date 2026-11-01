# P13-bv · 报告：保存失败横幅的 ✕ —— 这一族最后一枚「按下去纹丝不动」的键

> 总目标（2026-10-30）：细节打磨 + 轻便液态玻璃 + 空间感。
> 选题 = 通用队列「状态反馈」的指针第三档（P13-al / P13-am 那条线的收尾）。
> 规格 `docs/rounds/P13-bv-spec.md`｜合约 `docs/liquid-glass.md` §6.40（新）。

## 一、这轮为什么让它更轻、更有空间感

设置页的**保存失败横幅**是这一支里唯一一条 `Float` 面 + `EdgeHighlight` 冠线的告警带（E2
里的 E1 例外），它上面的 ✕ 是「提前结束这条告警」的唯一出口。此前这枚键只有**静息（透明）**
与**悬停（长出一块面）**两档：指针按下去的那一帧与悬停帧逐像素相同——用户读到的不是「按到了」，
而是「按坏了、没反应」。补上第三档之后，这枚键第一次有**完整的三段阶梯**：无面 → 抬起 →
沉回（`ControlPress` 在悬停面与横幅面之间）。空间感在这里不是多画一条线，而是**同一个小盒子
在指针下走完三步**——这一支 §5 里「按下是唯一的几何变化」那句话，终于在这枚键上也成立。

## 二、先量（改前，720×80，横幅单独成帧，深/浅两套）

探针在 ✕ 盒子内 2px（24u 的键、14u 的记号，2px 落在面上，不会读到字形）。

| 状态 | 浅色（横幅面 250） | 深色（横幅面 20） |
|---|---|---|
| 静息（无面） | `{250,250,251}` | `{20,20,23}` |
| 悬停 | `{224,224,224}`（`ControlHover`） | `{53,53,56}` |
| **按下** | **`{224,224,224}`——与悬停逐像素相同** | **`{53,53,56}`——同** |

根因：`savealert.go` 的 ✕ 只有 `if button.Hovered()` 一条指针臂，没有 `Pressed()`。

## 三、落地（1 处产品改动）

| # | 位置 | 落成 |
|---|---|---|
| 1 | `internal/settingsui/savealert.go` 的 ✕ | 面写成变量 `plane`：`case button.Pressed(): plane = tokens.ControlPress; button.Scale(0.92,0.92)` / `case button.Hovered(): plane = tokens.ControlHover`；`button.Background(plane)` 之后 `if plane.A > 0 { tokens.LitFace(button) }`。一对线**提到臂外**（`launcher/power.go` 的既有形状，`litarm_contract_test.go` 的逃生口 ②），所以按下档也有那一对线——这一族的面是「一块能被碰的板」，压下去仍在地面之上，同 P13-q 在设置页的取舍。`Scale(0.92)` 是小格族的第三档几何（P13-q / P13-al）。 |

## 四、改后（同一把尺）

| 状态 | 浅色 | 深色 | 读法 |
|---|---|---|---|
| 静息 | 250 | 20 | 横幅自己的面 |
| 悬停 | 224 | 53 | 抬起一格 |
| 按下 | **242**（旧 224） | **32**（旧 53） | 沉回：在横幅面与悬停面**之间** |
| 按下帧的横幅面 | 250（不变） | 20（不变） | 第三档只属于这枚键的盒子 |
| 按下帧的键盒 | 与悬停帧同值 | 同值 | `Scale` 是变换，横幅没有重排 |

- 三档的 luma 阶梯（浅 250 > 242 > 224，深 20 < 32 < 53）两个方向都成立。
- **这张图上看到的一处变化**：`press` 帧里 ✕ 背后那块方板比 `hover` 帧**退回去**一截（浅色
  242 对 224），而且整块板还缩了一圈——同一枚键第一次在「按住不放」时说出「按住不放」。
  三帧的 md5 各不相同（`f8ef7290…` / `2ca880c8…` / `011bff43…`），不是同一张图改名。

## 五、V / B / S 台账

- **V 0**：没有一条钉旧数值的 pin 变红（`go test ./internal/settingsui/` 一次跑通）。
- **B 0**：行为/度量/接线 pin 零改动全绿。
- **S 0**：没有结构重写（一个 `if` 换成一个 `switch` + 一个臂外的 `if`；`Label` / `Clicked` /
  24u 的 `Size` / `dismissSaveFailed()` 全部原位）。
- **文档修（不是测试）**：本轮开工时 HEAD 的 `go test ./...` **本来就是红的**——
  `internal/ci` 的 `TestTheLiquidGlassContractSupersedesTheRaycastOne` 报
  「a gap-table row carries an unrecognised status …」（P13-bu 在 §6.39 里写的 39-2 / 39-3
  两行多了第五格，状态列不在第 4 格）。**测试零改动**，修的是文档：那两行并回四格
  （`现状 → 落成` 合到第三格）。原始报错串：

  ```
  liquidglass_contract_test.go:94: a gap-table row carries an unrecognised status "补前提：**只在列表还有别的行时**成立；并新增 `detachedStatusAlone` 谓词与量测注释"
  liquidglass_contract_test.go:101: the gap table has 272 rows, 271 of them carry a status
  ```

## 六、门槛（实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿（exit 0）
--- PASS 计数                → 3215 → 3216（+1）
--- SKIP 计数                → 119 → 120（+1 = opt-in dump）
--- FAIL 计数                → 0
测试函数数（git grep 求和）   → 2374（本轮 +2：1 条 pin + 1 条 dump）
三平台 build                 → linux/windows/darwin 全过
```

## 七、变异（2 条，全部实跑变红并 `md5sum -c` 逐字节还原）

`internal/settingsui/savealert.go` 改前 md5 = `76a2eeca353969e9051255e080f9f032`，两条变异后
都 `cp` 还原并 `md5sum -c` 通过（`成功`），随后 `touch` 复跑避免测试缓存。

| # | 注入 | 原始报错串（抄自实跑） |
|---|---|---|
| M1 | 按下档画成悬停面（`plane = tokens.ControlHover`，即本轮之前的行为） | `dark=false: banner {250 250 251 255 …} rest {250 250 251 255 …} hover {224 224 224 255 …} press {224 224 224 255 …}` / `savealertpress_test.go:158: dark=false: the ✕ under the pointer is {224 224 224 255 …} whether or not the button is down (banner {250 250 251 255 …}): the press rung paints no face of its own` |
| M2 | 删掉臂外的共享一对线（`if plane.A > 0 { LitFace }`） | `litarm_contract_test.go:212: ../settingsui/savealert.go:163:4: settingsui.saveAlert/Pressed paints a face in its pointer arm (plane = tokens.ControlPress` / 同处 `…/Hovered paints a face in its pointer arm (plane = tokens.ControlHover` |

## 八、用了 `qiaomu-design`（打磨模式，未走 Phase 2）

**读了**：`SKILL.md`（Router → 「已有方向，只要打磨」）→ `references/user-preferences.md`
（P-1 禁斜体 / P-5 可读性优先 / P-7 创意不吃功能契约，逐条对照）→ 本轮表面需要的
`references/craft-loop.md`（工艺密度 · 两轮制）与 `references/preflight.md`（A / A2 / B / C / D2）。
**没有**出四方向预览页、没有等用户点选：方向已由用户钉死（轻便液态玻璃），自动车道不许停。

**偷了什么（逐条对 `craft-loop.md` 的工艺密度菜单，写具体数值）**

1. **第 3 项「品牌 focus 环 → 换成本仓的一档强调」的反面用法**：菜单要的是「一个状态一个记号」，
   落成**三段 luma 阶梯**——浅色 250 / 242 / 224、深色 20 / 32 / 53，`pressBetween` 是这条阶梯的
   判据（不是「两个值不同」）。旧实现只有两档，第三档的差值是 **0**。
2. **第 6 项「编辑细节：hairline」**：一对线（`LensRim` + `LensEdge`）从「只属于悬停」改成
   「属于这块面的每一种状态」——提到臂外一次画。像素上静息帧不变（透明面无面无线），
   悬停帧不变，只有按下帧多了那对线。
3. **第 12 项「材质细节：内边框高光」**：按下档**不熄灭**受光边（同 P13-q 的取舍），
   理由是这一族的面压下去仍在地面之上（§3 的「受光边 = 这块面可以被碰」）；这条判断写进了
   `savealert.go` 的注释，而不是只留在报告里。
4. **第 9 项「自定义滚动条 / 状态的可读性」的等价物**：`Scale(0.92)` 这一档几何被**只**用在这枚
   键自己的盒子上，pin ④ 断言横幅面与键盒都不动——「第三档只改自己那一块」是这一族的要求
   （P13-am 的判据 ④）。
5. **两轮制（Render → Critique → Refine）的工程师视角**：「哪里状态缺失」就是本轮的 critique
   ——同一个族里五个键有三档、这一个只有两档；refine 只做一件：把缺的那一档补上，
   不动盒子、不动墨、不动文案。

**preflight 逐条**

- **A（反 AI 套路）**：过。零渐变/零白边/零模糊/零发光，零新增图标或装饰，零斜体，文案零新增
  （`copy.Close` 沿用既有 i18n key）。
- **A2（进攻三步）**：过。DNA 注入 = 本仓自己的 `rowControl` / `fieldSettingsButton` 第三档
  （不是外部风格）；工艺密度 = 上面 5 条；两轮制 = 先量（改前表）→ 工程师视角 critique →
  只修一处 → 复量（改后表）。
- **B（设计读取一致性）**：过。只用既有 token（`ControlPress` / `ControlHover` / `LitFace`），
  零硬编码色值；深浅两套读数逐位对应（242/32 都在各自的 between 区间内）。
  记忆点一句话：**「这条告警上唯一的出口，按下去会回应你。」**
- **C（动效抽查）**：**不适用（本轮零动效）**——`saveAlert` 没有给这枚键加 `Transition`
  （与它此前的悬停一致），`Scale` 是按下态的一帧静态变换，不进时间轴；横幅自己的入场
  `AnimateTo` 一个字节没动。
- **D2（中文页）**：**过一半、一条老账**。本轮零文案改动、零字号改动，所以 D2 的「正文 ≥ 14px」
  既不因本轮变好也不变坏：`tokens.Caption` 与 D2 的张力仍是**需拍板**项（第 10 份证据）。

## 九、仍待（P13-bv）

1. **需拍板（沿用，第 4 份证据）**：按下档要不要保留那一对线。本轮在**设置页这一族**选了
   「保留」（同 P13-q），`internal/ci/litarm_contract_test.go` 的 `pointerFaceAllow` 里仍有
   `launcher.rowControl/Pressed` 与 `launcher.fieldSettingsButton/Pressed` 两条写着「不保留、
   待拍板」。同一个应用里两族两个说法，拍板后一起收。
2. **下一格候选（同类，先量再改）**：① `internal/launcher/output.go:598` 捕获输出页页头的
   两枚 28u 方键（`hover=2 / press=0`，同一个「悬停才长面」形状）；② `internal/launcher/power.go`
   确认带的两枚 chip（`hover=2 / press=0`，但**暖面没有按下档 token**，要先定 token 才动）；
   ③ `internal/launcher/filters.go:219` 的过滤 chip（参考自己就没有面，按下要不要有档需拍板）。
3. **需拍板（沿用）**：`tokens.Caption` / `tokens.Body` 与 preflight D2「正文 ≥ 14px」；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
4. **HEAD 的门槛**：本轮开工时 HEAD **本来就是红的**（§6.39 的两行表格多一格，`internal/ci`
   的合约测试报错）。已按「测试零改动、修文档」处置，写在本报告 §五。

## 十、出图

```
FLOTER_PNG=1 go test ./internal/settingsui/ -run TestDumpP13BV -count=1
/tmp/floter-P13-bv-savealert-light-{rest,hover,press}.png   md5 011bff43… / f8ef7290… / 2ca880c8…
/tmp/floter-P13-bv-savealert-dark-{rest,hover,press}.png    md5 edf5fedb… / 958f7729… / 35b37f23…
```
