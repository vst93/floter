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

第四批 **c77d70b3** 已提交推送，原日志为 **31 PASS / 0 FAIL**、settingsui vet 净。该提交正文误写 28 PASS，此处按原日志纠正，不重写已推送历史。

第二轮四视角：艺术总监看见批准卡底部不再被旧告警占据；工程师增加“空数字值上还有 1e 半输入”反例；任务用户需要知道仍未完成的输入不能算整表成功；读者需要过长/越界提示继续贴着当前字段。反例实红 `accepting one field retired the unfinished form's failure or input`，原因是 nil 数值相等不能代表半输入完成。补上 live 草稿为空条件，保持原半输入与错误，不新增修复项凑数。

第二轮修正 **b5ae3106** 已提交推送，相关 **15 PASS / 0 FAIL**、settingsui vet 净。M2 注入撤销字段重验、已确认错误退役与范围清理，四项深浅均实红；原串 `external invalid replacement erased the current field problem`、`external acceptance left an obsolete save failure`、`accepted external selection retained old range: [alpha beta gamma]`。MD5 OK、touch 后 **15 PASS / 0 FAIL**。M1 与 M2 原日志和校验清单随仓库保存。

## 偷了什么

- craft-loop 工艺菜单 **#4 签名交互**：刷新到显示新增延时 **0ms**，原缓存 **2s TTL** 保留，已失效结果不再倒退屏幕。
- **#6 编辑细节**：外部确认整表后旧保存错误条 **1→0**，有效的越界/过长提示仍在字段的原 caption 行；没有再造说明卡。
- **#12 材质细节**：原控件 **1px** 边与受光边沿用，新增玻璃 **0层**。
- **#5 多层阴影**：浮面的 **2层** 投影沿用，不给字段或卡内提示添加外影。以上沿用值不冒称本轮改色。

Apple DESIGN 的中性颜色角色、按钮 **8px** 圆角、浅色投影 **0.22 alpha** 对应原 token；中文系统字体优先。用户指定只打磨已有功能，未为凑五种装饰加入纹理/大字/动画。为什么更轻、更有空间感：过期批准条和保存条撤掉后，表单只保留当前需要处理的一层反馈，原面差有了可读空间。

## 图与目检

八项 × 深浅 × 前后共 **32 张原图**，另有 **16 张对照板**，均在 `P13-jk/`；原图来自 ui.Tester，路径 `/tmp/floter-P13-jk-<场景>-<light|dark>-<before|after>.png`。对照板只裁掉窗口空白，原图保留。代表：[浅色导入](P13-jk/import-late-values-light-comparison.png)、[深色批准](P13-jk/approval-late-result-dark-comparison.png)、[浅色旧保存条](P13-jk/external-save-error-light-comparison.png)、[深色数字校验](P13-jk/external-number-validity-dark-comparison.png)、[浅色多选](P13-jk/external-selection-range-light-comparison.png)。

目检变化：导入图 endpoint 旧地址变为当前地址；深色运行卡原 Running 旁的 Tool unavailable 消失；批准告警条消失后配置行上移；保存失败条消失后按钮行上移；数字 20 保留红色边界和原错误 caption；多选 beta 由错误打勾变为空框，alpha/gamma 不变。大小分别为设置整页 960×1800/960×1400、真实配置组件 720×320。移动端不适用桌面自绘程序；未冒称 Windows/macOS 真机验证。

## Preflight 逐项

