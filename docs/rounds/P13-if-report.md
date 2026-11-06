# P13-if · 回调与表单刷新后的结果归属

开盘分支 mygo-rewrite，HEAD d89ef7ec，pull 最新、工作树干净。首个记录时钟 2026-11-06 22:26:35 UTC。实际承接 P13-ie；done.log 开盘不存在。

## Stash 清点

与 backlog 和 P13-ie 的 SHA 一致，无新增 stale-kill。八条反向检查退出 1；已审核的旧独立 WIP 继续保留，不重入旧主题/Tauri 链。

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

## 验证记录

基线运行中；旧 V/B/S 修改 0/0/0，无新增 Skip。后续逐批追加实测记录。

## 第一批（1–2）

1. 现象：点击运行，回调原地把 next 选项换成 fresh 后仍显示旧约束下的拒绝。改动：参数选项在派发之前复制，后续刷新识别条件变化。验证：真实按钮点击，深浅 fresh 已出现、旧拒绝消失，运行总次数 1。
2. 现象：运行回调已把答案 old 修正成 next，旧拒绝却绑定 next。改动：提交答案保存独立快照，既不读取回调改过的 live map，也不读取回调拥有的参数 map。验证：深浅真实点击，next 保留，旧拒绝消失。

相关事件 **82 PASS / 0 FAIL**，settingsui vet 退出 0。M1 撤回两项产品修改，深浅四叶子实红：`options callback left obsolete refusal on current answer` / `value callback left obsolete refusal on current answer`；恢复后 MD5 OK，touch 后 82 PASS。目视浅色前后：fresh 已在选择器中，修后底部过期红字消失，操作行位置不变。

## 第二批（3–4）

3. 现象：参数全部移除再恢复，字段重新出现时旧运行拒绝也复活。改动：集成行仍让 runForm 处理退场，空表单清理拒绝与显示错误，答案记忆不动。验证：真实提交失败 → 空参数帧 → 恢复；深浅无旧红字，remembered 答案保留，未自动运行。
4. 现象：RunWithParams 撤回再接上，恢复入口仍携带之前的失败。改动：无运行回调时同步退役拒绝。验证：真实提交失败 → 回调撤回 → 恢复；深浅无旧红字且答案不丢。

相关事件 **88 PASS / 0 FAIL**、settingsui vet 退出 0。M2 四叶子实红：`empty form resurrected retired refusal` / `unwired form resurrected retired refusal`；MD5 OK、touch 后 88 PASS。第三批测试初稿误用 KeyReturn 导致一次编译失败，修正为 KeyEnter 并暂存该未完成测试后复跑；未将编译失败算为 mutation。

## 第三批（5–6）

5. 现象：同一 Config 对象在写回调里换 owner 和值，旧返回仍回滚或覆盖新 owner 的值，甚至显示旧失败。改动：返回核对派发 owner 与当前 schema 是否存在。验证：真实开关点击，成功/拒绝 × 深浅四场景保留新 owner 的 false/25，不显示旧失败。
6. 现象：同 key 改型、移除、数字边界或选项值原地更新后，旧写返回覆盖新字段的值。改动：派发前复制字段选项，返回核对 Kind、Min/Max/Step、Options 值；标签更新不作为失效条件。验证：开关、数字输入及下拉真实事件，四种刷新 × 成功/拒绝 × 深浅，旧结果不进入新字段；改型后再输入提交以核对输入缓冲。

M3 撤销产品核对，20 个深浅/返回叶子实红；原串 `new owner values overwritten`、`new field kind values overwritten`、`removed field resurrected by result`、`new bounds values overwritten`、`new options values overwritten`；MD5 OK、touch 复验。夹具曾把数字标签当输入入口，调整到实测输入 x=660；HasText 不收集 TextInput 缓冲，改用输入/提交事件核对实际内容，未弱化旧断言。目视深色前后：改型输入由空白恢复“当前配置”，上限 10→25，错误提示退场。

开盘完整实测 **4541 PASS / 0 FAIL / 2979 测试函数**；普通/verbose 最终全量尚待轮末运行。
