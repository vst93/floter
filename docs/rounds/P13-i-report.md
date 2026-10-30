# P13-i 报告 · 设置页的十处控件面：`Lens` 的受光边一次都没亮过

> 轮号：**P13-i**｜表面：`settingsui`（细节打磨队列，不是新表面）
> 规格：`docs/rounds/P13-i-spec.md`｜单一来源：`docs/liquid-glass.md` §6.2b（新增）

## 1. 这轮为什么让它更轻 / 更有空间感（一句话）

设置页里十个控件此前戴着「四层透镜」——一条受光边 + 一片宽 sheen + 一条暗线 +
**四边画框**，而画框画在最后、把受光边压在同一个像素行下面：这块表面上的控件
**从来没有一条亮起来的顶边**，读起来就是「一堆用 1px 描边分家的平面」。退成 E1 的
一对面（顶亮一分、底暗一分、无环）之后，这一页的控件第一次靠**面差 + 一条受光边 +
对侧暗线**站起来，画框只剩控件自己那一条 `Border`。

## 2. 根因（实测，不是推断）

- `internal/theme/theme.go:1056-1061`：`Lens` 四层按「rim → sheen → edge → 四边
  `ControlEdge` 环」追加，后加的盖在上面。
- mygo `ui/paint.go:1241-1262`（`insetShadow`）：内阴影的 hole 从 **padding edge**
  算起。`spread=1` 的环占 padding 第一行；`y=1, spread=0` 的 rim 的 hole 也落在
  padding 第一行 —— **两条线是同一行，环后画，rim 输**。
- 所以 P12-a 把 `LensRim` 从 0.04 抬到 0.09 之后，凡是 `Lens` 画的控件，顶内线仍然是
  `ControlEdge`（浅色 0.06 黑 / 深色 0.07 白）。P13-h 在启动器 28u 图标板上量到同一件事
  （浅色顶边 212 < 面 221，一条暗线）；这一轮是同一族在设置页上的十处。

## 3. 落地（10 处，全是 `tokens.Lens(x)` → `tokens.LitFace(x)`）

`rows.go` `extensionActionFace` / `rows.go` `extensionIconButton` hover /
`parameditor.go` `paramIconButton` hover / `settingsui.go` `headerButton` hover /
`settingsui.go` `barButton` / `uninstall.go` 动作胶囊 / `uninstall.go`
`uninstallPartRow` 的 fixed+hover+default 三臂 / `savealert.go` 的 ✕。

**几何零改动**：这十处都各自带 `Border(1, ControlEdge)`（两处悬停小格是 hover 时才加），
所以环与边框本来就是同一条边的第二次上色；退掉环之后边缘仍是「边框 + 内线」两像素，
变的是内线那一像素的墨（暗线 → 受光边），以及左右两列回到面。

两处**没有**动，并写进账：

- `control/control.go:123` 的选中段：它的身份线是紧跟其后追加的 `AccentEdge` 四边环，
  四边都在、把 rim 与 edge 全盖住；换 `LitFace` 画出的是立刻被遮住的两条线（像素上不可见），
  §2 也不许一个面说两句话。→ §6.2b 第 9-11 行 `故意不做`。
- `shell/notices.go:440` 的 toast 动作 chip：`Control` 面 + `Lens`，**没有自己的边框**
  ——环就是它唯一的边界，补 `Border` 会改几何。→ §6.6 第 6-3 行 `未开始`（下一轮）。

## 4. V / B / S 台账

| 类 | 条目 | 处置 |
|---|---|---|
| **V** | `settingsui/lens_test.go` `TestTheSettingsCardControlsCarryTheStopLens` 的卡断言 | 旧值「卡的涨幅 ≤ 按钮的涨幅 + 50」默认卡自己不吃透镜；P12-c 给卡戴的 E1 一对线**自己也吃** `--glass-lens-scale`，前提已倒（实测卡 27612 / 独立胶囊 25626）。新值：把卡自己的涨幅**量出来**（同卡同行、不带按钮），再与同宽同面的参照 pane（`Control` + `LitFace`）比，容差 500（涨幅的 2%）；另加「卡里的按钮不能比独立按钮涨得更多」。注释写明 旧值 → 新值。 |
| **V** | `internal/shell/glasscensus_test.go` 三条 | `.extensions-action-button` / `.settings-save-alert button` / `.extensions-icon-button:hover` 从「必须还有那个调用」翻成 `dropped`（第三条原本是 `pending`，理由是「本仓没有静息透明的扩展图标钮」——那个理由早过期了，`extensionIconButton` 从 P8 起就在配置工具栏里）。 |
| **S** | `settingsui/uninstallmetrics_test.go` `TestComponentizedUninstallCardDropsItsShadowOnPress` | 旧结构：宽 sheen 的顶重（读 +3 / −3 两行）；新结构：E1 只有 1px 一对线（读 +1 / −2 两行）。量的东西没变：静息有顶重、按下归零。 |
| **B** | 其余全部 | 零改动全绿；高度链、键盘模型、设置写回、i18n、命令接线一个没碰（本轮只替换十处材质调用）。 |

## 5. 门槛（全部实测）

```
gofmt -l cmd internal                       → 空
go vet ./...                                → 净
GOOS=windows go vet ./...                   → 净
GOOS=darwin  go vet ./...                   → 净
go test -count=1 ./...                      → 全绿
PASS 计数                                   → 2917（基线 2907，+10 = 3 条像素 pin × 深/浅 + 3 个父测试 + 1 条结构 pin）
go build ./...（linux / windows / darwin）  → 三平台过
```

