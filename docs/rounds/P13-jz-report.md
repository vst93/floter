# P13-jz · 本地工具选择与内容回调的草稿归属

开盘：2026-11-08，HEAD 8a6cf2df，pull已最新，原工作区干净。任务书进度停在jw，仓库已完成jx/jy，因此接续jy“同表单选择回调”线索。指定done.log不存在。本轮唯一执行者Codex，未调用其他模型或代理。

## stash清点

八条逐条实跑reverse-check均exit1，SHA与交接一致；它们是早期独立WIP，不是尚未接续的stale-kill。沿用交接语义审计保留，不恢复旧主题或Tauri链；无drop，无遗漏的新抢救稿。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4e42eb518f4a9732465cc31be84002078 | 旧keycap/主题 | 保留 |
| stash@{1} | 8af94a07586e56646fe74cc4da76355ecb0cc343 | 旧行距/滑块 | 保留 |
| stash@{2} | 7e4aa710feb96f0afdb6328418ded0ea3d39051d | 旧卸载布局 | 保留 |
| stash@{3} | 1e3aec9484cb2be99e73171beb05cd6806de0127 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42e2825109a5f2688831cb0495434e7444 | 旧字段/确认栏 | 保留 |
| stash@{5} | 9b8ce23cbd7a74c9fa9c92faae340b5881602070 | 旧快捷键 | 保留 |
| stash@{6} | 90d314b27296c6ee60ad29297ae1a26a8ea36165 | 旧动作栏 | 保留 |
| stash@{7} | 7a9f6e136bb2b7af5265f18ada33b408c9c4f6f7 | 旧Tauri发布链 | 保留 |

## 增量记录

开盘中文locale全量实测6069 PASS / 0 FAIL / exit0，原3425测试函数。基线日志在本轮目录。首轮未修夹具提前取了英文label，后三组导航/复制复现因此失败；这不是产品缺陷证据，修正语言初始化后单独重新复现，原日志保留。

### 第一批

1. **选择返回撤销新输入**：现象→同一表单改模式、改写字段或主动清空路径后，旧选择覆盖整份局部字段；改动→操作前捕获原始字段与表单身份，返回时不一致便退役当前帧；验证→真实选择按钮事件，回调同步注入模式/脚本、名称/路径/命令/参数/版本/说明和清空三种更新，两套外观都保留最新值。
2. **选择器错误键盘断路**：现象→报错后焦点仍在选择按钮，直接输入丢失；改动→保留一次既有警告并返回原程序输入，沿用焦点归属检查；验证→Tab/Enter触发权限错误后直接全选键入，路径成为/usr/bin/retry，深浅均通过。

M1撤回两处保护后实红`picker rewound newer mode`、`picker rewound newer fields`、`picker refilled cleared path`、`picker error lost editable exit`、`picker error dropped keyboard correction`。MD5还原并touch后定向26 PASS / 0 FAIL / exit0，vet exit0；旧V/B/S断言修改0/0/0，无新增Skip。

第一批d80da969已推送。

### 第二批

3. **复制把旧内容确认到新草稿**：现象→复制期间更新命令/执行计划/脚本，旧帧覆盖新输入且显示已复制；改动→三类内容操作统一复核原始草稿，失效后停止旧帧写回与确认；验证→三个真实复制按钮分别触发同步更新，深浅均保留全部新字段、没有旧确认，再次复制严格得到新内容。
4. **导出后新脚本倒退**：现象→导出回调期间修改命令/脚本，回到表单又变旧，第二次导出仍是旧快照；改动→导出返回也复核草稿并停止旧帧写回，已派发内容不变；验证→连续两次导出分别为weather-script.py/旧脚本和forecast-script.py/新脚本，两套外观均严格相等。

第二批与既有回调/复制生命周期/内容出口回归88 PASS / 0 FAIL / exit0，vet exit0。M2退回仅校验form身份，实红`command/plan/script copy rewound newer draft`、`copy confirmed stale bytes`、`export rewound newer draft`；MD5还原并touch后88 PASS / exit0。旧V/B/S断言修改0/0/0、新增Skip0。

第二批aa358d54已推送。

### 第三批

5. **折叠重开后接收旧选择**：现象→选文件期间真实点击添加入口折叠再打开，同一表单指针/字段均没变，旧路径仍写入；改动→回调身份加入既有toolDisclosure代次；验证→第二个ui.Tester视图泵入两次真实折叠按钮事件，旧选择不落地，重开后的新选择正常。
6. **折叠重开后复活旧复制确认**：现象→折叠已清确认，晚回的复制重新显示已复制；改动→相同代次保护覆盖内容操作；验证→真实折叠往返后确认为空，显式重新复制才点亮，深浅均通过。

