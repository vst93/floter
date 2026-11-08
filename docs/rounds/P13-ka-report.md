# P13-ka · 本地工具导出等待与表单拒绝后的编辑出口

开盘 2026-11-08 05:38:38 UTC，HEAD e91168f3，pull 已最新、工作区干净。仓库最新为 jz，指定安装恢复已在 jx 完成。指定 done.log 开盘不存在。Codex 独立执行，无其他代理、模型或依赖。

## stash 清点

逐条实跑反向校验均 exit1，SHA 与交接一致；八条早期独立 WIP 的语义审计沿用交接结论，不覆盖新主题或恢复旧 Tauri 链。本轮无新增 stale-kill、无 drop。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4e42eb518f4a9732465cc31be84002078 | 旧 keycap/主题 | 保留 |
| stash@{1} | 8af94a07586e56646fe74cc4da76355ecb0cc343 | 旧行距/滑块 | 保留 |
| stash@{2} | 7e4aa710feb96f0afdb6328418ded0ea3d39051d | 旧卸载布局 | 保留 |
| stash@{3} | 1e3aec9484cb2be99e73171beb05cd6806de0127 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42e2825109a5f2688831cb0495434e7444 | 旧字段/确认栏 | 保留 |
| stash@{5} | 9b8ce23cbd7a74c9fa9c92faae340b5881602070 | 旧快捷键 | 保留 |
| stash@{6} | 90d314b27296c6ee60ad29297ae1a26a8ea36165 | 旧动作栏 | 保留 |
| stash@{7} | 7a9f6e136bb2b7af5265f18ada33b408c9c4f6f7 | 旧 Tauri 发布链 | 保留 |

## 增量记录

规格已落盘，开盘全量测试运行中；后续只填写实测数字。当前未提交产品修改。

初次全量继承 LC_ALL=C.UTF-8，覆盖了 LANG=zh_CN.UTF-8，导致既有中文字体行盒 pin 失败。原始日志保留；不是本轮产品回归。显式 LC_ALL=zh_CN.UTF-8 后两个旧行盒测试通过，正在用开盘源码 overlay 重测全量。六类拒绝出口均已真实复现，编辑/连接 × 深/浅每类四组。

### 第一批：名称与命令

3. **名称拒绝后键盘落空**：现象 → 提交空名称后焦点留在提交按钮，直接键入不改名称；改动 → 将第一个名称错误关联现有输入焦点，只在提交拒绝时返回；验证 → 连接/编辑 × 深/浅四组直接输入“天气工具”，再次提交恰一次。
4. **命令拒绝后键盘落空**：现象 → 非法命令“bad command”被拒绝后无法直接修正；改动 → 返回既有命令字段；验证 → 四组直接改成 forecast，再提交恰一次，不改验证规则。

定向 **10 PASS / exit0**，settingsui vet exit0。M1 撤回两个目标后实红 `name refusal lost editable keyboard exit` / `command refusal lost editable keyboard exit`，以及 `refused direct keyboard correction: submissions=0`；MD5 还原成功，touch 后 **10 PASS / exit0**。V/B/S 旧断言修改 0/0/0，无新 Skip。

第一批 d2b4dcea 已推送。对照图肉眼看到名称输入获得蓝色焦点环，其余字段和面层保持不动。

### 第二批：版本与程序

5. **版本拒绝后键盘落空**：现象 → “version one”拒绝后直接键入无法修正；改动 → 返回当前版本输入；验证 → 连接/编辑 × 深/浅四组直接改为 2.3.4，再提交一次。
6. **空程序拒绝没有出口**：现象 → 空程序提交静默返回，输入仍落在按钮；改动 → 返回已有程序字段，不增加文案或改空值规则；验证 → 四组直接输入 /usr/bin/weather，再提交一次。

M2 撤回两个目标，实红 `version refusal lost editable keyboard exit` / `program refusal lost editable keyboard exit`；还原 MD5、touch 后累计 **20 PASS / exit0**，settingsui vet exit0，旧 V/B/S 0/0/0。

第二批 93b2a4ad 已推送，但发生提交时序错误：图片汇总后才运行的 git add 尚未结束，就开始第三批修改，导致该提交误带第三批的两个新测试及尚未还原的空焦点分支。版本/程序两项自身已过门槛；整份第二批树不满足新脚本/平台测试，不能声称其全绿。第三批立即补回两个焦点目标并验证，不重写远端历史。后续所有提交明确等待完成才改源码。

