# P13-g 报告 · §6.6 弹层：toast 进 `ElevThrow` 那一档，折叠带子补上冠线与接触影

## 1. 这轮为什么让它更轻、更有空间感（一句话）

它原来顶着一层 `0 40px 104px`、alpha 0.62 的黑雾（几何写成了 `t.Space(10)`/`t.Space(26)`，
单位被乘了两次 4），把整页都染暗；现在是 `0 10px 26px` + `0 1px 2px`、alpha 0.30/0.20 ×
`ElevScale`，**近处更深、远处消失**——同一块纸，从「压着一团黑」变成「浮着一条边」。

## 2. 落地项

| # | 位置 | 现状（改前） | 落成（改后） |
|---|---|---|---|
| G1 | `internal/theme/elevation.go:51` | 只有 `ElevThrow`（10u/26u + 1u/2u） | 新增 `ElevThrowCompact`：同 alpha × `ElevScale`，几何 6u/18u + 1u/2u（`--elev-3-compact`） |
| G2 | `internal/shell/notices.go:381-391` 满高臂 | `Shadow(0, Space(10)=40, Space(26)=104)` + `Shadow(0,1,2)`，alpha 取 `ShadowAmbient` 0.62 / `ShadowContact` 0.44 | `theme.ElevThrow(p.Root, t, stored.MainOpacity)`：10px/26px + 1px/2px，alpha 0.30/0.20 × scale |
| G3 | 同处 `collapsed` 臂 | 只有一笔 `Shadow(0, Space(1.5)=6, Space(4.5)=18)`，alpha 0.62，**无接触影、无冠线** | `theme.ElevThrowCompact(...)`（两层）+ `InsetShadow(0,1,0,0, tokens.EdgeHighlight)` |
| G4 | `internal/shell/toastthrow_test.go`（新） | 只有一条「有影」的 pin，只判非零 | 三条 pin：影随框透明度反向缩放 / 两条臂的冠线都是 E2 的线（深+浅） / 折叠带子的接触层 |

**单位 bug 的算术**：`ui.Theme.Space(n)` 返回 `n × Spacing`（默认 4）。改前那两笔写的是
`t.Space(10)`、`t.Space(26)`，即 40px 偏移、104px 模糊——参考的 `--elev-3` 是 `0 10px 26px`。
同一行里的 `t.Space(0.25)`/`t.Space(0.5)`（=1px/2px）是对的，所以这处错一直没有被肉眼归因到单位上。

## 3. V / B / S 台账

- **V 0**：没有一条既有视觉数值被重定基线。`TestTheToastFloatsOnTheElevationLadder` 与
  `internal/ci/elevation_contract_test.go` **测试零改动**，仍然全绿（前者只要求「有影且更暗」，
  不钉几何；后者扫的三个包里没有 `shell`，本轮也没有把 `shell` 加进去——`shell.go:4392`
  那两笔窗口影是 E0，本来就该用 token 而不是 `ElevThrow`，加进去只会逼出一条例外）。
- **B 0**：没有行为/接线改动。`toastViewport` 的几何、锚点、排序、入场弹簧、
  dismiss/action 接线、i18n 文案、`PassThrough` 全部一字未动。
- **S 0**：结构未变，只多了一个导出函数。

## 4. 门槛（实测，全跑）

```
gofmt -l cmd internal                       → 空
go vet ./...                                → 净
GOOS=windows go vet ./...                   → 净
GOOS=darwin  go vet ./...                   → 净
go test -count=1 ./...                      → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS" → 2898   （基线 2891，+7 = 3 条新 pin + 4 条子用例）
--- SKIP 计数                                → 65       （基线 64，+1 = 本轮 opt-in dump）
--- FAIL 计数                                → 0
测试函数数（grep -h "^func Test"）            → 2144     （基线 2140，+4 = 3 pin + 1 dump）
GOOS=linux/windows/darwin go build ./...     → 三平台过
```

## 5. 变异（实跑，逐条 `md5sum -c` 逐字节还原 + `touch` 复跑绿）

| # | 注入 | 变红的 pin 与原始报错串 |
|---|---|---|
| M1 | `notices.go` 满高臂换回 `Shadow(0, t.Space(10), t.Space(26), 0, tokens.ShadowAmbient)` | `--- FAIL: TestTheToastThrowFollowsTheFrameTransparency`；`deepest relative darkening: 100% = 0.049, 10% = 0.043` / `the throw does not follow the frame: 0.049 at 100%, 0.043 at 10%` |
| M2 | `ElevScale` 改成常数 `1` | 同上；`100% = 0.337, 10% = 0.333` / `the throw does not follow the frame` |
| M3 | 两条臂的 `InsetShadow(... EdgeHighlight)` 删掉 | `--- FAIL: TestBothToastArmsWearTheFloatingRim/full` 与 `/collapsed`；`full: rim 193114 vs face 193114 at x=682 top=56` / `the pane's top row (193114) is not brighter than its own face (193114)`；`collapsed: rim 202228 vs face 202228`（改前这两条是 250000 vs 193114、251000 vs 202228） |
| M4 | `ElevThrowCompact` 里删掉接触影那一笔 | `--- FAIL: TestTheLauncherToastKeepsItsContactLayer`；`alpha added under the strip: +1 row = 40, +5 rows = 32` / `the strip has no contact layer` |

## 6. 出图（深/浅两套，前后各一）

