# P13-ie · 表单拒绝与动作确认的条件刷新

开盘 2cfe705d，pull 最新、工作树干净。实际承接 P13-id。首个时钟 21:44:03 UTC。done.log 开盘不存在，结束创建。已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple 供体。

## Stash 清点

八条均与交接账本及上轮 SHA 一致，没有新增 stale-kill；reverse-check 全部退出 1。它们是已审核的独立旧 WIP，不能据反向应用失败覆盖当前主题或 Go 发布链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap；reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider；reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局；reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段；reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段；reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键；reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏；reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri；reverse=1 | 保留 |

## 验证记录

完整基线正在运行；后续只写实测数字。旧 V/B/S 断言暂为 0/0/0，无新增 Skip。

## 第一批（1–2）

1. 现象：运行拒绝后改正 Target，旧红字仍说参数无效。改动：拒绝记录绑定提交值；该值改变后退役，编辑 Note 保留 Target 的错误。验证：真实点击运行、点击输入、Cmd+A/输入；深浅均通过，刷新派发 0 次。
2. 现象：被拒绝的 Target 移除后，底部仍责备不存在的字段，重加还会复活。改动：拒绝随该参数值槽退役。验证：真实运行失败后移除/重加，错误消失且默认值恢复；深浅通过。

定向回归 **65 PASS / 0 FAIL**，settingsui vet 退出 0。M1 用 HEAD 原产品注入两项缺陷，四个深浅叶子均红；原串 `edited parameter left obsolete refusal visible`、`removed parameter left obsolete refusal visible`、`re-added parameter resurrected refusal`。cp 还原后 `md5sum -c` 为 OK，touch 后以上 65 PASS。最初测试夹具有语法及辅助函数重名，两次编译失败均修正后才取得实红，未把编译失败算 mutation。

目视浅色 before：输入已经是 fixed，底部仍有红色旧错误；修复只撤掉失效反馈，不改变输入框和操作行。

## 第二批（3–4）

3. 现象：同一参数从自由文字改为可选值，原来的后台拒绝仍挂在新控件下面。改动：拒绝快照保存类型、必填、参数旗标与选项副本；校验条件变化即退役。验证：运行拒绝 → 原值仍在的 Select schema → 不留旧错、不自动运行，深浅通过。
4. 现象：同一参数改名后，错误仍点名已不存在的 Target。改动：保存结构化拒绝，按当前字段标签与语言重新解释，保留错误本身。验证：Target → Destination 后只出现当前名称的拒绝，深浅通过。

定向回归 **71 PASS / 0 FAIL**，settingsui vet 退出 0。M2 撤回这批产品后四叶子实红：`constraints parameter left obsolete refusal visible`、`renamed parameter left obsolete refusal visible`、`renamed parameter lost current refusal wording`；MD5 OK、touch 后以上 71 PASS。旧 V/B/S 0/0/0。

## 第三批（5–6）

5. 现象：已问“确认清理”后，schema 把范围改成“包括收藏”，同 key 仍处于已确认第一步的状态。改动：确认绑定 plugin/key/label/help/confirm 的值副本，刷新或派发前不匹配即退役。验证：真实行点击打开确认，原地刷新问题，回到首按入口、自动执行 0 次；深浅通过。
6. 现象：动作拒绝后更新范围，同一行仍显示旧动作的失败。改动：拒绝也绑定动作定义，定义变化退役。验证：行点击 → 确认点击 → 拒绝 → 刷新范围，旧失败消失、执行总数仍为 1；深浅通过。

定向回归 **141 PASS / 0 FAIL**，settingsui/launcher vet 退出 0。M3 四叶子实红：`changed question inherited old confirmation`、`changed action inherited old refusal`；MD5 OK、touch 复验通过。初版完整启动器夹具 Tab 留在搜索入口，未到动作，已改真实行点击；该夹具失败不算变异证据。目视深色 after：新范围说明下只剩“清理历史”首按胶囊，旧问题与取消按钮退场，无新增面。

开盘全量 **4507 PASS / 0 FAIL**，起点 2cfe705d 测试函数 **2970**。基线 shell 收尾统计一度读到已新增的函数数 2974，该值不是起点；以 git grep 起点树实测 2970 为准。

## 第四批（7–8）

7. 现象：动作回调中更新同 key 的范围，再返回拒绝，新动作却显示旧失败。改动：返回时核对派发前动作定义，结果只归原定义。验证：真实确认点击触发回调内刷新，界面显示新范围，旧失败不进入；深浅通过。
8. 现象：动作回调已发起新一轮确认，旧拒绝仍写进同一行；取消新问题后会露出旧失败。改动：确认记录递增代次，返回只写到原代次。验证：真实确认 → 回调重开问题 → 点击取消，旧失败不复活；执行次数严格为 1，深浅通过。

M4 撤回返回归属校验后四叶子实红：`replaced action inherited callback's obsolete refusal`、`rearmed action inherited callback's obsolete refusal`；MD5 OK、touch 复验。定向回归 **147 PASS / 0 FAIL**，两包 vet 退出 0。V/B/S 0/0/0；每两项独立提交推送，不攒到轮末。
