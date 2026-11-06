# P13-ia：异步入口随条件刷新消失时的键盘交接

开盘 a7f1fb39，pull 最新、工作区干净；当前最新交接为 P13-hz，未重做 hx–hz 已交付项。Codex 独立执行，无外部模型。done.log 开盘不存在；结案时新建。

## Stash 清点

八条 SHA 与已审核 backlog 相同，全部 reverse-check 退出 1；它们是早期独立 WIP，不是未接续的 stale-kill。按已归档处置保留，不覆盖新主题或恢复旧 Tauri。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 增量记录

规格先于产品修改落盘。原确认在飞路径与普通退场已有覆盖，本轮转向保存、修复和启用的异步入口；以下只登记真实复现的缺陷。

1. 现象：键盘保存尚未返回时 owner 变 tool，保存按钮消失，Space 没有作用。改动：保存入口记住原焦点句柄，交给当前可用工具编辑入口。验证：深浅真实编辑、Tab、Space、owner 刷新、失败回调；原实现报 `pending save-owner lost keyboard outlet "Open configuration"`，且派发 0 次；用户预先移焦页头的对照通过。修后结果待本批日志。
2. 现象：同一保存中 schema 被清空，当前 Check 未接键盘。改动：沿用已存在的可用操作选择链，把保存入口交给 Check。验证：同事件与 schema 清空，断言当前 Check 的焦点和一次派发，不重复保存；修后结果待本批日志。

第一批相关 **74 PASS / 0 FAIL**，settingsui vet 退出 0。M1 撤回保存句柄，四个 entry 场景实跑红（owner/schema × 深浅），页头已持焦的对照仍通过。原串：`pending save-schema lost keyboard outlet "Check"`；恢复 MD5 与 touch 复验见 M1-restore.txt。浅色原图中“打开配置”原本没有键盘轮廓，修后出现完整 accent 环；没有改变布局。

第一批 **7b9008b6** 已推送。开盘全量基线结束：**4282 PASS / 0 FAIL / 2929 测试函数**，退出 0。

3. 现象：保存仍在飞时集成变 Orphan，host 表单撤下，Adopt 没接住键盘。改动：孤立集成分支也归还原保存/表单焦点。验证：深浅真实保存后刷新，旧实现报 `pending save-orphan lost keyboard outlet "Adopt"`；迟到失败不重开表单、不重复保存，用户已移到页头则保持。
4. 现象：保存期间按原契约可以继续编辑，用户回到 Token 字段后 owner 变 tool 且无编辑命令，保存按钮之外的焦点仍丢失。改动：记录整张 host 表单的 FocusWithin，把字段焦点也交给当前 Check。验证：真实 Shift+Tab 回字段再刷新；旧实现报 `pending save-unopenable lost keyboard outlet "Check"`，不是直接篡改焦点状态。草稿保持 edited，禁用“打开配置”不执行。

第二批相关 **84 PASS / 0 FAIL**、settingsui vet 退出 0；旧 V/B/S 断言零改。四项深浅前后图随本批保留，基线 pin 不修改。M1 已实跑，新增整张表单与孤立分支将在最终 M2 一并回注核验。

第二批 **d1f6d196** 已推送。修复“回调与健康刷新同时抵达”的候选实测全部通过，保留为 settled-repair-control.txt，不算 bug。缺陷只在刷新先到、回调仍未返回的真实顺序出现。

5. 现象：Recheck 在飞期间 Broken 已被刷新解除，入口消失，同行 Check 仍禁用，键盘无可用出口。改动：保留修复按钮句柄，在它本帧不再构建时选当前可用出口，全部忙时回页头。验证：深浅真实 Recheck → 健康刷新 → 两帧 → 延迟失败；原串 `pending repair-healthy lost keyboard outlet "Close settings"`。
6. 现象：Reconnect 在飞期间运行时已刷新可用，同样失去按钮与可用键盘位置。改动：重新连接的静息/忙态都绑定同一交接句柄，入口撤下时使用同一可用性检查。验证：深浅真实 Reconnect → runtime 可用刷新；原串 `pending reconnect-healthy lost keyboard outlet "Close settings"`。两项均保留页头原焦点，回调后不重复修复、不抢焦。

第三批相关 **94 PASS / 0 FAIL**，settingsui vet 退出 0；焦点交接立即完成，回调只结算原操作，不承担延迟抢焦。V/B/S 旧断言修改仍为 0/0/0。
