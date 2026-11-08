# P13-kd · 内容动作恢复时看得见编辑出口

开盘4330be43，pull已同步、工作树干净。首个明确时钟07:30:02 UTC，阅读在此前。给定done.log不存在。Codex独立执行，未切模型、未委派。

## Stash清点

八条与P13-kc相同，无新增stale-kill。逐条reverse-check exit1，交接账明确均为早期独立WIP，不把旧主题/Tauri覆盖当前实现；本轮保留原件，无静默丢弃。

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

先复现，再逐条追加现象→改动→验证。完整开盘基线仍在运行，不沿用旧报告数字充数。旧V/B/S断言修改目标0/0/0。

1. 程序选择失败：原按钮持焦后滚到声明页尾，再Enter遇到选择器错误，程序框返回焦点但Y=-1152.0001、视口Y=51.399998 → 只在原按钮实际返焦时ScrollIntoView → 深浅事件都显示原程序框，警告恰好1次，直接键入修改路径，滚轮自由。
2. 程序选择取消：同样的长表单Enter取消，编辑出口在屏外 → 返焦时一次显示原框 → 草稿原样保留、警告0次、选择器仅派发1次，直接键入有效。

第一批定向6 PASS / 0 FAIL / exit0，gofmt空、settingsui vet净。M1回注旧settingsui.go，四个深浅场景实红`editing outlet outside viewport: rect={X:386.8 Y:-1152.0001 W:240 H:30}`；MD5逐字节还原成功，touch复验6 PASS。浅色after目视：程序框和蓝色插入点出现在正文顶端，下面仍是原命令与参数行，无新增面层。原图暂存/tmp，最终归档深浅对照。

第一批e75f97b0已推送。

3. 程序选择成功：长表单页尾按Enter选定路径，路径更新但程序框仍Y=-1152.0001 → 同步返回原框时显示它 → 深浅新路径精确为/usr/bin/forecast，派发1次、警告0次，可直接改为/usr/bin/corrected。
4. 程序选择能力撤回：ChooseProgram变nil后按钮消失，返焦原框仍屏外（X=449.8、Y=-1152.0001）→ 仅实际拥有焦点的退役按钮显示原程序框 → 深浅回调0次、原路径保留、直接编辑与自由滚轮通过。

M2回注第一批产品，四场景实红`editing outlet outside viewport`；MD5逐字节还原，touch后四条程序选择测试12 PASS / 0 FAIL / exit0。settingsui vet净；旧V/B/S断言0/0/0。

第二批4318c08a已推送。开盘完整基线6243 PASS / 0 FAIL / exit0；4330be43的cmd/internal测试函数3459。运行结束时对工作树计数会包含新增测试，因此函数基线按开盘树复核。

5. 脚本导出退役：宿主撤回exporter或脚本清空后，原编辑器接住键盘但在Y=-1023.9002 → 只在原导出按钮持焦时显示原代码编辑器 → 两种退役×深浅，原内容/空内容保留、派发0次，直接键入与自由滚轮通过。
6. 导出调用返回：同步/异步导出从页尾Enter后原返焦目标在同一屏外位置 → 不改变返焦时机，只显现原脚本框 → 两入口×深浅精确导出原文件名与字节，派发1次；异步等待保持，完成不覆盖新输入。

导出夹具最初从代码编辑器按Tab，Tab被编辑器消费，没到按钮；该失败仅属夹具，保留M3-fixture-error.txt，不算产品/mutation。改为绑定的按钮设起点后Tab往返，使用真实Enter与状态刷新复现。

M3回注第二批产品，八场景实红`editing outlet outside viewport: rect={X:201.8 Y:-1023.9002 W:487 H:64.8}`；MD5还原、touch后含既有导出等待/重试回归共18 PASS / 0 FAIL / exit0，settingsui vet净。V/B/S旧断言0/0/0。

第三批ebd46d4d已推送。

7. 内容复制退役：command/plan/script能力撤回、command/script变空时，原源字段返焦但在Y=-1097/-1152/-1023.9 → 各动作只在实际返焦时显示自己的字段 → 五种场景×深浅，直接输入落到各自草稿；复制派发0次、确认0条、滚轮自由。plan始终是有效JSON，没有捏造空plan场景。
8. About深链接复制撤回：短窗口先聚焦第二条复制，再滚回顶部，宿主撤回CopyText；链接焦点在Y=369.6而视口底300 → 保留按URL分开的返焦，显示原链接凹槽 → 两条链接×深浅，选择目标可见、Tab可退出、不派发复制、不显示虚假确认。

第一次只显示链接文字仍报`rect={X:208.6 Y:285.00003 W:399.80002 H:15}`，占用像素跨过底300；不加容差，改为显示含内边距的现有chip，严格像素包含通过。此中间失败保留M4-incomplete-reveal.txt。

M4两源文件回注第三批产品，14个深浅场景实红`editing outlet outside viewport`；两文件MD5还原、touch复验，加不抢焦点10场景共27 PASS / 0 FAIL / exit0，settingsui vet净。其他编辑器仍持焦时500px偏移精确保留、名称输入有效。旧V/B/S断言0/0/0。深色导出after目视：代码首行与插入点出现在正文顶端，原命令/版本在下面；浅色链接after目视：register文字及蓝色选择环完整进入凹槽，复制按钮退淡。
