# P13-av 报告 · 多选列表箱的两端：边是算出来的面，门是读出来的位置

规格：`docs/rounds/P13-av-spec.md`　合约：`docs/liquid-glass.md` §6.18（本轮由 `未开始` 转 `已落地`）

**这轮为什么让它更轻、更有空间感**：这是一块有边界的视口——56px 的滚动区里，一行 20px 的
选项会在两端被 `.Clip()` 水平切断。补上两头 3u(12px) 的退让之后，内容不再是被刀切开的，
而是**装在一个盒子里**：滚到中间时两头都软，停在开头/滚到底时正在读的那一行不淡。
盒子一格没长高，行程一格没变短——空间是画出来的，不是让出来的。

## 改了什么

- `internal/settingsui/config.go` `configMultiselectBox`：滚动视口的两端各一块
  `glass.ScrollEdge`；新增 `configListEdgeFace`（不透明合成面）。
- `internal/settingsui/listboxedge_test.go`（新）：4 条像素 pin + 1 条 opt-in 截图 dump。

## 三处「量出来的」决定

1. **边高 3u(12px)，不是原计划的 2u**。2u 的实测最坏值：顶 **44/106（41%）**、
   底 **67/106（63%）**——下沿压不住；3u 后 **35/106（33%）**、**50/106（47%）**。
   与 §6.16/§6.17 同高，也和「视口 56px 里 12px = 21%，且小于一行 20px」相容。
2. **锚在 padding box 里退一格**（`Top(padY)` / `Bottom(padY)`）。P13-au 记的
   「顶边一个像素都没画出来」的根因：绝对定位从 padding box 起算，`Top(0)` 把 8px 的边
   画进箱子的 4px 内边距，下沿那 8px 整个落在空白上（实测下沿仍 106/106 = 100%）。
   这不是颜色问题、不是 `.Clip()` 的问题，是坐标系的问题。
3. **门开在滚动位置上**（`0 < Y < MaxY`）。66px 是原生地板、padding 不许长，所以 §6.16 那条
   「边落在预留空白上」的几何自门控在这里不成立。一格滚动的 A/B 实测（内容只差 1px）：

   | 外观 | 顶：门关 → 开 | 底：门关 → 开 |
   |---|---|---|
   | 浅 | 97 → 52 | 106 → 80 |
   | 深 | 85 → 46 | 93 → 70 |

## 像素证据

- pin ①（19 档滚动位置的最坏值）：浅 顶 4 行 **35/106（33%）**、底 4 行 **50/106（47%）**；
  深 **30/93（32%）**、**44/93（47%）**。改前（§6.18 记录，静止帧）两端 100%。
- pin ③（盒子零改动）：2 项短列表 **MaxY = 0**、盒高 **66px**；12 项长列表盒高 **66px**、
  **MaxY = 206**（12×22 − 2 − 56 = 206）。
- pin ④（边画的是箱子自己的面）：边上最外两行与箱内一条空白行的平均亮度
  **212 vs 213（浅，Δ=1）**、**61 vs 61（深，Δ=0）**——边只让内容淡出，不自己变成第三块面。
- 截图（`FLOTER_PNG=1`，`FLOTER_PNG_TAG` 命名 before/after，同一探针同一帧）：
  `/tmp/floter-P13-av-listbox-{before,after}-{light,dark}.png`。
  **这张图上看到的一处变化**（滚到 `Y=MaxY/2`，直接量 PNG 的行墨展开）：
  视口第 2/3 行 **105/105 → 20/27**（浅）、**95/95 → 18/25**（深）。
  也就是同一条被切断的字行，改前是硬边，改后是被自己的面吃掉大半。

## V / B / S 台账

- **V 0 / B 0 / S 0**：本轮没有重定任何视觉基线，也没有改任何既有断言。
  两处容易被误伤的老 pin 都没动：`TestTheConfigMultiselectIsOneListBox`
  （传 `scroll = nil`，靠本轮新增的 nil 守卫继续绿）、`TestTheMultiselectBoxIsAFaceNotARing`。

## 门槛（全部实测）

```
gofmt -l cmd internal                  # 空
go vet ./... / GOOS=windows / GOOS=darwin  # 三平台净
go test -count=1 ./...                 # 全绿
--- PASS 3079 → 3091（+12）| FAIL 0 | SKIP 100 → 101（新增的是 opt-in 截图 dump）
测试函数 2279 → 2284（+5）
GOOS={linux,windows,darwin} go build ./...  # 三过
```

## 变异（3 条，全部实跑变红并逐字节还原）

`md5sum internal/settingsui/config.go` 改前改后同为 `8b50a59ab97505341a1740feeaba438a`。

