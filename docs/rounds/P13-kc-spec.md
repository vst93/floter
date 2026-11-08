# P13-kc · 声明变动与模式退出的可见编辑目标

开盘 HEAD 27e48ad1，工作树干净、pull 已同步。按最新 P13-kb「仍待」接续，不重复任务书旧轮次已完成的安装恢复。

意图：让已有安装表单在声明增删、类型字段退役和模式刷新后，当前键盘出口同时完整可见。读取为面向工具作者的功能表单，沿用轻便液态玻璃；视觉冒险3（稳定）、动效2（即时）、密度7（保留紧凑表单）。3秒内看见下一处可编辑目标，能继续输入、重开选项、保存。

## 代码证据与计划

以下是动手前读取的真实行号；每项须先事件复现，未红的线索不计修复。

| 项 | 位置 | 当前 | 目标 |
| --- | --- | --- | --- |
| 1 | parameditor.go:117–119 | 新增只append | 新声明可见并可继续编辑 |
| 2 | parameditor.go:177–186 | 删除中项只Focus后继 | 滚动后按Enter删除，后继完整可见 |
| 3 | parameditor.go:177–186 | 删除尾项只Focus前驱 | 长选项记录的前驱完整可见 |
| 4 | parameditor.go:121–127 | 活跃声明外部撤回只Focus Add | 原Add出口完整可见，存活输入不抢焦点 |
| 5 | parameditor.go:268 | 选项字段退出只restore | 原类型选择完整可见且选项草稿保留 |
| 6 | parameditor.go:235 | placeholder退出只restore | 原类型选择完整可见且提示草稿保留 |
| 7 | settingsui.go:5835 | 程序模式退出只restore | 原模式组完整可见，程序草稿保留 |
| 8 | settingsui.go:5837 | 脚本模式退出只restore | 原模式组完整可见，脚本草稿保留 |

组件：声明列表、声明条件字段、程序/脚本模式组。不得通过重复深浅/新建编辑场景凑项数。

## 红线与工艺

保留高度链、键盘顺序/快捷键、会话管理、设置写回、i18n、命令接线、验证与3秒窗口。仅修当前操作目标丢失；不新增页面、依赖或系统。滚动使用已有ScrollIntoView，事件一次、0ms新增动画，后续滚轮自由。

已读qiaomu-design、user-preferences、craft-loop、preflight、chinese-typography。Apple DESIGN供体：单一交互accent、8px矩形圆角、1px边框职责映射现有token，不套网页背景滤镜。工艺菜单#3让1个原焦点可见，#4一次最小滚动，#12保留1px受光边、0层新增材质。用户禁止装饰扩张优先于凑5项。

## 验证

基线全量verbose实测6211 PASS / 0 FAIL / exit0，函数3449。每批一至两项定向事件+gofmt+vet，红线旧B测试零改，再commit/push；最终完整门槛一次：gofmt -l cmd internal；三平台go vet ./...；go test -count=1 ./...；go test -count=1 -v ./...统计PASS；三平台go build ./...。统一LC_ALL=zh_CN.UTF-8、GOMAXPROCS=4。删除唯一声明原实现已通过，不计修复，保留回归场景。

至少两组mutation：M1撤回新增/删除定位，目标焦点或像素包含断言须红；M2撤回撤回/空表出口定位须红；M3撤回条件字段定位须红；M4撤回模式定位须红。源码备份、md5sum -c、touch后无缓存复验。每项深浅ui.Tester PNG，完整设置页短窗口，目视复核。
