# P13-ao 报告 · 终端状态灯：形说状态，alpha 不再兼职

方向一句话：**把「有程序在跑 / 没有」从动效搬到形上**——实心盘 = 亮，空心环 = 暗。

## 为什么选它

`docs/liquid-glass.md` §6 无 `未开始` 行，按纪律走通用队列「状态反馈」这一格；
P13-an「仍待 3」把「`terminalui.statusDot` 的墨在两套外观下的级数」列成候选。
量下来它不是「墨的级数」问题，是**三个状态共用两档墨、而两档墨的范围还重叠**。

## 实测的根因（改前）

| # | 事实 | 证据 |
|---|---|---|
| 1 | 空白页按「活着」在呼吸 | `exited()` 在 `a.Current() == nil` 时 `return false`（`terminalui.go:1726`），`statusDot` 于是落到呼吸那一支；而 `terminalui.go:906` 的注释写的是「its dot says the page holds no live session」 |
| 2 | 两个状态的范围重叠 | 呼吸 alpha = `1-0.5*breath` → **1.00 ↔ 0.50**；结束档固定 **0.70**。每个 2.4s 周期里约 **40%** 的时间，活着的点比死掉的还暗 |
| 3 | alpha 差太细，且方向会翻面 | 深色 `TextTertiary` 110,110,118 vs 页面 13,13,15；浅色 142,142,148 vs 244,244,245 —— 0.30 的 alpha 差两边都约 33 级 |
| 4 | 1 DIP 的环在这块尺寸上画不出来 | 在 6 DIP（`Space(1.75)`，本仓间距刻度 3.5）的圆上，1 DIP 描边**满墨像素只有 1 枚**、墨量只有圆盘的 **0.08**：读出来是灰影，不是环 |

## 改后（实测）

| 量 | 改前 | 改后 |
|---|---|---|
| 结束了 / 空白页 | 实心盘 `TextTertiary`@0.7 | **空心环** `p.Stroke(..., t.Space(0.375))`，盒子仍是 1.75u |
| 中心像素 | 0.7 的墨 | **页面本色**（深 14,14,16 / 浅 0,0,0 —— 测试里浮条压在黑根上） |
| 满墨像素 | 0（0.7 的墨永远到不了） | **2 枚**（与 `TextTertiary` 逐字节相同） |
| 墨量 / 跑着的圆盘 | —（旧实现是 0.7 的盘） | **0.49**（深）/ **0.49**（浅），带宽 pin 在 0.4~0.9 |
| 宽度 | 6 DIP | **6 DIP**（形差不动布局） |
| 空白页 | 呼吸（说假话） | 与「结束了」同一支；`a.live()` 直量可证 |

## 落地清单

- `internal/terminalui/terminalui.go`：`statusDot` 的暗档改空心环 + 新增 `live()`（只读判定）+ 文档注释改成「形答状态」。
- `internal/terminalui/dotform_test.go`（新）：`TestTheStatusLightAnswersWithAFormNotAnAlpha`（① 空心 ② 满墨 ③ 盒子 1.75u ④ 墨量 0.4~0.9，× 深浅两套）、`TestTheBlankPageDoesNotBreathe`、`TestStatusDotPictures`（opt-in 出图）。
- `internal/terminalui/barchrome_test.go`、`accentguard_test.go`：两条钉「中心 = `TextTertiary`@0.7」的旧断言按 **S** 重定基线（中心 = 底色 / 环 = 满墨）。
- `docs/liquid-glass.md`：§6.3 的 3-6 追加「形态另案」，新增 **§6.12**（15-1 … 15-8）。

**V / B / S 台账**：V 0 / B 0 / **S 2**（上面两条把「实心盘的中心墨」改成「空心环的中心/环」）。
行为契约（高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n / 命令接线）零改动；唯一的代码新增是一个只读的 `live()`。

## 门槛（实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 全净 |
| `go test -count=1 ./...` | 全绿 |
| PASS 计数 | **3050 → 3054**（+4：新测试的两条子测试 + 父 + `TestTheBlankPageDoesNotBreathe`） |
| FAIL / SKIP | 0 / **95 → 96**（新增的是 opt-in 出图那条） |
| 测试函数 | 2257 → **2259** |
| 三平台 `go build` | 过 |

