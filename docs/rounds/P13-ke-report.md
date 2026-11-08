# P13-ke · 运行与配置表单退出时的可见目标

开盘1e0bec57，pull同步、工作树干净。首个明确时钟08:08:04 UTC，读取在此前。Codex独立执行，未委派或换模型。给定done.log不存在。

## Stash清点

活动8条与P13-kd相同，无新增stale-kill；逐条reverse-check exit1。交接账已判为早期独立WIP，不以反向失败当作独有工作的证明，也不把旧主题/Tauri链覆盖当前实现。原件全部保留。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap/主题，reverse1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri发布链，reverse1 | 保留 |

## 增量实施

事件复现后逐项追加。旧V/B/S断言目标0/0/0，完整基线运行中，未用上轮数字充数。

开盘完整基线实测6292 PASS / 0 FAIL / exit0，函数3468。

1. 当前运行参数被移除：原Cancel已持焦但在Y=1156.2、绘制面积0 → 原返焦同时一次ScrollIntoView → 深浅Cancel完整可见、其他答案12不变、自由滚轮后Enter仍可取消。
2. 当前参数改型：参数1仍持焦但Body.Y=1515.3时绘制面积0 → 替代控件原Focus同时显现 → 深浅新数字框可直接输入24，其他答案不变，滚轮不被锁住。最初夹具停在顶部导致改型目标本来可见，不算缺陷；改为合法滚到页尾后实红。

第一批定向6 PASS / 0 FAIL，settingsui vet净、gofmt空。M1回注开盘settingsui.go，四个深浅场景实红`run outlet outside viewport`（Cancel Y=1156.2；改型scroll=1515.3）；MD5逐字节还原成功、touch复验6 PASS。旧V/B/S断言0/0/0。

第一批1c3c7f3c已推送；浅色after目视：参数1的新数字框与插入点在正文顶端，原紧凑参数行与侧栏保持。

3. 运行提交变为不可用：原Cancel接焦后仍在Y=1216.2、绘制面积0 → 只在原Run失去可用性时显现Cancel → 深浅退出目标可见，未派发运行；恢复运行状态后Tab/Enter恰好提交一次，12个答案准确保留。
4. 取消运行表单：长表单折叠后原Run已持焦但scroll=689.30005处被完全裁剪 → 原返焦同时显现 → 深浅原入口可见，Enter能重开同一草稿，取消本身不运行。

第二批定向6 PASS / 0 FAIL，settingsui vet净、gofmt空。M2回注第一批产品，四场景实红`run outlet outside viewport`；MD5还原、touch复验6 PASS。旧V/B/S断言0/0/0。
