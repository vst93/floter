# P11-bg 报告：终端的失败提示也挂成画布顶边的一条带

规格 `docs/rounds/P11-bg-spec.md`。合约 `docs/raycast-design.md` §6.19。

## 1. 选题与根因

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完，`Super+N` 编号已由 P11-bb
收完）→ 合约未落地条目（§6 全表 `已落地`，只剩 `⌘K`＝拍板）→ 上一轮「仍待」
（P11-bf 第 1 条全是老拍板项，第 2 条 `configDiscardBar` 的 12u margin 是设计决定，
第 3/4 条是侦察结论）→ **界面质感与交互逻辑**（用户 2026-10-28 定的优先档）。
P11-ba 的报告把这一条列成「界面质感这一档剩下的新候选」：*终端退出便条 / 失败提示
要不要改 inline*；P11-bc 做掉了前者（`residentNote`），并写明后者「故意没动」。本轮做它。

**根因（一句话）**：同一页有两种说法——底边的常驻便条（P11-bc）是贴边带，顶边这条
4.5 秒的失败提示还是**浮卡**：`internal/terminalui/terminalui.go` 的 `feedbackNote`
用 `Absolute().Top(Space(3)).Left(Space(3))`（离面板左上各 12u、自然宽）、
`Radius(RadiusMD)`、`Border(1, ControlEdge)`、`Background(Float)`、
`Transition{Enter: Y-2, 160ms}`，外加 `theme.ElevThrow` 投影与 `LensRim` 受光边。
用户 F3 的判据只看「它是不是一块压在内容上的浮面」，不看它是常驻还是转瞬。

## 2. 改了什么

`internal/terminalui/terminalui.go`：

- `feedbackNote` 的元素改成 `Absolute().Top(barHeight(t)).Left(0).Right(0)` +
  `Background(tokens.Control)` + `BorderWidth(0, 0, 1, 0).BorderColor(tokens.HairlineSoft)`
  ——挂在标题条下沿、整宽贴边，无圆角、无描边环、无投影、无受光边、无入场动画。
  底边那条带是**上边**一条发丝，这条是**底边**一条：两条带各自朝画布内侧收口。
- 新增 `barHeight(t *ui.Theme) float32 { return t.Space(7) }`：同一个「标题条高度」
  以前在 `titleBar` 的 `Height`、`body` 的 `MinHeight` 与这条带的 `Top` 三处各写一次。
- `theme` 的 `ElevThrow`/`InsetShadow` 调用整段删除。

**没动的**：15u 警示字形、body 步 560、`t.Warning` 墨（用户 F2 的验收要求错误提示
仍然看得见）、7u/10u 内边距、`Gap(Space(2))`、`terminalFeedbackDuration` 的 4.5s 时钟
与 `feedbackShown` 去重、`c.AnimationFrame()`、外层与两个字层各自的 `PassThrough()`
（参考 `terminal.css:414` 的 `pointer-events: none`）。
**画布高度与 PTY 行数零改动**：这条带是 `Absolute()`，不占布局高度（与 P11-bc 同一条理由）。

## 3. 台账

- **V=0 / B=0 / S=0**：既有测试**零改动**全绿。P11-bc 的源码契约
  （`residentstrip_test.go` 的四件套 + `PassThrough`）没有一条断言过这条提示的几何，
  所以没有重定基线。
- 新增 3 个测试函数（2 条 pin + 1 条 opt-in dump）：**1984 → 1987**。
- PASS **2440 → 2442**（+2；dump 是 opt-in 跳过）。FAIL 0。

## 4. 变异（3 条实跑变红 + `md5sum -c` 还原 + `touch` 复跑绿）

| # | 注入 | 原始报错串 |
|---|------|-----------|
| M1 | `Background(tokens.Control)` → `Background(tokens.Float)` | `feedbackNote does not wear the drawer's own band face`（`TestTheFailureNoteLetsThePointerThrough`）——浮面档与带子档分得开，这条 pin 认的是那一档 |
| M2 | 把 `theme.ElevThrow(notice, ...)` 加回去 | `light 860x600: the band's first row runs [255 255 255] at x=0, not its face [254 254 254] — it is inset or rounded like a floating card` + `the row under the band's hairline is [242 242 242] but the canvas is [255 255 255] — something still lies between the band and the output` |
| M3 | 把两个字层的 `.PassThrough()` 删掉 | `feedbackNote marks 2 of its 3 layers PassThrough: the rest still take the pointer`（P11-bd 的 `TestTheCanvasOverlaysGoQuietWhenHidden`） |

