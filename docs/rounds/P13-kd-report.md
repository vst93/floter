# P13-kd · 内容动作恢复时看得见编辑出口

开盘4330be43，pull已同步、工作树干净。首个明确时钟07:30:02 UTC，阅读在此前。给定done.log不存在。Codex独立执行，未切模型、未委派。

## Stash清点

八条与P13-kc相同，无新增stale-kill。逐条reverse-check exit1，交接账明确均为早期独立WIP，不把旧主题/Tauri覆盖当前实现；本轮保留原件，无静默丢弃。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap/主题，reverse1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri发布链，reverse1 | 保留 |

## 增量实施

先复现，再逐条追加现象→改动→验证。完整开盘基线仍在运行，不沿用旧报告数字充数。旧V/B/S断言修改目标0/0/0。

1. 程序选择失败：原按钮持焦后滚到声明页尾，再Enter遇到选择器错误，程序框返回焦点但Y=-1152.0001、视口Y=51.399998 → 只在原按钮实际返焦时ScrollIntoView → 深浅事件都显示原程序框，警告恰好1次，直接键入修改路径，滚轮自由。
2. 程序选择取消：同样的长表单Enter取消，编辑出口在屏外 → 返焦时一次显示原框 → 草稿原样保留、警告0次、选择器仅派发1次，直接键入有效。

第一批定向6 PASS / 0 FAIL / exit0，gofmt空、settingsui vet净。M1回注旧settingsui.go，四个深浅场景实红`editing outlet outside viewport: rect={X:386.8 Y:-1152.0001 W:240 H:30}`；MD5逐字节还原成功，touch复验6 PASS。浅色after目视：程序框和蓝色插入点出现在正文顶端，下面仍是原命令与参数行，无新增面层。原图暂存/tmp，最终归档深浅对照。

第一批e75f97b0已推送。

3. 程序选择成功：长表单页尾按Enter选定路径，路径更新但程序框仍Y=-1152.0001 → 同步返回原框时显示它 → 深浅新路径精确为/usr/bin/forecast，派发1次、警告0次，可直接改为/usr/bin/corrected。
4. 程序选择能力撤回：ChooseProgram变nil后按钮消失，返焦原框仍屏外（X=449.8、Y=-1152.0001）→ 仅实际拥有焦点的退役按钮显示原程序框 → 深浅回调0次、原路径保留、直接编辑与自由滚轮通过。

M2回注第一批产品，四场景实红`editing outlet outside viewport`；MD5逐字节还原，touch后四条程序选择测试12 PASS / 0 FAIL / exit0。settingsui vet净；旧V/B/S断言0/0/0。

第二批4318c08a已推送。开盘完整基线6243 PASS / 0 FAIL / exit0；4330be43的cmd/internal测试函数3459。运行结束时对工作树计数会包含新增测试，因此函数基线按开盘树复核。

5. 脚本导出退役：宿主撤回exporter或脚本清空后，原编辑器接住键盘但在Y=-1023.9002 → 只在原导出按钮持焦时显示原代码编辑器 → 两种退役×深浅，原内容/空内容保留、派发0次，直接键入与自由滚轮通过。
6. 导出调用返回：同步/异步导出从页尾Enter后原返焦目标在同一屏外位置 → 不改变返焦时机，只显现原脚本框 → 两入口×深浅精确导出原文件名与字节，派发1次；异步等待保持，完成不覆盖新输入。

导出夹具最初从代码编辑器按Tab，Tab被编辑器消费，没到按钮；该失败仅属夹具，保留M3-fixture-error.txt，不算产品/mutation。改为绑定的按钮设起点后Tab往返，使用真实Enter与状态刷新复现。

M3回注第二批产品，八场景实红`editing outlet outside viewport: rect={X:201.8 Y:-1023.9002 W:487 H:64.8}`；MD5还原、touch后含既有导出等待/重试回归共18 PASS / 0 FAIL / exit0，settingsui vet净。V/B/S旧断言0/0/0。

第三批ebd46d4d已推送。

7. 内容复制退役：command/plan/script能力撤回、command/script变空时，原源字段返焦但在Y=-1097/-1152/-1023.9 → 各动作只在实际返焦时显示自己的字段 → 五种场景×深浅，直接输入落到各自草稿；复制派发0次、确认0条、滚轮自由。plan始终是有效JSON，没有捏造空plan场景。
8. About深链接复制撤回：短窗口先聚焦第二条复制，再滚回顶部，宿主撤回CopyText；链接焦点在Y=369.6而视口底300 → 保留按URL分开的返焦，显示原链接凹槽 → 两条链接×深浅，选择目标可见、Tab可退出、不派发复制、不显示虚假确认。

