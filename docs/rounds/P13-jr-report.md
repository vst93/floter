# P13-jr · 健康刷新入口、执行反馈与命令恢复

开盘 2026-11-07 22:31 UTC，HEAD ecc53976，pull 已同步，工作区空。执行者 Codex；已读 qiaomu-design、偏好账本、craft-loop、preflight、中文排版与 Apple 供体。没有使用外部模型。

## stash 清点

八条均与前轮已审计的早期 WIP SHA 相同，reverse-check 均为1；无新 stale-kill。独有内容的既有语义审计继续有效，不把旧主题/旧 Tauri 链重新覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap WIP，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/slider WIP，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载度量/调试 WIP，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧 number-field WIP，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽 WIP，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验 WIP，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器 action bar WIP，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri WIP，reverse=1 | 保留 |

当前基线测试运行中；报告只在实际结果落盘后记数。

## 第一批：当前声明

1. 现象：probes 全部移除，历史报告旁仍可点击刷新。改动：由当前声明投影不可刷新状态，撤下入口但保留历史报告。验证：真实 manifest 写入、Store 刷新、Tester 点击；失效时调用不增加，恢复后再次可点。
2. 现象：manifest 损坏或身份换成另一工具，旧报告仍提供刷新。改动：读取错误与 ID 不匹配同样撤下入口。验证：两类真实磁盘变更各覆盖深浅色，修复声明后恢复调用；不改原报告和既有 B pin。

第一批开盘实红：`current declaration cannot run probes but still offers health refresh`、`stale health refresh still dispatches a request`，三个条件深浅均复现。before 为修改前真实事件帧；after 使用相同夹具。

第一批相关 **44 PASS / 0 FAIL**，shell/settingsui vet净，diff --check净。原V/B/S断言0/0/0。开盘全量仍在运行，未拿局部数字冒充完整门槛。

第一批 9ea9f128 已提交推送，退出0。开盘完整基线 **5840 PASS / 0 FAIL / 3353函数**，退出0。

## 第二批：执行中的操作出口

3. 现象：健康刷新执行中重复点击会启动多份进程。改动：shell 在启动前进入健康忙态，所有结束路径在主线程退出忙态。验证：真实点击与宿主重复请求均须只接受一次，结束后可再点，按钮尺寸不变。
4. 现象：健康刷新期间同行检查/重连/停用仍可并行写同一记录。改动：健康忙态接入既有 rowBusy 与 rowBusyExceptRepair。验证：健康刷新期间点击检查、重新检查不得调用；另一工具可执行；结束后原出口恢复。

第二批相关 **20 PASS / 0 FAIL**，两包vet净。M2撤回两处行锁后，重复启动和竞争操作断言实红；原文件通过MD5逐字节恢复并touch复验。before 图为该组缺陷回注，非开盘整仓。

第二批 e88bebd4 已提交推送，M2还原后 **6 PASS / 0 FAIL**。

## 第三批：失败反馈

5. 现象：健康刷新无法绑定运行时，仅写日志，点击没有明确结束反馈。改动：沿用既有失败通知，先刷新绑定故障再显示；保留原健康报告。验证：真实缺失 executable、错误 toast 类型和文案、原报告时间、故障码及修复后重试。
6. 现象：仓库文件损坏时刷新只写日志，界面没有本次操作的失败反馈。改动：读取错误也进入已有错误通知；失效/被替代的请求不报旧错误。验证：实际写入损坏 JSON，断言错误反馈与忙态释放，恢复文件后健康检查成功且无新增错误通知。

第三批修改前深浅事件实红：`health refresh binding failure is silent` 与 `health refresh repository failure is silent`。原错误不是成功消息，而是完全没有消息；此处接活现有 error toast。

第三批相关 **50 PASS / 0 FAIL**、两包vet净，cd38337c 已提交推送。

## 第四批：报告与启动器同步

7. 现象：健康刷新将工具标成 broken，启动器仍保留旧的运行命令。改动：完成时走既有 RefreshIntegrations，同步目录、运行时事实与启动器。验证：真实必选 probe 退出7后，命令数组与搜索结果同时撤下。
8. 现象：再次健康检查成功、工具已恢复 enabled，启动器仍缺少命令。改动：同一次刷新同步恢复可用命令。验证：真实失败→成功进程，命令重新出现在搜索结果；disabled 工具成功后仍不进入启动器。

第四批首次回归失败（53 PASS）：命令夹具未打开既有命令开关，而本仓命令默认不进入启动器，因此不能把这次红当产品缺陷证据。描述符和绑定均正常；修正夹具开关后重新做旧实现注入红→恢复绿。尚未提交，不把预期计数当实测。

修正后M4实红：`health refresh leaves launcher commands stale: got=1 want=0` 与 `got=0 want=1`；两外观的搜索结果同时红。MD5全OK、touch后 **10 PASS / 0 FAIL**，disabled反例始终保持不可用。

第四批最终相关 **60 PASS / 0 FAIL**、两包vet净，**159aa4f3** 已提交推送。四批均每两项提交；三平台vet/build、gofmt随后已完成且全绿，普通/verbose完整测试与异步入口race待退出后记数。

