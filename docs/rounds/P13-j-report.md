# P13-j 报告 · toast 的动作 chip：`Lens` 最后一个「未开始」的面，与它那条唯一的边界

> 轮号：**P13-j**｜表面：`shell/notices.go` 的 toast 动作 chip（`.app-toast__action`）
> 规格：`docs/rounds/P13-j-spec.md`｜单一来源：`docs/liquid-glass.md` §6.6 第 6-3 行
> ＋ §6.2b 第 9-12 行（两行同指一处，本轮一起收）

## 1. 这轮为什么让它更轻 / 更有空间感（一句话）

这枚 chip 此前戴的是**四层透镜**（rim + 宽 sheen + 暗线 + **四边 `ControlEdge` 环**），
而环画在最后、和第一条 `LensRim` 落在同一个像素行上——所以它既「没有一条亮起来的顶边」，
又「四边都画了一圈框」，在一张已经浮起来的 toast 里读成「贴上去的一块带框小按钮」。
退成 E1 的一对面（顶亮一分、底暗一分、**无环**）之后，它是这一层里一个**站起来的面**：
边由面差说话（浅色 15 级 / 深色 24 级），亮由一条 1px 的线说话，框没有了。

## 2. 根因（实测）

- `internal/theme/theme.go:1056-1061`：`Lens` 四层按 rim → sheen → edge → 四边
  `ControlEdge` 环追加，后加的盖在上面；mygo 的 `insetShadow` 把 hole 从 **padding edge**
  算起，于是 `spread=1` 的环与 `y=1` 的 rim 占**同一个像素行**。
- `internal/shell/notices.go:433-441`（改前）：`ui.Row` + `Height(Space(5.5))` +
  `Padding(0, Space(2))` + `Radius(RadiusSM)` + `Background(Control)`，**没有 `Border`**，
  唯一边界就是那个环；`:440` 是 `tokens.Lens(action)`。
- 实测量到的「环压住 rim」（改前，真 toast，浅色档）：chip 面 194，顶内行 **214**
  （`ControlEdge` 的暗线压住了 `LensRim`），左侧列 **189**（环的墨，不是面）。
  深色档：面 46，顶内行 66（0.09 白 + 0.07 白叠在一行）、左侧列 54（≠ 面）。

## 3. 落地

| # | 位置 | 改成 |
|---|---|---|
| J-1 | `notices.go:440` | `tokens.Lens(action)` → `tokens.LitFace(action)`；注释重写成「E1 的一对面 + 为什么不补边框」 |
| J-2 | `internal/shell/glasscensus_test.go` `.app-toast__action` 条目 | 从「钉调用」翻成 `dropped`（`test:` 指向新 pin）；旧的「toast 没有孤立渲染」理由已过期 |
| J-3 | 新增 `internal/shell/toastactionface_test.go` | 一条像素 pin（五条判据 × 深/浅）+ opt-in before/after 对照图 |
| J-4 | `docs/liquid-glass.md` §6.6 6-3 / §6.2b 9-12 / §6.6 尾段 | `未开始` → `已落地`（附实测），并写明「表里再次没有 `未开始` 行」 |
| J-5 | `internal/settingsui/lensface_test.go` 家族 pin 的注释 | 从「还剩两个调用点」改成「只剩 `control.SegmentedIndex` 一处（9-11 故意不做）」 |

### 3.1 §6 挂着的那半个问题：它没有边框，边界怎么办

**答：不补。** 面差本身就是边界，实测：

| | chip 面 | toast 面（同一行、chip 外 24px） | 面差 |
|---|---|---|---|
| 浅色 | `194` | `209` | **15 级** |
| 深色 | `45` | `21` | **24 级** |

两条理由写在代码注释里：①`Border(1, ControlEdge)` 会把 padding box 缩 1px
（`insetShadow` 从 padding edge 算起），那是**几何**改动，越出「只改墨与面」的红线；
②边框正是 §1 禁掉的「画框分层」——而 M5 的变异实测证明：补一条 `Border` 之后，
左列立刻变成 `{189}`（对面 `{194}`），像素签名与旧的四层环**一模一样**。也就是说
「补边框」不是换个说法，是把刚退掉的东西原样加回来。

### 3.2 一对线的实测（真 toast，`720×580`，settled）

| 外观 | 面 | 顶内行 | 底内行 | 左列 | 右列 | toast 面 |
|---|---|---|---|---|---|---|
| 浅色 | `194` | **219** | **182** | `195` | `195` | `209` |
| 深色 | `45` | **56** | **43** | `45` | `45` | `21` |

