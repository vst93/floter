# P11-am 报告 · 「这块面没有 accent」的判据要跟着 accent 走

方向一句话：**上一轮换掉 accent 的色相之后，两条反面 pin 就再也不会红了**——它们的判据把
「红」写死在条件里；本轮把判据换成「相对 accent token 的色度」（并扣掉底面的自身色度），
再把这条历史债冻成自己的一条 pin。

## 1. 选题依据

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完，P11-al 又把 F2 的真根因 accent 换回参考的蓝）
→ 合约未落地条目（`docs/raycast-design.md` §6 全表 `已落地`，只剩 `⌘K` 动作面＝拍板）
→ **上一轮报告「仍待」第 1 条**（P11-al 报告 §7 原文：「这是本轮明确留下的债，不是已收口」）
→ 界面质感。四条拍板项（caret、默认浅色 3.04:1、终端滚动缓冲、`⌘K`）都卡在用户决定上，
所以本轮领这条债，并顺手把「仍待」第 2 条（`palette.contrast()` 没随 accent 换色复核）用测量收掉。

## 2. 缺陷（本轮实测，不是推测）

`internal/launcher/outputaccent_test.go:119-125`（旧行号）里的判据是：

```go
return ri-gi > 24 && ri-bi > 24 && absInt(gi-bi) <= 8   // 「accent 是红」
```

`internal/launcher/raycast_test.go:347`（旧行号）把同一条规则又内联手写了一遍。P11-al 把 accent
换成参考的蓝（浅 `#064AA6` = `(6,74,166)`、深 `#8FB7FF` = `(143,183,255)`）之后：

- 浅色 `R−G = −68`、`R−B = −160`；深色 `R−G = −40`、`R−B = −112` —— 三条子条件全假；
- 于是 `outputaccent_test.go:85` 的 `accent > 0` 恒假、`raycast_test.go:355` 的 `tinted > 0` 恒假；
- 两条 pin（`TestTheOutputSelectedRowIsTheNeutralChosenRung` 的反面半边、`TestTheLauncherListCarriesNoAccent`
  整条）**都失去了失败能力**，却一直是绿的。

同一个判据还有第二处错：它会把 **`Warning` 墨当 accent**（`#9C1F18`：`R−G = 125`、`R−B = 132`、
`|G−B| = 7`，三条都过）。这条在 launcher 列表里没被踩到（那片没有警告墨），但它是判据本身的性质，
M2 的实测报错把这件事原样打了出来（见 §5）。

## 3. 落地

### 3.1 一把跟着 token 走的尺子（`internal/launcher/outputaccent_test.go`）

```go
type accentRuler struct{ ax, ay, bx, by, norm int }   // accent 色度 + 底面色度
func newAccentRuler(accent ui.Color, base color.RGBA) accentRuler
func (r accentRuler) measure(c color.RGBA) (share, degrees float64)
func (r accentRuler) carries(c color.RGBA) bool       // share ≥ 0.05 && 偏角 ≤ 12°
```

三点设计，逐条都有实测依据：

1. **比色度方向，不比某个通道。** 像素的色度取 `(R−B, G−B)`（亮度被减掉）。accent 混进中性面
   （淡染、keyline、抗锯齿字形）就是这条向量乘 alpha，所以 accent 是一条**射线**而不是一个色值。
2. **相对底面量，而不是绝对值。** 这套设计的中性灰自己就带一档蓝（`#F7F7F8` 的色度是 `(−1,−1)`、
   `#1B1B1D` 是 `(−2,−2)`），绝对色度分不出「安静的 accent 淡染」和「冷灰」。第一版用绝对值 +
   残差容差，实测被冷灰打成 3141 / 5523 个假阳性（`{149,149,156}` 那种），所以 ruler 改成
   `chroma(pixel) − chroma(base)` 再投影。
