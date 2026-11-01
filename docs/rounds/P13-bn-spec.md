# P13-bn 规格 · 权限审计行的尾部：一枚警示环陪着一条「在/不在」

**轮号**：P13-bn（P13-bm 之后）。
**表面**：设置页 → 集成页 → 展开的权限审计（`permissionTier`）。
**选题来源**：`docs/liquid-glass.md` §6 已无 `未开始` 行 → 通用队列「状态反馈」的另一半
（禁用 / 只读面）之后的**状态标记**这一格；上一轮（P13-bm）「仍待 1」点名了路径截断（需拍板，
不擅自动），所以本轮取**不需要拍板**的一格。

## 一、意图（一句话）

权限审计行尾那格今天画**两枚**标记：一枚 `alert` 警示环 + 一枚 `●`/`○` 圆盘，两者都染同一个
墨、同一个 `enforced` 状态——**一枚警示环在说「这里有问题」，而这一行说的是「已强制 / 仅声明」**。
这轮把那一格收成**一枚**标记。

## 二、为什么现在做（证据，行号已现量）

`internal/settingsui/settingsui.go:1269-1299` 的 `permissionTier`：

```go
a.row(c, label, description, func() {
        mark := "\u25cb"                    // :1284
        if enforced { mark = "\u25cf" }
        color := t.TextMuted                // :1289
        if enforced { color = t.Accent }
        ui.Icon(c, mustGlyph("alert")).Size(t.Space(4), t.Space(4)).TextColor(color)  // :1292
        ui.Text(c, mark).FontSize(t.FontSize-3).TextColor(color)                      // :1293
})
```

`:1292` 是 P10 时代留下的：`git log -S` 追到 `1dc788f4`「the permission audit is two named
tiers」的**同一次提交**——那次提交把 `permissions` 从「一 id 一行 + 一段解释」改成「两个具名
层级 + 一 id 一行」，同时给每一行加了这枚警示环。它在审计行里**没有语义**：`alert` 是
`triangle-alert` 之外的**错误**记号（`internal/shell/notices.go:294-299` 的 `toastMark`
把 `alert` 当 `error` 用），而审计行说的是「这条权限属于哪一层」。

实测（`ui.Tester`，520u 宽的卡，深浅两套，`slotRuns` 以面为基准扫行尾的墨段）：

| 外观 | 状态 | 行尾墨段（x 坐标，间隙即空格） |
|---|---|---|
| 浅 | enforced | `493..506`(14) + `514..521`(8) |
| 浅 | declared | `493..506`(14) + `514..520`(7) |
| 深 | enforced | `493..506`(14) + `514..521`(8) |
| 深 | declared | `493..506`(14) + `515..516`(2) + `520..520`(1) |

`493..506` = 14px 的 `alert` 环，`514..52x` = 7~8px 的 `●`/`○`。**两枚**。`alert` 与圆盘之间
只隔 **7 级像素**（514−507）——它们读成**一枚徽标的两个部件**，而不是两件东西。
（`○` 是空心环，所以它的墨段更碎：浅色 7px 一段，深色碎成 2+1；`●` 是实心盘，恒 8px。）

真页复核（`oneIntegrationApp` + `auditOpen`，720×1200，集成卡展开审计 + 审批记录）：
`tt.Texts()` 里 `●` 与 `○` 各出现 1 次，但**同一张卡上两个层级标题出现两次**：

| 位置 | 墨 | 字号/字重 |
|---|---|---|
| 审计块 `permissionTier`（`:1272`） | `TextStrong`（浅 27 / 深 237） | `FontSize-1` / 650 |
| 审批记录块（`:2465`、`:2471`） | `TextSecondary`（浅 107 / 深 154） | `Caption` / 620 |

两处都是「这一层的名字」，同一张卡、相隔 146u。**这一条本轮只记账、不改**（见 §六 的「需拍板」）：
两处的来源是两份不同的参考 CSS（审计块没有参考，审批记录的层级标题来自
`extensions.css`），统一它们要定「谁是这一层的权威写法」，属于语义决定。

