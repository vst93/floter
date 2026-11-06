# P13-ib：整行退场后的键盘归还

2026-11-07 03:02 +08:00 开盘，HEAD 4f62e976，pull 最新、工作树干净。最新队列已是 P13-ia，不重复 hw–ia 的仍存在目标刷新。

意图：集成整行消失后保住当前可用的键盘出口，同 ID 重现和迟到结果不能接走用户的键盘。证据：`settingsui.go:1881` 列表可直接走空态；`noticefocus.go:88` 的 restoreNotice 只在仍绘制的行内调用。真实事件八项全部红，见本轮 reproduce.txt；不是只凭代码推测。

设计读取：原生集成设置页，面向键盘用户，沿用轻便液态玻璃。视觉冒险 3（克制）、动效 2（即时）、密度 7（原密度）。三秒内要知道当前可操作位置；不增加页面、系统或依赖。空间感来自唯一可见焦点，不加装饰层。

## 八项与四批

| 批/项 | 开盘位置 | 现状 → 应改成 |
| --- | --- | --- |
| 1/1 保存 | config.go:363，noticefocus.go:88 | 保存中整行消失，焦点丢失且返回后 Space 重派保存 → 归还页头，回调仍结算原保存 |
| 1/2 丢弃确认 | settingsui.go:3383 | 脏表单问题退役，键盘也消失 → 归还页头，保留 edited 草稿 |
| 2/3 重新检查 | settingsui.go:3185 | Recheck 在飞时整行消失，Space 无出口 → 归还现有返回键 |
| 2/4 重新连接 | settingsui.go:3185 | Reconnect 同样失焦 → 对实际撤下的按钮执行同样归还 |
| 3/5 启用开关 | settingsui.go:3156 | 启用在飞时目标退场，旧开关焦点复活 → 清理旧焦点身份，保持只派发一次 |
| 3/6 停用答案 | settingsui.go:6735 | 忙态答案随目标消失时无立即出口 → 当帧归还页头，不改变在飞状态 |
| 4/7 移除答案 | settingsui.go:7019 | 忙态移除退场，目标回来之前键盘丢失 → 当帧归还可用入口 |
| 4/8 组件卸载 | uninstall.go:72 | 浮层随目标退场后旧焦点在同 ID 回来时复活 → 归还页头并退役旧 UI 句柄 |

以上行号为产品修改前实际读取；产品只改焦点退场接线，不改确认条件。每批两项先通过相关回归与包 vet，再 commit/push；最终集中跑一次全套门槛。

## 红线

高度链、Tab/Space/Escape 模型、3000ms 确认窗口、会话管理、设置写回时机、保存中草稿归属、i18n、命令接线均保持。恢复只移动实际持有焦点的旧入口；已移焦到页面复选框则不抢焦。不从 ID 推断新的操作、不取消在飞回调。

## 基线与门槛

开盘测试函数实测 2937；中文 locale verbose 全量实测 4322 PASS / 0 FAIL，退出 0。最终：gofmt -l cmd internal 为空；三平台 go vet ./...；普通与 verbose go test -count=1 ./... 全绿并计 PASS；三平台 go build ./...。固定 LC_ALL=zh_CN.UTF-8、GOMAXPROCS=4。

## Mutation

1. 删除配置两类失踪目标的焦点交接：Save/Discard 的深浅 entry pin 必须红，elsewhere 对照保持。
2. 删除修复类交接：Recheck/Reconnect 的深浅 entry pin 必须红。
3. 删除开关/停用交接：Toggle/Disable 必须红。
4. 删除移除/卸载交接：Remove/Uninstall 必须红。

每次注入后真实运行、记录错误、还原源码、md5sum -c 核验、touch 后 -count=1 复跑。原始 before 图与 after 图使用同一事件，由 ui.Tester 输出 PNG。

## 工艺选择

已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography，补读 Apple DESIGN 的色彩/交互语义。取菜单 #3：既有 accent 焦点环 2px、间隙 1px；#4：键盘交接 0ms、自动派发 0 次；#12：保留 1px 材质边，新增玻璃层 0。用户既定方向优先，不生成风格选择页，不为菜单五项添装饰。
