# P13-ib：整行退场后的键盘归还

开盘 4f62e976，分支 mygo-rewrite，pull 最新、工作树干净；03:02 +08:00 开始。Codex 独立执行；本机 done.log 开盘不存在。最新遗留是 P13-ia 的目标消失/重现，已用真实事件证实，未重做仍存在目标的刷新。

## Stash 清点

八条与上轮 SHA 相同，reverse-check 全部退出 1。它们是已审核早期独立 WIP；stale-kill 已接续归档，继续保留，不重放旧主题与旧 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实测复现与第一批

八项深浅 entry 全红，16 个 elsewhere 对照全部通过；原始 reproduce.txt 落盘。首批只接配置两类，后续按组件成对接入，避免攒到轮末。

1. 现象：保存中整行消失，键盘没有出口，同 ID 返回后 Space 再派发保存。改动：在列表进入空态前，比对前后 ID，原表单持焦则归还页头，并退役其 UI 句柄。验证：编辑 Token → Tab/Space 保存 → 消失 → 重现 → 延迟失败 → Space；旧实现报 `missing save row lost keyboard outlet "Close settings"` 与 `return or callback repeated the operation`。修后结果待本批日志。
2. 现象：丢弃确认虽已随目标消失退役，却遗失键盘。改动：同一消失检测归还丢弃答案焦点，保留草稿。验证：真实编辑 → Configure 开问题 → 消失/重现；旧实现报 `missing discard row lost keyboard outlet "Close settings"`。用户已移到页面复选框的对照不抢焦；修后结果待日志。

V/B/S 旧断言修改 0/0/0，无新增 Skip。全量基线运行中，不提前填写 PASS。

第一批相关 **268 PASS / 0 FAIL**、settingsui vet 退出 0。开盘全量实测 **4322 PASS / 0 FAIL / 2937 测试函数**。M1 撤销 save/discard 的交接后四个深浅 entry 场景红，elsewhere 保持；原串如上。恢复后 MD5 核验与 touch 复验随日志落盘。浅色原图对照已目视：空态不变，右上返回键从没有焦点变为完整蓝色 2px 环，并出现原有快捷键提示。

## 第二批：修复入口

3. 现象：Recheck 在飞时整个损坏工具行消失，键盘滞留在不可见按钮；同 ID 回来后旧按钮重新持焦。改动：列表差分也退役 repair 的焦点句柄，将持焦入口交给返回键。验证：真实 Space → 列表变空 → 同 ID 重现 → 延迟失败；旧串 `missing repair row lost keyboard outlet "Close settings"`，不重派 Recheck。
4. 现象：系统拥有的工具 Reconnect 在飞时退场，同样丢失当前可用出口。改动：连接修复和重新检查共享既有 repair 焦点归属，两个入口均在行消失时归还。验证：运行时不可用 → Reconnect → 消失/重现 → 延迟失败；旧串 `missing reconnect row lost keyboard outlet "Close settings"`，主动移焦对照不受影响。

第一批 **a7d619df** 已推送，MD5 还原后 touch 复验 **10 PASS**。第二批相关 **278 PASS / 0 FAIL**、包 vet 净；M2 撤销 repair 交接后四个深浅 entry 红，elsewhere 全绿。还原、touch 复验与图片随本批落盘。V/B/S 旧断言仍为 0/0/0。

## 第三批：启停入口

第二批 **55a34832** 已推送，M2 还原复验 **10 PASS**。

5. 现象：启用写回未返回时整行消失，键盘失效；重现后旧开关焦点可能重新接管。改动：退役失踪目标的 toggle UI 句柄，真实持焦时归还页头。验证：关闭的开关 → Tab/Space 启用 → 消失/返回 → 失败；旧串 `missing toggle row lost keyboard outlet "Close settings"`，整个序列只写入一次。
6. 现象：停用答案已派发、仍处忙态时目标消失，在重新出现之前没有可用焦点。改动：同样归还 disable 答案焦点，既有 busy 与回调保持。验证：点击开关开问题 → Tab/Space 答案 → 整行退场；旧串 `missing disable row lost keyboard outlet "Close settings"`，失败后原动作结算、页头仍可键盘关闭。

