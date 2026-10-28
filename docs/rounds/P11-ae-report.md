# P11-ae 报告：终端标题栏的墨属于它浮着的那块面

方向按任务书选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目
（§6 全表已落地，只剩 `⌘K`＝拍板）→ **上一轮「仍待」第 3 条**（P11-ad：终端页标题栏是一块
没有底的浮条，实测两个方向都不到 2:1）→ 界面质感与交互。**视觉轮，出图四张。**
规格 `docs/rounds/P11-ae-spec.md`，合约更新在 `docs/raycast-design.md` §6.11。

## 1 缺口（实测，本轮复核）

浮条读窗口的四档墨，浮着的面却是画布（`panelFill` 画的调色板底色）。两者相反时：

| 调色板（浅色窗口） | 合成面 | 调色板前景 | 窗口次级墨 `#6B6B70` |
|---|---|---|---|
| `ink` | `(100,101,103)` | **4.80:1** | **1.10:1** |
| `forest` | `(88,95,92)` | **4.96:1** | **1.24:1** |

P11-ad 给的 (c)「墨随合成底明度选」为什么不成立，这张表就是答案：合成面是一层中灰，
调色板前景和中灰之间只有 4.8:1，而窗口的次级墨只有 1.1:1 —— **问题在面不在墨**。

## 2 改法（一条规则）

- `inherit` / 未知 id / **空页**（没有会话，也就没有它的调色板）→ 窗口自己的四档、
  `Face.A == 0`：**逐像素与今天一致**（出厂默认外观零改动）。
- 命名调色板 + 有会话 → bar 穿画布自己的颜色：面 = 调色板底色，alpha 取「在窗口面之上
  让调色板前景过 4.5:1 的**最小** 1/32 档」（`ink` 浅窗 **0.624**、`forest` **0.686**）；
  墨 = 调色板前景；quiet 两档 = 同一墨在**这块合成面**上过 3.0:1 / 2.5:1 的最小不透明度
  （`quietRung`，所以梯子随调色板走，不掺第二种材料）；hover 面 = 前景 0.10。
- 面**只在揭示时**画（`revealed` = hover 或 focus-within，本来就只有这时内容可见），静止时
  浮条不覆盖第一行输出；顶角取面板的 `panelCorner`，因为浮条的矩形就是面板的上沿。
  **没有渐变、rim、sheen、投影变化**——F3 拿掉的「类玻璃边」没有回来。

## 3 台账

- **V=0 / B=0 / S=0**：没有旧 pin 钉过 bar 的墨在命名调色板下的取值（`barchrome_test.go` 的
  两条 pin 都用出厂默认 `inherit`，逐像素不变，实跑仍绿）。
- 新增 pin：`TestTheBarWearsTheColourOfTheCanvasItFloatsOn`（每一套调色板 × 两套窗口外观：
  墨在它自己的面上 ≥4.5:1；quiet 两档 ≥3.0/2.5 且是**最静**的那一档；面是**最薄**的那一档）、
  `TestTheShippedBarKeepsTheWindowInk`（`inherit`/未知 id/空页 = 窗口四档且不画面）、
  `TestTheBarPaintsItsOwnFaceOverAPalette`（像素：`ink` 在浅窗上有面、`inherit` 没有，
  带子里最亮的墨 230 vs 107）。函数 1915 → **1919**（3 条 pin + 1 条 opt-in dump）。

## 4 mutation（实跑，逐字节还原后 `touch` 复跑绿）

- **M1** 调色板分支短路 → 红：
  `barink_test.go:68: amber (window dark=true): the muted rung is not the quietest that clears 3.0 (alpha 1.000)`。
- **M2** 面强制不透明 → 红：
  `barink_test.go:77: ink (window dark=false): the bar's face is at 1.000, want the least readable 0.624`。
- **M3** 删掉 `titleBar` 里的 `Background(inks.Face)` → 红：
  `barink_test.go:163: the bar over `ink` in a light window painted no face: column 450 is the root's black`。

每条 `cp` 还原 → `md5sum -c` 逐字节一致 → `touch internal/terminalui/*.go` → 复跑绿。

## 5 门槛（实测）

gofmt 空 · `go vet ./...` 净 · Windows/Darwin vet 净 · **PASS 2355**（基线 2352，HEAD
`5c1a01a` 实测）· SKIP **22**（基线 21 + 本轮 1 条 opt-in dump）· FAIL **0** · 函数 **1919** ·
三平台 `go build ./...` 过。

## 6 图上实测

`/tmp/floter-P11-ae-bar-{inherit,ink}-{light,dark}.png`（900×40，黑底浮条，指针停在条上）：

- `ink` + 浅色窗口：x=450 那一列从 `(0,0,0)`（根黑）变成 **`(7,8,10)`**——那是调色板底色
  0.624 落在黑底上的合成，**浮条第一次有自己的面**；带子里最亮的墨 **`(230,233,239)`**
  = 调色板前景，而不是窗口的 `(107,107,112)`。
- `inherit` + 浅色窗口：x=450 那一列 **整列还是 `(0,0,0)`**（不画面），最亮墨仍是
  **`(107,107,112)`**（窗口次级档）——出厂默认那张图与改前逐像素一致。
- `ink` + 深色窗口：**不画面**（调色板前景在深色窗口面上本来就 8:1 以上，`barFaceAlpha`
  返回 0），最亮墨 `(230,233,239)`——「够用就最薄」在像素上也能读出来。

## 7 回收 `stash@{0}` 说明

`stash@{0}`（`d2458db0`「floter orphan 2026-10-28 17:58」）给的是本轮骨架：`barInks` 类型、
`canvasInks` 方法、四个调用点改读 `inks.*`、`statusDot` 多一个参数。**原样接上**（`git apply -3`
干净落盘），补齐它缺的部分：**bar 自己的面**（stash 只有墨、没有面，实测那样只有 2.8:1，
不够）、`barFaceAlpha` / `quietRung` / `inkContrast` / `relativeLuminance`，以及三条 pin 与
dump。**没有一条被后续轮次覆盖**；stash 本体保留不删（非破坏性），`.pending-bugs.md` 记「已回收」。

## 8 仍待

1. **`⌘K` 动作面**仍是合约里唯一未落地的条目（§4.6）：页脚画着键帽但 chord 没绑定 →
   **需拍板**（原样有效）。
2. 拍板清单原样有效（P11-aa 的默认浅色安静档、P11-z 的搜索框 caret、P11-y 的滚动缓冲、
   P9-g 表那一批）。
3. **需拍板（本轮缩窄，原 P11-ad 第 3 条的残余）**：bar 的墨/面解决了，但**同一块合成面上
   终端内容本身**在「浅色窗口 + 深色调色板 + 46% 透明度」下仍然只有 ~1:1——那是**画布的
   不透明度**问题（`glassmap.CanvasAlpha`），不是墨的问题；要改就得动透明度语义 → 拍板。
4. 侦察结论（别当 TODO 重领）：`barButton` 的 `Lens` 仍读窗口 rim（≤0.04 alpha，几乎不可见，
   不改）；`barFaceAlpha` 每帧最多 32 次便宜算术，无需缓存。