**浅色档顶内行 219 这个数要说清**：toast 的 `Float` 面是半透明的，E2 那两层影在它
**下面**透出来，于是 pane 自己的面在 chip 那几行是一条纵向渐变（该行 pane = 236/233，
chip 面 = 194）。所以 219 里既有 rim 也有渐变的贡献；rim 自身的净贡献是 **+2~3 级**
（对照 `docs/liquid-glass.md` §6.1 第 1-2 行的「221→227」）。pin 因此**不**把顶边当
主判据（深色档 56 才是 rim 的干净读数），主判据是底内行与两侧列——这两条在 M1 / M2 / M5
里都是第一批红的。

## 4. V / B / S 台账

| 类 | 条目 | 处置 |
|---|---|---|
| **V** | `internal/shell/glasscensus_test.go` 的 `.app-toast__action` | 从「必须还有 `tokens.Lens(action)`」翻成 `dropped`：那个调用不许回来，新面由 `TestTheToastActionChipIsAFaceNotARing` 逐像素量着 |
| **B** | 其余全部 | 零改动全绿：`TestTheToastActionRunsAndClosesTheToast`（动作执行 + 关闭）、`TestTheToastStackAnchorsPerSurface`、`TestTheToastThrowFollowsTheFrameTransparency`、P13-g 的折叠带子全部原样通过。高度链、键盘模型、`OnAction`/`CloseToast` 接线、i18n 一个字节没动 |
| **S** | — | 无（材质调用替换，结构未变） |

## 5. 门槛（全部实测）

```
gofmt -l cmd internal                       → 空
go vet ./...                                → 净
GOOS=windows go vet ./...                   → 净
GOOS=darwin  go vet ./...                   → 净
go test -count=1 ./...                      → 全绿（exit 0）
PASS 计数                                   → 2917 → 2920（+3 = 2 条外观子测试 + 1 个父测试）
SKIP                                        → 67 → 68（+1 = opt-in dump）
测试函数                                    → 2153 → 2155（+2）
GOOS=linux/windows/darwin go build ./...    → 三平台过
```

## 6. mutation（5 条实跑，全部真红并逐字节还原）

| # | 注入 | 原始报错（节选） |
|---|---|---|
| M1 | `notices.go` 换回 `tokens.Lens(action)` | `[light] the chip's right columns at mid height are {189 189 191 255} but its face is {194 194 196 255} — a ring is drawn around the block: it is an E1 face, not the four-layer lens`（深色同红：`{54} vs {46}`）＋ census `.app-toast__action` 红 |
| M2 | `theme.Tokens.LitFace` 删掉底对侧那行 | `[dark] the chip's bottom rows inside its frame are {45 45 48 255}, not darker than its face {45 45 48 255} — E1 is a pair (§3: 顶亮一分、底暗一分)` |
| M3 | chip 加 `Shadow(0, 4, 12, 0, ShadowAmbient)` | `[light] the chip casts a shadow beside itself ({202 202 203 255} against the pane 24px out {209 209 211 255}): E1 adds none (§2 判据 2)` |
| M4 | 删掉 `Background(tokens.Control)` | 深/浅两档同时红：盒验证（内=外）与 ⑤ 面差归零 |
| M5 | 补 `Border(1, tokens.ControlEdge)` | `[light] the chip's left columns at mid height are {189 189 191 255} but its face is {194 194 196 255} — a ring is drawn around the block` |

每条：`cp` 备份 → 注入 → 跑测试看到红 → 还原 → `md5sum -c` **逐字节核对通过**
→ `touch` 后复跑一次绿（避免测试缓存复用）。

**pin 自己在写的时候被实测纠正过两次**（记下来，免得下次重犯）：
① 一开始把 chip 的盒按「label 矩形 + 2u padding」算，实测发现 `ui.Tester.Find` 返回的
就是 `Shrink(0)` 那一行**自己的盒**（`622.6..665.0`），pad 8 的假设是错的；
② 盒是小数（`66.7..86.5`），渲染器把它**吸附**到 67..86 行，用 `floor` 读顶内行会把
pane 的 238 一起读进来（深色档因此假红：44 vs 45）——现在顶/底都按 `round` 取，
并且左右列也按吸附后的列取，否则只有左边那列能抓到环。

## 7. 图（视觉证据）

`FLOTER_PNG=1 go test -run TestDumpP13JToastActionPNGs ./internal/shell/` 写出
`/tmp/floter-P13-j-chip-{light,dark}.png`：同一个 `Float` 面上并排两枚同尺寸 chip，
上「`Lens` · before」、下「`LitFace` · after」。

