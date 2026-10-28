# P11-av 规格：抽屉与画布之间那条缝，在抽屉的上边

## 1. 意图

一行边框的边写错了：终端外观抽屉的发丝画在**窗口底边**上，而用户真正要读的那条缝
（终端输出 × 抽屉）没有任何线。改回「上边」，别无其它改动。

**为什么现在做**：用户 2026-10-28 定的优先档是「界面质感与交互逻辑」。这条属纯质感：
一像素的线出现在错误的位置，还在窗口圆角的过渡里。

## 2. 证据（本轮实测，HEAD `786bc7c`）

- **参考的写法**（`main:src/styles/terminal.css:627-632`）：
  ```css
  .terminal-settings-drawer {
    padding: 10px 12px;
    border-top: 1px solid var(--glass-control-edge);
    background: var(--glass-control);
  ```
  只有 `border-top`——一条线，在抽屉**上边**。
- **本仓的写法**（`internal/terminalui/drawer.go:68`，HEAD）：
  ```go
  BorderWidth(0, 0, 1, 0).BorderColor(tokens.HairlineSoft)
  ```
  `mygo` 的 `BorderWidth` 与 CSS 同序（top, right, bottom, left，`ui/api_gen.go:2989-2994`
  写着 `// BorderWidth sets the widths of the border on each side, CSS style`），所以第三个槽是
  **下边**。抽屉是面板列的最后一个孩子（`drawer` 的注释：`docked at the panel's bottom
  edge`），它的下边就是**窗口的底边**。
- **同一段代码自己的注释**（`internal/terminalui/drawer.go:39-42`，P8-c 写的）：
  「只有**上边**那条线从 `ControlEdge` 降成弱发丝 `HairlineSoft`」——注释说的是上边，代码
  画的是下边。这不是设计变更，是代码与自己的合约不一致。
- **像素实测**（`ui.Tester`，HEAD，900×600，浅色，中列 x=450）：

  | y | 位置 | 颜色 | 读法 |
  |---|------|------|------|
  | 337–338 | 画布 | `255,255,255` | 终端输出的面 |
  | 339 | 抽屉第一行 | `239,239,240` | 抽屉**面**的直接起笔——**没有缝** |
  | 597–598 | 抽屉面 | `239,239,240` | |
  | 599 | 窗口最底一行 | `226,226,227` | **发丝在这里** |

  且 `y=599` 那一行横向有 4 种颜色（`x=2` 是 `229,229,229`）：这条线穿过了 12u 圆角的
  抗锯齿过渡——它是画在窗口边缘上的一条残线，不是分区的线。

## 3. 逐条落地项

| 位置 | 现状 | 应改成 |
|------|------|--------|
| `internal/terminalui/drawer.go:68`（条带的 `BorderWidth`） | `BorderWidth(0, 0, 1, 0)`：发丝在抽屉下边＝窗口底边 | `BorderWidth(1, 0, 0, 0)`：发丝在抽屉上边＝画布与抽屉的交界 |
| 同上注释块 | 只写了「只有上边那条线」 | 补 §P11-av：写明参考行号、CSS 简写第三个槽＝下边、以及本轮实测的两个像素坐标 |

**不动**：色号（`HairlineSoft`，P8-c 的降档决定保留）、抽屉的面（`Control`）、内边距、
gap、cell floor、滚动（P11-at / P11-au）、`BorderWidth` 的总量（仍是 1u，布局高度不变）。

## 4. 行为契约零改动红线

高度链（面板列 `body Grow(1)` + 抽屉自然高）、键盘模型、会话管理、设置写回、i18n 文案、
命令接线：**零改动**。本条只挪一条线的边——墨的量与颜色都不变，只有它落在哪一边变
（内容整体下移 1u，抽屉盒子高度不变：900×600 实测 `strip={Y:339 H:261}` 前后一致）。

## 5. 门槛命令 + PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2388，本轮应 2389
grep -c -- "--- SKIP"                                  # 基线 32，本轮 33（+1 opt-in dump）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l   # 基线 1954，本轮 1956
GOOS=linux|windows|darwin go build ./...               # 三平台过
```

## 6. 新增 pin

`internal/terminalui/drawerseam_test.go`

- `TestTheDrawerSeamIsOnItsTopEdge`：2 外观 × 5 尺寸（`640×360`、`720×360`、`720×600`、
  `900×600`、`1200×800`）：
  1. 条带第一行是**缝**：与它的面不同；
  2. 缝是**整宽一色**（`x=0..w-1` 全等，不许半截线）；
  3. 缝的上一行不是缝色（发丝没漏进画布）；
  4. 抽屉放得下（`DrawerScroll.MaxY == 0`）时，**窗口最底一行＝抽屉的面**（底边上没有线）。
- `TestDumpP11AVDrawerSeamPNGs`（`FLOTER_PNG=1`，SKIP）：`/tmp/floter-P11-av-drawer-seam-<light|dark>.png`。

> 面的读法：条带第一行**下面那一行**（`top+1`）。不许读条带中段——1200×800 时中段落在
> 抽屉里的调色板预览上（浅色预览是白的），会读成画布。见报告 §3。

## 7. mutation 计划（全部实跑）

1. 把 `BorderWidth(1, 0, 0, 0)` 改回 `(0, 0, 1, 0)`（＝HEAD 的缺陷本身）→ 断言 1 与 4 双红。
2. 改成 `BorderWidth(0)`（干脆没有线）→ 断言 1 红。
3. 改成 `BorderWidth(1, 0, 1, 0)`（两边都画）→ 断言 4 红（底边上多一条线）。

每条都要 `md5sum -c` 逐字节还原 + `touch` 后复跑一次绿。
