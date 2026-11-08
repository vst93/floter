# P13-jt · 重新探测命令的执行状态与写回归属

2026-11-07 23:33 UTC 开盘，HEAD 2afab0d9，pull 已同步、工作树干净。实际最新 P13-js，按其首条仍待推进。Codex 独立执行；已读取 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN。本轮只修既有入口和异步结果的归属。

## Stash 清点

下列 SHA 与已归档语义审计一致；逐条 reverse-check 均为 1。活动 stale-kill 已在历史轮接续归档，当前八条为早期独立 WIP，不重新覆盖现有 Go 主题或恢复 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/调试，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

done.log 开盘不存在，收尾按指定路径创建。基线运行中；以下逐批只记真实完成结果。

## 第一批：探测期间的行操作

1. 现象：真实点击重新探测，再点击一次并重复宿主派发，同一工具 epoch 达到 3。改动：复用既有 BeginDiagnosis/FinishDiagnosis 的行锁和主线程收尾；不再另建忙态系统。验证：深浅真实 Click、Go 子进程 --help 门控，epoch 必须为 1，完成后原入口可再次点击。
2. 现象：帮助程序尚未返回，“检查”和“健康刷新”各派发一次，竞争调用共 2。改动：重新探测加入现有整行互斥，沿用禁用面；完成自动释放。验证：深浅两套，等待中竞争派发为 0，完成后重试 1 次。

改前运行已红：`pending command reprobe starts repeatedly: epoch=3`、`pending command reprobe permits competing actions: 2`、`command reprobe never entered and settled its row busy state`。before 为原代码实际等待帧；旧 V/B/S 断言修改 0/0/0。

第一批 **10225ea5**，相关实测 **42 PASS / 0 FAIL**，shell/settingsui vet 净。提交正文误写为 44，正确数字为日志中的 42，此处明确更正，不沿用错误数字。第二批新夹具首次编译发现 bool 索引和不存在的 Commands 方法，修正后才作行为复现；编译错误不计 mutation。

## 第二批：当前声明与命令文件

3. 现象：真实 Click 后帮助子进程等待，manifest 已更新，旧输出仍覆盖命令并写入“新鲜”时间。改动：帮助返回后复核读取的声明，过期结果安静退出。验证：深浅事件，descriptor 字节不变、freshness 文件不生成、当前行无旧失败。
4. 现象：等待期间保存了 Current command 描述文件，旧帮助仍将其改回 Original command 并追加子命令。改动：写前逐字节核对 descriptor，当前编辑优先；shell 先刷新再发布诊断，过期结果不污染当前行。验证：两主题同样断言文件、freshness 与可见行，before 真实覆盖。

改前四场景全红：`late command reprobe overwrote current manifest descriptor`、`late command reprobe overwrote current descriptor descriptor`、`obsolete … probe wrote a freshness record`。旧 V/B/S 仍 0/0/0。

开盘全量实测 **5902 PASS / 0 FAIL / 3371 函数**，exit0。基线完整日志留存。

第二批 **7e6be57d**，相关 **22 PASS / 0 FAIL**，shell/extensions vet 净。首次目视浅色等待帧：底部重新探测、检查、编辑以及命令开关统一进入原禁用面，无额外浮层或说明占高。

## 第三批：目标来源刷新

5. 现象：帮助等待期间，同 ID 已改为发布者维护的包，旧探测仍写入其描述文件。改动：写回前复核当前 publisher/provider 与包来源，失效请求退出。验证：深浅真实事件，当前文件字节保留、无伪新鲜度、无过期失败提示。
6. 现象：等待中运行时已经换绑，旧程序的帮助仍追加到新目标的命令目录。改动：复核 ExecutablePath、RuntimeRoot 与包版本/路径后才接受输出。验证：深浅文件和 freshness 断言，当前行保持新运行时事实。

改前两类均实红：`late command reprobe overwrote current publisher descriptor`、`late command reprobe overwrote current runtime descriptor`；均错误写了 freshness。旧 V/B/S 仍 0/0/0。

第三批 **940ad2a6**，相关 **28 PASS / 0 FAIL**，shell/extensions vet 净。

## 第四批：读取失败的反馈与重试

