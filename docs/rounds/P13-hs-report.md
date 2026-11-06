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
