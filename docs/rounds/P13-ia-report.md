# P13-ia：异步入口随条件刷新消失时的键盘交接

开盘 a7f1fb39，pull 最新、工作区干净；当前最新交接为 P13-hz，未重做 hx–hz 已交付项。Codex 独立执行，无外部模型。done.log 开盘不存在；结案时新建。

## Stash 清点

八条 SHA 与已审核 backlog 相同，全部 reverse-check 退出 1；它们是早期独立 WIP，不是未接续的 stale-kill。按已归档处置保留，不覆盖新主题或恢复旧 Tauri。

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

## 增量记录

规格先于产品修改落盘。原确认在飞路径与普通退场已有覆盖，本轮转向保存、修复和启用的异步入口；以下只登记真实复现的缺陷。

1. 现象：键盘保存尚未返回时 owner 变 tool，保存按钮消失，Space 没有作用。改动：保存入口记住原焦点句柄，交给当前可用工具编辑入口。验证：深浅真实编辑、Tab、Space、owner 刷新、失败回调；原实现报 `pending save-owner lost keyboard outlet "Open configuration"`，且派发 0 次；用户预先移焦页头的对照通过。修后结果待本批日志。
2. 现象：同一保存中 schema 被清空，当前 Check 未接键盘。改动：沿用已存在的可用操作选择链，把保存入口交给 Check。验证：同事件与 schema 清空，断言当前 Check 的焦点和一次派发，不重复保存；修后结果待本批日志。

第一批相关 **74 PASS / 0 FAIL**，settingsui vet 退出 0。M1 撤回保存句柄，四个 entry 场景实跑红（owner/schema × 深浅），页头已持焦的对照仍通过。原串：`pending save-schema lost keyboard outlet "Check"`；恢复 MD5 与 touch 复验见 M1-restore.txt。浅色原图中“打开配置”原本没有键盘轮廓，修后出现完整 accent 环；没有改变布局。

第一批 **7b9008b6** 已推送。开盘全量基线结束：**4282 PASS / 0 FAIL / 2929 测试函数**，退出 0。

3. 现象：保存仍在飞时集成变 Orphan，host 表单撤下，Adopt 没接住键盘。改动：孤立集成分支也归还原保存/表单焦点。验证：深浅真实保存后刷新，旧实现报 `pending save-orphan lost keyboard outlet "Adopt"`；迟到失败不重开表单、不重复保存，用户已移到页头则保持。
4. 现象：保存期间按原契约可以继续编辑，用户回到 Token 字段后 owner 变 tool 且无编辑命令，保存按钮之外的焦点仍丢失。改动：记录整张 host 表单的 FocusWithin，把字段焦点也交给当前 Check。验证：真实 Shift+Tab 回字段再刷新；旧实现报 `pending save-unopenable lost keyboard outlet "Check"`，不是直接篡改焦点状态。草稿保持 edited，禁用“打开配置”不执行。

第二批相关 **84 PASS / 0 FAIL**、settingsui vet 退出 0；旧 V/B/S 断言零改。四项深浅前后图随本批保留，基线 pin 不修改。M1 已实跑，新增整张表单与孤立分支将在最终 M2 一并回注核验。

第二批 **d1f6d196** 已推送。修复“回调与健康刷新同时抵达”的候选实测全部通过，保留为 settled-repair-control.txt，不算 bug。缺陷只在刷新先到、回调仍未返回的真实顺序出现。

5. 现象：Recheck 在飞期间 Broken 已被刷新解除，入口消失，同行 Check 仍禁用，键盘无可用出口。改动：保留修复按钮句柄，在它本帧不再构建时选当前可用出口，全部忙时回页头。验证：深浅真实 Recheck → 健康刷新 → 两帧 → 延迟失败；原串 `pending repair-healthy lost keyboard outlet "Close settings"`。
6. 现象：Reconnect 在飞期间运行时已刷新可用，同样失去按钮与可用键盘位置。改动：重新连接的静息/忙态都绑定同一交接句柄，入口撤下时使用同一可用性检查。验证：深浅真实 Reconnect → runtime 可用刷新；原串 `pending reconnect-healthy lost keyboard outlet "Close settings"`。两项均保留页头原焦点，回调后不重复修复、不抢焦。

第三批相关 **94 PASS / 0 FAIL**，settingsui vet 退出 0；焦点交接立即完成，回调只结算原操作，不承担延迟抢焦。V/B/S 旧断言修改仍为 0/0/0。

第三批 **2f84f6d6** 提交完成；第一次 push 被 GitHub 断开 SSH，已原样重试，不重写提交。

7. 现象：修复仍在飞时集成变 Orphan，原按钮被整行撤下，Adopt 没接焦点。改动：孤立分支归还 repair 句柄。验证：深浅真实 Recheck → Orphan → 延迟失败，旧实现报 `pending repair-orphan lost keyboard outlet "Adopt"`；认领只在用户按 Space 后派发一次。
8. 现象：启用写入仍在飞时集成变 Orphan，开关消失、焦点悬空。改动：现有开关的正常/忙/禁用三态绑定同一个句柄，孤立分支交给当前 Adopt。验证：深浅真实关态开关 → Space 启用 → Orphan，旧实现报 `pending toggle-orphan lost keyboard outlet "Adopt"`；原启用只执行一次，迟到结果不抢用户页头焦点。

