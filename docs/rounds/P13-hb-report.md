# P13-hb · 本地工具草稿、编辑焦点与内容反馈

起点 `72d291ff`，2026-11-05；工作树干净，pull --ff-only 已同步。当前 done.log 不存在，收尾创建。执行者为当前 Codex，无外部模型、无子代理。

## stash 开盘清点

活动 stash 8 条，全部实际 reverse-check 失败；均为早期非 stale-kill WIP。P13-gq/ha 已归档清理本任务书所指抢救稿，此次没有新 orphan。以下沿用已入库语义审核，不恢复旧主题/发布链，也不把 reverse-check 失败误报为独有改动。

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | P7 keycap 与旧主题稿 | 保留原件 |
| stash@{1} / 8af94a07 | 设置行距/滑块旧刻度 | 保留原件 |
| stash@{2} / 7e4aa710 | 卸载旧布局与 scratch 探针 | 保留原件 |
| stash@{3} / 1e3aec94 | 旧数字框与透镜稿 | 保留原件 |
| stash@{4} / 363f5a42 | 旧确认带/快捷键稿 | 保留原件 |
| stash@{5} / 9b8ce23c | P7 快捷键拒绝反馈稿 | 保留原件 |
| stash@{6} / 90d314b2 | action bar 旧键盘合约 | 保留原件 |
| stash@{7} / 7a9f6e13 | 旧 Tauri 发布链 | 保留原件 |

基线：中文 locale 全量 **3592 PASS / 0 FAIL**，函数数 **2711**（`^func Test` 同口径）。首次 C locale 字体 pin 红，已按 AGENT-NOTES 使用中文 locale 复核，未修改断言。下文逐批增量记录实测，不预填最终门槛。

## 已完成（逐条）

1. **删除脚本又冒出模板**：全选删除后两帧回到 264 字节模板，提交甚至成功一次 → 模板只在首次新建初始化，用户清空保持空 → 深浅两套实际键盘删除后为空，提交 0 次并显示既有空脚本错误。原始红串：`deleted script was repopulated: 264 bytes`、`empty script submit: calls=1 error=false`。
2. **撤销窜回上一工具**：编辑第二工具时 Undo 出现 `first edited` → beginEdit/endEdit 释放旧 editor.State，载入稿由 NewBuffer 初始化 → 第二工具无历史撤销，自己的新编辑仍可撤销。原始红串：`new tool inherited undo: "first edited\n"`、`undo escaped current tool: "first edited\n"`。

首批 V/B/S = 0/0/0，旧测试零改。红测日志 `/tmp/floter-P13-hb-pair1-before.log`，通过日志 `pair1-after.log`。深浅截图已通过 ui.Tester 导出，图上可见脚本编辑器从 9 行模板回到仅第 1 行空稿；完整尺寸和错误验证保留。

首批定向回归 **24 PASS / 0 FAIL**；internal/ci 通过。旧 copy/export 回归曾发现点击切换到 Script 的同帧未初始化模板，已将一次初始化归到 editor 创建路径修复，未改旧断言。
