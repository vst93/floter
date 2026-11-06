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
