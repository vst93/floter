# P13-jq · 诊断过程的当前事实与完整失败说明

本轮完成八项修复，跨健康面、诊断摘要与通知组件；四批产品 dd96d76d / 9506c8b5 / 71e369e6 / 978a8fc8 与去重补修 fce187ba 均已推送。最终 **5840 PASS / 0 FAIL**，比开盘增加 32；三平台 vet/build、普通/verbose 全量、gofmt、MD5 和补充 race 全过。

开盘 2a8b1e1c，工作区干净、ff-only 已同步。沿最新 P13-jp 的实际遗留继续，P13-hw 线索已由后续轮次覆盖。规格先落盘，下文增量记录保留从候选、实红到落地的过程。

## stash 清点

当前没有新 stale-kill；以下 SHA 与既有语义审计一致。reverse=1 不能说明独有工作，不把旧主题或 Tauri 链重复接回。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap；reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider；reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载/scratch；reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 数字字段；reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 插件字段；reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 快捷键；reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 动作栏；reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | main/Tauri；reverse=1 | 保留 |

## 增量记录

基线运行中。已读 qiaomu-design SKILL、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN。仅由当前 Codex 执行，沿既有方向打磨。

1. 现象：磁盘移除 lifecycle probes 后，ErrNoProbes 仍执行旧 provider。改动：这一返回值与已失效目标共同刷新后结束。验证：真实提供者标记文件出现，原串 `removed lifecycle declaration still launches the stale provider` 深浅均红；恢复后要求未启动、无旧通知、当前声明已刷新。
2. 现象：没有 lifecycle probes 时，provider 在飞期间磁盘换声明，旧失败结果和通知仍发布，后续检查也读旧来源。改动：完成出口先刷新库存，再按现有来源/epoch 校验发布。验证：真实提供者门控，原串 `provider completed against stale disk owner and published old facts`、`fresh diagnosis cannot recover after disk owner changes` 深浅均红；恢复后还验证新来源能正常检查。

首次红图夹具在输出前重复 Open 会清通知；已取消重复 Open，正式 mutation 使用修正后的同一夹具重出前后图，不将旧夹具缺失的 toast 当证据。

第一批相关回归 **53 PASS / 0 FAIL**，shell vet 净、gofmt 完成。原 V/B/S 断言修改 0/0/0；两项六个节点实红后绿，正式 MD5 mutation 待轮末统一执行。完整基线仍运行中，未宣称全量通过。

第一批 dd96d76d 已推送。

3. 现象：lifecycle 已失败并写盘，provider 尚在等待时卡片仍显示 healthy、没有失败探针。改动：已完成报告在第二阶段开始前刷新进库存。验证：真实 probe exit7 后 provider 门控，原串 `completed lifecycle failure stays invisible while provider waits: health="healthy" broken=false` 深浅均红；恢复后同一帧显示 unhealthy、broken 和 exit7。
4. 现象：lifecycle 失败后 provider 协议损坏，摘要仅剩 JSON 错误。改动：协议失败追加到已有失败原因，不覆盖。验证：真实子进程输出错误类型 JSON，原串 `provider protocol failure erased lifecycle reason` 深浅均红；恢复后摘要完整显示 `Unhealthy; extensions: invalid diagnose answer...`。

开盘基线完成：**5808 PASS / 0 FAIL / 3345 测试函数**。第二批相关 **59 PASS / 0 FAIL**、shell vet 净；旧 V/B/S 断言 0/0/0。六个节点实红后绿。

第二批 9506c8b5 已推送。

5. 现象：两个阶段同时返回结构化失败，lifecycle 的 unhealthy 丢失，界面只显示 provider 的 Error。改动：合并已有摘要，并投影“有前阶段失败”这一事实使原摘要位置显示两阶段原因；独立 provider 的状态标题不改。验证：`provider findings erased lifecycle reason` 与 `combined failure summary hides one stage behind provider status` 深浅均红；恢复后保留原 status/error、明细和完整摘要。
6. 现象：顶层成功词为带空格的 ok 或大写 OK 时，状态墨能识别成功，失败标志和通知却按失败处理。改动：按既有 UI 分类方式 trim/equal-fold，仅归类、保留原值。验证：真实提供者四场景的 `successful provider status is painted/notified as failure` 实红；通知队列在首次帧移交 toastViewport，因此通知断言在首次 Tester 帧前读取，画面仍在帧后验。

