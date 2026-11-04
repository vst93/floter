# P13-go 规格 · 配置入口禁用时，行面也保持静息

意图：工具自管配置入口不可用时，消除包着禁用胶囊的整行假悬停 / 假按下反馈，让可操作表面才抬升。
方向是轻便液态玻璃既有体系的打磨，单一表面为设置页。设计读取：面向桌面工具用户的配置表单，
克制、清楚、少层级；视觉冒险度 2/10（沿用现有形状），动效 2/10（已有颜色反馈），密度 6/10（原行高）。
功能契约：3 秒内看清配置入口是否可用；有命令且有宿主动作时仍能打开提供方配置。

## 证据与落地项（改前代码实际行号）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/settingsui/settingsui.go:3236–3248` `configOpenToolRow` | `a.row` 永远传可反馈的 `rowFace{}`，`openable` 仅控制子胶囊 | 把同一个 `openable` 提到行外，传 `rowFace{Inert: !openable}` 给已有 `rowAligned` |
| `internal/settingsui/rows.go:1569–1577` | `face.Inert` 已可关闭行的悬停 / 按下墨；P13-eu 只给静态事实行接上 | 复用，不增加第二套判断或颜色 |
| `internal/settingsui/settingsui.go:3238` | 无命令、无宿主 callback 两个原因都禁用胶囊 | 两种原因都覆盖；正常入口仍保留行面反馈 |
| 新 `configinert_test.go` | 无直接覆盖该真实调用的像素 pin | en/zh × 深浅 × 无命令 / 无回调 / 都有，检查 rest/hover/press 及行盒与文字墨 |

行为契约零改动：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线全部不动。
只改既有 `Inert` 视觉开关，不给整行加 `Disabled`，不淡化行文字，不改变子按钮的禁用与点击判断。

## 先量与验证

起点 `ddc3ed19`；测试函数实测 **2654**。默认进程 locale 的首次全量为 **3271 PASS**，字体盒大批失败；
`LC_ALL=zh_CN.UTF-8` 恢复历史字体后，行盒 pin 实测通过；正确 locale 的全量基线 **3487 PASS / 0 FAIL**，与 HEAD 报告相符。
目标现物先量：缺命令 / 缺回调的两条臂与可用臂的 rowInsetFace，同帧记录标题盒和按钮盒。

所有门槛用同一个 `LC_ALL=zh_CN.UTF-8`：

```sh
gofmt -l cmd internal
go vet ./...
GOOS=windows go vet ./...
GOOS=darwin go vet ./...
go test -count=1 ./...
go test -count=1 -v ./... | grep -c -- '--- PASS'
GOOS=linux go build ./...
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

mutation：M1 把 `Inert: !openable` 改成 `false`，不可用入口的行面稳定 pin 必红；
M2 改成 `true`，可用入口必须仍有反馈的对照 pin 必红。每次实跑、逐字节 MD5 还原、touch、无缓存复验。

## 工艺取值

从 qiaomu-design 的 `craft-loop.md` 第 12 项取“材质细节”：状态决定面是否抬升，不添加材质；
第 6 项取 hairline 的克制：复用既有 1px 行规则线；第 4 项取 hover 揭示：只对可操作入口保留
`theme.ColorStep=120ms` 的既有反馈。参考 Apple DESIGN.md 的 980px 胶囊、1px 边和 400/600 字重角色，
转译为本仓既有 30u 胶囊 / 半高半径、1px 边、正文 600；不复制品牌蓝或 Web blur。
不为了凑满五项加装饰，用户的单表面、只改墨与面优先。
