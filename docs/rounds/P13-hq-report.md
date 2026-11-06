# P13-hq 报告：配置刷新后的编辑与校验

开盘 ef5e7b13，pull 最新，树干净。中文 locale 基线 3923 PASS / 0 FAIL / 2842 函数。done.log 不存在。

## 抢救 stash 清点

权威 git stash list 当前八条，reverse-check 全部退出 1；SHA 与 P13-hp 相同，stat 已逐项核对。backlog 的 stale-kill 抢救稿已接续/归档，这八条是早期独立 WIP，继续保留原件；不把 reverse 失败当作独有工作强行套回产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap 未完稿 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/滑块稿 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/探针 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段稿 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽稿 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键形状稿 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏行为稿 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 Tauri 发布稿 | 保留 |

## 增量记录

1. 前置分组后文本失焦、未提交内容丢失 → 分组身份按存活字段匹配 → 原实现深浅均红：`text-prepend lost focus on group refresh`、`lost draft: "", want "/local"`。
2. 数字跨分组后半输入丢失 → 优先沿用聚焦字段原分组身份 → 原实现深浅均红：`number-move lost focus on group refresh`、`lost draft: "", want "1e"`。

第一批两项通过，相关事件回归 **80 PASS / 0 FAIL**，settingsui vet 通过；旧 V/B/S 断言零改动。M1 回退 pluginsheet.go 后两条深浅场景确实报上述失焦/丢草稿，md5sum -c 两文件均 OK，touch 后复跑通过。第一次变异误将 App 新成员一起撤回导致编译失败，该次不计有效变异；已纠正注入范围并以真实断言红日志覆盖。文本保留后 Enter 只向自己的字段提交一次；数字夹具覆盖从非首行移入已有分组。

第一批提交 e2bd2905 正文将相关 PASS 误录为 93；原始 batch1.txt 实测为 80，本报告更正。基线 3923 与变异结果无误。

后续两组旧校验与移除重加路径的复现和修复见下文。完整门槛在八项完成后运行，发现回归后按要求修复并重跑。

3. 已存文本按 Enter 显示长度错误后，外部纠正为 ok，仍留旧红句 → 未编辑字段接纳新宿主值时清理旧校验 → 深浅字段 clean、旧句消失。
4. 已存数字 25 按 Enter 显示范围错误后，外部纠正为 5，旧红句仍在 → 同一宿主刷新路径清旧校验 → 深浅值 5、状态 clean、旧句消失。

第二批相关 80 PASS / 0 FAIL、settingsui vet 净。M2 回退 config.go 两项均红：`text-correct retains obsolete validation: "outside its allowed length"`、`number-correct retains obsolete validation: "outside its allowed range"`；md5sum -c OK、touch 后复跑。V/B/S 0/0/0。

5. 存量文本按 Enter 报错后放宽长度，仍显示旧错误 → 对已显示的错误按当前 schema 重验，未验证值仍不主动验证 → 深浅放宽后旧句退出、值与 dirty 不变。
6. 存量数字按 Enter 报错后放宽范围，仍显示旧错误 → 同上覆盖数字 → 深浅放宽后旧句退出，不触发写回。

第三批相关 80 PASS / 0 FAIL、settingsui vet 净。M3 原文 `text-relax retains obsolete validation: "outside its allowed length"` 与 `number-relax retains obsolete validation: "outside its allowed range"`；两文件日志留存，MD5 还原 OK、touch 复跑。V/B/S 0/0/0。

7. 文本只按 Enter 验证、从未输入，移除再加回字段时旧长度错误复活 → 错误按字段存在性独立清理，不依赖 live 草稿 → 重加不显示旧校验；下一次 Enter 仍正确拒绝非法值。
8. 数字同样移除重加后复活旧范围错误 → 同一清理路径覆盖数字 → 重加静息、下一次 Enter 正常验证，stored/draft 值均未改。

