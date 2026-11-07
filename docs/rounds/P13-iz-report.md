# P13-iz：快捷键编辑的拒绝归属与可触达性

八项修复已完成并分四批推送。最终 **5301 PASS / 0 FAIL / 3192 测试函数**（基线 **5274 / 0 / 3183**），普通/verbose 全量及三平台 vet/build 全部通过。四组 mutation 均实红并 MD5 还原，16 组深浅前后图已归档。

开盘 038f2cb6，pull 已最新、工作树干净。实际最新 P13-iy，按最新交接继续。Codex 独立执行；无其他执行器、外部模型或代理。

## Stash 清点

逐条导出 patch、reverse-check 全为 1。SHA 与 P13-iy 及 stash-backlog 一致；早期独立 WIP 继续保留，不重复合入旧主题或旧 Tauri 链。已接续的 stale-kill 不在活动清单内。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse 1 | 保留 |

## 增量记录

候选以真实 Click/Key 事件复现；原代码通过者不算修复。宿主仅保存注册成功项的行为保持。

### 第一批

1. 现象：B 录 A 被本地拒绝后，Add → Escape 抹掉 B 的拒绝。改动：Add 只清草稿所属的拒绝。验证：深浅真实点击与录键，旧代码两场景均红 `unrelated recorder erased saved refusal`；原三个绑定不变。
2. 现象：同一拒绝之后 C 开始录制再 Escape，也抹掉 B 的拒绝。改动：保存行开始录制只清本行拒绝。验证：相同真实事件确认 B 的提示仍属于 B，取消不写绑定。旧代码深浅均红。

基线实测 **5274 PASS / 0 FAIL / 3183 函数**。第一批相关 **179 PASS / 0 FAIL**、settingsui vet 退出 0；M1 回注旧代码 **6 FAIL**，MD5 OK，touch 后相关回归通过。V/B/S 0/0/0。前后图已导出，初次目视确认旧图 B 行下的拒绝说明消失。

第一批 c80e9397 已推送。

### 第二批

3. 现象：B 本地冲突后 C 成功录为 R，B 的说明被抹掉。改动：保存行成功录键只清当前录制行的拒绝。验证：深浅真实录键，C 保持 clipboard 动作且键变 R，B 原键与拒绝仍在。
4. 现象：Add 录重复 A 后，另一个保存 C 成功改 R，未完成草稿的拒绝也被清掉。改动：同一归属规则保护草稿。验证：深浅真实事件，成功保存与草稿拒绝同时存在；旧代码四场景均红 `successful neighbor capture erased unrelated refusal`。

大字号几何候选六组（保存/草稿录制、保存/草稿警告、内置录制、长未知动作）原代码全部通过，未计修复；警告真实换行，录制提示没有覆盖邻控件。截图保留。

第二批相关 **185 PASS / 0 FAIL**，settingsui vet 退出 0；M2 回注旧成功捕获分支 **6 FAIL**，MD5 OK、touch 后复验通过。V/B/S 0/0/0。

图像复核发现几何探针的边界不够严格：长未知动作的箭头被右侧裁掉；窗口 704 边界检查通过并不表示控件内部完整。后续补测内容盒与箭头位置，不把初始通过冒充已验证。

第二批 bb77a68d 已推送。

### 第三批

5. 现象：B 本地冲突后，Add 成功录 Z（尚未 Bind）也抹掉 B 说明。改动：草稿成功捕获只清草稿拒绝。验证：深浅真实 Add/录键，Z 留草稿、三个保存绑定不变、B 警告保留；原串 `successful draft capture erased saved refusal`。
6. 现象：草稿 Z 被系统拒绝，删除草稿后重新 Add 并录 Z，新草稿未经注册就继承旧“被占用”。改动：取消草稿时退役它本次的注册拒绝，保留其他键的拒绝。验证：深浅真实 Bind→删除→Add/录 Z，新草稿没有旧占用提示，仍能 Bind；原串 `new draft inherited canceled draft refusal`。

另一个探针确实观察到“命令编辑回原文但未 Enter，外部更新仍被草稿覆盖”；这是仍持有未提交草稿的既有契约，自动认定 pristine 会改变草稿优先级，未修改、不计修复。原始输出保留 batch3-before。

第三批相关 **191 PASS / 0 FAIL**，settingsui vet 净；M3 **6 FAIL**，MD5 OK、touch 后复验。另加真实 shell 宿主路径（实际注册器拒绝、写入成功列表、恢复注册），连同既有草稿拒绝恢复共 **9 PASS / 0 FAIL**，不以单独 fake setter 代替宿主验证。

第三批 00bce344 已推送。

### 第四批

