# P13-fb 报告：终端抽屉的行名「低一档」——先量，量出这是**故意**的

轮号 P13-fb（HEAD 前 P13-fa）。方向：通用队列 · **文案密度 / 字阶**。表面 = **终端页 ·
外观抽屉**（`internal/terminalui/drawer.go`）。合约新增 **§6.123（故意不做，附理由）**。

## 结论一句话

假设是「抽屉是设置页那一行的第二份实现，行名却低一档（`t.FontSize` + 560 +
`TextSecondary` 对 `Emphasis` + 580 + `TextStrong`）」。**量下来不成立**：抽屉的行是
**测量行**，值是这一行的主语——把名字抬一档，名字的墨峰就反超它命名的读数（713 : 707），
并且行盒 17 → 18 让条带长 2px，而条带高 = 画布高 = **PTY 的行数**。两条已落地的契约
（`raycast_test.go:194`、`draweredge_test.go`）当场变红。**生产代码零改动**，结论与判据
写进 §6.123，并用两条 pin 把现状与理由钉住。

## 量的事实（真抽屉 900×400 + 合成行；深浅两套逐值相同）

| 探针 | 现状 | 抬到 emphasis |
|---|---|---|
| 行名盒高（`drawerRow` / `drawerRowStacked` / `drawerPalette` 三处） | **17** | **18** |
| 滑轴读数盒高（`drawerReadout`，`tokens.Body` + mono + 600） | 17 | 17 |
| 行名墨峰 : 读数墨峰（深色，模态底色法） | **390 : 629** | **713 : 707** |
| 条带滚动量 @640×280 | **54.4** | **56.4** |
| 帧上会动的像素 | — | 13268（浅）/ 18952（深） |

两条否掉它的契约：

1. `TestTheDrawerReadoutWearsNeutralInk`（`raycast_test.go:194`）：
   *「the readout's ink (707) is not brighter than its label's (713): the value and its
   name read at one level」* —— 抽屉是一排测量行，**值才是要读的东西**。
2. `draweredge_test.go` 两条：*「the drawer's scroll range moved to 56.4 (P13-dp measured
   54.4 at 640×280): … it is the drawer's height, and the height is the canvas's, and the
   canvas is the PTY's」* —— 视觉轮只改墨与面，高度链不动。

## 落地（生产代码零改动）

- `internal/terminalui/drawerlabel_test.go`（新）：两条 pin + 一条 opt-in 出图。
  - `TestTheDrawerRowsNameIsDeliberatelyTheBodyStep`：三个行形（含真实 `drawerPalette`）的
    名字盒高 **必须等于正文档**，且**不许**等于 emphasis 档。
  - `TestDumpP13FBDrawerPNGs`（opt-in `FLOTER_P13FB_DIR`）。
- `internal/terminalui/textheight_test.go`：阶梯注释补一句「这一档永远是 17」以及为什么
  （抬一档 = 条带 +2px = PTY 少 2px）。
- 合约 §6.123：量表 + 两条否证 + 规矩「**同一角色两个寄存器只在同一类行之间成立**；
  先看这一行里谁是主语」。

## 台账

**V 0 / B 0 / S 0** —— 生产代码零改动，只新增测试与合约行（本轮性质是「量出来不用改」）。

## 门槛（全部实测）

- `gofmt -l cmd internal`：空
- `go vet ./...` + `GOOS=windows go vet ./...` + `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：**3386 PASS / 0 FAIL / 185 SKIP**（基线 3385 / 0 / 184；
  +1 PASS 为 pin，+1 SKIP 为新增 opt-in 出图）
- 测试函数：2555 → **2557**
- `GOOS=linux/windows/darwin go build ./...`：三平台全过

## mutation（2 条实跑 + 1 条等价写法）

1. **M1** `drawerRow` 的行名 `t.FontSize` → `tokens.Emphasis`：
   红，报错串 `drawerRow dark=true: the row's name is 18.0 high, not the body step's 17.0 —
   the drawer's name register moved (§6.123)`（深浅两臂）。
