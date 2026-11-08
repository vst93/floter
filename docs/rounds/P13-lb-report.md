# P13-lb · 前置工具刷新后的命令与行底出口

开盘e23ab358，工作树空，pull已最新；最新队列来自P13-la而非任务书过期的P13-jw。安装恢复P13-jx不重复。完整门槛基线为上轮原始日志7170 PASS / 3658函数。

统一入口先增加每门槛最多两次和逐次日志归档。方向：19px净空保证焦点环与淡边分离，新增材质0、布局增量0，保持轻量空间感。

## Stash交接

|ref|SHA|判断/处置|
|---|---|---|
|stash@{0}|422483b4|reverse=1；早期keycap独立WIP，保留，未接入|
|stash@{1}|8af94a07|reverse=1；旧设置尺寸/hover独立WIP，保留，未接入|
|stash@{2}|7e4aa710|reverse=1；旧卸载尺寸与scratch，保留，未接入|
|stash@{3}|1e3aec94|reverse=1；旧number-field未完成稿，保留，未接入|
|stash@{4}|363f5a42|reverse=1；旧配置字段稿，保留，未接入|
|stash@{5}|9b8ce23c|reverse=1；旧快捷键/Tauri稿，保留，未接入|
|stash@{6}|90d314b2|reverse=1；旧启动器行为稿，保留，未接入|
|stash@{7}|7a9f6e13|reverse=1；旧main/Tauri稿，保留，未接入|

逐条原始reverse日志在本轮logs；八条SHA与交接账一致。reverse失败不能证明独有逻辑仍适用；不据此覆盖当前行为。历史移植未完成，保留为遗留限制。

## 逐项验收（增量）

1. 命令别名：前置区块刷新后仍持焦却离屏 → 命令列表纳入integrationLead → 三来源×深浅插入/增长/撤回/恢复均可见，stored-next仅在181ms后写原forecast命令一次。
2. 命令开关：刷新后开关离屏 → 同一反馈接线 → 三来源×深浅均保持19px净空，Space仅关闭原forecast命令一次。

第一批修前20 FAIL，修后20 PASS / exit0；旧V/B/S断言修改0/0/0，新增Skip0。其余六项修前分别20 FAIL/批，均为活动出口离屏；完整门槛与独立mutation待后续完成，尚不声明全量通过。

3. 运行：前置内容增长使原Run入口离屏 → 加入前置反馈 → 三来源×深浅四次刷新均可见，Enter仅运行原weather且参数为空。
4. 诊断：前置内容增撤使诊断按钮离屏 → 加入前置反馈 → 同组刷新均可见，Enter只诊断原weather一次。

第二批修前20 FAIL，修后20 PASS / exit0。深浅各一组前后图已目视：修前别名槽在下沿之外；修后stored-next和整个焦点环出现在淡边上方。两主题均无额外面层。

5. 移除：前置刷新挤走移除入口 → 加入前置反馈 → 三来源×深浅四步可见，Enter仍只开启原工具确认，不提前卸载。
6. 启停：前置刷新挤走工具开关 → 加入前置反馈 → 同组四步可见，Space只启用原weather一次。

第三批修前20 FAIL，修后20 PASS / exit0；旧V/B/S断言修改0/0/0，新Skip0。

7. 重新发现命令：前置刷新后动作无显现包络、离屏 → 接入既有19px包络和前置/输出反馈 → 三来源×深浅四步可见，Enter只重新发现原weather一次。
8. 编辑工具：前置刷新后编辑入口离屏 → 接入相同包络和反馈 → 同组四步可见，Enter读取原weather并打开其编辑器，不提前保存。

第四批修前20 FAIL，修后20 PASS / exit0。八项合计80个新增PASS，8个测试函数；原测试断言未改。全部用例同时核验等值数据和隐藏推荐不会抢回手动滚动。

四批产品提交：0f202b1c / 89003a02 / a7239198 / e641bed7，均已推送mygo-rewrite。

## 变异实测

|变异|注入|实红|还原核验|touch后复验|
|---|---|---|---|---|
|M1|从commandList撤掉integrationLead|20 FAIL / 0 PASS|MD5 OK|20 PASS / exit0|
|M2|从row-run-output-outlet撤掉integrationLead|10 FAIL / 0 PASS|MD5 OK|10 PASS / exit0|

M1原始报错：`feedback reflow hides active outlet "Alias for 天气预报": target={X:208.8 Y:585.7999 W:0 H:0} viewport={X:170.6 Y:51.399998 W:549.4 H:468.6} focused=true`。