7. 现象：开始时仓库 JSON 损坏，重新探测结束却没有任何失败通知。改动：保留实际读取错误，刷新后用既有失败通知收尾，释放行锁。验证：真实损坏文件、Click 与深浅可见错误文案，文件恢复后原入口重试成功。
8. 现象：帮助已开始、仓库随后损坏，操作也静默结束。改动：完成阶段区分读取错误与目标退役；不提交旧描述文件，当前 epoch 使用原通知反馈失败。验证：两主题通知类型/标题/实际文本，以及 descriptor 字节保留、freshness 不写、修复文件后的命令数恢复到 2。

改前四场景红：`command reprobe repository-start read failure is silent`、`command reprobe repository-completion read failure is silent`。旧 V/B/S 仍 0/0/0。

第四批新夹具首次回归暴露测试观察对象错误：生成命令默认未开启搜索，不能用启动器数组数目验证描述文件刷新。改为当前 Store 描述的准确两个命令（第二个 ID=fresh）以及设置页真实可见的 Original command fresh；未改任何旧 B 断言。保留首次失败日志并在修正后重跑变异。

函数数更正：按开盘树 `git grep '^func Test' 2afab0d9 -- cmd internal` 为 **3369**。前述 3371 是基线结束时工作树已新增两函数后的统计，不是开盘树；完整 PASS 基线 5902 不变。

第四批 **93c72555**，修正后相关 **64 PASS / 0 FAIL**，shell/extensions vet 净。四批产品均每完成两项通过相关回归和vet后提交推送。完整门槛与正式变异另列，未将定向测试冒称全量。

## 偷了什么

- craft-loop 工艺菜单 **#4 签名交互**：在飞请求 **1次**、重复与同行竞争派发 **0次**；额外反馈延迟 **0ms**。沿用已有禁用面。
- **#6 编辑细节**：旧探测不再把当前 **1条**命令变成 **2条**；用户新保存的 Current command 原位保留，新增常驻说明 **0条**。
- **#12 材质细节**：保留 **1px** 受光边与 **8px** 控件圆角，新增玻璃及阴影 **0层**。此项是继承约束，不冒充重绘成果。
- Apple DESIGN 供体取单一交互蓝角色、**8px** 圆角、黑色 **0.22** 浮层柔影，映射本仓现有 token；不照搬网页字号和模糊。

用户明确要求修已有功能，优先于 skill 通用“至少五种工艺”数量；没有添加装饰凑数。为什么本轮更轻、更有空间感：等待落在原行，失效结果不再扩展命令区，读取失败只占用既有通知浮层。

## 两轮图像复核

第一轮用原实现捕获事件帧。艺术总监看到底部活动面与实际忙态冲突；工程师看到同一请求可排队三次；任务用户看到 Current command 被改回旧名；读者看到失败后空目录却无解释。分别落成行锁、输入复核、文件保留与已有通知出口。

第二轮逐张核对深浅总览：浅色底部按钮和命令开关一起变暗；manifest/publisher/runtime 对照中的错误子命令退出后，原操作条回到可见区域；descriptor 对照保留 Current command；深色读取失败出现在右上原通知中。图是 Linux ui.Tester 960×1600 软件自绘，不是 Windows/macOS 真机。4× 最近邻局部只作细节检查，不冒称高DPI。

32张原始PNG与16张局部对照均入库。before按批次分别来自2afab0d9、10225ea5、7e6be57d、940ad2a6；after为对应修复后的真实事件，不冒称所有before来自同一版本。像素差在相应控件/列表/通知区域计算，时间文字不是成果依据。

