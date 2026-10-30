# P13-d 规格：终端页表面（`docs/liquid-glass.md` §6.3）——E1 的受光边与 E2 的两层影落到这一页

**轮号**：P13-d（视觉队列第 3 片；§6.1 启动器 = P12-b、§6.2 设置页 = P12-c 已落地）
**表面**：终端页（`internal/terminalui`）**一个表面，一轮**。
**为什么现在做**：`docs/liquid-glass.md` §6 是唯一权威选题队列，§6.3 全行 `未开始`。
P13-a/b/c 那一弧（终端会话宿主）剩下的 S1b **改会话管理语义 = 需用户拍板**
（`.pending-bugs.md` 的「需用户拍板」P13-a 第 1 条），所以本轮回到排在弧之后的视觉队列。

**这轮为什么让它更轻 / 更有空间感**（每个视觉轮必答的一句话）：
终端页现在只有两档在场——**底**（面板 + 画布，两者都是 E0）与**一条四面环的透镜**
（悬停的图标按钮）。中间那档**面**（E1）在这页上一次都没画过，所以画布、抽屉、
键帽、浮条全挤在同一个 z 平面上；而唯一像「浮」的那件东西（悬停的标题条）
又只压了一道紧贴边的重影，读起来是「底下加深一条」而不是「浮起来一张纸」。
本轮把 E1 的受光边接到抽屉的缝与键帽上、把 E2 的两层影接到浮条上、
并把图标按钮那个四层环退成一条受光边。

---

## 落地项（位置 → 现状 → 应改成）

行号是**本轮实测**（`grep -n` 在动手前重核过），不是旧报告里的漂移值。

### 3-1 面板（`terminalui.go:870`）——保持 E0

- 现状：`ui.Box(c).Fill().Radius(panelCorner(tokens)).Background(panelFill(...))`，
  只有一层面，没有 rim/sheen/lens（P8-c 复核过）；平台自己的 `panelEdge`
  （`shell/cardedge.go:148`）是它的框。
- 应改成：**不动**。`panelCorner` = `WindowRadius`(12)、`mountCorner` = `RadiusMD`(10)
  两条几何 pin 继续钉住（`raycast_test.go:263` / `:271`）。
- 状态：`已落地`（P11-z 的 `TestTheTerminalCanvasDrawsNoSecondFrame` 已在守着）。

### 3-2 会话挂载块（画布，`terminalui.go:939-946`）——**故意不做**（改判）

- 现状：`ui.Box(c).Fill().Radius(mountCorner(tokens))`，只有一层填充（P11-z 删掉了
  那圈 `Border(1, edge)`）。
- §6.3 原行写的是「E1：`LensRim` 顶边」。**本轮改判为 `故意不做`**，理由两条：
  1. 画布**是**这一页的**底**（终端的内容面，程序写什么就在它上面写什么），
     不是浮在底上的一个块；§2 的判据「它长在底上，压不出影子」说的是**块**，
     而画布与面板之间不是「块放在底上」，是「同一张面换个色号还没换」。
  2. P11-z 刚按用户实测 F3 退掉的就是这一页的**第二层框**（`Border(1, edge)`，
     聚焦时还是 accent 红）。给画布补一条受光边，是在 F3 的同一处加回第二层分界，
     只是换了颜色——这正是用户读到的「类玻璃边」。
- 证据：`docs/raycast-design.md` F3 条目 + `internal/terminalui/redframe_test.go`。

### 3-3 外观抽屉的缝（`drawer.go:118-121`）——`HairlineSoft` → `LensRim`

- 现状：`Background(tokens.Control)` + `BorderWidth(1, 0, 0, 0)`（上边一条）+
  `BorderColor(tokens.HairlineSoft)`（0.055）。抽屉与画布之间是一条**分区线**。
- 应改成：`BorderColor(tokens.LensRim)`（0.09，§3 的唯一受光边）。
  抽屉是**长在面板里的一个块**（它是画布的兄弟、占布局高度、把画布压小，
  不压在内容上）→ §2 判据给的是 **E1**，E1 的边就是 `LensRim`。
  **底对侧不补**：抽屉的下边就是窗口的底边，P11-av 已经证明那条线会落进
  窗口最底一行、压进 12u 圆角的过渡里（当时修的就是这个 bug）。
