# P13-ip · 刷新后的提示、录制目标与命令草稿

2026-11-07 04:46 UTC 开盘，da989db0，pull 最新、工作树净。Codex 独立执行；无 goal 池也不等待选题。基线 **4937 PASS / 0 FAIL / 3077 函数**。done.log 开盘不存在，收尾创建。

## Stash 清点

SHA 与上轮及 backlog 一致，无新增 stale-kill。逐条 reverse-check 均退出 1；沿已审查结论保留早期独立 WIP，不重新覆盖当前主题/Tauri 已撤销路径。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse 1 | 保留 |

## 实施记录

候选未实测前不计修复。旧 V/B/S 断言修改 0/0/0。同帧输入退出受 ui.Tester 每次事件自动 settle 限制，暂未取得真实批量事件证据，未改退出/写回时机。

### 第一批

1. 现象：别名从 taken 改 available、甚至清空后仍显示旧冲突。改动：Taken 仅在显示值与宿主 alias（TrimSpace 后）一致时显示。验证：深浅真实全选/输入/删除，改值立即清旧提示，改回 taken 保留有效提示；Flush 一次写入空串，180ms 不改。
2. 现象：快捷键命令输入切换内置动作再恢复后显示 say obsolete，Enter 覆盖宿主 say refreshed。改动：内置动作刷新时清对应命令草稿。验证：深浅真实输入、宿主刷新、复制读取、Enter，恢复后严格为 say refreshed。

相关 **94 PASS / 0 FAIL**，包 vet 净。M1 注入开盘两处产品：`old alias conflict labels a different pending draft`、`reopened command resurrected retired draft: "say obsolete"`、`Enter overwrote the new host action`。MD5 恢复、touch 后 **6 PASS**。旧 V/B/S 0/0/0，新 Skip 0。

### 第二批

3. 现象：第二条录制中宿主交换前两条，Ctrl+Shift+R 写进原第一条。改动：录制目标保留 key，绘制及捕获时重新定位。验证：深浅真实 Tab/Enter 启动录制、宿主交换、录入新键，只改原目标的 key，其他两个对象逐字段相同。
4. 现象：录制目标被宿主移除后，下一条仍显示录制态并接受新键。改动：目标 key 不再存在即退役录制状态。验证：深浅事件中原两条与草稿都不改变，录制提示消失。

相关 **34 PASS / 0 FAIL**，包 vet 净；M2 回注上一产品实红：`reordered recorder targets row 1, want 0`、`recorded key reached wrong binding`、`missing binding left a recorder on its neighbor`。MD5 恢复并 touch 后 **6 PASS**。最初新夹具 Tab 到第三条，已改为从命令框反向三次 Tab 确认第二条，再取真实产品失败；不使用夹具失败计修复。

### 第三批

5. 现象：录制重复键产生冲突后，宿主交换绑定，提示跟随旧行号落到另一条。改动：冲突保留绑定 key，刷新时重新定位。验证：真实录入 Ctrl+Shift+A 制造冲突，再重排，原对象仍有冲突、相邻对象无提示。
6. 现象：冲突所属绑定被宿主移除后，相邻行仍显示冲突。改动：冲突目标不再存在即退役。验证：深浅真实重复键与移除事件，提示及 customDuplicate 均清空，其他绑定未改。

相关 **40 PASS / 0 FAIL**、包 vet 净；M3 回注旧产品实红：`conflict followed row index instead of binding: row=1`、`missing binding left a conflict on its neighbor`。MD5 恢复、touch 后 **6 PASS**。旧 V/B/S 0/0/0。
