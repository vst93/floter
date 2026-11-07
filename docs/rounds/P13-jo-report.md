# P13-jo · 失败反馈与健康恢复

20:52 UTC 开盘，HEAD 2df719ab，pull 已最新、工作树干净。按最新 P13-jn 仍待选题；Codex 独立执行。done.log 与 stash-backlog 指定路径开盘不存在。

## Stash 清点

八条均为此前登记的独立早期 WIP，SHA 与 P13-jn 相同，无新增 stale-kill。逐条导出 reverse-check 都退出 1，沿用已归档语义审计，保留原稿；不把旧主题/Tauri 工作重复覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载/scratch，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | main/Tauri，reverse=1 | 保留 |

## 增量实施

规格已落盘。基线执行中，测试函数起点实测 3328。没有提前填写 PASS 或 mutation 数字。

1. 现象：当前有效诊断报告问题，浮层却显示成功勾。改动：将 DiagnosisFailed 传到既有反馈路由，显示警告字形和警告墨。验证：真实提供者子进程由设置 DiagnoseIntegration 动作启动，深浅 ui.Tester 均复现 `diagnosis-failed failure wears a success mark: type="success" timeout=4s`，改后警告类型/原 warningNotice 与可见句子同时通过。
2. 现象：剪贴板清除失败也显示成功勾。改动：清除结果沿原完成出口携带失败类型。验证：清除失败完成回调深浅图与准确类型/时效断言；原 hidden 系统通知及成功启动器路由继续通过。不声称模拟了磁盘写失败，后续补实事件验证。

第一批相关 **34 PASS / 0 FAIL**（batch1-green.txt），shell vet 退出0，旧 V/B/S 断言修改 **0/0/0**。深浅前后图已导出。正式 mutation 在最终产品上执行。cf949cd2 已推送；其正文及初版报告误写35，本处按原日志纠正为34，不改历史日志。

剪贴板进一步补验真实写失败：创建非空目录占住索引目标，使 Clear 原子写入失败；确认历史仍保留且警告可见，不只调用完成回调。

3. 现象：重连失败文案带成功勾。改动：Reconnect 的失败分支进入 notifyFailure。验证：设置动作真实返回 ErrNoIntegration 后，深浅图与回调均确认失败，改前原串 `reconnect-failed failure wears a success mark: type="success" timeout=4s`。
4. 现象：修复失败文案带成功勾。改动：Repair 的失败分支使用同一警告出口。验证：设置动作真实错误回调、深浅截图，改前 `repair-failed failure wears a success mark: type="success" timeout=4s`，改后失败类型与6s既有警告窗口正确。

第二批相关 **21 PASS / 0 FAIL**，shell vet 退出0。首稿测试导入误落在函数外导致编译错误，修正后才记录有效红绿；旧测试未修改。旧 V/B/S **0/0/0**。

第二批 f46a5dd1 已推送。

5. 现象：安装/接管失败被提示为成功；推荐连接的提交失败同源。改动：注册表准备/提交、孤包接管准备/提交及推荐连接提交五个失败出口统一警告。验证：本地 HTTP 503 与不存在的孤包触发真实动作，深浅均从 type="success" 实红到 error；原安装成功回归保留。
6. 现象：普通移除与组件卸载失败显示成功。改动：两个卸载错误分支使用失败出口。验证：真实 Uninstall/UninstallComponentized 错误回调，两个场景各深浅类型、时效、可见句子均断言；改前 `remove-failed failure wears a success mark` / `uninstall-failed failure wears a success mark`。

第三批相关 **30 PASS / 0 FAIL**，shell vet 退出0，旧 V/B/S **0/0/0**。范围内所有 notifyCompletion 的明确失败分支已排查。

基线首轮错误 locale 的进程在重跑中文基线后主动终止，原日志保留但不作门槛通过依据；中文基线继续。

第三批60349290已推送。正式开盘基线完成 **5727 PASS / 0 FAIL，exit=0**；测试函数起点3328。

7. 现象：探测期间同ID换包、修改探测声明、换程序或已经收到新报告，旧进程的失败仍写回当前工具。改动：探测与绑定错误写回前核对包/程序/版本/声明及起始报告，过期则拒绝，短写回段串行。验证：真实子进程启动/释放门控制上述四种刷新，改前均 `outdated probe overwrote current repository health (err=<nil>)`；修改后仓库逐字节不变，当前健康标签保留。目标删除是额外反向控制，不计第九项。
8. 现象：探测从失败恢复后仍是broken、命令不可用，错误时间残留；其他来源故障反而被清空原因。改动：成功只解除probe_failed故障及时间，按当前Enabled恢复enabled或disabled，其他故障保留。验证：真实进程先exit7后exit0，启用/停用/包校验故障三种状态各深浅；改前 `successful health retry left wrong availability: state="broken" running=false want="enabled"`、`recovered tool retained stale failure metadata`、`health retry erased an unrelated package failure`。

