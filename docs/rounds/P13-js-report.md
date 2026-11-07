# P13-js · 普通检查的执行状态与当前目录

开盘2026-11-07 23:03 UTC，HEAD a0b6d11d，pull已同步、工作树空。实际最新轮为P13-jr，按其仍待推进。Codex独立执行，已读用户指定设计skill及本轮参考。

## Stash清点

逐条导出patch并reverse-check，SHA与已归档审计一致，无活动stale-kill；旧独立WIP不覆盖当前Go实现。此轮不重新合入已审计的旧主题或Tauri链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/调试，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

done.log开盘不存在；收尾创建指定目录和记录。完整基线运行中，仅记实测结果。

## 第一批：普通检查忙态

1. 现象：真实点击Check后，提供者仍在等待，第二次点击和宿主重复派发令epoch升到3。改动：开始前接上行锁，异步worker的defer在主线程释放；包括失败与过期退出。验证：深浅色真实Click、文件门控的提供者、epoch必须为1，完成后原Check可点。
2. 现象：普通检查等待时，健康刷新与修复各可再派发一次。改动：诊断忙态进入rowBusy及repair专用并集，沿用已有操作面。验证：深浅两种事件中竞争调用从2降到0，完成后健康刷新可再次派发。

修改前两项均实红，原始串为`pending diagnosis starts repeatedly: epoch=3`、`pending diagnosis permits competing actions: 2`、`diagnosis never entered and settled its row busy state`。before为原实现真实帧。相关测试 **39 PASS / 0 FAIL**，shell/settingsui vet退出0。旧V/B/S修改0/0/0。

第一批919d6382。开盘全量实测 **5872 PASS / 0 FAIL / 3361函数**，退出0。

## 第二批：生命周期阶段的命令同步

3. 现象：必选probe退出7，健康已显示失败，但等待提供者时仍能搜索Check Ping。改动：生命周期结果走现有完整目录刷新，立即撤下不可运行命令。验证：真实子进程、提供者release门控，命令数组与搜索结果同时从1变0。
4. 现象：此前probe_failed的工具重新检查成功，等待提供者时仍搜索不到命令。改动：同一刷新恢复当前可用目录。验证：失败→成功子进程，两主题命令与可见结果0→1；disabled反例仍为0。

两项改前均红：`diagnosis lifecycle-failure leaves launcher commands stale: got=1 want=0`、`diagnosis lifecycle-recovery leaves launcher commands stale: got=0 want=1`。图来自修改前真实等待帧。

第二批相关 **51 PASS / 0 FAIL**、shell/settingsui vet退出0。首次目视浅色等待帧：健康刷新与底部操作均使用原禁用面，没有新增面板；已失效命令将从原搜索行撤下。旧V/B/S修改0/0/0。

第二批88816c81。

## 第三批：完成时的当前启停状态

5. 现象：没有生命周期probe的提供者等待中，同一工具在仓库停用，检查完成后设置已停用而搜索仍有Check Ping。改动：完成时刷新完整目录而不只读Store。验证：真实磁盘写入、release事件、深浅命令数组与搜索同时1→0，诊断正常结果仍保留。
6. 现象：同一路径从停用恢复启用，检查结束后搜索仍为空。改动：完整刷新恢复当前已启用命令，沿用原命令开关。验证：深浅当前记录Enabled=true、诊断ok、命令与搜索同时0→1。

改前实红：`diagnosis completion-disable leaves launcher commands stale: got=1 want=0`、`diagnosis completion-enable leaves launcher commands stale: got=0 want=1`；不是重复第二批的生命周期变化，而是提供者等待期间外部启停记录刷新。

第三批相关 **76 PASS / 0 FAIL**、shell/settingsui vet退出0，旧V/B/S修改0/0/0。

第三批677cccd8。

## 第四批：读取失败有结束反馈

7. 现象：生命周期开始读取仓库失败，仍执行旧提供者；最终没有失败通知。改动：读取失败刷新当前事实后沿既有错误通知结束，不继续无法确认归属的提供者。验证：真实损坏JSON、Click事件、提供者started文件不存在、error toast存在；修复文件后原Check可重试。
8. 现象：提供者等待期间仓库损坏，完成时无当前归属，静默丢弃结果。改动：读取失败与“归属被替代”分开处理，当前请求使用既有失败通知；epoch过期保持安静。验证：release门控后error toast、忙态退出、恢复文件后成功；另加旧epoch反例。

改前实红：`unreadable repository still launches the old provider`、`diagnosis repository-start read failure is silent`、`diagnosis repository-completion read failure is silent`。两个深浅场景均已保存原实现before帧。

第四批首次回归141 PASS并失败：新测试在Frame消费toast队列后才读queuedToast，报告为空，但实图已经显示错误通知。修正新夹具为绘制前读取Type/Title、绘制后再断言真实文本可见；未改旧B断言，也未删失败条件。原始失败日志保留，修正后需重新实跑变异以证明断言能挡住缺陷。

