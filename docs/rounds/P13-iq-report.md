# P13-iq · 快捷键控件变化后的焦点归还

2026-11-07 05:23 UTC 开盘，5013b357，pull 最新，工作树净。Codex 独立执行。基线 4961 PASS / 0 FAIL，实数 3083 测试函数；上轮报告的 3085 与本轮 git grep 计数不同，未据此编数字。done.log 开盘不存在。

## Stash 清点

八条 SHA 与上轮、backlog 相同，无新增 stale-kill。逐条 reverse-check 均退出 1；沿已有语义审查结论保留早期 WIP，不合回旧主题或已撤销 Tauri 链。

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

## 实施记录

候选尚未验证不计修复。取消录制的两类事件已绿，不算新修复。冲突另一端消失后清旧提示与旧 B 断言冲突，写入待拍板；未修改旧断言。旧 V/B/S 修改 0/0/0。

### 第一批

1. 现象：自定义快捷键录成新键后焦点消失，Enter 无法重新录制。改动：将焦点一次性交给接受后的 key 对应录制器，失效请求随目标消失清除。验证：深浅真实 Tab/Enter/Ctrl+Shift+R，写入一次；随后 Enter 重新录制仍为原第二行。取消录制的原路径继续通过。
2. 现象：录制 New command 时，唤出快捷键也显示录制态。改动：空 action 只代表唤出动作，统一后精确匹配录制 ID。验证：深浅真实 Tab/Enter，原先四条 prompt（两块控件各有 label/text）缩为两条（唯一录制器），Escape 可取消。初版测试误将 label/text 视为一条，夹具已纠正，原始误计记录不算产品缺陷。

相关 58 PASS / 0 FAIL，包 vet 净。M1 回注旧逻辑，6 FAIL：`custom-accepted lost recorder focus`、`Enter cannot restart the original recorder`、`one recording painted 4 prompt entries, want 2 for one pane`。MD5 两文件还原、touch 后 6 PASS。旧 V/B/S 0/0/0。

第一批 f06e320e 已推送。

### 第二批

3. 现象：保存行切换到“命令行”会变成草稿，但键盘停在退场选择器，直接打字无效。改动：转换后一次性聚焦新命令框。验证：真实选择菜单、直接输入 say converted，草稿获得完整文本，绑定仍未保存。最初夹具使用不在选项中的 action:settings，已经显示命令模式；改为真实 plugin:clipboard 后才取得产品红证据。
4. 现象：删除未完成草稿后没有焦点，Enter 无法继续添加。改动：删除后回到原 Add 入口。验证：两主题真实添加、录键、Tab 到删除、Enter，草稿退出且绑定数 0，Enter 再开录制。

相关 64 PASS / 0 FAIL，包 vet 净。M2 撤掉两条焦点请求，6 FAIL：`command conversion lost its input focus`、`converted command cannot type: ""`、`draft cancel lost the Add outlet`、`Enter cannot add again after cancel`。MD5 还原 touch 后 6 PASS。旧 V/B/S 0/0/0。

第二批 7c2fd45b 已推送。

### 第三批

5. 现象：草稿按 Bind 成功后控件退场，Enter 不能继续编辑新绑定。改动：绑定成功后一次性聚焦接受后的录制器，未完整的草稿不跳走。验证：两主题真实 Add/录键/Tab/Enter，只有 Ctrl+Shift+R → plugin:clipboard 一条，Enter 再录制目标为第 0 行。
6. 现象：删除已保存绑定后键盘没有出口。改动：用户删除回到 Add。验证：两主题真实 Tab/Enter 删除，列表为空，Enter 立即再次打开草稿录制。

相关 70 PASS / 0 FAIL，包 vet 净。M3 回注两处遗漏，6 FAIL：`bound draft lost the saved recorder focus`、`saved removal lost the Add outlet`、`Enter cannot continue after binding exit`。MD5 还原 touch 后 6 PASS。旧 V/B/S 0/0/0。

第三批 f01b9e29 已推送。

### 第四批

7. 现象：宿主把命令改为插件动作后，聚焦输入框消失，Enter 无法打开仍在的选择器。改动：仅原命令框仍持有键盘时，将其交回同一行选择器。验证：深浅真实输入后宿主刷新，Enter 打开菜单；先移到 Add 的对照不抢焦点，刷新写入 0 次。
8. 现象：宿主移除绑定后，聚焦命令框、录制器、选择器或删除钮均无出口。改动：按上一帧控件句柄识别真正失主的焦点，退役句柄后转回 Add。验证：两主题四种焦点来源及主动移开对照，Enter 可继续添加，刷新写入 0 次；录制状态仍沿原退役规则。