M4 原文 `text-remove retained removed field error`、`text-remove resurrected validation before interaction`、数字对应两句；深浅均红。MD5 还原 OK、touch 后复跑。第四批 **108 PASS / 0 FAIL**、settingsui vet 净。补查分组合并、移除前置字段、改标题及 Ctrl+Z，沿用前两条测试，不计额外修复。

第一遍看图：文本上半帧 /old、下半帧 /local；数字上半帧 5、下半帧 1e；文本外部纠正后红框与长度错误消失、焦点蓝边保留；数字约束放宽后相同 25 不再背着范围错误。高度变少来自错误行退出，既有高度链与槽尺寸未改。

## 偷了什么

从 `craft-loop.md` 工艺密度菜单第 3 项“品牌 focus 环”取持续的输入位置反馈：已有 1px 焦点边在外部纠正与放宽约束后仍由当前字段持有，未新增环层；插件编辑光标和选择跟随字段。
第 6 项“编辑细节”落为六条校验路径中旧错误句数量 1→0，焦点字段草稿丢失数 1→0。第 12 项“材质细节”保留 RadiusSM=8、1px 槽边，新增透明层 0；该项是保留验收，不计第九项修复。
Apple DESIGN 的交互蓝、8px 圆角、0.48 弱化分别映射现有 accent、RadiusSM=8 和框架 0.5；不抄营销字体/玻璃模糊。用户要求修既有缺陷，因此没有为五项工艺密度加装饰；按 craft-loop 优先级先做两轮截图复核。

## 两轮复核与证据

32 张原图及 16 张前后板（上 before / 下 after）在 `P13-hq/`，原图同时位于 `/tmp/floter-P13-hq-*.png`。插件 720×400，集成 720×320。before 是回退旧产品实现后的同事件帧，after 是 MD5 恢复后的同事件帧。
第一遍检查 text-prepend-light、number-move-dark、text-correct-light、number-relax-dark；第二遍逐张检查其余十二张板，深浅均可见相同修复。移除重加两组仍保留相同原值，但不复活上次的红框与旧句。
4× 局部检查了浅色旧错误边与数字半输入的槽内字形：1e 完整落槽，现有小字号的栅格化仍明显；未以加粗/改字号绕过既有拍板项。

四视角：艺术总监看过期红框抢走焦点（3–6）；工程师看重分组让编辑器重建（1–2）；任务所有者看重新出现的字段带着上次错误（7–8）；读者看半输入被吞及既有小字号（2 与原有拍板项）。第二次补强合并/改标题/删前置字段后的 Ctrl+Z，均通过。未发现需再加视觉装饰的问题。
真实 Windows/macOS GUI 与系统透明合成未在本机运行；本轮图为 Linux ui.Tester 渲染，不冒充真机截图。

## Preflight 逐项

A1 无新渐变/纯黑/霓虹；A2 沿用系统中文字体（中文规则优先），未按网页建议换字体；A3 无斜体；A4 无 Hero；A5 无新三栏/嵌套卡；A6 测试数据不作为产品假用户；A7 无新营销词/感叹号；A8 无新编号眉批；A9 无 H1；A10 无 Bento；A11 无选中装饰竖线。
A2 工艺密度没有新增五种动作，用户修缺陷范围优先；不能将技能全项标为通过。当前 Codex 独立执行，无 K3/子代理。自发现问题落入本仓 §6.192 与可复现事件 pin，不改写公共技能长期偏好。

B1 冒险 2/动效 2/密度 7 与原有紧凑配置面一致；B2 单帧单一主题；B3 记忆点是输入仍在、错误只描述当前字段；B4 高频输入即时，不新增过渡。

C1 高频键盘反馈即时；C2 无新增 all/ease-in/scale(0)；C3 沿用现有 120ms 色阶/按压；C4 无新浮层；C5 测试 ReduceMotion 开启；C6 原生鼠标事件、CSS hover media 不适用；C7 无新增几何动画；C8 无 keyframes；C9 无手势改动；C10 无 Framer Motion；C11 无新动画，不声称设备慢放验收。

