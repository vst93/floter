# P13-hs：配置动作的确认、恢复与状态归属

起点 `6e3e4f09`，pull --ff-only 最新，工作树干净。done.log 在开盘时不存在。临时主线优先：同族八项，跨配置浮层按钮与启动器配置状态组件，每两项提交推送。

## Stash 清点

八条均为早期独立 WIP，SHA 与 P13-hr 一致，逐一 reverse-check 退出 1。已归档 stale-kill 的接续记录在 backlog 与前轮报告，未重复试合。原件逐条保留，不把旧主题/Tauri 实现覆盖当前 Go。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap WIP，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider WIP，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实施记录

取消与失败键盘路径已在未改产品上实跑红；首次确认与成功返回均原样通过。后续逐批追加实测，不把候选写成完成。

### 第一批

1. 现象：取消后 Space 无效、焦点消失。改动：确认状态退出且焦点仍属于原按钮组时，把焦点交回主按钮。验证：键盘取消仅 Unarm=1、Run=0，随后 Space 再次 Arm；深浅两套通过。原红串 `cancel lost action keyboard focus`、`cancel cannot rearm by keyboard: arms=1 armed=""`。
2. 现象：运行失败插入错误句，按钮身份改变，键盘不能重试。改动：主按钮在初始/确认/失败三态共用显式 key，错误句不再挤掉身份。验证：失败后原焦点仍在，Space 能重新 Arm，Run 只发生一次；成功返回、初次 Arm 与跨组操作回归通过。原红串 `failure lost action keyboard focus`、`failure cannot rearm by keyboard: arms=1 armed=""`。

M1 回退 pluginsheet.go 两项修复，两个事件深浅实跑红；MD5 恢复 OK，touch 后 6 PASS。旧断言 V/B/S 修改数 0/0/0。深浅 before/after 已落盘；人工看图：取消后主按钮重新出现蓝色焦点环，失败句保留，右侧按钮的焦点环不再消失。基线 3971 PASS / 0 FAIL / 2858 函数。

第一批 `41c51dac` 已推送，相关回归 38 PASS / 0 FAIL，settingsui vet 净。

### 第二批

3. 现象：确认后的动作被 schema 删除，再加入同 key 时仍显示“确认清理”，已失败的提示也会复活。改动：每帧同步配置时，持有者对照当前 Action 字段清理 Armed/ActionFailed。验证：真实 Click→移除→重加后是初始按钮；持有者同时覆盖 armed/failed 两种状态。原红串 `retired action kept state: armed="clear_history" failed=""`、`re-added action resurrected confirmation or failure`。
4. 现象：动作改为开关再改回，也继承旧确认/失败。改动：存在同名字段也必须仍为 Action 才保留状态。验证：真实 Click→Toggle→Action 后必须重新确认；原红串 `re-added action still asks to run the retired confirmation`。未改模式退出/高度链。

M2 回退状态清理，两个真实 UI 场景深浅实跑红；MD5 恢复 OK，touch 后 6 PASS。定向回归与两包 vet 通过。V/B/S 0/0/0。

第二批 `480e78c5` 已推送，相关回归 30 PASS / 0 FAIL。

### 第三批

5. 现象：RunConfig 没核对是否确认，首次直接调用与一次确认后的重复回调都会执行。改动：只接受当前 Armed 对应 key；执行前消费这次确认。验证：未确认 Run=0，一次 Arm 后连续两次 Run 总共只派发 1 次。缺陷回退红串 `unconfirmed action ran 1 times`、`one confirmation dispatched 3 commands`。
6. 现象：已删除字段的 Arm/Run 会覆盖、消费另一个动作的确认，普通值字段也能进入动作链。改动：Arm/Run 同时核对当前 schema 字段必须为 Action；不匹配的调用保留其他动作的确认。验证：stale key 与 Toggle 均不能派发或夺走确认，合法动作仍只执行一次；改型到 Text 即使尚未画下一帧也拒绝旧回调。原红串 `stale arm replaced current confirmation: "removed"`、`stale run consumed current confirmation: runs=1 armed=""`。

M3 回退两入口校验，两项实跑红，MD5 还原与 touch 后 2 PASS。相关回归和 launcher vet 通过，V/B/S 0/0/0。这两项是入口回调计数证据，不伪称普通静态截图能证明重复派发。

第三批 `bb91c79d` 已推送，相关回归 34 PASS / 0 FAIL。

### 第四批

7. 现象：写回回调内关闭配置，返回后解引用 nil；回调内换配置，旧接受值或失败标记覆盖新浮层。改动：保存发起写入的 Config 实例，返回后仅向仍为该实例的浮层发布答案。验证：关闭/切换 × 接受/拒绝四场景；写回只调用一次，目标 plugin/key/value 正确，新值不变、失败标记不串。原红串 `write answer crashed after close-refused: runtime error: invalid memory address or nil pointer dereference`、`old write answer contaminated new sheet: failed=false enabled=false`。
8. 现象：动作失败回调关闭配置后崩溃；切换配置后错误挂到新动作。改动：动作返回时同样检查发起实例。验证：关闭/切换 × 成功/失败四场景，新浮层的确认保留、ActionFailed 为空，命令只调用一次。原红串 `action answer crashed after close-refused: runtime error: invalid memory address or nil pointer dereference`、`old action answer contaminated new sheet: failed="clear_history" armed="clear_history"`。

M4 回退实例归属，两测试含八子场景实跑，六子场景红；MD5 还原、touch 后 10 PASS。定向回归与 launcher vet 通过。V/B/S 0/0/0，旧断言零改；八项均已实现，完整门槛固定源码后执行。
