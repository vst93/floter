# P13-hr：配置刷新后的非文本控件

起点 d3b1961b，工作树干净，pull --ff-only 已最新。禁用墨已在 P13-hl/hm 结案，本轮沿最新待办检查选择控件；不是重复执行过期任务书行号。

## 抢救 stash 清点

当前八条均非 stale-kill，SHA 与 P13-hq 表一致，reverse-check 均退出 1；此前 stale-kill 原件已归档并接续，不重复合入。保留原件，不以 reverse-check 失败推断可覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 早期 keycap WIP，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 早期 rows/slider WIP，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载布局与旧探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧 number-field WIP，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段 WIP，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 WIP，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏 WIP，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri 实现，reverse=1 | 保留 |

## 实施记录

规格八项均先跑真实事件；门槛、变异、图与每批 SHA 随完成增量记录。V/B/S 暂为 0/0/0，旧断言不改。

### 第一批：下拉与开关

1. 现象：下拉跨组后菜单消失，待选 apps 丢失，Enter 后仍是 files。改动：choose 返回原 trigger，插件将其绑定到已有字段焦点句柄。验证：真实 Click→Down→移组→Enter，深浅均选 apps、保留焦点；原实现报 `group move closed the active select menu` / `group move lost pending choice: files, want apps`。图上可见移组后的“应用”仍高亮，菜单位于新触发器下方。
2. 现象：开关跨组后焦点消失，Space 不再切换。改动：checkbox 返回原开关，绑定同一分组焦点路径。验证：点击→移组→Space 后 true 回 false，深浅均保焦点；原实现报 `group move lost toggle keyboard: value=true focus=false`。

相关回归 **146 PASS / 0 FAIL**，settingsui vet 净。M1 回退 pluginsheet.go 两条绑定，新增两函数六个 PASS 项变红，恢复后 MD5 OK、touch 后 **6 PASS**。日志和深浅图在 P13-hr/。旧 V/B/S 断言修改数 0/0/0。

第一批 `a1e2fe9c` 已推送。开盘完整 verbose 实测 **3947 PASS / 0 FAIL / 2850 测试函数**。

### 第二批：单选与复选

3. 现象：单选跨组后 Right 不再选下一项，files 原地不动。改动：原分段轨道绑定字段句柄，分组匹配改用 FocusWithin 识别内部选项。验证：Click 文件→移组→Right 得 apps，深浅均通过；旧实现报 `group move lost radio keyboard: files, want apps`。
4. 现象：复选跨组后 Space 不再取消刚勾选项，焦点丢失。改动：原复选行绑定同一字段句柄，用子树焦点保住组身份。验证：Click 文件→移组→Space 得空列表且焦点仍在文件；旧实现报 `group move lost checkbox keyboard: value=[files] focus=false`。

相关回归 **171 PASS / 0 FAIL**，settingsui/control vet 净。M2 将 FocusWithin 退为 Focused，两项事件实跑红；MD5 还原与 touch 后 **6 PASS**。没有新布局包装，也没有修改旧断言；V/B/S 0/0/0。

第二批 `0805b4b9` 已推送。

### 第三批：滑块与动作取消

5. 现象：滑块跨组后 End 不再响应，数值停在 0。改动：原滑块绑定字段焦点句柄。验证：点击轨道→Home=0→移组→End=100，深浅通过；旧实现报 `group move lost slider keyboard: 0, want 100`。
6. 现象：焦点位于已武装动作的取消按钮时，移组后 Space 无效。改动：原确认按钮行绑定字段句柄，识别其内部焦点。验证：Tab 到取消→移组→Space，仅 Unarm 1 次、Run 0 次；旧实现报 `group move lost cancellation: cancels=0 runs=0 armed="value"`。确认/取消机制未变。

相关回归 **206 PASS / 0 FAIL**，settingsui/control vet 净。M3 回退两条绑定，两项事件实跑红，MD5 还原与 touch 后 **6 PASS**。V/B/S 0/0/0。

第三批 `1bbdc9ed` 已推送。

### 第四批：已收起下拉的外部刷新