| 项 | 浅色 | 深色 |
| --- | --- | --- |
| 1 重复探测 | [图](P13-jt/repeat-light-compare.png) | [图](P13-jt/repeat-dark-compare.png) |
| 2 同行竞争 | [图](P13-jt/competing-light-compare.png) | [图](P13-jt/competing-dark-compare.png) |
| 3 声明刷新 | [图](P13-jt/manifest-light-compare.png) | [图](P13-jt/manifest-dark-compare.png) |
| 4 命令文件刷新 | [图](P13-jt/descriptor-light-compare.png) | [图](P13-jt/descriptor-dark-compare.png) |
| 5 发布者变更 | [图](P13-jt/publisher-light-compare.png) | [图](P13-jt/publisher-dark-compare.png) |
| 6 运行时换绑 | [图](P13-jt/runtime-light-compare.png) | [图](P13-jt/runtime-dark-compare.png) |
| 7 开始读取失败 | [图](P13-jt/repository-start-light-compare.png) | [图](P13-jt/repository-start-dark-compare.png) |
| 8 完成读取失败 | [图](P13-jt/repository-completion-light-compare.png) | [图](P13-jt/repository-completion-dark-compare.png) |

[浅色总览](P13-jt/overview-light.png)、[深色总览](P13-jt/overview-dark.png)、[像素记录](P13-jt/image-differences.txt)。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿用近黑主题 |
| A2 非系统字体 | 中文系统字体与项目合约优先，不为网页规则换字库 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生功能页 |
| A5 卡片海/三等分 | 无新增容器 |
| A6 假数据 | 无产品假数据，Original/Current command为真实子进程夹具 |
| A7 空词/感叹号 | i18n零改 |
| A8 编号眉批 | 产品无新增，证据图编号不进产品 |
| A9 H1 | 既有设置页头完整可见 |
| A10 Bento | 不适用 |
| A11 左竖线 | 无新增 |
| B1 读取/拨盘 | 3/2/7，熟悉、即时、保持密度 |
| B2 主题锁定 | 每个Tester单独设深浅主题 |
| B3 记忆点 | 当前可执行状态与当前命令一致 |
| B4 动效气质 | 额外延迟0ms |
| C1 目的/频率 | 高频点击没有新增动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原过渡与按压反馈保留，未调时长 |
| C4 origin | 原行与原通知位置不变 |
| C5 reduced-motion | 新Tester均ReduceMotion=true |
| C6 hover媒体查询 | Go自绘，不适用CSS，指针路由不变 |
| C7 布局动画 | 无新增尺寸/间距动画 |
| C8 keyframes | 无新增 |
| C9 手势 | 不涉及 |
| C10 Framer | 不适用 |
| C11 慢放 | 无新增动效，未宣称慢放真机验收 |
| D2-1 中文字体 | 系统字体，无webfont |
| D2-2 字号/行高/字重 | 通用≥14px/1.5仍不满足；Caption9.9、正文10.8和行盒继续待拍板 |
| D2-3 混排空格 | 产品文案零改 |
| D2-4 标点/暗色对比 | 原墨与文案未改，不宣称旧次要墨全部过7:1 |

本轮问题沉淀为§6.247和事件pin，不修改全局skill偏好。用户指定已有方向优先，不做HTML预览或外部模型。

## 正式 mutation

| 组 | 注入 | 运行时原始报错 | 还原复验 |
| --- | --- | --- | --- |
| M1 | 撤掉重新探测入口的 BeginDiagnosis | `pending command reprobe starts repeatedly: epoch=3`、`pending command reprobe permits competing actions: 2` | 4文件MD5全OK、touch后6 PASS |
| M2 | 跳过 manifest/descriptor 复核 | `late command reprobe overwrote current manifest descriptor`、`late command reprobe overwrote current descriptor descriptor` | 4文件MD5全OK、touch后6 PASS |
| M3 | 跳过 publisher/runtime 来源复核 | `late command reprobe overwrote current publisher descriptor`、`late command reprobe overwrote current runtime descriptor` | 4文件MD5全OK、touch后6 PASS |
| M4 | 跳过当前读取失败通知 | `command reprobe repository-start read failure is silent`、`command reprobe repository-completion read failure is silent` | 4文件MD5全OK、touch后6 PASS |

四组注入exit1且有预期行为失败，不以编译错误冒充红；还原均exit0。修正后的重试夹具包含在M4复验中。证据见[mutation结果](P13-jt/mutations.json)、[源码MD5](P13-jt/final-source.md5)及同目录M1–M4原始日志。

## 仍待

