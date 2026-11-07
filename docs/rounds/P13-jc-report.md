# P13-jc：快捷键恢复、菜单键盘与长键可读性

起点 4e967fd8，pull 已同步、工作区干净。实际基线 5352 PASS / 0 FAIL，cmd/internal 3209 测试函数；全部测试/截图使用 zh_CN.UTF-8。普通英文大字号八种布局探针全部过界限检查，未把它们算成修复；截图暴露录制中的菜单键盘失效及长键全文不可达。

## Stash 清点

八条早期独立 WIP 与 P13-jb SHA 一致；reverse-check 均退出 1。backlog 已确认旧主题、旧数字字段或 Tauri 链不能覆盖当前实现，逐条保留；没有新增活动 stale-kill，没有静默删除。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | reverse=1，早期 keycap | 保留 |
| stash@{1} | 8af94a07 | reverse=1，早期间距 | 保留 |
| stash@{2} | 7e4aa710 | reverse=1，卸载指标/临时测试 | 保留 |
| stash@{3} | 1e3aec94 | reverse=1，旧数字字段 | 保留 |
| stash@{4} | 363f5a42 | reverse=1，旧插件字段 | 保留 |
| stash@{5} | 9b8ce23c | reverse=1，旧快捷键校验 | 保留 |
| stash@{6} | 90d314b2 | reverse=1，旧启动器动作栏 | 保留 |
| stash@{7} | 7a9f6e13 | reverse=1，旧 main/Tauri | 保留 |

done.log 开盘不存在。按本轮实测创建交接，不追认历史日志。

## 第一批：召回恢复

1. 刷新拒绝反馈：现象 → 请求 K 被系统拒绝、旧 J 可用却没有提示。改动 → 当前请求失败可见，设置恢复 J 后清旧状态。验证 → 初始探针 `refused settings refresh is invisible` / `working settings refresh retained old refusal` 实红。
2. 旧键重新确认：现象 → 录入界面上的 J，设置仍存被拒的 K。改动 → no-op 同时核对持久化值与实际注册，重新注册同键后不把新句柄注销。验证 → 两套外观真录键，文件写入恰 1 次、重新 OpenStore 读到 J，原键继续注册、焦点可重试。

第一批相关 115 PASS / 0 FAIL，shell vet 净。M1 回注开盘源码，两项深浅全部实红，原串含 `refused settings refresh is invisible`、`working settings refresh retained old refusal`、`recording fallback did not save the working key`。MD5 还原、touch 后 6 PASS。旧 V/B/S 0/0/0。首次新测试错误地用内存 Store 断言文件写入次数，改为真实临时文件 Store 后验证，不放宽断言；未把夹具错误计为产品缺陷。

第一批 **49d14733** 已推送。

## 第二批：保存行菜单的键盘出口

3. 保存行动作选择：现象 → 录制时打开菜单，Home/Down/Enter 被录制框吃掉，动作不变且菜单不关。改动 → 记录既有选择器打开状态，保存行的已开菜单在本帧取回焦点。验证 → 真实按键把动作改为浏览器，关闭后 Ctrl+Shift+Z 仍录到原行，动作选择只提交 1 次。
4. 保存行菜单 Escape：现象 → Escape 取消底下录制，菜单仍留着，后续组合键不能完成原行。改动 → 同一菜单拥有键盘，Escape 只关闭菜单。验证 → Escape 写入 0 次，录制继续且聚焦，下一组合键完成原行。

相关 355 PASS / 0 FAIL，settingsui vet 净。M2 回注旧 custom.go 实红，原串 `recorder left action menu open after keyboard exit`、`saved menu did not commit keyboard-selected action`、`menu exit canceled or lost the original recorder`、`menu return cannot finish the original binding`；两文件 MD5 还原，touch 后 6 PASS。旧 V/B/S 0/0/0。实现时漏一处闭括号的编译错误已修，不把编译错误计入 mutation。
