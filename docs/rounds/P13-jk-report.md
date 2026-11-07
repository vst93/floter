# P13-jk · 刷新后的配置与状态不倒退

起点 8213e4d2，分支 mygo-rewrite，ff-only pull 已完成，工作区干净。唯一执行者 Codex，沿用 qiaomu-design 既有方向打磨流程。本文增量记录，不把计划写成完成。

## Stash 清点

活动清单无 stale-kill。以下 SHA 与 P13-jj 及已登记的早期独立 WIP 一致；reverse-check 全部退出 1。保留旧稿，不把旧主题、旧 Tauri 发布链重新合入。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | keycap WIP，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 设置间距/slider WIP，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载度量/scratch WIP，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 插件数字框 WIP，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 插件样式 WIP，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 快捷键 WIP，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 动作栏 WIP，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | main/Tauri WIP，reverse 1 | 保留 |

## 增量记录

已核对最新报告的外部导入刷新线索。配置/集成状态缓存的后台发布未区分失效前后代际，开始构造真实事件复现。没有改终端持久化语义或需拍板项。

基线实测 **5627 PASS / 0 FAIL / 3294 测试函数**，退出 0。

1. 现象：配置已成功导入，旧后台读取晚到，输入框从 imported.example 退回 before.example。改动：缓存记录发布代际，导入失效或新读取落地后拒绝旧代发布。验证：真实导入动作、磁盘与实际输入框复制读取；深浅修前均红，修后地址保持当前值。
2. 现象：保存成功后，早先失败读取晚到，已打开的 endpoint 输入框变空。改动：保存失效同步退役旧后台读取，新保存代际不可被空失败结果覆盖。验证：真实 SaveConfiguration 动作、磁盘不变而 UI 旧实现读空；深浅修后保留 saved.example。

第一批相关 **8 PASS / 0 FAIL**、shell vet 净。M1 撤销发布保护实红：`retired read replaced current value`、`editor displays "", want "https://saved.example"`；逐字节 MD5 OK、touch 后复验即上述 8 PASS。旧 V/B/S 断言修改 0/0/0。早期夹具误点了内置剪贴板同名入口；已隔离无关插件入口，正式证据为 M1-red.txt 与 batch1-test.txt。

第一批 **71d7de39** 已提交推送。

3. 现象：刷新已确认运行时可用，较早失败探测晚到，又画出 Tool unavailable。改动：集成状态缓存只接受当前代际后台结果，刷新和单行新事实都会退役旧结果。验证：注入受控旧探测结果，经真实 RefreshIntegrations 与真实卡片渲染，深浅修前 `retired runtime failure disabled a repaired integration`，修后可用状态保持。
4. 现象：当前批准摘要与磁盘一致，旧摘要回调仍会重新显示“manifest has changed”告警。改动：同一状态缓存代际保护批准摘要。验证：真实仓库批准记录、实际 manifest digest 与整页渲染，深浅修前 `retired digest revived approval warning`，修后摘要保持当前值。

第二批相关 **15 PASS / 0 FAIL**、shell vet 净；旧 V/B/S 断言 0/0/0。探测的旧结果用受控注入重现返回顺序，不声称实际卸载了系统运行时。亲眼复核深色旧图：Running 旁错误出现 Tool unavailable；批准卡底部多出过期告警。浅色导入对照明确看到 endpoint 从 before.example 改为 imported.example。

第二批 **bb1e398a** 已提交推送。第三批按同一校验路径调整实现顺序，先一起处理数字与文字，保存错误和范围锚点留第四批。

5. 现象：数字 15 超过上限 10，Enter 已出现错误；外部配置改为 20 后，错误反而消失。改动：已校验字段被外部值替换后，用当前值重算现有边界提示。验证：深浅实际 Enter 和配置刷新，修前 `external invalid replacement erased the current field problem`，修后 20 仍提示、更新为 5 后自然恢复。
6. 现象：已校验的过长文字换成另一段过长外部值，长度提示消失。改动：沿同一替换路径重新检查当前长度。验证：深浅 Enter、长文替换及恢复为 ok；不强制校验从未编辑/提交的字段，不改保存验证时机。

第三批 **21adb46f** 已提交推送，相关 **20 PASS / 0 FAIL**、settingsui vet 净。

7. 现象：保存失败后，外部导入/写入已确认完全相同的草稿，界面读 Saved 但仍留旧保存失败条。改动：本地改动被外部实际值接受且整个草稿不再有差异时，撤掉这条旧失败；不触发保存或新成功通知。验证：真实输入、点击 Save 失败、配置值刷新，深浅修前 `external acceptance left an obsolete save failure`。
8. 现象：选 alpha 后外部确认相同选择，再 Shift 点 gamma，会连 beta 一起带上。改动：外部接受当前多选值时退役旧范围锚点。验证：真实点击与 Shift 点击，修前 `[alpha beta gamma]`，修后 `[alpha gamma]`。首版四选项夹具最后一行在滚动区外，未派发点击；正式三选项事件与 batch4-red.txt 已排除该夹具错误。
