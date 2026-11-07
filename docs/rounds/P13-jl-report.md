# P13-jl · 同步读取与配置刷新交错

起点087bee9f，ff-only pull 已最新、工作树干净，19:28 UTC 开盘。唯一执行者 Codex。本文增量记录实测结果。

## Stash 清点

活动清单无 stale-kill；八条 SHA 与 P13-jk 及交接清单相同。逐条导出原patch并 reverse-check 均退出1；沿用语义核对结论，保留独立旧 WIP，不覆盖新主题或旧 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载/scratch，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

## 增量实施

已读取最新“仍待”、视觉合约与 qiaomu-design 打磨资料。同步首读/补读使用无条件 publish，正在建立事件证据；不把代码线索当已修bug。

1. 现象：同步首读取到旧地址后，真实导入成功，旧结果仍使表单显示 before.example。改动：首读原子记录所属代，发布被拒后重新读取当前结果。验证：深浅真实导入、实际编辑框 Cmd+A/C 和磁盘；修前 `cold-import: visible editor reverted to "https://before.example"`，修后 imported.example。
2. 现象：旧首读失败晚于成功保存返回，endpoint 变空。改动：同一首读保护覆盖失败结果，保存使旧代失效。验证：深浅真实 SaveConfiguration、磁盘和编辑框；修前 `cold-save-read-error: visible editor reverted to ""`，修后 saved.example。

第一批相关 **14 PASS / 0 FAIL**，shell vet净、gofmt空。旧 V/B/S 0/0/0；修前6个测试节点实红，M1集中注入待后续。浅色图目检：endpoint正确显示 imported.example，原字段、圆角、焦点操作都保留。基线全量仍运行，未冒写最终数字。

第一批 **7a353649** 已推送。基线普通源码的 verbose 实测 **5654 PASS / 0 FAIL**、退出0。基线脚本在结束时数到了新加两条函数，函数数改由起点087bee9f树独立核对，避免把当前工作树计入基线。

3. 现象：补读第二个已安装工具时，第一工具保存成功，旧合并基底仍把 before.example 交给调用帧。改动：单行补读发布同时核对原表代际，失效就重新获取。验证：两个真实provider、真实保存、补读返回值；修前 `added-neighbour-save: synchronous read returned retired value https://before.example`。
4. 现象：补读自己的配置期间导入新值，旧读取让打开的字段回到旧地址。改动：补读结果也受同一代际保护。验证：真实导入、实际编辑框复制与磁盘；修前 `added-own-import: visible editor reverted to "https://before.example"`，修后保持 imported.example。

第二批相关 **20 PASS / 0 FAIL**，shell vet净、gofmt空，V/B/S旧断言0/0/0。第一帧证据额外使用实际applyIntegrationConfig映射返回快照，防止后续帧自动修复掩盖错误；M1会再次验证这一加强后的事件。基线函数数从087bee9f树实测为 **3303**。

第二批 **25eccc8f** 已推送。

5. 现象：同步首次探测的旧不可用结果晚于刷新完成，已修好的工具重新显示 Tool unavailable。改动：事实首读检查所属代，旧结果被拒后取当前已确认状态。验证：真实provider、RefreshIntegrations、实际卡片；深浅修前 `cold-runtime: synchronous probe disabled a repaired tool`，修后工具可用。
6. 现象：首次摘要读取的旧值在刷新后返回，批准卡重新显示 manifest changed 警告。改动：事实首读保护覆盖批准摘要，保留已确认的新摘要。验证：真实批准记录/磁盘摘要、卡片与返回快照；深浅修前 `cold-approval: synchronous probe revived approval warning`，修后摘要等于磁盘。

第三批相关 **18 PASS / 0 FAIL**，shell vet净、gofmt空、旧V/B/S 0/0/0。M2将在四批完成后统一回注旧代码。

第三批 **8df4c0a0** 已推送。

7. 现象：新行补探测与原行修复刷新交错，旧合并基底重新禁用原工具。改动：补探测同时核对基底代际，保留刷新后原行的可用状态。验证：两真实provider、实际刷新与卡片，深浅修前 `added-neighbour-runtime: synchronous probe disabled a repaired tool`，修后可用。
8. 现象：补探测自己时返回旧摘要，已批准工具重新出现变更警告。改动：补探测结果发布检查代际，拒绝失效摘要。验证：真实批准记录、实际磁盘摘要、卡片和返回值，深浅修前 `added-own-approval: synchronous probe revived approval warning`，修后匹配。

第四批相关 **24 PASS / 0 FAIL**、shell vet净、gofmt空、旧V/B/S 0/0/0；八项全部已红绿复现，完整门槛与两组mutation随后执行。

第四批 **868954ac** 已推送。

## Mutation 与过程偏差