7. 现象：下拉用 Escape 收起后，外部把当前值换成未知值，字段焦点丢失，方向键无法重新选择。改动：记录菜单是否展开和触发器句柄，仅对仍可用、原先已收起且拥有焦点的控件交接焦点。验证：外部值先原样显示、配置不脏、菜单不自动打开；Down→End→Enter 可改为便条。旧实现报 `closed select refresh lost keyboard focus` / `refreshed select cannot recover with keyboard: 远程来源, want 便条`。
8. 现象：下拉已收起，schema 删除当前选项，也会使整个字段退出键盘操作。改动：同一交接路径覆盖选项撤回。验证：未显式选择前旧值保留、未写回，随后键盘可选便条；旧实现报 `refreshed select cannot recover with keyboard: 文件, want 便条`。已展开菜单、禁用控件、其他字段焦点继续由原回归守住。

相关回归 **212 PASS / 0 FAIL**，settingsui/control vet 净。M4 回退 selectvalue.go，两项实跑红，MD5 还原与 touch 后 **6 PASS**。V/B/S 0/0/0；测试迁到 selectrefreshfocus_test.go 后产品前红证据中的旧文件行号仍保留。

第四批 `a8571d00` 已推送。八项产品均已分批保住；完整门槛在固定源码上运行，期间不再注入变异。

## 偷了什么

已读 qiaomu-design 的 SKILL、偏好账本、craft-loop、preflight、chinese-typography，参考 Apple DESIGN 的单一交互蓝、8px 控件圆角、0.48 弱化墨，映射到现有 accent、RadiusSM=8、框架 Disabled=0.5，没有复制网站大字或纯黑背景。

- 从 craft-loop 工艺菜单第 3 项取「焦点可见且属于当前控件」：六类控件跨组后仍只有 1 个键盘目标；复选与取消按钮截图恢复原蓝色焦点环，框架原值为 2px 宽、外扩 3px，不增加焦点动画。
- 从第 4 项取「签名交互必须服务任务」：schema 刷新后键盘继续操作，额外等待 0ms，动作取消只调用 1 次 Unarm、0 次 Run；下拉显式确认前写回 0 次。
- 从第 12 项取「材质细节」：保留既有槽面、8px 控件半径与菜单层，新增材质层 0、额外布局包装 0。变化是焦点与面不再凭空消失，不把增加光泽当打磨。

没有为凑工艺密度五项增加装饰，依据用户本轮禁扩张约束；P-2/P-37 四方向流程不适用，用户已指定方向与直接打磨路径。设计记忆落到本仓合约 §6.193 与事件 pin，不改全局 skill 偏好。

## 两轮看图与 preflight

第一轮原实现截图先定位消失的菜单与焦点；第二轮修复后深浅同事件复拍，40 张原始 PNG、20 张左右对照（左旧右新）。截图是 ui.Tester 的原生自绘结果，未使用 HTML 或假玻璃图。不是三平台真机截图：本机 Linux 软件渲染，其他平台验 vet/build。

四视角：艺术总监看菜单在移组后仍锚定触发器；工程师看取消按钮焦点是否被丢；任务使用者看 End/Space 是否继续生效；读者看已收起下拉刷新后的可见值是否保留。前六项修掉凭空退场，后两项保留静态原值并恢复后续键盘操作。对照图可见开关关掉、单选移到应用、复选清勾、滑块到 100、取消按钮蓝环恢复；外部未知值的静息图无像素变化，recovered 图明确由远程来源/文件变为便条，保存状态随显式编辑变为未保存。

补看 4× 局部：取消按钮深浅两套原焦点环完整，未切进卡片底边；下拉的“应用”文字在高亮中完整，三个选项之间的空间保持原样。[取消·浅](P13-hr/action-light-detail.png)、[取消·深](P13-hr/action-dark-detail.png)、[下拉](P13-hr/select-light-detail.png) 是局部检视证据。

