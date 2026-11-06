# P13-hu：确认带的键盘出口与可见边界

开盘 `f3514951`，pull 最新，工作树干净。done.log 与 stash-backlog 文件在本机不存在。中文 locale 基线 **4032 PASS / 0 FAIL / 2879 测试函数**。逐批落盘，完成项才记为修复。

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
