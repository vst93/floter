# P13-iy：快捷键表单的编辑与退出归属

开盘 a74fcfb5，pull 最新、工作区干净。实际上一轮 P13-ix，未按旧任务书重复 P13-hw。Codex 独立执行。读取 qiaomu-design 指定参考与 Apple DESIGN。开盘 done.log 不存在。

## Stash 清点

8 条 SHA 与上一轮相同，reverse-check 均退出 1。与 stash-backlog 和 P13-ix 表比对，是早期独立 WIP，没有新的 stale-kill；保留原件，不把旧主题/Tauri 链重放到 HEAD。

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

基线完成：**5206 PASS / 0 FAIL，3165 测试函数**。先执行候选事件探针；失败、通过与待拍板分别记录，不把探针通过算修复。

### 第一批：字面命令

1. 现象：保存命令行输入 `command` 并按 Enter，绑定消失，出现空命令草稿。改动：命令编辑提交直接更新命令值，菜单的 mode sentinel 只属于菜单。验证：深浅主题真实输入/Enter，`TestCustomCommandLiteralDoesNotBecomeMode` 旧代码均红，原串 `literal command became a mode switch: entries=[] draft=true`；修复后必须保持原键、保存字面命令、不出现新草稿。

其他探针：命令 Bind 焦点、模式往返文本、成功换键焦点、提交 trim/空值恢复、重复录键重试、草稿重建 Undo、动作选择焦点均原代码通过，均不计修复。原始日志 probe1/probe2。

需拍板：真实 Add→录 Z→另一保存 B 的动作菜单选择命令，已有 Z 被 B 覆写。现有设计仅有一个草稿槽；保留多个草稿、禁止转换或确认覆盖会改变交互，暂不修改，已实证且纳入收尾队列。

第一批相关 **128 PASS / 0 FAIL**、包 vet 净；M1 恢复旧命令提交分支后 **3 FAIL**，MD5 OK、touch 后 **3 PASS**。V/B/S **0/0/0**。转换覆盖的探针只留日志，不把“证明旧缺陷存在”的断言放入长期产品测试。

第一批 **45f3c096** 已推送。

### 第二批：同一行的命令与键帽

2. 现象：命令输入 `say unfinished` 尚未 Enter，改录 Z 后输入框回到 `say original`。改动：换键时命令草稿随原行迁到新键，仍只在 Enter 提交。验证：真实输入→录键→读编辑器→Enter；原串 `changing key discarded command draft: "say original"`，深浅均红。
3. 现象：720px 完整设置页的长组合键越出键帽，叠到垃圾桶与动作菜单。改动：键帽文字单行可收缩并省略，命中框与全量 accessible 文本保持。验证：真实长键文本右边界必须在删除控件之前；原图显示覆盖，原串 `key text overlaps remove: key={X:210.40001 Y:670.89996 W:164.00002 H:17} trash={X:300.4 Y:666.7999 W:25.200012 H:25.200012}`。

第二批相关 **11 PASS / 0 FAIL**、包 vet 净。M2 同时恢复两处旧分支后 **5 FAIL**，两文件 MD5 OK、touch 后 **12 PASS**（含 recorder 保护测试）。V/B/S **0/0/0**。
