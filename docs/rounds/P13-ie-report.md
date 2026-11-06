# P13-ie · 表单拒绝与动作确认的条件刷新

开盘 2cfe705d，pull 最新、工作树干净。实际承接 P13-id。首个时钟 21:44:03 UTC。done.log 开盘不存在，结束创建。已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple 供体。

## Stash 清点

八条均与交接账本及上轮 SHA 一致，没有新增 stale-kill；reverse-check 全部退出 1。它们是已审核的独立旧 WIP，不能据反向应用失败覆盖当前主题或 Go 发布链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap；reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider；reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局；reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段；reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段；reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键；reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏；reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri；reverse=1 | 保留 |

## 验证记录

完整基线正在运行；后续只写实测数字。旧 V/B/S 断言暂为 0/0/0，无新增 Skip。

## 第一批（1–2）

1. 现象：运行拒绝后改正 Target，旧红字仍说参数无效。改动：拒绝记录绑定提交值；该值改变后退役，编辑 Note 保留 Target 的错误。验证：真实点击运行、点击输入、Cmd+A/输入；深浅均通过，刷新派发 0 次。
2. 现象：被拒绝的 Target 移除后，底部仍责备不存在的字段，重加还会复活。改动：拒绝随该参数值槽退役。验证：真实运行失败后移除/重加，错误消失且默认值恢复；深浅通过。

定向回归 **65 PASS / 0 FAIL**，settingsui vet 退出 0。M1 用 HEAD 原产品注入两项缺陷，四个深浅叶子均红；原串 `edited parameter left obsolete refusal visible`、`removed parameter left obsolete refusal visible`、`re-added parameter resurrected refusal`。cp 还原后 `md5sum -c` 为 OK，touch 后以上 65 PASS。最初测试夹具有语法及辅助函数重名，两次编译失败均修正后才取得实红，未把编译失败算 mutation。

目视浅色 before：输入已经是 fixed，底部仍有红色旧错误；修复只撤掉失效反馈，不改变输入框和操作行。

## 第二批（3–4）

3. 现象：同一参数从自由文字改为可选值，原来的后台拒绝仍挂在新控件下面。改动：拒绝快照保存类型、必填、参数旗标与选项副本；校验条件变化即退役。验证：运行拒绝 → 原值仍在的 Select schema → 不留旧错、不自动运行，深浅通过。
4. 现象：同一参数改名后，错误仍点名已不存在的 Target。改动：保存结构化拒绝，按当前字段标签与语言重新解释，保留错误本身。验证：Target → Destination 后只出现当前名称的拒绝，深浅通过。

定向回归 **71 PASS / 0 FAIL**，settingsui vet 退出 0。M2 撤回这批产品后四叶子实红：`constraints parameter left obsolete refusal visible`、`renamed parameter left obsolete refusal visible`、`renamed parameter lost current refusal wording`；MD5 OK、touch 后以上 71 PASS。旧 V/B/S 0/0/0。

## 第三批（5–6）

5. 现象：已问“确认清理”后，schema 把范围改成“包括收藏”，同 key 仍处于已确认第一步的状态。改动：确认绑定 plugin/key/label/help/confirm 的值副本，刷新或派发前不匹配即退役。验证：真实行点击打开确认，原地刷新问题，回到首按入口、自动执行 0 次；深浅通过。
6. 现象：动作拒绝后更新范围，同一行仍显示旧动作的失败。改动：拒绝也绑定动作定义，定义变化退役。验证：行点击 → 确认点击 → 拒绝 → 刷新范围，旧失败消失、执行总数仍为 1；深浅通过。

定向回归 **141 PASS / 0 FAIL**，settingsui/launcher vet 退出 0。M3 四叶子实红：`changed question inherited old confirmation`、`changed action inherited old refusal`；MD5 OK、touch 复验通过。初版完整启动器夹具 Tab 留在搜索入口，未到动作，已改真实行点击；该夹具失败不算变异证据。目视深色 after：新范围说明下只剩“清理历史”首按胶囊，旧问题与取消按钮退场，无新增面。

开盘全量 **4507 PASS / 0 FAIL**，起点 2cfe705d 测试函数 **2970**。基线 shell 收尾统计一度读到已新增的函数数 2974，该值不是起点；以 git grep 起点树实测 2970 为准。

## 第四批（7–8）

7. 现象：动作回调中更新同 key 的范围，再返回拒绝，新动作却显示旧失败。改动：返回时核对派发前动作定义，结果只归原定义。验证：真实确认点击触发回调内刷新，界面显示新范围，旧失败不进入；深浅通过。
8. 现象：动作回调已发起新一轮确认，旧拒绝仍写进同一行；取消新问题后会露出旧失败。改动：确认记录递增代次，返回只写到原代次。验证：真实确认 → 回调重开问题 → 点击取消，旧失败不复活；执行次数严格为 1，深浅通过。

