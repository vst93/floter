# P13-el · 扩展页配置工具栏：写入在飞时它仍在指控（报告）

规格 `docs/rounds/P13-el-spec.md`｜合约新增 **§6.107**｜台账 **V 0 / B 0 / S 0**

## 意图

`configToolbar` 的状态词是这张卡上唯一回答「我改的东西存住了没有」的地方。它此前只有两档面：

| 状态 | 词 | 墨 |
|---|---|---|
| 干净 | `ConfigSavedState` "Saved" | `TextTertiary` |
| 脏 | `ConfigUnsaved` "Unsaved changes" | `Warning`（全卡最响） |
| **写入在飞** | **`ConfigUnsaved`**（同一句话） | **`Warning`**（同一档） |
| 失败 | `ConfigUnsaved` + 下方 notice 带 | `Warning` |

第三行是缺陷：写入在飞期间 baseline 还没重设（`finishConfigSave` 才 re-baseline），
`configDirty` 仍为真，所以**按下 Save 之后**这一格仍在用「按下之前」那句话指控用户，
而同一帧里卡片底部的药丸已经在说 `ConfigSaving` 并把两颗药丸都熄了灯。
**同一件事两个控件说两句话，响的那一句还是过时的。**

## 改前读数（实测，900×1400，Integrations 页，深浅各一套）

用「词框里离底色最远的那个像素」量墨（`boxFace`/`boxInk`：卡面是**半透明薄罩**，
`tokens.Control` 浅色 A=18 / 深色 A=26，所以底色只能从图上读，不能拿 token 当底色）：

| 状态 | 浅色墨 | 深色墨（合成后） | 最近的一档 |
|---|---|---|---|
| 脏（按下前） | `{156 31 24}` | `{216 153 142}` | `Warning`，0 级 |
| **在飞** | **`{156 31 24}`** | **`{216 153 142}`** | **`Warning`，0 级** |
| 落地 | `{143 143 149}` | `{109 109 117}` | `TextTertiary` `{142 142 148}` / `{110 110 118}`，1 级（抗锯齿） |

在飞与脏**逐字节相同**——不是「相近」，是同一个像素值。

## 落地（产品代码 5 行）

`internal/settingsui/config.go` `configToolbar`：

```go
if a.configSaveBusy[integration.ID] {
    status, ink = copy.ConfigSaving, tokens.TextTertiary
}
```

- 词复用 **已有的** `copy.ConfigSaving`（"Saving…" / "正在保存…"）——**没有新增 i18n 文案**，
  它就是同一帧里 Save 药丸已经在用的那个词。
- 墨落到与落地后 "Saved" **同一档**（`TextTertiary`）：于是「在飞 → 落地」是**纯换词、零换墨**，
  每次保存不会在两个灰之间闪一下；而「按下 → 在飞」把警报关掉，因为用户已经照做了。
- 一个反直觉的点记在这里：**在飞不是警告**。它此前之所以是警告色，是因为它复用了脏的整档面，
  而不是因为「写入中」这件事有什么问题。

**没动的**：Save/Defaults 两颗药丸的 disabled 与标签、`noticeLine` 的失败带、
`finishConfigSave` 的 re-baseline、任何 i18n 字符串、工具栏行盒与两个图标按钮
（`busy` 仍只由 copy/export 驱动）。

## 出图

`/tmp/floter-P13-el-configstatus-{before,after}-{light,dark}-{dirty,inflight,saved}.png`
（260×31 裁自工具栏那一行：状态词 + 两个图标按钮）。三帧逐对 `compare -metric AE`：

| 帧 | 浅色 | 深色 |
|---|---|---|
| dirty（按下前） | **AE=0** | **AE=0** |
| inflight（在飞） | **AE=477**（bbox **83×12+10+11**） | **AE=477**（bbox **83×12+10+11**） |
| saved（落地） | **AE=0** | **AE=0** |

**这张图上看到的一处变化**：变的只有那一个词自己占的 83×12 的框——`Unsaved changes` 的
琥珀色笔画整段换成 `Saving…` 的灰；框外（两个图标按钮、行盒的上下沿）一个像素没动，
`dirty` / `saved` 两帧逐像素相同。也就是说这一轮**只改了那一格说的话和用的墨，没改任何几何**。

## pin

`internal/settingsui/configsaveword_test.go`：

- `TestTheConfigToolbarStopsAccusingWhileTheWriteIsOut`（深浅两套，四条）：
  ① 按下前那一格必须最近 `Warning`（否则这条 pin 自己就失去了对照）；
  ② 在飞时整帧**不许**再出现 `ConfigUnsaved`；
  ③ 在飞那一格的墨必须最近 `TextTertiary`（候选里放了 `Warning` 与相邻的 `TextMuted`，
     所以「只要不是警告色就绿」不成立）；
  ④ 在飞与落地的墨**逐通道差 ≤3**（落地是换词，不是跳色）。
- `TestDumpP13ELConfigStatus`（opt-in，`FLOTER_P13EL_TAG=before|after`）出图。

## 门槛（实测）

