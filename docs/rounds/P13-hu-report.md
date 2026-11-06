# P13-hu：确认带的键盘出口与可见边界

开盘 `f3514951`，pull 最新，工作树干净。done.log 开盘不存在。中文 locale 基线 **4032 PASS / 0 FAIL / 2879 测试函数**。逐批落盘，完成项才记为修复。

补正开盘读取：第一次串行读取在缺失 done.log 处退出，未读到 stash-backlog，不能据此说后者不存在。收尾已单独读取实际存在的 `~/.hermes/state/iteration/stash-backlog-floter.md`，它确认 P13-gq/ha/hh/hl/hn 的抢救归档与本轮保留八条早期 WIP 的处置一致。

## Stash 清点

八条活动 stash 均为早期非 stale-kill WIP，与 P13-ht 清单 SHA 完全一致。本轮逐条 `git apply --check --reverse` 均退出 1；已抢救的八条早已归档，不重复合入，不以 reverse 失败推断可丢弃。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留独立 WIP |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留独立 WIP |
| stash@{2} | 7e4aa710 | 卸载布局/探针，reverse=1 | 保留独立 WIP |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留独立 WIP |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留独立 WIP |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留独立 WIP |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留独立 WIP |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留独立 WIP |

## 实施记录

规格中的八项均须真实事件或像素复现，未复现候选不冒充已修。V/B/S 暂为 0/0/0，旧测试未改。

### 第一批：停用与移除的键盘确认

1. 现象：停用确认每帧抢回取消，Tab 无法到达停用。改动：初次出现 AutoFocus，后续保留当前焦点。验证：深浅 × 中英文 × 同步/延迟失败，Tab→Space 执行，失败后 Space 可重试；忙态重复 Space 不派发。旧实现红串 `Tab cannot leave cancel for the confirmation action`。
2. 现象：移除确认同样困住 Tab。改动：同族入口只在挂载时聚焦取消。验证：上述八场景覆盖移除，明确两次确认恰好两次回调；新开仍默认取消，错误原文保留。

相关回归 **34 PASS / 0 FAIL**。M1 将两处 AutoFocus 注回 Focus 实跑，16 场景红；恢复源码 MD5 与 touch 后复跑。全量基线仍运行中，不填估计数。旧 V/B/S 断言修改 0/0/0。

第一批 `9f8c781c` 已推送；M1 恢复后 **18 PASS**，settingsui vet 通过。

### 第二批：取消后的键盘出口

3. 现象：停用取消后焦点消失，Space 无法再次询问。改动：确认退场当帧还给原开关。验证：完整 integrationRow 真实入口→Space 取消→Space 重开，深浅都重新聚焦取消，派发 0 次；旧实现 `cancel lost its original keyboard opener`。
4. 现象：移除取消后原按钮没有焦点。改动：同样还给原移除按钮，进入卸载对话框时不抢走浮层焦点。验证：深浅完整入口事件重开确认，派发 0 次；旧实现 `keyboard reopen failed: open=false cancel=false mutations=0`。

相关回归 **46 PASS / 0 FAIL**；M2 回注第一批产品，四场景红，MD5 恢复与 touch 后 **6 PASS**，settingsui vet 通过。V/B/S 0/0/0，旧断言不改。

第二批 `5453ae9f` 已推送。

### 第三批：提示带边界与失败语义

5. 现象：100% 宽度另加左右 margin，右边距实测 -11px，左边距 11px，卸载滚动容器中右沿被裁。改动：用 AlignSelf(Stretch) 在扣除外边距后的可用宽度内展开。验证：420/720 宽 × 深浅 × 中性/危险八场景像素边界对称；长句仍保留按钮自然宽度。旧红串 `notice escaped symmetric gutters: left=11.00 right=-11.00 want=10.80`。
6. 现象：停用失败只换错误句，仍画普通确认的中性墨与冠线。改动：已有 disableError 非空时使用 noticeBand 的错误臂，清错恢复中性面。验证：两主题错误环的暖色差与普通确认分开，清错后无旧警告；旧实现 `failed disable still wears neutral question ink: ring warmth=0`（具体读数以 M3 日志为准）。

未凑数：原长句挤压按钮候选排除，Cancel=49.4、Remove=118.5 在长短句中相等；长句的越界归第 5 项。

第三批相关回归 **64 PASS / 0 FAIL**；M3 回注旧宽度和中性错误臂，边界/长句/错误墨均实跑红，MD5 恢复与 touch 后 **18 PASS**，settingsui vet 通过。V/B/S 0/0/0，旧行为与材质 pin 不改。`8ec6415f` 已推送；该提交正文把 M3 的 18 误写为 17，此处按原始日志更正，不冒充再次测量。

### 第四批：卸载失败的空间与焦点边界

