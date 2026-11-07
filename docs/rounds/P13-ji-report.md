# P13-ji · 导入部分生效与传输重试的结果归属

起点 da23e1b8。17:55 UTC 开盘，Codex 独立执行。已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN；保持既定方向。done.log 开盘不存在，收尾建立。基线与候选正在实测，不把计划记成完成。

## Stash 清点

活动清单无 stale-kill；以下八条与 P13-jh SHA 相同，reverse-check 全为 1。沿用已审核的早期 WIP 保留结论，不把旧主题或 Tauri 链重新覆盖现产品。每条原 patch 与检查日志暂存 /tmp/floter-P13-ji。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 增量记录

真实 UI 的 transferRow 点击已清空旧报告（settingsui.go:2219/2231），所以“新导入保留旧报告”线索不成立，不改产品、不计修复。下一步只验证部分生效与已解决通知。

基线中文 locale verbose 全量实测 **5555 PASS / 0 FAIL**、退出 0；全仓 Go 测试函数 **3270**（git grep '^func Test' -- '*.go'；只数 cmd/internal 的窄路径会漏掉两条，不能混口径）。八项候选均已在原产品复现：部分生效三阶段遗漏事实，五入口成功仍显示旧错。初次新夹具部分场景按钮在滚动区外，补真实 Scroll 后全部复现；初次日志保留，不拿夹具错误计产品缺陷。

1. 现象：导入已把原工具停用，随后损坏配置解析失败，报告只显示 JSON 原因。改动：已写回启用状态后遇到配置失败，补充真实 enabled 值与未完成阶段，保留原始 cause。验证：深浅真实 Click 导入、磁盘 enabled=false、破损配置字节未变；原始红 `partial enabled hides committed state`，修复配置可同入口重试。
2. 现象：新包已经安装并出现在仓库里，配置错误却让报告只说失败。改动：安装完成后的配置错误加“package was installed”，不改变写入顺序或回滚语义。验证：深浅 Click、新包记录存在/已停用、失败计数 1 成功 0；原始红 `partial package hides committed state`，修复后导入值真正落盘。

第一批相关回归与 vet 结果见 batch1-test.txt / batch1-vet.txt；完整全量留到轮末。旧 V/B/S 断言修改 0/0/0。

第一批定向 **13 PASS / 0 FAIL**，extensions/shell vet 净。新夹具首次有一次检查时磁盘尚无目标，补 Click 下一帧和明确派发断言后通过；首次没有记录派发，不能把这一场景直接判为产品错误，原失败日志保留。未来六项当时仅保留复现 helper，逐批启用相应测试，避免提交已知红测试。

第一批 **19c4a8e5** 已提交推送。

3. 现象：新包的配置模板目录被同名文件阻塞，包和仓库已落盘，报告只给 mkdir 错误。改动：保留 Commit 返回的已安装 entry 事实，说明包已安装、配置模板未完成。验证：深浅真实导入，仓库有新包且仍 enabled=true（尚未执行后续停用），失败数 1；原始红 `partial template hides committed state`。不自动补跑模板或回滚。
4. 现象：集成备份因仓库损坏而失败，修好后导出成功仍和旧错误同时浮在页面。改动：通用错误槽记录操作归属，导出成功只退役本入口的旧错；成功原 ID 和时长保留。验证：深浅真实 Click、文件确实写出、旧错误消失；原始红 `successful export retry retains its resolved error`。另验已绘制/排队中的外部错误及其他操作错误均不被清除。

第二批定向 **27 PASS / 0 FAIL**，extensions/shell vet 净。V/B/S 0/0/0；没有改变旧断言。

第二批 **9be9b5db** 已提交推送。

5. 现象：配置复制遇到损坏文本失败，修正后剪贴板已有正确文档，但旧错还在。改动：复制操作按动作+集成 ID 登记错误，实际剪贴板交付后退役。验证：深浅宿主 Actions 与 ui.Tester 通知事件，原始红 `successful copy retry retains its resolved error`；剪贴板含修正 endpoint 且无原密码，旧错误消失、成功可见，旧延迟交付测试通过。
6. 现象：配置导出选择器失败后恢复，文件已写出却同时显示旧错误。改动：同样按导出动作+集成 ID 退役，仅发生在成功写入后，取消不算恢复。验证：深浅选择器失败→成功写盘，原始红 `successful config-export retry retains its resolved error`；旧错消失、新结果可见，原验证失败与磁盘错误回归通过。

