# P13-is：菜单与表单刷新的身份和恢复

07:00 UTC 前后开盘，起点 `753d3979`，pull 最新、工作树净。基线 **5025 PASS / 0 FAIL / 3101 函数**。只用 Codex 独立执行，读取 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN。

## Stash 清点

八条仍为此前逐轮登记的早期独立 WIP；本轮逐条 reverse-check 全部退出 1。stale-kill 已由历史轮次接续归档；下表保留既有处置，不把旧主题/Tauri 链覆盖当前产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

done.log 与 stash-backlog 的已知路径本机未提供；以 git stash list 和仓库前轮处置为准。未全盘搜索。

## 进展

P13-hx 已覆盖任务书中目标仍在但 owner/schema/状态变化的八条确认线索；本轮接实际最新 P13-ir 的菜单刷新遗留，不重复计数。

## 第一批：语言只换标签

1. 现象：参数菜单高亮 Number 后切到中文，Enter 留在 text。改动：参数类型按已有 paramKinds ID 选择，译文只显示。验证：深浅真实 Tab/Down/Enter，最终 number，类型入口保持焦点。
2. 现象：驻留高亮 10 秒后切中文，Enter 留在 0。改动：按已有 choices ID 选择。验证：深浅真实事件最终 10 秒，焦点仍在驻留入口。

相关 **25 PASS / 0 FAIL**，包 vet 净。回注两处旧实现 **6 FAIL**，原串 `translation lost parameter highlight: text`、`translation lost residency highlight: 0, want 10`。MD5 两文件 OK，touch 后 **6 PASS**。V/B/S 旧断言修改 **0/0/0**。原帧已入仓。

整行动作菜单移除和重排探针为绿，不计修复。下一批已复现 command-refresh 和 palette-unknown 覆盖新值；其测试暂放 /tmp，待修后单独提交。

第一批 `8662be1f` 已推送。目视深色：切中文后“数字”仍蓝色高亮，焦点环留在类型入口；原图蓝条回到“文本”，准备选择的位置丢失。

## 第二批：显示回落不能掩盖原值刷新

3. 现象：动作菜单展开后，宿主把命令从 say original 改为 say updated，两者都显示命令行；Enter 却提交旧高亮 plugin:browser。改动：选择器额外观察原始命令源值，源变而显示槽不变时退役旧菜单、保留键盘入口。验证：深浅真实 Down/Home/Enter，宿主新命令保留、写入 0 次，入口仍有焦点。
4. 现象：调色板被宿主更新为未知 ID 后，控件把它伪装为首项，Enter 写回旧 contrast。改动：保留未知 ID，复用现有未知值菜单退役路径。验证：深浅真实事件保留 retired-palette、写入 0 次，焦点可继续选择。

包 vet 净；相关与 mutation 实测计数见同目录 batch2-test/M2 日志。回注旧实现原串 `command-refresh overwrote refreshed value: "plugin:browser" writes=1`、`palette-unknown overwrote refreshed value: "contrast" writes=1`；MD5 还原、touch 后复验。V/B/S 旧断言修改 **0/0/0**。

第二批 `c626a424` 已推送；相关 **92 PASS**，M2 **6 FAIL → MD5 OK → touch → 6 PASS**。

## 第三批：声明整块退场的两个入口

5. 现象：参数名称输入到 Edited target 时宿主移除声明，Enter 无法继续添加参数。改动：记录正在使用的参数板，仅当该板退场且仍拥有焦点时归还 Add。验证：深浅真实 Tab/Type/Enter，移除后 Add 聚焦，Enter 新增一条；不改变原保存时机。
6. 现象：参数类型菜单展开时宿主移除声明，浮层撤走但键盘没有出口。改动：参数板跟踪覆盖菜单触发器的焦点归属。验证：深浅 Down 展开后移除，Add 保有焦点，Enter 可新增。

相关测试与包 vet 通过。M3 回注旧实现，原串 `param-editor-removed lost Add outlet`、`param-menu-removed lost Add outlet`，以及各自 `could not add after refresh`；MD5 还原 touch 后两主题复验。旧 V/B/S **0/0/0**。驻留两项仅见红，尚未计为完成。

第三批 `8e786da8` 已推送；相关 **20 PASS**，M3 **6 FAIL → MD5 OK → touch → 6 PASS**。

