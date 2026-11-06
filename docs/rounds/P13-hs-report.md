# P13-hs：配置动作的确认、恢复与状态归属

起点 `6e3e4f09`，pull --ff-only 最新，工作树干净。done.log 在开盘时不存在。临时主线优先：同族八项，跨配置浮层按钮与启动器配置状态组件，每两项提交推送。

## Stash 清点

八条均为早期独立 WIP，SHA 与 P13-hr 一致，逐一 reverse-check 退出 1。已归档 stale-kill 的接续记录在 backlog 与前轮报告，未重复试合。原件逐条保留，不把旧主题/Tauri 实现覆盖当前 Go。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap WIP，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider WIP，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实施记录

取消与失败键盘路径已在未改产品上实跑红；首次确认与成功返回均原样通过。后续逐批追加实测，不把候选写成完成。

### 第一批

1. 现象：取消后 Space 无效、焦点消失。改动：确认状态退出且焦点仍属于原按钮组时，把焦点交回主按钮。验证：键盘取消仅 Unarm=1、Run=0，随后 Space 再次 Arm；深浅两套通过。原红串 `cancel lost action keyboard focus`、`cancel cannot rearm by keyboard: arms=1 armed=""`。
2. 现象：运行失败插入错误句，按钮身份改变，键盘不能重试。改动：主按钮在初始/确认/失败三态共用显式 key，错误句不再挤掉身份。验证：失败后原焦点仍在，Space 能重新 Arm，Run 只发生一次；成功返回、初次 Arm 与跨组操作回归通过。原红串 `failure lost action keyboard focus`、`failure cannot rearm by keyboard: arms=1 armed=""`。

M1 回退 pluginsheet.go 两项修复，两个事件深浅实跑红；MD5 恢复 OK，touch 后 6 PASS。旧断言 V/B/S 修改数 0/0/0。深浅 before/after 已落盘；人工看图：取消后主按钮重新出现蓝色焦点环，失败句保留，右侧按钮的焦点环不再消失。基线 3971 PASS / 0 FAIL / 2858 函数。

第一批 `41c51dac` 已推送，相关回归 38 PASS / 0 FAIL，settingsui vet 净。

### 第二批

3. 现象：确认后的动作被 schema 删除，再加入同 key 时仍显示“确认清理”，已失败的提示也会复活。改动：每帧同步配置时，持有者对照当前 Action 字段清理 Armed/ActionFailed。验证：真实 Click→移除→重加后是初始按钮；持有者同时覆盖 armed/failed 两种状态。原红串 `retired action kept state: armed="clear_history" failed=""`、`re-added action resurrected confirmation or failure`。
4. 现象：动作改为开关再改回，也继承旧确认/失败。改动：存在同名字段也必须仍为 Action 才保留状态。验证：真实 Click→Toggle→Action 后必须重新确认；原红串 `re-added action still asks to run the retired confirmation`。未改模式退出/高度链。

M2 回退状态清理，两个真实 UI 场景深浅实跑红；MD5 恢复 OK，touch 后 6 PASS。定向回归与两包 vet 通过。V/B/S 0/0/0。

第二批 `480e78c5` 已推送，相关回归 30 PASS / 0 FAIL。

### 第三批

5. 现象：RunConfig 没核对是否确认，首次直接调用与一次确认后的重复回调都会执行。改动：只接受当前 Armed 对应 key；执行前消费这次确认。验证：未确认 Run=0，一次 Arm 后连续两次 Run 总共只派发 1 次。缺陷回退红串 `unconfirmed action ran 1 times`、`one confirmation dispatched 3 commands`。
6. 现象：已删除字段的 Arm/Run 会覆盖、消费另一个动作的确认，普通值字段也能进入动作链。改动：Arm/Run 同时核对当前 schema 字段必须为 Action；不匹配的调用保留其他动作的确认。验证：stale key 与 Toggle 均不能派发或夺走确认，合法动作仍只执行一次；改型到 Text 即使尚未画下一帧也拒绝旧回调。原红串 `stale arm replaced current confirmation: "removed"`、`stale run consumed current confirmation: runs=1 armed=""`。

M3 回退两入口校验，两项实跑红，MD5 还原与 touch 后 2 PASS。相关回归和 launcher vet 通过，V/B/S 0/0/0。这两项是入口回调计数证据，不伪称普通静态截图能证明重复派发。

第三批 `bb91c79d` 已推送，相关回归 34 PASS / 0 FAIL。