第三批新旧回归94 PASS / 0 FAIL / exit0，vet exit0。M3将代次固定为0后实红`picker fold accepted departed result`、`copy revived after fold departure`；MD5还原、touch后6 PASS / exit0。旧V/B/S断言修改0/0/0、新增Skip0。

第三批ea3a2950已推送。

### 第四批

7. **跨页返回后接收旧选择**：现象→选择器等待期间离开集成页又回来，旧选择写入当前表单；改动→只在离开集成页时推进内容回调的页面代次，与连接失败恢复代次独立；验证→真实侧栏一般页/集成页往返，旧路径不落地，重新选择正常；重选当前页不退役有效操作。
8. **跨页返回后复制确认复活**：现象→导航清除的已复制状态被旧回调重新点亮；改动→内容操作也验证页面代次；验证→真实侧栏往返后无旧确认，新的复制恰一次成功，两套外观通过。

第四批新旧定向回归100 PASS / 0 FAIL / exit0、vet exit0。M4固定页面代次为0，实红`picker page accepted departed result`与`copy revived after page departure`；MD5还原并touch后6 PASS / exit0。八项已分别复现，不将夹具错误计为缺陷；旧V/B/S断言修改0/0/0、新增Skip0。

第四批82a691f5已推送；四批每两项完成即提交。

## 偷了什么

实际读取qiaomu-design SKILL、user-preferences、craft-loop、preflight、chinese-typography，另按功能表单需要读Carbon模式和Apple DESIGN供体。唯一方向为轻便液态玻璃，采用已有方向打磨路径。Apple的单一交互蓝、8px矩形控件半径、平面/浮层阴影职责映射本仓token；不把Apple网页字号或负字距移植到中文表单。

- 从craft-loop工艺密度菜单**#3品牌focus环**取“状态对应可操作输入”，落成错误后**1个程序输入焦点、额外等待0ms**；沿用原accent墨。
- 从**#4签名交互**取“反馈有明确归属”，落成**1次操作只确认1份仍有效的草稿**；退出后晚返回确认**0次**，原复制确认时长**1600ms**不变。
- 从**#12材质细节**取“材质服务可用状态”，沿用**1px受光边、0层新增阴影/玻璃**；内容过期就撤销确认，不加一层说明卡或通知。

用户修已有功能与不扩张要求优先，不为凑满五项工艺增加装饰。**这轮为什么更轻、更有空间感**：焦点回到可编辑字段，反馈只依附当前有效内容，退出的表单不被旧操作重新点亮，底/面/浮无需新增层就能表达真实状态。

## 两轮图像复核

第一轮四视角记录：艺术总监看到旧“已复制”占据新草稿的动作位；工程师看到回调返回把新字段退回旧值；任务用户看到跨页回来仍接收旧选择；读者看到错误后蓝环留在按钮而无法直接修订路径。第二轮只收敛这四处：草稿归属、确认归属、退出代次、错误焦点。

Linux ui.Tester的720×1500原始帧，另一个960×1600视图用于真实侧栏事件。48张原始PNG、24组深浅对照、4张最近邻4×局部；放大图用于检查边与焦点，不冒充原生高DPI。长画布是测试观察范围，不改产品高度链。

肉眼复核：浅色错误图蓝环从“选择”按钮回到路径槽；深色模式图保留Python脚本和新内容；浅色导出图保留forecast与print('new')；折叠/导航图里的旧“命令已复制”恢复为正常操作标签，程序路径保持weather。框与受光边没有新增层。首两项初版before夹具的CopyText可用性与after不同，已用同一最终夹具和Go overlay重渲染；未修改工作树源码，MD5均OK，原五类断言仍实红。最终24组均有像素变化，最小**532像素**，见image-differences.json。再次查看4×错误出口，仅路径/选择按钮焦点有变化，无关复制按钮已保持一致。

