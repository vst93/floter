# P13-hp 报告：配置编辑的冲突与恢复

开盘 035ca0bd，pull 最新，树干净。中文 locale 基线 3893 PASS / 0 FAIL / 2832 测试函数（全仓 Test 声明统计）。
qiaomu-design 与指定参考已读；不调用子代理或外部模型。done.log 开盘不存在。

## 抢救 stash 清点

八条 reverse-check 均退出 1，SHA 与 ho 相同，git stash show --stat 已核对。
backlog 明确这八条是早期独立 WIP，stale-kill 抢救稿已归档/接续；不把 reverse 失败当作应强行覆盖产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | P7 keycap 未完稿 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/滑块稿 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/探针 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段稿 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽稿 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键形状稿 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏行为稿 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 Tauri 发布稿 | 保留 |

## 逐项验收

1. 文本先本地编辑，再收到外部冲突，回到外部值仍显示未保存 → 冲突只保留草稿、saved 更新为新宿主值 → 深浅真实输入，回到 external 为 Saved、回到 old 为 Unsaved。
2. 数字 1e 遇外部 40，恢复默认 5 却显示 Saved → 同一宿主基准用于默认恢复和合法数值比较 → 半输入保留、恢复默认仍 dirty、输入 40.0 后 clean。

M1 回退 config.go，两项深浅全红；原文 `returning to external text still says Unsaved changes`、
`restoring obsolete default falsely says Saved`。MD5 还原成功，touch 后相关测试与 vet 通过；
日志 mutation1.txt、batch1-after.txt。V/B/S 0/0/0，旧断言零改动。

第一批相关 93 PASS；0c0d38d5 已推送。

3. 插件文本改走再改回原文、不按回车，外部刷新被旧草稿挡住 → 按上一代宿主值辨认撤回 → 深浅复制得到 external、0 次保存且焦点仍在。
4. 外部已写入同一份插件草稿，后续值仍被旧文字挡住 → 新宿主代际同值确认后退役草稿 → 下一代 latest 可见，真正冲突的 unsent 继续保留。

M2 回退 pluginsheet/settingsui 两文件，原文 `withdrawn plugin text masks host`、
`host-confirmed plugin text masks next value`；MD5 恢复并 touch，相关回归与 vet 通过。V/B/S 0/0/0。

第二批相关 99 PASS；060ae37e 已推送。

5. 集成数字存 1e308 却展开 309 个字符，槽内只见尾零 → 采用已有数字显示格式 → 深浅复制完整 1e+308，dirty=false，宿主值不变。
6. 插件无步长数 1e-308 展开 310 个字符 → 非零绝对值小于 1e-6 用科学计数法 → 完整 1e-308，最小次正规数、负数和阈值逐值往返解析，0 次写入。

M3 原文 `integration number expands to 309 characters, want 1e+308`、
`tiny number expands to 310 characters, want 1e-308`。MD5 还原、touch 后相关 105 PASS，vet 通过。
V/B/S 0/0/0；第三批只改显示，保存精度与步长不变。

侦察撤回：关闭插件浮层两帧后重开，文本/数字 Ctrl+Z 均不复活旧输入，四场景已 PASS；不计修复。

第三批 a1554b73 已推送。

7. 多选恢复空默认后 Shift 点击应用，已清掉的文件又被选上 → 恢复有默认值字段时结束其旧范围锚点 → 深浅结果仅 [应用]，不再 [文件 应用]。
8. 外部把选择从文件换成书签，Shift 点击应用又选回文件 → 未编辑字段采用新宿主值时结束旧范围锚点 → 深浅结果 [应用 书签]，不再 [文件 应用 书签]。

M4 回退 config.go，原文 `defaults-range resurrected old range: [文件 应用], want [应用]`、
`external-range resurrected old range: [文件 应用 书签], want [应用 书签]`。
MD5 还原并 touch；本批相关测试和多选既有范围/键盘/刷新测试通过，settingsui vet 通过。V/B/S 0/0/0。

第四批相关 117 PASS、既有多选回归 57 PASS；9fffba1c 已推送。

## 偷了什么

从 `craft-loop.md` 工艺密度菜单第 6 项“编辑细节”取读数与状态收口：309/310 字符各收成 6 字符，
阈值为绝对值 ≥1e21 或非零且 <1e-6；两条路径的过期未保存/已保存状态各归零。
第 3 项“focus 环”保留已有 1px 语义边，外部刷新不偷焦点；第 12 项“材质细节”保留 8u 槽圆角、1px 边，新增透明层 0。
后两项为保留验收，不充当新修复。Apple 供体的单一交互色、8px 圆角与约 0.48 弱化映射原有 accent/RadiusSM/框架 0.5。
当前用户限制功能扩张，故没有为五项工艺密度硬加装饰。缺陷和防复发 pin 落在本仓 §6.191，没有改写公共技能偏好。