## 变异（实跑，全部变红并逐字节还原）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 环换回 `p.Fill(r, inks.Tertiary.Alpha(0.7), r.W/2)` | **红**：`accentguard_test.go:120` 「the exited dot's centre is {100 100 104} … still a filled disc」、`barchrome_test.go:168` 同一条、`TestTheStatusLightAnswers…` 两套外观都红 |
| M2 | `!a.live()` → `a.exited()`（空白页重新呼吸） | **红**：`dotform_test.go:202` 「blank: the dot's centre is {110 110 118}, 118 levels off the page」+ `:226`「the ring carries 1.00 of the running dot's ink」 |

两条都是先 `cp` 备份、注入、`touch` 后复跑（避开测试缓存），再 `cp` 还原并 `md5sum -c`
逐字节核对（`internal/terminalui/terminalui.go: 成功`）。

## 出图

`/tmp/floter-P13-ao-{live,exited,blank}-{dark,light}.png`（浮条那一块，400×96）。
把 dark 的两帧按亮度打成字符图看：

```
live                 exited
    +##+            　   +##+
   +####+            　 ++..++
   ######            　 #.  .#
   ######            　 #    #
   +####+            　 #.  .#
    +##+             　 ++..++
```

**在这张图上看到的一处变化**：同一个位置、同一个 6 DIP 的盒子，左边是一枚实心盘，
右边中间空出来了——环的内孔 3 DIP 宽，四周仍是满墨（`#`）。两帧的**外廓逐像素一致**，
所以这不是「点变小了」，是「点变成了环」。

## 偷了什么（qiaomu-design）

- `craft-loop.md` 工艺密度菜单 **第 5 项「多层染色阴影」** → 本轮没用：灯不是浮层，
  1.75u 上加影就是噪点（写进 §6.12 的 15-6「故意不做」）。
- 菜单 **第 12 项「材质细节：内边框高光 / 折射边」** → 同样**拒绝**：§1 禁令 3 点名
  「渐变白边冒充高光」。取的是它的反面判据：形要**由几何本身**表达，不靠伪高光。
- 菜单 **第 4 项「签名交互」** → 落在呼吸上：不改周期与 keyframe，只把它从**唯一**的状态
  信号降级为**加固**信号（静止帧也能读出状态）。
- `craft-loop.md` 的**两轮制**：第一轮量出「1 DIP 环 = 0.08 墨量」（chars 图只有一圈灰），
  第二轮才定 1.5 DIP（0.49 墨量）；`preflight.md` D2 的中文可读性与本轮无关（无中文文案改动）。

## preflight 逐条

- **A（反 AI 套路）**：无渐变光晕、无玻璃拟态叠加、无新增装饰；本轮是**减法**（两档墨 → 一档墨 + 一个形）。
- **B（设计读取一致性）**：§6.12 与 §1「一个面只说一句话」一致——灯只说「有没有程序在跑」，
  exit code 归文字、accent 归「哪个被选中」。
- **C（动效抽查）**：呼吸的 2.4s / 30fps / 0.5 / 0.88 与 `ReduceMotion` 路径**一字节未改**；
  新 pin 在 `ReduceMotion: true` 下取值，因此它量的是**静止形**，不是某一帧的运气。
- **D2（中文页）**：本轮无中文字号/文案改动。

## 仍待（P13-ao）

1. **需拍板（沿用 P13-an 仍待 1/2）**：`cardRow` 控制槽 `Shrink(0)`；`tokens.Caption` 11px 与
   preflight D2「正文 ≥ 14px」的张力。
2. 终端弧剩下的都等拍板：S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾（客户端不告而别）。
3. 下一轮候选：通用队列的**空态/满态**（设置页空列表、扩展页无结果、剪贴板无历史），
   或「禁用面」里最后两格 accent 禁用（`9-25b` / `9-26b`，都写着「需拍板」）。
4. 抢救 stash（`114b7d15`、`afc38b24`）本轮复核未变，内容已在 HEAD，没有静默 drop、没有再试合。
