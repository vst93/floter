# P13-hd · 配置表单的输入、撤销与错误归属

起点 c8ee659e，2026-11-05；pull --ff-only 已同步，工作树空。Codex 独立执行。基线中文 locale 全量实测 **3624 PASS / 0 FAIL / 2727 测试函数**。done.log 开盘不存在。

## stash 清点

当前八条逐一 reverse-check 均未通过；已对照 P13-hc 报告与 stash-backlog：均为早期独立 WIP，原任务书 stale-kill 已由前轮归档清理，没有新 orphan。不将旧主题与旧 Tauri 发布链覆盖当前实现。

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | P7 keycap 旧主题稿 | 保留原件 |
| stash@{1} / 8af94a07 | 设置行距/滑块旧刻度 | 保留原件 |
| stash@{2} / 7e4aa710 | 卸载旧布局与 scratch 探针 | 保留原件 |
| stash@{3} / 1e3aec94 | 旧数字框与透镜稿 | 保留原件 |
| stash@{4} / 363f5a42 | 旧确认带/快捷键稿 | 保留原件 |
| stash@{5} / 9b8ce23c | P7 快捷键拒绝反馈稿 | 保留原件 |
| stash@{6} / 90d314b2 | action bar 旧键盘合约 | 保留原件 |
| stash@{7} / 7a9f6e13 | 旧 Tauri 发布链 | 保留原件 |

## 增量验收

1. **重排 schema 后编辑写错字段**：原 Directory 已全选，换序后输入把 Endpoint 改成 `/workspace/searchcalhost` → 每个 configField 以字段 key 标识 → 深浅实际输入恰为 Directory=`/workspace/search`，Endpoint 保持 `https://localhost`。原始红串 `reorder wrote into another field: map[directory:/workspace endpoint:/workspace/searchcalhost]`。
2. **切换中文后错误仍是英文**：无效数字 `1e` 后换语言，仍画 `a number is expected` → 三种本地校验错误按当前语言显示，宿主错误原样保留 → 深浅均画「这里要填数字」，随后键入 12 清除错误并更新值。原始红串 `field error kept previous language`。

第一批相关回归 **43 PASS / 0 FAIL**，internal/ci 通过。V/B/S **0/0/0**，旧断言零改。两场景深浅 before/after 已导出；浅色错误图已目视：中文错误紧贴原字段，未添加第二层面。

3. **换集成后 Undo 恢复上一集成路径**：编辑 search 后切到 notes，撤销得到 `/workspace/search` → 表单以现有 draft 对象区分控件历史 → notes 保持 `/workspace/notes`，自己的后续编辑仍可撤销。原始红串 `integration inherited another undo history: /workspace/search`。
4. **schema 新字段显示空值**：已有草稿刷新 schema，新增结果上限与服务地址没有读取默认 25 与已保存地址 → 仅初始化缺失 key，并同步该字段 baseline → 显示 25 与 `https://localhost:8080`，旧路径编辑保留；键入无效 `1e` 后不会被默认值覆盖。原始红串 `new schema fields lost their initial values: map[directory:/workspace/edited]`。

第二批相关回归 **42 PASS / 0 FAIL**，原始日志 `/tmp/floter-P13-hd/batch2-after.log`。V/B/S 0/0/0，旧断言零改。候选「关闭后隔帧重开继承撤销」原实现通过，未计入八项。首批 ae79c5f3 已推送。

5. **插件字段换序丢失未提交文本**：目录键入 `/workspace/edited` 后换序，Enter 仍留下 `/workspace/search` → 字段使用自己的 schema key → 深浅均将原编辑提交到 directory，endpoint 不变。原始红串 `plugin reorder lost or misdirected edit: map[directory:/workspace/search enabled:true endpoint:https://localhost]`。
6. **插件 Undo 带回另一插件的路径**：从搜索切便条后撤销恢复搜索路径 → 字段身份同时包含插件 ID → 便条保持自身值，便条自己的后续编辑仍能撤销。原始红串 `plugin inherited another undo history: /workspace/search`。

第三批两项深浅 before/after 已导出，旧断言零改。第二批 6f717a67 已推送。插件相关回归通过，计数在本批提交中记录。
