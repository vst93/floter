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
