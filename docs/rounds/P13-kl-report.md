# P13-kl · 安装输入与配置字段的完整活动边

13:49 UTC开盘，pull已同步且工作树空。实际最新P13-kk，P13-jx已修安装恢复。已读qiaomu-design、偏好、craft-loop、preflight、中文排版和Linear供体；Codex独立执行。

## 开盘抢救

031f454f53有独有P13-kk收尾文档与日志，通过git apply -3接回；8a268ffae9的tracked树与HEAD一致，第三父树有补充exit/计数文件。合并后发现verbose日志只有1637 PASS、pass-count为空，不能用exit0声称完整通过；正在重新跑acc02b1d完整基线。产品MD5与P13-kk已验收源码一致。逐条stash处置和最终数字后续补录。

## 进度

已写八候选与真实代码位置；先事件复现再实施。旧断言零改，新Skip零。

开盘完整基线6485 PASS / 0 FAIL / exit0、3527函数；P13-kk收尾独立提交e0240437并推送。两对象已保留完整archive引用后drop，活动10→8。

| 开盘ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 8a268ffae9 | tracked等于HEAD，无patch（reverse128）；第三父树有独有exit文件 | 接回第三父树，完整对象归档refs/archive/floter/P13-kk/完整SHA后drop |
| stash@{1} | 031f454f53 | reverse1，独有收尾文档与普通测试尾部 | git apply -3接回；完整对象归档同上后drop；报告补验后reverse不通过，不冒称原patch完全相同 |
| stash@{2} | 422483b4 | reverse1，旧keycap/主题 | 原件保留，不覆盖当前主题 |
| stash@{3} | 8af94a07 | reverse1，旧rows/slider | 原件保留 |
| stash@{4} | 7e4aa710 | reverse1，旧卸载尺寸/探针 | 原件保留 |
| stash@{5} | 1e3aec94 | reverse1，旧数字字段 | 原件保留 |
| stash@{6} | 363f5a42 | reverse1，旧禁用bar/表单 | 原件保留 |
| stash@{7} | 9b8ce23c | reverse1，旧快捷键/i18n | 原件保留 |
| stash@{8} | 90d314b2 | reverse1，旧动作栏 | 原件保留 |
| stash@{9} | 7a9f6e13 | reverse1，旧main/Tauri发布链 | 原件保留，不混入Go链 |

## 第一批

1. registry包名：现象→Shift+Tab返回顶边落在Y51.4的视口起点、深色蓝红差21；改动→原输入显现19px净空，Grow留在外层保持292px宽；验证→原事件/失败恢复共25 PASS，编辑只改包名、滚轮可自由离开。
2. registry版本：现象→同一行独立Tab入口同样落在淡区；改动→独立一次显现19px，固定160px；验证→通过真实Tab从包名进入，以真实光标确认位置（原无显式label），继续输入只改版本。

M1原样撤销两处接线，真实日志见M1-red；还原MD5与touch复验见M1-restore/M1-restored。无旧V/B/S修改、新Skip0；本批包vet通过。

第一批1257c70a已推送。M1原报错：`registry-version top active edge missing: blue-red=21`；还原MD5 OK、touch后6 PASS。目视浅色版本图：原框顶部贴住页头被洗淡，修后蓝边完整、文字与160px宽保持。

## 第二批

3. 配置文本：现象→返回时原1px活动边落在淡区，深色蓝红差21；改动→控制列内一次显现19px；验证→深浅像素、继续编辑本字段、邻项值与后续滚轮。
4. 配置密码：现象→与文本相同的顶部边界缺损；改动→同列显现净空，Password遮蔽调用不动；验证→深浅事件与原配置表单/身份/恢复回归，旧断言零改。

分批条件仅用于每两项及时交付，最终覆盖单控件配置字段；嵌套多选列表保留自己的滚动链。

## 中断接续清点

接续HEAD 2427b1ff，分支已同步、工作树起初为空。安装失败恢复已由P13-jx完成，按最新队列接续P13-kl，不另起轮号。第二批2427b1ff已推送。重新读取qiaomu-design、偏好、craft-loop、preflight、中文排版；供体路径核实为references/design-systems/linear.app/DESIGN.md。