`/tmp/floter-P13-g-settings-{dark,light}-{before,after}.png`（满高：右上）
`/tmp/floter-P13-g-launcher-{dark,light}-{before,after}.png`（折叠：左下 20u 带子）
同一 fixture、同一尺寸 720×580；before = 生产改动前的 HEAD 代码 + 本轮新增的 dump。

**这张图上看到的一处变化（浅色 settings）**：toast 下面那一片**整页都亮了一点**——
改前那笔 104px 模糊的影把内容页 y=0..245 都染过（84267 px 有差异，x 从 284 一直到 719），
改后同样的像素只是各自回到页面自己的亮度；量成数字是 y=160..240、x=320..700 这条
「影根本够不到」的带子，平均 alpha 深色 161.0 → 157.1、浅色 154.2 → 152.8。
折叠那张的变化只落在 toast 自己身上（6043 px，bbox (0,542)-(294,579)），因为那条带子
离窗口底边只有 6px，宽影本来就出不去。

## 7. 偷了什么（qiaomu-design，走「已有方向，只要打磨」那条车道）

读了 `SKILL.md` → `references/user-preferences.md`（P-1 禁斜体 / P-2 先看后选 / P-5 可读性优先，
本轮不涉及排版与方向选择）→ `references/craft-loop.md` + `references/preflight.md`；
DNA 供体取自 `references/design-systems/linear.app/DESIGN.md`（§6 Depth & Elevation）：

1. **craft-loop 工艺密度菜单第 5 项「多层染色阴影」** → 落成 E2 的两层：
   环境 `0 10px 26px` alpha `0.30 × ElevScale`、接触 `0 1px 2px` alpha `0.20 × ElevScale`
   （`internal/theme/elevation.go:44-80`）。Linear 的原话是「multi-layered shadows」，
   具体到本仓就是两层影、两层各自乘 `--elev-shadow-scale`。
2. **菜单第 12 项「材质细节：内边框高光」** → 落成折叠带子的 `InsetShadow(0,1,0,0, EdgeHighlight)`
   （深 0.20 / 浅 0.92，一条 1px 实色线，不是渐变——§0 的硬禁令 3）。
3. **菜单第 6 项「编辑细节：hairline」** → 受光边与接触边都锁在 1px：`Space(0.25)`=1px、
   `Space(0.5)`=2px 模糊，改前是 4px/8px（同一个单位 bug 的另一面）。
4. **从 Linear 偷的第 4 条（不是菜单项，是它的判据）**：「don't use drop shadows for elevation
   on dark surfaces」——深色下靠受光边与面阶说话，影只留薄薄一层。本轮的 0.62 → 0.278
   正是这条：影不再是层级的主语。
5. **没偷的两项（如实说明）**：菜单第 1 项「氛围层」（`body::before` 噪点/雾）与第 9 项
   「自定义滚动条」在本仓没有对应物——自绘 Go 没有 DOM，也没有滚动条 CSS；硬造等于给
   §0 的「不许假装有 blur」开一个后门。菜单第 2/3 项（`::selection`、focus 环）同理不适用。

## 8. preflight 逐条结论

- **A 反 AI 套路**：过。无渐变白边（受光边是 1px 实色）、无外发光、无玻璃叠玻璃
  （toast 内部一切是 E1，`p.Root` 只有一层 Float 面）、无 italic。
- **A2 进攻三步**：DNA 注入 ✓（Linear 四条，见 §7）；工艺密度 3 项落地 + 2 项写明不适用；
  两轮制：第一轮改完 → 用 before/after 四张图做「艺术总监/工程师」两视角复核
  （发现：settings 那张的前后差异远大于 toast 自身 → 顺藤摸到 `Space()` 的单位 bug，
  第二轮只修这一处，未推倒重做）。
- **B 设计读取一致性**：过。本轮的设计读取是「桌面命令面板的浮层，深/浅双外观，
  轻玻璃气质，Linear 那一支的层级算术」；产出与之一致：三档 elevation 里只有 E2 被碰。
- **C 动效抽查**：过（且未改）。toast 入场仍是 `--dur-4` 220ms 弹簧、只动
  `opacity/transform`，`ReduceMotion` 兜底在 `toastEntrances` 里；本轮没有新增任何动画。
- **D2 中文页**：不适用（本轮没有中文文案进出，i18n 字符串零改动）。
- **E 三秒测试**：过。眯眼看两张 after 图，视线第一落点是 toast 的受光边，不是一团黑影。

## 9. 仍待

1. **§6 差距表在本轮之后没有 `未开始` 行**——视觉这一支（表面覆盖）收口。下一轮转回
   任务书的「细节打磨通用队列」：间距节奏 / 状态反馈 / 空态满态 / 文案密度 / 边界情况。
   队列里现成的第一条是 P13-f「仍待」第 3 条：`iconPlate` 的四层 `Lens`（第四层是四边
   `ControlEdge` 的环）是启动器所有页共用的板，收成 `LitFace` 是跨表面的一轮。
2. **P13-c 的四条原样有效**：S1b（`CloseAll` 改 detach，等拍板）、S1c（Windows ConPTY）、
   客户端不告而别的检测、capture 通道迁移。
3. **本轮观察（新）**：`t.Space()` 被当成 px 用的写法在仓里可能不止 toast 一处
   （`t.Space(10)`/`t.Space(26)` 这种两位数是明显的信号）。本轮只修了 toast 这一处，
   没有扫全仓——扫描属于「一族同源 bug」，值得单独一轮，已写进 `.pending-bugs.md` 的「仍待」。
