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
