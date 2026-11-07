# P13-in · 集成刷新后的键盘目标

意图：让现有运行表单与命令列表的键盘位置跟随当前对象，撤回后有可操作出口。接 P13-im 的卡片退场与忙态改型线索；只修真实事件复现的缺口。

设计读取：面向已有工具用户的设置功能页，沿用轻便液态玻璃；视觉冒险度 2（保持结构）、动效 1（即时反馈）、密度 6（现有紧凑表单）。3 秒内识别当前工具与可用动作；刷新后仍能编辑、取消或退出。

## 开盘与基线

起点 bc606b67，pull 已最新，树净。实测中文 locale verbose 全量 4821 PASS / 0 FAIL / 3057 测试函数（全仓 func Test 口径；cmd/internal 行首口径为 3055）。done.log 不存在。八条早期 WIP 与上轮 SHA 相同，reverse-check 全部 1；沿用已审结论保留，报告逐条列。

## 复现候选（读过当前行号）

1. noticefocus.go:20 退场清单无 run → 整卡移除后回页头 Close。
2. settingsui.go:3043 orphan 回收清单无 run → 表单随 Params 消失时回 Adopt，缺少 Adopt 回 Close。
3. settingsui.go:7463 改型 refocus 被 locked 拦住 → 验证同帧忙态改型是否已有 Cancel 出口；成立则只保护、不计修复。
4. settingsui.go:7576 Run 变禁用 → 仅原提交键持焦时回 Cancel；验证停止/破损/忙态/验证条件刷新。
5. settingsui.go:3505 commandList 按位置画 → 重排后别名输入仍交给原 ID。
6. 同处 → 重排后开关 Space 仍切原命令。
7. settingsui.go:3539 commandPlate 无退场焦点记账 → 命令移除/声明撤回/整卡移除回既有页头出口。
8. settingsui.go:2984 orphan 隐去命令列表 → 原命令持焦时回 Adopt。

若第 3 条已通过，从同族新增独立失效场景补足八项，先复现再更新规格。每批只修 1–2 项，相关测试/vet 通过后 commit/push；全量门槛本轮集中跑一次。

## 红线与门槛

高度链、Tab/方向键模型、会话管理、设置写回、180ms debounce、3000ms 窗口、i18n 与命令接线不改。只修控件身份和既有出口的焦点归还；不添加系统/页面/依赖。旧 V/B/S 断言零修改。

最终：gofmt -l cmd internal 空；go vet ./... 与 GOOS=windows/darwin go vet ./...；普通与 verbose go test -count=1 ./...（统计 PASS）；GOOS=linux/windows/darwin go build ./...。全部 LC_ALL=zh_CN.UTF-8。

Mutation：M1 撤回 run 整卡/orphan 回收；M2 撤回命令语义 key，别名/开关会误派给邻居；M3 撤回退场命令回收；M4 撤回禁用提交键出口。每条看到失败后恢复 MD5、touch 重验。深浅 ui.Tester PNG 导出 /tmp/floter-P13-in-*，原帧和对照入仓。

## 工艺取值

复现后的范围修订：第 3 条忙态改型已通过，仅保护。替换为 settingsui.go:3080 集成 Run 入口禁用时的焦点出口；停止/破损/忙态三组深浅事件均已复现。第 4 条提交键覆盖停止/破损/忙态/验证失败四组。最终八项按报告批次排序。

craft-loop 第 3 项：沿用 2px 品牌焦点环，当前键盘目标恰好 1 个；第 6 项：既有 1px 分隔与 8px 命令板圆角；第 12 项：新增材质层 0。修复让焦点与当前面一致，不以增加装饰凑五项。用户指定仓库单一设计来源，不引外部网页供体、不走四方向预览。
