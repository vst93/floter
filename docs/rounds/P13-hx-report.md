# P13-hx：确认条件刷新后的出口

开盘 fe52ed35，pull 已最新、工作树干净。中文 locale 基线实测 4171 PASS / 0 FAIL / 2904 函数。独立 Codex 执行；本轮不调用外部执行器。

## Stash 清点

逐条导出 patch 到 /tmp/floter-P13-hx 并 reverse-check；八条 SHA 与 P13-hw 完全一致，均退出 1。已接续的 stale-kill 均不在活动清单，以下按 backlog 的语义审核结论保留，不能用旧主题与 Tauri 链覆盖现产品。

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

规格八个候选逐批事件复现，只有已复现修复的条目计数。初次事件夹具点击文字 label 未聚焦输入，尚不作为产品缺陷；校正事件命中后再验证。

### 第一批：丢弃问题的 dirty 与 schema 条件

1. 现象：编辑 Token 后提问，外部存储变为同值；带子隐藏但 Escape 被吞，再编辑会复活旧问题。改动：View 路由键盘前同步已有 draft 的外部值，若已无差异就 disarm，保留编辑面。验证：深浅各 return/escape 事件；Configure 焦点恢复，新输入保留，关闭 1 次或旧问题不复活，操作 0 次。
2. 现象：问题展开期间 schema 清空，集成仍在；Escape 被吞，恢复 schema 后旧问题重现。改动：无字段时撤问题与时钟，保留原 draft/configOpen。验证：深浅四场景，恢复后 Token 仍为 edited，旧确认不出现。

M1 回注旧条件八场景实跑红：`discard question survived changed clean condition`、`clean draft lost its configuration opener focus`、`invalid discard question swallowed Escape: close=0`、`discard question revived when schema condition returned`。MD5 恢复 OK，touch 后 10 PASS。V/B/S 旧断言修改 0/0/0；未新增 Skip。

第一批相关回归 210 PASS / 0 FAIL、包 vet 净；6bf0242d 已提交推送。浅色第一次目视：第二次输入 second edit 仍在原字段，旧丢弃带撤走，输入焦点不再被 Cancel 夺走。

### 第二批：owner 与 orphan 分支

3. 现象：host 配置切到 tool owner 后旧丢弃问题藏在外部配置入口后面，Escape 只取消看不见的问题；切回 host 后又问。改动：按 host 配置块的实际可见条件撤旧问题，保留草稿。验证：深浅 × 返回/退出四场景，edited 保留、操作 0 次。
4. 现象：已提问移除的目标刷新成 orphan，维护入口提前返回，但问题仍阻挡 Escape，恢复登记后复活。改动：清除未派发的移除问题、时钟与错误；在飞移除仍交原回调结算。验证：四场景，退出 1 次或恢复后问题为关，不派发移除。

M2 回注旧条件八场景红：`discard question survived changed owner condition`、`remove question revived when orphan condition returned`、两类 `invalid … question swallowed Escape: close=0`。MD5 恢复 OK，touch 后相关回归 220 PASS / 0 FAIL，包 vet 净。旧 V/B/S 0/0/0。

第二批 fae5f208 已提交推送。

### 第三批：停用前提不再成立

5. 现象：停用问题展开时，外部状态已经 Disabled，旧确认仍可见可答。改动：未派发的问题随 Enabled=false 退役，清时钟与旧错误。验证：深浅 × 返回/退出四场景，刷新不派发动作，目标再次启用不带旧问题。
6. 现象：目标变 Broken 后开关已不可操作，旧停用答案却仍可用。改动：沿用开关的可用条件撤旧问题；同时覆盖 orphan 的同源提前返回。验证：四场景，Escape 关闭设置 1 次，恢复正常后不复活旧停用答案。

M3 回注旧条件八场景红：`disable question survived changed disabled condition`、`disable question revived when broken condition returned`、`invalid disable question swallowed Escape: close=0`。MD5 恢复 OK，touch 后相关 230 PASS / 0 FAIL，包 vet 净；旧 V/B/S 0/0/0。

最初整包运行编译的是尚未校正的输入夹具，最终仅两条新测试的八场景失败（Token 未命中）；原 B 测试均无失败。校正后的相关测试已绿，最终普通/verbose 全量将使用最终代码重跑，不把该次失败计作成功。

第三批 f3970015 已提交推送。

### 第四批：组件卸载条件与未提交选择

7. 现象：组件卸载浮层打开后目标变 orphan；浮层虽隐藏，Escape 被吞，恢复登记后旧浮层连同已取消的 Tool data 勾选复活。改动：未派发时撤浮层、旧错误与未提交的勾选草稿。验证：真实 Click/Tab/Space 打开浮层并取消一项，深浅 × 返回/退出四场景；问题和草稿都退役，卸载调用 0 次。
8. 现象：npm 包刷新成普通本地链接，已经不走组件卸载，但旧浮层还允许选择数据类别。改动：按 componentizedRemoval 当前条件撤不再适用的浮层及草稿。验证：相同四场景；恢复 npm 后旧浮层不复活，Escape 正常退出。

