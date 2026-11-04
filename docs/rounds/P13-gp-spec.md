# P13-gp 规格 · 设置表面的刷新时钟与保存中反馈

意图：关闭设置页后台刷新在门槛中偶发重入的问题，并用真实配置表单核对保存期间的静息面。
为什么现在做：P13-go「仍待」第 1 条先要求定位 `TestAStaleMemoIsRefreshedBehindTheFrame`，
再量锁定字段。§6 无可直接实施的未开始行；原任务书的禁用胶囊已在 P13-s / go 落地。

## 真实位置与落地项

| 位置（开工实读） | 现状 | 本轮处理 |
|---|---|---|
| `internal/shell/integrationfacts.go:191–235` | 冷态与 TTL 比较读参数 `now`，异步发布却读 `time.Now()` | 保持请求的时钟域，并加实际计算耗时；完成时才起算 TTL |
| `internal/shell/integrationfacts_test.go:110–162` | 模拟时钟前进 TTL，第二帧可能恰好落在第一轮刷新完成后；新结果用旧时钟，被判过期 | 现有测试与断言零改；新增明确等待发布、再给同一帧时钟的确定性回归测试 |
| `internal/settingsui/config.go:477–515` | 外层是无填色、无 Hovered/Pressed 分支的网格，并非 `cardRow` | 先量，不预设有缺陷；保存中的 7 类字段外层应静息，描述和标签仍能读 |
| `internal/settingsui/config.go:545–759` | `configLocked` 给字段一次 Disabled；字段材质已有 DimFace | 不重画已落地材质，验证真实保存标志及恢复后的面/盒 |

设计读取：紧凑桌面设置表单，面向键盘和指针用户，轻便液态玻璃。拨盘 2/2/6：
视觉变化克制、动效轻、信息密度沿用。功能契约：可读现值、知道正在保存、保存结束恢复编辑。
Apple 供体的近白/近黑、单一交互蓝、1px 细边转译为既有 `#F7F7F8/#0E0E10`、accent 预算、
1px token 边；不移植网页字体、blur 或新增装饰。工艺菜单优先第 12（材质）、第 4（指针反馈）、第 6（细边）。

## 红线与台账

高度链、键盘模型、会话管理、设置写回、i18n、命令接线零改。前后台保持原来两秒 TTL、
冷态同步、过期态异步、完成后通知的规则；仅统一计时来源。旧 V/B/S pin 零改。
不将配置网格换成行、不新增透明度、不整行 Disabled，不给不可用控件造高光。

## 门槛与基线

统一使用 `LC_ALL=zh_CN.UTF-8`；开盘 HEAD `cfb17949`。完整基线实测 **3488 PASS / 0 FAIL**，exit 0。
测试函数开盘实测 **2656**。每轮门槛：

```sh
gofmt -l cmd internal
go vet ./...
GOOS=windows go vet ./...
GOOS=darwin go vet ./...
go test -count=1 ./...
go test -count=1 -v ./... | grep -c -- '--- PASS'
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## mutation 计划

1. 后台发布退回 `time.Now()`：新时钟 pin 的「完成时间不能早于请求时间」变红，证明旧缺陷。
2. 发布只用 `now`、丢掉计算耗时：新 pin 的「完成时间晚于请求时间」变红，保护完成时起算 TTL。
3. 若锁定网格静息判据成立，临时给网格加 hover 填色：保存态前后像素相等的判据变红。

每条实跑失败后还原，以 `md5sum -c` 核对，再 `touch` 和 `-count=1` 复跑。
保存中真实表单深浅 before/after PNG，若无视觉缺陷如实记零像素变化。

实施前复核：`rowBusy`（settingsui.go:3833）不含 configSaveBusy，因此保存并不锁字段。
新增图与判据明确覆盖 idle/saving/toggleBusy 三态，不把字段加锁冒充视觉修正。
