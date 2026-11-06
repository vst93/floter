# P13-if · 回调与表单刷新后的结果归属

开盘分支 mygo-rewrite，HEAD d89ef7ec，pull 最新、工作树干净。首个记录时钟 2026-11-06 22:26:35 UTC。实际承接 P13-ie；done.log 开盘不存在。

## Stash 清点

与 backlog 和 P13-ie 的 SHA 一致，无新增 stale-kill。八条反向检查退出 1；已审核的旧独立 WIP 继续保留，不重入旧主题/Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 验证记录

基线运行中；旧 V/B/S 修改 0/0/0，无新增 Skip。后续逐批追加实测记录。

## 第一批（1–2）

1. 现象：点击运行，回调原地把 next 选项换成 fresh 后仍显示旧约束下的拒绝。改动：参数选项在派发之前复制，后续刷新识别条件变化。验证：真实按钮点击，深浅 fresh 已出现、旧拒绝消失，运行总次数 1。
2. 现象：运行回调已把答案 old 修正成 next，旧拒绝却绑定 next。改动：提交答案保存独立快照，既不读取回调改过的 live map，也不读取回调拥有的参数 map。验证：深浅真实点击，next 保留，旧拒绝消失。

相关事件 **82 PASS / 0 FAIL**，settingsui vet 退出 0。M1 撤回两项产品修改，深浅四叶子实红：`options callback left obsolete refusal on current answer` / `value callback left obsolete refusal on current answer`；恢复后 MD5 OK，touch 后 82 PASS。目视浅色前后：fresh 已在选择器中，修后底部过期红字消失，操作行位置不变。

## 第二批（3–4）

3. 现象：参数全部移除再恢复，字段重新出现时旧运行拒绝也复活。改动：集成行仍让 runForm 处理退场，空表单清理拒绝与显示错误，答案记忆不动。验证：真实提交失败 → 空参数帧 → 恢复；深浅无旧红字，remembered 答案保留，未自动运行。
4. 现象：RunWithParams 撤回再接上，恢复入口仍携带之前的失败。改动：无运行回调时同步退役拒绝。验证：真实提交失败 → 回调撤回 → 恢复；深浅无旧红字且答案不丢。

相关事件 **88 PASS / 0 FAIL**、settingsui vet 退出 0。M2 四叶子实红：`empty form resurrected retired refusal` / `unwired form resurrected retired refusal`；MD5 OK、touch 后 88 PASS。第三批测试初稿误用 KeyReturn 导致一次编译失败，修正为 KeyEnter 并暂存该未完成测试后复跑；未将编译失败算为 mutation。

## 第三批（5–6）

5. 现象：同一 Config 对象在写回调里换 owner 和值，旧返回仍回滚或覆盖新 owner 的值，甚至显示旧失败。改动：返回核对派发 owner 与当前 schema 是否存在。验证：真实开关点击，成功/拒绝 × 深浅四场景保留新 owner 的 false/25，不显示旧失败。
6. 现象：同 key 改型、移除、数字边界或选项值原地更新后，旧写返回覆盖新字段的值。改动：派发前复制字段选项，返回核对 Kind、Min/Max/Step、Options 值；标签更新不作为失效条件。验证：开关、数字输入及下拉真实事件，四种刷新 × 成功/拒绝 × 深浅，旧结果不进入新字段；改型后再输入提交以核对输入缓冲。

M3 撤销产品核对，20 个深浅/返回叶子实红；原串 `new owner values overwritten`、`new field kind values overwritten`、`removed field resurrected by result`、`new bounds values overwritten`、`new options values overwritten`；MD5 OK、touch 复验。夹具曾把数字标签当输入入口，调整到实测输入 x=660；HasText 不收集 TextInput 缓冲，改用输入/提交事件核对实际内容，未弱化旧断言。目视深色前后：改型输入由空白恢复“当前配置”，上限 10→25，错误提示退场。

开盘完整实测 **4541 PASS / 0 FAIL / 2979 测试函数**；普通/verbose 最终全量尚待轮末运行。

