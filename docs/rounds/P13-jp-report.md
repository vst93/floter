# P13-jp · 检查结果的恢复、归属与可读边界

起点97e16695；规格已落盘，尚未把线索算作修复。当前基线在跑，未预填数字。每两项验收后提交推送；末尾补完整门槛。

## stash清点

当前无新stale-kill；八条SHA与已归档语义审计相同。reverse=1不证明独有成果，不重复合入已过时的主题、字段或Tauri发布链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载/scratch，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | main/Tauri，reverse=1 | 保留 |

## 增量记录

开盘已核对最新报告与.pending-bugs.md顶部，沿P13-jo实际待办推进。统一中文locale。测试函数与PASS数字待基线结束后填写。

1. 现象：重新探测已成功恢复3条命令，旧“没有可用帮助输出”仍在诊断行。改动：成功沿同一个epoch/来源检查清除旧诊断，并在工作线程完成刷新。验证：真实脚本先静默再恢复帮助输出，原串 `successful command retry retained old diagnosis` 深浅均红，修复后通过。
2. 现象：磁盘已有更新的健康报告，过期探测仍下结论并通知“报告了问题”。改动：ErrProbeSuperseded/ErrNoIntegration刷新当前事实后结束。验证：真实健康子进程门控制磁盘更新、不提前刷新Inventory；原串 `discarded lifecycle check published a diagnosis or notice: diagnosis="ok" failed=true` 深浅均红，修复后无旧诊断/通知且新时间可见。

第一批相关 **37 PASS / 0 FAIL**、shell vet净；gofmt已执行。旧V/B/S断言修改0/0/0。首次新增测试有未使用import，编译拒绝后已修正，该次不计有效红。截图首查发现命令卡长于1600px，正式mutation前将统一增高夹具以完整覆盖诊断区。正式mutation尚未执行，不预填结果。

第一批73291c3a已推送。

3. 现象：同路径切换CurrentVersion后，旧运行版本的失败仍显示在当前卡片。改动：诊断来源包含CurrentVersion。验证：真实提供者先报告旧故障，仓库版本刷新，已完成/在飞两种情况×深浅均由 `changed runtime version inherited stale diagnosis` 转绿，并确认新检查可显示。
4. 现象：RuntimeRoot刷新后同样沿用旧诊断。改动：诊断来源包含RuntimeRoot的值。验证：真实提供者子进程配合仓库运行根刷新，同样四场景由 `changed runtime root inherited stale diagnosis` 转绿；没有运行新命令或改变会话语义。

第二批相关 **41 PASS / 0 FAIL**、shell vet净，gofmt完成；旧V/B/S断言0/0/0。

第二批0871d6aa已推送。长汇总/长状态两个探针在开盘代码上已通过，撤回候选改动与测试，不计修复；规格以实际缺陷替换。开盘全量为5777 PASS / 3 FAIL（同一个既有TestHealthCompletionRecovery/disabled/dark及父节点），原串exit=4：夹具release文件被读到空内容，非产品断言被改弱；保留日志并复验。

5. 现象：提供者某项只有ID、状态而没有消息时，诊断行只画Error，看不出检查对象。改动：空白消息回退已有ID，非空消息原样。验证：空字符串与空格/Tab两种情况，深浅均由 `empty check message hides the failing check identity` 转绿，非空消息的ID不额外显示。
6. 现象：提供者整体ok但某检查失败时，页面总标题仍Ok，与通知矛盾。改动：已有失败标志成立时，用已记录的具体失败说明作为汇总；原始DiagnosisStatus不改。验证：真实提供者返回ok+account/error，`failed provider checks still display an OK summary` 深浅转绿，当前明细和失败标志保留。

第三批相关 **49 PASS / 0 FAIL**、两包vet净；gofmt完成，旧V/B/S断言0/0/0。正式mutation仍待最终代码。

第三批0ced391d已推送。

7. 现象：真实生命周期探测exit7，但提供者随后返回ok，诊断汇总丢掉unhealthy。改动：提供者成功不覆盖已有生命周期失败说明。验证：真实健康子进程+提供者协议，原串 `provider OK erased a real lifecycle failure: diagnosis="ok" failed=true status="ok"` 深浅均红后绿，健康失败与提供者成功明细同时保留。
8. 现象：诊断错误说明不在状态枚举中时呈灰色，视觉上与普通信息无差别。改动：已有DiagnosisFailed决定汇总Warning墨。验证：真实组件的错误文字像素由 `failed diagnosis summary has no warning ink` 转绿；成功及既有状态枚举旧pin不改。

