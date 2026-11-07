# P13-iz：快捷键编辑的拒绝归属与可触达性

开盘 038f2cb6，pull 已最新、工作树干净。实际最新 P13-iy，按最新交接继续。Codex 独立执行；无其他执行器、外部模型或代理。基线正在运行。

## Stash 清点

逐条导出 patch、reverse-check 全为 1。SHA 与 P13-iy 及 stash-backlog 一致；早期独立 WIP 继续保留，不重复合入旧主题或旧 Tauri 链。已接续的 stale-kill 不在活动清单内。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse 1 | 保留 |

## 增量记录

候选以真实 Click/Key 事件复现；原代码通过者不算修复。宿主仅保存注册成功项的行为保持。

### 第一批

1. 现象：B 录 A 被本地拒绝后，Add → Escape 抹掉 B 的拒绝。改动：Add 只清草稿所属的拒绝。验证：深浅真实点击与录键，旧代码两场景均红 `unrelated recorder erased saved refusal`；原三个绑定不变。
2. 现象：同一拒绝之后 C 开始录制再 Escape，也抹掉 B 的拒绝。改动：保存行开始录制只清本行拒绝。验证：相同真实事件确认 B 的提示仍属于 B，取消不写绑定。旧代码深浅均红。

基线实测 **5274 PASS / 0 FAIL / 3183 函数**。第一批相关 **179 PASS / 0 FAIL**、settingsui vet 退出 0；M1 回注旧代码 **6 FAIL**，MD5 OK，touch 后相关回归通过。V/B/S 0/0/0。前后图已导出，初次目视确认旧图 B 行下的拒绝说明消失。