第二次144 PASS仍有启动阶段夹具红：无窗口Tester允许快速失败在启动操作的同一Frame被消费。把真实Click的目标派发与后台执行按真实窗口Update队列顺序串行，仍断言准确ID、后台结果Type/Title及画面文本；没有用sleep等待或弱化断言。两次新夹具失败日志均保留，产品代码无需为此改动。

第四批修正后相关 **147 PASS / 0 FAIL**、shell/settingsui vet退出0；旧V/B/S修改0/0/0。前两项新测试一并使用串行Click派发夹具，真实事件仍在、重复epoch/竞争调用断言不变。第四批提交包含八项的32张原始PNG和深浅局部对照；正式mutation与完整门槛随后进行。

第四批 **512bad85** 已推送；四批均每两项通过相关测试和vet后提交。产品变更只在shell与settingsui，启动器通过现有SetCommands链恢复，不加系统或依赖。

## 正式mutation

| 组 | 注入缺陷 | 原始运行时错误 | MD5还原、touch后 |
| --- | --- | --- | --- |
| M1 | 撤回普通检查开始忙态 | `pending diagnosis starts repeatedly: epoch=3`；`pending diagnosis permits competing actions: 2` | 12文件全OK，6 PASS |
| M2 | 生命周期刷新退回Store-only | `diagnosis lifecycle-failure leaves launcher commands stale: got=1 want=0`；恢复时`got=0 want=1` | 12文件全OK，10 PASS |
| M3 | 完成刷新退回Store-only | `diagnosis completion-disable leaves launcher commands stale: got=1 want=0`；启用时`got=0 want=1` | 12文件全OK，6 PASS |
| M4 | 撤回读取失败出口 | `diagnosis repository-start read failure is silent`；`unreadable repository still launches the old provider`；`read failure notice never reaches the visible surface` | 12文件全OK，7 PASS |

四组均exit1且包含预期运行时断言，恢复后均exit0；没有以编译失败冒充mutation红。修正后的第四批夹具再次用M4证实通知类型与画面文本均能拦住缺陷。注入、恢复、MD5清单与JSON结果入同目录；正式门槛仅在全部恢复后启动。

## 偷了什么

- craft-loop工艺密度菜单 **#4 签名交互**：等待期间同一检查接受 **1次**，重复启动 **0次**；额外反馈延迟 **0ms**。复用原禁用面，让可执行性在原控件上可见。
- **#6 编辑细节**：失效搜索结果 **1→0行**、恢复 **0→1行**；新增常驻状态说明 **0条**。状态由原结果行和原通知说清楚。
- **#12 材质细节**：沿用浮层 **1px** 受光边、控件 **8px** 圆角；新增玻璃/阴影 **0层**。这是继承约束，不冒充新材质成果。
- Apple DESIGN供体的单一交互accent角色、8px圆角和rgba黑 **0.22** 柔影，转译为已有token角色；不复制网页字体、纯黑底或模糊实现。

已实际读取SKILL、偏好账本、craft-loop、preflight、chinese-typography与Apple DESIGN。用户已定方向，直接打磨；不走四方案，不用外部模型。未凑五种装饰，用户“修已有问题”优先。

**为什么更轻、更有空间感**：不可执行的结果及时让出搜索空间，执行状态落在原行，失败只进入已有浮层；不增加常驻说明或玻璃嵌套。

## 两轮图像复核

第一轮看真实原帧：艺术总监看到可点的旧操作面与实际忙态冲突；工程师看到同行重复派发；任务用户看到已停用命令仍有运行入口；读者遇到空库存页但没有当前失败解释。四处分别落成行锁、目录刷新、原通知出口和重试恢复。

第二轮看两套总览与4×局部：浅色操作条的Check/修复/移除一起变暗；深色竞争图中误启动的修复转圈恢复为原图标；失败后的Check Ping结果撤下、恢复后回到同一行；深色读取错误在顶部既有浮层呈现警告字形和墨。4×为最近邻检查图，不冒充高DPI真机截图。

32张原始PNG、16张局部对照、4张4×图与两张总览已入库。八项每个主题都有相关区域像素变化，最小 **59像素**（误启动修复的转圈与静态图标）。原始前图按批次分别来自a0b6d11d / 919d6382 / 88816c81 / 677cccd8，不冒称同一开盘整仓；后图来自最终产品与修正后的事件夹具。时间戳/耗时的差异不作为成果，像素计数只取控件、结果或通知区域。

| 项 | 浅色 | 深色 |
| --- | --- | --- |
| 1 重复检查 | [对照](P13-js/repeat-light-compare.png) | [对照](P13-js/repeat-dark-compare.png) |
| 2 同行竞争 | [对照](P13-js/competing-light-compare.png) | [对照](P13-js/competing-dark-compare.png) |
| 3 生命周期失败 | [对照](P13-js/lifecycle-failure-light-compare.png) | [对照](P13-js/lifecycle-failure-dark-compare.png) |
| 4 生命周期恢复 | [对照](P13-js/lifecycle-recovery-light-compare.png) | [对照](P13-js/lifecycle-recovery-dark-compare.png) |
| 5 完成时停用 | [对照](P13-js/completion-disable-light-compare.png) | [对照](P13-js/completion-disable-dark-compare.png) |
| 6 完成时启用 | [对照](P13-js/completion-enable-light-compare.png) | [对照](P13-js/completion-enable-dark-compare.png) |
| 7 开始读取失败 | [对照](P13-js/repository-start-light-compare.png) | [对照](P13-js/repository-start-dark-compare.png) |
| 8 完成读取失败 | [对照](P13-js/repository-completion-light-compare.png) | [对照](P13-js/repository-completion-dark-compare.png) |

