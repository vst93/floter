# P13-im · 条件刷新后的键盘出口

开盘 f7beaee5，pull 已最新、工作树净。Codex 独立执行；qiaomu-design 主规则、偏好账本、craft-loop、preflight、chinese-typography 已读。done.log 开盘不存在。按最新 P13-il 交接继续。

## Stash 清点

八条 SHA 与上轮及 stash-backlog 一致；实际 reverse-check 退出码均为 1。旧 stale-kill 已被前轮接续归档，当前原件是历史已审核独立 WIP，不能拿旧主题/Tauri 覆盖现实现，无新增独有抢救工作。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap；reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider；reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针；reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段；reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段；reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键；reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏；reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri；reverse 1 | 保留 |

## 开盘复现

实测基线 **4758 PASS / 0 FAIL / 3044 函数**。八条候选在旧产品深浅事件均失败，原始 24 条 FAIL（含父测试）在 reproduce.log。脚本初始夹具 Tab 被代码编辑器当作缩进，改用方向键建立键盘态后重测；这个夹具问题不计产品缺陷。首次测试编译误写 i18n Back（应为 Close），已纠正，未改产品文案。

## 第一批（1–2）

1. 现象：选项 two 重排到首位后焦点留在旧索引 three，下一次 Right 把答案改为 one。改动：运行参数分段按选项值绑定身份。验证：重排后仍聚焦 two，Right 选择 three。
2. 现象：聚焦 two 被撤回后 Space 误选邻居 three。改动：语义身份接活已有分段组退出机制。验证：Space 不改变保留答案，Tab 回当前选项后可继续用方向键选择。

定向实测 **95 PASS / 0 FAIL**，settingsui/control vet 净。M1 换回未传语义 key 的产品路径后 6 FAIL（4 个深浅叶子），原串 `reordered choice focus moved to another value`、`withdrawn choice activated its replacement`。源码 MD5 一致、touch 后复验 **6 PASS**。旧 V/B/S 断言修改 0/0/0，新增 Skip 0。

## 第二批（3–4）

3. 现象：当前文字参数撤回后，焦点落空，Enter 无法退出表单。改动：表单记录自己的持焦字段，字段退场时回现有 Cancel。验证：参数删除后 Cancel 持焦，Enter 折叠；存活字段答案仍为 kept。
4. 现象：全部参数撤回后表单消失，原输入的焦点未回运行入口。改动：记录运行表单的焦点归属，空表单退场时回当前卡片的 Run。验证：刷新不运行、不清会话答案；Enter 才执行无参数运行一次。

定向 **75 PASS / 0 FAIL**，包 vet 净。M2 换回本批之前的产品代码实跑 6 FAIL，原串 `withdrawn parameter lost Cancel keyboard exit`、`empty form lost Run keyboard exit`；MD5 恢复、touch 后 **6 PASS**。V/B/S 0/0/0。

## 第三批（5–6）

5. 现象：宿主折叠运行表单后，原字段持焦位置消失，Enter 不能重开。改动：折叠时将焦点还给同卡片 Run。验证：刷新不执行命令，Enter 可重新展开且答案 mine 保留。
6. 现象：宿主撤回 RunWithParams 时表单与入口同时消失，键盘无出口。改动：复用既有页头 Close 作为可用回退点。验证：Close 获焦，刷新不关闭页面、不运行、不删除答案。

定向 **81 PASS / 0 FAIL**，包 vet 净。M3 实跑 6 FAIL，原串 `closed form lost Run keyboard exit`、`withdrawn Run action lost page keyboard exit`；MD5 恢复、touch 后 **6 PASS**。V/B/S 0/0/0。