第三批相关 **288 PASS / 0 FAIL**、包 vet 净。M3 暂时撤去 toggle/disable 交接，四个 entry 场景红，已有页面焦点的对照不受影响；恢复后 MD5 与 touch 复跑结果随本批保存。图片继续使用原始 Tester 事件。

## 第四批：移除与卸载

第三批 **f3bda08f** 已推送，M3 还原复验 **10 PASS**。

7. 现象：移除答案已经派发时目标从列表撤下，在它回来前键盘没有落点。改动：移除答案的旧句柄随目标退役，焦点归还已存在的返回键。验证：真实移除确认 → Tab/Space → 列表变空 → 同 ID 重现 → 延迟失败；旧串 `missing remove row lost keyboard outlet "Close settings"`，全程只派发一次移除。
8. 现象：组件卸载忙态浮层随目标消失，旧对话框焦点悬空，目标回来后可能重新接管。改动：卸载 panel 句柄也随目标消失退役，原在飞操作继续由回调结算。验证：两级确认实际进入浮层并提交 → 消失/重现 → 延迟失败；旧串 `missing uninstall row lost keyboard outlet "Close settings"`。对照场景尊重模态焦点陷阱，先让卸载完成再移到页面复选框。

最终测试增加“另一个集成仍在”的矩阵，检查列表未变空时同样正确；没有新增第九项来凑数。已有重复 ID 的 B 测试继续纳入每批回归。

第四批提交前捕获旧 B 回归：`TestNoticeVanishedUninstallRetiresDialog` 深浅两例报 `invisible uninstall question swallowed Escape: close=0`。新增焦点交给返回键后，框架 Tooltip 的 Escape 抢先隐藏提示而没有退出设置。修正：返回键也注册现有 Escape 路由，复用原录制/确认/会话/关闭优先级；没有改旧断言，没有跳测。第四批首次失败日志保留为 batch4-initial.txt，修正后结果另录。此为第八项的兼容修正，不另计数量。

修正后第四批相关 **360 PASS / 0 FAIL**、包 vet 净；包含原有 Escape、录制与重复 ID 回归。M4 八个 entry 场景实跑红（空列表/有剩余行 × 深浅 × 两项），MD5 还原后 touch，连同新增页头 Escape 优先级对照共 **21 PASS / 0 FAIL**。新增对照真实打开问题、Tab 到页头，第一 Escape 只取消问题、第二次才关闭，不把页头的出口修复变成绕过确认。

第四批 **d4bb4bf4** 已推送。M5 从最终源码撤掉列表交接入口，**32 个 entry 子场景全部红**，elsewhere 对照和页头优先级保持；两份源码 MD5 逐字节还原，touch 后 **75 PASS / 0 FAIL**。至此五组变异均有原始报错、校验与复验；最终普通/verbose 全量及三平台检查开始执行，尚未完成前不写通过。

## 偷了什么

- craft-loop 菜单 **#3 品牌 focus 环**：取 Apple 单一交互色的职责，转译为本仓既有 accent 环 **2px**、环外间隙 **1px**。返回键接住实际焦点，新增颜色 **0 个**。
- 菜单 **#4 签名交互**：归还焦点 **0ms**；刷新自动派发 **0 次**，用户 Space 关闭 **1 次**。保留原确认时限 **3000ms**，延迟回调不移动键盘。
- 菜单 **#12 材质细节**：原 **1px** 受光边、E1/E2 层级保持，新增玻璃/投影 **0 层**。这轮通过清理不可见操作层来变轻，不为五项菜单添无关装饰。

已顺序读取 qiaomu-design、user-preferences、craft-loop、preflight、chinese-typography，补读 Apple DESIGN 的色彩/交互部分。沿用用户明确方向，Codex 独立执行，无风格选择 HTML、无外部模型。

## 两轮复核与边界

第一轮真实 before：艺术总监视角——空态缺少键盘落点；工程师——行内交接在整行缺席时无法运行；使用者——Space 失效或在重现后重复原操作；读者——只看到列表状态，看不到当前键盘位置。第二轮集中核对空态、有剩余行、模态卸载退场、返回键 Escape 四种场景；旧 B 测试揭示提示吞键后已修正，没有重画页面。