D2-1 系统中文字体、零外部字体；D2-2 **未通过**，原有 Body 10.8px/Caption 9.9px、620/650 字重继续列需拍板；D2-3 无新增中文产品文案，数字/单位规则未变；D2-4 无新增标点问题，暗色沿用 token，未证明全页 7:1。用户要求记录不满足项继续授权修复，不等待确认。

## 提交与仍待

四批两项已推送：e2bd2905、de6cbb14、e148b39b、72c302b3。V/B/S 0/0/0，旧测试零修改、无新增 Skip。第一批正文 PASS 误录已在第二批和本报告纠正为 80。
下一轮首选同族选择控件（下拉/单选/复选）的跨组焦点与打开菜单，先复现真实事件；本轮八项验收只宣称文本/数字编辑与校验，不把未测场景写成已修。
原有字号/行盒、accent 禁用填充、保存期间草稿归属、未知 Radio、多选宿主过滤、终端 S1b/S1c/S2 尾沿用需拍板。八条早期 WIP 继续保留，无新增抢救稿。

## 完整门槛

gofmt 输出 0 字节，linux/windows/darwin vet 与 build 各退出 0。首次普通全量发现两条 B 回归：`TestConfigDefaultsClearOnlyRestoredFieldErrors` 报 `unrestored field lost its error: ""`；`TestTheConfigFieldCarriesItsOwnBoundsHint` 报字段提示不见。第三批对“已有错误”每帧重验过宽，宿主注入但无草稿的错误也被退役。
已收窄产品：记录边界值快照，只有当前字段约束真的改变才重验无草稿错误；有 live 的编辑仍按原规则逐帧验证。旧断言零改，相关回归已恢复。首次失败日志保留 `full-test-first-failure.txt`，不隐藏已推送期间发生的回归；最终完整门槛已通过。
收窄后相关 **113 PASS / 0 FAIL**，三平台 vet/build 再次各退出 0。M3 的最终实现重新注入旧实现，两条放宽约束 pin 仍红，MD5 还原与 touch 后 113 PASS；证据 `mutation-final.txt`、`mutation-final.md5`、`refinement.txt`。共五次有效变异运行（四批 + 收窄后复验），不把第一次编译失败计入。
第二次全量因执行编排失误，在 mutation-final 仍在运行时启动编译，编入了临时旧 config.go，四条新增 pin 红；不是可交付门槛。日志保留 `full-test-mutation-overlap.txt`。确认变异进程退出及当前 MD5 后，以固定源码顺序重跑普通/verbose 两次全量，期间不再改 Go 文件。
固定源码的普通与 verbose 全量均退出 0，最终 **3947 PASS / 0 FAIL / 2850 测试函数**；基线 3923 / 0 / 2842，增加 24 PASS 与 8 个函数。此次源码冻结快照覆盖五个改动 Go 文件，逐字节检查全部 OK。gofmt 空，三平台 vet/build 各退出 0。
最终证据：[gates.txt](P13-hq/gates.txt)、[普通全量](P13-hq/full-test.txt)、[平台门槛](P13-hq/platform-gates.txt)、[收窄回归](P13-hq/refinement.txt)。完整 verbose 位于 `/tmp/floter-P13-hq/verbose-final.txt`。
工具调用超出任务书的 40 次预算：定位与失败复验编排不够紧凑，属于本轮执行偏差；八项完成后停止扩题，仅完成回归修正与必要收尾。没有用继续加选题掩盖预算超支。

对照图：[文本·浅](P13-hq/text-prepend-light-comparison.png) / [文本·深](P13-hq/text-prepend-dark-comparison.png)，[数字·浅](P13-hq/number-move-light-comparison.png) / [数字·深](P13-hq/number-move-dark-comparison.png)，[校验·浅](P13-hq/text-correct-light-comparison.png) / [校验·深](P13-hq/text-correct-dark-comparison.png)。其余逐项图按 text/number 与 correct/relax/remove 命名。