2. **M2** `drawerPalette` 的行名 → `tokens.Caption`：
   红，`drawerPalette dark=true: its name is 15.0 high, not the body step's 17.0 (§6.123)`。
3. 还原：`cp` 回原文件 + `md5sum -c`（`internal/terminalui/drawer.go: 成功`）+ `touch` 复跑
   全绿（避免测试缓存复用）。

出图（被否掉的变体也留了证）：`/tmp/p13fb/shipped/floter-P13-fb-drawer-{light,dark}.png`
与 `/tmp/p13fb/rejected/` 同名两帧，像素差 **13268（浅）/ 18952（深）**，bbox
`(10,14)-(899,202)` / `(0,14)-(899,216)`——整条带都在动，不是一处小数点。
**这张图上看到的一处变化**：rejected 帧里 `Font size` / `Line height` 这类行名比它右边的
`12 px` 读数更抢眼（墨峰 713 > 707），而 shipped 帧里名字退回灰档、读数仍是整行最亮的一处。

## 用 skill 的账（`qiaomu-design`）

开工前读了 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md` →
`preflight.md`（本轮表面是「已有方向、只要打磨」，只读这几份 + 账本，没有全库通读，
也没有走 Phase 2 的四方向预览/HTML 预览页）。

**偷了什么**（逐条落成具体数值/动作）：

1. `craft-loop.md` 二·工艺密度菜单**第 6 项「编辑细节」**的「先量后改」：落成 17 vs 18 的
   盒高实测、390 : 629 的墨峰实测、54.4 → 56.4 的滚动量实测——先量，再决定改不改。
2. `craft-loop.md` 三·两轮制**四视角评审的「工程师」视角**（对不齐/溢出/状态缺失）：
   落成「条带高 = 画布高 = PTY 的行数」这一条，是被量出来才发现的红线，不是读出来的。
3. `craft-loop.md` 二·工艺密度菜单**第 5 项「多层染色阴影」**：不适用（E1 级面按 §2 判据 2
   不画投影），本轮**故意不做**，不硬凑密度。
4. `user-preferences.md` **P-5「可读性优先于气质」**：落成拒绝把行名抬到 `TextStrong`
   ——那样名字比它命名的值还亮（713 > 707），一行的主语就换了。
5. `preflight.md` **A4.1「disabled / read-only / hidden 语义没有混淆」**的反向应用：
   本轮确认抽屉的行名**不是** disabled/read-only 面，它只是安静的那一半，语义没有被借用。

## preflight 逐条结论

- **A 反 AI 套路**：通过。无渐变/发光/纯黑/斜体；本轮只动了测试与文档，帧上的墨一字节未变。
- **B 设计读取一致性**：通过，且**本轮就是这一条抓到的**——两个表面看着像同一角色，
  量完发现抽屉那一行的主语是值而不是名字，于是「同一角色两个寄存器」不适用。
- **C 动效抽查**：通过（不适用）。本轮不触碰动效；`ReduceMotion` 的帧在 pin 里照旧开启。
- **D2 中文页**：通过（不适用）。`zh` 两臂（深/浅）与 `en` 逐值相同：中文行名（`行距`）
  同样是 17 盒高，抬一档同样 18。
- **不适用并说明**：A2（DNA 注入/工艺密度 ≥5）——本轮不是新建界面；A5（四方向预览）
  ——用户已给死方向，走「已有方向只要打磨」车道。

## 仍待

1. **下一格**：通用队列 · **空态满态**的「密度」一格（满态已在 §6.16 / §6.119 收口）。
   「禁用 / 只读面」那一半已由 P13-eu / P13-s / P13-t / P13-aq 收口，**不要重领**。
2. **需拍板（沿用，未动）**：见 `.pending-bugs.md` 顶部「仍待」第 2 条（字号阶梯对
   preflight D2 的 14px、启动器结果行盒高、§6.111/§6.113/§6.114 的尾巴、终端弧 S1b/S1c/S2 尾）。