第四批首轮48 PASS / 0 FAIL、两包vet净。补查发现960×1000的整页截图裁掉了目标卡片，已改为960×1600并重验；正式前后图将在mutation中按同一夹具重出。首次改动未同步Reconnect对recordBindingFailure的调用，编译拒绝；已补齐来源参数后才记录绿色门槛。

第四批复验仍48 PASS / 0 FAIL，6b535619已推送。八项四批完成，测试函数3328→3337（新增8个独立验收入口与1个真实健康子进程入口）。

## 正式 mutation

最终代码串行注入四组缺陷；每组还原三个产品文件，五文件MD5全部OK，再touch后无缓存复跑。

| 组 | 注入 | 实红节点 | 还原后PASS | 原始报错摘录 |
| --- | --- | --- | --- | --- |
| M1 | 完成出口丢掉warning | 6 | 6 | `diagnosis-failed failure wears a success mark: type="success" timeout=4s`；clipboard-failed同型 |
| M2 | 重连/修复回到成功出口 | 6 | 6 | `reconnect-failed failure wears a success mark`；repair-failed同型 |
| M3 | 安装/移除的失败出口改成功 | 14 | 14 | `install-failed failure wears a success mark`；adopt/remove/uninstall均实红 |
| M4 | 恢复开盘探测写回与恢复逻辑 | 23 | 26 | `outdated probe overwrote current repository health (err=<nil>)`；`successful health retry left wrong availability`；`health retry erased an unrelated package failure` |

M4的目标删除3个节点原本通过，作为反向控制，不计修复数。原始日志M1至M4-red/green/restore.txt均入仓库。所有正式前后图由同一最终夹具导出。

## 偷了什么

实读 qiaomu-design 的 SKILL、偏好账本、craft-loop、preflight、chinese-typography、Apple DESIGN；采用用户指定的已有方向打磨。视觉变化幅度2、动效1、密度6。

- craft-loop 菜单 **#4 签名交互**：结果进入当前反馈面，额外等待 **0ms**，通知仍 **1条**；不新增动画。
- **#6 编辑细节**：失败沿既有 **6s** 警告窗口，成功保留 **4s**；没有改动确认窗口、保存延迟或TTL常量。
- **#11 图形资产**：复用已有警告字形替代错误的勾，尺寸仍 **Space(3.75)=15u**，不引入新图标或emoji。
- **#12 材质细节**：保留浮层 **1px** 顶受光边；新玻璃 **0层**，不重叠玻璃、不模拟blur。
- **#5 多层阴影**：保留浮层环境/接触 **2层**；健康卡内部外影 **0层**。Apple供体只取交互专用accent、8px圆角与0.22柔影的角色，现有本仓token优先，不冒称本轮改过这些数值。

**为什么更轻、更有空间感**：失败在原浮层中用准确字形说清，恢复后撤掉旧故障面；当前工具卡不再被迟到的旧警告撑高。没有新增一层解释卡片。

## 两轮图像复核

第一轮实看浅色失败全页与深色健康全页。艺术总监：失败句与成功勾矛盾；工程师：1000px高的健康截图未覆盖目标卡；使用者：健康但无法运行；读者：原Caption小与次要墨偏弱属于既有字号拍板项。前三项分别由严重度、1600px真实完整帧和恢复接线修正；第四项不越权改变字号。

第二轮检查深浅对照：失败为警告字形；声明变化后旧异常块撤回；健康恢复后“已损坏”和“重新检测”消失，“运行”重新可用，开关保留原启用值。64张原始PNG与16张局部对照已入本目录，原路径 `/tmp/floter-P13-jo-<场景>-<dark|light>-<before|after>.png`。对照只裁剪ui.Tester帧，未合成产品内容。像素记录见 `P13-jo/image-differences.txt`。

