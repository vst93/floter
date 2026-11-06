# P13-ih · 获取与提交回调换表单后保住当前面

开盘 b6a77746（实际前轮 P13-ig），pull 最新，工作区干净。首个时钟 2026-11-06 23:34:48 UTC。done.log 不存在，收尾补建。基线运行中，数字只在完成后填写。

## Stash 清点

当前八条 SHA 与 P13-ig 及 backlog 一致，无新增 stale-kill；早期抢救已接续归档。以下早期独立 WIP 不覆盖新主题、不恢复旧 Tauri 链，逐条保留。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 过程

规格先于产品修改。读取 skill、偏好及 craft-loop / preflight / chinese-typography，按已定方向打磨；不走四方案。运行表单同步关闭再重开仍无真实公开路径，不把直接改私有布尔值当成产品复现。

## 第一批：配置获取（1–2）

1. 现象：配置获取内部通过公开 OpenConfig 再开一面，外层返回把新上限 25 改回 10。改动：获取前记录已有显示代次，返回失配即不接纳旧结果。验证：深浅真实配置界面保留新值；Escape 仍关闭、新请求可再开。
2. 现象：内层最新获取返回 nil，外层仍复活旧配置控件。改动：最新空结果也拥有显示期。验证：深浅旧“保留上限”控件消失，下一次正常打开成功。

原版四叶子实红：`fetch-new lookup overwrote latest configuration: open=true`、`empty latest lookup resurrected old controls`。M1 撤回同一产品 diff 实跑、MD5 还原、touch 复验；结果见 batch1 日志。旧 V/B/S 修改 0/0/0。

第一批最终定向 **50 PASS / 0 FAIL**，相关 vet 净，d546dad4 已推送。

## 第二批：文件选择返回（3–4）

3. 现象：选择器内换到 local.new，画面却保留 oldtool 和 selected-for-old 路径。改动：整帧字段回写前核对既有表单身份。验证：深浅真实“选择”点击后保留 newtool、/usr/bin/newtool、--newtool；下一次普通选择仍准确接纳一次。
4. 现象：选择器内结束编辑，关闭后又回写旧草稿，再展开新连接出现旧名字和路径。改动：已关闭或换代的表单不接纳旧帧局部值。验证：关闭时草稿为空，真实点击展开仍为空，再选择当前路径成功。

M2 原串 `picker overwrote new draft: editing="local.new" name="oldtool" program="/usr/bin/selected-for-old" args="--oldtool"`、`closed picker resurrected draft: open=false name="oldtool" program="/usr/bin/selected-for-old" command="oldtool"`。四叶子实红，MD5 还原、touch 复验，旧 V/B/S 0/0/0；精确 PASS 见 batch2-green.txt 与本批提交正文。

第二批 **50 PASS / 0 FAIL**，384d26de 已推送。基线完整退出 0：**4615 PASS / 0 FAIL**；开盘 b6a77746 对象实测 **2995 测试函数**。基线脚本完成后数到 2999 包含当时新增的四个，不能冒充开盘值。

## 第三批：提交收尾（5–6）

5. 现象：保存 local.old 的回调打开 local.new，旧保存收尾又关闭并清空 local.new。改动：收尾核对提交表单身份。验证：深浅真实“保存”后保留新编辑器、名字、路径、参数；再保存新目标仅调用一次并正常关闭。
6. 现象：新建连接回调打开编辑目标，同样被旧连接的 endEdit 清空。改动：连接与保存共用已有表单身份约束。验证：深浅真实“连接”后 local.new 可见、可保存，原提交只发一次。

M3 四叶子实红，原串 `save-new completion erased newer editor: open=false id="" name="" program=""`、`connect-new completion erased newer editor: open=false id="" name="" program=""`。MD5 还原与 touch 复验，相关 vet 净；精确 PASS 见 batch3 日志和提交正文。旧 V/B/S 0/0/0。

第三批 **50 PASS / 0 FAIL**，0321fcec 已推送。

## 第四批：编辑工具获取（7–8）

7. 现象：点击旧工具“编辑”，获取回调里已打开 local.new，旧结果仍抢回 local.old。改动：beginEdit/endEdit 当场更新既有表单 key，获取返回核对该身份。验证：深浅真实编辑点击保留 local.new 与 newtool，后续明确点击可再编辑旧工具。
8. 现象：获取回调刷新目标使 Generated=false，旧结果仍打开本应不可编辑的表单。改动：返回按 ID 重读当前目标，核对存在、Generated、非 Orphan、写入能力和忙态。验证：深浅不出现失效编辑器；恢复资格只刷新不获取，下一次明确点击才打开且只调用一次。

M4 四叶子实红：`old tool lookup replaced newer editor: id="local.old" name="oldtool"`、`retired generated tool opened stale editor`。MD5 还原、touch 复验，相关 vet 净；精确 PASS 见 batch4 日志和本批提交正文。八项均有独立事件验收，旧 V/B/S 0/0/0、无新增 Skip。