第四批相关 **263 PASS / 0 FAIL**、settingsui vet 退出 0。八项全部真实复现并修复，跨配置表单、修复按钮、启用开关。第三批重推已成功。最终全量、M2 和图像复核随后执行。

第四批 **132d72f1** 已推送。针对旧重复 ID 兼容性的复查实跑发现 `TestASettingsFrameCopiesTheSettingsOnce` 崩溃：`panic: ui: Handle bound to two controls in one build`。原因是新增句柄按 ID 无条件绑定；修正为只记录真正持焦的控件/表单，未聚焦同 ID 行不争用句柄。保存按钮的归属由整张表单统一覆盖。旧 B 断言零改，此为八项的兼容补修，不另计第九项；报告保留此前提交已推送的过程偏差。

兼容补修相关 **265 PASS / 0 FAIL**，包含原重复 ID 的 B 测试和本轮八项；settingsui vet 退出 0。最终实现只保留实际焦点持有者，字段、保存按钮、修复/开关都通过同样的旧句柄归属判断。

## 偷了什么

已按顺序读取 qiaomu-design / user-preferences / craft-loop / preflight / chinese-typography；另读 Apple DESIGN 的色彩与字体部分。采用用户已经指定的方向，不生成四方向 HTML、不调用外部模型。

- craft-loop 菜单 **#3 品牌 focus 环**：借 Apple 单一交互蓝的职责分配，保持本仓 accent；原 **2px** 环、**1px** 间隙交给实际可操作出口，新增颜色 **0 个**。
- **#4 签名交互**：刷新交接 **0ms**；自动派发 **0 次**，用户 Space 后当前出口 **1 次**；失败回调不延迟抢焦，原确认窗口 **3000ms** 不动。
- **#12 材质细节**：原 **1px** 边及 E1/E2 材质保留，新增玻璃和投影 **0 层**。用正确的焦点落点标明操作层，不给禁用面加高光。

为什么更轻、更有空间感：消失的入口立即把键盘交给当前可用层，用户不再面对看不见的焦点。既有功能修复优先，不为菜单五项硬加装饰；字号、圆角与主题色值均不重定基线。

## 图像复核过程

第一轮 before 捕捉真实失焦，after 使用相同事件。四视角记录：艺术总监——退场后的操作没有视觉落点；工程师——异步回调和状态刷新顺序不一致时入口已缺席；任务使用者——Space 无响应；读者——禁用编辑入口与可用 Check 并列时看不出键盘位置。第二轮集中复核“工具编辑、Check、Adopt、页头返回”四种落点，保留实际焦点环，无手工绘图。

## Preflight A / B / C / D2

| 项 | 实际结论 |
| --- | --- |
| A1 渐变/黑底/霓虹 | 无新增；保留既有主题 |
| A2 字体 | 保留系统中文字体，用户仓库约定优先于通用非系统字体建议 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置页；标题与任务入口保留 |
| A5 卡片海 | 无新增容器 |
| A6 假数据 | 无产品假指标，Workspace Tools 仅为测试夹具 |
| A7 空词/感叹号 | i18n 与产品文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原有单行页头 |
| A10 Bento | 不适用 |
| A11 左侧竖线 | 无新增装饰 |
| B1 读取/拨盘 | 3/2/7，克制、即时反馈、原密度 |
| B2 主题 | 每个 Tester 固定深或浅，分别检查 |
| B3 记忆点 | 退场后唯一可用键盘落点清楚 |
| B4 动效气质 | 0ms 交接符合高频键盘 |
| C1 目的/频率 | 无新增键盘动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原按压反馈、颜色过渡不改 |
| C4 origin | 沿用原操作位置；无新浮层 |
| C5 reduced-motion | 所有新增事件设置 ReduceMotion=true |
| C6 hover | Go 自绘，无 CSS 媒体查询；原指针路由保持 |
| C7 布局动画 | 无新增 |
| C8 keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新增动画，不声称真机慢放证据 |
| D2-1 系统字体 | 沿用，没有 webfont |
| D2-2 字号/行高/字重 | 旧 Caption 9.9、正文 10.8 与行盒未达通用 ≥14px/1.5；仍需拍板，不擅改 |
| D2-3 混排 | 产品文案零改，无新增混排问题 |
| D2-4 标点/对比 | 原标点与墨值保持，未声称所有旧次要墨达到 7:1 |

没有为满足通用网页门禁添加装饰、素材或网页布局。可复现问题归档到本仓 §6.202 和事件断言，不升级为全局 skill 偏好。

## 最终变异与图像证据

