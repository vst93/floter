# P13-kc · 声明变动与模式退出的可见编辑目标

开盘27e48ad1，pull已同步、工作树干净。Codex独立执行，未换模型/执行器。指定done.log开盘不存在。首个时钟06:50:01 UTC，阅读此前已开始。

## Stash清点

八条活动stash与前轮SHA一致，逐条reverse-check exit1，无新增stale-kill；已有抢救账明确它们是早期独立WIP。保留原件，不把旧主题/Tauri覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse1 | 保留 |

## 实施记录

先用完整设置页事件复现八条线索，未复现不计修复。基线运行中；函数3449；旧V/B/S断言修改目标0/0/0。逐批实测后追加。

开盘完整基线 **6211 PASS / 0 FAIL / exit0，3449函数**。删除唯一声明原实现通过，不计修复；中项/尾项分别验证不同接替目标。初版测试在空列表索引-1崩溃，属于夹具错误已修；用长选项和真实滚动复核，不把它算产品缺陷。滚轮检查改在正文右侧空白，避免滚入textarea内滚动。

1. 新增：5条已有声明后新增，焦点留在Add，新行ID位于Y1290、视口底520，键入无效 → 新声明显示名接住键盘，整组字段一次滚入视口 → 深浅真实Tab/Enter、直接输入、后续自由滚轮。先只滚label时其同行ID计算到520.00006，严格像素包含pin红；改为显示整组字段，让标题与上下文一起可见，不加容差。

第一批定向 **3 PASS / 0 FAIL / exit0**，gofmt空、settingsui vet净。M1用HEAD旧产品注入，两个外观实红：`editing outlet lost focus: 显示名`、`editing outlet outside viewport`；MD5逐字节还原成功，touch后3 PASS。完整基线6211通过，最终全量门槛在全部产品完成后运行。

第一批 **354323db** 已推送。浅色图目视：新记录显示名的蓝环和插入点进入正文，标题可见；第二排字段仍可继续向下滚，不宣称整张参数板都可见。

2. 删除中项：Remove聚焦后滚到页尾再Enter，后继记录ID留在Y−1348.2002 → 沿原规则返焦后继，同时一次reveal其字段组 → 深浅按键删除、后继键入、自由滚轮通过。
3. 删除尾项：含20个城市选项的长声明删尾后，前驱ID留在Y−134.80005 → 前驱字段组一次reveal → 深浅删尾、前驱继续编辑通过。删除唯一声明另作回归，原来与现在都通过，不计修复。

第二批定向 **9 PASS / 0 FAIL / exit0**，gofmt空、settingsui vet净。M2四个深浅场景实红 `editing outlet outside viewport`；MD5还原成功，touch后9 PASS。旧V/B/S断言0/0/0。

第二批 **2ab81ce0** 已推送。

4. 活跃声明撤回：外部刷新撤回当前第三张记录，焦点回Add，但Add被裁成0×0 → 仅原活跃记录消失时，Add一次滚入视口 → 深浅撤回、Enter再增一条、自由滚轮通过。M3两场景实红 `editing outlet outside viewport: rect={X:675.8 Y:51.399998 W:0 H:0}`；MD5还原、touch后 **3 PASS / 0 FAIL / exit0**；gofmt空、settingsui vet净。

第三批 **f319d469** 已推送。

5. 选项字段退役：单选改为文本后原类型选择有焦点，但同行ID为Y−537.8002 → 只有原选项持有键盘时显示类型字段组 → 深浅退役、下拉键盘重选、选项草稿原样保留通过。
6. 占位提示退役：文本改为布尔后提示字段隐藏，原类型选择仍屏外 → 只在实际返焦时显示字段组 → 深浅退役、键盘切回文本、提示与选项保留通过。

第四批 **6 PASS / 0 FAIL / exit0**，gofmt空、settingsui vet净。M4旧产品四场景实红 `editing outlet outside viewport`，两个源文件MD5还原成功，touch后6 PASS。restore仅增加是否发生返焦的返回值，其他调用点保持原行为。深色尾删图目视：前驱显示名的焦点环位于正文顶部，城市选项留在同一张板内，侧栏未滚动。

第四批 **984fdea5** 已推送。

7. 程序模式退出：程序字段持焦、滚到页尾后收到脚本模式刷新，模式组返回焦点但停在Y−1209.2002 → 实际返焦时显示原模式组 → 深浅Tab/Space切回程序，程序和脚本草稿保持。
8. 脚本模式退出：代码编辑器持焦、滚到页尾后收到程序模式刷新，同样留下屏外模式组 → 实际返焦时显示原模式组 → 深浅Tab/Right切回脚本，已有代码保持。

第五批 **11 PASS / 0 FAIL / exit0**，gofmt空、settingsui vet净。M5四场景实红 `editing outlet outside viewport`，MD5还原成功、touch后11 PASS。额外四场景验证名称仍持焦时，后台类型/模式刷新不改变500px滚动偏移、不抢焦点、继续输入落到原名称。该防回归不计第九项。