| 接续ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 98b34e773b | reverse1；独有路径/数字接线及两测试，第三父树含门槛脚本与第三批日志 | 原对象先归档refs/archive/floter/P13-kl/完整SHA；第三父树接回，产品待本次复验后独立提交 |
| stash@{1} | 422483b4 | reverse1，早期keycap/主题WIP | 原件保留 |
| stash@{2} | 8af94a07 | reverse1，早期rows/slider WIP | 原件保留 |
| stash@{3} | 7e4aa710 | reverse1，旧卸载尺寸/探针 | 原件保留 |
| stash@{4} | 1e3aec94 | reverse1，旧数字字段WIP | 原件保留 |
| stash@{5} | 363f5a42 | reverse1，早期配置控件WIP | 原件保留 |
| stash@{6} | 9b8ce23c | reverse1，旧快捷键/i18n WIP | 原件保留 |
| stash@{7} | 90d314b2 | reverse1，旧动作栏WIP | 原件保留 |
| stash@{8} | 7a9f6e13 | reverse1，旧main/Tauri链 | 原件保留，不覆盖当前Go实现 |

八条早期WIP与前轮SHA一致，无新的接续证据；不把reverse失败直接等同于应合入的新工作。

## 第一轮看图与复核计划

- 艺术总监：原数字框顶边融入页头淡区，焦点的层级低于其余静息字段；保留原1px蓝边，通过完整显现恢复识别。
- 工程师：第三批净空条件按字面类型临时列举，完整收尾需与configField既有小写化/默认文本分支一致，防止空类型漏接；多选列表另有滚动链，不包整列表。
- 任务使用者：返回数字框后虽能继续输入，但半条活动边让当前编辑位置不清楚；八项都必须实际输入/选择/切换，且不能误触保存或安装。
- 读者：浅色图上旧顶边贴Y51.4，修后在Y70.4；正文仍沿既有小字号，属于已记录拍板项，本轮不擅改行盒。
- 图证工具：抢救稿的局部图横向2倍、纵向4倍会拉伸，已修成等比例4倍；总览删除原先每行多出的286px空白。此为证据修正，不计产品修复项。

## 第三批（抢救稿独立接续）

5. 配置路径：现象→Shift+Tab返回时顶边落在Y51.4，深色蓝红差21、浅色30；改动→控制列内接回19px一次显现净空；验证→本次先接测试、旧产品真实重现失败，原始日志resume-batch3-before.txt；接回产品后验证路径原字符串编辑与邻项不变。
6. 配置数字：现象→同一事件使数字框顶边进入16px淡区；改动→接回同一控制列的显现接线，原ParseFloat/边界验证不变；验证→旧产品同样实红，修后继续输入12得到float64(12)，相邻c5仍为stored；后续滚轮仍能离开焦点。

原始失败串：`path top active edge missing: blue-red=21`、`number active edge intersects fade: target={X:325.8 Y:51.399994 W:369 H:28}`。两产品/测试文件均由git apply -3接回，无冲突；本次定向与vet结果随批次日志落盘。

本次第三批65 PASS / 0 FAIL / exit0，包vet exit0；旧V/B/S 0/0/0、新Skip0。抢救稿完整patch reverse-check通过，完整原对象归档引用核对一致，提交后drop活动stash；其余八条保留。接续HEAD完整基线仍在运行，不拿部分PASS数充当完整数字。

第三批0a34638b已推送，98b34e773b已归档后drop，活动9→8。接续HEAD（2427b1ff编译的测试）完整基线已结束：6497 PASS / 0 FAIL / exit0、3531函数；原日志将归档为resume-baseline。

## 第四批

7. 配置下拉：现象→返回时顶外环进入淡区，深/浅蓝红差2/1，近乎消失；改动→单控件共享19px显现净空；验证→旧产品深浅均真实失败（batch4-before），修后验证上下外环、Down/End/Enter选South且只改当前字段。
8. 配置开关：现象→相同事件使32.4×18开关的顶部外环缺失，深/浅蓝红差2/1；改动→沿用同一显现范围，开关尺寸与原环不变；验证→旧产品深浅均失败，修后Space只切当前开关，旁边值不变且滚轮仍自由。

类型条件改为与现有configField一致的小写判断，排除自带滚动区的multiselect；空类型/大小写TEXT/未知类型沿用默认文本控件并补测，不另计修复项。旧V/B/S断言仍零改，新Skip零。

第四批配置/安装/身份/返回/只读与表单回归98 PASS、包vet exit0。六类配置字段增加视口底端正向Tab抽查，八项及默认文本分支定向34 PASS；原严格像素阈值60与16px净空均保留，没有放宽断言。

## 两类正式变异（本次重新实跑）