3. **角度是真正的判据。** 8-bit 的 accent 混合像素只偏 1–6°；冷灰偏 ~15°。阈值 12°，配 5%
   的 share 下限（`AccentTintQuiet` 是 0.065 / 0.085，实测投影 0.071 / 0.088）。

三个调用点：`outputRow` 的选中行带（base = 同列的普通行填充）、
`TestTheLauncherListCarriesNoAccent` 的整帧扫描（base = 卡片左内边距的填充）、
以及新 pin 的合成面（base = 面板）。

### 3.2 新增 pin（3 条函数 / +7 PASS）

- `internal/launcher/accentruler_test.go` · `TestTheAccentRulerFollowsTheToken`
  —— 钉的是**尺子的形状**，不是某个颜色：四个互不相关的色相（蓝 `#064AA6`、红 `#E5484D`、
  绿 `#0F7A3D`、紫 `#7A1FA8`）各建一把尺子，自己的色相在 alpha 1 / 0.36 / 0.065 三档必须被认出，
  另外三个色相（含纯色与 0.36 淡染）与中性面板必须不被认出。**任何把色相写死的判据都过不了这条**。
- `internal/launcher/accentruler_test.go` · `TestTheLiveTokensClearTheRuler`
  —— 同一把尺子对着两套外观的真实 token 读：accent 填充 / keyline / 淡染 / 35% 抗锯齿字形算 accent；
  面板、选中行、`Warning` 墨、系统图标暖面不算。
- `internal/theme/theme_test.go` · `TestTheAccentFamilyAlreadyClearsTheReadableContrast`
  —— 收 P11-al「仍待」第 2 条（**实测，不改色**）：

  | 量 | 浅色 | 深色 | 门槛 |
  |---|---|---|---|
  | accent 墨在面板上 | **7.73:1** | **9.54:1** | 4.5 |
  | accent 墨在选中行上 | **6.76:1** | **7.76:1** | 4.5 |
  | `AccentText` 在 accent 填充上 | **7.86:1** | **8.87:1** | 4.5 |
  | accent 填充对面板（非文字控件） | **7.73:1** | **9.54:1** | 3.0 |

  **结论是 `palette.contrast()` 不该碰 accent**，pin 把「高对比度档不许移动 accent 一族」也钉住
  （`Accent`/`AccentText`/`AccentEdge`/`AccentHalo` 四项 high 必须逐字节等于 plain）。

### 3.3 一条实测出来、本轮**没有改**的事实（留给用户）

`AccentEdge` 合成到面板上只有 **2.44:1（浅）/ 1.57:1（深）**，低于 WCAG 对「控件边界」的 3:1。
它不是一个可以自己改的数：浅 0.36 / 深 0.30 就是参考 `--accent-edge` 自己的值，而且它从不单独
承担语义（分段控件的选中段同时有填充与墨的变化）。

**没做**，因为它要么是「向参考对齐」（保持现状），要么是「抬 keyline」（改参考值）——按纪律这属于
需要拍板的项，不擅自实现。已写进 `.pending-bugs.md` 的「需用户拍板」候选。

## 4. 台账 V / B / S

- **V = 0**：没有任何视觉数值变。产品代码 `git diff -- internal/ ':!*_test.go'` **为空**，
  所以本轮没有出图：帧是逐像素同一帧（这也意味着「图上看变化」没有可看的东西，见 §7）。
- **B = 0**：既有 pin 一条没改，`go test -count=1 ./...` 全绿。
- **S = 2 处**（测试结构）：`carriesAccent(c)` → `accentRuler`（判据的度量换了，不是放宽）；
  `raycast_test.go` 的内联红规则 → 同一把尺子。两处的注释都写清了旧规则 → 新规则与原因。
- 测试函数 **1934 → 1937**（+3），PASS **2365 → 2372**（+7，含 4 个子测试），SKIP 27 不变。

## 5. 门槛（本轮实测）

