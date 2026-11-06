# P13-hz：确认退场后的可用键盘出口

开盘 6bcc6dd7；pull 最新且树干净。Codex 独立执行，沿最新 P13-hy 交接。规格已在产品修改前落盘；中文 locale 基线实测 4242 PASS / 0 FAIL / 2921 函数。

## Stash 清点

本轮逐条 reverse-check 均退出 1；SHA 与已审核 backlog 相同，早期非 stale-kill WIP 继续保留。不反复合入旧主题/Tauri，已接续抢救仍在归档引用中。

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

## 增量实施

1. 现象：丢弃问题打开后 owner 变 tool，问题撤走但 Open configuration 失焦，Space 派发 0 次。改动：工具配置入口返回可用元素，退役丢弃问题把焦点交给它。验证：深浅真实编辑/提问/刷新后入口持焦，Space 仅运行当前 settings 命令一次；原 edited 草稿、保存 0 次。
2. 现象：丢弃问题打开后目标变 Orphan，Adopt 不接键盘。改动：孤立目录分支补丢弃问题的交接。验证：深浅相同事件后 Adopt 持焦，Space 认领一次，破坏操作/保存 0 次，Escape 退出一次。

不抢焦点对照采用真实 Shift+Tab 移到页头 Close，再刷新后保持 Close 并用 Space 退出一次；页头 tooltip 可以先消费 Escape，此处不把它误判成确认吞键。侧栏点击/Tab 探针未得到持焦状态，夹具改用明确可聚焦的页头出口。早期 mutation 还原测试被上述夹具失败阻断，修正后重新完整跑 M1，不把早期结果计作通过。

第一批相关 199 PASS / 0 FAIL，settingsui vet 净。M1 回注旧 settingsui.go 后四条答案内场景红：`retired discard-owner lost available keyboard outlet "Open configuration"` / `retired discard-orphan lost available keyboard outlet "Adopt"`；还原 MD5 OK、touch 后 199 PASS（含本轮 10 PASS）。旧 V/B/S 0/0/0。

第一批 0fe3891c。第二批：

3. 现象：配置 schema 清空，丢弃问题退场、Configure 消失，Check 未接键盘。改动：非 host 配置分支在没有有效配置入口时交接给可用 Check。验证：深浅真实编辑/提问/刷新，焦点与 Space 派发 Check 一次，破坏操作/保存 0 次。
4. 现象：owner 变 tool 但没有 openCommand，只剩禁用配置入口；丢弃答案焦点悬空。改动：只返回可用配置入口，无入口时同样交接 Check。验证：深浅 Check 持焦、Space 一次，edited 草稿保留，禁用配置不派发；两项均有页头已持焦的不抢焦点对照。

规格修正：settingsui.go:1065 明确侧栏为 click-only，不能作为兜底而改其键盘模型；后四项无可用同行出口时转交已有页头 Close。此次属于退场后恢复出口，不改变任何键位。

第二批 2c5211c8 已推送；相关 209 PASS，vet 净。M2 四场景实跑红 `retired discard-schema lost available keyboard outlet "Check"` / `retired discard-unopenable lost available keyboard outlet "Check"`，MD5 恢复 OK，touch 后相关 209 PASS。

5. 现象：停用问题打开时目标变 Orphan 且 Adopt 未接线，答案与原开关消失，焦点悬空。改动：保持已可用 Adopt 优先，无该入口时用已有页头 Close。验证：深浅刷新后 Close 持焦、Space 关闭恰好一次，删除/停用/保存 0 次。
6. 现象：移除问题同样在无 Adopt 的 Orphan 状态失去键盘出口。改动：同族安全出口覆盖移除带。验证：深浅答案与用户已在页头两条路径都只关闭一次，不派发任何移除；旧问题退场。

第三批 b860ef46 已推送；相关 219 PASS，vet 净。M3 回注后四场景实跑红，`retired disable-no-adopt lost available keyboard outlet "Close settings"` 与 remove 同串；MD5 两文件恢复 OK，touch 后相关 219 PASS。

7. 现象：组件卸载浮层因目标变 Orphan 退场，但没有 Adopt，浮层 Cancel/组件勾选里的焦点无处返回。改动：浮层原 handle 与现有安全出口连接。验证：深浅 Cancel/checkbox 路径，Close 接焦、Space 退出一次，遗留选项已清、卸载调用 0 次。
8. 现象：停用目标变 Broken，开关禁用且 Check 未接线，原问题退场后键盘悬空。改动：所有退役问题统一只选择有效可用 opener，否则现有页头 Close；用户已移焦时不接管。验证：深浅刷新后 Close 持焦，Space 退出一次、停用/删除/保存 0 次。

最终截图前加入真实 Tab 进入键盘模式，保证焦点环可见；八条全部在开盘源码重新跑 before，与最终源码同事件重拍 after。
