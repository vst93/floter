# P13-io · 编辑状态与键盘边界

开盘 bb8c6567，04:18 UTC，pull 最新、工作树净。Codex 独立执行，已读本轮设计 skill 与三个参考。中文 locale 完整基线实测 **4888 PASS / 0 FAIL / 3068 测试函数**。

## Stash 清点

八条 SHA 与 P13-in/backlog 一致；当前无新增 stale-kill，早期抢救已归档。本轮逐条导出 patch、实跑 reverse-check 均退出 1。沿已审查结论保留，不能用旧主题/Tauri 链覆盖当前产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse 1 | 保留 |

## 实施记录

旧 V/B/S 断言修改 0/0/0。尚未验证的候选不计入完成。

### 第一批：命令忙态键盘出口

1. 现象：别名输入持焦时，repair/disable/remove/uninstall/toggle 任一整行忙态使输入变灰，键盘停在禁用框，Enter 不能退出。改动：命令板仍存在但整行已锁定时归还既有页头 Close。验证：五种忙态 × 深浅，真实 Tab → 刷新 → 解锁 → Enter，Close 恰好 1 次、写回 0 次，解锁不抢焦。
2. 现象：命令开关持焦时遇到上述五类锁定，同样没有键盘出口。改动：同一命令板出口覆盖开关。验证：五种忙态 × 深浅，退出 1 次、开关与别名回调 0 次；另测未持焦命令不抢页头位置。

相关回归、包 vet 与 M1 已完成，原始数字见本批日志。M1 回注开盘产品实际变红：`busy command stranded keyboard instead of returning to Close`、`busy roundtrip cannot exit: closed=0 writes=0`。恢复源码后 md5sum -c 一致、touch 复跑通过。旧 V/B/S 0/0/0，新 Skip 0。

目视浅色忙态：两块命令板保持原几何与暗线，焦点环明确在右上关闭键，未留在灰掉的输入上。

第一批相关 **139 PASS / 0 FAIL**、包 vet 净，M1 注入 **22 FAIL**，逐字节恢复并 touch 后 **23 PASS / 0 FAIL**；7951a323 已推送。

### 第二批：未写回别名的显示与恢复

3. 现象：输入 json 后一帧普通重绘就显示旧别名，再输入 view，最终只写回 view。改动：待写队列在 180ms 窗口内同时作为显示草稿，重绘不重置文本编辑器。验证：真实 Type → Frame → Frame → 全选复制精确读值 → 继续输入，179ms 写回 0 次，181ms 恰好写回 jsonview。
4. 现象：已输入的别名因命令临时 undeclared/orphan/整卡消失而撤回，恢复后显示 stored；空草稿也复活旧值。改动：新建别名输入从已有 pendingAliases 取值，空字符串按存在性处理。验证：四类路径 × 深浅，恢复后全选复制精确读取 draft 或空串，再续写 x，Flush 只向 ext/jv 写回正确内容一次。

相关 **59 PASS / 0 FAIL**，包 vet 净。M2 回注旧显示来源实跑 **12 FAIL**，原串 `redraw replaced the pending alias: "", want json`、`continued typing lost pending prefix: [[ext jv view]]`、`redeclared editor lost its pending alias: "stored", want "draft"`；MD5 恢复、touch 后 **12 PASS / 0 FAIL**。初版新测试误用 HasText 读取输入内容，改为真实全选复制并严格比较；旧 B 断言零改，没有放宽容差或删除校验。

第二批 5ad1269c 已推送。

### 第三批：别名的按键归属

5. 现象：别名框内按 ↑ 切到快捷键页、↓ 切到关于页，原编辑位置消失。改动：别名接入设置文本字段已有的上下键保护。验证：两个方向 × 深浅，真实按键后仍为 Integrations、焦点在别名，复制内容仍为 draft。
6. 现象候选：将“新命令”改绑编辑键后，宿主注册该键，别名仍优先编辑。使用明确的 Ctrl+Backspace 复验；最初 Cmd+A 夹具在 Linux 的实体 Command 与 ui.Cmd（主编辑修饰键）不等价，不能作为该缺陷证据。

M3 以修正后的实体 Ctrl 夹具重新实跑，结果在后文记录。新测试曾因 Key→string 转换被 vet 拒绝，改用 fmt.Sprint 后才进入事件验证，没有跳过 vet。

修正夹具后的第 6 项成立：Ctrl+Backspace 先删别名、Close=0。改动：OnTextKey 在编辑动作前回答已配置的退出键；其他键仍由编辑器处理。真实窗口快捷键 + 输入键事件在深浅均 Close=1，pendingAliases 为空。M3 两类共 **8 FAIL**，原串 `alias swallowed rebound new-command: closed=0`；MD5 恢复、touch 后相关回归 **68 PASS / 0 FAIL**，包 vet 净。最终日志覆盖最初错误夹具，不用错误夹具计修复。