### 第四批

7. 现象：写回回调内关闭配置，返回后解引用 nil；回调内换配置，旧接受值或失败标记覆盖新浮层。改动：保存发起写入的 Config 实例，返回后仅向仍为该实例的浮层发布答案。验证：关闭/切换 × 接受/拒绝四场景；写回只调用一次，目标 plugin/key/value 正确，新值不变、失败标记不串。原红串 `write answer crashed after close-refused: runtime error: invalid memory address or nil pointer dereference`、`old write answer contaminated new sheet: failed=false enabled=false`。
8. 现象：动作失败回调关闭配置后崩溃；切换配置后错误挂到新动作。改动：动作返回时同样检查发起实例。验证：关闭/切换 × 成功/失败四场景，新浮层的确认保留、ActionFailed 为空，命令只调用一次。原红串 `action answer crashed after close-refused: runtime error: invalid memory address or nil pointer dereference`、`old action answer contaminated new sheet: failed="clear_history" armed="clear_history"`。

M4 回退实例归属，两测试含八子场景实跑，六子场景红；MD5 还原、touch 后 10 PASS。定向回归与 launcher vet 通过。V/B/S 0/0/0，旧断言零改；八项均已实现，完整门槛固定源码后执行。

第四批 `43d22172` 已推送，相关回归 44 PASS / 0 FAIL。产品源码固定后启动三平台与全量门槛，后续不再注入 mutation。

## 偷了什么

已按要求读取 qiaomu-design 的 SKILL → user-preferences → craft-loop、preflight、chinese-typography；Apple DESIGN 仅取单一交互蓝、弱化墨 .48、柔和圆角的角色，映射现有 accent、框架禁用 .5、RadiusSM=8。不是重设计，不替换本仓 token，也未调用外部执行器。

- craft-loop 工艺密度菜单第 3 项「品牌 focus 环」：取消/失败后键盘目标仍为 1 个，恢复原蓝色焦点环，新增焦点动画 0ms。见 cancel 与 failure 的深浅图。
- 第 4 项「签名交互」：额外等待 0ms，取消 Run=0，一次确认最多 Run=1；Space 直接恢复下一次确认，失败句退场不再拆掉控件身份。
- 第 12 项「材质细节」：按钮保留原半高圆角 `Space(3.5)`、原 1px 边与受光条件，新增材质层 0、布局包装 0。旧确认退场后画面减少一枚无效取消按钮，层次属于当前动作。

未为凑五项密度增加装饰；用户临时主线禁止功能扩张，已有方向的打磨路径优先。设计记忆落本仓 §6.194 与事件 pin，未修改全局 skill 偏好。

## 两轮看图与预检

第一轮旧实现 ui.Tester 导图定位丢焦点与复活确认；第二轮同事件重放深浅图复核。16 张原始图覆盖四条可见路径，左右对照图左旧右新。第 5–8 项属于命令与回调边界，用调用计数、崩溃与实例断言验收，不用无差别静态图冒充证据。

四视角：艺术总监看旧确认与取消是否在退役后多占注意力；工程师看取消焦点消失与失败插句引起身份改变；任务使用者看能否持续 Space 重试、一次确认是否只执行一次；读者看失败句是否保留完整并有当前按钮可操作。修复后取消与失败蓝环完整，动作重加只显示初始按钮，底面和行高相同。测试以 ReduceMotion=true 渲染，不声称三平台真机或系统桌面透明混合实测。

4× 检视：原焦点环没有切进卡片边缘；失败句与按钮之间的原间距仍完整，没有因焦点交接新增边或面。[取消浅色局部](P13-hs/cancel-light-detail.png)、[失败深色局部](P13-hs/failure-dark-detail.png)。

前后图（左旧右新）：[取消·浅](P13-hs/cancel-light-comparison.png) / [深](P13-hs/cancel-dark-comparison.png)，[失败·浅](P13-hs/failure-light-comparison.png) / [深](P13-hs/failure-dark-comparison.png)，[移除重加·浅](P13-hs/removed-light-comparison.png) / [深](P13-hs/removed-dark-comparison.png)，[改型恢复·浅](P13-hs/retyped-light-comparison.png) / [深](P13-hs/retyped-dark-comparison.png)。