| 项 | 结论 |
| --- | --- |
| A1 紫蓝渐变/纯黑/霓虹/渐变字 | 未新增；原语义 token 保留 |
| A2 非系统字体偏好 | 不采用；中文系统字体与用户既有合约优先 |
| A3 斜体 | 未新增，截图无斜体 |
| A4 Hero 构图 | 不适用，设置页无 Hero |
| A5 三等分/卡片海 | 未新增卡片，布局沿用 |
| A6 占位假数据 | 仅测试夹具使用 .example 地址，不进入产品默认数据 |
| A7 空洞文案/感叹号 | 无新产品文案 |
| A8 编号眉批 | 未新增 |
| A9 H1 行数 | 原 Integrations 单行标题保留 |
| A10 Bento 空洞 | 不适用，无 Bento |
| A11 左侧竖线 | 未新增；既有批准警告强调边沿用，本轮仅退役无效条，未冒称全仓无竖线 |
| A2.1 执行者 | Codex 单执行者，无外部模型/子代理 |
| A2 工艺/两轮 | 四视角和半输入反例已实跑；少装饰优先于凑满五项 |
| B1 读取与拨盘 | 2/1/6，既有布局、即时交接、原密度 |
| B2 明暗主题 | 深浅独立 Tester，无混主题 |
| B3 记忆点 | 当前值、当前状态不被旧读取倒退 |
| B4 动效气质 | 高频刷新不新增动画 |
| C1 目的/频率 | 状态更新等待 0ms |
| C2 all/ease-in/scale(0)/fade | Go 自绘，未引入这些效果 |
| C3 时长/按压 | 原控件按压保留，无新时长 |
| C4 出现 origin | 没有新增浮窗/弹窗 |
| C5 reduced motion | 实际截图设置 ReduceMotion=true |
| C6 hover 媒体条件 | 无 CSS，关键信息不依赖悬停 |
| C7 属性 | 未新增布局动画 |
| C8 重触发 | 刷新直接交接，未重播关键帧 |
| C9 手势 | 不适用，无手势改动；Shift 仍为现有模型 |
| C10 Framer Motion | 不适用，无新依赖 |
| C11 慢放 | 无新动画，不冒称真机帧率/慢放验证 |
| D2-1 字体 | 中文使用既有系统字体，组件图可见中文标签 |
| D2-2 ≥14px/1.5/字重 | 通用建议未全满足；原 Caption 9.9、正文10.8及行盒仍为拍板项，遵用户禁改边界 |
| D2-3 混排空格 | 无新产品中文文案；测试标签使用现成中文 |
| D2-4 标点/对比度 | 原 i18n 标点沿用；未重审全套 AAA，不声称所有次要墨达到7:1 |

缺陷已沉淀为本轮事件 pin 与 §6.238；不把一次修复写为全局审美规则。当前用户边界优先于 skill 通用全项通过措辞，以上不适用/受限项明确列出。

## 仍待

1. 下一轮首选：同步首读/新增行补读与外部失效交错时，是否仍可能把已过期结果交给当前帧。只有代码线索，先通过真实表面复现；不做纯内部缓存整理、不重做本轮已修的后台晚到路径。
2. 外部值接受期间的保存中草稿归属仍按 161-4/176-9 拍板项，不借清旧错扩大权限。重复新包覆盖、模板失败补跑、未知多选/Radio、字号/行盒、accent 禁用、终端 S1b/S1c/S2 尾沿用。
3. Linux 自绘、真实 provider/磁盘/配置动作与控制返回顺序已验；原生选择器和 Windows/macOS 运行时未验。八条早期 WIP 原样保留。

## 完整门槛与交接

最终产品 **b5ae3106**。全量普通测试退出 **0**；verbose 实测 **5654 PASS / 0 FAIL**（基线5627，增加27），退出 **0**；函数 **3303**（基线3294，增加9）。gofmt 输出为空；linux/windows/darwin 的 vet 与 build 六项均退出 **0**。最终源码 MD5 六文件全部 OK，旧测试断言 V/B/S 修改 **0/0/0**，新增 Skip **0**。

证据：[门槛汇总](P13-jk/gates.txt)、[普通全量](P13-jk/full-test.txt)、[verbose全量](P13-jk/full-verbose.txt)、[函数计数](P13-jk/test-functions.txt)、[最终MD5](P13-jk/final-source.md5)、[M1实红](P13-jk/M1-red.txt)、[M2实红](P13-jk/M2-red.txt)、[M2还原](P13-jk/M2-restored.txt)。

四批每完成两项先定向回归/vet，再立即提交推送；第二轮半输入反例另作修正提交。全量门槛在最终产品与测试源码上集中执行一次普通、一次verbose；没有改旧 B 断言或靠容差/Skip消红。开盘 18:59 UTC，门槛完成 2026-11-07 19:23:22 UTC，低于45分钟目标；八项及完整证据完成后不以无关工作填时长。交接日志记录最终归档提交。