相关 86 PASS / 0 FAIL，包 vet 净。M4 撤销两类恢复，12 FAIL（10 失败场景与两个父测试），原串：`refresh-action from command lost its visible outlet "Action"`、`refresh-removed from recording lost its visible outlet "Add"`、`refreshed outlet cannot add`。MD5 还原 touch 后 16 PASS。实现时曾把无返回值的 sessionIconButton 当元素使用而编译失败，已改为可选句柄绑定并通过回归；没有带红提交。旧 V/B/S 0/0/0。

第四批 2dda0e33 已推送。四批每两项先过相关事件与包 vet，再 commit/push；完整门槛集中收尾，相关数字不冒充全量。

## 偷了什么

真实读取 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography 与 Apple DESIGN 的角色表。当前用户已指定唯一方向，直接打磨，不做四方向网页，不调用外部模型、不委派。

- craft-loop 工艺菜单 **#3 品牌 focus 环**：取 Apple 的单交互蓝语义和 **2px** 焦点环，沿本仓蓝色；焦点交接 **0ms**。新键、转换后的命令框、刷新后的选择器都有唯一可见键盘归处。
- **#6 编辑细节**：保留 **1px** 发丝、原行间距和控件排列；无效双录制态从 **2 块到 1 块**，额外提示占高 **0px**，添加入口继续使用原位置。
- **#12 材质细节**：保留录制器 **8px** 圆角、原受光边、E1 面；新增透明面和阴影 **0 层**，状态只由原焦点环表达，不再造反馈容器。

本轮优先修真实功能问题，不为工艺菜单凑五项添加装饰。为什么更轻、更有空间感：同一时刻只有真正被录制的行亮起；退场控件把键盘交给现有面中的可见入口，焦点层与内容层重新对应。

## 图像与两轮复核

32 张深浅 before/after 原帧、16 张对照板、总览和局部图进仓。Linux ui.Tester 自绘（720×440；内置双行例为 720×220），不冒充桌面系统截图。局部最近邻放大只检查像素，不冒充高 DPI 真机。

第一轮四视角：艺术视角看到两条同时亮起的录制态争抢注意力；工程视角看到 key 改变使控件失主；任务视角发现 Bind/Delete 后 Enter 无法继续；读者视角无法看出刷新后键盘在哪里。第二轮逐项深浅对照，集中复核唯一录制态、原行焦点、新命令入口与 Add 出口。

目视深色内置对照：上方 Show / hide floter 退回静息，仅 New command 保留录制环。目视浅色刷新对照：Clipboard history 选择器获得清晰蓝环，行框、字和间距没有移动。最小差为 **240 像素**（Add 焦点）；全部八项深浅差非零。旧小字号与部分次要墨仍是拍板项，不冒充解决。

| 项 | 浅色对照 | 深色对照 |
| --- | --- | --- |
| 1 新键录制 | [图](P13-iq/custom-accepted-light-comparison.png) | [图](P13-iq/custom-accepted-dark-comparison.png) |
| 2 内置录制归属 | [图](P13-iq/builtin-owner-light-comparison.png) | [图](P13-iq/builtin-owner-dark-comparison.png) |
| 3 命令改型 | [图](P13-iq/draft-convert-light-comparison.png) | [图](P13-iq/draft-convert-dark-comparison.png) |
| 4 草稿取消 | [图](P13-iq/draft-cancel-light-comparison.png) | [图](P13-iq/draft-cancel-dark-comparison.png) |
| 5 草稿绑定 | [图](P13-iq/saved-bind-light-comparison.png) | [图](P13-iq/saved-bind-dark-comparison.png) |
| 6 保存行删除 | [图](P13-iq/saved-remove-light-comparison.png) | [图](P13-iq/saved-remove-dark-comparison.png) |
| 7 宿主改型 | [图](P13-iq/refresh-action-light-comparison.png) | [图](P13-iq/refresh-action-dark-comparison.png) |
| 8 宿主移除 | [图](P13-iq/refresh-removed-light-comparison.png) | [图](P13-iq/refresh-removed-dark-comparison.png) |

