# P13-lc · 恢复与管理入口在刷新后保持可见

开盘bb08b7d0，分支mygo-rewrite、pull已最新、工作树空。统一验证入口已存在，直接复用；上轮日志现场计数7250 PASS，3666测试函数。给定done.log不存在。

## stash清点

本轮逐条导出补丁并执行reverse-check，全为1；无可直接判定已被HEAD覆盖者，未drop。以下旧独立WIP与P13-lb SHA相同；旧主题/Tauri链不覆盖现行行为。移植尚未完成，明确保留。

|ref|SHA|判断与处置|
|---|---|---|
|stash@{0}|422483b4|旧keycap/主题；保留|
|stash@{1}|8af94a07|旧行距/滑块；保留|
|stash@{2}|7e4aa710|旧卸载布局；保留|
|stash@{3}|1e3aec94|旧数字字段；保留|
|stash@{4}|363f5a42|旧配置字段；保留|
|stash@{5}|9b8ce23c|旧快捷键；保留|
|stash@{6}|90d314b2|旧启动器动作栏；保留|
|stash@{7}|7a9f6e13|旧Tauri主线；保留|

原始检查与补丁：P13-lc/logs/stash-*。另核对~/.hermes/state/iteration/stash-backlog-floter.md：P13-gq/go已归档处理stale-kill；现存八条明确为早期独立WIP，reverse失败不能直接认定独有工作。沿用该清单保留，不用旧主题或Tauri链覆盖当前产品。本轮不声称清空历史stash。

## 逐项交付（增量记录）

1. 重新检查：前置增撤后焦点仍在、按钮已离屏 → 加入19px显现包络与前置/输出反馈，仅键盘入焦自动显现 → 三来源×深浅×键鼠四次增撤后可见，激活只检查weather一次。
2. 重连：同样刷新会挤走原重连入口 → 原入口观察布局反馈，并保持鼠标按下/松开位置 → 相同矩阵验证原ID、单次回调、等值数据与隐藏推荐不抢手动滚动。

第一批修前44 FAIL / 0 PASS；修后44 PASS / exit0。原始失败：`feedback reflow hides active outlet "Reconnect"`，末步target Y=685.30005/W=0/H=0。旧V/B/S断言修改0/0/0、新Skip0；完整门槛及独立mutation尚待执行。

3. 安装工具主页：本地/推荐/扫描更新使原主页按钮离屏 → 观察前置与输出反馈并保护鼠标按下 → 四次增撤保持可见，Enter/点击只打开原天气工具URL一次。
4. 健康重新检查：已有报告的重新检查入口被前置区块挤走 → 对既有图标按钮补键盘显现与反馈包络 → 相同矩阵保留weather报告归属、激活一次且无多余滚动。

第二批修前44 FAIL，修后44 PASS / exit0。第一批3bd4ea4c、第二批3224ebb1已推送。

6. 接管孤立目录：前置增撤使接管入口离屏 → 对原按钮的静息/禁用/等待面使用同一个反馈包络 → 四步刷新与键鼠验证原weather接管一次，没有接管推荐项。
7. 删除孤立目录：相同前置变化会挤走删除入口 → 原删除按钮补齐包络并保持等待锁 → 同样44用例矩阵中的删除半组验证原ID回调一次，没有更改宿主确认流程。

第三批修前44 FAIL，修后44 PASS / exit0；旧V/B/S断言修改0/0/0、新Skip0。

5. 权限披露：前置区块刷新使仍持焦的权限入口离屏 → 既有权限按钮接入前置反馈与显现包络 → 三来源×深浅×键鼠四步刷新保持可见，激活只打开原weather权限披露。
8. 命令搜索开关：前置区块增撤导致卡片身份变化、焦点丢失，Space无效 → 卡片使用稳定commands-search键，并加入19px显现包络 → 原设置焦点保留，Space/点击切换原设置，等值内容不抢滚动。

第四批首次22 PASS/22 FAIL，搜索开关仍有`focused=false`和`search switch did not change`；定位到卡片非稳定身份后修正产品，未改断言。唯一一次复验44 PASS / exit0。原失败保留在attempt1/batch4.log；第三批9f90c24c、第四批3d18a1d5已推送。八项均有修前实红，四批各44 PASS，共176个新增PASS。

## 深浅目视

images/内四张ui.Tester原图已逐张目视。同一代表状态只截一次：修前重新检查在窗口下方不可见；修后按钮及蓝色键盘焦点环完整出现在淡边上方。深浅均保留原卡面与故障提示色，没有叠加玻璃层。原图为720×520 Linux自绘，不声称原生Windows/macOS截图。

|主题|修前|修后|
|---|---|---|
|深色|[前](P13-lc/images/floter-P13-lc-recovery-dark-before.png)|[后](P13-lc/images/floter-P13-lc-recovery-dark-after.png)|
|浅色|[前](P13-lc/images/floter-P13-lc-recovery-light-before.png)|[后](P13-lc/images/floter-P13-lc-recovery-light-after.png)|

