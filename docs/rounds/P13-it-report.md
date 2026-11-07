# P13-it：设置表单的草稿、刷新与失败恢复

07:39 UTC 开盘，起点 `1be93d7a`；pull 最新，工作区干净。实际从 P13-is 接续；P13-hx 已完成旧任务书的确认条件刷新，不重复计数。Codex 独立执行，已读取指定设计 skill 与参考。

## Stash 清点

八条 SHA 与 P13-is 一致；逐条导出 patch、reverse-check 均退出 1。早期独立 WIP 的语义处置已由 backlog 与前轮审计确认，沿用保留，未重合旧主题/Tauri 链；没有新的 stale-kill。开盘 done.log 不存在，收尾按指定路径创建。

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

基线全量实测 **5050 PASS / 0 FAIL / 3110 函数**，退出 0。

## 第一批

1. 现象：首次打开已保存的 45 秒自定义值，字段为空，Enter 写成 0。改动：初始化展开状态时同时给草稿种子。验证：深浅真实 Tab/复制读值/Enter，要求读回 45 且提交仍为 45；旧实现两主题均红。
2. 现象：自定义草稿 47 秒、菜单高亮 Off 时宿主改为 53 秒，旧 Enter 写成 0 且抹掉草稿。改动：菜单观察原始秒数，原值变更即退役旧菜单。验证：两主题旧菜单撤走、宿主 53 保留、草稿 47 保留、选择器焦点保留；旧实现两主题均红。

相关 **16 PASS / 0 FAIL**，包 vet 净。M1 回注旧文件 **6 FAIL**，原串 `stored custom duration opened as "", want 45`、`stale draft menu overwrote host duration with 0`；MD5 OK，touch 后 **6 PASS**。旧 V/B/S 0/0/0。新菜单测试最初用 HasText("关闭") 匹配到了说明文字，已改为只存在于菜单的 20 秒选项；数值、草稿、焦点断言全部保留。目视浅色：旧浮层消失，左侧自定义（53 秒）、右侧草稿 47，2px 蓝环仍在原入口。

第一批 `6d0d46fa` 已推送。

## 第二批

3. 现象：45 秒原值下输入 30，切换 Off 写文件失败后，输入仍退场，草稿变回 45。改动：存储接受预设后才收起输入。验证：真实下拉键盘选择 + 本地文件 rename 失败，原值 45、原草稿 30、重试入口保留；恢复文件后 Enter 恰再写一次成功。
4. 现象：直接 Enter 提交 30 秒失败也提前关闭并覆盖输入。改动：存储接受后再规范化草稿及决定展开状态。验证：两主题真实输入、失败、恢复后重试，写次数 1 → 2，未提前改写原值。

相关 **30 PASS / 0 FAIL**，包 vet 净。M2 回注旧文件 **6 FAIL**，原串 `preset-save-failure discarded retry draft: open=false draft="45"`、`custom-save-failure discarded retry draft: open=false draft="45"`；MD5 OK，touch 后 **6 PASS**。旧 V/B/S 0/0/0。参数 Required 与即时校验探针均为绿，不计修复、不改实现。

第二批 `06d75e0d` 已推送。

## 第三批

5. 现象：输入 4294967296 秒、-5、45.6、1e2 均错误回落到 10 秒。改动：依据参考 `surface-residency.ts` 的既有数值规范，有限数值先限幅再舍入；保留 never 哨兵与 NaN/Inf 默认值。验证：深浅真实 Type/Enter，分别为 86400/0/46/100；哨兵、非有限数保护均绿。此族算一项。
6. 现象：输入闭合时宿主更新为 53 秒，自定义菜单可见却无法选择同值打开编辑器。改动：闭合态收到 custom 时种入原值并打开输入，不触碰活动草稿。验证：两主题真实 Down/End/Enter，53 可读可编辑且未产生写入。

相关 **46 PASS / 0 FAIL**，包 vet 净。旧 V/B/S 0/0/0。上述全部数值先在旧实现上见红；沿用前两组正式 mutation，不虚称本批额外变异。最初尝试的历史路径 `src/components/settings/GeneralPage.tsx` 不存在，随后通过该分支树清单定位 `src/settings/GeneralPage.tsx`；未做全盘搜索。

第三批 `fd06c6bd` 已推送。

## 第四批

7. 现象：新快捷键草稿绑定时磁盘写入失败，整行消失、键位及命令清空，未提示失败。改动：检查本次 Store 写入结果，失败时保留草稿并沿用 save alert；系统注册拒绝语义不变。验证：真实 Add/录制/输入/Bind，原实现深浅均红，修后恢复文件能直接重试原命令。
8. 现象：已存命令编辑 Enter 写失败，原编辑缓存被清，下一次 Enter 只能存回旧值。改动：接受写入后再清缓存。验证：深浅真实命令编辑、文件故障、Enter、恢复后再次 Enter，原文本与焦点保留，最终提交 unfinished。

