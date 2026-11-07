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