## 三、逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `settingsui.go:1292` | `ui.Icon(c, mustGlyph("alert")).Size(t.Space(4), t.Space(4)).TextColor(color)` | **删掉整行**：这一格只留 `●`/`○` 一枚 |
| 2 | `settingsui.go:1293` | `ui.Text(c, mark).FontSize(t.FontSize-3)` = **11.00px**（shipped 步 `FontSize` 是 14.00） | **不动**：`FontSize-3` 已经是 11.00px，`tokens.Caption` 是 **9.90px**——换成 `Caption` 是**降**一档，而这一枚是行尾唯一的状态记号。实测量：`FontSize-3` 盒 11×17、墨 11×10；`Caption` 盒 10×15、墨 10×9。**结论：这一格的字阶本来就对，不动。** |
| 3 | `settingsui.go:1264-1268` 的注释 | 说「an enforced permission's row is a plane with the accent edge」——**这行没有 plane、也没有 edge**（只有一枚盘） | 注释改成实况：一枚圆盘，实心 = 主机自己决定，空心 = 仅声明 |
| 4 | 新增 `internal/settingsui/permimark_test.go` | 无 pin | 两条：①行尾**只有一段**墨（一枚标记，两套外观）；②`●` 是 accent、`○` 无色相——沿用 `raycast_test.go:1517` 的两侧判据，但量的是**唯一**那一枚 |
| 5 | `docs/liquid-glass.md` | 无此条 | 新增 **§6.32**：`32-1`…`32-4`（3 `已落地` + 1 `需拍板`） |

**`tokens.Caption` vs `FontSize-3` 的取舍（现量之后：不动）**：先量再说。`FontSize` 在 shipped
步是 **14.00**，所以 `FontSize-3` = **11.00px**；`tokens.Caption` = **9.90px**。把这一枚
改成 `Caption` 是**降一档**（盒 11×17 → 10×15、墨 11×10 → 10×9），而它是这一行尾**唯一**的
状态记号——降它等于让「已强制 / 仅声明」更难读。**所以这一格的字阶本来就在对的地方，
本轮不动它。** 这也解释了为什么 `raycast_test.go:1517` 的旧 pin 一直读得出这枚点：11px 下
accent 的 R−B 是 160 级，远在阈值之上。

## 四、行为契约零改动红线

- 高度链：`a.row` / `cardRow` 的 44u 地板、分隔线、行内 padding **一个字不动**。
  实测行高 44u（`●` 盒 y=97.8、`○` 盒 y=189.2，差 91.4 = 2×44 + 分隔 1 + tier 标题 1.4 行盒）。
- 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**。
  `copy.PermissionsEnforcedLabel` 等文案一个字节不动。
- `raycast-design.md` §5 注脚第 1 条（权限 `●` 保留 accent）：**继续成立**，本轮不动它的墨。
- 视觉轮只改墨与面：本轮**删**一枚图标、把一枚字阶对齐到已有档。

## 五、门槛命令与 PASS 基线（现量）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

**PASS 基线 = 3192**（HEAD `e6ee7570`，P13-bm 实测）。SKIP **112**，测试函数 **2346**。

## 六、mutation 计划（≥2 条，说明每条让哪个 pin 变红）

| # | 注入 | 应红在哪 |
|---|---|---|
| M1 | 把 `:1292` 那行 `ui.Icon(alert)` **加回去** | 新增 pin ①（行尾只剩一段墨）：`slotRuns` 从 1 段变 2 段 |
| M2 | 把 `mark` 的 `enforced` 分支改回 `t.TextMuted`（丢掉 accent） | 新增 pin ②（`●` 必须是 accent）：色相判据（R−B ≥ 100）红 |
| M3 | 把 `enforced` 的 `t.Accent` 换成 `t.TextStrong`（丢掉色相但保留墨重） | pin ② 的色相判据（R−B）红 |

**M3 原计划（字阶 11 → 9.9）在现量之后撤销**：这一格的字阶本来就对（见 §三 第 2 条），
没有可回退的旧值，按纪律不造脆弱断言。换成上面那条同族注入（色相），让 M3 仍然有意义。

## 七、出图

`/tmp/floter-P13-bn-permimark-{dark,light}-{before,after}.png`
（`FLOTER_P13BN_DUMP=1`；`before` 由 M1 注入取得）。图上看一处变化：行尾那格从「环 + 盘」
变成「一枚盘」，且那一格的左边界**右移** 21px（`493` → `514`），行尾不再有第二个物体。