7. 现象：卸载按钮的外置焦点环右侧、底侧被 Scroll 内容边界裁切，短窗口自动露出按钮时仍缺底边。改动：17u 内边距从 panel 移到原 Scroll，内容位置和总高不变；动作行保留 3px 环的露出范围，负 margin 抵消占高，只在焦点目标或错误结果变化时 ScrollIntoView。验证：640/300 高 × 深浅，两条直边都读到原蓝环；滚轮仍可把焦点按钮滚出视口，避免每帧抢滚动。原红串 `focused uninstall ring clipped`，旧边外像素浅 `{213 213 214 255}` / 深 `{18 18 21 255}`，新为浅 `{43 99 175 255}` / 深 `{99 134 204 255}`。
8. 现象：卸载错误带直接接动作行，带边与焦点环碰在一起，实测间距 0。改动：仅错误存在时留 `Space(2)=7.2px`；清错同时撤回。验证：短句/185 字符长句 × 深浅，间距精确 7.2px，清错后的按钮坐标与从未报错帧一致。旧红串 `failure band touches answers: gap=0.000 want=7.200`。

第二轮自查修正：焦点环采样应落在 2px 蓝环内而非 1px 留白带，新增 pin 的取样点纠正为框外 +2px；旧断言未动。首次修复在短窗口仍漏底边，补露出范围；每帧 ScrollIntoView 会干扰手动滚轮，最终限制为焦点/错误变化并增加真实滚轮断言。最终定向回归 **86 PASS / 0 FAIL**；M4 正式使用最终 pin 回注旧产品后实跑，恢复 MD5、touch 后复验，原始日志随批提交。

第四批 `5e2fb023` 已推送，M4 恢复后 **10 PASS / 0 FAIL**，settingsui vet 通过。四批各两项；产品固定后开始普通/verbose 全量与三平台门槛。

## 偷了什么

真实读取 qiaomu-design SKILL → user-preferences → craft-loop / preflight / chinese-typography。唯一方向按任务书直接执行，不开四方向网页。Apple DESIGN 取单一交互色、2px focus、8px 常规圆角三个具体角色：落到已有主题 Focus、框架 2px 环 + 1px 间隙、RadiusSM=8，保留本仓字体与液态玻璃合约。

- craft-loop 菜单 **#3 品牌 focus 环**：保持一个当前键盘目标；外置范围 **3px**，短窗口也画完整。取消归还焦点额外等待 **0ms**。
- **#4 签名交互**：Tab 到确认、Space 取消/重开、失败重试；取消派发 **0 次**，两次明确确认派发 **2 次**，忙态重复 Space 不增加次数。
- **#6 编辑细节**：提示带两侧像素边距 **11/11px**（原 **11/-11px**），卸载失败与动作间距 **0→7.2px**，清错后空隙 **0**。
- **#12 材质细节**：停用失败使用已有警告环，普通确认保留既有 1px 受光边；新增材质层 **0**、新增阴影 **0**。没有为凑五项菜单添加装饰，用户“修现有功能”优先。

为什么更轻、更有空间感：面有完整边界、错误与按钮之间有清楚留白，焦点始终有归属。只恢复已有层次，没有堆更多玻璃。

## 两轮看图

第一轮真实事件与截图看到：Tab 困住取消、取消后落空、提示带右移、错误带贴住断裂的焦点环。第二轮修正短窗口自动露出范围，并用滚轮断言确认不会为了保环而锁住滚动。

四视角：艺术总监检查错误与按钮是否挤在同一边（加入 7.2px）；工程师检查右沿与 2px 环（修左右 margin 与裁切）；任务用户检查取消后的下一次 Space（回到入口）；读者检查长句是否挤掉动作（长短按钮宽度相等，不把已正确行为算修复）。4× 细节图可见原来缺右边和底边的蓝环现在闭合，错误带与环不再相碰。

深浅原图 `/tmp/floter-P13-hu-*` 已入本目录，左右图统一为旧→新：