- M1撤回registry两个输入的显现接线：`registry-name top active edge missing: blue-red=21`、`registry-version top active edge missing: blue-red=30`，四个深浅场景实红，exit1，见M1-resume-red.txt。
- M2关闭配置控制列显现：六类字段与三种默认文本类型均实红，exit1。顶部例：`boolean top active edge missing: blue-red=2`；底部例：`text forward bottom active edge missing: blue-red=22`，见M2-red.txt。
- 两产品文件均先保存MD5，回注后恢复原件；`md5sum -c`逐字节成功，随后touch并`-count=1`复跑34 PASS/0 FAIL/exit0。日志mutation-restore.txt、mutation-restored.txt，不依赖缓存。

第四批14513ebd已推送，整轮八项按四批每两项提交；接续的第三批独立保住抢救工作。完整三平台vet/build均exit0，gofmt输出为空；普通与verbose全量仍运行，暂不填最终计数。

## 为什么更轻、更有空间感

原活动边完整离开淡区后，编辑位置与页头之间有19px显现净空，层级可读；不增加玻璃层、阴影、描边宽度或高度，静息页面完全相同。

## 偷了什么

- 从craft-loop.md菜单第3项“品牌focus环”取原活动墨与偏移：文本保留框内1px边；下拉/开关保留原生外扩3px环，只让它们离开16px淡区。
- 从第4项“签名交互”取一次滚动显现：净空16+3=19px，正padding19/负margin19，焦点进入0ms显现一次；之后滚轮自由，不持续锁焦点。
- 从第12项“材质细节”取少层原则：新增组件透明度0、阴影0、额外高光0，保留既有Control面与Hairline；16组静息PNG逐字节相同。
- Linear供体具体取#0f1011面板/#191a1b浮面分层、0.05–0.08弱边、单一accent只承担交互三条DNA；映射到本仓既有token，未移植其紫色、字体或Web布局。只补可见状态，不用新装饰凑满五项。

## 出图与第二轮目视

ui.Tester实际720×520，深浅分别导出。原始图56张（顶端32、配置底端24），28组对照、16张静息、2总览、6张等比例4×局部；make-comparisons.py严格要求每组活动图有差异、每组静息PNG字节相同，日志image-differences.txt。

目视：深色开关修前顶半环融进黑色页头，修后环完整包住白色滑块；浅色下拉修后顶环与页头分离，右侧箭头与胶囊宽度不变；深色密码框在底端修前下边线消失于黑色淡区，修后上下边完整，密码仍为圆点。顶部19px与底部19px均由真实Tab事件触发，不是截图平移。配置类型兜底和底端事件是第二轮复核补强；未改设计方向。

## Preflight逐项结论

| 检查 | 结论 |
| --- | --- |
| 0 偏好/缺陷沉淀 | 已读偏好；P-1无斜体、P-3系统中文字体、P-7任务契约、P-38无左装饰条沿用；缺陷与pin沉淀在本仓§6.265，不修改全局skill偏好。 |
| A1 渐变/纯黑/霓虹 | 产品差异无新增渐变、纯黑、外发光或渐变字；原页头深色按既有token。 |
| A2 字体默认 | 保留项目字体；中文规范优先于“每次换字体”，不引入字体。 |
| A3 斜体 | 无新增斜体；截图未见中文倾斜。 |
| A4 Hero | 原生设置表单无Hero，不适用。 |
| A5 卡片模板 | 保留现有集成卡；新玻璃/卡片0。 |
| A6 假数据 | 截图为明确测试夹具，未把样例写入产品数据或宣传。 |
| A7 空话/感叹号 | i18n文案零改；没有新增空话。 |
| A8 编号眉批 | 无新增眉批；“配置5”为测试字段标签。 |
| A9 H1行数 | 无新标题；页头未改。 |
| A10 Bento空洞 | 无Bento；对照总览已清除多余空行。 |
| A11 左侧装饰线 | 无新增选中竖线，焦点是完整原环。 |
| A2工艺流程 | 两轮看图、四视角、三条DNA已记录；只采用3项适用工艺。≥5装饰不执行：用户要求既有功能细节与不扩张优先。 |
| B1 设计读取/拨盘 | 紧凑原生表单，冒险3/动效2/密度7；只恢复键盘活动识别。 |
| B2 主题锁定 | 深浅各自独立，原图未混主题。 |
| B3 记忆点 | 返回哪一格，哪一格的完整活动边出现。 |
| B4 动视觉一致 | 高频键盘0ms，无新增位移动画。 |
| C1 目的/频率 | 焦点进入一次显现，后续滚轮自由；事件pin通过。 |
| C2 过宽transition/ease/scale | Go自绘无CSS，无新增动画。 |
| C3 ≤300ms/按压 | 新增0ms；原控件按压处理不动。 |
| C4 浮层origin | 下拉仍走原valueSelect触发源，未新建浮层。 |
| C5 reduced-motion | 深浅夹具以ReduceMotion=true验证。 |
| C6 hover能力 | 本轮无hover动画；Web媒体查询不适用。 |
| C7 动画属性 | 无width/height/margin/padding动画；19px是显现几何。 |
| C8 高频重播 | 无keyframes或动画重播。 |
| C9 手势物理 | 无新拖拽/手势/sheet，不适用。 |
| C10 Motion性能 | 无Framer Motion，不适用。 |
| C11 慢放 | 新动画0，静帧与实际Tab/滚轮事件验收。 |
| D2-1 中文字体 | 沿用系统字体，无装饰字或字体下载。 |
| D2-2 字号/行高/字重 | 通用≥14px/1.5行高未满足：既有Caption9.9/正文10.8与行盒属于明确需拍板项；不冒称通过，不越权改。 |
| D2-3 混排空格 | 产品文案零改；测试“配置 5”等标签留空格。 |
| D2-4 标点/对比度 | 文案与墨token不变；本轮只实测活动边，不宣称整页中文正文全部达到7:1。通用要求与既有合约的历史差异保留。 |