| 项 | 浅色对照 | 深色对照 |
| --- | --- | --- |
| 1 新草稿 | [图](P13-jz/picker-fields-light-comparison.png) | [图](P13-jz/picker-fields-dark-comparison.png) |
| 2 错误键盘出口 | [图](P13-jz/picker-error-light-comparison.png) | [图](P13-jz/picker-error-dark-comparison.png) |
| 3 复制归属 | [图](P13-jz/copy-command-light-comparison.png) | [图](P13-jz/copy-command-dark-comparison.png) |
| 4 导出归属 | [图](P13-jz/export-draft-light-comparison.png) | [图](P13-jz/export-draft-dark-comparison.png) |
| 5 折叠选择 | [图](P13-jz/picker-fold-light-comparison.png) | [图](P13-jz/picker-fold-dark-comparison.png) |
| 6 折叠复制 | [图](P13-jz/copy-fold-light-comparison.png) | [图](P13-jz/copy-fold-dark-comparison.png) |
| 7 导航选择 | [图](P13-jz/picker-page-light-comparison.png) | [图](P13-jz/picker-page-dark-comparison.png) |
| 8 导航复制 | [图](P13-jz/copy-page-light-comparison.png) | [图](P13-jz/copy-page-dark-comparison.png) |

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，近黑/浅色token沿用 |
| A2 非系统字体 | 中文系统字体优先，不套网页换字体规则 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置表单 |
| A5 三等分/万能卡片 | 无新增卡片 |
| A6 假数据 | 产品无假指标，weather/forecast是隔离测试输入 |
| A7 空词 | i18n零改，无新增产品文案 |
| A8 眉批 | 无新增 |
| A9 标题 | 既有标题完整可见 |
| A10 Bento | 不适用 |
| A11 装饰竖线 | 无新增 |
| B1 读取/拨盘 | 3/2/7与已有功能表单一致 |
| B2 主题 | 深浅分别设置，单图无混用 |
| B3 记忆点 | 回来仍是当前草稿，旧反馈不复活 |
| B4 动效气质 | 即时更新，无新动画 |
| C1 目的/频率 | 键盘操作无额外动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 新过渡0ms，复制确认仍1600ms；按压反馈沿用 |
| C4 origin | 错误回原字段，通知出口不变 |
| C5 reduced-motion | 新Tester设置ReduceMotion=true；未增加动效 |
| C6 hover查询 | Go自绘不适用CSS媒体查询；指针模型不变 |
| C7 布局动画 | 无新增 |
| C8 重播keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用Go自绘 |
| C11 慢放 | 没有新动效，只核确定静态帧，不声称真机慢放 |
| D2-1 字体 | 系统中文字体，无webfont |
| D2-2 字号/行高/字重 | 通用≥14px/1.5仍不满足；原Caption9.9/正文10.8与行盒需拍板，本轮未越权改 |
| D2-3 混排空格 | i18n零改，无新产品句子 |
| D2-4 标点/对比 | 原标点/墨沿用，不声称所有次要墨达到7:1 |

缺陷沉淀在§6.253和事件测试，不升级为全局审美规则。原生系统选择器未截图，回调更新使用确定性注入；折叠和导航由另一个Tester视图泵入实际按钮事件。Windows/macOS仅编译验证，不声称真机交互通过。

## 仍待

1. 下一轮首选已有本地工具脚本导出的等待与退出路径：保存对话框打开时重复点击、取消/失败后的当前状态。先用真实宿主saveFile回调复现，不改文件覆盖事务，不把代码线索当已确认bug。可沿同表单平台/权限选择的键盘出口继续查，不重复本轮回调草稿/导航归属。
2. 生成工具编辑/转换覆盖、重复新包ID覆盖、重装事务、配置模板失败重试、另一端冲突、字号/行盒、accent禁用填充、未知多选/Radio、极小步长、176-9/161-4保存期草稿、终端S1b/S1c/S2尾继续需拍板。
3. 八条早期WIP继续原样保留；旧ImageCopy/Rescan/memo pending时序线索沿用，本轮不放宽旧B断言。

## 最终验收

2026-11-08 **05:32:00 UTC**完整门槛完成。最终 **6101 PASS / 0 FAIL / 3433测试函数**，对开盘6069/3425为 **+32 PASS、+8函数**。普通与verbose全量分别完整exit0；gofmt输出空；linux/windows/darwin的vet/build全部exit0；定向race **32 PASS / 0 FAIL / exit0**；最终两份产品源码MD5均成功。

V/B/S台账：旧视觉断言改动0、旧行为断言改动0、旧结构断言改动0；新增8个行为测试函数，旧测试删除0、新增Skip0。四组mutation均exit1且有上述真实断言报错；逐字节还原成功，touch后分别26/88/6/6 PASS。补拍overlay使用同一最终夹具再次触发M1五类失败，不改工作树源码，不另凑变异数量。

四批提交均已推送：**d80da969 / aa358d54 / ea3a2950 / 82a691f5**。收尾仅补文档、完整日志和同夹具图片。首个时钟05:05:06至完整验收05:32:00为26分54秒（开盘读档在此前），短于45–75分钟目标；本轮八项及全部门槛已完成。一次apply_patch因同文件Delete/Add格式被拒绝，未改文件，随后用Update修正；未将工具错误或首版夹具错误计作产品缺陷。

证据：[基线](P13-jz/baseline.txt)、[普通全量](P13-jz/full-test.txt)、[verbose全量](P13-jz/full-verbose.txt)、[race](P13-jz/focused-race.txt)、[源码MD5](P13-jz/final-source.md5)、[门槛退出](P13-jz/gates.exit)、[像素差](P13-jz/image-differences.json)、[同夹具补拍](P13-jz/same-fixture-before.txt)。每条命令的txt/exit、48原始PNG、24对照及四组mutation原始证据随本报告入仓。
