# P11-bj · 搜索页：字段带是面板自己的第一条带；空查询不再列内置指令

选题：`.pending-bugs.md` 顶部「⚠️ 用户实测反馈（2026-10-28）」的 **F4**（最新一轮反馈，
压过其它待办）。F4 的三条里第 3 条（按住 Super 的「进入终端」固定底部、不重排、不重复）
已由 **P11-bb** 落地（`heldTerminalItem` + 窗口为它长一行，不进编号、不进排序），
本轮做剩下的两条：**F4-1 输入框的「浮卡」观感**与 **F4-2 空查询时的建议行为**。

## 意图

搜索页的输入框现在是一块**自己的面**：`launcher.go:1412` 给字段行
`Background(tk.FieldFill)`（浅 `rgba(250,250,251,.997)` / 深 `rgba(28,28,31,.99)`），
而它下面的面板是 `tk.Sunken`（`launcher.go:1291`）。字段行的下沿还有一条
`HairlineSoft`（`launcher.go:1600`）。于是 y=0..55 是一块近白/近黑的实心块，
y=56 起是另一块面 —— 一块**自带下边框的卡片贴在另一块面上**，正是用户说的
「像一块浮在上面的卡片」「不连续边框」。实测（`720×488`，x=4 与 x=708 同值）：
浅 字段 `(250,250,251)` / 发丝 `(236,236,237)` / 面板 `(253,253,254)`；
深 字段 `(28,28,31)` / 发丝 `(40,40,43)` / 面板 `(23,23,25)`。
两套外观下字段面与面板面都是**两个不同的面**。

空查询那一条是**与参考实现相反**的行为：参考 `useLauncherCatalog.ts:1138-1166`
的空查询分支只返回 `recentItems(...)`，一句 `SYSTEM_COMMANDS` 都不加（内置指令只在
**有查询时的打分**里出现，`:1201` 起）。本仓 `search.go:870` 在空查询里先
`a.commands()` 六行再来最近启动 —— 用户看到的「一堆内置指令」。

## 逐条落地项

### F4-1 字段带是面板的第一条带（不是浮卡）

- 位置 `internal/launcher/launcher.go:1407-1413`（字段行）。
- 现状 `Background(tk.FieldFill)`：在**已经**是 `tk.Sunken` 的面板之上再压一层自己的面，
  形成一块独立的实心带。
- 应改成 **不画面**：字段行不再有 `Background`，那条带就是面板自己的面
  （一层 `Sunken`，不是两层）。`Padding(0, Space(4))`（左右 16u）与 `Height(fieldRow)`
  不动 —— 56u 是 R37 的约定（与设置页头同高），高度链一个字节不改。
- 反向必须一起做：字段带原来是**不透明的遮罩**，挡住从下面滚上去的行。去掉面之后
  行会从查询文字底下透出来（R21 那笔账：光标下不许有一行鬼影）。所以
  `launcher.go:1385` 的列表不再用 `Padding(listEdge, …)` 把内容顶下去，
  改成在它前面放一条 `listEdge` 高的**流内占位**（`Spacer`），列表的**盒子**从
  `listEdge` 起，滚动区因此从带子下沿开始 —— 行根本不进那条带。
  `a.listRect` 相应改成直接读 `list.Bounds()`（原来手写的 `Y += listEdge; H -= listEdge`
  正是为了扣掉被盖住的部分，盒子自己已经在下面了）。
- 类型：**V**（字段带的像素）+ **B**（行不进带子、编号读的视口不变）。

### F4-2 空查询只给最近启动

- 位置 `internal/launcher/search.go:864-877`（`otherResults` 的空查询分支）。
- 现状 `out = append(out, a.commands()...)` → 六个内置指令（重启/关机/剪贴板/浏览器/
  计算器/终端）排在最近启动**前面**。
- 应改成 **不追加**：空查询只剩 `calculator` 的答案（有算式时）、`RecentSection`
  标题与最近启动。内置指令**仍然可达**：`Catalog()` 把它们和应用程序放在一起
  （`search.go:696`），输入任何字符就进 `matchedCatalog()` 的打分，`restart` 照样
  排第一。
- 「收藏夹优先序」：搜索页没有应用级收藏（收藏只存在于剪贴板/计算器两个模式里，
  `filters.go:92-116`），空页的顺序就是 `Recent` 的既有名次（启动次数降序，同分按扫描序，
  `recentItems` 注释）。本轮**不新造**一个应用收藏功能（那是新功能，不是修 bug）。
- 类型：**B**（列表内容）+ **V**（空页的行集变了，钉旧行集的断言重定基线）。

## 零改动红线

- 高度链：`fieldRow/breath/chips/TopInset/Tail` 一个数不动，`Geometry.Height()` 不动。
- 键盘模型：`Super+Enter` 的进入终端（P11-bb 的底部常驻行）、`Super+N` 编号（P11-bi
  的 70% 视口判据）、`Enter` 的 `defaultSelection`、Esc/Dismiss 全不动。
- 会话管理、设置写回、i18n 文案、命令接线：零改动。

## 门槛与基线（本轮开工实测）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 全绿
PASS 计数                                  # 2444（P11-bi 收尾实测）
测试函数数                                 # 1991
```

## mutation 计划

- **M1**：把 `otherResults` 的 `out = append(out, a.commands()...)` 加回去 →
  `TestTheEmptyPageOffersRecentsAndNoBuiltIns` 必须红（空页又出现 `Restart the computer`）。
- **M2**：把字段行的 `Background(tk.FieldFill)` 加回去 →
  `TestTheFieldBandIsThePanelsOwnSurface` 必须红（带子与面板的像素不同）。
- **M3**：把列表的占位 `Spacer` 换回 `Padding(listEdge, …)` →
  `TestNoRowScrollsUnderTheFieldBand` 必须红（滚到某个偏移后带子里出现字）。
