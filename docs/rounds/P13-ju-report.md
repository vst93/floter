# P13-ju · 重连选择的当前归属与修复后的搜索恢复

开盘 HEAD 1a83f975，pull 已同步、工作树干净。Codex 单独执行，读取 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN。无外部模型与代理。指定 done.log 开盘不存在，收尾创建。

## Stash 清点

八条早期 WIP 的 SHA 与 P13-jt 语义审计一致，无新 stale-kill；逐条 reverse-check 退出1，保留原件，不重复接入已归档旧主题/Tauri工作。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/调试，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

基线正在运行。生成工具编辑覆盖保留待拍板；本轮转入已有修复和重连的结果恢复，逐项实测追加如下。

## 第一批：搜索恢复

1. 现象：真实点击重新连接后当前行可用，搜索仍为空。改动：Reconnect 完成调用已有 RefreshIntegrations，投影当前命令并清事实缓存。验证：深浅 Click→完成回调→搜索 root，命令准确1条且可见 Recovered command；改前均红 `reconnect-search completed but search stayed unavailable: commands=0`。
2. 现象：真实点击重新检查清除旧 broken 后搜索仍空。改动：Repair 同步采用完整刷新。验证：深浅当前行不再 broken，搜索命令1条且显示；改前均红 `repair-search completed but search stayed unavailable: commands=0`。

旧V/B/S断言0/0/0。before 为改前真实事件帧，after 在同一夹具下重新生成；定向结果见本批提交正文。

第一批 13d13fa4 已推送，相关8 PASS。开盘全量 **5926 PASS / 0 FAIL / 3377函数**，基线日志完整退出情况收尾核对。

## 第二批：选择目标的声明与包

3. 现象：打开运行时选择后磁盘声明已换 executableNames，旧选择仍重写当前记录并报成功。改动：返回后重读磁盘声明，与打开时比较，过期答案安静退出。验证：真实 Click→选择门控→磁盘更新→回答，深浅仓库字节不变、无旧通知、当前入口可重试。
4. 现象：同ID已指向另一包目录，旧选择仍绑定到新包。改动：同样复核 manifestPath。验证：两主题包路径及记录字节保持，原选择不产生成功通知且释放忙态。

夹具初次编译误引用不存在的 Managed 常量，已改为字段实际字符串值；编译失败不计变异。行为红串与完整验收收尾归档。

第二批 75450f68 已推送。原始红串 `obsolete manifest choice rewrote current repository` / `obsolete package choice rewrote current repository`，旧答案还导致“重新连接”入口消失。第三批把通知采样移到绘制前，避免已消费队列造成漏检；不改旧断言。

## 第三批：选择期间运行时刷新

5. 现象：选择窗口等待时工具已绑回当前程序，旧答案仍覆盖该路径。改动：复核当前 ExecutablePath。验证：深浅实际选择事件，仓库逐字节不变、当前绑定保留、无“已重新连接”假通知；改前红 `obsolete binding choice rewrote current repository`。
6. 现象：运行目录已更新，旧候选仍写入新运行时。改动：复核 RuntimeRoot 以及包/运行版本，选择归属随运行时变化退役。验证：深浅目录与仓库保留，原行解锁可再次重连；改前红 `obsolete root choice rewrote current repository`，并错误显示“检查工具 已重新连接”。