## 截图与两次复核

32 张 ui.Tester 原图（720×320 集成、720×220 插件）及 8 张前后对照板在 `P13-hp/`，原图同时保留于 `/tmp/floter-P13-hp-*.png`。
第一次看 1-light/2-dark/3-light/4-dark，第二次看各自另一外观；全部逐张检查。
可见变化：1 板红色 Unsaved 与灰色 Saved 按当前宿主值交换；2 板 /old→/external、/shared→/latest；
3 板槽中的长串零变成完整 1e+308 与 1e-308；4 板文件勾选清除，应用及外部书签的选择保留。

四视角：艺术总监关注过期警告抢焦点（1/2）；工程师关注状态与真实宿主值相反（1–4）；
任务所有者关注恢复后误选文件（7/8）；读者关注几百位数字只剩零（5/6）及既有小字号。
before 是注入旧实现的同事件帧，after 是恢复后同事件帧。第二次复核未发现新样式缺陷，不额外装饰。
没有声称已做 Windows/macOS 真机 GUI 或显示器透明合成验收。

## Preflight 逐项

A：A1 无新增纯黑/紫蓝渐变/霓虹；A2 沿用系统字体，原生中文规则优先于网页换字体建议；A3 无斜体；
A4 无 Hero；A5 无新增卡片嵌套/三栏；A6 无假用户或营销数据；A7 无新营销词与感叹号；
A8 产品无编号眉批；A9 无 H1；A10 非 Bento；A11 无装饰左竖线。
A2 工艺密度未增五种新动作，遵循用户修缺陷范围；不伪报全部技能门槛通过。

B：B1 冒险 2/动效 2/密度 7 一致；B2 每帧单一外观；B3 记忆点是槽内值完整、状态可信；B4 即时反馈与安静材质一致。

C：C1 高频输入即时；C2 无新增 all/ease-in/scale(0)；C3 沿用 120ms 色阶与按压；
C4 无新弹层；C5 测试 ReduceMotion；C6 原生事件、CSS hover media 不适用；C7 无新增几何动画；
C8 无 keyframes；C9 未改手势；C10 无 Framer Motion；C11 无新增动效，不声称设备慢放验收。

D2：D2-1 中文系统字体、无外部字体；D2-2 **未通过**，Body 10.8px/Caption 9.9px 和 620/650 字重沿用既有需拍板；
D2-3 无新增中文产品句，ASCII 数字与原单位空格保持；D2-4 无新标点问题，暗色沿用 token，未证明全页正文 7:1。
用户已要求将不满足项记录而继续可授权修复，因此这些既有差异不成为等待确认的理由。

## 仍待

1. 下一轮优先真实复现插件字段跨分组移动/前置整组增删的焦点与草稿；当前只有父级身份的代码线索，尚未计作 bug。
2. 插件无步长极小值的宿主存储精度不在本轮范围；显示可往返不等于宿主支持任意步长，原拍板项继续保留。
3. 字号/行盒、accent 禁用填充、保存期间草稿归属、未知 Radio、多选宿主过滤、终端 S1b/S1c/S2 尾沿用需拍板。
4. 八条早期 WIP 保留，抢救稿没有新增；本轮没有接入已归档的旧变更。

## 门槛

三平台 vet/build 均退出 0，gofmt 输出 0 字节；最终普通与 verbose 全量均退出 0。
最终 **3923 PASS / 0 FAIL / 2842 测试函数**，基线 3893 / 0 / 2832，增加 30 PASS 与 10 个函数。
V/B/S 0/0/0，旧测试零修改、无新增 Skip；新增 8 条修复 pin 和 2 条重开撤销排除 pin。
四组 mutation 均实跑红、逐字节 MD5 还原并 touch 复跑。分批回归 93→99→105→117 PASS，范围回归 57 PASS。

最终日志：[gates.txt](P13-hp/gates.txt)、[普通全量](P13-hp/full-test.txt)、[变异 1](P13-hp/mutation1.txt)、
[变异 2](P13-hp/mutation2.txt)、[变异 3](P13-hp/mutation3.txt)、[变异 4](P13-hp/mutation4.txt)。
完整 verbose 保留 `/tmp/floter-P13-hp/verbose-final.txt`；CI 文档契约在 §6.191 更新后复跑通过。

深浅前后板：[状态·浅](P13-hp/comparison-1-light.png) / [状态·深](P13-hp/comparison-1-dark.png)；
[文本·浅](P13-hp/comparison-2-light.png) / [文本·深](P13-hp/comparison-2-dark.png)；
[读数·浅](P13-hp/comparison-3-light.png) / [读数·深](P13-hp/comparison-3-dark.png)；
[多选·浅](P13-hp/comparison-4-light.png) / [多选·深](P13-hp/comparison-4-dark.png)。