第四批相关 **82 PASS / 0 FAIL**、两包vet净；八项均已实红后绿，旧V/B/S断言0/0/0。截图夹具统一960×2400，解决首查命令区截断问题；非新增产品布局。2930aeae正文及报告首稿误写99，本处按batch4-green.txt纠正为82，保留历史不重写。

第四批2930aeae已推送。产品八项完成，后续只补验收与证据。

## 正式mutation

四组逐次注入，每组恢复两个产品文件、六文件MD5全部OK、touch后无缓存重跑。前次局部截图夹具误传**ui.Theme导致编译失败、M1还原绿不完整；已修复后四组全部重做，下表只计正式成功轮。

| 组 | 撤回内容 | 实红节点 | MD5/touch还原PASS | 原始报错 |
| --- | --- | --- | --- | --- |
| M1 | 命令成功清旧错、过期健康出口 | 6 | 6 | `successful command retry retained old diagnosis`；`discarded lifecycle check published a diagnosis or notice` |
| M2 | CurrentVersion/RuntimeRoot归属 | 10 | 10 | `changed runtime version inherited stale diagnosis`；`changed runtime root inherited stale diagnosis` |
| M3 | 空消息ID、失败明细优先汇总 | 6 | 6 | `empty check message hides the failing check identity`；`failed provider checks still display an OK summary` |
| M4 | 生命周期失败说明、错误汇总Warning墨 | 6 | 6 | `provider OK erased a real lifecycle failure`；`failed diagnosis summary has no warning ink` |

M4准确合成墨：浅(156,31,24)、深(212,148,137)。深色Warning本身alpha0.82，须与真实背景合成；未加容差。像素扫描只取一次Image，避免逐像素重复渲染。

基线及第二次全量都复现上轮健康夹具的exit4：子进程看见.release创建，但WriteFile尚未写入数字。改为写完整.tmp再rename发布；产品断言、超时及预期值全部不改。这是测试握手修复，不计第九项产品成果。原始5777/3与5793/3日志保留，不冒称开盘全绿。首轮测试函数3337，最终3345（+8）。

## 偷了什么

实读qiaomu-design SKILL、偏好账本、craft-loop、preflight、chinese-typography及Apple DESIGN。用户已指定轻便液态玻璃，采用已有方向打磨；视觉变化幅度2、动效1、信息密度6。

- craft-loop **#4 签名交互**：过期检查通知从1条归0条，完成出口额外等待0ms，保留现有重试动作。
- **#6 编辑细节**：空白消息以1个已有ID补足对象；保留54px状态列及8u间隔。失败说明进入原汇总位置，额外说明面0个。
- **#12 材质细节**：保留1px受光边，新玻璃0层；属于约束继承，不计新增装饰。
- **#5 多层阴影**：原浮层2层影保留，卡片内部新增影0层；同样不冒称本轮改了阴影数值。
- Apple供体的交互专用accent、8px圆角、0.22柔影仅作角色参照；本仓token优先。未采用#11，不把文字ID冒称图形资产；不为凑满工艺密度新增无关装饰。

**为什么更轻、更有空间感**：成功和归属刷新时撤回旧故障行与无效浮层，让原卡片的空间只承载当前事实；错误用原有语义墨说清，不加解释卡片。

## 两轮图像复核

第一轮四视角：艺术总监看到“Ok+失败通知”的冲突；工程师发现1600px帧裁掉命令卡底部；使用者看不出空消息对应哪一项检查；读者看到灰色错误难以辨认。分别由汇总事实修正、2400px完整帧、ID回退、Warning墨解决。另发现局部组件夹具用了框架默认黄色，改为产品主题后重新导出，不将错误夹具当产品效果。

第二轮使用同一最终夹具，在mutation前后分别导图；前图只撤回对应缺陷，其余为最终代码。原图与差异裁剪对照进P13-jp目录，裁剪没有改绘产品内容。长文本候选不复现，相关实验图不作为修复证据。