M4 撤回返回归属校验后四叶子实红：`replaced action inherited callback's obsolete refusal`、`rearmed action inherited callback's obsolete refusal`；MD5 OK、touch 复验。定向回归 **147 PASS / 0 FAIL**，两包 vet 退出 0。V/B/S 0/0/0；每两项独立提交推送，不攒到轮末。

## 偷了什么

- craft-loop 工艺密度菜单 **#3 品牌 focus 环**：保留输入处已有 **2px** 环与 **1px** 间隙，错误退役不抢焦；新焦点目标 **0 个**。Apple 的单一交互色职责落到既有 accent。
- 菜单 **#4 签名交互**：条件刷新 **0ms**、自动运行 **0 次**；确认仍是 **2 次**明确操作。高频编辑不加动画，3 秒问题期限不改。
- 菜单 **#12 材质细节**：保留 **1px** 受光边与半高药丸，新玻璃/阴影各 **0 层**。Apple 的减法转成删除无效提示与旧问题，而不新增装饰或浮层。

这三项服务本轮恢复路径；未为工艺密度凑满五项而扩张设计。单一来源仍是 liquid-glass.md，不另建 DESIGN.md。用户已指定方向，不走四方向预览或外部模型。

## 两轮看图

第一轮四视角：艺术总监——无效红字与橙色失败夺走主操作注意；工程师——状态只记 key/字符串，无法区分旧条件；使用者——已改正或已取消仍看到旧错；读者——名称刷新后错误指向不存在的标签。第二轮只修反馈归属与确认归属，未加布局包装。

已目视深浅对照板及浅色修正前后、深色确认退役、浅色回调取消后的原图。具体变化：修正值/移除字段/条件刷新三类各少 **875** 个旧错误像素；改名替换 **1317** 像素；确认退役 **2427** 像素；三类动作失败退役各 **503** 像素。16 对图无零差，原控件及标题位置保持。

32 张 ui.Tester 原图与 [深色对照](P13-ie/comparison-dark.png) / [浅色对照](P13-ie/comparison-light.png) 已入轮次目录；左 before、右 after，从上至下为八项。代表图 `/tmp/floter-P13-ie-form-{dark,light}.png`。像素原始计数见 [pixel-diff.txt](P13-ie/pixel-diff.txt)。

为什么更轻、更有空间感：反馈只描述当前可操作内容，过时的错误和问题不再占据同一层；新增面、玻璃、投影都是 0。

## Preflight

| 项 | 结论 |
| --- | --- |
| A1 渐变/霓虹/纯黑 | 无新增，沿用主题 token |
| A2 字体 | 原系统字体不改，无 webfont |
| A3 斜体 | 无新增 |
| A4 Hero | 原生表单，无 Hero；不适用 |
| A5 卡片海 | 新增卡片 0 |
| A6 假数据 | 无产品假指标，测试输入仅为夹具 |
| A7 空词/感叹号 | i18n 字符串零改 |
| A8 眉批 | 无新增 |
| A9 标题 | 原标题层级与宽度保留 |
| A10 Bento | 无此布局，不适用 |
| A11 装饰竖线 | 无新增 |
| A2 工艺密度 | #3/#4/#12 三项；用户修 bug 范围优先，不凑五项 |
| A4 功能状态 | 编辑、拒绝、刷新、确认、取消、回调重入均有真实事件 |
| B1 设计读取 | 克制 3/2/7，保留紧凑表单 |
| B2 主题 | 深浅各自一致，无混色 |
| B3 记忆点 | 提示只指向当前字段/当前问题 |
| B4 动效气质 | 0ms 状态刷新，无装饰动效 |
| C1 目的与频率 | 高频编辑即时，新增动画 0 |
| C2 all/ease-in/scale(0) | 无新增 |
| C3 时长/按压 | 既有 120ms 颜色与按压保持 |
| C4 origin | 无新增浮层 |
| C5 reduced-motion | 事件夹具 ReduceMotion=true，已验收 |
| C6 hover | 原生框架指针行为保持，CSS 不适用 |
| C7 属性 | 无布局属性动画 |
| C8 keyframes | 无新增 |
| C9 手势 | 未改手势/拖拽，不适用 |
| C10 Framer | Go 自绘，不适用 |
| C11 逐帧 | 静态 before/after 状态事件，无新时序动画 |
| D2-1 字体 | 系统中文字体，无资产新增 |
| D2-2 字号/行高/字重 | 通用 ≥14px/≥1.5 与本仓高度链及既有字号冲突；沿用需拍板，不能全勾选 |
| D2-3 混排空格 | 产品文案零改 |
| D2-4 标点/对比度 | 标点零改；既有安静墨不等于通用 AAA，本轮不虚称重测全过 |