[总览](P13-iq/overview.png)、[像素差](P13-iq/image-differences.txt)、[浅色局部](P13-iq/focus-detail-light.png)、[深色局部](P13-iq/focus-detail-dark.png)。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿原近黑/浅色主题 |
| A2 字体 | 用户既定中文系统栈优先于每轮换字体的泛化规则 |
| A3 斜体 | 无新增 |
| A4 Hero | 原生功能设置页，不适用 |
| A5 卡片海/三等分 | 无新增容器 |
| A6 假数据 | 无产品假指标；测试输入为可执行命令/实际动作 ID |
| A7 空词/感叹号 | i18n 零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原有分组标题完整可见 |
| A10 Bento | 不适用 |
| A11 装饰竖线 | 无新增 |
| B1 读取/拨盘 | 2/1/6：稳定、即时、原密度 |
| B2 主题锁定 | 每个 Tester 单独指定深/浅 |
| B3 记忆点 | 录制和焦点始终指向正在操作的目标 |
| B4 气质 | 0ms 键盘恢复，与轻便高频操作一致 |
| C1 目的/频率 | 高频键盘不增加动画 |
| C2 all/ease-in/scale(0)/纯 fade | 无新增 |
| C3 时长/按压 | 既有 120ms 颜色过渡和删除钮 0.92 按压不变 |
| C4 origin | 选择器仍从原入口展开，未改浮层 origin |
| C5 reduced-motion | 全部新事件使用 ReduceMotion=true |
| C6 hover 媒体查询 | Go 自绘不适用 CSS；原指针路径不变 |
| C7 布局动画 | 无新增，删除和改型沿既有布局 |
| C8 keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | Go 自绘，不适用 |
| C11 慢放 | 无新增动效，未声称真机慢放验证 |
| D2-1 系统字体 | 沿原系统字体，无 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px/1.5 未满足；旧 Caption 9.9、正文 10.8/行盒继续待拍板 |
| D2-3 混排空格 | i18n 零改，无新增混排文案 |
| D2-4 标点/对比 | 原墨与标点不变，不声称全部次要墨过 7:1 |

缺陷和可复现门禁沉淀在仓库合约 §6.218 与事件测试；未将本轮项目选择扩成全局 skill 偏好。

## 仍待

1. 下一轮先查快捷键选择器菜单已经展开时，宿主改型/移除后的浮层和焦点归属；本轮覆盖的是关闭菜单的选择器、命令框、录制器和删除钮。展开菜单是下一条未复现线索，不算已知 bug。
2. 需拍板：冲突另一端移除/改键后是否自动清除拒绝历史。旧 TestCustomConflictFollowsRemainingBinding 明确要求移除对方后仍显示拒绝；本轮不改 B 断言。宿主回写命令字符串后草稿反馈尚无新复现。
3. Linux 为 ui.Tester 事件与自绘，Windows/macOS 仅 vet/build；原生输入法、Option+字母和真实桌面仍缺真机证据。
4. 字号/行盒、accent 禁用填充、未知多选、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾继续沿用待拍板。八条早期 WIP 保留。

## 完整门槛与交接

最终产品 **2dda0e33**。完整普通与 verbose 测试均退出 **0**，最终 **5001 PASS / 0 FAIL**（基线 4961，+40）。同一 git grep 口径测试函数 **3083 → 3093**，新增 10 个函数包含 8 项修复与两个已正常的取消录制对照；旧测试函数与断言零修改。gofmt 空，linux/windows/darwin vet/build 全部退出 0；新 Skip 0。文档更新后独立 CI 通过。

四组 mutation 均见红、MD5 还原、touch 后复验：6/6/6/16 PASS。所有证据只描述实跑的 Linux 自绘事件；其他平台只作编译门槛，未声称真机交互验收。

证据：[完整普通测试](P13-iq/full-test.txt)、[全量计数](P13-iq/test-summary.txt)、[三平台门槛](P13-iq/gates.txt)、[文档 CI](P13-iq/docs-ci.txt)、[最终源码 MD5](P13-iq/final-source.md5)。四组原始红/绿日志在本目录；完整 verbose 与开盘基线保留于 `/tmp/floter-P13-iq/`。

05:23 UTC 开盘，05:50 UTC 完整门槛结束，约 28 分钟，低于 45–75 分钟目标；八项已完成，不追加无关装饰凑时长。达到 40 次外层工具调用预算后停止新增选题与产品修改，仅等待已启动的完整门槛并完成这份验收归档。实际首轮探针的计数/夹具错误和一次编译错误均已在逐批记录中说明，没有以失败夹具凑修复数。