兼容补修 **04ff48f0** 已推送。M2 把三份产品源码回注开盘 a7f1fb39，同一最终测试八项全部红，16 个 entry 子场景失败，用户已移焦页头的对照保持通过。随后 `git apply` 还原、三文件 `md5sum -c` 全 OK、`touch` 后本轮 **40 PASS / 0 FAIL**。记录见 [M2-red](P13-ia/M2-red.txt)、[M2-restore](P13-ia/M2-restore.txt) 与 [源码指纹](P13-ia/M2-source.md5)。M1 为早期保存句柄缺失，四个 entry 场景红，恢复后 10 PASS；最终由 M2 覆盖兼容修正后的实现。

代表原始报错除逐项条目外，还有 `late save result changed keyboard outlet "Check"`、`outlet dispatch: open=0 check=0 adopt=0 close=0`，不是编译失败。两组 mutation 均实跑、还原与复验；旧 V/B/S 断言修改 **0/0/0**，旧测试函数未删，新增 Skip **0**。

最终 **32 张 720×900** ui.Tester 原图；before/after 使用同一最终测试，仅替换产品源码。对照板：[浅色表单](P13-ia/comparison-light-1.png)、[深色表单](P13-ia/comparison-dark-1.png)、[浅色操作](P13-ia/comparison-light-2.png)、[深色操作](P13-ia/comparison-dark-2.png)。[像素差](P13-ia/image-differences.txt) 逐对非零；代表原图另在 `/tmp/floter-P13-ia-pending-{dark,light}.png`。来源是 Linux 软件自绘，其他平台只有编译证据。

逐图目视：①工具编辑胶囊出现完整蓝环；②schema 清空后 Check 环仍在；③保存变孤立时 Adopt 接环；④禁用工具编辑保持灰淡，环落在 Check；⑤Recheck 提前退出后页头返回出现环；⑥Reconnect 同样回到页头；⑦旧修复行撤下后 Adopt 持环；⑧忙态启用开关撤下后 Adopt 持环。深浅各一套一致，页头 tooltip 是既有焦点提示。

## 仍待

1. 下一轮首选：异步操作的目标整行消失、随后以相同 ID 重现时，旧焦点句柄是否串到新行；以及保存期间 host 表单主动折叠/重开后的键盘位置。这里只是代码线索，先事件复现，不重复本轮仍存在目标的 owner/schema/Orphan 刷新。
2. 不改变错误呈现期限、保存时机与保存中草稿归属。176-9/161-4、字号/行盒、accent 禁用填充、未知多选/Radio、小步长六位存储、终端 S1b/S1c/S2 尾继续需拍板。
3. Windows/macOS 没有真机交互证据；早期八条 WIP 原样保留。参数全部移除再恢复仍为未复现线索。

## 完整门槛与流程记录

最终产品 04ff48f0：gofmt 输出 0 字节，Linux/Windows/macOS vet 全部退出 0。普通全量退出 0（settingsui 575.421 秒；开盘基线同包 329.629 秒），verbose 全量计数及三平台 build 仍在执行；不将等待状态记作通过。每批提交前运行相关事件与包 vet，完整门槛在最终产品集中执行。

流程偏差：40 次工具调用预算未能守住，额外调用用于定位并补修重复 ID 的既有 B 回归、等待完整验收和收尾；发现预算超额后停止扩题。重复 ID 崩溃在第四批推送后被定向复查捕获，已由 04ff48f0 补修；没有修改旧断言、隐去失败或将补修算作第九项。第三批推送一次 SSH 断开，原样重推成功。

## 最终结果

最终产品 **04ff48f0**，四批每两项提交推送 **7b9008b6 / d1f6d196 / 2f84f6d6 / 132d72f1**，随后独立兼容补修。普通与 verbose 全量均退出 **0**；最终 **4322 PASS / 0 FAIL / 2937 测试函数**，对比基线 **4282 / 0 / 2929**，增加 **40 PASS / 8 函数**。三平台 vet/build 全部退出 0，gofmt **0 字节**；旧 V/B/S **0/0/0**，新增 Skip **0**。

两组 mutation 真实变红，MD5 逐字节还原后 touch 复跑 **10/40 PASS**；最终相关含重复 ID B 回归 **265 PASS / 0 FAIL**。八项都有同事件深浅 before/after，16 对像素变化均非零，最小 **420 像素**。Linux 软件自绘已验，Windows/macOS 仅 vet/build。

原始记录：[门槛汇总](P13-ia/gates.txt)、[普通全量](P13-ia/full-test.txt)、[开盘基线](P13-ia/baseline.txt)、[最终源码指纹](P13-ia/final-source.md5)。完整 verbose 和平台原始输出在 `/tmp/floter-P13-ia-gates/`；单项说明、stash 表、工艺取值和逐条 preflight 均见上文。

验收完成时间 **2026-11-06 18:58:24 UTC**；从开盘记录 18:20 UTC 至此约 **38 分钟**，低于 45–75 分钟目标，不追加无关改动填时长。八项与完整门槛均完成；本轮存在调用超预算与提交后兼容补修的过程偏差，已如实记录。下一轮队列已替换待办顶部，done.log 记录最终结案 SHA。