第三批最终相关事件 **155 PASS / 0 FAIL**、settingsui/launcher vet 退出 0。三个产品提交均已推送：7036e3d2、6b096ccf、129c9b94。

## 第四批（7–8）

7. 现象：写回调内的新写已成功，外层旧请求返回拒绝，却把新值 false/25 回滚为 true/10 并报错。改动：每次派发递增写入代次，旧代次返回不写当前表单。验证：真实开关点击触发嵌套写入；深浅保留 false/25，无旧失败，严格两次派发。
8. 现象：回调内的新写被拒绝并回滚为 false/10，外层旧成功返回却覆盖成 true/80，失败提示与值不再对应。改动：同一代次核对也保护更新拒绝的回滚值。验证：深浅保留 false/10 与当前失败提示，严格两次派发。实测失败标记原本仍在，未把它虚报成“错误被清掉”。

M4 四叶子实红：`older-refused result overwrote newer write: values=map[enabled:true format:text limit:10] failed=true`；`older-accepted result overwrote newer write: values=map[enabled:true format:text limit:80] failed=true`；恢复后 MD5 OK，touch 复验。

第四批相关事件 **161 PASS / 0 FAIL**、settingsui/launcher vet 退出 0。旧 V/B/S 修改 0/0/0，无新增 Skip；四批每批两项提交推送。

## 偷了什么

- craft-loop 菜单 #3 品牌 focus 环：采用 Apple 供体单一交互蓝色的职责，保留本仓已有 2px focus 环；新增焦点目标 0 个，改型后的文字输入实际聚焦并续写。
- 菜单 #4 签名交互：刷新 0ms，自动提交 0 次；旧回调结果不会添加一次隐含写入。原两次动作确认与 3000ms 窗口保持。
- 菜单 #12 材质细节：Apple 的克制圆角与少层级落到已有 8px 控件半径、1px 受光边，新玻璃/投影 0 层。只移走已经失效的红字，不新增面。

工艺密度采用三项，未为凑五项扩张范围；用户 bug 主线优先。单一来源仍为 liquid-glass，不另建 DESIGN.md、不生成 HTML、不调用外部模型。

## 两轮看图

第一轮四视角：艺术总监——过时红字夺走主操作注意；工程师——选项切片与 live map 在回调后已改；用户——已看到新字段却收到旧结果；读者——错误与当前值矛盾。第二轮只修结果归属。已目视浅色选项前后及深色字段改型前后：前者撤掉过时红字、操作位置不变；后者输入恢复“当前配置”，上限保持 25，旧失败退场。最终深浅对照板待归档。

深浅对照已归档并目视复核：[深色](P13-if/comparison-dark.png) / [浅色](P13-if/comparison-light.png)，从上至下八项，左 before、右 after。每套四个运行场景各撤掉 875 个错误像素；owner 与旧拒绝场景分别改变深 64323 / 浅 64332 像素（错误行退场让卡回到原位）；改型深 83898 / 浅 83888；旧成功场景深 638 / 浅 703，保留失败提示而把 true/80 恢复为 false/10。16 对图均有实际变化，原始计数见 `P13-if/pixel-diff.txt`。代表图 `/tmp/floter-P13-if-form-{dark,light}.png`。

为什么更轻、更有空间感：提示只描述当前表单，失效红字退场后原有字段、操作与留白层级恢复；新增面、玻璃、阴影均为 0。

## Preflight（逐条）