## 正式 mutation

| 组 | 回注缺陷 | 运行时原始报错 | 还原复验 |
| --- | --- | --- | --- |
| M1 | 撤回当前声明门槛 | `current declaration cannot run probes but still offers health refresh` | 四文件MD5全OK，touch后10 PASS |
| M2 | 撤回健康忙态的两处行锁 | `health refresh starts twice while pending: 2`；`health refresh permits competing row actions: repairs=1 checks=0` | 当时产品文件MD5 OK，touch后6 PASS |
| M3 | 撤回错误通知 | `health refresh binding failure is silent`；`health refresh repository failure is silent` | 四文件MD5全OK，touch后6 PASS |
| M4 | 完整刷新改回只刷新Store | `health refresh leaves launcher commands stale`，失败1→0与恢复0→1 | 四文件MD5全OK，touch后10 PASS |

M1时夹具尚未补命令开关，M2是第二批源码；各日志只证明其当时验证的场景，不冒充同一文件哈希。最终源码哈希单列 final-source.md5。M3使用真正的Settings动作异步回调，以既有Invalidate事件确认进入与结束，无sleep猜完成时刻。所有before图均注明来源：声明的三种图为第一批改前事件，忙态/通知/命令为缺陷回注。

## 偷了什么

- craft-loop **#4 签名交互**：额外延时 **0ms**，等待中的重复请求 **0次**；同一行竞争操作锁住，另一行仍可用。
- **#6 编辑细节**：失效刷新入口 **1→0**，历史报告仍保留 **1份**；旧命令不会继续占结果行，成功恢复可再次搜索。
- **#12 材质细节**：沿用现有 **1px** 受光边；禁用图标仅由原组件处理，新增玻璃层 **0**。
- Apple DESIGN 的交互accent角色、按钮 **8px** 圆角、柔影 **0.22** 提取为角色与层级；以本仓token为准，不抄网页字号。后两项是约束继承，不冒称新增材质成果，不为凑五项添装饰。

**为什么更轻、更有空间感**：撤下不能执行的入口与不能运行的结果，恢复原有行内留白；错误只进入已有反馈层，不添加说明面板或第二层玻璃。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹 | 无新增，沿用主题 |
| A2 字体不同于系统默认 | 本仓中文系统字体规则优先，未换字体 |
| A3 斜体 | 无新增 |
| A4 Hero构图 | 不适用，原生设置与启动器 |
| A5 三等分/卡片海 | 无新增容器 |
| A6 假数据 | 无产品假数据；测试使用真实进程与临时仓库 |
| A7 空词/感叹号 | i18n文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 沿用既有页头 |
| A10 Bento | 不适用 |
| A11 竖线装饰 | 无新增 |
| B1 读取与拨盘 | 3/2/7，保留高密度功能页，反馈即时 |
| B2 主题 | Tester逐张固定深/浅 |
| B3 记忆点 | 当前可执行性与入口、结果一致 |
| B4 动效气质 | 新增状态转换额外延时0ms |
| C1 目的/频率 | 高频重试不添加动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 沿用120ms颜色过渡、图标0.92按压 |
| C4 origin | 沿用原反馈位置 |
| C5 reduced-motion | 全部新增Tester使用ReduceMotion=true |
| C6 hover媒体查询 | Go自绘，不适用CSS媒体查询，原指针路径保留 |
| C7 布局动画 | 无新增 |
| C8 keyframes重播 | 无新增 |
| C9 手势 | 不涉及 |
| C10 Framer | 不适用 |
| C11 慢放 | 无新增动效，未声称慢放真机验证 |
| D2-1 字体 | 沿用系统中文字体，不加webfont |
| D2-2 字号行高字重 | 通用≥14px/1.5未满足，旧字号与行盒仍需拍板；本轮不动 |
| D2-3 混排 | 产品文案零改 |
| D2-4 标点与对比 | 原墨/标点保留，不宣称次要墨全过7:1 |

## 两轮图像复核

第一轮四视角：艺术总监看到无效刷新占据报告行尾；工程师看到刷新期间仍能启动另一写操作；使用者点击失败时没有结束反馈；读者看到报告已恢复而搜索仍空。四处分别以当前声明门槛、行级锁、已有错误通知、目录同步修复。

第二轮看原像素及深浅对照：最初总览把整张绑定失败页缩得过小、窄裁剪的BEFORE/AFTER标签重叠；将该项聚焦到通知区域、对照列最小宽300px，重生成并看图。只改证据排版，产品截图未改绘。