第三批 11f5703b 已推送。

### 第四批：配置与参数选项的同类按键遗漏

7. 现象：参数选项多行框中，Ctrl+Backspace 已绑定退出却删除 apps，关闭 0 次。改动：多行选项接入同一编辑前快捷键处理。验证：深浅真实键盘事件，退出恰好 1 次，files/apps 原文保留；重绑到 Alt+N 后，原全选、复制、换行并输入 notes 全部仍可用。
8. 现象：普通配置 Token 输入已有 HandleInput，但被窗口已注册的快捷键绕过，Ctrl+Backspace 仍删除 stored。改动：普通设置字段也通过原生编辑前回调回答已绑定操作。验证：深浅配置真实输入路径，退出 1 次、stored 不变；重绑后标准选择/复制继续正常。

M4 回注两处旧接线实跑红，原串 `options-shortcut swallowed rebound new-command: closed=0`、`config-shortcut swallowed rebound new-command: closed=0`、`shortcut modified editor draft`；MD5 恢复并 touch 后通过。精确计数与完整门槛集中记录于末尾。旧 V/B/S 断言修改仍为 0/0/0，新增 Skip 0。

第四批 **118 PASS / 0 FAIL**，包 vet 净，M4 **6 FAIL**，恢复后 **6 PASS**；产品 9e93ec1d 已推送。四批每完成两项即提交推送，没有将八项攒到轮末。

## 偷了什么

真实读取 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography，并查 Apple DESIGN 的单交互色角色。用户已经指定方向，使用打磨路径，由 Codex 独立执行。

- craft-loop 工艺密度菜单 **#3 品牌 focus 环**：沿 Apple 单一交互蓝角色与仓库 **2px** 环，忙态时 **0ms** 交给既有 Close，解锁 **0 次**抢焦；原主题蓝值不改。
- **#6 编辑细节**：保留 **1px** 发丝线、别名 **4px/8px** 内距；普通重绘后完整草稿仍在同一槽内，空别名精确为 **0 个字符**。180ms 之外才写回，不添加状态解释条。
- **#12 材质细节**：沿命令板 **8px** 圆角、别名 **6px** 圆角与原受光边，新增面/外阴影 **0 层**。这轮修内容与环的稳定性，不冒充材质重定基线。

为什么更轻、更有空间感：界面用原有焦点环和原槽里的真实内容表达当前状态，灰掉的控制不占用键盘，用户无需额外提示层解释输入去了哪里。用户要求修已有问题优先，不为五签名配额添加装饰。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹 | 无新增，沿原主题 |
| A2 字体 | 中文系统字体优先，不执行每轮换非系统字体的网页规则 |
| A3 斜体 | 无新增 |
| A4 Hero | 原生功能页，不适用 |
| A5 三等分/卡片海 | 新增容器 0 |
| A6 假数据 | 本地测试工具名，仅为事件夹具 |
| A7 空词/感叹号 | 产品文案零改 |
| A8 眉批 | 新增 0 |
| A9 标题 | 原页头与行盒保留 |
| A10 网格空洞 | 无新布局 |
| A11 装饰竖线 | 无新增 |
| B1 读取/拨盘 | 功能页 2/1/6：稳定、即时、原密度 |
| B2 主题 | 深浅独立 Tester，未混主题 |
| B3 记忆点 | 当前输入与唯一焦点始终可辨 |
| B4 动效气质 | 键盘恢复 0ms，新动画 0 |
| C1 目的/频率 | 高频编辑即时反馈 |
| C2 all/ease-in/scale/fade | 无新增 |
| C3 时长/按压 | 原反馈保留，不延迟输入 |
| C4 origin | 无新浮层 |
| C5 reduced-motion | 全部新事件/图使用 ReduceMotion=true |
| C6 hover 媒体查询 | 非 Web，沿原指针路由 |
| C7 布局动画 | 新增 0 |
| C8 keyframes | 新增 0 |
| C9 手势 | 本轮不涉及 |
| C10 Motion shorthand | Go 自绘，不适用 |
| C11 慢放 | 没有新增动效，逐帧事件验证；未声称真机慢放 |
| D2-1 字体 | 中文 locale、系统 CJK，无 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px/1.5 与旧尺寸合约冲突，继续待拍板 |
| D2-3 混排空格 | i18n 未改 |
| D2-4 标点/对比度 | 旧墨与标点不改，默认主题通用 7:1 未重测，不宣称通过 |

用户方向与仓库唯一来源优先于四方向/网页预览/换字体等通用规范。可复现缺陷写入 §6.216 和事件 pin，不把本轮修复变成新的全局审美规则。

## 图像证据

