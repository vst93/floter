# P13-jw · 安装确认的当前声明与等待出口

开盘 2cca8ae9，pull 已同步，工作树干净。仓库已到 P13-jv，本轮按最新遗留推进。Codex 独立执行。开盘指定 done.log 不存在。基线与分批证据增量记录。

## Stash 清点

八条均为已审计早期独立 WIP，SHA 与上轮相同，无新增 stale-kill；逐条 reverse-check 退出1，不等于独有工作。沿用既有语义审核，不把旧主题/Tauri覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

## 实施记录

待实际事件复现后逐条记现象→改动→验证。旧V/B/S断言保持0/0/0；不改旧断言、不新增Skip。

### 第一批：两类安装入口的等待态

1. 现象：本地安装审核未回答时，原目录入口仍可点击；深浅事件红串 `pending local-busy permits repeat: calls=1`。改动：本地入口在审核、安装、刷新期间使用既有忙态胶囊，并在宿主拒绝重复派发。验证：真实Click等待时派发0次，取消结束后原入口可再点1次。
2. 现象：推荐工具连接审核期间同样可重复点击，红串 `pending recommended-busy permits repeat: calls=1`。改动：按推荐ID占用其自己的入口，复用同一忙态材质。验证：深浅Click和直接重复宿主派发，审核仅1次，取消后恢复。

旧代码的四个深浅场景均实红，原始 before PNG 和 M1-before.txt 已生成。后续正式回注、MD5复原、门槛数字据实追加。

第一批相关回归 **53 PASS / 0 FAIL**，settingsui/shell vet净。M1回注两个原入口，四场景实红；两文件MD5逐字节还原，touch后 **6 PASS**。浅色图目视：推荐行“接管”原位出现等待环并退淡，本地“选择…”保持可用；没有遮罩或新卡层。完整基线仍运行中，提交只引用已完成的定向数字。
