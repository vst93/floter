# P13-hm · 配置输入的值与状态

意图：让现有配置输入框画出的值、可用状态与宿主一致。起点 2fa36d8e，工作区为空，pull 已同步。沿 P13-hl 的输入框组合状态待办推进，不重复按钮用墨。

设计读取：紧凑配置表单，面向日常键盘用户，轻便液态玻璃；冒险度 2 / 动效 2 / 密度 7。3 秒内能读到当前值与可编辑性，保留原输入、提交、失焦与失败恢复路径。

## 逐项规格（开工代码行号）

1. pluginsheet.go:571：无上下界仍 clampNumber → 用 boundedValue，未声明界时保留输入。
2. pluginsheet.go:575：草稿只随夹界修正，不随步长修正 → 输入文字与派发值采用同一量化结果，半输入保留。
3. pluginsheet.go:547：聚焦草稿遮住新约束 → 合法数字草稿按当前约束重算，非法半输入不清除。
4. pluginsheet.go:475：滑块位置来自原值，读数来自 boundedValue → 轨道与读数取同一存量值，不派发无手势的写回。
5. pluginsheet.go:356：成功提交等值时草稿不退役 → 成功确认只退役本次提交的草稿，后续外部值可见；失败与新输入保留。
6. config.go:757/817：锁定文本/数字仍是主墨乘 0.5 → 采用现有 Disabled 墨，单次淡化与 DimFace 保留。
7. settingsui.go:3523：禁用命令别名仍是主墨 → 同档 Disabled，180ms 写回与忙态守卫不变。
8. settingsui.go:7217：运行中的参数文本仍是主墨 → 同档 Disabled，运行值归属与原恢复行为不变。

两批数字问题、提交与集成墨、别名与参数墨，每批 1–2 项验收后 commit + push。第 3–5 项须先用真实 ui.Tester 复现，未红不冒称修复。

红线：窗口高度链、键盘模型、会话管理、写回时机、i18n、命令接线不改；仅修既定取值规则的遗漏与现有禁用态墨。

## 门槛

中文 locale：LC_ALL=zh_CN.UTF-8，GOMAXPROCS=4。开盘 verbose 全量正在实跑，数字按完成日志补录；前轮 3816 PASS 只作历史参考。开盘测试函数实测 2808。

分批执行受影响 tests / vet / diff-check；最终一轮一次 gofmt -l cmd internal、go vet ./...、GOOS=windows/darwin go vet ./...、普通与 verbose go test -count=1 ./...、linux/windows/darwin go build ./...。

Mutation：M1 将数字处理退回 clampNumber，未声明界/步长显示 pin 必红；M2 删除约束刷新同步，schema 刷新 pin 必红；M3 恢复成功提交旧分支，外部值 pin 必红；M4 去掉禁用 TextColor，真实组件像素 pin 必红。每次保存 md5、注入、实跑、还原、md5sum -c、touch 后复跑。

## 设计取用

craft-loop #6 编辑细节：数字文字与读数同账；#3 焦点：保留现有焦点，无额外环；#12 材质：Disabled 前置墨沿用浅 #57575B / 深 #B5B5BD，框架淡化 0.5，禁用冠线 0。
Apple 供体的弱墨 48%、标准圆角 8px、交互蓝限定可操作态，分别映射现有 0.5、RadiusSM=8、禁用不加 accent。原生 Go，无 backdrop-filter。优先两轮截图复核，不为凑五项新增装饰。
