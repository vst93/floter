# P13-ir 报告：展开选择器刷新后的选项身份

开盘：2026-11-07 UTC。起点 `7c54e11d`，工作树净；中文 locale 基线为 5001 PASS / 0 FAIL，测试函数 3093。八条早期 WIP stash 均已登记，reverse-check 失败且保留。

## 进展

- 规格已写入 `P13-ir-spec.md`。读取为已有原生设置页，面向键盘使用者，延续轻便液态玻璃；稳定程度 2/10、动效 1/10、密度 6/10。先守“看见哪项就选中哪项”的任务契约。
- 首次批量替换未先复现，且 palette/residency 初值仍传 label，已全部撤回；一次包测试漏中文 locale 引发字体度量误报，未改旧断言。此过程不计完成项，原始 diff 在 `/tmp/floter-P13-ir/first-attempt.diff`。后续均按中文 locale 真实事件见红再修。

## Stash 清点

八条 SHA 与 backlog/前轮一致，无新增 stale-kill；逐条 reverse-check 均退出 1，沿既有语义审查保留旧 WIP，不恢复旧主题或 Tauri 链。

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

## 第一批

1. 现象：动作菜单展开并高亮浏览器后，宿主改为计算器，Enter 又写回浏览器。改动：复用现有 valueSelect 的外部值同步。验证：深浅真实 Tab/Down/Enter，刷新写入 0 次、Enter 保持计算器并保留焦点。
2. 现象：展开动作菜单重排后，Enter 按旧索引选中剪贴板。改动：高亮随值重排。验证：深浅真实事件，Enter 只写浏览器一次，触发器焦点仍在。

相关 **64 PASS / 0 FAIL**，包 vet 净；旧 V/B/S **0/0/0**。M1 恢复原 custom.go，**6 FAIL**，原串 `custom external selected "plugin:browser", want "plugin:calculator"`、`custom reorder selected "plugin:clipboard", want "plugin:browser"`；MD5 还原 touch 后 **6 PASS**。

第一批 `44dbb0ef` 已推送。

## 第二批

3. 现象：调色板菜单展开后，宿主从 inherit 改为 dracula，Enter 又写回 contrast。改动：复用当前值刷新高亮。验证：深浅真实 Down/Enter，外部值保持 dracula、没有多余回调、焦点留在触发器。
4. 现象：调色板选项重排后，Enter 落回 inherit。改动：保存选项值身份。验证：深浅真实事件，重排后只提交 contrast 一次；原有调色板预览回归同时通过。

其余四项已见红但测试待对应批次再启用；不提交未修复的红测试。旧 V/B/S **0/0/0**。

第二批 `d7989fcf` 已推送。相关 **19 PASS / 0 FAIL**，包 vet 净；M2 **6 FAIL**，原串 `palette external selected "contrast", want "dracula"`、`palette reorder selected "inherit", want "contrast"`；MD5 还原 touch 后 **6 PASS**。

## 第三批

5. 现象：两个动作标签相同时，按 Down 选择第二项会被去重成下一动作。改动：选择器值改为动作 ID，标签仅负责显示。验证：深浅真实键盘选择第二个同名项，只提交 plugin:browser 一次。
6. 现象：两个调色板同名时，第二项无法独立选中。改动：传入 palette ID，初值也改为 ID；预览代码继续取实际 ID。验证：深浅真实键盘选择第二个同名项，只提交 contrast 一次。测试参数包含自定义 dracula ID，仅验证选项身份；它不在当前九种内置 palette 中，不能据其截图声称验证了 dracula 预览。

相关 **44 PASS / 0 FAIL**，包 vet 净；旧 V/B/S **0/0/0**。M3 回注按 label 选择的实现，**6 FAIL**，原串 `custom duplicate selected "plugin:calculator", want "plugin:browser"`、`palette duplicate selected "dracula", want "contrast"`；MD5 两文件还原 touch 后 **6 PASS**。

第三批 `051abe6d` 已推送。

## 第四批