这轮为什么更轻、更有空间感：活动入口与16px淡边隔开，3px焦点环完整可见；仅恢复已有操作的19px净空，不新增卡片或材质。

## 变异实跑

|变异|注入与原始报错|实红|还原与touch后复验|
|---|---|---|---|
|M1|撤掉重新检查/重连integrationLead；`feedback reflow hides active outlet "Re-check"`，target Y=587.30005/W=0/H=0|44 FAIL / 0 PASS|md5sum -c OK；44 PASS / exit0|
|M2|撤掉命令搜索卡稳定键；`feedback reflow hides active outlet "Show PATH commands in search"`，focused=false|22 FAIL / 0 PASS|md5sum -c OK；22 PASS / exit0|

逐字节还原日志为mutation1-restore.log和mutation2-restore.log。统一验证入口的失败摘要原先只显示verbose的RUN开头，本轮改为优先提取前三条真实诊断；M2已现场验证摘要能指出焦点丢失。完整门槛在还原后启动，执行期间不改产品或验证脚本。

## 偷了什么

- craft-loop工艺密度菜单#3：沿用mygo的3px键盘焦点环，新增包络保证环不压进淡边。
- 菜单#4：一个签名交互，前置布局变化后0ms显现；按下鼠标不显现，避免松开失配。
- 菜单#9：沿用16px滚动淡边，与焦点环相加为19px净空，布局高度增量0。
- Apple供体取交互强调色专属、留白服务任务、单一主焦点三个动作，保留仓库token。未复制Apple网页字号；不为工艺凑满5项叠加装饰。

四视角复核：艺术总监看见修前缺少操作焦点→修后焦点环可见；工程师发现搜索开关稳定身份缺失→补键并实测；任务所有者需要原ID恢复入口→每次回调断言ID/URL；读者面对长说明看不到出口→真实720×520图确认原按钮进入视口。字号偏小仍属既有待拍板，未擅改。

## preflight逐条

A1无新增渐变/霓虹/装饰；A2系统中文字体沿用项目合约，通用“不得系统字体”不适用；A3无斜体；A4无Hero；A5无新卡片分组；A6夹具天气工具为测试数据、无虚构业绩；A7生产文案不改；A8无眉批；A9无标题改动；A10无Bento；A11无新增左侧装饰竖线。

B1克制变化/0ms动效/原密度与声明一致；B2深浅独立；B3记忆点是始终可达的活动入口；B4显现即时，符合键盘操作气质。

C1高频操作0ms；C2无transition-all/ease-in/scale入场；C3沿用按压反馈；C4无新浮层；C5测试使用ReduceMotion=true；C6无新增hover动画；C7无布局动画；C8不重播keyframes；C9无手势；C10无Framer；C11本轮无新增动效待判定，不重复截图。

D2-1系统中文字体与zh_CN.UTF-8；D2-2既有Caption9.9/正文10.8/行盒不满足通用≥14px和1.5行高，属于待拍板；D2-3生产文案未改；D2-4夹具全角标点，生产墨色不改，7:1本轮未重测。A2工艺密度仅取与本轮任务相关的3项，不为凑数改变方向。原生Go不使用网页四方向/URL同步流程；未调用K3；自发现问题沉淀为仓库测试和§6.282，不改全局skill偏好。

## 仍待

1. 本地表单真实展开/折叠对其他活动控件的影响尚未单独事件复现，下一轮先验；自然通过不计新bug。继续检查现有管理页中的入口和异步恢复，不重复本轮八项。
2. 字号/行盒、accent禁用、未知多选、保存期间草稿、输出首尾键/选区、字段删除改型焦点、终端S1b/S1c/S2尾仍待拍板；3秒窗口、保存与会话语义保持。
3. 八条历史独立WIP未完成移植，保留原stash；其他平台只交叉vet/build，不声称原生UI验收。

## 最终完整门槛

统一入口`bash tools/verify.sh P13-lc`仅执行一轮，exit0、failed_gates=0，终端10行摘要。四批聚焦、修前、变异与还原均使用同一入口的focused模式；只有第四批失败后复验一次。

|项目|实测|
|---|---|
|gofmt|空，exit0|
|Linux / Windows / Darwin vet|三项净，exit0|
|普通全量|exit0；settingsui 315.324s，shell 189.783s|
|verbose全量|7426 PASS / 0 FAIL，exit0；基线7250，增加176|
|Linux / Windows / Darwin build|三项exit0|
|测试函数|3674；基线3666，增加8|
|旧V/B/S断言修改|0 / 0 / 0；新增Skip 0|
|独立变异|M1 44 FAIL、M2 22 FAIL；各自MD5还原、touch后44/22 PASS|

[完整摘要](P13-lc/logs/verify.log)。详细原始日志、首次失败、两次变异、还原校验与产品diff均随本报告归档。四批产品3bd4ea4c / 3224ebb1 / 9f90c24c / 3d18a1d5已推送；合约§6.282与.pending-bugs.md已更新。完整门槛通过后仅补档案，没有再修改产品。