| 检查 | 结论 |
| --- | --- |
| A1 紫蓝渐变/纯黑/霓虹/渐变字 | 无新增；既有近黑 E0 沿用 |
| A2 换字体 | 不适用；中文系统字体由用户现有合约与 D2 优先，不为本轮换字体 |
| A3 斜体 | 无 |
| A4 居中 Hero | 不适用，现有原生配置页 |
| A5 卡片海/三等分 | 无新增卡片，沿用 schema 的真实分组 |
| A6 假数据 | 使用真实形状的字段/选项与可执行事件，未向产品加入假指标 |
| A7 空洞文案/感叹号 | 产品文案零改 |
| A8 编号眉批 | 无新增 |
| A9 H1 行数 | 原页头短标题，未改 |
| A10 Bento 空洞 | 不适用，原生表单 |
| A11 左侧竖线强调 | 无 |
| B1 读取与拨盘 | 克制、即时、原密度，与规格一致 |
| B2 单页主题 | 深浅分别渲染，不混主题 |
| B3 记忆点 | 操作位置变化后，当前焦点和选择仍连续 |
| B4 动效气质 | 高频键盘更新即时，未加入动画 |
| C1 目的/频率 | 修复高频键盘连续性，附加动画 0 |
| C2 all/ease-in/scale(0)/纯 fade | 无新增 |
| C3 时长/按压 | 沿用原控件，新增过渡 0ms |
| C4 浮层来源 | 下拉始终锚定当前触发器；深浅截图已核 |
| C5 reduced-motion | 新场景全部以 ReduceMotion=true 运行 |
| C6 hover 媒体条件 | CSS 不适用；原生指针机制未动 |
| C7 布局动画 | 无新增 |
| C8 快速触发 | 原控件机制保留，没有新增 keyframes |
| C9 手势/拖拽 | 未新增手势；滑块只修焦点，不改拖动算法 |
| C10 Framer/Motion | 不适用，Go/mygo |
| C11 不确定动画 | 无动画改动，不宣称完成真机慢放验收 |
| D2.1 系统中文字体 | 沿用本机字体，所有截图与测试使用 zh_CN.UTF-8 |
| D2.2 ≥14px/1.5/400-600 | 未通过，既有 Body/Caption 低于 14px、部分字重 580/620/650；仍属字号/行盒拍板项，本轮不越权改 |
| D2.3 混排空格 | 无新增产品文案；本轮测试标签中文，数字读数沿用原格式 |
| D2.4 标点/7:1 | 无新增标点；默认弱墨不承诺 7:1，沿用对比度合约与拍板项，不谎报通过 |

## 仍待

1. 本轮六种非文本控件跨组缺口全部接上，上一轮两种编辑器回归仍通过。下一轮首选插件动作的「武装→字段被移除/改型→重加」生命周期，先以真实事件确认是否残留武装，未复现不算 bug。
2. 打开中的下拉因未知值失效退出沿用既有契约；本轮只修菜单已经收起的焦点遗失，不改 Enter 的原行为。
3. 字号/行盒、禁用 accent、未知 Radio 模型、未列举多选值、176-9/161-4 保存期间草稿归属、终端 S1b/S1c/S2 尾继续待用户拍板。
4. 八条早期 WIP 保留，未产生新抢救稿；stash 表见开头。

## 门槛与证据

基线 **3947 PASS / 0 FAIL / 2850 Test 函数**；最终 **3971 PASS / 0 FAIL / 2858 Test 函数**，增加 24 PASS 与 8 函数。旧测试文件零修改，无新增 Skip。gofmt 输出 **0 字节**；linux/windows/darwin 的 vet 与 build 均退出 0；文档合约新增 §6.193 后 internal/ci 单独通过。固定源码的普通与 verbose 全量均退出 0，最终 MD5 复核逐项 OK。

门槛结果：[完整普通测试](P13-hr/full-test.txt)、[结果汇总](P13-hr/gates.txt)、[三平台](P13-hr/platform-gates.txt)、[源码校验](P13-hr/final-source.md5)。完整 verbose 日志保留 `/tmp/floter-P13-hr/verbose-final.txt`。

四次正式 mutation 的原文见 [M1](P13-hr/batch1-mutation.txt)、[M2](P13-hr/batch2-mutation.txt)、[M3](P13-hr/batch3-mutation.txt)、[M4](P13-hr/batch4-mutation.txt)，对应 MD5 清单与 touch 后复跑日志均在同目录。最终源码 MD5 固定后才启动完整门槛，没有测试与 mutation 重叠。

前后对照：[下拉·浅](P13-hr/select-move-light-comparison.png) / [下拉·深](P13-hr/select-move-dark-comparison.png)，[单选·浅](P13-hr/radio-move-light-comparison.png) / [单选·深](P13-hr/radio-move-dark-comparison.png)，[滑块·浅](P13-hr/slider-move-light-comparison.png) / [滑块·深](P13-hr/slider-move-dark-comparison.png)，[外部值恢复·浅](P13-hr/select-external-recovered-light-comparison.png) / [外部值恢复·深](P13-hr/select-external-recovered-dark-comparison.png)。其余 toggle/checkboxes/action/select-removed 按同规则命名。
