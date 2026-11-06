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

第四批 7758aa05；相关 **229 PASS / 0 FAIL**、settingsui vet 净。最终 M4 将产品回注开盘 6bcc6dd7，八项共 18 个失焦场景实跑红，三份源文件 MD5 逐字节恢复 OK、touch 后 229 PASS。此次同时覆盖前面三批在最终统一出口实现中的回归。最终全量门槛运行中；三平台 vet 已退出 0，gofmt 输出为空。

## 偷了什么

已读 qiaomu-design SKILL → user-preferences → craft-loop、preflight、chinese-typography，并读 Apple DESIGN 的色彩、排版和控件章节。用户已定轻便液态玻璃，直接打磨；Codex 独立执行，无外部模型、无四方向 HTML。

- craft-loop 工艺菜单 **#3 品牌 focus 环**：Apple 的单一交互蓝与 **2px** 轮廓映射到本仓现有 accent，八个退场出口恢复这一笔；不新配色。
- **#4 签名交互**：原问题到当前可用入口的交接 **0ms**；刷新自动操作 **0 次**，Space 只执行当前入口 **1 次**；原有效问题时限 **3000ms** 不变。
- **#12 材质细节**：保留既有 **8px** 控件圆角、**1px** 边与 E1/E2 分层，新增玻璃/投影 **0 层**。失效浮层退出后，焦点属于仍在屏上的操作。

为什么这轮更轻、更有空间感：确认退场后的键盘位置与可见操作一致，禁用面不接焦点；用户只需关注当前可用的一层。用户要求已有功能修复优先，不为工艺菜单凑五项增加装饰。没有重钉材质/字阶数值。

## 图像复核

八项 × 深浅 × 前后 = **32 张 720×900** ui.Tester PNG；[浅色对照](P13-hz/comparison-light.png)、[深色对照](P13-hz/comparison-dark.png)、[像素差](P13-hz/image-differences.txt)。最终 before/after 使用同一测试与键盘事件，只替换产品实现。最小变化 **420 像素**，不存在“只改文案凑数”。原图来自 Linux 软件自绘，Windows/macOS 仅编译验证。

首轮四视角：艺术总监看到失效确认退出后没有视觉落点；工程师看到配置分支没有交接；任务用户按 Space 没反应；读者分不清下一步是禁用配置还是可用 Check。第二轮集中核对四类：工具配置、Adopt、Check 和页头返回。截图先前由指针打开导致焦点环未显，补真实 Tab 进入键盘模式后重拍，不人工绘制环。

1. owner：浅色 Open configuration 周围出现完整蓝环，深色也是同一个出口。
2. orphan：Adopt 接到环，旁边 Delete 保持无环。
3. schema：配置区消失后，Check 的环仍明确标出下一步。
4. unopenable：灰淡的 Open configuration 不接焦点，Check 环可见。
5. disable-no-adopt：页头返回环出现，删除没有被触发。
6. remove-no-adopt：同行没有安全原入口，环回到右上角。
7. uninstall-no-adopt：浮层消失后页头环恢复，主内容没有残留遮罩。
8. disable-no-check：损坏状态与禁用开关保留，右上角仍可键盘返回。

4× 局部只用于检查轮廓连续性，不宣称高 DPI 真机截图。早期拼图抢在截图测试完成前运行，缺末项文件而退出；等截图完成后重新生成，最终 16 对全部非零像素变化。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿用原近黑主题 |
| A2 字体 | 沿用系统中文字体；本仓要求优先于通用非系统字体偏好 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置面，页面标题和操作保留 |
| A5 三等分/卡片海 | 无新增容器 |
| A6 假数据 | 无产品假指标；Workspace Tools 是测试夹具 |
| A7 空词/感叹号 | i18n 与产品文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原页头单行可见 |
| A10 Bento | 不适用 |
| A11 左侧竖线 | 无新增装饰 |
| B1 设计读取/拨盘 | 3/2/7，克制即时反馈、原有密度 |
| B2 主题锁定 | 每个 Tester 固定深/浅，分别验收 |
| B3 记忆点 | 每个退场问题都留下可操作的键盘位置 |
| B4 动效气质 | 交接 0ms，适合高频键盘 |
| C1 目的/频率 | 无新增键盘动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 沿用原颜色过渡与按压；本轮不改时长 |
| C4 origin | 内联带仍属于原集成，modal 仍居中 |
| C5 reduced-motion | 新增真实事件均 ReduceMotion=true |
| C6 hover | Go 自绘，无 CSS 媒体查询；原指针路由保留 |
| C7 布局动画 | 无新增，退场仍是原收回路径 |
| C8 keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新增动画，未声称真机慢放验收 |
| D2-1 系统字体 | 沿用，无 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px/1.5 不满足，Caption 9.9/正文 10.8 与行盒仍为旧拍板项 |
| D2-3 混排 | 产品文字未改，无新增中西文句 |
| D2-4 标点/对比 | 原标点与墨值保留，未声称旧次要墨全部达到 7:1 |