M4 回注旧条件八场景红：`uninstall question survived changed kind condition`、`invalid dialog retained abandoned component choices`、`uninstall question revived when orphan condition returned`、`invalid uninstall question swallowed Escape: close=0`。MD5 恢复 OK，touch 后相关 **247 PASS / 0 FAIL**，包 vet 净。包含额外六场景：停用/移除/组件卸载已派发时刷新不取消 busy，迟到失败仍可读、调用恰好 1 次。旧 V/B/S 0/0/0，无新增 Skip。

第四批 ae37c102 已提交推送，远端确认 f3970015 → ae37c102。四批产品每批两项，未攒到轮末。

## 偷了什么

真实读取 SKILL、user-preferences、craft-loop、preflight、chinese-typography；另读 Apple DESIGN 的色彩与控件规则。沿用户已经指定的方向直接打磨，由当前 Codex 完成，不走四方向选择，不调用 K3。

- craft-loop 菜单 **#3 品牌 focus 环**：Apple 单一交互蓝映射本仓 accent，沿用 **2px** 焦点环；草稿变干净时 **0ms** 返回 Configure，继续编辑时新输入保持焦点。
- **#4 签名交互**：有效问题沿用 **3000ms**；条件失效在下一次 View 路由键盘前撤回，刷新派发 **0 次**，在飞对照只派发 **1 次**。
- **#6 编辑细节**：删除失效提示，没有增加常驻成功消息；新提示占高 **0px**，schema/owner 返回仍保留 edited。
- **#12 材质细节**：Apple 的 **8px** 控件圆角与 **1px** 边对应既有面规则；E2 浮层失效时整层撤回，新增透明面/阴影 **0 层**。

具体借的是色彩角色、焦点宽度和控件圆角刻度；没有照搬 Apple 的纯黑、17px 字号或网页负字距。用户已有合约优先，不为工艺密度凑第五项添装饰。

**这轮为什么更轻、更有空间感**：不再让失效警告带占据表单高度，也不让失效的卸载浮层压住当前内容；保留的每一层都对应当前可完成的任务。

## 两轮图像复核

第一轮从真实事件捕获旧分支 before：艺术总监视角看见配置问题在新编辑时重新争抢注意力；工程师视角看见显示条件已变而布尔状态未变；任务用户遇到 Escape 被看不见的问题吞掉；读者看到恢复后的旧卸载选择遮住工具状态。第二轮集中修正四个组件的有效性条件后，同事件重拍 after。

浅色整图可见新输入 second edit 的蓝色环完整保留，旧问题收回后动作行上移；4× 最近邻图核对输入边与圆角连续，没有新增阴影。深色卸载对照可见失效浮层与全屏 scrim 一起撤走，Workspace Tools 回到原 E1 层。4× 是局部放大检查，不是高 DPI 真机证据。

八项各有深浅 before/after，共 32 张原始 720×900 PNG；每组像素差均非零，最小 **22774 像素**。图例展示的是条件恢复后的同一帧，验证旧问题不会复活。

| 项 | 图上看到的变化 | 浅色 | 深色 |
| --- | --- | --- | --- |
| 1 变干净后再编辑 | second edit 焦点保留，旧丢弃带撤走 | [对照](P13-hx/discard-clean-light-comparison.png) | [对照](P13-hx/discard-clean-dark-comparison.png) |
| 2 schema 返回 | Token 草稿保留，下方不再插旧丢弃带 | [对照](P13-hx/discard-schema-light-comparison.png) | [对照](P13-hx/discard-schema-dark-comparison.png) |
| 3 owner 返回 | 配置回到 host 后只显示当前表单 | [对照](P13-hx/discard-owner-light-comparison.png) | [对照](P13-hx/discard-owner-dark-comparison.png) |
| 4 登记恢复 | 工具行下的旧移除警告带撤走 | [对照](P13-hx/remove-orphan-light-comparison.png) | [对照](P13-hx/remove-orphan-dark-comparison.png) |
| 5 重新启用 | 开关下不再保留旧停用问题 | [对照](P13-hx/disable-disabled-light-comparison.png) | [对照](P13-hx/disable-disabled-dark-comparison.png) |
| 6 恢复正常 | 损坏期间失效的停用带不复活 | [对照](P13-hx/disable-broken-light-comparison.png) | [对照](P13-hx/disable-broken-dark-comparison.png) |
| 7 卸载登记恢复 | 旧浮层与 scrim 撤走，工具行完整露出 | [对照](P13-hx/uninstall-orphan-light-comparison.png) | [对照](P13-hx/uninstall-orphan-dark-comparison.png) |
| 8 类型恢复 | 旧组件勾选浮层不重新覆盖页面 | [对照](P13-hx/uninstall-kind-light-comparison.png) | [对照](P13-hx/uninstall-kind-dark-comparison.png) |