### 第三批：脚本与平台

7. **空脚本拒绝无法直接补写**：现象 → 空脚本拒绝后输入未进入代码编辑器；改动 → 返回原编辑器焦点，保留已有缓冲；验证 → 四组直接输入 print('sunny') 后提交一次。
8. **平台全清后没有键盘选择出口**：现象 → 提交后 Space 仍按提交按钮；改动 → 聚焦平台行第一个既有复选框；验证 → 四组 Space 补选平台后提交一次，不自动选择、不更改平台限制。

M3 撤回脚本/平台目标后实红 `script refusal lost editable keyboard exit` / `platform refusal lost editable keyboard exit`；MD5 还原、touch 后累计 **30 PASS / exit0**，settingsui vet exit0。旧 V/B/S 0/0/0。

第三批 41aab284 已推送，六类基本表单出口已全绿。中文 locale 开盘源码 overlay 的完整基线 **6101 PASS / 0 FAIL / exit0**。

### 第四批：导出等待与编辑出口

1. **导出等待重复打开保存窗口**：现象 → 连点导出派发两个保存对话框，换表单还能继续派发；改动 → 接通宿主完成通知，当前导出出口等待期间保留原几何并显示禁用面；验证 → 深浅两组连续点击只派发一次、换表单仍等待，完成后恰一次导出 forecast-script.py 与新脚本内容。
2. **导出取消或失败后不能接着写脚本**：现象 → 焦点停在动作按钮，直接输入落空；改动 → 派发后把自己持有的焦点交回原脚本，晚完成只解锁、不夺回用户已移动的焦点；验证 → 深浅原编辑器和主动改焦点四组直接输入正确落地，真实宿主 saveFile 的取消/选择失败/写入失败/成功四支各完成一次且可重试，取消无通知。

新旧定向回归 **146 PASS / 0 FAIL / exit0**。M4 撤回等待闸和返焦后实红 `waiting export dispatched 2 save dialogs` / `export completion lost current keyboard exit: 脚本` / `export return dropped keyboard correction`；MD5 还原、touch 后 **13 PASS / exit0**，settingsui/shell vet exit0。旧直接宿主导出的并发通知 B 测试保持通过。无新增动画、无几何变化；只复用禁用面，不新增 spinner。

第四批 197c3c50 已推送。

### 第五批：同类遗漏补齐

9. **参数声明拒绝后无法修正对应字段**：现象 → 第二条声明的 ID、重复 ID、空选项、缺 flag、非法 flag、数字/布尔/选择默认值八类错误都只拒绝提交，直接输入落在按钮；改动 → 按原验证顺序找到第一个错误行，焦点精确落到该行对应字段，绑定按表单与声明身份隔离；验证 → 八类 × 深浅共 16 组直接输入后可提交，第一条有效声明原样保留。既有参数删除和选项退出回归一起通过。

M5 撤回提交拒绝的返焦调用后实红 `id declaration refusal lost its field`、`duplicate declaration cannot correct and retry`（其余六类同样红）；MD5 还原、touch 后复验通过。该补充只完善同表单错误出口，不更改任何参数验证或保存语义。

第五批 a65102d1 已推送，定向 **27 PASS / 0 FAIL / exit0**，settingsui vet exit0。累计九项，跨导出动作、基本字段、平台选择、参数声明与宿主完成回调。

## 偷了什么

实际读取 qiaomu-design SKILL、偏好账本、craft-loop、preflight、chinese-typography；按功能表单补读 Carbon patterns、Apple DESIGN。轻便液态玻璃方向已定，走已有方向打磨；没有四方向预览、外部模型或新依赖。

- craft-loop 工艺密度菜单 **#3 品牌 focus 环**：落成 **1 个明确编辑焦点、额外等待 0ms**；六类基本字段及参数声明只在提交被拒绝时定位，沿用已有 accent。
- 菜单 **#4 签名交互**：落成 **1 个等待导出、0 次重复派发**，对话框四种结束分支都通知完成；晚完成返焦 **0 次**，新焦点继续编辑。
- 菜单 **#12 材质细节**：沿用 **1px 受光边**，等待采用原禁用面熄灯；按钮位置和尺寸前后严格相等，新增玻璃/阴影/动画 **0 层**。