第三批定向 **22 PASS / 0 FAIL**，shell vet 净。V/B/S 0/0/0。

第三批 **d0ce7afd** 已提交推送。

7. 现象：脚本导出选择器失败后重试成功，原失败标题和原因仍浮在成功消息旁。改动：按脚本名归属错误，成功写盘后退役；尝试序号防止较早成功擦掉较晚失败。验证：深浅选择器恢复、文件非空且成功可见，原始红 `successful script retry retains its resolved error`；另验不同脚本与晚到完成不串线。
8. 现象：导入版本 99 被拒绝，修正为合法备份后已有导入报告，旧坏文件浮层还压着它。改动：本次导入生成逐项报告时退役该入口的旧总错误，新报告里的失败不受影响。验证：深浅真实 Scroll/Click、成功汇总可见且旧版本原因消失，原始红 `successful import retry retains its resolved error`；文件拒绝原有 i18n 标题及原因断言全部保留。

第四批定向 **43 PASS / 0 FAIL**，shell vet 净。八项已落地，跨集成传输、配置工具条、脚本导出及通知组件。旧 V/B/S 0/0/0；未新增 Skip。

第四批 **7433fe62** 已提交推送。第一轮图像复核：浅色报告把已停用事实放在错误原因前，正文在原滚动列内折行；深色配置导出恢复后右上只剩成功浮面，原失败浮面撤下。第二轮聚焦排队通知、其他目标、取消、并发脚本回调与中文小窗口，不新增装饰。

## Mutation（实跑）

- M1 撤销已启用状态/已安装的错误包装，两主题四场景实红：`partial enabled hides committed state`、`partial package hides committed state`；还原 `md5sum -c` OK，touch 后 **6 PASS / 0 FAIL**。
- M2 让 resolveNotice 提前返回，五入口两主题十场景实红：`successful export/copy/config-export/script/import retry retains its resolved error`（逐条原文在 M2-red.txt）；MD5 OK，touch 后 **15 PASS / 0 FAIL**。

两组变异退出 1，恢复与复验均退出 0。未改旧测试来消红。

## 偷了什么

实际读取 qiaomu-design 的 SKILL、偏好账本、craft-loop、preflight、chinese-typography；Apple DESIGN 的单一交互色角色、8px 控件圆角、2px 焦点线映射到既有 token，不更换本仓字号或品牌色。

- 从 craft-loop 工艺密度菜单取 **#4 签名交互**：恢复结果交接新增延迟 **0ms**，成功所在帧退役旧错；原错误自然到期仍是 **8000ms**，成功仍 **4000ms**。没有缩短阅读窗口，已解决才撤回。
- 取 **#6 编辑细节**：三个失败阶段的已生效事实与原因进入原 **1 个报告块**，新常驻状态容器 **0 个**；报告保留原前三条摘要和尾部滚动布局。
- 取 **#5 多层阴影 / #12 材质细节**：沿用 E2 的 **2 层投影、1px 受光边**；恢复后的同时可见浮面由 **2 个→1 个**，新增玻璃层 **0 层**。这些是原组件的值，未冒称本轮重新调色。

为什么更轻、更有空间感：撤下已经失效的错误浮面，成功结果只占原来的一层；部分生效的说明留在平面报告里，避免另加警告卡。用户要求功能修复优先，不为凑菜单五项加入装饰。

## 两轮图像与事件复核

第一轮四视角：艺术总监看到两块互相矛盾的浮面争注意力；工程师看到写盘结果和失败报告不同步；任务用户无法判断包是否已安装；读者修好问题后还被要求修同一个错误。第二轮只针对这四类问题收敛：部分生效事实、恢复后唯一有效结果、错误归属、中文小窗口。