| 检查 | 结论 |
| --- | --- |
| A1 紫蓝渐变/纯黑/霓虹/渐变字 | 无新增；原生近黑底沿用 |
| A2 不用系统字体 | 不适用，用户现有合约与中文 D2 优先，保留系统中文字体 |
| A3 斜体 | 无 |
| A4 居中 Hero | 不适用，原生配置表单 |
| A5 三等分/卡片海 | 无新增卡片或容器 |
| A6 假数据 | 无产品假指标；夹具使用真实 schema 形状 |
| A7 空词/感叹号 | 产品文案零改 |
| A8 编号眉批 | 无新增 |
| A9 H1 换行 | 页头未改，截图完整 |
| A10 Bento 空洞 | 不适用 |
| A11 左侧竖线强调 | 无 |
| B1 读取/拨盘 | 克制、即时、原密度，与规格一致 |
| B2 主题锁定 | 深浅独立渲染，无混搭 |
| B3 记忆点 | 当前动作始终有键盘落点，过期确认及时退场 |
| B4 动效气质 | 额外过渡 0ms，不打断键盘连续性 |
| C1 目的/频率 | 高频键盘不加动画 |
| C2 all/ease-in/scale(0)/纯 fade | 无新增 |
| C3 时长/按压 | 原 120ms 颜色过渡与 0.97 按压沿用 |
| C4 浮层来源 | 未新增浮层；按钮仍在原行尾 |
| C5 reduced-motion | 深浅事件均以 ReduceMotion=true 跑 |
| C6 hover 条件 | CSS 不适用，原生指针逻辑未改 |
| C7 布局动画 | 无新增 |
| C8 快速触发 | 一次确认只执行一次，不重播动画 |
| C9 手势 | 未增改手势 |
| C10 Framer/Motion | 不适用，Go/mygo |
| C11 动效抽查 | 未修改动效，无真机慢放证据，不虚报 |
| D2.1 中文系统字体 | 沿用；测试与截图统一 zh_CN.UTF-8 |
| D2.2 ≥14px/1.5/字重 | 未通过，现有字号/行盒属用户保留拍板项，本轮不越权 |
| D2.3 混排空格 | 产品文案零改，本轮中文标签无新增混排问题 |
| D2.4 标点/暗色7:1 | 文案未改；默认弱墨不保证7:1，沿用对比度合约与拍板项，未声称通过 |

## 仍待与证据边界

1. 下一轮首选集成设置已有动作的取消/失败恢复：runForm 参数、配置折叠确认、卸载取消的焦点和忙态交接；只是线索，必须先复现。
2. 第 5–8 项直接调用已有配置入口、注入同步回调验证；它们证明边界故障可复现，不等于在系统插件上重现了用户真实崩溃。前四项以真实 UI 事件与图验证。
3. 终端 S1b/S1c/S2 尾、字号/行盒、禁用 accent、未知 Radio、多选未列举值、保存期间草稿归属继续待拍板。八条早期 stash 保留。
4. 无新增依赖、页面、系统或架构重构。V/B/S 0/0/0，旧测试零修改，新增测试全部执行，无新增 Skip。

## 最终门槛

基线 **3971 PASS / 0 FAIL / 2858 Test 函数**；最终 **4007 PASS / 0 FAIL / 2870 Test 函数**，增加 36 PASS、12 函数。普通全量和 verbose 全量分别完整退出 0；最终源码 MD5 五项逐字节 OK。gofmt 输出 0 字节，linux/windows/darwin 的 vet/build 均退出 0，合约增补后 internal/ci 另跑通过。未修改旧断言、未减少测试、无新增 Skip。

四批产品 commit/push：`41c51dac`、`480e78c5`、`bb91c79d`、`43d22172`。每批先定向回归与对应包 vet，再提交推送；完整三平台与两次全量按用户“一轮一次”在第四批后集中执行，未声称每批都跑完整门槛。

证据：[门槛汇总](P13-hs/gates.txt)、[普通全量](P13-hs/full-test.txt)、[文档检查](P13-hs/docs-ci.txt)、[源码校验](P13-hs/final-source.md5)。verbose 原始完整日志 `/tmp/floter-P13-hs/full-verbose.txt`；基线 `/tmp/floter-P13-hs/baseline.log`。四组 mutation 的红/绿原文在本目录 M1–M4-red/green.txt，对应 MD5 文件保留；M1/M2/M3/M4 恢复后分别 6/6/2/10 PASS。

本轮从 12:49 UTC 开盘至完整门槛结束约 24 分钟，低于 45–75 分钟目标；已完成八项并分四批推送，未为延长时间另开系统或重复跑门槛。
