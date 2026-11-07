# P13-jg：导入结果完整性与备份失败恢复

开盘 8835887e，pull 已同步、工作树干净。2026-11-07 16:48 UTC 起，Codex 独立执行。done.log 开盘不存在。按 P13-jf 最新仍待接续；本报告增量记录，未完成项不计交付。

## Stash 清点

无新增 stale-kill；下列 SHA 与前轮一致。reverse-check 均为 1；沿用已完成语义审核的早期独立 WIP 处置，保留原件，不重新合入旧主题或 Tauri 链。

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

## 逐项记录

1. 现象：同一真实导入包含成功、失败、跳过时，失败详情藏掉已生效结果。改动：保留原汇总与失败详情并列，零成功且全失败仍只显示失败。验证：原产品深浅事件均报 `partial failure hides the already-applied result`；导入实际计数 1/1/1。
2. 现象：未携带包的未安装集成只显示 unchanged 1，原因只进日志。改动：宿主保留 Skipped 的 ID 与 Reason，报告用既有警告墨显示诊断。验证：原产品深浅事件均报 `skipped import hides the integration and its reason`；修后检查同一报告与忙态释放。

首批相关 **21 PASS / 0 FAIL**，shell/settingsui vet 净。基线 verbose 全量退出 0，**5480 PASS / 0 FAIL**；基线 HEAD 用 `git grep -h '^func Test' HEAD -- '*_test.go'` 实数为 **3247**，前轮报告的 3245 未沿用。开盘计数只包括已跟踪文件，与后来新增的未跟踪测试无关。旧 V/B/S 断言修改 0/0/0；新增 Skip 0。

第三项原候选长来源路径越界未复现，深浅均通过；保留边界验证、不计修复，改查失败尾部详情。

3. 现象：第四个失败只剩“and 1 more”，具体集成与原因不可查看。最终改动：保留三条正文与计数，后续失败逐条放入现有页面滚动区。验证：原产品深浅均报 `failure tail cannot reveal the fourth integration or reason`；第二轮再验 40 个失败，无须悬停即可读取，最后一条能滚进视口。首批临时 tooltip 方案已被后续修正取代。
4. 现象：报告收到每个工具的排除字段数量，却只有名称可见。改动：警告行的详情提示展示逐工具数量；常驻正文保持原样。验证：原产品深浅均报 `excluded-fields notice hides per-integration field counts`；修后验证 2/5 两个数量、布局与退场。此提示只承载补充数量；警告本身一直可见。

第二批相关 **21 PASS / 0 FAIL**，settingsui vet 净。首轮图像复核：浅色混合导入现在同时看得到成功计数和两种未生效原因；深色排除详情在原警告之上显示 2/5，报告原行不移动。数量工具提示沿用框架 600ms 和现有 E2 材质。未增加展开按钮或键盘步骤。

5. 现象：version=-1 的真实 JSON 导入会写仓库、配置并报告成功。改动：版本范围从 `==0` 拒绝改为 `<1` 拒绝，v1/v2 保持兼容。验证：改前报 `invalid document changed repository` 与 `invalid document wrote configuration`；修后检查本地化错误标题、具体版本原因、无写入及修正文件可重试。
6. 现象：配置字符串内的 0xff 被 JSON 解码替换，损坏值静默写入配置。改动：解析前拒绝非法 UTF-8 字节。验证：改前报 `invalid document wrote configuration`；修后验证具体原因、无写入、busy 释放和同入口恢复。合法中文与合法 U+FFFD 不误拒。

第三批相关 **15 PASS / 0 FAIL**，extensions/shell vet 净；包括原 ImportSync 权限、合并、跳过、排除字段行为与 ReadSync 文档拒绝回归。

7. 现象：已声明 script 的文件丢失，导出仍成功并覆盖已有备份，所得文件无法在新机器恢复。改动：读取必需脚本失败立即返回路径与原因，在选择保存位置前拒绝。验证：原产品两主题均报 `incomplete export replaced the existing backup`；修后备份逐字节不变、选择器 0 次、busy 释放，补回脚本后成功导出原文。
8. 现象：已声明 static descriptor 的文件丢失，同样导出不完整备份。改动：descriptor 读错返回原因；无包集成的兼容路径不变。验证：两主题真实删除事件复现；修后同样保护备份，补回 descriptor 后完整 JSON 内容相等。独立覆盖 script 和 static-descriptor 两类现有组件。