这些可复现缺陷落到本仓合约和事件 pin，不把单次修复改写成全局 skill 偏好。网页 Hero、浏览器 viewport 和多方向预览不适用于此 Go 自绘修复轮。

## 仍待

1. 同族下一格：等待结果期间入口整体不可用时，失败回调恢复的目标是否仍存在、是否会抢走用户新焦点。现在已有无在飞刷新与 P13-hy 的有替代按钮在飞证据；其组合未完整实测，不提前记 bug。
2. 页头 Close 的 tooltip 可消费第一下 Escape 是现有浮层退出规则，本轮用 Space 验证返回；未改变 Escape 传播。
3. 参数全移除再恢复、保存期间草稿归属、字号/行盒、accent 禁用填充、未知多选/Radio、小步长六位存储、终端 S1b/S1c/S2 尾仍沿用此前拍板/未复现记录。
4. 八条早期 WIP 继续保留；Linux 有软件自绘与事件证据，其他平台没有真机交互证据。

## 完整门槛执行记录

第一次验收进程在三平台 vet 完成后以 143 中断，普通全量仅有部分包输出、没有最终结论；原始日志另存 full-test-interrupted.txt，不能记为通过或代码失败。补跑未完成的普通/verbose 测试与三平台 build；不重复已通过的 vet。四批均在相关事件及包 vet 通过后提交，完整门槛集中最终产品执行；提交中没有注册尚未修复的后续测试。

三平台 build 已全部退出 0；最终源码 MD5 三文件一致。V/B/S 旧断言修改 0/0/0，新增 Skip 0；规格、逐项记录、mutation 与 PNG 均随每两项提交落盘。

普通全量补跑退出 0，settingsui 287.443 秒、shell 41.153 秒，其余所有包通过；verbose 全量运行中取最终 PASS 计数。文档增补 §6.201 后专门复跑单一来源检查也通过。

流程偏差：调用预算未能控制在 40 次内，主要额外调用用于确认焦点夹具、补齐变异还原验证与处理中断门槛；本轮不再扩题，完成已开始的验收和交接。前两批已修复项的小步提交因夹具诊断延后，提交前相关事件与包 vet 仍全部通过。保留过程记录，不将其描述为完全符合预算的执行。

## 最终结果

最终产品 **7758aa05**，四批每两项提交推送：0fe3891c / 2c5211c8 / b860ef46 / 7758aa05。普通与 verbose 全量均退出 **0**；最终 **4282 PASS / 0 FAIL / 2929 函数**，对比本轮基线 **4242 / 0 / 2921**，增加 40 PASS、8 函数。新增八项覆盖两主题与答案/主动移焦（modal 内 checkbox）两种位置，相关最终 **229 PASS**。三平台 vet/build 全部退出 0，gofmt **0 字节**，MD5 一致，旧测试不改、新增 Skip 0。

四组变异的 red、md5、source.md5 和恢复后的 batch1–4 日志均入 [证据目录](P13-hz)；M1/M2/M3 各四个答案失焦场景，最终 M4 八项共十八个失焦场景。均为事件断言红，非编译红。代表报错还包括 `retired uninstall-no-adopt lost available keyboard outlet "Close settings"`、`retired disable-no-check lost available keyboard outlet "Close settings"`。

最终记录：[门槛](P13-hz/gates.txt)、[普通全量](P13-hz/full-test.txt)、[源码 MD5](P13-hz/final-source.md5)、[stash 实测](P13-hz/stash-audit.txt)。完整 verbose 与全部平台原始输出在 `/tmp/floter-P13-hz/`；代表 PNG `/tmp/floter-P13-hz-notice-{light,dark}.png`。报告先前“运行中”段落保留为增量过程，本节与待办顶部为最终状态。

18:16 UTC 验收结束；从最早记录 17:37 UTC 到结束约 39 分钟，含此前开盘读取仍低于 45–75 分钟目标。八项和完整验收均已完成，不追加无关改动填时长。最终 done.log 写结案 SHA 与实测门槛；沿用所有需拍板项。