第一次只显示链接文字仍报`rect={X:208.6 Y:285.00003 W:399.80002 H:15}`，占用像素跨过底300；不加容差，改为显示含内边距的现有chip，严格像素包含通过。此中间失败保留M4-incomplete-reveal.txt。

M4两源文件回注第三批产品，14个深浅场景实红`editing outlet outside viewport`；两文件MD5还原、touch复验，加不抢焦点10场景共27 PASS / 0 FAIL / exit0，settingsui vet净。其他编辑器仍持焦时500px偏移精确保留、名称输入有效。旧V/B/S断言0/0/0。深色导出after目视：代码首行与插入点出现在正文顶端，原命令/版本在下面；浅色链接after目视：register文字及蓝色选择环完整进入凹槽，复制按钮退淡。

第四批5f14e67c已推送，八项四批每批两项完成。完整门槛正在运行；第一次nohup启动未留下进程或exit文件，不计为门槛结果，改用可跟踪工具会话执行同一组命令。

## 偷了什么

- craft-loop工艺菜单 **#3 品牌focus环**：恢复时只有 **1个**原键盘目标，严格包含于正文的占用像素，容差 **0px**；原交互蓝继续指向可编辑值。
- **#4 签名交互**：每次返回只请求 **1次**最小滚动，新增动画 **0ms**；10个非当前出口场景的 **500px** 偏移不变，后续滚轮不锁住。
- **#12 材质细节**：原 **1px** 受光边、**8px** 通用圆角保留，新增玻璃/阴影 **0层**；链接连同原chip内边距一起显现，边线不贴着视口被裁掉。
- Apple DESIGN供体只取单一交互accent、8px矩形圆角、1px边框职责，映射已有token；未复制其网页滤镜、字体或大标题。用户限定已有功能打磨优先于通用菜单凑五项，不为数量加装饰。

这轮为什么更轻、更有空间感：完成或退出动作后，原本藏在屏外的输入重新回到所属内容位置，焦点、内容与下一步操作同时可见；层数不增加。

## 两轮视觉复核

第一轮四视角：艺术总监看到蓝环留在屏外、当前内容失去重点；工程师发现链接仅显现文字仍差1行占用像素；任务所有者看到选择/导出返回后无法看见接下来修改的位置；读者看到脚本/路径的新值未进入视线。第二轮修复原出口显现与chip整体定位，并复查动作返回、键入和后续滚动。没有改为重新设计表单。

60张原始PNG、30组深浅before/after对照、两张八项总览、两张4×局部归档到本轮目录。原图为ui.Tester完整设置页，表单720×520、About短工作区720×300；实际SettingsHeight在300px工作区可降至276（shell.go:5150–5154），不是声称默认窗口只有300px。放大图为最近邻复核，非原生高DPI截图。

| 项 | 图上可见的一处变化 | 浅色对照 | 深色对照 |
| --- | --- | --- | --- |
| 1 选择失败 | 页尾声明换为原程序框，蓝色插入点可见 | [图](P13-kd/picker-error-light-comparison.png) | [图](P13-kd/picker-error-dark-comparison.png) |
| 2 选择取消 | 原路径weather显示在正文顶端，复制行仍退淡 | [图](P13-kd/picker-cancel-light-comparison.png) | [图](P13-kd/picker-cancel-dark-comparison.png) |
| 3 选择成功 | 新路径forecast进入原程序框 | [图](P13-kd/picker-success-light-comparison.png) | [图](P13-kd/picker-success-dark-comparison.png) |
| 4 选择撤回 | 选择按钮消失，扩宽的原路径框可见 | [图](P13-kd/picker-withdraw-light-comparison.png) | [图](P13-kd/picker-withdraw-dark-comparison.png) |
| 5 导出退役 | 导出按钮消失，代码与插入点在正文顶端 | [图](P13-kd/export-withdraw-light-comparison.png) | [图](P13-kd/export-withdraw-dark-comparison.png) |
| 6 导出返回 | 原代码框可见，导出胶囊保持原位置关系 | [图](P13-kd/export-sync-light-comparison.png) | [图](P13-kd/export-sync-dark-comparison.png) |
| 7 复制退役 | 当前命令框出现焦点环与weather文本 | [图](P13-kd/copy-command-withdraw-light-comparison.png) | [图](P13-kd/copy-command-withdraw-dark-comparison.png) |
| 8 链接退役 | register链接选择环完整进入凹槽，两个复制按钮退淡 | [图](P13-kd/link-register-light-comparison.png) | [图](P13-kd/link-register-dark-comparison.png) |

