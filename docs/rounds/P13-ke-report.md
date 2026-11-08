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

第二批fe136ecc已推送。

5. 运行表单外部关闭：当前字段消失，原Run返焦在scroll=689.30005下绘制面积0 → 仅run表单实际返焦事件显示原入口 → 深浅入口可见、原答案mine保留、Enter可重开，没有命令派发。
6. 全部运行参数撤回：原Run变为无参运行入口，焦点仍藏在页外 → 同一已有返焦事件显示原入口 → 深浅Enter恰好调用一次当前ID，参数严格nil，未将保留的会话答案送入命令。

第三批定向6 PASS / 0 FAIL，settingsui vet净、gofmt空。M3回注旧noticefocus.go，四场景实红`run outlet outside viewport`；MD5逐字节还原、touch复验6 PASS。其他notice种类沿原规则，旧V/B/S断言0/0/0。

第三批95396bfb已推送。

7. 取消配置撤回确认：长配置列表保留草稿后Configure已持焦，但scroll=1270.3处绘制面积0 → 原返焦同时显示Configure → 深浅入口可见，草稿mine及打开态保留，无保存；Space可重新打开确认。
8. 确认配置撤回：丢弃后Configure已持焦但scroll=718.30005处被裁掉 → 原返焦同步显示入口 → 深浅入口可见，原存储值stored恢复，无保存；Space可重开配置。

第四批含两个修复与十个非当前刷新保护场景，17 PASS / 0 FAIL，settingsui vet净、gofmt空。M4回注第三批settingsui.go，四个配置深浅场景实红`run outlet outside viewport`；MD5还原、touch复验17 PASS。用户已转去Close时五种运行刷新×深浅均保持原焦点与100px偏移，不追随后台变化。旧V/B/S断言0/0/0。