7. 现象：参数类型菜单已经高亮数字，宿主更新为开关，Enter 写回数字。改动：参数选择器接回现有 valueSelect 外部值同步。验证：深浅真实 Down/Enter，新类型保持开关，键盘仍停在类型选择器。
8. 现象：驻留菜单已经高亮 20 秒，宿主改为 120 秒，Enter 写回 20 秒。改动：驻留选择器复用外部值高亮同步，保留原 label 到秒数映射和自定义保存规则。验证：深浅真实 Down/Enter，存储仍为 120，焦点保留。

相关 **45 PASS / 0 FAIL**，包 vet 净；旧 V/B/S **0/0/0**。M4 回注旧参数和驻留选择器，**6 FAIL**，原串 `open parameter menu overwrote external kind with "数字"`、`open residency menu overwrote external 120 with 20`；MD5 两文件还原 touch 后 **6 PASS**。

32 张深浅原帧、16 张对照和总览已入仓。先目视复核整体和深色动作局部：旧图触发器是计算器、菜单却亮浏览器；新图两者一致，2px 蓝环仍围绕原入口。驻留浅色由错误的 20 秒蓝条移到 120 秒。最小差 **11229 像素**；包含复用现有菜单材质产生的像素差，不能全部归因于高亮。同名动作 before 是第一批后的 label 去重实现，仍有第二项不可选择的原缺陷，未冒充起点原帧。

完整门槛实测结果见文末；上述定向数字分别记录每批的独立回归。

第四批 `30c14be8` 已推送。八项跨动作选择器、调色板、参数类型和驻留四个组件，四批各两项。Linux ui.Tester 为真实键盘事件和软件自绘；Windows/macOS 仅 vet/build，不声称真机验收。

## 偷了什么

已读 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography 和 Apple DESIGN 角色/按钮表。用户已经指定唯一方向，直接打磨；项目合约优先，不创建网页预览和第二设计来源。

- craft-loop **#3 品牌 focus 环**：取 Apple 单交互蓝与 **2px** 焦点环，沿本仓配色；刷新高亮 **0ms** 同步。八种场景 Enter 后仍保有唯一触发器焦点。
- **#6 编辑细节**：保留 **1px** 发丝和原文案列；调色板 **220px MinWidth**、驻留 **200px Width** 沿用。去掉旧索引造成的错误高亮，不加解释条，额外说明占高 **0px**。
- **#12 材质细节**：沿现有选择器 **RadiusSM 8px** 与胶囊触发器；菜单复用已有浮层阴影和圆角，新增透明层 **0**、新容器 **0**。焦点层始终对应当前菜单值。

不为凑五项加装饰。为什么更轻、更有空间感：原面上的当前值与浮层上的蓝色高亮只表达同一个选择，用户无需在两处冲突的状态间判断。

## 图像与两轮复核

[总览](P13-ir/overview.png)、[深色局部](P13-ir/focus-detail-dark.png)、[浅色局部](P13-ir/focus-detail-light.png)、[像素差](P13-ir/image-differences.txt)。每项均有 `before/after/comparison` 深浅图，原帧 720×400；总览缩放仅用于比较，不冒充高 DPI/真桌面截图。