| 项 | 结论 |
| --- | --- |
| A1 渐变/霓虹/纯黑 | 无新增，保留主题 token |
| A2 字体 | 系统中文优先，既有字体保留；不按通用反默认字体条款换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 原生表单，无 Hero，不适用 |
| A5 卡片海 | 新增卡片 0 |
| A6 假数据 | 无产品假指标，测试值仅为事件夹具 |
| A7 空词/感叹号 | i18n 零改 |
| A8 眉批 | 无新增 |
| A9 标题 | 既有层级与宽度保持 |
| A10 Bento | 不适用 |
| A11 装饰竖线 | 无新增 |
| A2 工艺密度 | #3/#4/#12；按用户范围不凑五种装饰 |
| B1 设计读取 | 克制 3/动效 2/密度 7，紧凑原生表单 |
| B2 主题 | 深浅分别自洽 |
| B3 记忆点 | 结果只归发出它的表单/字段/请求 |
| B4 动效 | 即时状态刷新，无装饰动效 |
| C1 目的/频率 | 高频输入 0ms，新增动画 0 |
| C2 all/ease-in/scale(0) | 无新增 |
| C3 时长/按压 | 既有反馈保持，无新时长 |
| C4 origin | 未新增浮层 |
| C5 reduced-motion | ui.Tester ReduceMotion=true 已覆盖 |
| C6 hover | Go 框架指针行为，CSS 媒体条件不适用 |
| C7 属性 | 无布局属性动画 |
| C8 keyframes | 无新增 |
| C9 手势 | 未改手势，不适用 |
| C10 Framer | Go 自绘，不适用 |
| C11 逐帧 | 前后帧状态验证；无新时序动画 |
| D2-1 字体 | 系统中文，无 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px/≥1.5 与既有高度链冲突，沿用需拍板；未宣称全过 |
| D2-3 混排空格 | 产品文案零改 |
| D2-4 标点/对比度 | 标点零改；既有安静墨不承诺通用 AAA，未虚报重测 |

Linux 自绘事件与截图；Windows/macOS 只做编译/vet，没有真机交互证据。

## 完整门槛进度

四批产品已推送：7036e3d2 / 6b096ccf / 129c9b94 / 60f27987。三平台 vet/build 均退出 0，gofmt 空；使用短磁盘临时目录 `/home/tar/.fif`，避免上一轮的内存盘耗尽和 Unix socket 路径过长。普通与 verbose 全量串行运行；未计未结束的 PASS 为最终数字。

本轮夹具调试增加了工具调用；进入收尾后不再扩题，只完成必要门槛和交接。第三批曾在前一次定向测试收尾时重新编译复跑，最终以独立 `batch3-final.log` 的 155 PASS、退出 0 为准；无失败门槛被当作通过提交。更新后的八项产品全部落盘，旧行为测试未改。

普通全量 `go test -count=1 ./...` 已退出 **0**；新合约表补充 `internal/ci` 检查退出 **0**。verbose 全量继续执行。调用数已超过 40，记录为过程偏差；后续不扩题，只完成已启动门槛和最终交接。

## 最终验收

- 完整基线 **4541 PASS / 0 FAIL**；最终 verbose **4585 PASS / 0 FAIL**，增加 44，退出 0；普通全量同样退出 0（settingsui 432.233s）。
- 测试函数 **2979 → 2987**，新增 8；旧 V/B/S 断言修改 **0/0/0**，新增 Skip **0**。
- gofmt -l cmd internal 空；Linux/Windows/macOS 的 vet 与 build 六项全退出 0；新增合约表的 ci 补验通过。
- 四组 mutation 的 4/4/20/4 个叶子实红，恢复产品后 MD5 OK、touch 定向复验。最终产品 MD5 与全量启动时一致。
- 四批每两项提交并推送：`7036e3d2` / `6b096ccf` / `129c9b94` / `60f27987`；最终证据提交单独收尾。
- 原始普通/verbose、基线、四批回归、四组变异、三平台门槛日志与深浅图均保存在 `P13-if/`。Linux 自绘已验，其他平台没有真机证据。

首个记录时钟 22:26:35 UTC，最终全量退出时钟 22:55:59 UTC；约 30 分钟完成八项与门槛，短于 45–75 分钟目标。工具调用超过 40 是过程偏差，未用增加选题填时长，也未将未完成的全量冒充通过。

## 仍待

1. 下一轮首选运行表单在同步回调内关闭/重开或替换整个参数列表后的同帧归属；本轮只复现并完成选项/值快照、可见帧的空表单/撤回退役。先事件复现，不把线索当缺陷。
2. 需拍板沿用：字号/行盒、accent 禁用填充、未知多选值/Radio、176-9/161-4 保存中草稿、极小数字步长、终端 S1b/S1c/S2 尾。
3. 八条旧 WIP 保留；无新增系统、依赖或页面。
