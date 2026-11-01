# P13-cb 报告：锁定的字段，灯跟着状态一起熄

选题：`.pending-bugs.md` 的 P13-ca「仍待」第 1 条（上一轮自己量到的那一格）。
规格 `docs/rounds/P13-cb-spec.md`，合约新增 **§6.45**（并把 §6.44 标成「已由 §6.45 落地」）。

## 这轮为什么让它更轻、更有空间感

因为「不可碰」这件事终于在三块面上说同一句话了。锁定的字段此前戴着**半盏灯**——
冠线还比面高 10 级（深）/ 2 级（浅），底对侧那条暗线却被 0.5 那道淡吃掉了，
读起来是「一块平涂 + 一条半亮的顶线」：它一边说「我不能碰」，一边还举着
「这里可以按」的那条受光边（§3）。灯熄掉之后，锁定的字段是一个**干净的、往后退的凹槽**：
面还在 z 上（离地 9 / 12 级），只是不再反光——同一张表单上，字段、下拉框、开关、列表框
现在用同一档材质说同一件事。

## 落地

| # | 位置 | 改法 |
|---|---|---|
| 1 | `internal/settingsui/textinput.go:74` `fieldFace` | 加 `lit bool`：假 → `tokens.DimFace`（摘冠线、留暗线），真 → 照旧 `tokens.LitFace`。字段材质仍然只由这一处决定 |
| 2 | `textinput.go:36` `textInput` | 拆 `textInputLit(c, value, lit)`；`textInput` 是 `lit = true` 的壳。两条 `Shortcut`、用户和弦的 `HandleInput` 全在原处，一行没动 |
| 3 | `textinput.go:118` `extensionField` | 同上：`extensionFieldLit(..., lit)`；`extensionField` 是 `lit = true` 的壳 |
| 4 | `config.go:502` `configSelect` | 加 `lit` 参数（唯一调用点在同一张表单上，同一处 `locked` 判断） |
| 5 | `config.go:643 / :701`（数字 / 文本字段）、`:560`（下拉） | `!locked` 传给材质；`lockedControl` 那一行**保留**（状态仍要挂，0.5 那道淡是框架的） |
| 6 | `custom.go:335`、`parameditor.go:187,222`、`settingsui.go:4507,4533,4869` | 传 `true`——这些字段没有锁定臂，像素不变 |

**为什么不事后叠 `DimFace`**：mygo 的内阴影只追加不撤回（`mygo@v0.7.3/ui/element.go:1162`），
冠线一旦画上就摘不掉（P13-ca 的两条失败路径，§6.44）。所以状态必须在**建面**的那一刻到场——
这就是 `lit` 这个参数存在的全部理由。

## 实测（`lockProbe`，地面浅 247 / 深 14）

| 读数 | 静息 | 锁定（改前） | 锁定（改后） |
|---|---|---|---|
| 面 | 230 / 39 | 238 / 26 | **238 / 26**（不变） |
| 冠线 − 面 | +6 / +19 | +2 / +10 | **0 / 0** |
| 面 − 地 | 17 / 25 | 9 / 12 | **9 / 12**（不变） |

下拉框（`configSelect`，同一条 `fieldFace`）：静息冠线 − 面 **+6 / +19**，锁定后 **0 / 0**。

产品路径（`configField` + `toggleBusy`，`lockedfield_test.go`）：帧上字段顶边那一行，
行闲时比面亮（浅 236 > 230、深 58 > 39），行忙时**逐像素等于面**（238 / 26）。

## pin 台账

- `lockedform_test.go` A 臂 `crownLit`：`true` → **`false`**（P13-ca 备好的那一行，注释从
  「这一轮不改」改成「P13-cb 已熄灯」）。E 臂的 `t.Logf` **转正成断言**（冠线 − 面 ≤ 1 级）。
- 新 `lockedfield_test.go`：
  - `TestTheLockedSelectGoesUnlit`（下拉框跟着熄灯；静息必须先证明它亮着，否则这条 pin 什么也没证）；
  - `TestTheConfigFormFieldGoesUnlitWhenTheRowIsBusy`（**产品路径**：数的是帧上字段自己的顶边，
    不是 builder 的返回值。这是本轮接线的唯一判据——`!locked` 若写死成 `true` 就红）；
  - `TestDumpP13CbLockedField`（opt-in 出图，`FLOTER_PNG=1`）。
- **V 0 / B 0 / S 0**：没有任何旧断言因为「数值变了」而改口径——`lockedform_test.go` 的两处是
  P13-ca 就留好的开关，翻的是语义（半盏灯 → 熄灯），不是放宽。产品侧的 `liteedge_test.go`、
  `raycast_test.go` 等 E1 pin 全绿（静息字段一个像素没动）。

## 门槛（实测）

```
gofmt -l cmd internal            → 空
go vet ./...                     → 净
GOOS=windows/darwin go vet ./... → 净
go test -count=1 ./...           → 全绿
--- PASS 计数                    → 3226 → 3228（基线 3226）
测试函数                         → 2383 → 2386（+2 pin +1 opt-in dump）
SKIP                             → 123 → 124（+1 opt-in dump）
三平台 go build ./...            → 过
```