Apple 供体的单一交互蓝、8px 矩形控件半径、平面无投影三项映射已有 token；不移植网页字体、负字距或 backdrop-filter。用户修已有问题与不扩张的约束优先于凑五项装饰。

**这轮为什么更轻、更有空间感**：等待由原按钮退到不可操作的面档，错误由原字段焦点接住；层级表达真实状态，不用增加说明卡或弹窗。

## 两轮图像复核

第一轮四视角：艺术总监看到等待按钮仍像可按状态；工程师看到提交拒绝后的焦点脱离错误字段；任务用户看到取消导出后接着输入无效；读者看到参数错误在第二张声明里却没有编辑起点。第二轮分别收敛等待材质、基本字段出口、导出编辑出口和参数精确定位。

肉眼检查：浅色名称对照的蓝环回到名称槽；深色导出等待按钮熄去受光边而宽度不变，脚本插入点可见；浅色第二条参数选项框得到焦点，第一条有效声明没有变化。1× 查看完整 720×1500 自绘帧，4× 最近邻局部用于检查边与焦点，不冒称原生高 DPI。画布高度为测试观察范围，不改产品高度链。

| 项 | 浅色对照 | 深色对照 |
| --- | --- | --- |
| 1 导出等待 | [图](P13-ka/export-wait-light-comparison.png) | [图](P13-ka/export-wait-dark-comparison.png) |
| 2 导出编辑出口 | [图](P13-ka/export-return-light-comparison.png) | [图](P13-ka/export-return-dark-comparison.png) |
| 3 名称 | [图](P13-ka/refusal-name-edit-light-comparison.png) | [图](P13-ka/refusal-name-edit-dark-comparison.png) |
| 4 命令 | [图](P13-ka/refusal-command-edit-light-comparison.png) | [图](P13-ka/refusal-command-edit-dark-comparison.png) |
| 5 版本 | [图](P13-ka/refusal-version-edit-light-comparison.png) | [图](P13-ka/refusal-version-edit-dark-comparison.png) |
| 6 程序 | [图](P13-ka/refusal-program-edit-light-comparison.png) | [图](P13-ka/refusal-program-edit-dark-comparison.png) |
| 7 脚本 | [图](P13-ka/refusal-script-edit-light-comparison.png) | [图](P13-ka/refusal-script-edit-dark-comparison.png) |
| 8 平台 | [图](P13-ka/refusal-platform-edit-light-comparison.png) | [图](P13-ka/refusal-platform-edit-dark-comparison.png) |
| 9 参数声明 | [图](P13-ka/refusal-param-options-light-comparison.png) | [图](P13-ka/refusal-param-options-dark-comparison.png) |

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 紫蓝渐变/纯黑/霓虹/渐变字 | 未新增，沿用既有深浅 token |
| A2 字体 | 中文系统字体优先，不套网页换字体规则 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生功能表单 |
| A5 三等分/万能卡片 | 无新增卡片 |
| A6 假数据 | 产品无新增假数据；天气工具为隔离事件夹具 |
| A7 空词 | i18n 零改，无新增产品文案 |
| A8 眉批 | 无新增 |
| A9 标题行数 | 原有标题完整可见 |
| A10 Bento | 不适用 |
| A11 装饰竖线 | 无新增 |
| B1 读取/拨盘 | 3/2/7 对应可预期、即时、保留信息的原生表单 |
| B2 主题 | 每张图固定一套明暗，深浅分拍 |
| B3 记忆点 | 失败后可以直接修正，不再寻找输入起点 |
| B4 动效气质 | 即时反馈，无新动画 |
| C1 目的/频率 | 高频键盘操作没有额外动效 |
| C2 all/ease-in/scale(0)/fade | 未新增 |
| C3 时长/按压 | 新过渡 0ms；按压反馈沿用 |
| C4 origin | 返回自己所属的编辑字段，不增加弹层 |
| C5 reduced-motion | 测试显式 ReduceMotion=true，无新动效 |
| C6 hover 媒体查询 | Go 自绘无 CSS 查询；现有指针模型不变 |
| C7 布局动画 | 无新增；导出等待几何严格相等 |
| C8 重播 keyframes | 无新增 |
| C9 手势 | 本轮不涉及 |
| C10 Framer | 不适用于 Go 自绘 |
| C11 慢放 | 无新动效，仅核验确定帧，不声称真机慢放 |
| D2-1 字体 | 系统中文，无 webfont |
| D2-2 字号/行高/字重 | 原 Caption 9.9 / 正文 10.8 等通用 ≥14px 限制继续需拍板；未越权改字阶/行盒 |
| D2-3 中西文空格 | i18n 零改，无新产品句子 |
| D2-4 标点/暗色对比 | 原标点与墨沿用，不冒称所有次要墨达到 7:1 |