7. 现象：large 档长未知动作把 picker 伸到 x699.2（内容右沿 x684.8），文字吃掉下拉箭头。改动：仅快捷键动作 picker 跟随已有 FlexibleControls 槽收缩，选择器文字让出箭头与间距。验证：真实完整设置页两主题，箭头保留 33px 预算，原值仍可见且能选择命令恢复；旧串 `long action consumes dropdown arrow space` / `long action exceeds row content edge`。
8. 现象：插件选项名很长时，popup 以窗口宽为自然上限，定位后选中面的蓝色到达最右可绘制列 x718，右侧间隙与圆角被裁。改动：popup 最大宽度扣除左右各 1u 间隙（default 共 7.2px）；正常宽度与键盘选择不变。验证：真实插件配置下拉→End，逐像素检查 x718 不得仍是选中面的颜色，Enter 保留正确 value。最初的文字右沿 710px 阈值并无契约依据，已弃用；第一版像素探针误取根本不绘制的 x719，旧代码通过，已修为实际被裁的位置。不以 0.4px 凑修复，不隐去探针失误。

第四批相关 **253 PASS / 0 FAIL**，settingsui/shell vet 净。最终 M4 两处旧实现回注 **6 FAIL**，包括 `long option highlight is cut off at the window edge`；两文件 MD5 OK，touch 后 **6 PASS / 0 FAIL**。长动作 trigger 宽 **342.4 → 328.0px**，文字宽 **326.0 → 277.6px**；长选项文字宽 **692.8 → 685.6px**。旧 V/B/S 0/0/0。

第四批 94abe14e 已推送。四批均为每两项定向回归/vet 后立即 commit/push；完整门槛已集中执行通过。

## 偷了什么

- craft-loop 菜单 **#3 品牌 focus 环**：Apple DESIGN 的单蓝交互角色与 **2px** focus 沿用，录制出口仍 **0ms** 交还原控件；真实 Escape/Enter 走原模型。
- **#4 签名交互**：一次只存在 **1** 个录制者，取消写绑定 **0** 次；成功草稿在 Bind 前保存 **0** 次。拒绝始终跟随对应对象。
- **#6 编辑细节**：large 长动作文字给右侧箭头及 padding/gap 留 **33px**；长 popup 留左右各 **1u**（default 合计 **7.2px**），与现有菜单内边距同尺度。
- **#12 材质细节**：沿用 Apple 8px 控件圆角在本仓的语义 token、**1px** 受光边，新增玻璃/阴影 **0** 层。修复已有浮层圆角裁切，不增加白边或高光。

为什么这轮更轻、更有空间感：错误说明各归自己的行，结束的尝试不会在新表单复活；长文字给箭头留位，浮层右侧轮廓不再贴边截断。用户要求优先修现有问题，依 craft-loop 优先级不为凑五个装饰动作加效果。

## 两轮图像复核

第一轮：艺术总监看到长动作的胶囊在右边被切断；工程师发现窗口边界检查漏掉了控件内部箭头；任务用户看见邻行录制抹掉错误，无法追溯失败；读者在新草稿上遇到上一张表单的占用提示。第二轮只收敛这四处，重新导出深浅图与两处 4× 局部。

已目视：浅色长动作 after 出现省略号，右侧箭头与圆角完整；深色长选项 after 的选中面收在浮层边内。原字号问题没有冒充解决。完整原帧由 Linux ui.Tester 导出，4× 是最近邻检查图，不冒充其他平台或高 DPI 真机。

| 项 | 浅色前后 | 深色前后 |
| --- | --- | --- |
| 1 Add 保留拒绝 | [图](P13-iz/refusal-add-light-comparison.png) | [图](P13-iz/refusal-add-dark-comparison.png) |
| 2 邻行录制保留拒绝 | [图](P13-iz/refusal-record-light-comparison.png) | [图](P13-iz/refusal-record-dark-comparison.png) |
| 3 保存行成功捕获 | [图](P13-iz/capture-saved-refusal-light-comparison.png) | [图](P13-iz/capture-saved-refusal-dark-comparison.png) |
| 4 草稿拒绝保留 | [图](P13-iz/capture-draft-refusal-light-comparison.png) | [图](P13-iz/capture-draft-refusal-dark-comparison.png) |
| 5 草稿成功捕获 | [图](P13-iz/draft-capture-saved-refusal-light-comparison.png) | [图](P13-iz/draft-capture-saved-refusal-dark-comparison.png) |
| 6 草稿拒绝退役 | [图](P13-iz/draft-cancel-refusal-light-comparison.png) | [图](P13-iz/draft-cancel-refusal-dark-comparison.png) |
| 7 动作箭头 | [图](P13-iz/unknown-action-arrow-light-comparison.png) | [图](P13-iz/unknown-action-arrow-dark-comparison.png) |
| 8 长选项浮层 | [图](P13-iz/long-option-popup-light-comparison.png) | [图](P13-iz/long-option-popup-dark-comparison.png) |

