# P13-hn 规格：配置表单外部刷新与无变化提交

## 意图

让设置配置表单和插件配置浮层在宿主值发生变化时显示当前值，同时保留真正未提交的本地编辑。现在 `internal/settingsui/config.go:127-170` 只在首次建 draft 时读取 `ConfigValues`，以及 `internal/settingsui/pluginsheet.go:350-395` 在文本 Enter 后保留草稿；这会让外部刷新被旧草稿遮住。当前中文 locale 基线为 3842 PASS / 0 FAIL / 2816 测试函数。

## 落地项

1. pluginsheet.go:377 文本无变化 Enter 不退役草稿 → 外部刷新可见。
2. pluginsheet.go:548 数字草稿不观察宿主后续值 → 已确认数字跟随宿主更新，拒绝/半输入保留。
3. pluginsheet.go:580 重输同一个数字仍派发 → 不产生无意义保存/失败。
4. pluginsheet.go:499 滑块相同量化格仍派发 → 同格不写回。
5. pluginsheet.go:802 滑块读数固定舍入六位 → 小于 1e-6 的合法步长读数可见。
6. pluginsheet.go:408 单选不存在的值默认 index=0 → 无选择，不伪装首项已存储。
7. config.go:127 旧 draft 永不观察外部刷新 → 未编辑字段跟随真实新一代，保存后旧列表不能回滚。
8. pluginsheet.go:817 数字步长 0.1 的浮点尾数进入框 → 显示网格精度内的十进制，不出现 0.30000000000000004。

## 行为红线

高度链、键盘模型、会话管理、设置写回、i18n 文案与命令接线零改动。只同步未发生本地编辑的值；live 草稿和失败重试仍优先。

## 门槛

`gofmt -l cmd internal` 为空；`go vet ./...`、`GOOS=windows go vet ./...`、`GOOS=darwin go vet ./...` 净；`LC_ALL=zh_CN.UTF-8 go test -count=1 ./...` 全绿；三平台 `go build ./...` 通过。基线 3842 PASS / 0 FAIL / 2816 测试函数，PASS 只增不减。

## Mutation 计划

- M1 回退文本/数字刷新，M2 回退数字/滑块 no-op，M3 回退小数读数/未知单选，M4 回退外部配置刷新/数字精度。每批 pin 必须红，恢复后 md5sum -c 并 touch 复跑。

设计读取：现有设置表单，轻便液态玻璃；冒险 2 / 动效 2 / 密度 7。craft-loop #6 让数值显示/保存同一网格，#12 保留单层槽面、8px 圆角，#3 保留焦点身份。Apple 供体取交互蓝只给交互、8px 圆角与弱化约 0.5，映射既有 token，不复制网页模糊与大字。