没有把“部分门禁不适用/待拍板”写成全数通过；原生Go自绘不采用HTML/browser模拟证据。

## 仍待

1. 下一轮先事件复现参数运行表单文本/数字/路径/选择控件及多选列表内活动项在视口上下边缘的完整墨。整组列表有自己的滚动链，本轮不把显现整个列表等同于显现内部选项；线索未复现前不记bug，不重复本轮八项。
2. 字号/行盒、accent禁用填充、未知多选/Radio、保存期间草稿、输出首尾键/选区、终端S1b/S1c/S2尾继续需拍板。3秒窗口、保存与会话语义零改。
3. Windows/macOS本轮只有交叉编译与vet，未跑原生交互或文件选择弹窗；深浅图均为Linux ui.Tester。
4. 八条早期独立WIP原SHA保留，本次新抢救稿已接回并归档；stash表区分开盘与接续时的ref，避免drop后编号漂移。

## 最终门槛与提交

2026-11-08，本轮完整收尾 **6519 PASS / 0 FAIL / 3536函数**，`tests-verbose.exit=0`、`gates.exit=0`。相对原轮开盘6485/3527为 **+34 PASS、+9函数**；相对接续前6497/3531为 **+22 PASS、+5函数**。八项修复与三种默认文本归属的补充覆盖，没有把补充覆盖另算产品项。

| 检查 | 实测 |
| --- | --- |
| gofmt -l cmd internal | 空，exit0 |
| go vet ./...（linux/windows/darwin） | 三份日志均空、exit0 |
| go build ./...（linux/windows/darwin） | 三份日志均空、exit0 |
| go test -count=1 ./... | 全部包通过、exit0 |
| go test -count=1 -v ./... | 6519 PASS、0 FAIL、exit0 |
| 文档CI补验 | 最终文档修改后internal/ci通过、exit0；产品MD5再次一致 |
| 测试函数 | 3536，原3527；未删除旧函数 |
| 旧断言V/B/S | 0/0/0；新增Skip0，无容差或断言弱化 |
| 正式变异 | M1/M2均exit1，MD5逐字节还原，touch后34 PASS/exit0 |
| 图证 | 108 PNG：56原始前后、28对照、16静息、2总览、6局部；16组静息字节相同 |

产品四批1257c70a / 2427b1ff / 0a34638b / 14513ebd均已推送；第三批为抢救稿独立接续。原轮13:49 UTC开盘、中断稿14:20 UTC归档后继续同一P13-kl，未另起轮号或把中断等待计作开发时间。收尾仅归档完整门槛、报告和待办，不追加无关功能。

图证入口：[深色总览](P13-kl/overview-dark.png)、[浅色总览](P13-kl/overview-light.png)、[开关深色](P13-kl/boolean-dark-comparison.png)、[下拉浅色](P13-kl/select-light-comparison.png)、[密码底边深色](P13-kl/password-bottom-dark-comparison.png)。