```
gofmt -l cmd internal                    # 空
go vet ./...                             # 净
GOOS=windows go vet ./...                # 净
GOOS=darwin  go vet ./...                # 净
go test -count=1 ./...                   # 0 FAIL
--- PASS 计数                            # 2372（基线 2365，+7）
--- SKIP 计数                            # 27（基线 27）
测试函数数                                # 1937（基线 1934）
三平台 build                              # linux / windows / darwin 全过
```

## 6. 变异（3 条，实跑变红、逐字节还原）

- **M1（产品代码）** `internal/launcher/launcher.go:2228` 的选中行 `tokens.RowSelected` →
  `tokens.AccentTintQuiet`（P8-a 之前的旧语言）：
  `TestTheLauncherListCarriesNoAccent` 红 ——
  `raycast_test.go:361: 30148 pixels below the field carry the accent (e.g. {239 243 249 255})`（浅）/
  `30457 pixels … (e.g. {29 31 37 255})`（深）。
  **同一帧在旧判据下计数为 0**：`{239,243,249}` 的 `R−B = −10`、`{29,31,37}` 的 `R−B = −8`，
  三条红子条件都不成立——这正是「修前绿、修后红」的证据。
- **M2** `accentRuler.carries` 退回退役的红规则：
  `TestTheLiveTokensClearTheRuler` 红，报错同时打出两件事：
  `the quiet tint ({231 235 243 255}) is not read as the accent … share 0.071, off the hue line 2.6`
  与 `the warning ink ({156 31 24 255}) is read as the accent`（旧判据的**双向**错误：对自己瞎、
  对警告墨误报）。`TestTheAccentRulerFollowsTheToken` 亦红（红/绿/紫三把尺子互认）。
- **M3（主题）** 深色 `accent` `#8FB7FF` → `#2B2B30`：
  `TestTheAccentFamilyAlreadyClearsTheReadableContrast` 红 ——
  `accent ink on the panel = 1.37:1, want at least 4.5:1`、
  `AccentText … = 1.27:1`、`the accent fill against the panel = 1.37:1`。
  （第一次试的 `#EFEFEF` **没红**——近白的中性色对比度反而更高；换深灰才打中这条 pin 的判据，
  这也是它的语义边界：它量的是比值，不是色相。）
- 三条都 `cp` 备份 + `md5sum -c` 逐字节成功还原，`touch` 后复跑同一个 `-run` 全绿。

## 7. 图

本轮**不出图**：产品代码零改动（`git diff -- internal/ ':!*_test.go'` 为空），渲染出的帧与
P11-al 的 `/tmp/floter-P11-al-accent-{light,dark}.png` 逐像素相同。本轮改的是**读帧的尺子**，
不是帧。若要看帧：那张图仍是当前状态（侧栏选中项前导图标 = `{6,74,166}` 深蓝 / `{143,183,255}` 浅蓝）。

## 8. 仍待

1. **四条拍板项原样有效**（P11-ak 的表）：搜索框 caret 是否脱离 accent、默认浅色安静档
   3.04:1 是否抬到 AA、终端「上次会话滚动缓冲」是否回来（要插件只含文本的导出）、`⌘K` 动作面。
2. **新增候选（本轮实测，见 §3.3）**：`AccentEdge` 的 keyline 在面板上只有 2.44:1 / 1.57:1。
   保持现状＝对齐参考；抬它就是改参考值 —— 请拍板。
3. `internal/terminalui` 的同族探针本轮复核**不需要改**：它们用 `B−R` 或通道散度
   （`accentguard_test.go:69-75`、`drawermetrics_test.go:228`、`redframe_test.go:88`），
   与色相无关。P11-al 报告把 launcher 与 terminalui 并提是笼统说法，本轮已逐条核对。
4. `outputaccent_test.go` 那条 pin 扫的其实是**整个输出面板内部**（`top..bottom` 的走位走到
   面板边界，实测 y=76..115，40 行），不是「一行」。这比它自己声明的更严，不是缺陷，但注释与
   实现不一致；本轮没动它的范围（动了就是改断言宽度）。