第二轮新增 **6 PASS / 0 FAIL**：排队错误在首次绘制前解决后不复活；外部新错误在关闭队列之后到达仍显示；另一工具的复制/导出错误保留；取消不冒充成功；真实并发脚本选择器中较早成功不擦掉较晚失败。中文 720×520 场景另实测 **3 PASS / 0 FAIL**。

亲眼复核：浅色配置导出对照左侧有错误和成功两层浮面，右侧仅成功；深色模板错误的第一句现在说明包已安装与模板未完成，路径仍在原内容列内折行。中文深浅图里“配置已导出”和关闭按钮都完整，旧错误没有残留。原始图为 **720×1000 Linux ui.Tester 软件自绘**，中文图 **720×520**；不冒称原生桌面或 Windows/macOS 真机截图。

| 项 | 浅色对照 | 深色对照 |
| --- | --- | --- |
| 1 已停用后配置失败 | [图](P13-ji/partial-enabled-light-comparison.png) | [图](P13-ji/partial-enabled-dark-comparison.png) |
| 2 已安装后配置失败 | [图](P13-ji/partial-package-light-comparison.png) | [图](P13-ji/partial-package-dark-comparison.png) |
| 3 已安装后模板失败 | [图](P13-ji/partial-template-light-comparison.png) | [图](P13-ji/partial-template-dark-comparison.png) |
| 4 集成导出恢复 | [图](P13-ji/retry-export-light-comparison.png) | [图](P13-ji/retry-export-dark-comparison.png) |
| 5 配置复制恢复 | [图](P13-ji/retry-copy-light-comparison.png) | [图](P13-ji/retry-copy-dark-comparison.png) |
| 6 配置导出恢复 | [图](P13-ji/retry-config-export-light-comparison.png) | [图](P13-ji/retry-config-export-dark-comparison.png) |
| 7 脚本导出恢复 | [图](P13-ji/retry-script-light-comparison.png) | [图](P13-ji/retry-script-dark-comparison.png) |
| 8 导入恢复 | [图](P13-ji/retry-import-light-comparison.png) | [图](P13-ji/retry-import-dark-comparison.png) |

16 组深浅对照像素差均非零，[像素差日志](P13-ji/image-differences.txt) 同时注明临时路径和主机工具发现会变化，不能把整图差异当作纯设计墨值。总览：[浅色](P13-ji/overview-light.png)、[深色](P13-ji/overview-dark.png)。中文小窗口：[浅色](P13-ji/chinese-compact-light.png)、[深色](P13-ji/chinese-compact-dark.png)。代表图 `/tmp/floter-P13-ji-transfer-{light,dark}.png`。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿用近黑与浅色主题；截图窗外透明区不是产品纯黑面 |
| A2 非系统字体 | 按用户原生 Go 与中文合约保留系统字体，不执行通用换字体建议 |
| A3 斜体 | 无新增，中文图无伪斜体 |
| A4 Hero | 不适用设置/工具功能面 |
| A5 三等分/卡片海 | 无新增分组卡、无新页面 |
| A6 假数据 | 产品无新假指标，Configured 是真实脚本 provider 夹具 |
| A7 空词/感叹号 | 新诊断只描述已生效事实与失败阶段 |
| A8 眉批 | 无新增 |
| A9 H1 | 中文小窗口标题单行可见；长报告进入原滚动区 |
| A10 Bento | 不适用 |
| A11 左色条装饰 | 无新增 |
| B1 读取与拨盘 | 2/1/6：原布局、即时反馈、原密度，符合既定方向 |
| B2 主题锁定 | 深浅分别创建 Tester，不混主题 |
| B3 记忆点 | 失败报告说明已经发生的事，成功后不再被旧错遮挡 |
| B4 气质 | 高频结果交接不加动画，与克制表面一致 |
| C1 目的/频率 | 新延迟 0ms；原成功和错误自然到期不变 |
| C2 all/ease-in/scale(0)/fade | Go 自绘无 CSS，未添加这些动画 |
| C3 时长/按压 | 未修改原按压和 toast 入场时长，不用退出动画延迟状态事实 |
| C4 origin | 复用原右上通知位置，没有新 modal/popover |
| C5 reduced motion | 全部深浅与中文对照实际使用 ReduceMotion=true |
| C6 hover | 无 CSS 媒体查询；必要诊断和恢复结果不依赖悬停 |
| C7 动画属性 | 未新增宽高或布局动画 |
| C8 重触发 | 现有 ID 更新机制保留，新增的是失效错误的退役 |
| C9 手势 | 未涉及 |
| C10 Motion | 未引入该依赖 |
| C11 慢放 | 无新动效；未宣称真机帧率或慢放验收，事件与静态帧有证据 |
| D2-1 系统中文字体 | 沿用系统字体，无下载字体；中文小窗口已目视 |
| D2-2 ≥14px/1.5 行高 | 通用要求仍未满足，旧 Caption 9.9/正文10.8 与行盒待拍板，用户红线禁止本轮擅改 |
| D2-3 混排空格 | 既有中文 i18n 保留；新底层英文诊断不拼接中文单位 |
| D2-4 标点/AAA | 中文原标点不改；没有重新测全套 AAA，不冒称旧次要墨均达 7:1 |