Linux ui.Tester 自绘；Windows/macOS 仅 vet/build，无真机交互证据。补充必填、旗标、选项刷新后，新增事件共 **34 PASS**；旧 V/B/S 0/0/0，无新增 Skip。三平台 vet/build 退出 0，gofmt 空。完整测试仍在运行。

## 完整门槛环境故障

首次普通/verbose 同时链接时临时盘耗尽：`mapping output file failed: no space left on device`，另有 `final link failed: 设备上没有空间`。这两次不能计为通过，已停止自己的两组测试进程，保存原始错误。改用有空间的任务临时目录串行重跑；不删除其他项目工作、不放宽断言。三平台 vet/build 已独立全部退出 0。

`df` 实测 /tmp 为 7.7G tmpfs，剩 64M；/home 剩 263G。清理本轮旧 go-build 的命令被执行器拒绝（`rm -f style commands are not permitted`），没有执行删除；改用 `GOTMPDIR` 与 `TMPDIR` 均指 `/home/tar/workspace/.floter-P13-ie-go-tmp`，无须清理即可继续。测试源码总数实测 **2979**（基线 2970，新增 9），产品 MD5 与完整门槛启动时一致。

## 仍待

1. 下一轮首选运行回调内部刷新同一表单、关闭后重开的拒绝归属，特别是 options 原地修改；这里只是代码线索，未宣称复现，不重复本轮普通刷新或插件动作回调的八项。
2. 参数全部移除后恢复的会话记忆仍未复现；Windows/macOS 真机交互未验，只完成 vet/build。
3. 字号/行盒、accent 禁用填充、未知多选值、未知 Radio 模型、176-9/161-4 保存中草稿归属、小于 1e-6 步长、终端 S1b/S1c/S2 尾继续需拍板，本轮不改。
4. 八条旧 WIP 原样保留，SHA 未变；不重复合入旧主题或 Tauri 链。

## 提交与基线证据

四批产品均已推送：`336784eb` / `f29954c5` / `eabb5cb7` / `40865f3a`。每批相关测试与 vet 退出 0 后才提交，没有把失败运行计入门槛。
基线日志 `/tmp/floter-P13-ie/baseline.txt` 的 SHA-256 为 `1c0e2a6f11e92b126ba66180ac31cdb012b7d407de2cfd5ba6b23e170e5a7bb0`，4507 PASS。完整重跑仍在进行，结案数字后补；不把初次链接失败的部分输出计成最终 PASS。

第二次普通全量因磁盘临时路径过长触发 Unix socket 的路径上限，termsession 报 `bind: invalid argument`；原始日志保存为 `full-test-longpath.txt`。路径调整为短目录 `/home/tar/.fie`，Go 构建产物仍用独立长目录；先检查 termsession 再重跑全量。不改变终端产品或断言。调用已超过 40，后续仅做必需门槛和交接，不继续扩题。

短目录首次只改 TMPDIR 未奏效，实际 t.TempDir 仍落在 GOTMPDIR；该次定向 termsession 退出 1 后，刚启动的全量已终止。最终两个变量都改成 `/home/tar/.fie`；只有 termsession 通过才继续串行全量。此处记录纠正，未声称第一次路径调整已通过。

两个变量均缩短后，termsession 定向退出 **0**（4.365s），完整门槛继续。最终产品 MD5 再核对为 OK；初次长路径全量的 settingsui 已通过（294.856s），但不据单包通过宣称完整门槛通过。

短路径普通全量已退出 **0**，原始日志入仓 `P13-ie/full-test.txt`。verbose 全量继续运行，最终计数待结束。普通全量后产品 MD5 与启动前一致。

## 最终验收

普通与 verbose 完整全量均退出 **0**；最终 **4541 PASS / 0 FAIL / 2979 测试函数**，基线 **4507 / 0 / 2970**，增加 **34 PASS / 9 函数**。三平台 vet/build 全部退出 0，gofmt 空；最终产品 MD5 与门槛启动时一致。旧 V/B/S 断言 **0/0/0**，新增 Skip **0**。

原始完整日志：[普通](P13-ie/full-test.txt)、[verbose](P13-ie/full-verbose.txt)；[门槛](P13-ie/gates.txt)、四批定向与四组变异日志均入同目录。32 张原图、2 张对照板；16 对全部有像素变化。八条 stash 原样保留，逐条表在本文开头。

四批产品 `336784eb` / `f29954c5` / `eabb5cb7` / `40865f3a` 已按每两项推送。结案补充参数条件三个分支的测试、§6.206、当前进度与全部证据。工具调用超过 40，超出后只完成门槛、环境故障修复与交接；从首次时钟 21:44 UTC 到结案约 38 分钟，低于 45–75 分钟目标，没有用额外改动或重复成功测试凑时长。两次环境失败与一次未生效的路径调整均保留记录，不声称流程完全合规。