相关 **94 PASS / 0 FAIL**，包 vet 净；旧 V/B/S 0/0/0。原始红串 `failed add discarded draft: open=false key="" command=""`、`failed command discarded command draft: map[]`；本批先见红再修，正式 mutation 仍为 M1/M2 两组。

第四批 `ceff09ac` 已推送。

## 第五批

9. 现象：把已存动作改为命令行时写文件失败，原绑定还在，却又出现同键的新草稿。改动：接受移除后才初始化命令草稿。验证：两主题真实菜单 End/Enter + 磁盘失败，原绑定/选择器仍在、重复草稿不出现。
10. 现象：删除有未提交命令的绑定时写失败，行仍在却丢掉编辑内容，焦点误回 Add。改动：接受删除后才清编辑/重排录制归属；确认原键已消失才交 Add。验证：真实 Tab/Enter，原 unfinished 文本与删除入口焦点保留。

旧实现原串 `failed command-mode switch duplicated saved binding as draft`、`failed remove discarded command draft: map[]`、`failed remove lost original keyboard outlet`。相关实测计数见 batch5-test；包 vet 结果见 batch5-vet。旧 V/B/S 0/0/0。

第五批 `8f3d8610` 已推送，相关 **52 PASS / 0 FAIL**，包 vet 净。

## 第六批

11. 现象：已有命令草稿的绑定重新录制键位，磁盘写入失败后草稿被清。改动：先复制绑定列表，写回接受后再迁移编辑状态；失败回原录制入口。验证：两主题真实 Tab/录制新键，原命令草稿和已存键位不变，恢复文件可重试原动作。原始红串 `failed record discarded command draft: map[]`。

跨驻留输入/菜单、自定义快捷键新建/命令/录制/删除组件共 11 项；没有把参数两条原本为绿的探针凑进数量。第六批补强五个磁盘故障场景的原绑定零改与恢复后重试，完整数字按落盘结果收尾。

第六批 `b3d08780` 已推送，相关 **53 PASS / 0 FAIL**，包 vet 净。六批各完成 1–2 项即提交推送；最终完整门槛集中执行。没有引入系统、页面或依赖，没有改变系统注册拒绝的历史处置。

## 偷了什么

读取 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography 和 Apple DESIGN 的颜色、按钮部分；用户已定方向，所以采用既有方向打磨流程，不出四方案、不调用其他执行器。

- craft-loop **#3 品牌 focus 环**：取 Apple 单蓝交互角色、**2px** focus，沿用本仓蓝色 token；失败恢复交接 **0ms**，删除失败前后 **800 像素**变化中可见环从 Add 回到原删除键。
- **#4 签名交互**：输入以明确提交为节点，刷新 **0 次**派发；失败一次与恢复重试各 **1 次**写入，事件中实际累计 **1 → 2**。沿用原 Down/Enter/Tab 模型，不加延迟。
- **#6 编辑细节**：输入框如实显示 45/53/86400 秒；原 **1px** 发丝与行密度保留，额外常驻说明 **0 条 / 0px**；失败不抹去 unfinished 文本。
- **#12 材质细节**：取 Apple **8px** 控件角角色并映射既有 token；沿 E1/E2 面差，新增透明面 **0 层**、新增阴影 **0 层**。撤回旧菜单和重复草稿，而非增加容器。

用户要求修既有问题优先，不为菜单凑五项添加装饰。为什么更轻、更有空间感：失效浮层与重复行不再争夺位置；仍有意义的输入面与蓝色焦点稳定保留。

## 两轮图像复核

第一轮真实事件截图：艺术视角发现旧菜单盖着新时长、改型失败多出一整行；工程视角发现存储拒绝但本地状态已清；任务用户遇到 Enter 把 45 秒变为关闭、重试内容消失；读者看到“自定义（53 秒）”却没有输入入口。

第二轮保留原布局，集中复核种子数字、旧菜单退场、失败编辑保留、重复行消失与焦点出口。已目视深浅两套总览及原浅色菜单图：45 不再为空，旧 Off 浮层退出，30 秒失败草稿仍可见，删除失败的蓝环留在原删除键，改型失败只剩原绑定行。字号仍属既有拍板项，没有宣称解决。