为什么更轻、更有空间感：移除不可见的焦点滞留，让现有返回键明确承担下一步操作位置；不叠加新的面、影或提示。Linux ui.Tester 软件自绘，Windows/macOS 本轮仅编译与 vet，不声称真机交互验证。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/黑底/霓虹 | 无新增，主题 token 零修改 |
| A2 字体 | 保留系统中文字体；用户仓库约定优先于通用换字体建议 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用：原生设置页，任务标题与入口保持 |
| A5 卡片海 | 无新增容器 |
| A6 假数据 | 无产品假指标；Workspace/Documentation Tools 是事件测试夹具 |
| A7 空词/感叹号 | 产品文案和 i18n 零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原有单行标题 |
| A10 Bento | 不适用 |
| A11 左竖线 | 无装饰性新增 |
| B1 设计读取 | 3/2/7：克制、即时键盘反馈、原信息密度 |
| B2 主题 | 每个 Tester 单一深/浅主题 |
| B3 记忆点 | 目标退场后唯一返回位置可见 |
| B4 动效气质 | 高频键盘交接 0ms |
| C1 目的/频率 | 无新增键盘动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原按压和颜色过渡保持 |
| C4 origin | 原返回位置；无新增浮层 |
| C5 reduced-motion | 测试设 ReduceMotion=true，新增接线即时 |
| C6 hover | Go 自绘沿用原指针路由；CSS 媒体查询不适用 |
| C7 布局动画 | 无新增 |
| C8 keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新增动效，不虚构真机慢放证据 |
| D2-1 系统字体 | 保留，无 webfont |
| D2-2 字号/行高/字重 | 原 Caption 9.9、正文 10.8 和行盒未达通用 ≥14px/1.5，沿用需拍板，不擅改 |
| D2-3 混排 | 文案零改，无新增混排问题 |
| D2-4 标点/对比 | 沿用原标点与墨值，未声称旧次要墨全部达到 7:1 |

可复现问题沉淀在本仓 §6.203 与事件断言；不把一次修复升级成全局 skill 审美偏好。通用网页不适用项和既有字号例外已明列。

## 图像证据与目视结论

**64 张 720×900 原始 PNG**（8 项 × 空列表/有剩余行 × 深浅 × before/after），来自相同最终事件。before 由 M5 暂停整行交接生成，after 恢复最终源码生成；无手工绘制。另有 [浅色对照板](P13-ib/comparison-light.png)、[深色对照板](P13-ib/comparison-dark.png) 与 [32 对像素差](P13-ib/image-differences.txt)。每一对变化均非零；原图也在 `/tmp/floter-P13-ib-<入口>-<dark|light>-<before|after>.png`。

逐项看图：①保存、②丢弃、③重新检查、④重新连接、⑤启用、⑥停用、⑦移除、⑧卸载在整行撤下后都把焦点环落到右上返回键；before 的该位置仅有灰色关闭图标，after 有完整蓝环和原提示。深浅均一致，图像差集中于页头返回区。深色卸载 survivor 原图还可见 Documentation Tools 完整留在页面，剩余行的检查、移除、启用三处没有误获焦点，浮层已退场。截图本身不证明按键派发，Space/Escape 与回调行为由上述事件断言验证。

## 仍待

1. 下一轮先复现 host 配置表单主动折叠/重开，以及 Copy/Export 在飞与 owner/schema 刷新的组合出口。仅为线索，不重复本轮整行消失的八类已修场景。
2. 保存期间草稿归属 161-4、排队别名 176-9、字号/行盒、accent 禁用填充、未知多选/Radio、小步长六位存储、终端 S1b/S1c/S2 尾仍需拍板。
3. 参数全部移除后恢复仍未复现；Windows/macOS 真机证据缺席。八条早期独立 WIP 原样保留。

## 完整门槛过程记录

三平台 vet/build 全部退出 0、gofmt 空。第一次普通/verbose 全量并行执行，两遍均在 settingsui 达到 Go 默认 10 分钟超时（总耗时约 634 秒），不能把 0 FAIL 断言误记为通过。保留 gates-initial.txt、full-test-initial.txt 与 /tmp 下两份 parallel 原日志；随后原命令、原默认超时串行重跑，排除并行资源竞争，不改产品与测试断言。