| # | 注入 | 红在哪（原始报错串摘录） |
|---|---|---|
| M1 | `Top(padY)`/`Bottom(padY)` → `Top(0)`/`Bottom(0)` | `the box's last 4 rows read at 80 against the inside's 106 (75% of it): the viewport cuts the list flat at the bottom too` |
| M2 | 顶边门 `scroll.Y > 0` → `scroll.Y >= 0` | `scrolling one pixel off the start does not soften the first line (59 → 52): the top edge is gated on the wrong side` |
| M3 | `configListEdgeFace` 直接 `return tokens.Control` | `the top edge reads 206 against the box's own face 213 (Δ=-7): the edge is laying down its own veil` |

## 用了 qiaomu-design skill（打磨模式，未走四方向预览）

读取路径：`SKILL.md` → `references/user-preferences.md`（偏好账本）→ `references/craft-loop.md`
→ `references/preflight.md`。**没有**出 HTML 预览页、**没有**停下等点选：用户已经把方向给死
（轻便液态玻璃），走的是「已有方向，只要打磨」这条车道。

**偷了什么（逐条）**

1. 从 `craft-loop.md` 的工艺密度菜单取**第 12 项「材质细节」（内边框高光 / glass 折射边）**，
   落成：边高 **3u = 12px**、起点 **0.85 → 0** 的线性淡出、底色是**不透明合成面**
   `FaceOver(FaceOver(FaceOver(bg, Sunken), Control), Control)`（实测与箱面 Δ=1 级）。
2. 取**第 6 项「编辑细节 / hairline」的收口纪律**，落成：门开在 `0 < Y < MaxY` 上，
   边**不在空白上留痕**（一格滚动的 A/B 读数 97→52 / 106→80 就是它的证据）。
3. DNA 供体这一条按 `craft-loop.md` 第一节的替代路径处理：本仓的 DESIGN.md 等价物是
   `docs/liquid-glass.md`，签名数值（0.85 / 线性 / 无模糊 / 只给浮层）早已钉在 §3。
   本轮没有引入新的供体，也没有复制任何品牌外观。
4. 密度菜单其余各项（氛围层、染色阴影、自定义滚动条…）属于**系统级**取值，已在 §1-§4
   落过，不按轮重偷；本轮是同一族表面的第三格（§6.16 → §6.17 → §6.18），不重复计。

## preflight 逐条结论

- **A（反 AI 套路）**：通过。本轮没有渐变、没有纯黑、没有外发光、没有渐变字；没有新增文案，
  所以「革命性/赋能」一类空词零命中。**一处不能全过并说明**：本仓仍有两处 italic 的
  空态句子（如 `copy.RunNoOutput`，`settingsui.go` 的 `runOutput`），这是参考实现自己的写法
  （`extensions.css:1259-1267` 的空态就是 italic），属于既有约定，本轮不改、也不扩大到别处。
- **B（设计读取一致性）**：通过。设计读取是「已存在的液态玻璃体系，只补一块视口」，产出与它
  一致——边、面、门都用本仓已有 token 与已有族规则（`glass.ScrollEdge` + `theme.FaceOver`），
  没有引入新材质；记忆点一句话：**「滚到中间时，那块箱子的上下两头都在往自己的面里退」**。
- **C（动效抽查）**：通过（本轮无动效）。边是随滚动偏移连续的静态渐变，没有 `transition: all`、
  没有入场动画；门的开合发生在滚动的 0→1px 之间，跟着滚动连续发生，不是离散跳变。
- **D2（中文页追加项）**：文档注释与报告用中文，界面文案零改动。**一处不能过的已记录**：
  多选选项用的是 `tokens.Body`（本机实测 **10.8px**，即 0.9 缩放下 12px），低于 D2 的
  「正文 ≥ 14px」；这与既有的 `tokens.Caption` 11px 张力同源，属于**全局字阶**问题，
  不是这一轮能擅自改的，已并入 `.pending-bugs.md` 的「需用户拍板」。

## 四视角自评（两轮制）

- 艺术总监：这块箱子此前是「一块被画框圈住的平涂里装着一列被切开的行」；补上两头之后它才
  读成一个视口。**没解决**的是它的宽度——322px 里最右的勾选框离边框还有 8px，属于同一族的
  下一处（未开）。
- 工程师：修好的是坐标系（padding box），不是颜色；这条教训值得记住。
- 甲方：三秒内能认出「这里可以滚动、还有更多选项」——两端软化正是那个提示。
- 读者：12px 的边只吃掉一行的四成，行本身仍然可读；停住时正在读的那一行不被淡。

## 仍待

1. `internal/settingsui/config.go` 的多选箱**宽度**：勾选框列与箱面右沿的 8px 关系还没量过，
   与 `cardRow` 控制槽 `Shrink(0)` 是同一族（需拍板）。
2. `pluginsheet.go:64` 的插件配置弹层（浮层，§6.2 管辖）——同族另一处，独立一轮。
3. 需拍板（沿用）：`cardRow` 控制槽 `Shrink(0)`；`tokens.Caption` 11px / `tokens.Body`
   10.8px 与 preflight D2「正文 ≥ 14px」的张力；终端弧 S1b / S1c / S2 尾。