第四批相关 **19 PASS / 0 FAIL**，extensions/shell vet 净。八项均已完成首轮修后验证，正式 mutation 与最终门槛尚未结束。

第二轮诊断：失败尾部不应依赖鼠标提示，长列表也不适合不可滚动浮层。新真实页面事件在临时 tooltip 产品上报 `failure tail is unavailable without hover`。因此第三项修正为原滚动区内逐条正文；原三条摘要字串与 B 测试不改。只调整本轮新测试的 S 断言（临时 hover-only → 常驻详情），没有动开盘旧断言。

第二轮修正相关 **24 PASS / 0 FAIL**，settingsui vet 净；40 条失败最后一条可滚进 720×900 视口。该修正单独提交，不把它冒充第九项。

## Mutation 实跑

| 注入 | 实际红串 | 还原验证 |
| --- | --- | --- |
| M1 移除失败时的成功/跳过汇总 | `partial failure hides the already-applied result`（深浅两次） | MD5 OK；touch 后 6 PASS / 0 FAIL |
| M2 恢复静默忽略脚本/descriptor 读错 | `missing essential was exported without visible cause`；`incomplete export replaced the existing backup`（两类 × 深浅） | MD5 OK；touch 后 6 PASS / 0 FAIL |

原始红、逐字节校验与恢复日志均在本轮目录 `M1-*` / `M2-*`。注入用独立原件备份，不 stash 用户工作。全量门槛只在恢复完成后启动。

## 偷了什么

- craft-loop **#6 编辑细节**：一行汇总与失败诊断分开；前三条摘要保留，尾部逐项可读；报告纵向节奏沿用 `Space(1.5)`，额外装饰容器 **0**。
- **#4 签名交互**：结果反馈增加等待 **0ms**；补充数量沿用 **600ms** hover / **400ms** warm 提示，移开即退；必要诊断放正文。
- **#5 多层阴影**：数量提示复用两层外投影，alpha **0.30 / 0.20**，偏移/模糊为 `Space(1.5)/Space(4.5)` 与 `Space(0.25)/Space(0.5)`；面内新增阴影 **0**。
- **#12 材质细节**：提示复用 **1px** `EdgeHighlight` 受光边与 `RadiusSM`，没有新透明面或叠玻璃。Apple DESIGN 的 **8px** 圆角、**1px** 边与单一交互 accent 角色映射到既有 token；结果汇总继续中性、警告继续语义墨。

用户要求修现物优先，不为凑满五项添加装饰。**为什么更轻、更有空间感**：结果与原因在原平面内完整呈现，只有补充数量临时浮起，失败尾部不再挤进一个浮层。

## 两轮看图与证据

第一轮四视角：艺术总监发现多层红色提示容易争夺注意，保持一行中性结果主线；工程师发现失败尾部仅悬停可读，不适合长列表；任务用户无法知道“已生效”与“未变化”的分别，真实 1/1/1 导入现已齐全；读者看到路径过长，实测原路径换行正常，取消猜测性修改。

第二轮集中复核：常驻失败尾部、40 项真实滚动、排除数量不挤动原行、两类文件修复后备份内容完整。深色长列表图里最后一条 `dev.workspace.39` 在视口内；浅色第四条失败直接位于原摘要下方；无须新的浮层或操作入口。

八项 × 深浅均有 before/after PNG 与对照板，见 [总览](P13-jg/overview.png)。[像素差](P13-jg/image-differences.txt) 16 组均非零；错误文件的临时路径含测试随机目录，不能把整张差值误称为纯墨值变化。代表图 `/tmp/floter-P13-jg-transfer-{light,dark}.png`。原始图是 Linux ui.Tester 软件自绘，非三平台真机截图。

