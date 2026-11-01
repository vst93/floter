# P13-cp 报告 · 捕获输出页的便条板：一道淡，不是两道

**这轮为什么让它更轻、更有空间感**：捕获输出页左列那 28u 的图标板此前有**两个层高**——
门板站在 E1 上（面差 18 / 24），便条板只站到一半（11 / 15），于是同一列里的板**不在同一条
地平线上**，读起来是「越往下越糊」；现在它们同高，便条由「灯熄了 + 字低一档」说，不由
「整块淡下去」说——**一列东西一个高度，是空间感最便宜也最容易被漏掉的那一半**。

## 1. 证据（先量后改）

pin 在改动之前跑出来的原始报错串（HEAD `5b2eb21b`，产品代码未动）：

```
--- FAIL: TestTheNoteRowPlateCarriesOneFadeNotTwo/light
    noteplate_test.go:41: the note plate's face steps 11 levels off its ground and the door plate's steps 18: two registers of one material (liquid-glass.md §6.2b-6 的「一道淡画两遍」)
    noteplate_test.go:45: the note plate's face lifts only 11 levels off its ground ({242 242 243 255} over {253 253 254 255})
    noteplate_test.go:56: the note plate's counter-side line is 9 levels and the door plate's is 14: DimFace keeps the shade and drops the light, not the other way round
--- FAIL: TestTheNoteRowPlateCarriesOneFadeNotTwo/dark
    noteplate_test.go:41: the note plate's face steps 15 levels off its ground and the door plate's steps 24
    noteplate_test.go:45: the note plate's face lifts only 15 levels off its ground ({38 38 39 255} over {23 23 25 255})
    noteplate_test.go:56: the note plate's counter-side line is 5 levels and the door plate's is 10
```

三行样本（720×488 的真捕获输出页：选中门 / 静息门 / 便条），`channelGap` 级数：

| 外观 | 行 | ground | face | 面差 | 冠线 | 底对侧 |
|---|---|---|---|---|---|---|
| 浅 | 门（静息） | 253 | 235 | **18** | 5 | 14 |
| 浅 | 便条（改前） | 253 | 242 | **11** | 0 | 9 |
| 浅 | 便条（改后） | 253 | 235 | **18** | 0 | 14 |
| 深 | 门（静息） | 23 | 47 | **24** | 19 | 10 |
| 深 | 便条（改前） | 23 | 38 | **15** | 0 | 5 |
| 深 | 便条（改后） | 23 | 47 | **24** | 0 | 10 |

**读出来的两件事**：① 便条板的面色改前改后从 `242/38` 变成 `235/47`——**与门板逐字节相同**；
② 冠线在改前**就是 0**，说明第二道 `Opacity(0.62)` 没有做任何材质上的事，它只是把
`DimFace` 已经做完的结果又乘了一次。

## 2. 改法（只改一块板的面与它里面那枚字形的墨）

`internal/launcher/output.go`（`drawOutputRow` 的便条臂）：

```
- plate := a.iconPlate(c, false, dim)
- if dim {
-     plate.Opacity(0.62)
- }
- plate.Children(func() {
-     if glyph, ok := outputGlyph(row.Icon); ok {
-         ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
+ plate := a.iconPlate(c, false, dim)
+ plate.Children(func() {
+     if glyph, ok := outputGlyph(row.Icon); ok {
+         ink := t.TextMuted
+         if dim {
+             ink = tokens.TextTertiary
+         }
+         ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(ink)
```

- **为什么删的是 0.62、留的是 `DimFace`**：`DimFace` 是**材质**（面在、底对侧在、冠线熄），
  0.62 是**不透明度**。同一件事在 P13-s / P13-aq / P13-u / P13-y 已经判过四次：「一道淡画两遍」
  的结果永远比它想说的弱一档，而材质是唯一能被 pin 钉住的那一半。
- **为什么字形要低一档墨**：删掉 0.62 之后，字形会与门板字形同墨（实测 122 / 122）。
  「便条」这个登记改由**墨阶**说：标题 `TextMuted`（不动）、字形 `TextMuted → TextTertiary`，
  实测 **122 → 93**（浅）/ **122 → 70**（深）。这是设计自己的第四档墨，不是又一个手调 alpha。
- `internal/launcher/glyphs.go` 的 `iconPlate` 注释同步改掉（原话是「参考的 0.62 由调用点
  施加」——那句话从本轮起是假的）。
- **一个字节没动**：行高、行几何、28u 板宽（pin 断言板宽 = 28px）、字阶、字重、标题墨、
  i18n 文案、键盘模型、会话管理、命令接线。

## 3. V / B / S 台账

- **V 0**：没有任何一条视觉 pin 因为「旧值 → 新值」而改写（旧值本来就没有 pin 钉过）。
- **B 0**：`internal/launcher` / `internal/ci` / `internal/settingsui` 全绿，零测试改动。
- **S 0**：结构没变（一个调用点少了一次 `Opacity`，字形墨多了一个变量）。

## 4. 门槛（实测）

```
gofmt -l cmd internal                        → 空
go vet ./...                                 → 净
GOOS=windows go vet ./...                    → 净
GOOS=darwin  go vet ./...                    → 净
go test -count=1 ./...                       → 全绿，0 FAIL
go test -count=1 -v ./... | grep -c "--- PASS"  → 3280（基线 3277）
--- SKIP 计数                                  → 134（基线 133，新增 opt-in 出图 dump）
func Test 计数                                 → 2428（基线 2426，新增 pin 1 + dump 1）
GOOS=linux/windows/darwin go build ./...     → 过
```

