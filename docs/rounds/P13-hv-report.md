# P13-hv：内联确认的退出、超时与并存

开盘 `d0fd58d4`，pull 最新、工作树干净。done.log 不存在；已读取实际存在的 stash-backlog。基线正在实跑，完成后补录。开始时间 14:29 UTC 左右。

## Stash 清点

既有 stale-kill 抢救已在前轮归档接续；以下活动 stash SHA 与 P13-hu 一致，本轮逐条 reverse-check 均为 1，保留独立旧 WIP，不覆盖新主题或恢复 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实施记录

仅完成且通过事件验收的项计入；旧 V/B/S 断言修改暂为 0/0/0。

### 第一批：外部收起的键盘出口

1. 现象：停用确认在 Escape 或 3 秒到期后丢失焦点，Space 不能重开。改动：用跨帧 Handle 识别正在退出的答案，仅在它拥有键盘时归还原开关。验证：深浅 × Escape/timeout × 焦点仍在/已移到检查按钮，共八场景；取消派发 0 次，原入口可以 Space 重开。
2. 现象：移除确认同样丢失原入口。改动：同族按归属回到移除按钮，进入组件卸载时保留浮层焦点。验证：同样八场景，并回归已有卸载与显式取消路径。

基线全量实测 **4083 PASS / 0 FAIL / 2888 测试函数**。第一批相关回归 **97 PASS / 0 FAIL**。M1 回注旧产品：`external exit lost its keyboard opener`、`Space cannot reopen the cancelled question`，8 个场景红；还原后 MD5 通过、touch 复跑 **18 PASS**，settingsui vet 通过。V/B/S 旧断言修改 **0/0/0**。

第一批 `8412ad8a` 已推送。

### 第二批：配置丢弃的 Escape 与到期

3. 现象：丢弃问题打开时 Escape 执行设置 Close，问题没有取消。改动：把已有 configDiscard 加入全局确认取消集合，保留草稿、表单展开状态，不写回。验证：深浅 × 中英文 × 焦点仍在/已移开，Close=0、保存=0、值仍为 edited，Space 可重开。
4. 现象：配置丢弃在 3 秒到期后丢失 Configure 焦点。改动：沿用跨帧答案 Handle，仅在离场答案持有键盘时还给入口。验证：同样八场景，检查按钮上的焦点不受影响。

相关回归 **103 PASS / 0 FAIL**；M2 原始红串 `discard exit closed settings or lost draft: closed=1 armed=true open=true value=edited saved=0`、`discard exit lost Configure focus`；恢复 MD5 与 touch 后 **18 PASS**，settingsui vet 净。V/B/S 0/0/0，旧断言不改。

第二批 `83a8b09f` 已推送。

### 第三批：慢失败的阅读与重试窗口

5. 现象：停用操作等待 5 秒后失败，下一帧立即撤掉错误，用户既读不到原因也不能 Space 重试。改动：仅问题仍开着时，从失败到达时重新开始原有 3 秒回答窗口。验证：两主题等待 5 秒、失败显示、2 秒后仍显示、Space 第二次派发、3 秒后正常撤回；已 Escape 取消的迟到失败不复开。
6. 现象：移除操作同样被提问时钟吞掉失败。改动：同族重启回答窗口，不改执行与取消语义。验证：同上四场景，实际回调计数 1→2，取消场景不显示迟到失败。

相关回归 **112 PASS / 0 FAIL**；M3 回注旧时钟，原始红串 `slow failure vanished before it could be read`、`slow failure cannot retry from keyboard: calls=1`，四场景红。MD5 还原并 touch 后 **10 PASS**；settingsui vet 净。V/B/S 0/0/0。

第三批 `d9924ea6` 已推送。

### 第四批：两种确认并存的身份

7. 现象：先开停用、2 秒后再开移除并 Tab 到确认；停用到期后移除按钮失焦，Space 派发 0 次。改动：两条确认使用各自稳定的根 key，移除不再继承前一条的节点。验证：深浅完整页面真实事件，停用撤回、移除仍开且 Space 恰好移除 1 次，停用 0 次。
8. 现象：先开移除再点开关询问停用，新问题默认焦点却落在旧移除的取消；Space 取消了错误的问题。改动：给新停用独立身份，首次 AutoFocus 作用于它自己的 Cancel。验证：深浅 Space 只收起停用，移除仍保留，两种破坏性回调均 0 次。