[浅色总览](P13-js/overview-light.png)、[深色总览](P13-js/overview-dark.png)、[像素记录](P13-js/image-differences.txt)。Linux ui.Tester软件自绘，设置960×1600、启动器960×600；Windows/macOS未做真机操作。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿用近黑主题 |
| A2 非系统字体 | 本仓中文系统字体规则优先，不为网页规范换字库 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生功能页 |
| A5 卡片海/三等分 | 无新增容器 |
| A6 假数据 | 无产品假数据；Check Ping为真实提供者测试命令 |
| A7 空词/感叹号 | i18n零改 |
| A8 编号眉批 | 产品无新增；证据图编号不进入产品 |
| A9 H1 | 沿用设置页头，未遮蔽 |
| A10 Bento | 不适用 |
| A11 竖线装饰 | 无新增 |
| B1 读取/拨盘 | 3/2/7，熟悉、即时、原密度 |
| B2 主题锁定 | 每张Tester明确深/浅，页内不混 |
| B3 记忆点 | 正在检查、当前可执行性与入口一致 |
| B4 动效气质 | 新状态额外延迟0ms |
| C1 目的/频率 | 高频点击不额外加动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 沿用既有颜色与按压过渡，未调时长 |
| C4 origin | 原行与原通知位置不动 |
| C5 reduced-motion | 新Tester全部ReduceMotion=true |
| C6 hover媒体查询 | Go自绘，不适用CSS，原指针路由保留 |
| C7 布局动画 | 无新增尺寸/间距动画 |
| C8 keyframes重播 | 无新增 |
| C9 手势 | 不涉及 |
| C10 Framer | 不适用 |
| C11 慢放 | 无新增动效，不宣称慢放真机验收 |
| D2-1 中文字体 | 沿用系统字体，无webfont |
| D2-2 字号行高字重 | 通用≥14px/1.5仍未满足；旧Caption9.9/正文10.8及行盒继续需拍板，不擅改 |
| D2-3 混排 | 产品文案零改，测试命令名不是产品新增文案 |
| D2-4 标点/暗色对比 | 原文案与墨不改，不宣称既有次要墨全部过7:1 |

可复现问题沉淀为§6.246及事件pin，不将一次性修复升级为全局审美规则。无新增页面/依赖/模糊资产/动画系统。

## 仍待

1. 下一轮首选：生成工具“重新探测命令”的异步入口，检查它先启动时的重复派发与同行互斥。`settingsui.go`的ReprobeCommands回调目前仍无行级开始/结束交接；这是代码线索，先做事件复现，不重复本轮普通检查与上轮健康刷新。
2. 历史需拍板项沿用：字号/行盒、accent禁用填充、未知选项、保存期草稿归属、终端S1b/S1c/S2尾及.pending-bugs中的其他拍板项。确认三秒、保存时机与终端语义不变。
3. Linux自绘与真实Go子进程已验；其他平台仅编译门槛，不能外推真机行为。八条早期WIP保留，详见本报告表。

## 最终门槛与交接

最终产品 **512bad85**；四批 **919d6382 / 88816c81 / 677cccd8 / 512bad85** 均已推送。完整verbose **5902 PASS / 0 FAIL**，普通与verbose全量均exit0；测试函数 **3371**（开盘3361，+10），PASS比基线5872增加30。三平台vet/build全部exit0、gofmt输出为空；异步入口race **12 PASS / 0 FAIL**，相关回归147 PASS。旧测试函数未减少，旧V/B/S断言修改 **0/0/0**，新增Skip **0**。

完整普通/verbose并行运行，最终均一次通过；设置界面用例耗时较长但正常推进，未中断或放宽断言。两次失败只发生在第四批的新通知观察夹具，原始输出与原因已保留；正式M4与最终完整门槛基于修正后的夹具。四组变异恢复后的12文件MD5再次全OK，未带着注入代码跑正式门槛。

证据：[门槛汇总](P13-js/gates.txt)、[普通全量](P13-js/full-test.txt)、[verbose全量](P13-js/verbose-test.txt)、[race](P13-js/race.txt)、[基线](P13-js/baseline.txt)、[最终源码MD5](P13-js/final-source.md5)、[mutation结果](P13-js/mutations.json)。每批改前红/修后绿、M1–M4注入与恢复、两次新夹具失败原始日志同目录。

23:03 UTC开盘，23:28 UTC完成验收，约25分钟，低于45–75分钟目标；八项已有问题均完成，不以无关装饰或重复测试填时长。40次以内的批量工具调用完成实施与交接；未使用子代理或外部执行器。下一轮按“仍待”先复现生成工具重新探测命令的入口，不触碰任何等待拍板的交互语义。