1. 下一轮首选：生成工具的编辑/转换异步保存完成时，当前 owner、声明或描述文件已变化，是否仍覆盖当前内容。仅代码线索，先做真实事件复现；本轮没有改编辑/转换的保存时机或冲突策略。若触及既有“多草稿/转换覆盖”拍板边界，保留并换可直接修复的问题。
2. 重新探测的整段外部程序等待已复核；写前检查与磁盘 rename 之间仍不是跨进程事务，不宣称能阻止另一个进程恰在该瞬间写入。没有引入文件锁系统或新依赖。
3. 字号/行盒、accent禁用填充、未知选项、保存期间草稿、重复新包ID覆盖、终端S1b/S1c/S2尾等历史拍板项继续沿用。确认三秒、键盘、保存和会话语义不改。
4. Linux软件自绘与真实Go子进程已验；Windows/macOS仅编译，无真机证据。八条早期stash保留，无新增抢救稿。

## 完整门槛

gofmt输出为空、linux/windows/darwin vet与build均exit0。普通/verbose全量及race正在运行，最终只按落盘结果补录。旧V/B/S断言修改0/0/0，新增Skip0。

首次race在新忙态夹具报错：帮助子进程的真实3秒超时先到，较慢的插桩绘制继续点击已完成的行，出现`pending command reprobe permits competing actions: 1`。日志无DATA RACE报警；这不能当成busy语义正确的证据。夹具增加Store.OnChange完成门控，让刷新仍在飞直到真实UI事件完成，模拟真实窗口队列；产品超时、断言和旧测试不改。随后重新运行新夹具、M1–M4字节还原、race和完整门槛，首次日志保留。

## 2026-11-08 抢救接续记录

本次开盘实际 HEAD `93c72555`，不是任务书旧进度的 P13-hw。按仓库轮次档案规则继续 P13-jt；八项产品此前已分四批推送，本次不把它们冒称为八项新改动。最新抢救稿仍包含验收未完成的 race 夹具修正与证据，故先完成本轮闭环。

| 本次开盘 ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 79ad8d537b | reverse=1；独有三处 tracked 修改及截图/日志 | git apply -3 接回；门槛通过后独立提交并归档原件 |
| stash@{1} | 422483b4 | reverse=1；旧 keycap，已归档语义审计 | 保留 |
| stash@{2} | 8af94a07 | reverse=1；旧行距/slider，已归档语义审计 | 保留 |
| stash@{3} | 7e4aa710 | reverse=1；旧卸载度量/调试，已归档语义审计 | 保留 |
| stash@{4} | 1e3aec94 | reverse=1；旧数字字段，已归档语义审计 | 保留 |
| stash@{5} | 363f5a42 | reverse=1；旧文本槽，已归档语义审计 | 保留 |
| stash@{6} | 9b8ce23c | reverse=1；旧快捷键，已归档语义审计 | 保留 |
| stash@{7} | 90d314b2 | reverse=1；旧动作栏，已归档语义审计 | 保留 |
| stash@{8} | 7a9f6e13 | reverse=1；旧 main/Tauri，已归档语义审计 | 保留 |

接回的普通/verbose 全量日志停在 settings 包，race 日志停在读取失败场景；都缺最终退出码，**不能据此声称通过**。原中断 race 日志另存 `P13-jt/rescued-incomplete-race.txt`。重新执行的日志与结果将以最终落盘记录为准。

已目视复核深浅总览：等待期间原行底部操作与命令开关一起变暗；描述文件变化后 Current command 保留，过时的 fresh 子命令不再挤出底部操作；读取失败时右上出现既有错误浮层。空间改善来自撤下过时内容与复用反馈层，没有加新材质。预检字号与暗色次要墨的历史限制仍按上表保留。

接续实跑结果：M1–M4 均为预期行为失败（exit1），四文件 MD5 校验全部 OK，touch 后分别 **6/6/6/6 PASS**。gofmt 空，三平台 vet/build 全部 exit0。`-race -count=1 -v ./internal/shell -run '^TestCommandReprobe'` 最终 **24 PASS / 0 FAIL**、exit0、无 DATA RACE 报警，耗时 110.667s。`FLOTER_P13JT_DUMP=retry` 的正常事件重放 **24 PASS / 0 FAIL**、exit0，重新导出 16 张 960×1600 原图；未新增 Skip、未修改旧 V/B/S 断言。