自发现缺陷与断言沉淀在本仓 §6.254；不把局部修复升级为全局审美规则。原生保存窗口未截图；文件操作和通知走真实宿主逻辑，saveFile 答案确定性注入。Windows/macOS 仅编译验证。

## 仍待

1. 下一轮首选本地工具表单的窄视口错误可见性与高级权限折叠出口：先复现错误字段是否随焦点滚入可见区域、权限项消失后键盘是否仍有出口；不改高度链，不重复本轮验证条件和基本字段返焦。
2. 生成工具编辑/转换覆盖、重复新包 ID 覆盖、重装事务、配置模板失败重试、另一端冲突、字号/行盒、accent 禁用填充、未知多选/Radio、极小步长、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾继续需拍板。
3. 八条早期 WIP 原样保留，旧 ImageCopy/Rescan/memo 时序线索不改旧 B 断言或放宽容差。

## 最终验收

2026-11-08 **06:11:13 UTC** 门槛完成，首个时钟 05:38:38 至验收约 **32 分 35 秒**，短于 45–75 分钟目标；九项及全部门槛已经完成，没有以无关改动或重复测试凑时间。

- 开盘源码 overlay、中文 locale：**6101 PASS / 0 FAIL / exit0**。
- 最终普通与 verbose 全量分别 exit0，verbose **6161 PASS / 0 FAIL**（+60）。
- gofmt 输出空；linux/windows/darwin 的 vet/build 全部 exit0。
- 导出异步链 race：**13 PASS / 0 FAIL / exit0**；四份产品源码 MD5 全部 OK。
- cmd/internal 按同一函数声明命令统计 **3431→3441**（+10）。上轮文字为 3433，本轮不沿用该数；基线直接对 e91168f3 执行 git grep 计数，最终按同一范围计数。旧测试文件零修改、旧函数零删除、新增 Skip 0，旧 V/B/S 断言修改 **0/0/0**。
- M1–M5 均实红并逐字节还原，touch 后分别 **10 / 20 / 30 / 13 / 27 PASS**；具体原始失败串见各批及日志。

92 张原始 PNG、46 组对照、4 张 4× 局部已归档。44 组确有变化，最小 **34 像素**；两张 export-return-moved 对照零变化是主动移焦未被夺回的正确结果。平台鼠标帧没有焦点可见环，补拍时通过 Shift-Tab/Tab 原位往返进入键盘模式，前后均不选择平台；每组变化 **3072 像素**。肉眼复核浅色平台图，焦点从保存按钮移到 macOS 复选框。补拍使用同一最终夹具与 Go overlay 的旧目标，原行为断言仍实红；产品源码不改，当前源码 MD5 复核仍一致。

产品五批已推送：**d2b4dcea / 93b2a4ad / 41aab284 / 197c3c50 / a65102d1**。第二批误带未完成分支的过程偏差和第三批纠正见上文；最终全量绿不能回填成中间树绿。第一次 git diff --check 被原始测试日志中 Go usage 的空格+Tab 拒绝，随后只对源码/Markdown 做空白检查，日志逐字保留；一次文档补丁上下文不匹配未改文件；两者未当成产品缺陷。

证据：[基线](P13-ka/baseline.txt)、[普通全量](P13-ka/full-test.txt)、[verbose 全量](P13-ka/full-verbose.txt)、[race](P13-ka/focused-race.txt)、[各门槛退出](P13-ka/gates.exit)、[最终源码 MD5](P13-ka/final-source-check.txt)、[图像像素差](P13-ka/image-differences.json)。旧 locale 原始失败、五组变异原始错误、还原记录与每条门槛的 txt/exit 均入仓。
