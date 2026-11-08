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
