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

## 仍待

1. 下一轮先检查普通“检查”按钮的异步忙态与健康刷新互斥；此轮只补健康刷新发起的一侧，普通诊断自己的入口尚无完成回调。先复现，不改确认时限、保存或会话语义。
2. 历史健康报告在schema变更后仍保留的语义不变；字号/行盒、accent禁用填充、未知选项、保存期草稿归属、终端S1b/S1c/S2尾继续待拍板。
3. Linux ui.Tester与真实Go子进程有证据；Windows/macOS只做编译门槛，无真机操作证据。