M2原始报错：`feedback reflow hides active outlet "Run": target={X:662.8 Y:587.30005 W:0 H:0} viewport={X:170.6 Y:51.399998 W:549.4 H:468.6} focused=true`。

原件逐字节MD5核验见logs/mutation1-restore.txt及mutation2-restore.txt；每次恢复后touch再运行-count=1，未复用测试结果缓存。两次变异均在完整门槛之前完成。

## 偷了什么（具体数值）

- craft-loop菜单#3「品牌focus环」：沿用3px焦点外扩，显现包络将它全部带回可视区。
- 菜单#4「签名交互」：反馈变化只触发一次0ms显现；后续手动滚动保持自由，未加动画。
- 菜单#9「自定义滚动条」：沿用16px淡边；19=16+3px净空，不改滚动条和行高。
- Apple供体只取交互色专属、留白服务内容、单一焦点三个原则，落成0新增材质、0布局增量、单一原控件焦点；未移植其字号与纯黑。
- 未凑足5项装饰：当前用户要求现有功能边界修复，craft-loop自身允许“两轮制 > DNA > 凑满5项”。

## 目视复核与preflight

深/浅各1张修前与1张修后原图，共4张；2张并排对照没有再次截图。[深色](P13-lb/images/dark-comparison.png)、[浅色](P13-lb/images/light-comparison.png)。两套图均看到：修前Alias槽在下沿之外；修后stored-next完整显示，焦点环位于淡边上方。

四视角：艺术视角关注卡片层级未被新增包络画出额外框；工程视角抓住“持焦却宽高为0”的裁切；任务视角要求继续编辑原命令；阅读视角关注输入文字与浅色焦点环完整可见。修后逐项已复核。小字号可读性仍是既有待拍板项，未借本轮擅改。

**A逐条**：A1无新增紫蓝渐变/纯黑/霓虹/渐变字；A2字体沿用项目合约，通用“每轮换字体”不适用；A3无斜体；A4没有Hero；A5无新增卡片或三等分；A6没有新增生产假数据；A7文案不变，无营销空词；A8无新增眉批；A9无新增标题；A10不是Bento；A11无新增左侧选中色条。A2两轮证据为修前复现图、评审和修后复核；A0证据归档项目合约/本报告，未把一次修复发布为全局skill偏好。

**B逐条**：B1符合既定轻量方向；B2深浅分别独立主题；B3记忆点是“正在编辑/操作的原控件继续可见”；B4一次0ms显现符合克制反馈。

**C逐条**：C1高频键盘反馈0ms；C2无CSS transition/all、ease-in、scale(0)；C3新增动画0ms，原按压反馈不改；C4无新浮层；C5用ReduceMotion=true测试；C6无新增hover动画；C7无布局动画；C8不重播keyframes；C9无手势；C10无Framer；C11没有待判定新增动效，不另拍逐帧图。

**D2逐条**：D2-1沿用系统中文字体与zh_CN.UTF-8；D2-2项目正文10.8/Caption9.9及行盒不满足通用14px/1.5倍规则，属既有待拍板；D2-3生产文案未改；D2-4夹具中文采用全角标点，生产墨色不改，7:1本轮未重测，不宣称新增合规。

## 仍待

## 全量发现的B回归及修复

首次普通全量发现原有TestCommandReprobeRejectsRepeatedClick与TestCommandReprobeLocksCompetingActions深浅均失败：`completed command reprobe cannot be retried: calls=0`。新增包络在鼠标按下获得焦点后滚动，移动了松开时的命中位置。

修复：只对本轮新加的重新发现/编辑包络限定“键盘入焦时自动显现”；前置内容和窗口变化的显现保持。复用outputFocusOutlet已有reveal回调，通过两层现有包装透传；其他调用行为不变。原B测试零改，统一入口加入可选包路径后聚焦shell得到6 PASS；新增重新发现/编辑键盘用例20 PASS。此次产品补丁需完整门槛唯一一次复验，首轮失败保留。

## 仍待（结案队列）

1. 下一轮先事件复现已有修复入口（重新检查/重连/主页）在前置区块刷新后的可见性，以及本地表单真实展开/折叠后活动控件是否仍可操作；未复现不计bug。本轮不重复P13-jx安装恢复与P13-la运行/配置表单八项。
2. 字号/行盒、accent禁用、未知多选、保存期间草稿、终端S1b/S1c/S2尾等原拍板项保持；3秒窗口、保存和会话语义不改。
3. Linux ui.Tester事件和图片已验；其他平台只做交叉vet/build，不声明原生窗口验收。八条历史独立WIP保留，未完成移植，不冒称stash清空。