第一轮四视角：艺术视角看到当前值灰条与旧高亮蓝条争抢；工程视角发现按索引保持菜单导致错选；任务视角确认 Enter 会覆盖宿主新值；读者视角看到同名选项无法对应具体动作。第二轮用相同事件截图复核，保留既有布局，只纠正高亮归属、同名身份及输入出口。局部可见深色计算器蓝条和触发器相符；浅色驻留 120 秒与蓝条一致。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增；沿原近黑与浅色主题 |
| A2 字体 | 保留用户指定中文系统栈，优先于泛化“每轮换字体”要求 |
| A3 斜体 | 无新增 |
| A4 Hero | 功能设置页，不适用 |
| A5 卡片海/三等分 | 无新增容器 |
| A6 假数据 | 产品零假指标；测试使用真实动作 ID 和受控中文标签 |
| A7 空词/感叹号 | i18n 零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原有标题不变；此轮展示控件自绘帧 |
| A10 Bento | 不适用 |
| A11 装饰竖线 | 无新增 |
| B1 读取/拨盘 | 2/1/6，稳定、即时、原密度 |
| B2 主题锁定 | 每个 Tester 分别固定深/浅 |
| B3 记忆点 | 菜单高亮与实际当前值一致 |
| B4 气质 | 0ms 高频键盘反馈 |
| C1 目的/频率 | 高频选择不新增动画 |
| C2 all/ease-in/scale(0)/fade | 无新增动效 |
| C3 时长/按压 | 新反馈 0ms，原按钮按压保留 |
| C4 浮层 origin | 仍由 SelectBase 锚定原触发器 |
| C5 reduced-motion | 测试使用 ReduceMotion；无新增时间依赖 |
| C6 hover 媒体查询 | Go 自绘，无 CSS；原指针规则保留 |
| C7 布局动画 | 无 width/height/margin 等动画 |
| C8 快速触发 | 原框架事件处理，不增加 keyframes |
| C9 手势 | 未修改拖拽/弹簧/手势，不适用 |
| C10 Motion 性能 | 无 Web Motion 运行时，不适用 |
| C11 慢放/逐帧 | 无新动画，已按 Frame 检查刷新及 Enter 后结果 |
| D2-1 字体 | 中文系统字体；无 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px/1.5 未满足，既有 Caption 9.9/正文 10.8 与行盒继续待拍板 |
| D2-3 混排 | 文案不变，无新增混排 |
| D2-4 标点/对比 | 标点不变，不宣称全部次要墨过 7:1 |

缺陷和门禁沉淀于仓库合约 §6.219 与真实事件测试；未将项目选择发布成全局 skill 偏好。

## 仍待

1. 首选继续验证菜单展开时绑定整行移除/重排，以及宿主改为未知动作后的关闭与焦点出口；仅代码线索，没有本轮复现证据，不计已知 bug。
2. 参数类型标签本身随语言变化的菜单身份尚未事件验证；不把同名 schema/翻译假设算已修。
3. 字号/行盒、accent 禁用填充、未知多选/Radio、176-9/161-4 保存期草稿、冲突另一端消失后的历史提示、终端 S1b/S1c/S2 尾继续待拍板。
4. 八条旧 WIP 保留；未做原生输入法、Option+字母及 Windows/macOS 真机交互验证。

已停止新增选题和产品修改，完整门槛/报告/推送收尾继续执行。外层编排达到 40 次；wait 轮询另计的严格总工具口径已经超过 40，预算偏差如实记录，不以此虚报验收。动作/调色板的重排与同名测试使用控件真实接收的选项参数，未宣称出厂静态词表本身已经含有同名或重排数据。

## 完整门槛与交接

最终产品 **30c14be8**。普通/verbose 全量均退出 **0**，最终 **5025 PASS / 0 FAIL**（基线 5001，+24）；测试函数同口径 **3093 → 3101**（+8）。旧测试函数和断言零修改、新增 Skip 0，V/B/S **0/0/0**。gofmt 空，Linux/Windows/macOS vet/build 全部退出 0。四组 mutation 各 6 FAIL，均逐字节 MD5 恢复、touch 后各 6 PASS；最终四个源码 MD5 全部 OK。

证据：[完整普通测试](P13-ir/full-test.txt)、[全量计数](P13-ir/test-summary.txt)、[三平台门槛](P13-ir/gates.txt)、[文档 CI](P13-ir/docs-ci.txt)、[最终源码 MD5](P13-ir/final-source.md5)。完整 verbose 与开盘基线保留在 `/tmp/floter-P13-ir/`。原始 before 和四组红/绿日志已入仓。

05:55 UTC 开盘，06:49 UTC 完整测试结束，约 **54 分钟**。四批各两项均在相关测试和包 vet 通过后立即提交推送，完整门槛集中收尾。首轮未复现的替换、初值 ID 接线失误、漏 locale 和工具预算超限均已如实记账；旧 pin 没有为消红而调整。没有新增系统、依赖、页面或终端会话语义。
