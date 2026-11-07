# P13-jq · 诊断过程的当前事实与完整失败说明

开盘 2a8b1e1c，工作区干净、ff-only 已同步。沿最新 P13-jp 的实际遗留继续，P13-hw 线索已由后续轮次覆盖。规格先落盘，八个候选尚未算成果。

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