## 6. mutation（5 条实跑，全部真红并逐字节还原）

| # | 注入 | 红在哪（原始报错节选） |
|---|---|---|
| M1 | `rows.go` 的胶囊换回 `tokens.Lens(button)` | `[light] the action pill: its left columns at mid height are {115} but its face is {120} — a ring is drawn around the block`（左右两侧同时红） |
| M2 | `theme.Tokens.LitFace` 删掉底对侧那一行 | 三条像素 pin 同时红：`the rows inside its bottom frame are {119}, not darker than its face {119} — E1 is a pair` |
| M3 | `rows.go` 的 30u 图标钮 hover 臂删掉 `LitFace` | `[light] … the rows inside its top frame are {113}, not brighter than its face {114} — the face lost its lit edge` |
| M4 | `savealert.go` 的 ✕ 换回 `tokens.Lens(button)` | `savealert.go still calls the four-layer lens on a control …`（结构 pin） |
| M5 | 胶囊加一条 `Shadow(0, 4, 12, 0, ShadowAmbient)` | `[light] the action pill casts a shadow beside itself ({122} against the ground {128}): E1 adds none` |

每条：`cp` 备份 → `md5sum` → 注入 → 跑测试看到红 → 还原 → `md5sum -c` **逐字节核对通过**
→ `touch` 后复跑一次绿（避免测试缓存复用）。

## 7. 图（视觉证据）

`FLOTER_PNG=1 go test -run TestDumpP13ILensFacePNGs ./internal/settingsui/` 写出
`/tmp/floter-P13-i-pill-after-{light,dark}.png`：同一张图上并排两枚同尺寸胶囊，
上「`Lens` · before」、下「`LitFace` · after」。

**在这张图上看到的一处变化**（dump 里逐行记录的实测值，浅色档）：上（`Lens`）的
内侧第一行 125、侧列 113；下（`LitFace`）的内侧第一行 **131**、侧列 **119（= 面）**。
也就是：受光边从「被环压住的 0.06 黑」变成 0.09 白的亮线，左右两侧回到面本身。
深色档同一对读数：内行 160 → **151**（环的白被 0.09 的 rim 替掉）、侧列 150 → **141（= 面）**。

## 8. 「偷了什么」（qiaomu-design / craft-loop 工艺密度菜单，逐条落成数值）

本轮走的是 `craft-loop.md` 的**打磨模式**（`SKILL.md` 的「已有方向，只要打磨」那条路径），
没有出四方向预览（用户已把方向钉死为轻便液态玻璃，自动车道没人能点选）。

| 取自 | 条目 | 落成的具体数值 |
|---|---|---|
| 工艺密度 #5「多层染色阴影」 | 材质分两层说 | E1 = **顶 1px `LensRim` 0.09 白 + 底 1px `LensEdge` 0.06 黑**，两层各说一件事（亮一分 / 暗一分），不是一片渐变 |
| 工艺密度 #12「材质细节：内边框高光」 | 受光边只在「面」上，且必须与框分层 | 受光边落在 **padding 第一行**（框内），框仍是 `ControlEdge`；两者不再同色 |
| 工艺密度 #6「编辑细节：hairline」 | 画框只留一条 | 十处控件的边缘从「`Border` + 环」两笔同色 → 只剩 `Border(1, ControlEdge)` 一条，内线改说「受光」 |
| `apple-fluid-interfaces.md` 的层级观（借其判据，不借其动效） | 同屏只有一个浮层 | 本轮**没有**新增任何 E2：十处控件全部是 E1，`ElevThrow` 一次没动 |
| `preflight.md` A（反 AI 套路） | 不用渐变白边模拟高光 | 受光边是**一条 1px 实色线**，没有 gradient、没有模糊贴图 |

## 9. preflight 逐条（A / B / C / D2）

- **A 反 AI 套路**：过。没有渐变白边、没有玻璃叠玻璃（本轮零 E2）、没有装饰性 glow；
  唯一新增的视觉是「一条亮线换掉一条暗线」。
- **B 设计读取一致性**：过。读取为「桌面效率工具的设置页，面向每天开合十几次的用户，
  用轻便液态玻璃语言，倾向 Raycast/Linear 那一族」。所有取值来自 `theme.Tokens`
  （单一来源），没有一处字面色值；`docs/liquid-glass.md` §4 的度量一个字没改。
- **C 动效抽查**：过（本轮没有动效改动）。十处 `Transition` 原样保留（120/150ms 只动颜色），
  没有新增 `transition: all`、没有 `scale(0)` 入场；`ReduceMotion` 在两套 pin 里都开着跑。
- **D2 中文页**：过（本轮零文案改动）。i18n 一个字符串没碰，也没有新增中文界面文本。

## 10. 仍待

1. `docs/liquid-glass.md` §6.6 第 6-3 行（toast 的动作 chip）：退成 `LitFace` 的同时要决定
   它唯一的边界怎么办——补 `Border(1, ControlEdge)` 会把面缩小 1px（`insetShadow` 从
   padding edge 算起），这是几何改动，需要单独一轮。
2. §6.2b 第 9-11 行（`control.SegmentedIndex` 选中段）判为 `故意不做`，理由写在表里；
   若以后要给它一条受光边，必须先决定 accent 环与白冠线谁说话。
3. 终端弧仍在等拍板：S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾
   （客户端不告而别）。**没有动。**
