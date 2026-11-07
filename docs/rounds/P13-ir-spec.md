# P13-ir 规格：展开选择器刷新后的选项身份

## 意图与证据

同一设置表面内，已展开的选择器在宿主刷新选项顺序或当前值时，菜单仍应保持用户正在看的选项身份。开盘 `internal/settingsui/selectvalue.go` 已为插件配置实现按值保存高亮；`internal/settingsui/custom.go:428`、`internal/settingsui/parameditor.go:248`、`internal/settingsui/settingsui.go:5033` 与 `:5395` 仍直接使用 `ui.Select`。事件已确认旧高亮覆盖新值和重排错选，复用既有组件修复四处接线。

本轮只修设置页已有选择器的刷新与视觉反馈，不新增页面、依赖或设置项；快捷键、保存时机、键盘高度链和 i18n 文案保持不变。

## 落地项（按真实代码确认）

首次替换已全部撤回：configSelect 没有实际产品调用，不计修复；初值 ID 接线和测试 locale 错误如实留档。当前八项按每批两个真实事件逐一见红再修，不用材质不变量凑数：

1. `custom.go:428` 动作菜单展开后宿主改当前动作：旧高亮覆盖新值 → 同步外部值。
2. 同处动作选项重排：原索引错选邻项 → 保留原高亮值。
3. `settingsui.go:5033` 调色板展开后宿主改当前值：旧高亮覆盖新值 → 同步外部值。
4. 同处调色板顺序刷新：高亮跟索引漂移 → 保留原高亮值。
5. `custom.go:426` 两个动作同名：第二项无法选择 → 以动作 ID 区分。
6. `settingsui.go:5006` 两个调色板同名：第二项无法选择 → 以 palette ID 区分。
7. `parameditor.go:248` 参数类型菜单展开后宿主改类型：Enter 写回旧类型 → 同步新类型。
8. `settingsui.go:5395` 驻留菜单展开后宿主改时长：Enter 覆盖新时长 → 同步当前值。

八项均有真实事件红/绿证据；后六项的原始红在 `P13-ir/remaining-before.txt`。开启菜单的绑定移除路径只作后续独立线索，本轮不假称已修。

## 视觉与工艺取舍

轻便液态玻璃只保留现有 field 面与浮层阴影；不增加组件级透明度或渐变高光。焦点使用现有蓝色 2px 语义环，菜单仍是单一浮层。

## 门槛与 mutation

- `LC_ALL=zh_CN.UTF-8 go test -count=1 ./internal/settingsui`
- 开盘完整 verbose 实测 **5001 PASS / 0 FAIL / 3093 测试函数**。完整普通/verbose、三平台 vet/build 收尾各一遍。
- `gofmt -l cmd internal`、三平台 `go vet`、三平台 `go build`
- mutation M1/M2：分别恢复动作/调色板 raw Select，两组外部改值和重排 pin 必须红。
- mutation M3：恢复以 label 作为 ID，两种同名选项 pin 必须红。
- mutation M4：恢复参数/驻留 raw Select，两种外部改值 pin 必须红。
- 四组均 MD5 还原、touch 后 `-count=1` 复验；不弱化旧断言。
