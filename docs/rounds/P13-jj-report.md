# P13-jj · 传输结果的当前状态与过期反馈

起点 17a5059f，分支 mygo-rewrite，pull ff-only 完成，开盘干净。采用 qiaomu-design 的已有方向打磨路径。规格见 P13-jj-spec.md。本报告增量记录，未完成验证不标通过。

## Stash 清点

无活动 stale-kill；八条均与 P13-ji SHA 相同，reverse-check 退出 1。前轮已逐项区分为早期独立 WIP，继续保留，不重新合入旧主题与旧 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap WIP，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧设置间距/slider WIP，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/scratch WIP，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧插件数字框 WIP，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件样式 WIP，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 WIP，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏 WIP，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri WIP，reverse 1 | 保留 |

## 增量验收

1. 现象：同一已安装 ID 两次导入，配置按最后一条合并，但 enabled 留在中间状态，仍显示成功两项。改动：每条从当前 repository 读取 enabled；不改顺序与配置合并。验证：真实点击导入，深浅 × 初始开/关四场景，磁盘开关/两字段合并/成功计数均断言；原红 `repeated import reports success but enabled=true, want false`（另一极性同样红）。
2. 现象：同脚本较早选择器晚报失败，当前失败原因被旧原因覆盖。改动：共享错误槽拒绝同 owner 的较早版本。验证：两个真实异步选择器反序返回，深浅确认较新错误保留；原红 `script-late-failure: obsolete failure replaced current result`、`latest failure disappeared`。

第一批修前日志 batch1-red.txt 实红；修后门槛数字随提交记录。旧 V/B/S 断言修改 0/0/0。后六项 helper 仅准备，尚未计完成。

全量基线实测 5591 PASS / 0 FAIL、退出 0；测试函数 3284。第一批初验有一场新夹具滚动后未派发点击，旧 B 测试未红；增加点击前真实几何复核后再验，原日志保留，不把夹具失败算产品问题。

第一批 fb086bf1 已提交推送，相关 **18 PASS / 0 FAIL**，包 vet 净。提交正文误写 17，原日志为 18，以此处及 batch1-retest.txt 实测为准；不重写已推送历史。

3. 现象：集成导出已成功，再次导出选择器失败，上方仍留“已导出”成功浮面。改动：成功记录动作归属，后续同动作失败同步退役旧成功。验证：真实 Export 点击，深浅 `failed export retains obsolete success` 修前红；修后只保留当前原因，原备份仍在。
4. 现象：配置复制成功后输入无效文本重试，旧“已复制”与 UTF-8 错误同时显示。改动：复制成功带工具 ID 与动作归属，仅自身失败可以退役。验证：真实 Actions 交付剪贴板，再无效 UTF-8 拒绝；深浅 `failed copy retains obsolete success` 修前红，修后当前字段原因可读。

第二批相关 **66 PASS / 0 FAIL**、shell vet 净；旧通知/新恢复/跨目标测试均通过。亲眼复核：浅色导出原先上下两层成功和错误，现在只剩错误一层；深色脚本只显示较新 picker 原因。截图背景工具发现与滚动位置有差异，不能当作纯墨值比较；通知区域是本轮有效变化。

第二批 e6918d61 已提交推送。

5. 现象：配置导出成功后重试选择器失败，“配置已导出”继续压在当前错误上。改动：配置文件成功通知带配置动作和工具 ID。验证：深浅 `failed config-export retains obsolete success` 修前红；修后只保留失败，第一次写成的 JSON 与密码占位符不变。
6. 现象：脚本导出成功后再失败，旧“脚本已导出”仍在。改动：脚本成功通知记录脚本名归属。验证：两个真实异步选择器串行成功/失败，深浅 `failed script retains obsolete success` 修前红，修后错误可见，原脚本文件保留。

第三批首次相关运行 24 PASS，有一条前轮恢复测试在真实主机工具发现插入行时点击未派发。定位到 transferCauseFrame 接入了与传输无关的异步 Detected 列表；夹具固定为不显示主机候选，安装的真实 provider、点击与全部旧断言不动。该辅助夹具变更不计产品修复、不改 B 断言、不放宽容差。复验数字见下一记录。

第三批复验 **26 PASS / 0 FAIL**，shell vet 净；源码与旧行为断言检查通过。7b5cee7d 已推送；提交正文误写 25，原始 batch3-retest.txt 为 26，报告已纠正，不改写已推送提交。

7. 现象：配置导出较早选择器失败，在较新导出已经写成后才返回，旧错误重新出现。改动：同成功槽记住当前完成版本，拒绝旧失败；先登记成功再解除旧错误，闭合回调交错窗口。验证：真实两个选择器反序返回，深浅修前 `config-late-success: obsolete failure replaced current result`。
8. 现象：脚本导出同样在新文件已写成后冒出旧 picker 错误。改动：同脚本版本保护，旧成功回调不降低记录版本。验证：真实并发脚本导出、磁盘文件存在，深浅修前 `script-late-success: obsolete failure replaced current result`。写入与回调照常完成，只修反馈归属。
