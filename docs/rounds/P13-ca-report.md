# P13-ca 报告：锁定的字段——灯还亮着一半，凹槽已经没了

选题：`docs/liquid-glass.md` §6 差距表无 `未开始` 行 → 通用队列「状态反馈」的**另一半**
（禁用 / 只读面）。这一族（`DimFace`）此前收过胶囊（P13-s / P13-u / P13-aq）、CJK 卡（P13-ar）、
小格（P13-q），字段是最后一个没走这条路的成员。规格 `docs/rounds/P13-ca-spec.md`。

## 这轮为什么让它更轻、更有空间感

因为这一轮把「不可碰」这件事在**最后一块面**上量清了：字段是 E1 面（`fieldFace` 画
`LensRim` 冠线 + `LensEdge` 暗线），而它在锁定之后**冠线还亮着一半、暗线已经归零**——
一个锁定的字段今天不是「灭了灯的凹槽」，是「一块平涂 + 一条半亮的顶线」。灯与凹槽各剩一半，
正是这一代要收掉的那种「说了一半的话」。**结论落在 §6.44 与「仍待」**：产品本轮不改（见下）。

## 先量（改前/改后都是同一个数——本轮不动产品）

探针 = `lockedform_test.go` 的 `lockProbe`，`extensionField` → `lockedControl`，深浅两套：

| 读数 | 静息 | 锁定（实测） | 结论 |
|---|---|---|---|
| 面（浅 / 深） | 230 / 39 | 238 / 26 | 一道淡，台阶 −9 / −12 ✅ |
| 冠线 − 面 | +6 / +19 | **+2 / +10** | 灯只熄了一半 ❌ |
| 底对侧 − 面 | −7 / −12 | **0 / 0** | 凹槽已经没了 ❌ |

同一张表单上的对照（都在 HEAD）：锁定**胶囊**冠线 **+0**（`DimFace`，P13-s）、锁定**透明面小格**
无灯可熄（P13-u）。所以是「三档对，一档漏」。

## 为什么没改（如实记）

熄灯的正确材质就是 `DimFace`（摘冠线、留暗线），但**它在这里撤不回来**：

- 冠线是 `fieldFace`（`internal/settingsui/textinput.go:74`）在**建元素时**用 `LitFace` 画的；
- mygo 的内阴影**只追加、不撤回**（`mygo@v0.7.3/ui/element.go:1162`：`e.shadows = append(...)`）；
- 试过的两条路都失败并已回退：
  1. `lockedField = DimFace(lockedControl(e))` → 冠线**原样留着**（DimFace 只加底暗线，摘不掉已画的冠线）；
  2. 再加一条 `InsetShadow(0, 1, 0, 0, tokens.Control)` 把那一行盖回去 → 浅色恰好等于面，
     **深色反而更亮**（冠线 36 → **47**，因为 `Control` 在深色是半透明的白，压在冠线上是两层白）。

要真熄灯，得把「锁定」带进字段材质本身（`fieldFace` / `textField` 的三条构建链
`extensionField` → `textInput` → `fieldFace`、`configSelect` → `fieldFace`），也就是改结构而不是改墨。
按纪律本轮**不改**，写进 §6.44（`需拍板 / 结构性`）与 `.pending-bugs.md` 的「仍待」。

## 落地（本轮真正的交付）

1. `internal/settingsui/lockedform_test.go`：`assertOneFade` 新增 `crownLit` 参数，把「灯该不该亮」
   从「半盏灯」这条算术里**分出来**——字段臂今天仍按半盏灯量（现状），但那一行注明了它**不是**答案，
   以及为什么不能就地改。开关 / 列表框 / 设置页禁用开关三臂显式传 `true`（各自的材质不在本轮）。
2. 新增 pin `TestTheLockedFieldIsStillAGroove`：锁定后的字段**面离地 ≥ 6 级**（凹槽还在 z 上），
   并记录两条现状读数（冠线 +10/+2、暗线 0）——这条 pin 是将来那次结构性改法的**护栏**
   （`DimFace` 若落地，冠线归零、暗线回来，它仍然绿）。
3. `docs/liquid-glass.md` **§6.44**：这一格的账（读数、两次失败的改法、为什么它属结构）。

## 门槛（实测）