[总览](P13-iz/overview.png)；每项两套原帧与像素差在同目录。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，既有主题 token 不动 |
| A2 非系统字体 | 中文系统字体规则优先，不为网页模板规则换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生功能设置 |
| A5 卡片模板 | 没有新增包装或卡片 |
| A6 假数据 | 没有产品假指标，测试字符串仅是夹具 |
| A7 文案空词 | i18n 零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原页头完整可见 |
| A10 Bento | 不适用 |
| A11 左侧装饰竖线 | 无新增 |
| B1 读取/拨盘 | 3/2/7，原密度、克制、即时反馈 |
| B2 主题锁定 | 每个场景独立设置深/浅，未混主题 |
| B3 记忆点 | 问题归属自己的行，长文字不吃掉入口 |
| B4 动效气质 | 高频键盘交接 0ms |
| C1 目的/频率 | 未新增高频键盘动效 |
| C2 all/ease-in/scale(0)/纯 fade | 无新增 |
| C3 时长/按压 | 沿用 120ms 颜色过渡与既有按压 |
| C4 origin | popup 仍跟随原 trigger |
| C5 reduced-motion | 新事件均 ReduceMotion=true；未新增动画 |
| C6 hover 查询 | Go 自绘，无 CSS 媒体查询；原指针语义保持 |
| C7 布局动画 | 无新增 |
| C8 keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用 |
| C11 慢放 | 无新动效，未声称慢放真机验收 |
| D2-1 字体 | 原中文系统字体，无 webfont |
| D2-2 行高/字号/字重 | 通用 ≥14px/1.5 尚不满足，原字号与行盒继续待拍板 |
| D2-3 混排空格 | i18n 零改，无新增产品混排句 |
| D2-4 标点/暗色对比 | 原标点与墨不动，不声称默认次要墨全过 7:1 |

可复现缺陷落入本仓 §6.227 与事件 pin；没有把一次修复改为全局 skill 的审美规则。四方向预览、外部模型、网页浏览器视口不适用用户指定的本轮流程。

## 仍待

1. 下一轮首选：快捷键删除/重录后，原生回调已排队再到达时是否仍派发旧动作。仅代码线索，先宿主事件复现；不改变保存成功列表、会话或键盘模型。
2. P13-iy 已复现的“Add Z 后另一保存 B 转命令覆盖共享草稿”继续需拍板。系统拒绝保存行后的保留/恢复策略也不能绕过仅保存注册成功项的契约；本轮修的是既有草稿与反馈归属。
3. 未 Enter 的命令草稿即便编辑回原文仍优先于外部更新，若要自动视作 pristine 须单独拍板。其他字号/行盒、accent 禁用填充、未知多选/Radio、极小步长、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾照旧保留。
4. Linux 自绘与注入系统注册器有证据；原生桌面热键抢占、Windows/macOS 真机未验证。八条早期 WIP 保留，无新 stash。

## 完整门槛与证据

基线 **5274 PASS / 0 FAIL / 3183 函数**；最终 **5301 PASS / 0 FAIL / 3192 函数**（+27 PASS / +9 函数）。gofmt 空，linux/windows/darwin vet/build 六项均退出 0；完整普通与 verbose（`-p 1 -count=1`）均退出 0。产品只改两个文件，旧测试与旧 V/B/S 断言零改，没有新增 Skip。收尾仅校准说明注释、归档文档与证据，未继续扩选题。

16 组前后均有像素变化：最小为长动作浅色 **983 像素**，最大为草稿捕获深色 **37320 像素**（包括保留提示后后续行的正常布局移动）。[像素差](P13-iz/image-differences.txt)；[最终产品 MD5](P13-iz/final-source.md5)。每组原帧保持相同尺寸。

原始变异错误：[M1](P13-iz/M1-red.txt)、[M2](P13-iz/M2-red.txt)、[M3](P13-iz/M3-red.txt)、[最终 M4](P13-iz/M4-final-red.txt)。M1/M2/M3 均 6 FAIL 后 MD5 还原、touch 再跑对应相关回归；最终 M4 同样 6 FAIL，恢复后 6 PASS。探针未通过和探针本身的纠正均保留原日志。

[门槛日志](P13-iz/gates.txt)、[普通全量](P13-iz/ordinary.txt)、[完整原始日志哈希](P13-iz/full-log-sha256.txt)。基线与 verbose 原始输出位于 `/tmp/floter-P13-iz/`；代表原帧 `/tmp/floter-P13-iz-shortcuts-{light,dark}.png` 与 `/tmp/floter-P13-iz-select-{light,dark}.png`。普通 settingsui 205.206s、shell 31.169s。

首个时钟读数 11:42:29 UTC，完整门槛结束 12:08:55 UTC，记录区间 **26 分 26 秒**，实际开盘略早。低于 45–75 分钟目标；八项与全部验收已完成，不添加无关工作补时长。