两轮复核：第一轮从艺术总监视角发现灰掉字段仍占焦点、工程视角发现帧刷新清空文本、任务视角发现方向键离开当前页、读者视角发现退出键让 Saved 变成 Unsaved changes。第二轮围绕这四处回看真实事件同帧图：深色别名保留 json，浅色配置保留 stored 且不产生伪未保存状态，忙态环落到 Close，方向键前后仍在 Integrations。浅色 4× 局部中，原圆角边没有新增重影，变化只在文本与光标；放大图不是高 DPI 真机渲染。

逐项深浅总览已目视，16 对的像素变化全部非零，最小 **180 像素**（重新声明恢复草稿）。对照图的快捷键处理由真实窗口级 Shortcut 与字段事件构成，Close 使用计数回调，未冒充整个 shell 真机退出画面。

八项均导出深浅 before/after 原帧，归档 32 原帧、16 对照板、2 局部与总览；像素差精确记录在 [image-differences.txt](P13-io/image-differences.txt)。截图来自 Linux ui.Tester；Windows/macOS 仅 vet/build，无真机原生输入法/Option 字母事件证据。

| 项 | 浅色 | 深色 |
| --- | --- | --- |
| 1 别名忙态 | [图](P13-io/alias-busy-repair-light-comparison.png) | [图](P13-io/alias-busy-repair-dark-comparison.png) |
| 2 开关忙态 | [图](P13-io/switch-busy-repair-light-comparison.png) | [图](P13-io/switch-busy-repair-dark-comparison.png) |
| 3 连续输入 | [图](P13-io/alias-redraw-light-comparison.png) | [图](P13-io/alias-redraw-dark-comparison.png) |
| 4 撤回再恢复 | [图](P13-io/alias-return-undeclared-light-comparison.png) | [图](P13-io/alias-return-undeclared-dark-comparison.png) |
| 5 方向键 | [图](P13-io/alias-arrow-14-light-comparison.png) | [图](P13-io/alias-arrow-14-dark-comparison.png) |
| 6 别名退出键 | [图](P13-io/alias-shortcut-light-comparison.png) | [图](P13-io/alias-shortcut-dark-comparison.png) |
| 7 参数选项退出键 | [图](P13-io/options-shortcut-light-comparison.png) | [图](P13-io/options-shortcut-dark-comparison.png) |
| 8 配置退出键 | [图](P13-io/config-shortcut-light-comparison.png) | [图](P13-io/config-shortcut-dark-comparison.png) |

## 仍待

1. 下一轮先事件复现别名待写期间的冲突/Taken 提示、宿主接受或规范化后的显示反馈；仅代码线索，不改变 180ms/Flush 或保存拒绝语义。
2. 重绑定的 macOS 原生 Option+字母路径仍缺真机证据；本轮实证为窗口注册的 Ctrl+Backspace 与现有编辑事件。脚本编辑器未证实缺陷，没有改动。
3. 字号/行盒、accent 禁用填充、未知多选/Radio、极小步长、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾仍需拍板；八条旧 WIP 保留。
## 完整门槛与交接

最终产品 **9e93ec1d**；四批每两项 **7951a323 / 5ad1269c / 11f5703b / 9e93ec1d** 均已推送。完整普通与 verbose 测试均退出 **0**，实测 **4937 PASS / 0 FAIL**（基线 4888，+49）；测试函数 **3077**（基线 3068，+9）。gofmt 空、Linux/Windows/macOS 三平台 vet/build 六项退出 0 且日志均 0 字节，更新合约后文档 CI 通过。旧 V/B/S 断言修改 **0/0/0**，旧测试函数未减少，新 Skip **0**；三份最终产品源码 MD5 校验一致。

| 变异 | 实跑红 | MD5 还原、touch 后 |
| --- | --- | --- |
| M1 命令忙态出口 | 22 FAIL | 23 PASS |
| M2 待写别名来源 | 12 FAIL | 12 PASS |
| M3 别名按键接线 | 8 FAIL | 68 PASS（相关回归全组） |
| M4 配置/选项编辑键 | 6 FAIL | 6 PASS |

完整证据：[普通测试](P13-io/final-test.txt)、[verbose 测试](P13-io/final-verbose.txt)、[门槛汇总](P13-io/gates.txt)、[源码 MD5](P13-io/final-source.md5)、[全图总览](P13-io/overview.png)。32 原帧、16 对照板、2 个 4× 局部和 1 总览均入仓。代表原帧另存 /tmp/floter-P13-io-alias-{light,dark}.png。

04:18 UTC 开盘，完整门槛收尾时间 **2026-11-07 04:42:13 UTC**，本轮短于 45–75 分钟目标；八项事件修复已经完成，不以无关改动补时。40 组 exec 编排之外发生了等待回收，按所有工具调用计已超 40；超过后仅完成既有工作的完整门槛、归档和交接，没有继续增加选题。普通与 verbose 全量并行导致单包运行比基线慢，均正常结束；未用中间包的计数冒充全量。