## 第四批：驻留自定义选择与提交出口

7. 现象：存储为关闭，选择自定义后触发器仍读关闭；再选关闭不触发 Changed，输入无法收起。改动：输入展开期间选择器保持 custom 这个尚未保存的 UI 选择。验证：深浅真实 Down/End/Enter 打开，再 Home/Enter 选关闭，输入收起、存储仍为 0。
8. 现象：自定义输入 30 秒后 Enter，值已存且输入退场，键盘焦点随输入消失。改动：提交落在预设值导致输入关闭时，同帧聚焦原选择器；按钮提交共用出口。验证：深浅真实 Tab/Type/Enter，存储 30、输入关闭、原驻留入口聚焦。

相关测试和包 vet 通过；M4 回注旧实现原串 `selecting stored Off cannot dismiss custom draft`、`preset apply removed focused input without returning to selector`；MD5 还原 touch 后复验。旧 V/B/S **0/0/0**。八项跨参数编辑、驻留、动作与调色板组件；不新增系统、依赖、页面，不改确认窗口或终端语义。

第四批 `84a1380a` 已推送；相关 **35 PASS**，M4 **6 FAIL → MD5 OK → touch → 6 PASS**。

## 偷了什么

本轮按用户指定已有方向打磨；读取 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography，以及 Apple DESIGN 的交互色和按钮表。

- craft-loop **#3 品牌 focus 环**：Apple 单交互蓝角色与 **2px** 环沿本仓 token；参数退场和驻留提交的焦点交接 **0ms**，Add 的环有 **288 像素**可测变化，驻留入口为 **1000 像素**。
- **#4 签名交互**：沿已有选择器 Down/Enter，以 **0 次**刷新写入、**1 次**明确提交为验收；不是另造动画。命令刷新后撤回旧菜单，原入口仍可继续操作。
- **#6 编辑细节**：类型保留语义 ID，驻留输入展开时显示“自定义…”，额外说明占高 **0px**。保留现有 **1px** 发丝与原行密度。
- **#12 材质细节**：沿 Apple **8px** 控件圆角角色与本仓 E1/E2；新增透明面 **0 层**、新增阴影 **0 层**。只撤回失效浮层，已有空间层级继续表达可操作状态。

用户要求修已有功能优先，不为工艺菜单凑五项增加装饰。为什么更轻、更有空间感：旧浮层不再盖住新状态，焦点在现有面之间有可追踪的去向，展开的输入与菜单只表达同一状态。

## 两轮图像复核

32 张深浅原帧与 16 张对照入仓：[总览](P13-is/overview.png)、[像素差](P13-is/image-differences.txt)、[浅色局部](P13-is/focus-detail-light.png)、[深色局部](P13-is/focus-detail-dark.png)。每项有 `floter-P13-is-<场景>-<dark/light>-before/after/comparison.png`。原帧为 Linux ui.Tester 软件自绘，720×400/440/700，缩放总览与 4× 最近邻局部不冒充高 DPI 真机截图。局部已目视确认 Add 环与驻留环均完整归还；代表图另存 `/tmp/floter-P13-is-settings-{dark,light}.png`。

第一轮四视角：艺术视角看见过期菜单遮着新状态；工程视角看见 label 身份和原值被显示回落掩盖；任务用户在参数移除后不能按 Enter 继续添加；读者遇到自定义输入打开、选择器却说关闭的矛盾。第二轮保留原布局，逐项核对菜单高亮、撤回、Add 焦点、驻留回到入口四处。