相关回归 **118 PASS / 0 FAIL**；M4 四场景实跑红：`earlier question expiry lost removal answer focus`、`new question default cancels the wrong band: disableOpen=true removeOpen=false disable=0 remove=0`。MD5 校验、touch 后 **6 PASS**，settingsui vet 净。第二轮看图将第 8 项截图时点移到 Space 之后，让图上直接看见留下的是哪个问题；断言不变。

第四批 `367c5d29` 已推送。产品固定后启动普通/verbose 全量各一次；三平台 vet/build 已全过，gofmt 空。完整测试数以最终日志为准。

## 偷了什么

本轮真实读取 qiaomu-design SKILL、user-preferences、craft-loop、preflight、chinese-typography，以及 Apple DESIGN 的色彩/控件章节。既定方向按任务书直接实施，Codex 独立完成。

- 工艺密度菜单 **#3 品牌 focus 环**：保留 Apple 的单一交互色角色与 **2px** focus 描边，映射到现有主题 Focus；焦点交接 **0ms**，只有离场答案持有键盘才归还。
- **#4 签名交互**：既有 Tab/Space/Escape 各司其职；取消派发 **0 次**，重试从 **1→2 次**；失败阅读沿用 **3000ms**，从结果到达起算。
- **#6 编辑细节**：多个问题各保留自身身份，插入和撤回不互换答案；不加空标签、重复提示或新布局包装，增加占高 **0px**。
- **#12 材质细节**：沿用 E1 面、**1px** 受光边与 **8px** 常规圆角；新增面、阴影、透明叠层均 **0**，错误仍用已有警告墨。

为什么更轻、更有空间感：提示离场后键盘回到有意义的入口，留在场上的动作不跳焦点，错误有完整阅读窗口；保住已有层级，不靠更多边框或玻璃堆叠。

## 两轮看图

第一轮以真实事件捕获失败帧；第二轮复核深浅对照和 4× 焦点局部。艺术总监视角最弱处是错误凭空消失，已给 3 秒可读窗口；工程师视角是并存提示的位置身份错配，已固定两根 key；任务用户视角是 Escape 关闭了设置和 Space 取消错问题，均有回调与草稿断言；读者视角保留旧小字号限制，不冒充已解决。

图上确认的变化：慢失败帧中，原来消失的错误句和重试按钮完整保留；移除问题中原来缺失的蓝色焦点环保留在确认按钮；插入停用再 Space，留下的是移除问题。中文丢弃 Escape 后只有提示带退场，Token 的 edited 草稿仍可见。

前两项外部出口和第 4 项到期在当前截图的“已收起”时点逐像素相同：鼠标打开时焦点环没有进入可见模式。这里**不声称像素改版**；真实 Focused + 下一次 Space 重开的 B 断言证实键盘修复。各项依然交付同帧 before/after，像素差范围详见 `P13-hv/image-differences.txt`。

| 场景 | 浅色对照 | 深色对照 |
| --- | --- | --- |
| 停用 Escape | [图](P13-hv/disable-escape-light-comparison.png) | [图](P13-hv/disable-escape-dark-comparison.png) |
| 停用到期 | [图](P13-hv/disable-timeout-light-comparison.png) | [图](P13-hv/disable-timeout-dark-comparison.png) |
| 移除 Escape | [图](P13-hv/remove-escape-light-comparison.png) | [图](P13-hv/remove-escape-dark-comparison.png) |
| 移除到期 | [图](P13-hv/remove-timeout-light-comparison.png) | [图](P13-hv/remove-timeout-dark-comparison.png) |
| 中文丢弃 Escape | [图](P13-hv/discard-escape-zh-light-comparison.png) | [图](P13-hv/discard-escape-zh-dark-comparison.png) |
| 中文丢弃到期 | [图](P13-hv/discard-timeout-zh-light-comparison.png) | [图](P13-hv/discard-timeout-zh-dark-comparison.png) |
| 停用慢失败 | [图](P13-hv/disable-slow-failure-light-comparison.png) | [图](P13-hv/disable-slow-failure-dark-comparison.png) |
| 移除慢失败 | [图](P13-hv/remove-slow-failure-light-comparison.png) | [图](P13-hv/remove-slow-failure-dark-comparison.png) |
| 前序确认到期 | [图](P13-hv/remove-survives-expiry-light-comparison.png) | [图](P13-hv/remove-survives-expiry-dark-comparison.png) |
| 插入新确认 | [图](P13-hv/disable-inserted-light-comparison.png) | [图](P13-hv/disable-inserted-dark-comparison.png) |

