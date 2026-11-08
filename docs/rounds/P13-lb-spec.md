# P13-lb · 前置工具刷新后的命令与行底出口

意图：本地入口、推荐和扫描结果增撤/说明变高后，保留正在操作的命令与行底动作。沿用19px可视净空，让焦点离开16px淡边，布局与材质增量均为0。

开盘e23ab358，pull已最新且工作树空。首项收紧tools/verify.sh：每门槛最多两次，逐次日志保留。上轮原始verbose现场计数7170 PASS；3658测试函数是上轮已验证基线。安装恢复P13-jx已经结案，不重做。

## 八项候选（修前现场行号）

1. settingsui.go:3158 命令别名：未包含integrationLead → 前置区块刷新后原输入继续可见。
2. settingsui.go:3158 命令开关：相同缺口 → 原命令开关继续可见。
3. settingsui.go:3272 运行：只观察outputFeedback → 还观察前置区块。
4. settingsui.go:3335 诊断：只观察outputFeedback → 还观察前置区块。
5. settingsui.go:3377 移除：只观察outputFeedback → 保留确认入口。
6. settingsui.go:3394 启停：只观察outputFeedback → 保留原工具开关。
7. settingsui.go:3347 重新发现命令：没有显现包络 → 使用既有19px包络。
8. settingsui.go:3352 编辑工具：没有显现包络 → 使用既有19px包络。

每项先ui.Tester事件复现；自然通过不计bug。跨命令列表和集成行动组件。本地/推荐/扫描的插入、文本增长、撤回、恢复均验；等值重建和隐藏推荐不抢滚动；激活后的ID/输入/确认语义要保持。

红线：高度链、键盘模型、会话管理、设置写回、i18n、命令接线、3秒窗口、保存草稿归属不改。

## 验证

每两项聚焦门槛通过即提交推送；交付前完整门槛只执行bash tools/verify.sh P13-lb，明细落盘、摘要不超过30行。同一失败最多重跑一次，第二次失败记录并停止该项扩展。

M1去掉命令integrationLead；M2去掉行底动作integrationLead/包络。两条均注入、实红、MD5还原、touch后复验。深浅两套各一个代表状态前后图，不重复同状态截图。

qiaomu-design既定方向打磨：craft-loop #3沿用3px焦点环，#4一次0ms显现，#9沿用16px淡边；不为工艺数量新增装饰。Apple供体只借交互色专属、留白服务内容、单一主焦点三个动作；保留项目token与字号合约。