现存字号/对比度拍板项如实列出，用户本轮修复授权优先于通用 skill 的“全项通过”措辞。可复现问题沉淀为 §6.236 和事件测试；没有把本项目一次修复写成全局审美偏好。

## 仍待

1. 下一轮先复现同一备份中重复集成 ID 对 enabled 的影响：ImportSync 保留开盘 inventory，而配置逐条合并，启用判断可能读取旧值。仅代码线索，先真实事件验证；重复项覆盖规则若需改变则列拍板，不擅改事务/覆盖语义。
2. 配置模板失败后包已存在，导入重试走更新路径，不会补跑安装模板；本轮只修正反馈，自动补跑/回滚属于现有安装语义，需另定规则。没有把“修好路径后重新导入”误记为模板自动恢复。
3. 既有多草稿/覆盖、字号/行盒、accent 禁用填充、未知多选/Radio、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾继续待拍板。原生选择器与 Windows/macOS 运行时未验，八条早期 WIP 继续保留。

## 完整门槛与交接

| 门槛 | 实测结果 | 证据 |
| --- | --- | --- |
| 基线 verbose | 5555 PASS / 0 FAIL，退出 0 | [日志](P13-ji/baseline-verbose.txt) |
| 最终普通全量 | 退出 0 | [日志](P13-ji/full-test.txt) |
| 最终 verbose 全量 | **5591 PASS / 0 FAIL**，比基线 +36，退出 0 | [日志](P13-ji/full-verbose.txt) |
| 测试函数 | **3284**，基线 3270，+14；旧函数零删 | [计数](P13-ji/test-functions.txt) |
| gofmt | 输出 0 字节 | [日志](P13-ji/gofmt.txt) |
| 三平台 vet/build | 六项退出 0 | [汇总](P13-ji/platform-gates.txt) |
| Mutation | 2 组实红、MD5 OK、touch 后 6/15 PASS | [M1](P13-ji/M1-red.txt)、[M2](P13-ji/M2-red.txt) |
| V/B/S | 0/0/0，旧测试零改，新增 Skip 0 | git diff da23e1b8 |

最终普通/verbose 全量首次均通过，未重跑全量；源码最终 MD5 全部一致。第二轮只增归属、取消与并发的验收，产品代码保持第四批 **7433fe62**。首轮夹具失败和两次补丁上下文匹配失败如实保留，均没有作为产品已修证据。深浅 4× 局部已亲眼复核，notice-detail 图片是最近邻检查图，不冒充真实高 DPI。

过程偏差：收尾等待出现多次短轮询，工具调用超过 40 次；进入收尾后只完成已有八项的门槛与归档，没有继续扩题。17:55 UTC 开始，2026-11-07 18:22 UTC 完成全量核对，低于 45–75 分钟目标窗口；不以额外装饰或重复测试补时长。任务进度替换到 .pending-bugs.md，done.log 写入最终结案 SHA 和实测数。