实际目视：深色参数菜单的蓝条留在数字；深色未知 palette 显示原 ID 且旧菜单消失；浅色自定义驻留左侧由“关闭”变为“自定义…”；参数空态右上角 Add 出现原有蓝环。全部八项深浅图像差非零，最小 **238 像素**（自定义文字），未以静态不变量凑数。字号仍有旧问题，本轮不冒充修复。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿原近黑和浅色 token |
| A2 字体 | 原生中文系统栈优先于泛化“每轮换字体” |
| A3 斜体 | 无新增 |
| A4 Hero | 原生功能设置，不适用 |
| A5 三等分/卡片海 | 无新增卡片 |
| A6 假数据 | 无产品假指标；测试命令与未知 palette 为受控边界夹具 |
| A7 空词/感叹号 | i18n 文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原标题保留，本轮图为控件和编辑器表面 |
| A10 Bento | 不适用 |
| A11 竖线装饰 | 无新增 |
| B1 读取/拨盘 | 2/1/6，稳定、即时、原密度 |
| B2 主题锁定 | 深浅 Tester 分别固定 |
| B3 记忆点 | 高亮与键盘位置始终有当前归属 |
| B4 动效气质 | 高频操作反馈 0ms |
| C1 目的/频率 | 未增加高频动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 沿原 120ms 色彩过渡与 0.92 小图标按压 |
| C4 origin | SelectBase 仍锚定原触发器 |
| C5 reduced-motion | 新事件全部 ReduceMotion=true，无新增时间依赖 |
| C6 hover 媒体查询 | Go 自绘，无 CSS；原指针规则保留 |
| C7 布局动画 | 无新增 width/height/margin 动画 |
| C8 快速触发 | 原框架即时事件，无重播 keyframes |
| C9 手势 | 未涉及拖拽/弹簧，不适用 |
| C10 Motion | 无 Web Motion 运行时，不适用 |
| C11 慢放 | 无新动画；已逐 Frame 查刷新与 Enter 后状态，未声称真机慢放 |
| D2-1 字体 | 中文系统栈、无 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px/1.5 仍未满足；原 Caption 9.9/正文 10.8 与行盒待拍板 |
| D2-3 混排 | 原 i18n 不改，无新增产品混排 |
| D2-4 标点/对比 | 原标点和墨保留，不宣称全部次要墨达 7:1 |

缺陷沉淀到本仓 §6.220 与事件测试；不把项目修复写成全局 skill 审美偏好。

## 仍待

1. 下一轮先实测驻留自定义草稿打开时，宿主另写预设/自定义值后草稿、菜单和取消出口的组合；本轮只修明确选择和提交的既有路径，未更改外部写回与草稿优先级。
2. 动作词表空表索引是代码线索，出厂静态词表始终非空，未用该假设凑修复；若产品实际可触发先事件复现。绑定整行展开菜单移除/重排探针已绿，不重复选题。
3. 字号/行盒、accent 禁用填充、未知多选/Radio、极小数字步长、176-9/161-4 保存期草稿、冲突另一端变化的拒绝历史、终端 S1b/S1c/S2 尾继续待拍板。
4. Windows/macOS 只编译检查，尚无真机输入法、Option+字母或同帧多事件证据。八条早期 WIP 原件保留。

## 完整门槛与交接

最终产品 **84a1380a**，四批两项均通过定向回归和包 vet 后立即 commit/push。完整门槛集中收尾，普通和 verbose 全量均退出 **0**。最终 **5050 PASS / 0 FAIL**（基线 5025，+25），测试函数 **3110**（基线 3101，+9，含一条原本为绿的绑定移除/重排保护）。所有旧测试函数和断言零修改，新增 Skip **0**，V/B/S **0/0/0**。

gofmt 空，Linux/Windows/macOS vet/build 全过；文档 CI 通过。四组 mutation 均 **6 FAIL**，MD5 逐字节还原并 touch 后各 **6 PASS**。最终四个产品文件与三个新测试文件 MD5 全部 OK。完整普通测试、门槛、计数、最终 MD5 入仓；完整 verbose 与基线保留 `/tmp/floter-P13-is/`。

证据：[普通全量](P13-is/full-test.txt)、[全量计数](P13-is/test-summary.txt)、[三平台](P13-is/gates.txt)、[文档 CI](P13-is/docs-ci.txt)、[源码 MD5](P13-is/final-source.md5)。32 原帧、16 对照、两张 4× 局部及总览均已目视复核。未知 palette 是控件接受的真实边界输入；不据此宣称验证了不存在的 palette 预览。Windows/macOS 只有编译证据。

07:00 UTC 前后开盘，07:33 UTC 完整门槛结束，约 **33 分钟**，低于 45–75 分钟目标；八项及验收已完成，不为填时长加无关修改。外层编排在 40 次内；若把编排内每个子工具和轮询都计入，已超 40，调用预算口径偏差如实记录。没有因省调用跳过门槛或修改旧 pin。

开盘原 done.log 未提供，本轮收尾将按任务书路径创建；下一步已替换到 `.pending-bugs.md` 顶部，八条早期 WIP SHA 与清点表一致、全部保留。
