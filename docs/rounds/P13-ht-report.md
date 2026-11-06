# P13-ht：集成动作退出与刷新后的恢复

开盘 `a1674426`，pull 最新，工作树干净。基线实测 **4007 PASS / 0 FAIL / 2870 测试函数**。done.log 开盘不存在。

## Stash 清点

活动八条 SHA 与 P13-hs 完全相同，逐条 reverse-check=1，属于早期独立 WIP；抢救 stale-kill 已有归档/接续记录，不重复试合，不覆盖当前 Go/主题实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实施记录

候选与真实行号见规格；下面随批次记录实测，未完成项不冒充修复。

### 第一批：参数随清单同步

1. 现象：表单已开后新增格式/数量参数，默认 `json`/`12` 没有显示。改动：按当前参数定义补齐缺少的默认答案，保留存量手改值。验证：深浅真实帧，旧实现报 `new parameters did not receive defaults without replacing edits: map[limit: target:my edit]`；修复后 target 仍为 my edit，format=json、limit=12。
2. 现象：删除参数后运行仍携带旧 key，重新加入也复活旧答案。改动：按当前完整 ID 集清除退役答案（含无默认值参数，不能用 defaults 的 key 集代替 schema）。验证：真实点击 Run 只派发当前 target，重加采用 fresh；旧实现报 `removed parameter reached command: map[obsolete:old target:current]` 与 `re-added parameter resurrected old answer: "old"`。

V/B/S 0/0/0，旧测试零改。M1 用 HEAD 产品回注两缺陷，红/绿原始日志与 MD5 校验落轮次目录；深浅新增参数前后图已导出。

第一批 `364ef2b8` 已推送；run 相关回归 69 PASS，M1 缺陷注入退出 1、恢复后 6 PASS，settingsui vet 净。

### 第二批：编辑与已提交答案分开

3. 现象：文字参数输入 24 后改成数字，Cmd/Ctrl+Z 将答案撤销回旧的 8。改动：参数编辑器身份包含 kind，改型时退役旧编辑历史，值仍保留 24。验证：两主题真实键盘事件；旧实现 `new number editor resurrected text undo: "8"`，新实现撤销不再引入旧答案；原参数重排保焦回归仍通过。
4. 现象：运行回调拿到表单 map，后续输入 second 把已提交 first 改掉；回调修改 map 又污染当前输入。改动：提交时拷贝字符串答案 map。验证：真实 Run 后继续编辑，命令快照仍 first、表单仍 second，派发恰好 1 次；原红串 `later edit rewrote submitted command: map[target:second]` / `command map mutated live form: "host-owned"`。

V/B/S 0/0/0。M2 回退本批产品变动，实际红；恢复后 MD5 校验、touch 复跑，原始日志随本批入库。
