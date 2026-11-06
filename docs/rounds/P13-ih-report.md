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