```
gofmt -l cmd internal          → 空
go vet ./...                   → 净
GOOS=windows/darwin go vet ./... → 净
go test -count=1 ./...         → 全绿
--- PASS 计数                  → 3225 → 3226（基线 3225）
测试函数                       → 2382 → 2383
SKIP                           → 123（不变；本轮无出图）
三平台 go build ./...          → 过
```

**V 0 / B 0 / S 0**：产品代码零改动，只改测试结构（`crownLit` 参数 + 1 条新 pin）。

## 变异（2 条实跑变红并逐字节还原）

- **M1** `lockedControl` 后面加 `.Opacity(0.4)`（把「一道淡」变成两道）→
  `TestTheLockedFieldFadesOnce` 红，原文：
  `the config field (light): the locked face is {243 243 244}, but one fade of {230 230 230} over the ground {247 247 248} is {239 239 239} (off by 4 levels); the control is faded twice — or not at all`
- **M2** `lockedControl` 退成 `return e`（不挂 `Disabled`）→ 同一条 pin 红：
  `the config field (light): the locked face is {230 230 230}, but one fade of {230 230 230} over the ground {247 247 248} is {239 239 239} (off by 9 levels)`。
- 还原：`cp` 备份 + `md5sum -c` 逐字节核对（`成功`）+ `touch` 后复跑绿（避免测试缓存）。

## 偷了什么（qiaomu-design）

- `references/craft-loop.md` **工艺密度菜单第 12 项「材质细节：内边框高光」** → 本轮量的是
  「内高光在禁用面上该不该存在」，落成两个具体数值：锁定字段的冠线 **+10 级（深）/ +2 级（浅）**
  必须为 **0**，底对侧 **−12 级（深）/ −7 级（浅）** 必须回来（现状 0）。这两个数就是
  `DimFace` 在字段上的验收线，已写进 `TestTheLockedFieldIsStillAGroove` 的注释与 §6.44。
- `references/craft-loop.md` **第 5 项「多层染色阴影」的反面**（`§3 受光边是「这块面可以被碰」的记号`）
  → 用来判「半盏灯算不算灯」：算。半亮的冠线仍然在说「这里可以按」，所以它是缺陷不是风格。
- `references/preflight.md` **A（反 AI 套路）**：本轮没有新增渐变、光晕、玻璃贴图；
  「半盏灯」正是 A 要收的那种「看起来有材质、其实只是加了一层白」。
- `references/preflight.md` **D2（中文页）**：本轮零文案改动（`tokens.Caption` 11px 的老账仍挂着，
  见「需拍板」）。
- `references/user-preferences.md` **P-5（可读性优先于气质）**：锁定字段的墨对比 101 级（P13-t 实测）
  保留不动——熄灯只动线，不动字。

## preflight 逐条结论

- **A 反 AI 套路**：过（无装饰性渐变/光晕/伪玻璃；本轮只量线）。
- **B 设计读取一致性**：过（字段仍是 E1，阶梯未变；`fieldFace` 一处决定材质这条没被绕开）。
- **C 动效抽查**：不适用（本轮零动效改动；锁定臂本来就没有过渡）。
- **D2 中文页**：过（零文案改动）。**但** `tokens.Caption` 11px 与「正文 ≥ 14px」的张力仍未解，
  这是「需拍板」第 7 份证据，不在本轮范围。

## 仍待（P13-ca）

1. **下一格（结构性，本轮量到、未改）**：把「锁定」带进字段材质——`fieldFace` 需要一个
   「不画冠线」的变体，三条构建链（`extensionField` / `configSelect` / 别名与参数编辑器的字段）
   各传一次；落地后把 `assertOneFade` 的字段臂 `crownLit` 翻成 `false`（那一行已经准备好）。
2. **需拍板（沿用 + 第 8 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；路径截断保头弃尾；按下档要不要保留那一对线（`pointerFaceAllow` 两条）；
   终端外观抽屉行要不要面；终端弧 S1b / S1c / S2 尾。
4. **抢救 stash 核对（本轮做）**：`git stash list` 顶部 `stash@{0}` = `33e03d1f6a`
   「orphan 2026-11-01 11:58」，`git diff --stat 6bee2c19 33e03d1f6a` 为空（P13-bw / P13-bz
   已判两次），本轮第三次复核结论不变：**没有未提交工作，不要再试合**。
   `stash@{1}`（`114b7d1585`）只改 `.pending-bugs.md`，文字已在文件里（P13-bz 判过）。