本轮产品已按四批推送；结案文档等完整门槛通过才提交。40 次调用预算用于收尾；如串行重跑尚未结束，只继续等待和必要记录，不扩展产品范围，最终如有超额会明列。

串行 GOMAXPROCS=4 也在 10 分钟超时，撤回“仅两遍并行争用”的假设。堆栈在旧 `TestTheSummonLineStandsOnTheInkLadder` → `actionRungProbe` → `sample` → `Tester.Image` → raster.RGBA；每个像素都转换整张图，基线该测试已耗时 107.26 秒。系统同时间另有高 CPU view.test（其他工作，未干预）。因此再以 GOMAXPROCS=1、GOGC=200 跑原命令，降低调度与 GC 竞争；仍不改默认 10 分钟超时、不改旧 B 测试与源码。首次串行输出保留 full-test-serial4.txt。

流程偏差：工具调用已超 40 次，额外调用只用于定位完整验收超时与必要复跑、结案；停止扩题。测试未成功前不提交结案文档、不把八项产品提交等同于整轮通过。

降低并发仍超时；单独实测旧 Summon pin **272.49 秒**且全绿，确认不是状态循环。必要验收修正：`actionRungProbe` 在一次调用中只取一次不可变帧截图，再逐像素读取；坐标、整数转换、RGBA 读取、遍历、最远墨算法与全部断言逐字保持。原每像素转换整张 720×900 图改为每次 probe 转换一次；不缓存跨帧、不降采样、不放宽误差、不删除测试。本修正仅解除验收时间阻塞，不算第九项产品打磨；恢复原 GOMAXPROCS=4 后重跑原全量命令。

## 最终结果

四批每两项产品提交并推送：**a7d619df / 55a34832 / f3bda08f / d4bb4bf4**。最终产品 **d4bb4bf4**；八项跨配置表单、丢弃确认、修复按钮、启停控件、移除确认与卸载浮层。页头 Escape 兼容修正属于第八项，不另计数量。

开盘 **4322 PASS / 0 FAIL / 2937 测试函数**；最终普通与 verbose 全量均退出 **0**，**4397 PASS / 0 FAIL / 2946 测试函数**，增加 **75 PASS / 9 函数**。最终测试环境恢复 **LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4**；原命令、默认 10 分钟超时保持，未依赖 GOGC 调整。gofmt 输出为空，三平台 vet/build 全部退出 **0**；修改采样辅助函数后三平台 settingsui vet 额外全绿。

V/B/S **断言**修改 **0/0/0**；另有一项测试辅助实现修正（每像素截图 → 每 probe 同帧截图），明确登记，不能称“旧测试文件零改”。全部旧测试函数、断言、阈值保持，新增 Skip **0**。修改前 Summon 探针 **272.49 秒**，修改后两主题读出的 plane/name/description/summon 四组 RGBA 与此前日志完全一致，具体耗时见 [probe-verification.txt](P13-ib/probe-verification.txt)。

最终批相关 **360 PASS**，本轮事件单独 **75 PASS**。五组 mutation 实跑变红并 MD5 还原、touch 复验（**10/10/10/21/75 PASS**），最终 M5 覆盖 **32 个 entry 场景**；已主动移焦与页头 Escape 优先级对照保持。64 张原图、2 张对照板入仓；32 对像素差非零，浅色每对 **4896** 像素、深色每对 **3679** 像素。

完整门槛见 [gates.txt](P13-ib/gates.txt)、[普通全量](P13-ib/full-test.txt)；verbose 原输出位于 `/tmp/floter-P13-ib/gates/verbose.txt`。三次完整验收超时与根因调查保留；未以失败运行的 PASS 数作为最终数字。Linux 自绘已验，另两平台仅 vet/build。

完成时间 **2026-11-06 20:08:20 UTC**，约 **65 分钟**。处于 45–75 分钟目标范围。调用预算超额已明列；待办顶部替换，done.log 记录结案 SHA。