| 项 | 浅色 | 深色 | 图上看到的变化 |
| --- | --- | --- | --- |
| 1 | [图](P13-jp/commands-recovered-light-comparison.png) | [图](P13-jp/commands-recovered-dark-comparison.png) | 3条命令恢复后，旧诊断错误行撤下 |
| 2 | [图](P13-jp/superseded-health-light-comparison.png) | [图](P13-jp/superseded-health-dark-comparison.png) | 无效失败toast和过期诊断消失 |
| 3 | [图](P13-jp/runtime-version-complete-light-comparison.png) | [图](P13-jp/runtime-version-complete-dark-comparison.png) | 当前版本不再挂旧失败明细 |
| 4 | [图](P13-jp/runtime-root-complete-light-comparison.png) | [图](P13-jp/runtime-root-complete-dark-comparison.png) | 新运行根不继承旧诊断块 |
| 5 | [图](P13-jp/empty-message-light-comparison.png) | [图](P13-jp/empty-message-dark-comparison.png) | 空白说明位置显示network.connection |
| 6 | [图](P13-jp/failed-checks-light-comparison.png) | [图](P13-jp/failed-checks-dark-comparison.png) | 总结由Ok变成需要重新登录 |
| 7 | [图](P13-jp/lifecycle-failure-light-comparison.png) | [图](P13-jp/lifecycle-failure-dark-comparison.png) | Unhealthy保留，配置正常仍作为独立明细 |
| 8 | [图](P13-jp/failure-ink-light-comparison.png) | [图](P13-jp/failure-ink-dark-comparison.png) | 错误说明由灰墨变为产品Warning墨 |

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，原中性主题保留 |
| A2 非系统字体 | 中文系统字体及项目合约优先，不换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 非Hero，不适用 |
| A5 三等分/万能卡片 | 新容器0个 |
| A6 假数据 | 产品无假指标；测试为真实子进程与仓库刷新 |
| A7 空词/感叹号 | 产品i18n零改 |
| A8 眉批 | 新增0个 |
| A9 H1行数 | 原标题保留 |
| A10 Bento空洞 | 非Bento，不适用 |
| A11 装饰竖线 | 无新增 |
| B1 读取/拨盘 | 2/1/6，原密度即时反馈 |
| B2 主题锁定 | 深浅分别渲染，无混搭 |
| B3 记忆点 | 当前工具只展示属于自己的、相互一致的诊断结果 |
| B4 动效气质 | 无新增动效 |
| C1 目的/频率 | 高频状态修正不加等待/动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原浮层入场与按压规则不改 |
| C4 origin | 无新浮层，原位置不改 |
| C5 reduced-motion | 截图显式ReduceMotion=true，原机制保留 |
| C6 hover条件 | Go自绘非CSS，原指针路由不改 |
| C7 布局动画 | 无新增；旧诊断撤下是内容事实变化 |
| C8 keyframes重播 | 无新增 |
| C9 手势 | 不涉及 |
| C10 Framer/Motion | 无此依赖，不适用 |
| C11 慢放 | 无新动效，前后实际帧验证状态；未声称真机慢放 |
| D2-1 系统字体 | 保留系统中文字体，无webfont |
| D2-2 行高/字号 | 不宣称通用≥14px通过；Caption9.9/正文10.8及行盒仍需拍板 |
| D2-3 混排/单位 | 产品文案零改，原始ID作为数据原样显示 |
| D2-4 标点/斜体 | 保留原全角文案，无斜体 |
| D2-5 字重 | 原580/620等不改，不宣称满足400/500/600通用规则 |
| D2-6 7:1 | 未重设调色板，不宣称次要墨全面过7:1 |

## 完整门槛与限制

普通/verbose最终全量运行中。三平台vet/build已净，夹具调整后三平台vet复验净，最终定向race **38 PASS / 0 FAIL**。最终全量数字只按完成日志填写。旧V/B/S断言修改0/0/0；旧测试夹具只改发布握手，原断言零改；新增Skip0。

最终普通全量第一进程在settingsui尚未结束时收到SIGTERM（exit143），未产生测试失败输出，也不算通过。保留full-test.txt；重跑采用仓库既有GOMAXPROCS=4的资源设置，普通和verbose均要求真实退出0，不隐藏错误。

Linux实际子进程、仓库刷新和ui.Tester软件自绘已验，Windows/macOS仅编译。读取/复现与夹具纠错超过40调用预算；此后停止扩项，仅收尾已完成八项的验收、归档和推送，过程偏差明确保留。

## 仍待

1. 下一轮先事件复现：诊断过程中仅磁盘声明刷新并删除全部probes时，ErrNoProbes路径是否继续使用旧Inventory的provider；以及生命周期和提供者同时失败时，汇总是否丢失其中一方原因。仅代码线索，未复现、不计本轮修复。
2. 字号/行盒、accent禁用填充、未知选项、176-9/161-4保存期归属、终端S1b/S1c/S2尾及其他既有拍板项沿用，未改语义。
3. 八条早期WIP按上表保留，没有新stale-kill。长汇总与长状态已通过原实现，不留伪缺陷。
