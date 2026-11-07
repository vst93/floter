# P13-jn · 诊断结果的完整呈现与当前归属

开盘8a4a7dc1，ff-only pull已最新，工作树干净。20:20 UTC开始；Codex独立执行。基线正在跑，未写未经完成的计数。

## Stash清点

八条早期独立WIP与P13-jm SHA一致，无活动stale-kill。逐个导出并reverse-check退出1，沿用已有语义审计，保留旧主题/Tauri原稿，不重复覆盖新实现。第一遍显示值因shell命令替换覆盖退出码误打印0，第二遍单独保存退出码核对，原错误日志均保留。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载/scratch，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

## 增量实施

正在用真实提供者进程、声明写盘、Store.Refresh和ui.Tester建立事件证据。

1. 现象：提供者返回 maintenance，总状态在诊断标题中缺失。改动：列表接出已实现的 DiagnosisStatus。验证：真实提供者子进程与深浅ui.Tester，修前 `provider verdict never reaches the diagnosis heading`；修后精确显示 Maintenance。
2. 现象：提供者返回网络正常和账号警告，玩家只见摘要，成功检查完全消失。改动：列表接出有序 DiagnosisChecks。验证：两个原始ID及顺序、两个可见消息联合断言；修前 `provider checks never reach the diagnostic grid`，修后全绿。

第一批相关7 PASS / 0 FAIL，shell vet退出0，旧V/B/S 0/0/0。原测试首稿误要求界面显示原始ID与小写状态，读取真实网格合约后修为检查ID数据顺序和既有首字母大写/消息显示；没有修改旧断言。正式mutation留到最终产品统一执行。

第一批bb8d9631已推送。

3. 现象：诊断结束后健康报告刷新为异常，摘要仍被旧诊断盖回健康。改动：健康块保留当前持久报告的投影。验证：真实仓库写盘、Store.Refresh和深浅界面，修前 `diagnosis overwrote current health: "健康" want "异常"`；修后异常文本与状态枚举一致。
4. 现象：诊断后由中文切英文，健康标签仍为“健康”。改动：同一投影只用当前语言，诊断不再拥有健康译文。验证：Store.Update触发实际语言切换，修前 `diagnosis health kept previous language: "健康" want "Healthy"`；修后Healthy可见。

第二批相关14 PASS / 0 FAIL，shell vet退出0，旧V/B/S 0/0/0。首轮目检浅色完整图：两个检查项已进入原有54px状态列与消息列，成功项不再消失；没有额外卡片或阴影。

第二批89107bb7已推送。全量基线日志完成：5702 PASS / 0 FAIL，起点3319测试函数。

5. 现象：同ID的工具换包后，新包卡片继续显示旧包诊断。改动：结果记录包路径、程序、版本与声明身份；显示前不匹配则退役。验证：真实包路径变更与Store.Refresh，修前 `owner-package: current tool inherited another declaration's diagnosis`；修后旧检查撤下，新包再检查能显示自己的结果。
6. 现象：检查已启动，提供者声明在等待期间变化，旧进程晚到仍污染当前诊断。改动：写回前核对当前声明，失效结果不发布。验证：真实子进程启动标记/释放门、声明写盘刷新、完整worker完成等待；修前 `owner-pending: current tool inherited another declaration's diagnosis`，修后旧结果被拒且新检查可用。

第三批相关20 PASS / 0 FAIL，shell vet退出0。把原异步闭包提为同一worker以便测试等待完整完成，公共动作仍异步；没有改诊断协议、超时或持久健康写回。命令重探测的失败也携带来源，兼容原有共用诊断出口。旧V/B/S 0/0/0。

第三批b8e6ac7f已推送。

7. 现象：连续检查时第二次先成功，第一次晚到的失败覆盖新诊断。改动：入口记录本工具请求序号，写回只接受当前序号。验证：两个真实子进程以文件门精确控制先后，修前 `older check overwrote the latest completed diagnosis`；修后只见最新成功，旧失败不复活。
8. 现象：同一顺序下界面先收到工作正常，随后又显示旧检查“报告了问题”。改动：失效结果在通知之前退出，不派发旧完成反馈。验证：深浅当前toast与队列共同断言，修前 `superseded check replaced current completion feedback: ["检查工具 工作正常" "检查工具 报告了问题"]`；修后只保留一条当前成功。

第四批相关26 PASS / 0 FAIL，shell vet退出0。首稿没有提供panelOnScreen seam导致通知进入系统出口，读实际路由后补用既有seam并重新得到上述真实两条toast失败；未修改产品通知路由。八项均已独立红绿复现，旧V/B/S 0/0/0、新增Skip0。随后统一进行正式变异与完整门槛。