[总览](P13-hx/overview.png)、[浅色局部](P13-hx/focus-detail-light.png)、[深色局部](P13-hx/focus-detail-dark.png)、[像素差](P13-hx/image-differences.txt)。Linux ui.Tester 软件自绘，Windows/macOS 没有真机交互证据。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 紫蓝渐变/纯黑/霓虹/渐变字 | 无新增，沿用原近黑主题 |
| A2 字体 | 中文系统字体与本仓合约优先，不换网页字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置面 |
| A5 卡片海 | 无新增卡片或包装 |
| A6 假数据 | 无产品假指标；Workspace Tools 是事件夹具 |
| A7 空词/感叹号 | 产品文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 行数 | 原页头完整可见 |
| A10 Bento | 不适用 |
| A11 左侧装饰竖线 | 无新增 |
| B1 读取与拨盘 | 3/2/7：克制、即时、保留原密度 |
| B2 主题锁定 | 每个 Tester 明确指定深/浅，无混主题 |
| B3 记忆点 | 当前可回答的问题才占据页面和键盘 |
| B4 动效气质 | 高频键盘反馈 0ms，未加动画 |
| C1 目的/频率 | 条件刷新即时反馈 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原颜色过渡 120ms、按压 0.97 保留 |
| C4 origin | 内联确认仍紧随原工具，modal 保留中心 |
| C5 reduced-motion | 新事件全部 ReduceMotion=true |
| C6 hover 查询 | Go 自绘不适用 CSS 媒体查询，原指针路由保留 |
| C7 布局动画 | 无新增，只有失效内容收回 |
| C8 重播 keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新增动效，没有声称慢放真机验收 |
| D2-1 系统字体 | 保留，未引 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px/1.5 未满足；旧 Caption 9.9/正文 10.8 与行盒继续待拍板 |
| D2-3 混排空格 | i18n 零改，无新增中文混排句 |
| D2-4 标点与暗色对比 | 沿用原墨与标点，不宣称旧次要墨全部满足 7:1 |

自发现缺陷写入本仓 §6.199 与事件 pin，不把一次性修复升级成全局 skill 审美禁令。用户限定原生自绘、既定方向与行为红线优先于浏览器/多方案/字号等通用规则。

## 仍待

1. 下一轮首选：异步结果到达时目标仍处于变化后的条件，错误反馈与键盘出口是否一致；本轮在飞对照在结果返回前恢复有效条件。仅代码线索，先事件复现，不改变回调写回或会话语义。
2. 参数全部移除再恢复的会话记忆仍是未复现线索；保存期间草稿归属 176-9/161-4、未列举多选值、字号/行盒、accent 禁用填充、终端 S1b/S1c/S2 尾继续需拍板。
3. 八条早期 WIP 仍按清点表保留，无新增抢救稿；其他平台本轮只有 vet/build。

## 完整门槛与交接

最终产品 **ae37c102**。正式普通与 verbose 全量均退出 **0**；**4218 PASS / 0 FAIL**（基线 4171，+47），测试函数 **2913**（基线 2904，+9）。gofmt 空，linux/windows/darwin vet/build 全部退出 0。最终两个源码文件 MD5 核对一致；旧测试未修改，V/B/S 0/0/0，无新增 Skip。

四批按相关事件与包 vet 通过后提交推送；本轮完整门槛随后全部通过。首次整包运行仅包含尚未校正的两条新输入夹具失败，保留原日志；最终校正后完整普通与 verbose 全量重跑通过，不把那次失败隐去。

证据：[门槛汇总](P13-hx/gates.txt)、[普通全量](P13-hx/full-test.txt)、[最终源码 MD5](P13-hx/final-source.md5)、[初次夹具失败](P13-hx/initial-fixture-failure.txt)、[M1 红](P13-hx/M1-injected-red.txt)、[M1 恢复](P13-hx/M1-restored.txt)、[M2 红](P13-hx/M2-injected-red.txt)、[M3 红](P13-hx/M3-injected-red.txt)、[M4 红](P13-hx/M4-injected-red.txt)、[最终相关回归](P13-hx/batch4-regression.txt)。完整 verbose、基线、三平台原始输出在 `/tmp/floter-P13-hx/`；代表图 `/tmp/floter-P13-hx-confirmations-{light,dark}.png`。

清点文件时间 15:39 UTC，完整门槛 16:03 UTC 左右结束，实作约 25 分钟，低于 45–75 分钟目标；八项均有真实事件、深浅图和变异证据，不重复跑已绿门槛或加无关改动填时间。进度替换为本轮结果，下一轮沿“异步结果在失效条件下到达”的线索继续，先复现。