## 5. mutation（3 条，全部实跑）

| # | 注入 | 结果 | 原始报错串 |
|---|---|---|---|
| M1 | 把 `if dim { plate.Opacity(0.62) }` 放回去 | 红（浅/深各 3 条） | `the note plate's face steps 11 levels off its ground and the door plate's steps 18` / `the note plate's counter-side line is 9 levels and the door plate's is 14` |
| M2 | 字形墨回退 `t.TextMuted` | 红（浅/深） | `the note plate's glyph reads 122 levels off its face and the door's 122 (a step of 0)` |
| M3 | `iconPlate` 的 `dim` 臂换 `tokens.LitFace` | 红（浅/深） | `the note plate is lit: its top row lifts 5 levels above its face — a note is not a door` |

`md5sum -c` 逐字节还原（`output.go` 成功、`glyphs.go` 因注释编辑而不参与），`touch` 后复跑绿。

## 6. 出图（自己复核过的）

`FLOTER_PNG=1` 导出 4 张：`/tmp/floter-P13-cp-noteplate-{light,dark}-{before,after}.png`。

- 前后差分：**762 px**（浅）/ **762 px**（深），bbox `(16,204)-(44,232)`——**恰好是那块
  28×28 的板**，上面两行门板零位移。
- **这张图上看到的一处变化**：`dark-after.png` 里第三行（便条）左边那块板，与它上面两行门板
  的板**是同一个灰**（47），只有板里那枚 file 字形比门板的暗一点；`dark-before.png` 里同一块板
  明显比上面两行浅一档（38），字形反而更亮——一列板里最低的那块塌下去了。

## 7. 「偷了什么」（qiaomu-design，`craft-loop.md` 的工艺密度菜单）

本轮读的是 `SKILL.md` → `references/user-preferences.md` → `references/craft-loop.md` +
`references/preflight.md`（打磨模式那条路径，**不走** Phase 2 四方向预览：方向是用户定死的
「轻便液态玻璃」）。菜单逐项落地：

1. **第 5 项「多层染色阴影」**——本仓的等价物不是 blur 而是**面差 + 顶受光边 + 底对侧**三层。
   落成具体数值：便条板的面差 **18 / 24**（与门板逐字节相同）、底对侧 **14 / 10**、冠线 **0**。
   这一项本来在 `DimFace` 里就有，本轮只是**不让第二道不透明度把它乘掉**。
2. **第 12 项「材质细节：内边框高光」**——受光边**熄灭**成为「不可碰」的唯一记号。落成：
   便条板顶行 = 面本身（差 **0**），门板顶行 **5 / 19**（浅/深）。一条线在不在，是这一族
   全部判据的来源。
3. **第 6 项「编辑细节：hairline / 小字」**——「便条」的登记从整块淡改成**墨低一档**。
   落成 `TextMuted → TextTertiary`（浅 `#8E8E94` / 深 `#6E6E76`），实测字形离面
   **122 → 93 / 70**，是墨阶的一步而不是 alpha 的一步。

**没偷的**（写清楚，免得下轮当成漏项）：第 1 项氛围层（本仓没有 backdrop，噪点会与
`glass_step` 打架）、第 9 项自定义滚动条（早已由 `t.Theme.Scrollbar` 接管）、
第 4 项签名交互（这一族是静态材质，动效属另一片）。

## 8. preflight 逐条

- **A（反 AI 套路）**：过。本轮只**减**不增——删掉一道全局不透明度，新增的墨是设计自己的
  第四档 token。没有渐变白边、没有外发光、没有斜体、没有三等分卡片。
- **B（设计读取一致性）**：过。一页一个主题（深浅两套各跑一遍，`tt.SetDark` 切画布）；
  记忆点一句话：**「便条不是门：灯熄了、字低一档，板还站在同一层」**；本轮零动效改动，
  不存在「动效气质与视觉气质不一致」。
- **C（动效抽查）**：**不适用**，逐条给理由——本轮没有新增/修改任何 `Transition`、没有
  hover/按压臂、没有 keyframes、没有 `scale`；`output.go` 里唯一的动效是既有的
  `ContentTransition{Numeric:true}`（读数），一个字节没动。
- **D2（中文页）**：过，带一条既有注记。本轮不碰任何中文文案、不新增字号（新增的只有
  一个**颜色**），行高与字阶零改动；`tokens.Caption` 11px 对「正文 ≥ 14px」仍是**既有
  拍板项**（第 17 份证据），本轮不新增证据也不擅自改。
- **P-5（可读性优先于气质）**：过——便条板的面与底对侧都变强了（11 → 18、9 → 14），
  字形只低一档（122 → 93），没有掉到「看不见」的那一侧（93 级仍远高于 3 级的判定阈值）。
- **P-46（默认由当前 agent 执行，不调 K3）**：过，本轮全程本机执行。

## 9. 下一格（自己定）

`.pending-bugs.md` 里「不需拍板」的下一格是**扩展页空态只有一行结论、没有出路**
（`settingsui.go:1838`：`hint` 空串、`action` nil）——但它要动 i18n（补一句理由），
属红线，**先量再进拍板**。所以下一轮走通用队列里不需要拍板的：**设置页
`actionWithGlyph` 的 hover/press 是「墨 + 整行不透明度」两道**（`rows.go:1670-1678`），
与本轮同一族的**第二处**，先量再决定收不收。