接续的新图与抢救稿的旧图分开保存：[浅色总览](P13-jt/overview-light-retry.png)、[深色总览](P13-jt/overview-dark-retry.png)，各八项另有 `*-retry-compare.png` 对照。浅色读取失败图可见右上既有错误通知，原内容起点和标题未被挤动；深色当前命令图中多余 fresh 行消失，操作条回到可见区。日期/“几小时前”的自然变化不计作修复成果。普通和 verbose 全量尚在运行，最终数字等待实际退出码。

下一轮线索补充真实位置：`extensions/custom.go:150` 的 UpdateCustom 只在开始读取目标；`custom.go:168` 调用 writeCustomPackage，`shell/shell.go:3069` 从编辑回调进入。这里的 owner/声明/描述文件刷新仍需事件复现，不以代码形状直接计为 bug；多草稿/转换覆盖的既有拍板边界继续有效。

八项验收定位（每个测试都包含 light/dark 两个事件场景）：

| 项 | 回归测试 | 必须成立的结果 |
| --- | --- | --- |
| 1 重复探测 | TestCommandReprobeRejectsRepeatedClick | 在飞 epoch=1；完成后重试派发1次 |
| 2 同行互斥 | TestCommandReprobeLocksCompetingActions | 竞争调用0次；完成后原入口恢复 |
| 3 声明更新 | TestCommandReprobeKeepsCurrentManifest | 当前 descriptor 字节不变；无伪 freshness |
| 4 描述文件更新 | TestCommandReprobeKeepsCurrentDescriptor | Current command 保留；无过时诊断 |
| 5 发布者变化 | TestCommandReprobeKeepsCurrentPublisher | 当前发布者文件保留；无过时提示 |
| 6 运行时换绑 | TestCommandReprobeKeepsCurrentRuntime | 新绑定文件保留；无伪 freshness |
| 7 开始读取失败 | TestCommandReprobeRepositoryStartFailure | 既有错误通知可见；修复文件后命令数=2 |
| 8 完成读取失败 | TestCommandReprobeRepositoryCompletionFailure | 不提交旧结果；错误通知可见且能重试 |

抢救稿原对象保留于 `refs/archive/floter/P13-jt/79ad8d537b9e85bf40f59e075dd5926fd1dd2b60`。接续后再次 reverse-check 仍为1，原因是原报告的 EOF 已追加接续记录；`git diff 79ad8d537b -- docs/liquid-glass.md internal/shell/commandreprobebusy_test.go` 为空，报告差异仅追加内容。不能把这个 reverse-check 当作未接续或无损覆盖的证据；独立提交推送成功后再从活动 stash 清理，原始证据仍在归档引用中。

## 接续最终门槛（全部完成）

普通 `go test -count=1 ./...` 与 verbose 全量均 **exit0**；verbose **5926 PASS / 0 FAIL**，开盘基线 **5902 PASS**。测试函数按 Git 树统计 **3369 → 3377**，新增8个、未减少。普通模式不打印 `--- PASS`，其日志计数0不代表零测试；退出结果分别见 `P13-jt/full-results.json`。

gofmt输出为空，linux/windows/darwin vet与build全部exit0；race **24 PASS / 0 FAIL**，重新出图事件 **24 PASS / 0 FAIL**；M1–M4均实红并四文件MD5还原、touch后各6 PASS。原产品两文件与最新产品提交一致，本次接续只补回测试夹具与验收材料。V/B/S旧断言修改 **0/0/0**，新增Skip **0**，依赖与数据格式未改。

本轮八项产品、前后事件、深浅截图、四组变异、完整门槛均已闭环。跨中断继续沿用P13-jt，没有把抢救接续冒称为新一轮八项修复，也未为了凑时长重复改已完成的功能。

提交前附加检查：`git diff --cached --check` 只报告 baseline.txt / verbose-test.txt 中 Go flag 帮助原始输出的 space-before-tab，共22行；保留两份原始日志字节，不为消除提示改写证据。排除这两个原始日志后，源码、脚本、文档及其余材料的 whitespace 检查通过。产品 gofmt 门槛此前已为空。