| 项 | 浅色对照 | 深色对照 |
| --- | --- | --- |
| 1 部分成功 | [图](P13-jg/partial-light-comparison.png) | [图](P13-jg/partial-dark-comparison.png) |
| 2 跳过原因 | [图](P13-jg/skipped-light-comparison.png) | [图](P13-jg/skipped-dark-comparison.png) |
| 3 失败尾部 | [图](P13-jg/failure-tail-light-comparison.png) | [图](P13-jg/failure-tail-dark-comparison.png) |
| 4 排除数量 | [图](P13-jg/excluded-details-light-comparison.png) | [图](P13-jg/excluded-details-dark-comparison.png) |
| 5 负版本 | [图](P13-jg/negative-version-light-comparison.png) | [图](P13-jg/negative-version-dark-comparison.png) |
| 6 非法 UTF-8 | [图](P13-jg/invalid-utf8-light-comparison.png) | [图](P13-jg/invalid-utf8-dark-comparison.png) |
| 7 缺脚本 | [图](P13-jg/missing-script-light-comparison.png) | [图](P13-jg/missing-script-dark-comparison.png) |
| 8 缺描述文件 | [图](P13-jg/missing-descriptor-light-comparison.png) | [图](P13-jg/missing-descriptor-dark-comparison.png) |

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹 | 无新增，沿用近黑主题 |
| A2 非默认字体 | 中文原生系统字体规则与本仓合约优先，不更换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置页 |
| A5 卡片海 | 新卡片 0；失败尾部用原正文行 |
| A6 假数据 | 产品无虚构指标；dev.workspace 为测试夹具 |
| A7 空词/感叹号 | 原 i18n 零改；新诊断仅具体损坏原因 |
| A8 眉批 | 无新增 |
| A9 H1 | 原页头完整可见 |
| A10 Bento | 不适用 |
| A11 竖线装饰 | 无新增 |
| B1 设计读取/拨盘 | 3/2/7，克制、即时、高信息密度 |
| B2 主题锁定 | 每个 Tester 明确深/浅 |
| B3 记忆点 | 一份报告同时说清生效结果与未生效原因 |
| B4 动效气质 | 结果 0ms，补充提示沿用原延迟 |
| C1 目的/频率 | 高频结果不加动画 |
| C2 all/ease-in/scale0/fade | 无新增 |
| C3 时长/按压 | 原控件过渡、按压不变；600ms 为提示等待，非动画时长 |
| C4 origin | 数量提示仍锚定原警告行 |
| C5 reduced-motion | 新 Tester 全部 ReduceMotion=true |
| C6 hover 媒体查询 | Go 自绘，不适用 CSS；指针进退均实测 |
| C7 布局动画 | 未增加 |
| C8 keyframes 重播 | 未增加 |
| C9 手势 | 无新手势；40 条结果使用原滚动链 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新动画，不声称真机慢放检查 |
| D2-1 字体 | 沿用系统字体，不引入 webfont |
| D2-2 字号/行高 | 通用 ≥14px/1.5 与现行 9.9/10.8 账不一致，旧拍板项保留；本轮不改 |
| D2-3 混排空格 | 原中英 i18n 零改；数量以 ID: 数字在补充详情展示 |
| D2-4 标点/暗色对比 | 原标点与墨保持；未宣称旧次要字全部达到 7:1 |

缺陷记忆落本仓 §6.234 和真实事件 pin，不把一次修复升级成全局 skill 审美规则。用户现有方向与原生合约优先于网页四方案、viewport 与装饰数量要求。

## 仍待

1. 下一轮首选单工具配置传输的损坏文本与恢复边界；对照集成列表文件路径，先真实事件复现，不凭同名函数认定问题。未复现前只是线索，不改变覆盖策略、保存时机或确认窗口。
2. 字号/行盒、accent 禁用填充、多草稿覆盖、未知多选/Radio、极小步长、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾继续待拍板。
3. 原生文件选择器、Windows/macOS 运行时未验。八条旧 WIP 仍按表保留，无新抢救稿。

## 最终门槛（进行中）

普通/verbose 顺序全量与三平台 vet/build 已启动；全部结束后按日志补数字。最终旧 V/B/S 修改 **0/0/1**，无新增 Skip；一条本轮临时 S pin 在第二轮改为可滚动正文并保留更强的无悬停断言。

第一次平台门槛进程退出 **143**，已完成 gofmt、Linux vet/build；Windows vet 未留下完成状态，不能计通过。保留原日志，后续只补跑未完成的 Windows/Darwin 四项，不重跑已通过 Linux 项。没有观察到断言失败，不猜测外部终止原因。

首次普通全量退出 1，唯一失败为 `TestTheSettingsPageHasNoFrameworkTips`：`transfer.go carries 2 tokens.Tip calls, want 1` / `the settings page carries 13 tips, want 12`。这是新增排除数量提示对应的 **S** 结构变化，精确计数从单文件 **1→2**、全页 **12→13**；注释注明 P13-jg，禁止框架默认 Tooltip 的断言原样保留，不加容差、不删断言。旧 B 断言没有改。修正后重新执行普通与 verbose 全量；首个失败日志保留。