**在这张图上看到的一处变化**（dump 里逐行记录的实测值）：浅色档上（`Lens`）chip 的
**顶内行 226、左右侧列 231**；下（`LitFace`）**顶内行 233、左右侧列 232（= 面）**。
也就是「顶边从一条比面暗 6 级的框线，变成一条比面亮 1~3 级的受光边」，左右两侧
不再有框。深色档同一对读数：顶内行 63 → **53**、侧列 46 → **44（= 面）**。

## 8. 「偷了什么」（qiaomu-design / craft-loop 工艺密度菜单）

本轮走 `craft-loop.md` 的**打磨模式**（`SKILL.md`「已有方向，只要打磨」那条路径），
没有出四方向预览（方向已由用户钉死为轻便液态玻璃，自动车道没人能点选）。

| 取自 | 条目 | 落成的具体数值 |
|---|---|---|
| 工艺密度 #5「多层染色阴影」 | 材质分两层说，且只给浮层 | chip 这一层**零投影**（M3 变异守着），影只在 toast 的 E2 上（P13-g 的 `ElevThrow` 10px/26px + 1px/2px）——「面」与「浮」的差别第一次在这张表面上说得清 |
| 工艺密度 #6「编辑细节：hairline」 | 画框只留一条，且不与受光边同色 | chip 的边界从「四边 `ControlEdge` 环」变成「**零条**」：它没有 `Border`，边界由面差 15/24 级承担；toast 自己那条 `InputStroke` 保持唯一 |
| 工艺密度 #12「材质细节：内边框高光」 | 受光边只在「面」上，1px 实色线 | 顶 `LensRim` 0.09（浅色档净 +2~3、深色档 +11）、底 `LensEdge` 0.06（浅 −12、深 −2），两条各说一件事 |
| `apple-fluid-interfaces.md` 的层级观（借判据，不借动效） | 浮层里不许再开浮层 | chip 判为**E1**（§2 判据 1），本轮零新增 E2，`ElevThrow` 一次没动 |
| `preflight.md` A | 不用渐变白边模拟高光 | 受光边是一条 1px 实色线，没有 gradient、没有模糊贴图、没有 glow |

## 9. preflight 逐条（A / B / C / D2）

- **A 反 AI 套路**：过。没有渐变白边、没有玻璃叠玻璃（本轮零 E2）、没有装饰性 glow；
  唯一新增的视觉是「一条亮线换掉一条暗线，并去掉一圈框」。
- **B 设计读取一致性**：过。读作「桌面效率工具的通知层，面向每天开合十几次的用户，
  轻便液态玻璃语言」。所有取值来自 `theme.Tokens`，没有一处字面色值；§4 的度量一字未改。
- **C 动效抽查**：过（本轮零动效改动）。toast 的入场（160ms translate/scale/opacity）、
  chip 的 hover 换面（90ms 只动颜色）原样保留；pin 在 `ReduceMotion` 下跑。
- **D2 中文页**：过（本轮零文案改动）。i18n 一个字符串没碰；chip 的文案是调用方给的
  `toast.Action`（`Retry` / 页面失败的本地化串），本轮没动。

## 10. 仍待

1. **§6.2b 9-11（`control.SegmentedIndex` 的选中段）仍是 `故意不做`**：它的身份线是紧跟
   其后的 `AccentEdge` 四边环，四边都在、把 rim 与 edge 全盖住——换 `LitFace` 画出来的是
   立刻被遮住的两条线。若以后要给它受光边，得先决定「accent 环与白冠线谁说话」，
   这需要拍板，不许擅自改。**全仓 `Lens` 的调用点现在只剩它 1 个。**
2. **本轮观察（新，下一轮的候选）**：toast 的 `Float` 面是半透明的，E2 的两层影在它下面
   **透出来**，于是 pane 的顶 14 行是一条 241→202 的纵向渐变（浅色档）。这是「轻便玻璃」
   想要的效果还是「脏」的读数，需要单独量一轮（同一支的 `glassmap` 链、`main_opacity`
   拉低时的表现）；本轮只是**量到并记下**，没有改。
3. **P13-c 的四条原样有效**：S1b（`CloseAll` 改 detach，等拍板）、S1c（Windows ConPTY）、
   客户端不告而别的检测、capture 通道迁移。
4. **P13-g 的观察原样有效**：`t.Space()` 被当成 px 用的写法可能不止 toast 一处
   （两位数参数是信号），仍未扫全仓。
5. **P13-h 的观察原样有效**：`iconPlate` 的暖面（电源动作的 `IconSurface`）戴 `LitFace`
   时深色档的 0.09 白线几乎读不出来，与 P12-c 的 2-6（CTA 胶囊对侧暗线）同类，需拍板。