| 场景 | 浅色 | 深色 | 图上可见变化 |
| --- | --- | --- | --- |
| 停用 Tab | [对照](P13-hu/disable-tab-en-light-comparison.png) | [对照](P13-hu/disable-tab-en-dark-comparison.png) | 蓝环从取消移动到停用 |
| 移除 Tab | [对照](P13-hu/remove-tab-en-light-comparison.png) | [对照](P13-hu/remove-tab-en-dark-comparison.png) | 蓝环可落到移除动作 |
| 停用取消 | [对照](P13-hu/disable-cancel-light-comparison.png) | [对照](P13-hu/disable-cancel-dark-comparison.png) | 原开关重新有焦点环 |
| 移除取消 | [对照](P13-hu/remove-cancel-light-comparison.png) | [对照](P13-hu/remove-cancel-dark-comparison.png) | 原移除按钮重新有焦点环 |
| 提示带边界 | [对照](P13-hu/band-true-420-light-comparison.png) | [对照](P13-hu/band-true-420-dark-comparison.png) | 右沿收回，与左边距相等 |
| 停用失败 | [对照](P13-hu/disable-error-light-comparison.png) | [对照](P13-hu/disable-error-dark-comparison.png) | 警示标、句子与环转为已有错误墨 |
| 卸载焦点 | [对照](P13-hu/uninstall-focus-640-light-comparison.png) | [对照](P13-hu/uninstall-focus-640-dark-comparison.png) | 蓝环右/下边完整，错误与按钮分开 |
| 短窗卸载 | [对照](P13-hu/uninstall-focus-300-light-comparison.png) | [对照](P13-hu/uninstall-focus-300-dark-comparison.png) | 滚到按钮时蓝环底部仍可见 |

[4× 焦点与错误间距](P13-hu/focus-detail-comparison.png)。图片来自 Linux ui.Tester 原生渲染；没有冒充 macOS/Windows 真机或浏览器截图。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，保留近黑主题 |
| A2 非系统字体 | 不适用；本仓既有字体与中文系统字体优先 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生桌面确认 |
| A5 三等分/卡片海 | 无新增卡片 |
| A6 假数据 | 无产品假指标；测试名和错误为夹具输入 |
| A7 空词/感叹号 | i18n 文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 现有页头完整，未改 |
| A10 Bento | 不适用 |
| A11 左竖线 | 无新增 |
| B1 读取/拨盘 | 克制、即时、保持原密度 |
| B2 单主题 | 深浅分别渲染，明确设置 Theme |
| B3 记忆点 | 错误、动作与焦点都有完整边界 |
| B4 动效气质 | 焦点即时交接，无新增动效 |
| C1 高频/目的 | 键盘路径不加等待 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 ≤300ms/按压 | 沿用 120ms 颜色与 0.97 按压 |
| C4 origin | 原 modal 居中；无新浮层 |
| C5 reduced-motion | 真实事件/像素使用 ReduceMotion=true，原降级保留 |
| C6 hover 媒体查询 | 不适用 Go 自绘，沿用指针路由 |
| C7 布局动画 | 无新增，原 uninstall Position 过渡保留 |
| C8 重播 keyframes | 无新增 |
| C9 手势/拖拽 | 无新增手势；真实滚轮可滚离焦点按钮 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 本轮无新增动效；验 reduced-motion 实帧，不声称慢放证据 |
| D2-1 中文字体 | 沿用系统字体，无 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px 未满足；Caption 9.9/Body 10.8 与旧行高沿用需拍板，不擅改 |
| D2-3 混排空格 | 产品 i18n 零改，中英文事件均验 |
| D2-4 标点/对比 | 文案零改；使用既有错误色，不声称所有旧墨达到 7:1 |

skill 通用五项工艺密度、网页 viewport 与字体换装不适用于本轮受限修复；没有为了这些条目增加系统或等待确认。可复现规则与 pin 落本仓 §6.196，不修改全局 skill 偏好。

## 仍待

下一轮首选内联确认的 Escape/定时收起出口与多条确认并存，先用真实事件复现再定修复；原 3s 过期窗口不改。运行参数全部移除/恢复继续保留为线索。字号/行盒、保存期间草稿归属及终端 S1b/S1c/S2 尾仍待拍板。本轮无 macOS/Windows 真机交互证据。

## 门槛

最终 **4083 PASS / 0 FAIL**（基线 4032，+51）；普通与 verbose 全量均退出 **0**。三平台 vet/build 已通过，gofmt 空，文档契约通过。V/B/S 旧断言修改 **0/0/0**，无新增 Skip。测试函数 **2888**（基线 2879，+9）。完整门槛后产品源码 MD5 再次核对通过。

证据：[普通全量](P13-hu/full-test.txt)、[三平台门槛](P13-hu/gates.txt)、[文档契约](P13-hu/docs-ci.txt)、[最终源码 MD5](P13-hu/final-source.md5)、[M1 红](P13-hu/M1-red.txt)、[M2 红](P13-hu/M2-red.txt)、[M3 红](P13-hu/M3-red.txt)、[M4 红](P13-hu/M4-red.txt)。基线与完整 verbose 保留在 `/tmp/floter-P13-hu/`。

过程记录：13:58 UTC 开盘，14:25 UTC 全量完成，约 28 分钟，低于 45–75 分钟目标时长；八项均已完成，未用重复运行或无关改动凑时长。四批每两项通过定向回归/vet 后立即提交推送，完整门槛固定产品后集中跑一次；没有声称每批跑了完整三平台测试。实际工具调用控制在 40 次以内，收尾不再扩展选题。
