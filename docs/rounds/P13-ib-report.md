# P13-ib：整行退场后的键盘归还

开盘 4f62e976，分支 mygo-rewrite，pull 最新、工作树干净；03:02 +08:00 开始。Codex 独立执行；本机 done.log 开盘不存在。最新遗留是 P13-ia 的目标消失/重现，已用真实事件证实，未重做仍存在目标的刷新。

## Stash 清点

八条与上轮 SHA 相同，reverse-check 全部退出 1。它们是已审核早期独立 WIP；stale-kill 已接续归档，继续保留，不重放旧主题与旧 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实测复现与第一批

八项深浅 entry 全红，16 个 elsewhere 对照全部通过；原始 reproduce.txt 落盘。首批只接配置两类，后续按组件成对接入，避免攒到轮末。

1. 现象：保存中整行消失，键盘没有出口，同 ID 返回后 Space 再派发保存。改动：在列表进入空态前，比对前后 ID，原表单持焦则归还页头，并退役其 UI 句柄。验证：编辑 Token → Tab/Space 保存 → 消失 → 重现 → 延迟失败 → Space；旧实现报 `missing save row lost keyboard outlet "Close settings"` 与 `return or callback repeated the operation`。修后结果待本批日志。
2. 现象：丢弃确认虽已随目标消失退役，却遗失键盘。改动：同一消失检测归还丢弃答案焦点，保留草稿。验证：真实编辑 → Configure 开问题 → 消失/重现；旧实现报 `missing discard row lost keyboard outlet "Close settings"`。用户已移到页面复选框的对照不抢焦；修后结果待日志。

V/B/S 旧断言修改 0/0/0，无新增 Skip。全量基线运行中，不提前填写 PASS。

第一批相关 **268 PASS / 0 FAIL**、settingsui vet 退出 0。开盘全量实测 **4322 PASS / 0 FAIL / 2937 测试函数**。M1 撤销 save/discard 的交接后四个深浅 entry 场景红，elsewhere 保持；原串如上。恢复后 MD5 核验与 touch 复验随日志落盘。浅色原图对照已目视：空态不变，右上返回键从没有焦点变为完整蓝色 2px 环，并出现原有快捷键提示。

## 第二批：修复入口

3. 现象：Recheck 在飞时整个损坏工具行消失，键盘滞留在不可见按钮；同 ID 回来后旧按钮重新持焦。改动：列表差分也退役 repair 的焦点句柄，将持焦入口交给返回键。验证：真实 Space → 列表变空 → 同 ID 重现 → 延迟失败；旧串 `missing repair row lost keyboard outlet "Close settings"`，不重派 Recheck。
4. 现象：系统拥有的工具 Reconnect 在飞时退场，同样丢失当前可用出口。改动：连接修复和重新检查共享既有 repair 焦点归属，两个入口均在行消失时归还。验证：运行时不可用 → Reconnect → 消失/重现 → 延迟失败；旧串 `missing reconnect row lost keyboard outlet "Close settings"`，主动移焦对照不受影响。

第一批 **a7d619df** 已推送，MD5 还原后 touch 复验 **10 PASS**。第二批相关 **278 PASS / 0 FAIL**、包 vet 净；M2 撤销 repair 交接后四个深浅 entry 红，elsewhere 全绿。还原、touch 复验与图片随本批落盘。V/B/S 旧断言仍为 0/0/0。

## 第三批：启停入口

第二批 **55a34832** 已推送，M2 还原复验 **10 PASS**。

5. 现象：启用写回未返回时整行消失，键盘失效；重现后旧开关焦点可能重新接管。改动：退役失踪目标的 toggle UI 句柄，真实持焦时归还页头。验证：关闭的开关 → Tab/Space 启用 → 消失/返回 → 失败；旧串 `missing toggle row lost keyboard outlet "Close settings"`，整个序列只写入一次。
6. 现象：停用答案已派发、仍处忙态时目标消失，在重新出现之前没有可用焦点。改动：同样归还 disable 答案焦点，既有 busy 与回调保持。验证：点击开关开问题 → Tab/Space 答案 → 整行退场；旧串 `missing disable row lost keyboard outlet "Close settings"`，失败后原动作结算、页头仍可键盘关闭。

第三批相关 **288 PASS / 0 FAIL**、包 vet 净。M3 暂时撤去 toggle/disable 交接，四个 entry 场景红，已有页面焦点的对照不受影响；恢复后 MD5 与 touch 复跑结果随本批保存。图片继续使用原始 Tester 事件。