第三批两包相关 **81 PASS / 0 FAIL**、两包 vet 净，旧 V/B/S 断言 0/0/0。独立 provider 原 status 标题的旧 pin 全部保留。

第三批 71e369e6 已推送。

7. 现象：明细中带空格/大写的成功状态以成功墨显示，却把整个检查变成失败并发错误通知。改动：明细判定也按既有状态墨的 trim/equal-fold 规则归类，原 status/message 不变。验证：真实 provider 四场景的 `successful individual check turns overall diagnosis into a failure` 实红；恢复后成功汇总/通知与明细一致。
8. 现象：失败项消息只有空格和 Tab，明细能回退 ID，摘要却显示带警告墨的 Ok。改动：摘要同样将空白消息回退 ID。验证：`blank failure message leaves an OK summary instead of check identity` 深浅实红；恢复后摘要 Network.connection、明细 network.connection，原消息保留。

第一批通知队列断言也统一移到 Frame 前，防止队列交付后断言无意义；正式 mutation 重新验证，不修改旧 B 断言。

第四批相关 **89 PASS / 0 FAIL**、两包 vet 净，八项均实红后绿。原 V/B/S 断言 0/0/0、新增 Skip 0。后续只做完整门槛、正式 mutation 与图像复核，不扩项。

第四批 978a8fc8 已推送。

## 正式 mutation

| 组 | 注入 | 实红节点 | 六文件 MD5 / touch 后 PASS | 原始报错 |
| --- | --- | --- | --- | --- |
| M1 | 恢复 ErrNoProbes 继续与完成不刷新 | 6 | 全 OK / 6 | `removed lifecycle declaration still launches the stale provider`；`provider completed against stale disk owner and published old facts` |
| M2 | 撤回报告提前刷新与协议失败合并 | 6 | 全 OK / 6 | `completed lifecycle failure stays invisible while provider waits: health="healthy" broken=false`；`provider protocol failure erased lifecycle reason` |
| M3 | 恢复结构化覆盖、旧摘要优先与顶层精确匹配 | 8 | 全 OK / 8 | `provider findings erased lifecycle reason`；`successful provider status is painted/notified as failure` |
| M4 | 恢复明细精确匹配与仅空字符串回退 | 8 | 全 OK / 8 | `successful individual check turns overall diagnosis into a failure`；`blank failure message leaves an OK summary instead of check identity` |

每组都是运行时断言失败，非编译失败；还原后源码 git diff 为空。图由同一个最终夹具在注入前后生成；before 为该组缺陷回注，其余修复仍保留，不冒称是开盘整仓截图。完整门槛开始运行，数字待真实退出后追加。

## 偷了什么

- craft-loop **#4 签名交互**：过期结果与旧失败通知由 1→0；lifecycle 同步额外延时 0ms，已有 provider 等待不遮蔽已完成的事实。
- **#6 编辑细节**：两阶段失败合在原有 1 条摘要中，空白摘要回退 1 个既有 ID；54px 状态列、8u 间距不变。
- **#12 材质细节**：继承原 1px 受光边，新玻璃 0 层；这是约束继承，不冒称新增材质。
- **#5 多层阴影**：保留既有浮层 2 层阴影，卡内新增影 0 层；同样不计新增装饰成果。
- Apple DESIGN 的交互专用 accent、8px 按钮圆角、0.22 柔影只提取角色与层级关系，本仓 token 优先。不为工艺数量凑满五项添加装饰。

**为什么更轻、更有空间感**：归属失效时撤下旧诊断和警告浮层，使既有卡片的留白恢复；真实错误在原摘要和健康面中说清，不叠新的解释卡片。

## 两轮图像复核

第一轮四视角：艺术总监看到过期 toast 抢注意力；工程师看到已落盘的 unhealthy 与界面 healthy 矛盾；使用者只看到 Error 无法知道两个阶段都失败；读者看到空白错误的摘要仍为 Ok。分别由来源撤回、阶段同步、合并摘要、ID 回退解决。

第二轮审视同一最终夹具的 mutation 前后帧：全页 960×2400 原图保留，第一次总览高约 5000px，缩图读不清；补出 toast/诊断局部并排图，再以原像素和 2× 最近邻详情核对。裁剪只用于证据阅读，没有改绘产品。2× 图不冒充高 DPI 真机渲染。