## 变异（2 条实跑变红并逐字节还原）

- **M1** `fieldFace` 的 `lit=false` 臂退回 `LitFace`（「忘了熄灯」）→ 四条 pin 红，其中产品路径那条：
  `the busy form (light): the field's top edge is {240 240 241 255 [0 0 0]} but its face is {238 238 239 255 [0 0 0]} (2 levels apart) — the row is locked, but its field is still lit (docs/liquid-glass.md §6.45)`
  （深色 `{36 36 38}` vs `{26 26 28}`，10 级）；另有 `TestTheLockedSelectGoesUnlit`、
  `TestTheLockedFieldFadesOnce`、`TestTheLockedFieldIsStillAGroove` 同步红。
- **M2** `lit=true` 臂也走 `DimFace`（「连静息字段也熄灯」）→ 四条红：
  `TestEveryFaceWithAnEdgeWearsTheLitPair`、`TestTheSettingsFieldIsOneLitSlot`（light/dark 两臂）、
  `TestTheLockedSelectGoesUnlit`、`TestTheConfigFormFieldGoesUnlitWhenTheRowIsBusy`
  （后两条是「静息必须先亮着」那条自检 fatal 打出来的）。
- 还原：`cp` 备份 + `md5sum -c`（`成功`）+ `touch` 后复跑绿（避开测试缓存，见 P13-bz 的教训）。

## 出图

`/tmp/floter-P13-cb-lockedfield-{light,dark}-{before,after}.png`（`FLOTER_PNG=1`，
`FLOTER_PNG_TAG=before` 画旧材质）。

**这张图上看到的一处变化**：逐像素差分里浅色 584 px / 深色 620 px，其中**整行 270 px 落在锁定控件的
顶边那一行**——浅色 `{240 240 241}` → `{238 238 239}`（深色 `{36 36 38}` → `{26 26 28}`），
其余是圆角抗锯齿的两三像素。也就是说：形状、面、暗线、高度一个像素都没动，**只熄了那条 1px 的灯**。

## 偷了什么（qiaomu-design）

- `references/craft-loop.md` **工艺密度菜单第 12 项「材质细节：内边框高光」** → 落成两个具体数值：
  锁定字段的冠线 − 面必须 **0 级**（改前 +10 / +2），面 − 地保持 **9 / 12 级**。
  这两个数现在是 `lockedform_test.go` E 臂与 `lockedfield_test.go` 的判据。
- `references/craft-loop.md` **第 5 项「多层染色阴影」的反面** → 「半盏灯算不算灯」的判据：
  算。受光边是「这块面可以被碰」的记号（合约 §3），半亮的边仍然在说「可以按」，
  所以它是缺陷不是风格——这正是本轮把它整条摘掉的理由，而不是把它调暗一点。
- `references/apple-fluid-interfaces.md` **§材质随状态变化，而不是叠一层白** → 决定用
  `DimFace`（摘灯）而不是再加一条 `InsetShadow` 盖住（P13-ca 试过，深色下反而更亮 36 → 47）。
- `references/preflight.md` **B（设计读取一致性）** → 字段材质的决定点仍只有 `fieldFace` 一处；
  `lit` 是参数不是第二条材质路径，三条构建链读的是同一个 `locked`。

## preflight 逐条结论

- **A 反 AI 套路**：过。本轮零新增渐变/光晕/伪玻璃；做的是**删掉**一条装饰性亮线。
- **B 设计读取一致性**：过。E1 阶梯未变（静息字段冠线仍 +6 / +19，`liteedge_test.go` 全绿），
  锁定的字段仍是 E1 的**面**（离地 9 / 12 级），只是不反光。
- **C 动效抽查**：不适用——锁定臂本来就没有过渡，本轮也没加。
- **D2 中文页**：过（零文案改动）。`tokens.Caption` 11px 与「正文 ≥ 14px」的张力仍在
  （「需拍板」第 8 份证据），不在本轮范围。

## 仍待（P13-cb）

1. **下一格**：`aliasField`（`internal/settingsui/textinput.go:183`）自己调 `LitFace`，
   绕开了 `fieldFace`——它是面板字段族里唯一还单独写材质的字段（P13-m 的注记里写着「外圈 `Hairline`
   不动」）。它没有锁定臂，所以不是本轮的范围；但「字段材质只有一处决定」这句话今天还不完全成立，
   下一轮可以把它收进 `fieldFace`（外圈颜色作为参数，或第二个 `Border` 参数）。**先量再改**。
2. **需拍板（沿用 + 第 8 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；路径截断保头弃尾；按下档要不要保留那一对线（`pointerFaceAllow` 两条）；
   终端外观抽屉行要不要面；终端弧 S1b / S1c / S2 尾。
4. **抢救 stash 核对（本轮第四次）**：`git stash list` 顶部 `stash@{0}` = `33e03d1f6a`
   「orphan 2026-11-01 11:58」，`git diff --stat 6bee2c19 33e03d1f6a` 为空（P13-bw / bz / ca 已判三次），
   本轮第四次复核结论不变：**没有未提交工作，不要再试合**。
   `stash@{1}`（`114b7d1585`）只改 `.pending-bugs.md`，文字已在文件里。