每条都 `md5sum -c /tmp/p11bg.md5`（「成功」）逐字节核对，`touch internal/terminalui/feedbackstrip_test.go`
再跑一次绿。（M3 的第一次注入只删了图标那一层、Perl 的跨行模式没匹配到，pin 因此没红；
改成「两层一起删」后红在层数断言上——如实记在这里。）

## 5. 视觉证据（我自己量了像素）

`/tmp/floter-P11-bg-feedback-{light,dark}.png`（修后）与
`/tmp/floter-P11-bd-notes-{light,dark}.png`（**修前**，同一个夹具：`ResidentAt(0,3)` +
`ShowFeedback("no emulator")`，由隔离 worktree `/tmp/p11bg-before`（= HEAD）跑 P11-bd 的
dump 得到）。

**我在图上量到的**（`/tmp/p11bgdiff` 逐像素比）：

- 整帧差异 **40 318 px（light）/ 37 328 px（dark）**，包围盒 **y=0…90（light）/ y=2…79（dark）**
  ——只有这条提示自己那块变了；底边那条常驻便条（P11-bc 的形状）一个像素没动。
- 修前的那块面**有影子**：`x=2` 的同一列从上往下 `(251,251,251) → (248,248,248) → (244,244,244)`
  （y=14/20/30，浅色），到 y=40 是 `(241,241,241)`——这就是 `ElevThrow` 铺在输出上的
  那一圈。修后同样的列是 `(255,255,255)`（y=14/20，正是画布自己的白）→ `(242,242,242)`
  （带子那一档）。
- 修前它只占左边一条：`(429,30)`、`(429,56)`、`(429,60)` 修前都是 `(255,255,255)`
  （画布），修后都是 `(242,242,242)`——**整宽贴边**。深色同位置：`(24,24,27) → (41,41,44)`。
- 带子的面：浅 `(242,242,242)`、深 `(41,41,44)`，正是 `Control`（与 P11-bc 的底边带、
  抽屉同一档），画布是 `(255,255,255)` / `(24,24,27)`，两档分得开。

**一处我特意看的**：修前 `x=12,y=40` 是 `(238,238,239)`（浮卡的面，离左边缘 12u 才有），
修后是 `(242,242,242)`（从 x=0 就是带面）——浮卡那圈「12u 见方的圆角面」在像素上已不存在。

## 6. 门槛（实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # FAIL 0
PASS 2442（基线 2440）| 测试函数 1987（基线 1984）
GOOS=linux/windows/darwin go build ./...   # 过
```

## 7. 一次操作事故（如实记录）

本轮第一次想用 `git stash push <两个路径>` 造修前的帧，其中一个是**未跟踪**的新测试文件，
`git stash push` 因此整体失败（「未匹配任何 Git 已知文件」），紧随其后的 `git stash pop`
于是弹出了**栈顶那条老 stash**（`P7-lg keycap` WIP），在 `internal/launcher/launcher.go`
与 `internal/theme/theme.go` 上留下冲突标记。处理：`git checkout -f HEAD --` 这两个文件、
`git reset` 清索引，工作树回到「只有本轮两个文件」的状态（`git status --porcelain` 三条），
**`stash@{0}` 原样保留未被丢弃**（`git stash list` 首条仍是它，与本轮无关、也不要再合）。
之后的修前帧改用**隔离 worktree**（`git worktree add /tmp/p11bg-before HEAD`）产出，
不再碰 stash。

## 8. 仍待（P11-bg）

1. 老拍板项原样有效：搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 的便条投影 / P11-bd 的「未揭示时窗口拖拽手柄接不接指针」/
   P11-be 的 `Permissions` 行「标签与胶囊同一个词」/ P11-bf 的 `configDiscardBar` 12u margin。
   （P11-ba 记的「终端『进程已退出』便条改 inline」已由 P11-bc 落地，本轮把这条从
   `仍待` 里划掉；「失败提示要不要 inline」由本轮落地。）
2. **新观察（未改，属设计决定）**：这条带覆盖画布顶部约两行输出 4.5 秒。`PassThrough`
   保证那两行仍可点、可拖选、可滚轮，但**它们在带子底下是不动的**。要不要改成把画布
   整体下移一行（＝改 PTY 行数）是设计决定，本轮按 P11-bc 同一条理由不做。
3. **侦察结论**：终端页上的 `Absolute()` 浮层现在只剩 `titleBar`（`:1368`，它自己的
   `PassThrough` 与逐层标记由 P11-af/bd 守着）与这条带；`residentNote` 在底边、
   `feedbackNote` 在顶边，两者都是带子形状。