| 项 | 浅色 | 深色 | 图上看到的变化 |
| --- | --- | --- | --- |
| 1 移除声明 | [图](P13-jr/removed-light-compare.png) | [图](P13-jr/removed-dark-compare.png) | 报告仍在，行尾刷新消失 |
| 2 无效声明 | [图](P13-jr/invalid-light-compare.png) | [图](P13-jr/invalid-dark-compare.png) | unexpected EOF保留，失效刷新撤下；ID替换另有原图 |
| 3 重复请求 | [图](P13-jr/repeat-light-compare.png) | [图](P13-jr/repeat-dark-compare.png) | 原位置的刷新图标变暗，位置和尺寸不动 |
| 4 同行竞争 | [图](P13-jr/competing-light-compare.png) | [图](P13-jr/competing-dark-compare.png) | 原来误启动的Re-check转圈不再出现，仍是禁用的刷新图标 |
| 5 绑定失败 | [图](P13-jr/binding-light-compare.png) | [图](P13-jr/binding-dark-compare.png) | 顶部出现当前错误通知 |
| 6 仓库失败 | [图](P13-jr/repository-light-compare.png) | [图](P13-jr/repository-dark-compare.png) | 空库存页也能看到本次失败 |
| 7 失败撤回 | [图](P13-jr/command-failure-light-compare.png) | [图](P13-jr/command-failure-dark-compare.png) | Health Ping旧行和运行栏撤下 |
| 8 恢复命令 | [图](P13-jr/command-recovery-light-compare.png) | [图](P13-jr/command-recovery-dark-compare.png) | 空搜索恢复Health Ping结果与运行栏 |

18对深浅原图共36张，18张局部并排图与2张总览；全部有可见变化，最小59像素。绑定失败的原始临时路径和进程时长可能不同，因此局部证据聚焦通知，断言验证错误类型和原报告时间，不靠路径差异算成果。[浅色总览](P13-jr/overview-light.png)、[深色总览](P13-jr/overview-dark.png)。

## 仍待（下一轮）

1. 下一轮先检查普通“检查”按钮的异步忙态与健康刷新互斥；此轮只补健康刷新发起的一侧，普通诊断自己的入口尚无完成回调。先复现，不改确认时限、保存或会话语义。
2. 历史健康报告在schema变更后仍保留的语义不变；字号/行盒、accent禁用填充、未知选项、保存期草稿归属、终端S1b/S1c/S2尾继续待拍板。
3. Linux ui.Tester与真实Go子进程有证据；Windows/macOS只做编译门槛，无真机操作证据。

## 完整门槛第一次结果

verbose全量 **5872 PASS / 0 FAIL / 3361函数**（基线5840/3353，+32 PASS、+8函数），退出0；三平台vet/build、gofmt全部通过。普通全量退出1，唯一失败是旧 `TestARescanAsksForAFreshDetectedList`：`the frame after a rescan started no pass`。它在本轮新健康测试之前失败，同批verbose中通过；原测试与产品扫描实现均未改。已保存首次失败原始日志，检查该测试的异步调度断言后单独重跑普通全量；不放宽、不加sleep、不修改旧B断言。

异步健康入口race **6 PASS / 0 FAIL**，文档契约检查通过。达到40调用预算后没有扩项，仅等待已启动门槛、处理这次门槛失败和收尾归档；这一预算偏差保留，不把失败日志抹掉。

扫描测试原断言在 `detectedtools_test.go:131-136` 启动goroutine后立即要求 `pending=true`；若扫描先结束，这个瞬时断言可失败，并不等于没启动。此处是可定位的调度竞争，不凭一次日志宣称已经修复。原测试单独 `-count=20` 全通过（退出0），原普通全量正在独立重跑。没有改产品扫描逻辑或旧B断言，没有用延时掩盖竞争；后续验收风险记入.pending-bugs。

## 最终门槛与交接

最终产品 **159aa4f3**。完整verbose **5872 PASS / 0 FAIL**，测试函数 **3361**（基线5840/3353，+32/+8）；普通全量独立重跑退出 **0**，verbose退出 **0**。gofmt空，linux/windows/darwin vet/build全过；相关60 PASS，异步入口race6 PASS，文档契约检查通过。原测试函数未减少，旧V/B/S断言修改 **0/0/0**，新增Skip **0**。最终四文件MD5全部一致。

四批产品每两项提交并推送：9ea9f128、e88bebd4、cd38337c、159aa4f3。第一至四批定向分别44/20/50/60 PASS；全部通过相关vet后提交。最终门槛保留首次普通失败exit1与最终普通exit0两条记录，不用后者抹掉前者。

证据：[门槛](P13-jr/gates.txt)、[最终普通全量](P13-jr/full-test.txt)、[完整verbose](P13-jr/verbose-test.txt)、[首次普通失败](P13-jr/full-test-first-failure.txt)、[扫描20次复验](P13-jr/rescan-recheck.txt)、[race](P13-jr/race.txt)、[最终MD5](P13-jr/final-source.md5)。四组mutation的原始红/恢复日志同目录；声明改前实帧、其余缺陷回注实帧和最终对照均保留。

22:31 UTC开盘，22:59 UTC普通重跑结束，约28分钟，低于45–75分钟目标。工具调用超过40的部分用于既定完整门槛等待、首次失败核查及交接；未扩充产品选题。下一轮从普通诊断发起的一侧查异步忙态和与健康刷新的互斥；终端、保存时机及历史拍板项不获得额外授权。