- 为什么不是 E2 的 `EdgeHighlight`（§6.3 原行写的）：E1 与 E2 的边在设置页
  已经是两件事（§6.2 2-7/2-8 的教训——浮起来的纸与贴着的块若同一条边，
  空间感就没有算术来源）。抽屉不满足 §2 判据 3（它不压在其他内容上），
  所以它拿 E1 的线。

### 3-4 标题条 hover 影（`terminalui.go:1477-1482`）——一条重影 → 两层影

- 现状：`bar.Shadow(0, t.Space(1.5), t.Space(4), 0, tokens.ShadowContact)`。
  几何是参考的宽影（`--elev-bar` 的 `0 6px 16px`，`terminal.css:99-104`），
  **墨**却是 §3 的紧贴边那一支（深 0.44 / 浅 0.12）。
- 应改成：宽影读 `ShadowAmbient`，再补一条紧贴边 `0 1px 2px` 读 `ShadowContact`
  （§2 判据 3：E2 必须两层影）。
  旧值 → 新值：`0 6px 16px ShadowContact` → `0 6px 16px ShadowAmbient`
  **+** `0 1px 2px ShadowContact`。
- 两条 pin 影响：`barshadow_test.go` 的 `TestTheTerminalBarLiftsItsShadowOnHover`
  三条断言（悬停那一行更深 / 静息不画 / row+30 无差）本轮**逐条复跑**，
  见报告的 V 台账。

### 3-5 / 3-6 —— 保持 `故意不做`（贴边带子不是浮层、1.75u 的点画不出受光边）

### 新增 3-7 图标按钮的悬停面（`terminalui.go:1410-1421`）——`Lens` → `LitFace`

- 现状：`button.Hovered()` 时 `tokensOf(a, c).Lens(button)` —— 四层（rim +
  宽 sheen + 底 shade + **四边 `ControlEdge` 环**）。注释还写着
  「The lens is the window's rim at ≤0.04 alpha, below the §1 gloss cap」——那句话
  **已经过期**：P12-a 把 `LensRim` 重定到 0.09（`docs/liquid-glass.md` §1 末尾的
  旧值→新值表）。
- 应改成：`tokensOf(a, c).LitFace(button)`（顶受光边 + 底对侧暗线，无环）。
  悬停的 28u 图标按钮是 E1 的一个小面；§2 要退掉的「画框分层」正是那圈环。

---

## 行为契约零改动红线（本轮的自我约束）

- **不动**：高度链（`barHeight` = `Space(7)`、画布地板、抽屉滚动）、键盘模型
  （`DISMISS_TABLE.terminal.escape` 仍为空）、会话管理（`CloseAll` 语义不动，
  S1b 仍等拍板）、设置写回（`set`/`preview` 两条路径）、i18n 文案、
  命令接线（`SessionCommands` 表）。
- **只改墨与面**：三条 `BorderColor`/`Shadow`/`InsetShadow` 的颜色与层数，
  几何（半径、padding、行高、按钮尺寸、缝的位置）一个字节不动。

## 门槛命令 + PASS 基线（开工实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 必须净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"        # 只许涨
三平台 go build ./...
```

开工基线（P13-c 实测）：**PASS 2869 / FAIL 0 / SKIP 61 / 测试函数 2129**。
本轮新增 pin 4 条 + 1 个 opt-in dump，预期 PASS ≥ 2873。

## mutation 计划（每条说明会让哪个 pin 变红）

1. **M1**：`terminalui.go` 的 hover 影写回单条 `ShadowContact`（宽几何）→
   `TestTheHoveredBarLiftsWithTwoThrows` 变红（悬停帧比「两层」参照帧更浅）。
2. **M2**：`drawer.go` 的缝写回 `HairlineSoft` → `TestTheDockedDrawerWearsTheE1LitEdge`
   变红（量到的线离「应该」那条更远）。
3. **M3**：`barButton` 写回 `Lens` → `TestTheHoveredBarButtonIsAFaceNotARing` 变红
   （按钮左右两侧出现环）。
4. **M4**：`emptyLine` 的键帽删掉底对侧 → `TestTheEmptyKeycapWearsTheE1Pair` 变红。

每条：注入 → 看到红并抄原始报错 → `edit` 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑。

## 出图

`/tmp/floter-P13-d-terminal-dark,light}-before,after}.png`。before = 把三个
生产文件的改动 `git stash` 掉、只留新增的 dump 测试（未跟踪，不受 stash 影响）跑一次；
after = 还原后再跑一次。图自己用眼睛复核，报告写「这张图上看到的一处变化」。