11 项均有深浅前后原帧（**44 张**）及逐项对照（**22 张**）。全部像素差非零，最小 **226 像素**（初始 45 数字），命令草稿保留为 **320 像素**，重录草稿为 **292 像素**。原帧为 Linux ui.Tester 的 720×400/440 软件自绘；总览缩图和 4× 最近邻局部不冒充高 DPI 真机证据。控件面截图没有全页 save alert；错误反馈由 saveFailed 事件断言及既有 banner B 测试覆盖。

| 项 | 场景文件前缀（每项均有 light/dark before/after/comparison） |
| --- | --- |
| 1 | initial-custom |
| 2 | draft-menu-refresh |
| 3 | preset-save-failure |
| 4 | custom-save-failure |
| 5 | numeric-boundary |
| 6 | host-custom-editor |
| 7 | shortcut-failure-add |
| 8 | shortcut-failure-command |
| 9 | shortcut-failure-mode |
| 10 | shortcut-failure-remove |
| 11 | shortcut-failure-record |

[浅色总览](P13-it/overview-light.png) · [深色总览](P13-it/overview-dark.png) · [像素差](P13-it/image-differences.txt)。逐项文件名为 `P13-it/floter-P13-it-<场景>-<主题>-<阶段>.png`。代表图保留 `/tmp/floter-P13-it-settings-{light,dark}.png`。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿原近黑与浅色 token |
| A2 每轮换字体 | 不适用；中文原生系统字体与项目合约优先 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生功能设置 |
| A5 三等分/卡片海 | 无新增包装或卡片 |
| A6 假数据 | 无产品假指标；45/53/86400 与命令为事件夹具 |
| A7 空词/感叹号 | i18n 文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原标题保留；截图为控件/快捷键分组，不声称全页标题改版 |
| A10 Bento | 不适用 |
| A11 竖线装饰 | 无新增 |
| B1 设计读取 | 3/2/7：原布局、高频即时操作、原密度 |
| B2 主题锁定 | 每个 Tester 固定深/浅，无混合 |
| B3 记忆点 | 失败后还能在原字段继续，浮层只属于当前值 |
| B4 动效气质 | 恢复 0ms，克制、可预测 |
| C1 目的/频率 | 无新增键盘动画 |
| C2 all/ease-in/scale(0)/纯 fade | 无新增 |
| C3 时长/按压 | 沿原按钮颜色与按压状态，不增时长 |
| C4 origin | 菜单仍从原选择器出现，失效时退场 |
| C5 reduced-motion | 所有新 ui.Tester 使用 ReduceMotion=true |
| C6 hover 媒体查询 | Go 自绘无 CSS，原指针路由保持 |
| C7 布局动画 | 无新增，重复行退出使用原布局 |
| C8 重播 keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新增动画，未声称完成真机慢放验收 |
| D2-1 字体栈 | 沿原中文系统字体，无 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px / 1.5 未满足；原 Caption 9.9/正文 10.8 与行盒仍需拍板 |
| D2-3 混排空格 | i18n 零改，无新增混排文案 |
| D2-4 标点/暗色对比 | 原标点/墨不改；未声称所有旧次要墨满足 7:1 |

本轮自发现缺陷写入本仓 §6.221 和事件测试，不将一次性实现修复提升为全局 skill 审美偏好。用户本轮授权优先于四方向、网页 viewport 和装饰数量等泛化要求。

## 仍待

1. 下一轮先查 `internal/shell/custom.go:28-42`：先注册新键再写磁盘，写失败时 OS 中的实际注册是否回到 Store 原值。当前只有代码线索，本轮测试覆盖设置 UI + 真实文件失败，尚未覆盖宿主/系统注册；先复现，不把本轮草稿修复冒充修复了系统注册。可顺查失败后原命令实际派发归属。
2. 活动驻留草稿始终优先于宿主另写值，沿既有参考契约；没有擅自改草稿优先级。字号/行盒、accent 禁用填充、未知多选/Radio、极小数字步长、176-9/161-4 保存期间草稿、冲突另一端变化的拒绝历史、终端 S1b/S1c/S2 尾继续待拍板。
3. Windows/macOS 只有 vet/build，尚无真机输入法、Option+字母或系统热键故障证据。八条早期 WIP 全部保留，没有新 stash。

## 完整门槛

产品 `b3d08780`；上一执行被中断，完整测试没有完成，不能宣称全绿。P13-iu 开盘抢救 stash `aa5e214d` 后，真实全量得到 **5096 PASS / 1 FAIL / 3121 函数**：旧 B `TestWindowBehaviourControls` 报 `the preset residency did not show`。45 秒自动展开后，宿主更新 120 秒仍显示 Custom。补修区分自动展开与用户明确选择/输入的草稿：未编辑值跟随宿主；活动草稿继续优先。旧 B 断言零改。定向结果与后续完整门槛见 P13-iu 报告，不把未完成日志当作通过证据。