[浅色总览](P13-kd/overview-light.png)、[深色总览](P13-kd/overview-dark.png)、[像素差实数](P13-kd/image-differences.json)。30组均有像素变化，未将重复主题/分支作为独立修复凑数。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/黑/霓虹 | 无新增装饰，原语义主题保留 |
| A2 字体 | 中文系统字体优先于通用换字体要求，字体零改 |
| A3 斜体 | 无新增斜体 |
| A4 Hero | 功能设置页，不适用Hero |
| A5 卡片 | 未增卡片，沿原分组 |
| A6 假数据 | 天气工具与路径只作事件夹具，不进产品 |
| A7 空词/感叹号 | i18n文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原设置标题保留，截图可见 |
| A10 Bento | Go设置表单，不适用 |
| A11 装饰竖线 | 无新增 |
| B1 读取/拨盘 | 功能表单、稳定3/即时2/紧凑7一致 |
| B2 主题 | Tester分别锁定深/浅，单页不混主题 |
| B3 记忆点 | 操作返回时能看见继续修改的原值 |
| B4 动效气质 | 直接显现、新增0ms，与功能页一致 |
| C1 频率/目的 | 仅实际返焦显现，用户移焦后不追随 |
| C2 all/ease-in/scale/fade | 无新增 |
| C3 时长/按压 | 原按钮按压保留，无新过渡 |
| C4 origin | 只滚动原正文，不新建浮层 |
| C5 reduced-motion | 新事件ReduceMotion=true，无新动画 |
| C6 hover媒体 | Go自绘，不适用CSS媒体查询 |
| C7 布局动画 | 无新增 |
| C8 keyframes | 无新增 |
| C9 手势 | 原滚轮继续自由，无新手势 |
| C10 Framer | 不适用Go |
| C11 慢放 | 无新动画，仅验确定帧；未声称真机慢放 |
| D2-1 字体 | 系统中文字体，无webfont |
| D2-2 字号/行高/字重 | Caption9.9、正文10.8仍不符合通用≥14px/1.5，沿用需拍板，未擅改高度链 |
| D2-3 混排 | 产品文案零改，无新增中文句子 |
| D2-4 标点/对比 | 原标点/墨不变，未声称全部次要墨达到7:1 |

可复现缺陷沉淀到§6.257与事件pin，不升级为全局审美偏好。没有K3、其他执行器或外发消息。Windows/macOS仅编译，原生选择器未截图，宿主回调由注入驱动。

## 仍待

1. 下一格首选已有运行参数表单的取消、重试与条件字段退役：长参数列表里原返焦目标是否可见，先真实事件复现；不重复本轮选择/导出/内容复制与链接出口。
2. 生成工具编辑/转换覆盖、重复新包ID覆盖、重装事务、配置模板失败重试、另一端冲突、字号/行盒、accent禁用填充、未知多选/Radio、极小步长、176-9/161-4保存期草稿、终端S1b/S1c/S2尾沿用需拍板。
3. 八条早期WIP逐条保留。旧ImageCopy/Rescan/memo线索不弱化B断言、不加睡眠或容差消红；本轮未改保存与终端语义。

## 最终门槛与边界

最终 **6292 PASS / 0 FAIL / 3468函数**，开盘6243/3459，增加 **49 PASS / 9函数**。普通全量与verbose全量均 **exit0**；gofmt输出 **0字节**，linux/windows/darwin的vet/build六项 **exit0**。完整原始日志与exit文件都在本轮目录。产品四批e75f97b0/4318c08a/ebd46d4d/5f14e67c已推送，结案只归档证据与交接文档。

M1/M2/M3/M4实红分别4/4/8/14个深浅场景（不含父级FAIL）；每组逐字节MD5还原后touch复验，最终产品MD5见`P13-kd/final-source.md5`。M4中间的1px越界没有放宽容差，已通过整体显现chip修正。测试新增9个函数，其中8个修复测试、1个非当前编辑保护测试；旧函数删除0、新增Skip0。

07:59:35 UTC确认全部完成。首个明确时钟07:30:02至门槛完成约30分钟，开盘阅读在此前；短于45–75分钟目标，没有重复测试或扩张无关改动凑时长。因全量进程等待和中间像素失败复核，工具调用超出40预算；八项产品已按每两项即时推送，超额阶段仅补必要门槛与交接，未另开选题。首次nohup启动未产生检查记录，后续可跟踪会话完整完成，无跳测或掩盖失败。

Windows/macOS仅vet/build，未作原生设备交互；Linux自绘截图覆盖30组前后场景。需拍板项与八条早期WIP均保留。完整基线见[baseline](P13-kd/baseline.txt)，最终[普通全量](P13-kd/full-test.txt)、[verbose全量](P13-kd/full-verbose.txt)、[像素差](P13-kd/image-differences.json)可复核。