| 项 | 浅色局部 | 深色局部 | 图上实际变化 |
| --- | --- | --- | --- |
| 1 | [图](P13-jq/removed-probes-light-detail.png) | [图](P13-jq/removed-probes-dark-detail.png) | 过期失败 toast 与诊断块撤下 |
| 2 | [图](P13-jq/disk-owner-light-detail.png) | [图](P13-jq/disk-owner-dark-detail.png) | 旧声明的失败结果不再占当前卡片 |
| 3 | [图](P13-jq/live-health-light-detail.png) | [图](P13-jq/live-health-dark-detail.png) | provider 等待中健康面即时出现失败 probe 与 exit7 |
| 4 | [图](P13-jq/protocol-failure-light-detail.png) | [图](P13-jq/protocol-failure-dark-detail.png) | 协议错误前保留 Unhealthy |
| 5 | [图](P13-jq/combined-failures-light-detail.png) | [图](P13-jq/combined-failures-dark-detail.png) | Error 摘要改为 Unhealthy 与需要重新登录 |
| 6 | [图](P13-jq/summary-spelling-ok-light-detail.png) | [图](P13-jq/summary-spelling-ok-dark-detail.png) | 成功摘要恢复正常墨，toast 改为成功勾 |
| 7 | [图](P13-jq/check-spelling-ok-light-detail.png) | [图](P13-jq/check-spelling-ok-dark-detail.png) | 单项成功不再触发报告问题，摘要恢复 Ok |
| 8 | [图](P13-jq/whitespace-failure-light-detail.png) | [图](P13-jq/whitespace-failure-dark-detail.png) | 摘要由 Ok 变为 Network.connection |

40 张原始深浅前后图、20 张完整差异裁剪和 20 张局部对照入仓；大小写两种成功输入各有单独图。20 对全部有变化，最小 448 像素；具体数值见 image-differences.txt。时长/检查时间也是实际进程产生的数据，不为截图固定假值。[浅色总览](P13-jq/overview-light.png)、[深色总览](P13-jq/overview-dark.png)。

## 合并原因的补充边界（属于第 4/5 项）

复核发现新合并逻辑会重复同一运行时错误，真实缺失 executable 的两阶段报错实红：`shared runtime failure is repeated in the summary`。相同的 unhealthy 也实红：`identical stage verdicts are duplicated: "unhealthy; unhealthy"`。改为仅合并不同原因；同文原因显示一次，原始 provider status 仍保留。新增场景扩入原两条测试函数，不算第九项产品成果。完整门槛已启动但尚未完成；最终须对补修后的源码验收，早期门槛日志不冒充最终版。

补修相关 **93 PASS / 0 FAIL**、两包 vet 净，原断言零改。正在补跑去重逻辑的独立 mutation；初轮四组 mutation 对应 initial-source.md5，最终补修使用 final-source.md5，两个版本不混称。

补修 fce187ba 已推送；M5 撤回两个相同原因判断，**6 节点实红**，六文件 MD5 全 OK、touch 后 **6 PASS / 0 FAIL**。此前异步事件 race **28 PASS / 0 FAIL**；最终补修再跑 shared 场景的 race，不将旧源码验证冒充最终验证。初轮普通全量运行中时做了补修，因此额外跑 final-full-test 和三平台 vet/build；初轮脚本的后续 verbose 在全部源码还原后才继续，作为最终 verbose。