```
gofmt -l cmd internal        空
go vet ./...                 净
GOOS=windows/darwin go vet   净
go test -count=1 ./...       全绿
--- PASS                     3343 → 3344
--- SKIP                     170 → 171（新增的就是那条 opt-in 出图）
测试函数                      2515 → 2517
三平台 go build ./...         过
```

## mutation（3 条实跑，全部变红后逐字节还原）

- **M1 退回**（删掉在飞那一段）：红在两处——
  `the toolbar still reads "Unsaved changes" while the write is out: the form accuses the user of an
  unsaved change in the same frame its Save says "Saving…"` 与
  `the in-flight word is painted {156 31 24 255 …} — nearest the warning step … want the tertiary step`。
- **M2 只换词不换墨**（`ink` 仍走 `t.Warning`）：②绿、③红
  `the in-flight word is painted {156 31 24 255 [0 0 0]} — nearest the warning step … (0 levels off)`，
  证明 pin 钉的是**面**，不只是词。
- **M3 相邻一档**（`TextMuted` 而不是 `TextTertiary`）：③红
  `the in-flight word is painted {128 128 135 255 [0 0 0]} — nearest the muted step {128 128 135 255 [0 0 0]}`，
  证明它不是「只要不是警告色就绿」。
- 每条都用 `cp` 备份 + `md5sum -c`（`成功`）逐字节核对，`touch` 后复跑。

## 偷了什么（qiaomu-design）

开工前读了 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md` → `preflight.md`
（走「已有方向，只要打磨」那条路径，**没有**出四方向预览页、没有等点选——方向是用户给的）。

- **`craft-loop.md` §二 工艺密度菜单第 5 项「多层染色阴影」→ 故意不做**：这一格是**卡内**的文字，
  §3 的规矩是影只给浮层。写下来是因为「密度凑满 5 项」的压力在这里正好是错的。
- **第 6 项「编辑细节」→ 落成一个具体数值**：状态词的墨档定在**全卡最轻一档**
  （`TextTertiary` 浅 `142/142/148`、深 `110/110/118`），与「Saved」共用同一档——
  也就是「在飞」和「已落地」之间**没有任何墨的位移**，差的只有那 7 个字。
- **`preflight.md` A4.1「loading 选择与任务匹配」→ 落地为**：在飞是 loading 态，状态词必须
  跟 loading 走，不能停在 loading 之前的那句话。
- **`user-preferences.md` P-14（加载/成功状态不得常驻占版面）→ 落地为**：`Saving…` 只在
  在飞期间存在（`configSaveBusy` 清掉就消失），落地后回到最轻一档的 "Saved"。
- **`craft-loop.md` §三 两轮制**：第一轮改完 → 出图 + `compare -metric AE` 复核（上面那张表）
  → 第二轮只修「在飞与落地必须同一档墨」这一处（M2 就是它的反证）。

## preflight 逐节结论

- **A 反套路**：过。本轮没有新增颜色、渐变、阴影、字号、图标；改动是一个词与一档墨。
  无纯黑、无斜体、无三等分卡、无假数据、无空词（复用既有 i18n）。
- **B 设计读取一致性**：过。一句话记忆点：**「按下保存的那一刻，警报就该关掉」**。
  一页一个主题（深浅各自独立渲染、各自出图）；无新增动效，气质与既有 120ms 过渡一致。
- **C 动效抽查**：过（不适用面）。本轮没有新增任何动效；既有的 sidebar 120ms 色彩过渡未触碰；
  无 `transition: all`、无 `ease-in`、无 `scale(0)`；「在飞 → 落地」的换词是状态切换，
  不是动画，`prefers-reduced-motion` 无需新增兜底。
- **D2 中文页**：**不能过**，且不是本轮引入的——状态词正是 `tokens.Caption` **9.9px** 那一档，
  对「正文 ≥ 14px」（**第 61 份证据**）。改它要动字号阶梯，属于 `.pending-bugs.md` 里
  长期挂着的**需拍板**项；本轮只换墨不动字号，所以没有扩大这条债。中英混排空格：本轮
  没有新增文案（复用 "Saving…"）；全角标点：中文档「正在保存…」用全角省略号，既有。
- **E 三秒测试**：过。这一格是卡上最安静的一行，三秒内读到的是「有没有事」，不是「发生了什么」。

## 仍待

1. **下一格**：状态反馈的另一半——`saving` 补齐之后，剩下的过渡态是**失败**：
   失败时状态词仍读 `ConfigUnsaved`（与「还没试过保存」同一个面），失败只由下方 notice 带承担。
   要不要让状态词在 `configError` 非空时也换一句，属于**文案级**决定，先量再定；
   或者转「空态满态」。
2. **需拍板（沿用 + 第 61 份证据）**：`tokens.Caption` 9.9px 对 preflight D2「正文 ≥ 14px」/
   浮层页头字号上阶梯（`104-2`）/ 启动器两行结果行 36px 盒装 27 行墨 / 便条行 rank 1.28 /
   终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 / 会话页空态说错对象 /
   `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 /
   路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
3. 抢救 stash（第六十五次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）仍判定
   与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