| 项 | 浅色对照 | 深色对照 | 看见的变化 |
| --- | --- | --- | --- |
| 1 诊断 | [图](P13-jo/diagnosis-failed-light-comparison.png) | [图](P13-jo/diagnosis-failed-dark-comparison.png) | 报告问题由勾变为警告 |
| 2 清除 | [图](P13-jo/clipboard-failed-light-comparison.png) | [图](P13-jo/clipboard-failed-dark-comparison.png) | 清除失败用警告墨 |
| 3 重连 | [图](P13-jo/reconnect-failed-light-comparison.png) | [图](P13-jo/reconnect-failed-dark-comparison.png) | 检查失败由勾变警告 |
| 4 修复 | [图](P13-jo/repair-failed-light-comparison.png) | [图](P13-jo/repair-failed-dark-comparison.png) | 修复失败类型正确 |
| 5 安装 | [图](P13-jo/install-failed-light-comparison.png) | [图](P13-jo/install-failed-dark-comparison.png) | 安装失败不再庆祝 |
| 6 移除 | [图](P13-jo/remove-failed-light-comparison.png) | [图](P13-jo/remove-failed-dark-comparison.png) | 移除失败不再庆祝 |
| 7 归属 | [图](P13-jo/health-owner-manifest-light-comparison.png) | [图](P13-jo/health-owner-manifest-dark-comparison.png) | 新声明保持健康，旧异常块撤回 |
| 8 恢复 | [图](P13-jo/health-recovery-enabled-light-comparison.png) | [图](P13-jo/health-recovery-enabled-dark-comparison.png) | 故障面收回，运行可用 |

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 紫蓝渐变/纯黑/霓虹/渐变字 | 无新增，沿用中性主题 |
| A2 字体必须非系统 | 中文系统字体与项目合约优先，明确不换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 非Hero，不适用 |
| A5 三等分/万能卡片 | 新容器0个 |
| A6 假数据 | 无产品假指标；测试子进程、HTTP503和索引写失败均真实触发 |
| A7 空词/感叹号 | 产品i18n零改 |
| A8 眉批 | 新增0个 |
| A9 H1 | 既有单行标题保留 |
| A10 Bento空洞 | 非Bento，不适用 |
| A11 左侧竖线 | 无新增；原故障轮廓随恢复撤下 |
| B1 读取/拨盘 | 2/1/6，克制、即时、原密度 |
| B2 主题 | 深浅分别渲染，未混主题 |
| B3 记忆点 | 操作失败与工具恢复都有准确且一致的反馈 |
| B4 动效气质 | 高频状态修正新增动画0个 |
| C1 目的/频率 | 高频结果无额外动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原220ms浮层入场保留，按压规则不改 |
| C4 origin | 原toast位置不改，无新浮层 |
| C5 reduced-motion | 截图显式ReduceMotion=true，原降级机制保留 |
| C6 hover条件 | Go自绘，非CSS；指针路由不改 |
| C7 布局属性动画 | 无新增；故障区收回是事实改变 |
| C8 keyframe重播 | 无新增 |
| C9 手势/弹簧 | 本轮不涉及 |
| C10 Framer/Motion | 无此依赖，不适用 |
| C11 慢放 | 无新动效，用实际前后帧验证状态；未声称真机慢放 |
| D2-1 系统中文字体 | 保留系统字体，无webfont |
| D2-2 行高/字号 | 不宣称通过通用≥14px；原Caption9.9/正文10.8与行盒需拍板 |
| D2-3 混排/单位 | 产品文案零改，无新增不规范单位 |
| D2-4 标点/斜体 | 原全角文案保留，无斜体 |
| D2-5 字重 | 原580/620等值保留，不宣称满足通用400/500/600 |
| D2-6 暗色7:1 | 未改墨值；不宣称既有次要墨全面满足7:1 |

自发现缺陷沉淀到§6.242和事件测试，不把一次性修复写为全局审美规则。用户固定方向优先于四方案预览与装饰密度要求。

## 完整门槛与限制

完整门槛已完成：**5780 PASS / 0 FAIL**，基线5727（**+53**）；测试函数3328→3337（**+9**）。普通与verbose全量、三平台vet/build均退出0，gofmt空；定向race **53 PASS / 0 FAIL**，文档契约检查退出0。最终五文件MD5全成功。

首轮gofmt发现Reconnect调用行缩进未格式化，已修正并更新最终MD5；此前产品提交带此格式瑕疵，明确记录，不声称初次gofmt就过。正式mutation针对的逻辑没有变化，最终全量已覆盖格式化后的源码。

完整证据：[门槛](P13-jo/gates.txt)、[普通全量](P13-jo/full-test.txt)、[verbose全量](P13-jo/full-verbose.txt)、[race](P13-jo/race.txt)、[基线](P13-jo/baseline-zh.txt)、[文档契约](P13-jo/docs-check.txt)、[最终MD5](P13-jo/final-md5.txt)。旧测试文件没有修改，旧测试函数减少0。

20:52 UTC开盘，21:17 UTC完整门槛完成，实际约25分钟，短于45–75分钟目标。八项四批及时推送，收尾只归档、补齐进度和done.log；未以新增系统或无关修改填时间。

Linux真实子进程、HTTP错误、文件写失败、仓库刷新及ui.Tester软件自绘已验；Windows/macOS仍只编译门槛，未宣称真机。探测写回互斥只覆盖本进程探测写回，未宣称提供跨进程仓库事务。旧V/B/S断言修改0/0/0，新增Skip0。

## 仍待

1. 下一轮先事件复现：命令重探测失败后成功，reprobeCommands成功分支未更新已有DiagnosisFailed，是否会残留旧失败行。另核对仅磁盘刷新、Inventory未刷新时，宿主收到ErrProbeSuperseded后是否仍派发旧诊断失败通知。两条均为代码线索，尚未事件复现，不计本轮修复；不重复已修的严重度与持久健康归属。
2. 运行时绑定错误恢复归Reconnect负责；本轮健康恢复只解除probe_failed，没有扩大为自动解除包校验/安装故障。
3. 字号/行盒、accent禁用填充、未知选项、多草稿/转换覆盖、重复新包ID覆盖、配置模板重试、176-9/161-4保存期归属、终端S1b/S1c/S2尾沿用需拍板。确认/保存/终端会话语义不改。
4. 八条早期WIP按表保留，未发现新stale-kill。