[总览](P13-hv/overview.png)，[4× 浅色焦点](P13-hv/focus-detail-light.png)，[4× 深色焦点](P13-hv/focus-detail-dark.png)。全部为 Linux ui.Tester 自绘截图，没有 macOS/Windows 真机证据。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，保留近黑主题 |
| A2 非系统字体 | 本仓既有系统中文字体优先；不换装 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置确认 |
| A5 三等分/卡片海 | 无新增卡片 |
| A6 假数据 | 无产品假指标，错误与名称为测试输入 |
| A7 空词/感叹号 | 产品文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 既有页头不改，图中完整 |
| A10 Bento | 不适用 |
| A11 左竖线 | 无新增 |
| B1 读取/拨盘 | 3/2/7：克制、即时、原密度 |
| B2 单主题 | 每张图显式指定深浅主题 |
| B3 记忆点 | 问题和焦点始终属于当前动作 |
| B4 动效气质 | 焦点交接无额外等待 |
| C1 高频/目的 | 键盘交互不加动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 ≤300ms/按压 | 沿用已有 120ms 颜色过渡与 0.97 按压 |
| C4 origin | 仍为原入口下的内联提示 |
| C5 reduced-motion | 真实事件和截图均用 ReduceMotion=true |
| C6 hover 媒体查询 | 不适用 Go 自绘，指针路由不变 |
| C7 布局动画 | 没有新增布局或布局动画 |
| C8 重播 keyframes | 无新增 |
| C9 手势/拖拽 | 本轮不涉及 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新增动效，不声称慢放验收 |
| D2-1 中文字体 | 保留系统字体，无 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px 未满足；Caption 9.9/Body 10.8 和既有行高仍属拍板项 |
| D2-3 混排空格 | i18n 零改，配置退出中英文均验 |
| D2-4 标点/对比 | 原文案与主题墨不改，不声称全部旧墨过 7:1 |

用户限定现有功能修复优先：不为通用五项工艺密度、网页 viewport 或字体换装要求添加装饰。可复现规则落本仓 §6.197 与事件 pin，不写全局 skill 偏好。

## 仍待

下一轮首选确认在列表刷新/当前集成消失、多个到期窗口与忙态组合下的归属。`expireConfirmations` 仍会把忙态已过去的时间列入 next；是否会让其他确认缺少主动唤醒只是代码线索，本轮未做真实静止页面复现，不算已确认 bug。运行参数全部移除/恢复继续沿用线索。字号/行盒、保存期间草稿归属、终端 S1b/S1c/S2 尾继续沿用需拍板，不改会话语义。

## 完整门槛与交接

最终 **4135 PASS / 0 FAIL**（基线 4083，+52）；普通/verbose 全量均退出 **0**。函数 **2896**（基线 2888，+8）；旧测试零改、无新增 Skip。gofmt 空，linux/windows/darwin vet/build 全过，最终文档契约通过，产品 MD5 再次核对一致。

证据：[普通全量](P13-hv/full-test.txt)、[三平台门槛](P13-hv/gates.txt)、[文档契约](P13-hv/docs-ci.txt)、[最终源码 MD5](P13-hv/final-source.md5)、[M1 红](P13-hv/M1-red.txt)、[M2 红](P13-hv/M2-red.txt)、[M3 红](P13-hv/M3-red.txt)、[M4 红](P13-hv/M4-red.txt)。完整 verbose 与开盘基线保留在 `/tmp/floter-P13-hv/`。最终代表帧另存 `/tmp/floter-P13-hv-notice-{light,dark}.png`。

任务书 `~/.hermes/state/iteration/task-floter.md` 的旧 P13-fz 进度和禁用材质选题已替换为本轮结果与下一步；历史拍板项仍以 `.pending-bugs.md` 保留，不因压缩交接而自动解禁。收尾追加 done.log。

过程记录：14:29 UTC 左右开盘，14:57 UTC 全量结束，约 29 分钟，低于 45–75 分钟目标。八项真实修复已完成，不以重复全量或无关改动填时长。四批每两项先定向回归/vet 再立即提交推送；全轮完整门槛集中一次执行，没有声称每批重复全量。第四批为了让图直接显示错误取消对象，将截图时点移到 Space 后，再运行该组变异与恢复；断言和产品不再变化。
