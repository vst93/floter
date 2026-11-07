# P13-in · 集成刷新后的键盘目标

开盘 bc606b67，工作树净，pull 已最新。Codex 独立执行；已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography，沿既有方向打磨。基线实测 4821 PASS / 0 FAIL / 3057 测试函数；done.log 开盘不存在。

## Stash 清点

早期 stale-kill 已在前轮归档。当前八条均与 P13-im 及 backlog SHA 一致，无新抢救稿；实跑 reverse-check 均 1。沿用语义审查结论，不把旧主题/Tauri 工作覆盖到当前实现。

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

## 复现与第一批（1–2）

旧产品七条候选失败（含三个命令退场子场景），忙态同帧改型已能到 Cancel，仅保留保护测试，不计修复。原始事件与深浅图已落盘。补查运行入口禁用作为第八条候选。

1. 现象：运行参数持焦时整张集成卡撤回，Enter 无法退出。改动：整卡退场清单加入 run，归还既有 Close。验证：深浅真实 Tab → 撤回 → Enter，关闭恰好一次，答案 12 保留、运行零次。
2. 现象：卡片成为 orphan 且不再声明参数，原表单消失后 Enter 无出口。改动：orphan 分支接入 run 焦点回收，优先 Adopt。验证：深浅刷新时零动作、Enter 才 Adopt 一次，保留答案。

相关门槛 **233 PASS / 0 FAIL**，settingsui vet 净。M1 撤回两文件到开盘实跑失败，原串 `removed run lost keyboard destination "Close settings"`、`orphan run lost keyboard destination "Adopt"`；MD5 一致并 touch 后 **6 PASS**。旧 V/B/S 断言修改 **0/0/0**。

## 第二批（3–4）

3. 现象：重排后继续输入别名，回调收到邻居 build。改动：命令板身份绑定 ID + 同 ID 出现序号。验证：深浅原 JSON Viewer 的焦点保留，输入 jview 后 Flush 只写 jv。
4. 现象：重排后按 Space 切换邻居 build。改动：同一身份覆盖板内开关。验证：深浅焦点仍跟随 JSON Viewer，Space 只切 jv；重复 ID 的 Build 开关仍能使用，无 key 冲突。

相关 **65 PASS / 0 FAIL**，包 vet 净。M2 撤回身份接线实跑红，原串 `reordered alias edited wrong command: id="build" value="jview"`、`reordered switch toggled wrong command "build"`；MD5 恢复、touch 后 **6 PASS**。旧 V/B/S 0/0/0。已看浅色前后全幅：蓝色输入边从错误的 Build 回到 JSON Viewer；深色开关环跟着原命令，几何不变。
