# P11-av 报告：抽屉的缝在它自己的上边，不在窗口的底边

## 1. 选它做什么

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 只剩
`⌘K` 动作面＝拍板）→ 上一轮「仍待」第 1 条（抽屉重开的滚动偏移＝**交互决定，需拍板**，
本轮不动它）→ **界面质感与交互逻辑**（用户 2026-10-28 定的优先档）。

这一档里本轮先做了两轮侦察，把「已经干净」的地方量掉，再在剩下的地方找到这条线：

- **终端页整帧审计**（空页/会话 × 抽屉开/关 × `en|zh` × 6 个尺寸 `640×360…1200×800`）：
  **0 条文字重叠、0 条出窗**。跑完删掉（结论记在 §6，别重领）。
- **设置页整帧审计**（5 页 × `en|zh` × 5 个尺寸 × 2 外观，含 `760×360` 这种最小高）：
  **0 条**。同上。
- **终端抽屉的滚轮**（P11-au 把条带做成滚动容器之后，用户真会用滚轮够最后几行）：
  指针在条带上 `tt.Scroll(…, 0, 200)` → `DrawerScroll.Y=33`（＝`MaxY`，够到底）；
  `ReduceMotion` 下 `tt.Wheel` 同效。**滚轮是通的**，不是缺陷。

## 2. 根因

`internal/terminalui/drawer.go:68`（HEAD `786bc7c`）：

```go
BorderWidth(0, 0, 1, 0).BorderColor(tokens.HairlineSoft)
```

`mygo` 的 `BorderWidth` 与 CSS 同序（`ui/api_gen.go:2989-2994`：`BorderWidth sets the widths
of the border on each side, CSS style`），所以 `0 0 1 0` = 只有**下边**。而：

- 参考是 `border-top`（`main:src/styles/terminal.css:627-632`）——一条线，在抽屉上边；
- 抽屉是面板列的最后一个孩子，它的下边就是**窗口的底边**；
- 同一段代码上面的注释（`drawer.go:39-42`，P8-c 写的）明写「只有**上边**那条线」。

于是这条发丝画在了窗口最底一行，并且穿过面板 12u 圆角的抗锯齿过渡；而用户真正会去读的
那条分界（终端输出 × 抽屉）只有一条 255→239 的硬阶。

像素实测（`ui.Tester`，900×600，浅色，x=450，HEAD）：

```
y=337..338  255,255,255   画布
y=339       239,239,240   抽屉第一行 —— 它的面，没有缝
y=597..598  239,239,240
y=599       226,226,227   ← 发丝在这里（窗口底边）
```

`y=599` 这一行横向有 4 种颜色、`x=2` 是 `229,229,229`：一条压进圆角里的残线。

## 3. 落地

- `internal/terminalui/drawer.go:76`：`BorderWidth(1, 0, 0, 0)`——发丝换到上边。
- 同一处注释补 §P11-av：参考行号、CSS 简写第三个槽＝下边、两个实测坐标。
- 色号 `HairlineSoft`（P8-c 的降档）**不动**；面、内边距、gap、cell floor、滚动、总量
  （仍是 1u）**不动**。

修后同一次实测（900×600，浅色，x=450）：

```
y=338       255,255,255   画布
y=339       226,226,227   ← 缝：整宽一色（x=0..899 全等）
y=340..     239,239,240   抽屉的面
y=599       239,239,240   窗口底边 —— 干净的面，没有线
```

`strip` 的盒子前后一致（`{X:0 Y:339 W:900 H:261}`）：内容整体下移 1u，条带高度没变。

## 4. 台账（V / B / S）

- **V = 1**：一条发丝的**边**换了（同一个色号、同一个 1u 宽度，从下边到上边）。没有新值。
- **B = 0**：行为契约零改动（高度链、键盘、会话、设置写回、i18n、命令接线）。
- **S = 0**：结构没变。
- 既有 pin：`internal/terminalui` 全套（含 `TestTheDrawerKeepsTheBarsBand`、
  `TestTheDrawerScrollsItsLastRowsIntoView`、`TestTheDrawerDoesNotScrollWhenItFits`、
  `TestTheDrawerGridCellsKeepTheReferenceFloor`、`TestTheDrawerRowsKeepTheirWordsInsideTheirCells`、
  `TestTheTerminalViewsPaintFromTokensNotLiterals`）**零改动全绿**——`drawerscroll` 的
  `MaxY` 也没被这 1u 碰到（滚动臂仍 `MaxY=33`）。