- M1：恢复配置首读/补读无条件发布。四项深浅共12个测试节点实红，含 `added-neighbour-save: visible editor reverted to "https://before.example", want "https://saved.example"`、`cold-save-read-error: visible editor reverted to "", want "https://saved.example"`；证明旧合并基底也会实际画错首帧。恢复后四文件MD5全部OK，touch再验12 PASS。
- M2：恢复事实首读/补读无条件发布。四项深浅共12个测试节点实红，含 `cold-runtime: synchronous probe disabled a repaired tool`、`added-own-approval: synchronous probe revived approval warning`。恢复后四文件MD5全部OK，touch再验12 PASS。
- 过程偏差保留：第一次M1注入保留了未使用revision变量，仅编译失败，**不算有效mutation**，原日志M1-invalid-injection.txt。修正注入后获得真实事件失败。随后M2开始时M1恢复校验仍在运行，M1整体MD5检测到当时事实文件正在变异；原日志M1-overlap-md5.txt保留。两份产品文件均恢复后重新串行MD5核对与touch复验，正式M1-md5/M1-restored-final均成功（退出0，12 PASS）。不把这一过程冒写成全程无误。

## 偷了什么

真实读取qiaomu-design SKILL、user-preferences、craft-loop、preflight、chinese-typography和Apple DESIGN。当前用户固定方向优先，Codex单执行者，没有方案点选、HTML预览、外部模型或子代理。

- craft-loop工艺菜单 **#4 签名交互**：刷新确认到当前帧额外延迟 **0ms**，原 **2s TTL** 保留；首读/补读不会把已退休结果重新画出来。
- **#6 编辑细节**：批准旧警告条 **1→0**，有效摘要仍在原记录行；补读后的实际输入保留已保存值，附加解释卡 **0个**。
- **#12 材质细节**：既有 **1px** 受光边和 **8px** 控件圆角保留，新增玻璃 **0层**。这两项是沿用值，不冒称改色。
- **#5 多层阴影**：浮面原 **2层** 外影沿用，字段额外外影 **0层**。Apple DESIGN的中性交互角色、8px圆角、2px focus、浅色0.22阴影只映射既有token，不复制品牌色。

用户要求修已有功能优先，未为凑五项装饰加纹理/大字/动效。**这轮更轻、更有空间感**：工具可用时不再被错误徽标和批准警告争夺注意力，撤掉旧告警后配置入口回到原层级；表单首帧与保存事实一致。

## 图像与两轮复核

8项×深浅×前后共32张ui.Tester原图，另有16张前后对照，全部在P13-jl目录。原图为960×1800或960×1400，路径`/tmp/floter-P13-jl-<场景>-<light|dark>-<before|after>.png`；对照裁掉下方空白，原图完整保留。详情图为最近邻2×放大，只用于检查，不冒充原生高DPI。

第一轮四视角：艺术总监看到过期警告抢主任务注意力；工程师发现后续帧自愈可能掩盖首帧错误；任务用户看到Saved却拿到旧地址；读者遇到已运行工具旁仍标Unavailable。第二轮集中核对四处：生产mapper投影精确返回值、真实复制当前字段、邻行不受补读污染、摘要与警告同时更新。没有因空白裁图改产品高度或字号。

| 项 | 浅色前后 | 深色前后 | 目检变化 |
| --- | --- | --- | --- |
| 1 首读导入 | [图](P13-jl/cold-import-light-comparison.png) | [图](P13-jl/cold-import-dark-comparison.png) | endpoint从旧地址变为imported.example |
| 2 首读失败/保存 | [图](P13-jl/cold-save-read-error-light-comparison.png) | [图](P13-jl/cold-save-read-error-dark-comparison.png) | 空字段恢复saved.example、token和eu |
| 3 补读/邻行保存 | [图](P13-jl/added-neighbour-save-light-comparison.png) | [图](P13-jl/added-neighbour-save-dark-comparison.png) | 第一工具保持saved.example，第二工具仍独立 |
| 4 补读/本行导入 | [图](P13-jl/added-own-import-light-comparison.png) | [图](P13-jl/added-own-import-dark-comparison.png) | 本行地址保持imported.example |
| 5 首探测/运行时 | [图](P13-jl/cold-runtime-light-comparison.png) | [图](P13-jl/cold-runtime-dark-comparison.png) | Running旁多余Tool unavailable撤掉 |
| 6 首摘要/批准 | [图](P13-jl/cold-approval-light-comparison.png) | [图](P13-jl/cold-approval-dark-comparison.png) | 摘要恢复匹配，过期警告消失，配置行上移 |
| 7 补探测/邻行修复 | [图](P13-jl/added-neighbour-runtime-light-comparison.png) | [图](P13-jl/added-neighbour-runtime-dark-comparison.png) | 原行不可用徽标退役，邻行正常 |
| 8 补摘要/本行批准 | [图](P13-jl/added-own-approval-light-comparison.png) | [图](P13-jl/added-own-approval-dark-comparison.png) | 本行批准匹配，第二工具记录保留 |

像素变化逐图计数见[image-differences.txt](P13-jl/image-differences.txt)，每项两主题均大于0。软件自绘图用中性底展示窗口透明区，未修改产品透明度。

