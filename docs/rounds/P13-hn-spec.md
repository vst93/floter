# P13-hn 规格：配置表单外部刷新与无变化提交

## 意图

让设置配置表单和插件配置浮层在宿主值发生变化时显示当前值，同时保留真正未提交的本地编辑。现在 `internal/settingsui/config.go:127-170` 只在首次建 draft 时读取 `ConfigValues`，以及 `internal/settingsui/pluginsheet.go:350-395` 在文本 Enter 后保留草稿；这会让外部刷新被旧草稿遮住。当前中文 locale 基线为 3842 PASS / 0 FAIL / 2816 测试函数。

## 落地项

1. pluginsheet.go:377 文本无变化 Enter 不退役草稿 → 外部刷新可见。
2. pluginsheet.go:548 数字草稿不观察宿主后续值 → 已确认数字跟随宿主更新，拒绝/半输入保留。
3. pluginsheet.go:580 重输同一个数字仍派发 → 不产生无意义保存/失败。
4. pluginsheet.go:499 滑块相同量化格仍派发 → 同格不写回。
5. pluginsheet.go:822 滑块读数乘 1e6 对合法大数溢出 → 1e308 仍显示有限值，不显示 +Inf。小于 1e-6 的步长因宿主 clampStep 本身固定六位，不能仅改 UI，留作需拍板。
6. config.go:127 外部移除一个未编辑配置键后框仍显示旧值 → 跟随当前有效值（默认值或空值）。原单选未知值属于 §6.163 已记录的需拍板项，本轮不改变分段控件选择模型。
7. config.go:127 旧 draft 永不观察外部刷新 → 未编辑字段跟随真实新一代，保存后旧列表不能回滚。
8. pluginsheet.go:817 数字步长 0.1 的浮点尾数进入框 → 显示网格精度内的十进制，不出现 0.30000000000000004。

## 2026-11-06 续跑

接续 HEAD 847be8ad：1–2 已提交；抢救稿 7dde43a78e 只含 3–4 的真实事件 pin，已 git apply -3 接回，首跑深浅均红。当前 HEAD 实测 3848 PASS / 0 FAIL（新增抢救 pin 的失败另计），2818 测试函数。统一 LC_ALL=zh_CN.UTF-8；首次 C locale 3628 PASS / 220 FAIL 是字体环境不一致，不修改旧 pin。每批 1–2 项通过相关回归后即提交推送，整轮结束跑一次完整三平台门槛。

规格行号为本次读码值：sheetSlider 483–516、sheetNumberSlot 555–615、boundedText 822、initialNumberText 837、configDraft/draft 25–170。数字无变化比较保存语义的 boundedValue，拒绝写入仍可重试。外部配置按新旧宿主代际比较，只更新未编辑字段，不覆盖 live 半输入；保存后旧列表不能回滚。

## 行为红线

高度链、键盘模型、会话管理、设置写回、i18n 文案与命令接线零改动。只同步未发生本地编辑的值；live 草稿和失败重试仍优先。

## 门槛

`gofmt -l cmd internal` 为空；`go vet ./...`、`GOOS=windows go vet ./...`、`GOOS=darwin go vet ./...` 净；`LC_ALL=zh_CN.UTF-8 go test -count=1 ./...` 全绿；三平台 `go build ./...` 通过。基线 3842 PASS / 0 FAIL / 2816 测试函数，PASS 只增不减。

## Mutation 计划

- M1 回退文本/数字刷新，M2 回退数字/滑块 no-op，M3 回退小数读数/未知单选，M4 回退外部配置刷新/数字精度。每批 pin 必须红，恢复后 md5sum -c 并 touch 复跑。

设计读取：现有设置表单，轻便液态玻璃；冒险 2 / 动效 2 / 密度 7。craft-loop #6 让数值显示/保存同一网格，#12 保留单层槽面、8px 圆角，#3 保留焦点身份。Apple 供体取交互蓝只给交互、8px 圆角与弱化约 0.5，映射既有 token，不复制网页模糊与大字。
