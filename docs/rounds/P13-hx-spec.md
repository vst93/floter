# P13-hx：确认条件刷新后的出口

意图：集成仍在列表中、提问条件已经变化时，及时撤回失效确认，保住草稿与键盘出口。起点 fe52ed35，工作树干净、pull 已最新；上一轮仍待第 1 项与以下真实分支支持本轮选题。

设计读取：原生集成设置面，服务配置与维护工具的用户；沿用轻便液态玻璃，视觉变化 3 / 动效 2 / 信息密度 7（克制、即时、保持原功能密度）。功能契约是看清当前工具状态、编辑与保存配置、明确回答有效的破坏性问题。

## 候选与验收（先事件复现，未复现不计数）

| 项 | 开盘位置 | 现状 → 应改成 |
| --- | --- | --- |
| 1 | settingsui.go:2921、noticefocus.go:30 | 配置外部写入与草稿一致后带子隐藏而问题仍开 → 同帧撤问题，归还 Configure 焦点，不清草稿 |
| 2 | settingsui.go:3198、noticefocus.go:30 | schema 清空不再绘制配置块，问题仍开 → 撤问题；schema 返回不复活 |
| 3 | settingsui.go:2934、3322 | owner 改 tool 隐藏原问题 → 撤问题，保留 host 草稿；owner 返回不复活 |
| 4 | settingsui.go:2984、noticefocus.go:23 | orphan 提前返回隐藏移除问题 → 清问题、计时与旧错误，Escape 正常退出 |
| 5 | settingsui.go:3137、6695 | Enabled 变 false 后旧停用确认仍可提交 → 撤未派发的问题 |
| 6 | settingsui.go:3150、6695 | Broken 后开关禁用但旧答案仍活 → 撤未派发的问题 |
| 7 | settingsui.go:2984、uninstall.go:66 | orphan 隐藏组件卸载浮层但状态仍开 → 撤浮层与未提交选择 |
| 8 | settingsui.go:3756、uninstall.go:66 | npm/custom 改普通链接后旧组件卸载仍开 → 撤不再适用的浮层 |

跨配置丢弃、移除带、停用带与组件卸载四组件。每批两项提交并推送；一个系统的同类有效性判断统一处理，不新增系统、页面、依赖。行号为实施前读取值。

## 红线与工艺

高度链、键盘模型、会话管理、设置保存时机、i18n 文案和命令接线不改；3 秒时钟不改；在飞操作仍由原回调结算。只修当前显示事实和确认状态不一致，不决定保存期间新输入归属。

已读 qiaomu-design / user-preferences / craft-loop / preflight / chinese-typography。Apple DESIGN 供体：单一交互蓝角色映射现有 accent；2px focus 环保留，失效问题交接 0ms；8px 控件圆角和 1px 边不增加层数。craft-loop 菜单 #3 焦点、#4 即时状态反馈、#6 减掉无效提示、#12 保留 E1/E2 材质。用户功能修复优先，不为凑第五项加装饰。

更轻与空间感的原因：不再让失效警告带和失效浮层占据当前工作面。

## 门槛与 mutation

基线中文 locale 实测 **4171 PASS / 0 FAIL / 2904 测试函数**。正式测试与截图均 LC_ALL=zh_CN.UTF-8；按环境记录使用 GOMAXPROCS=4。每批跑相关事件与包 vet，完整普通/verbose 测试、三平台 vet/build 和 gofmt 在本轮全跑，失败先修产品，原 B 断言零改。

命令：gofmt -l cmd internal；go vet ./...；GOOS=windows/darwin go vet ./...；go test -count=1 ./...；go test -count=1 -v ./... 并数 --- PASS；GOOS=linux/windows/darwin go build ./...。

M1：回注丢弃旧条件，clean/schema 事件应红。M2：回注 owner/orphan 旧条件，隐藏问题 Escape 或返回事件应红。M3：回注停用旧条件，disabled/broken 事件应红。M4：回注卸载旧条件，orphan/kind 事件应红。每组备份产品、注入、实跑红、还原、md5sum -c、touch、再跑无缓存测试。

ui.Tester 导出深浅 before/after 到 /tmp/floter-P13-hx-*.png，并复制至轮次目录；亲自复核整图及焦点局部。Linux 软件渲染不等同其他平台真机。