- 新增测试函数 2 个：`TestTheDrawerSeamIsOnItsTopEdge`（pin）、
  `TestDumpP11AVDrawerSeamPNGs`（opt-in dump，SKIP）。

## 5. 图（`FLOTER_PNG=1`）

`/tmp/floter-P11-av-drawer-seam-{light,dark}.png`（900×600，终端页 + 会话 + 抽屉全开），
对照 `/tmp/floter-P11-av-drawer-seam-{light,dark}-before.png`。

**我实测在这两张图（浅色臂）上看到了什么**（逐行扫中列 x=450 的墨）：

- **修前**：抽屉的上沿在 `y=339` 只是一次面色跳变（`255,255,255 → 239,239,240`），没有
  一条分隔线；而**窗口最底那一行 `y=599` 是 `226,226,227`**——一条横贯整宽的深一档的线，
  在 `x<6` 与 `x>w-6` 处被圆角抗锯齿染成 `229,229,229`，看着像窗口没画干净的下边框。
- **修后**：`y=339` 变成 `226,226,227` 的**整宽一色**的缝，抽屉从 `y=340` 起笔；
  `y=599` 回到 `239,239,240`——和它上面 260 行一样的抽屉面。也就是说：一条线从「窗口
  底边、压在圆角里」挪到了「画布与抽屉之间」，位置换了，粗细与颜色一个字节没改。

## 6. 门槛（全部实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2389（基线 2388）
grep -c -- "--- SKIP"                                  → 33（基线 32，+1 opt-in dump）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 1956（基线 1954）
三平台 build                                            → 过
```

## 7. 变异（3 条实跑变红并还原）

`internal/terminalui/drawer.go` 的 `md5sum` 基线 = `597deb8a6b7047d5396e4ca9ccfb4bbd`，
每条改完 `touch` 后复跑，还原后 `md5sum -c` 逐字节核对 = 成功。

1. **改回 HEAD 的缺陷 `BorderWidth(0, 0, 1, 0)`** →
   ```
   drawerseam_test.go:82: light 640x360: the strip's first row [239 239 240] is its own face —
   the drawer and the canvas above it meet with no seam
   drawerseam_test.go:100: light 900x600: the window's bottom row is [226 226 227], not the
   drawer's face [239 239 240] — the hairline belongs on the drawer's top edge, not on the
   window's bottom edge
   --- FAIL: TestTheDrawerSeamIsOnItsTopEdge
   ```
2. **`BorderWidth(0)`（干脆不画线）** → 同样的 `…meet with no seam`（断言 1）。
3. **`BorderWidth(1, 0, 1, 0)`（两边都画）** → 6 条 `the window's bottom row is [226 226 227],
   not the drawer's face`（5 尺寸 × 2 外观里抽屉放得下的那 3 个 × 2 外观）。

## 8. 仍待（P11-av）

1. **抽屉关闭再打开时的滚动偏移**（P11-au 仍待 1）原样有效：现在**保留**偏移。若要「一打开
   就在顶部」，`ToggleDrawer` 里置 `a.DrawerScroll = ui.ScrollState{}` 即可——一行，但属
   **交互决定**，仍需拍板。本轮没动它。
2. 老拍板项原样有效：外观抽屉 < 900 折单列 / caret / 默认浅色 3.04:1 / 终端滚动缓冲 /
   `⌘K` 动作面 / `AccentEdge` 3:1 / P11-an 的终端便条投影。
3. **侦察结论（本轮实测，别再扫）**：
   - 终端页整帧（空页/会话 × 抽屉开/关 × `en|zh` × `640×360`…`1200×800`）：文字重叠 0、出窗 0。
   - 设置页整帧（5 页 × `en|zh` × `{720,560} {760,360} {760,400} {860,900} {1100,700}` × 2 外观）：0。
   - 抽屉滚轮（`tt.Scroll` 立即路径 + `tt.Wheel` 滑行路径）：通，`Y` 到 `MaxY`。
   - 全仓生产代码只有**一处** `BorderWidth(`（本轮这一处），「线画在哪一边」没有同族第二例。