去重补充另有 8 张原图与 4 对局部图：[同一运行时错误浅](P13-jq/shared-runtime-light-detail.png)、[深](P13-jq/shared-runtime-dark-detail.png)，[同一状态浅](P13-jq/shared-word-light-detail.png)、[深](P13-jq/shared-word-dark-detail.png)。属于第 4/5 项的补验，不计额外产品项。临时目录路径是每次子进程夹具真实生成，前后路径数字不同；原因是否重复由精确字符串断言守住，不靠像素计数判断。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，保留中性主题 |
| A2 非系统字体 | 中文系统字体及现有合约优先，不换装 |
| A3 斜体 | 无新增 |
| A4 Hero | 原生状态面，不适用 |
| A5 三等分/万能卡片 | 新增容器 0 个 |
| A6 假数据 | 产品无假指标；事件用真实子进程和临时仓库 |
| A7 空词/感叹号 | i18n 文案零改 |
| A8 眉批 | 新增 0 个 |
| A9 H1 行数 | 原标题完整可见 |
| A10 Bento 空洞 | 非 Bento，不适用 |
| A11 装饰竖线 | 无新增，既有 broken 警告面不改 |
| B1 读取/拨盘 | 2/1/6：原密度、即时状态、克制变化 |
| B2 主题锁定 | 深浅分开渲染，无混主题 |
| B3 记忆点 | 当前工具的两阶段检查事实完整且一致 |
| B4 动效气质 | 高频状态同步 0ms 额外延时，无新增动效 |
| C1 目的/频率 | 不为键盘或高频状态更新添加动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 既有反馈动效与按压规则不改 |
| C4 origin | 原 toast 位置不改，无新浮层 |
| C5 reduced-motion | 截图显式 ReduceMotion=true，原机制保留 |
| C6 hover 条件 | Go 自绘非 CSS；原指针路由不改 |
| C7 布局动画 | 无新增，旧诊断撤下是内容事实变化 |
| C8 keyframes 重播 | 无新增 |
| C9 手势 | 不涉及 |
| C10 Framer/Motion | 没有此依赖，不适用 |
| C11 慢放 | 无新动效；只声称真实状态帧验证，未声称真机慢放 |
| D2-1 系统字体 | 保留系统中文字体，无 webfont |
| D2-2 行高/字号 | 通用 ≥14px/1.5 未满足；Caption9.9/正文10.8及行盒仍需拍板 |
| D2-3 混排/单位 | 原 i18n 零改，错误/ID作为提供者原始数据显示 |
| D2-4 标点/对比 | 中文文案原标点不改，不宣称旧次要墨全部过 7:1 |
| D2 字重 | 沿用原 580/620 等，不宣称满足通用 400/500/600 |

用户指定既有方向、Go 自绘和不改字号的限制优先于四方案 HTML、浏览器 viewport 和通用字号门。可复现问题入本仓 §6.244 与事件 pin，不把一次性修复改成全局 skill 审美规范。

## 仍待

1. 下一轮先事件复现健康报告自己的刷新按钮：probe 声明全部移除却仍有旧报告、绑定失败或仓库读取失败时，是否留下无效重试入口或只写日志而没有当前反馈。仅代码线索，不算已确认问题；不重复本轮诊断链八项。
2. 既有字号/行盒、accent 禁用填充、未知选项、保存期草稿归属、重复包 ID/模板重试、终端 S1b/S1c/S2 尾继续待拍板，未改语义。
3. Linux 真实子进程、磁盘刷新、ui.Tester 有证据；Windows/macOS 仅 vet/build，真机交互未验证。八条旧 WIP 保留，没有新 stash。

## 完整门槛与交接

最终产品 **fce187ba**。基线 **5808 PASS / 0 FAIL / 3345 函数**；最终 **5840 PASS / 0 FAIL / 3353 函数**（+32 PASS、+8 函数）。最终普通与 verbose 全量均退出 **0**；gofmt 空、Linux/Windows/Darwin 的 vet 和 build 全退出 0。原 V/B/S 断言修改 **0/0/0**，新增 Skip **0**，旧测试函数未减。

八项原场景的定向 race **28 PASS / 0 FAIL**；最终补充共享原因场景 race **6 PASS / 0 FAIL**，两次分别记录，不把重复父节点相加成独立测试数。五组 mutation 均实红、六文件 MD5 核对及 touch 后 count=1 复跑；四组初稿还原 **6/6/8/8 PASS**，最终去重补验 **6 PASS**。最终源码校验六文件全部 OK。文档契约检查退出 0。

证据：[最终普通全量](P13-jq/final-full-test.txt)、[最终 verbose](P13-jq/full-verbose.txt)、[最终门槛](P13-jq/final-gates.txt)、[verbose 统计](P13-jq/gates.txt)、[原事件 race](P13-jq/race.txt)、[补充 race](P13-jq/shared-race.txt)、[最终 MD5](P13-jq/final-md5.txt)、[文档检查](P13-jq/docs-check.txt)。初版普通全量 full-test.txt 也退出 0，但在补修前启动，保留为过程记录，不冒充最终普通结果。

前后图为 Linux ui.Tester 软件自绘；代表原帧 `/tmp/floter-P13-jq-diagnostics-{light,dark}.png`。没有 Windows/macOS 真机交互证据，未改变终端或保存语义。

按首次实测时钟 21:59 UTC 至收尾 22:27 UTC，约 28 分钟，低于 45–75 分钟目标。超过 40 次工具调用预算；扩项已停止，超额用于既定完整验收、还原核对与归档。数量、跨组件、逐批推送及全部验收已完成，这两项过程偏差明确保留。