## Preflight A/B/C/D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿用本仓近黑与语义token |
| A2 非系统字体偏好 | 不采用，中文系统字体和用户既有合约优先 |
| A3 斜体 | 无新增，截图无斜体 |
| A4 Hero | 不适用，桌面设置页 |
| A5 三等分/卡片海 | 未加容器，保留原集成卡 |
| A6 假数据 | 只在测试使用.example和Configured/Second Tool；不进入产品默认值 |
| A7 空词/感叹号 | 产品文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原Integrations标题单行 |
| A10 Bento | 不适用 |
| A11 装饰竖线 | 无新增；原批准告警强调边随无效警告退役，不宣称全仓无竖线 |
| B1 读取/拨盘 | 2/1/6：保留布局、即时更新、原密度 |
| B2 主题 | 每个Tester独立指定深浅 |
| B3 记忆点 | 当前已确认值与状态不会被旧读取倒退 |
| B4 动效气质 | 高频刷新新增动效0个 |
| C1 目的/频率 | 高频状态交接即时 |
| C2 all/ease-in/scale(0)/fade | Go自绘，无新增 |
| C3 时长/按压 | 原控件按压保留，无新动画时长 |
| C4 origin | 无新增浮窗，提示仍贴原对象 |
| C5 reduced-motion | 截图与事件实际启用ReduceMotion=true |
| C6 hover媒体条件 | 无CSS，关键信息无需悬停 |
| C7 属性 | 无新增布局动画 |
| C8 重播 | 无新keyframes |
| C9 手势 | 未涉及 |
| C10 Framer Motion | 不适用，无依赖新增 |
| C11 慢放 | 无新动效，不宣称真机慢放/帧率验证 |
| D2-1 字体 | 沿用系统字体；本轮截图为英文设置夹具，不冒充中文真机 |
| D2-2 字号/行盒/字重 | 通用≥14px/1.5未全满足；Caption9.9/正文10.8仍为拍板项，遵用户禁改边界 |
| D2-3 混排 | 无新中文产品文案，现有i18n不变 |
| D2-4 标点/暗色对比 | 保留现有标点与墨；未全面重测AAA，不声称全部次要墨达7:1 |

缺陷沉淀到本轮事件pin和合约；不把一次修复改成全局skill审美规则。当前用户边界优先于skill通用的五项装饰、浏览器/移动视口和全项通过措辞。

## 仍待

1. 下一轮首选已安装工具同ID的路径/声明更新后，单行freshness和运行入口是否仍对应当前对象。只是代码线索，先事件复现；不重复本轮已修的同对象首读/补读失效问题，不做纯缓存整理。
2. 多草稿/转换覆盖、另一端冲突、重复新包覆盖、模板失败补跑、字号/行盒、accent禁用填充、未知多选/Radio、极小步长、176-9/161-4保存中草稿、终端S1b/S1c/S2尾沿用需拍板。3000ms、5000ms、180ms/Flush和终端会话语义不改。
3. Linux真实provider/磁盘/配置动作和ui.Tester表面已验；旧读取结果及返回顺序受控，不冒称系统运行时真的卸载或原生选择器人工验收。Windows/macOS仅vet/build，运行时未验。
4. 八条早期WIP逐条保留；旧TestTheImageCopyLeavesTheDrawingThread并行时序线索沿用，不改旧B断言、不加延迟或容差。

## 完整门槛与交接

最终产品 **868954ac**。全量普通测试退出 **0**；verbose退出 **0**，实测 **5678 PASS / 0 FAIL**（基线5654，+24）；函数 **3311**（起点3303，+8）。gofmt输出为空；linux/windows/darwin的vet和build六项全部退出0。最终四文件MD5全OK。旧断言 **V/B/S 0/0/0**，新增Skip **0**。

四批每两项在定向事件/vet通过后立即提交推送，完整门槛在最终产品上集中一次普通、一次verbose。产品改动仅configmemo.go/integrationfacts.go，新增两文件共八条测试函数；没有新系统、依赖、页面。差距表本轮八条均已落地。

原始证据：[门槛](P13-jl/gates.txt)、[普通全量](P13-jl/full-test.txt)、[verbose全量](P13-jl/full-verbose.txt)、[基线](P13-jl/baseline.txt)、[最终MD5](P13-jl/final-md5.txt)、[M1实红](P13-jl/M1-red.txt)、[M1最终还原](P13-jl/M1-restored-final.txt)、[M2实红](P13-jl/M2-red.txt)、[M2还原](P13-jl/M2-restored.txt)。三平台原日志均随本目录保存。

19:28 UTC开盘，**2026-11-07 19:47:36 UTC**完整门槛完成，约20分钟，低于45–75分钟目标。八项、四批推送、两组变异和全量门槛均已完成，不以重复测试或无关修改填时间。工具预算用于收尾，不再新增题目。交接日志记录最终归档提交。
