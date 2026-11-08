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

第四批d78e5ba5已推送。八项四批产品全部完成，完整门槛进行中。测试函数3477（开盘3468，+9），旧函数删除0、旧断言修改0、新增Skip0。完整普通/verbose测试各一次独立并行执行，平台门槛另一可追踪会话执行。

## 偷了什么

- craft-loop工艺密度菜单 **#3 品牌focus环**：原返焦目标 **1个**，显现后占用像素须完整进入正文视口，容差 **0px**；原蓝环与插入点一同可见。
- **#4 签名交互**：每次原返焦事件只请求 **1次**最小滚动，新增动画 **0ms**；显现后滚轮可继续移动。非当前表单五种刷新×深浅的 **100px** 偏移严格不变。
- **#12 材质细节**：保留原 **1px** 受光边、**8px** 通用矩形圆角与既有胶囊轮廓，新增玻璃/阴影 **0层**。4×局部中输入框的原边线、蓝环和插入点均存在，未靠加框表达恢复。
- Apple DESIGN供体取单一交互accent、8px矩形圆角、1px边职责，映射本仓既有token。用户限定既有功能打磨优先于通用工艺菜单凑五项；未引入氛围、插图或多余装饰。

这轮为什么更轻、更有空间感：退出或替换控件时，原键盘目标回到可见的内容层，用户能在原表单位置继续操作；面层、阴影和信息量都不增加。

## 两轮采样与视觉复核

每批先在旧产品上驱动真实Tab/Enter/Space与字段刷新，导出before，再做最小显现修复并导出after；正式mutation重新回注旧代码，重新生成前图，MD5还原后重拍后图。最终又逐项看深浅总览与4×焦点局部，不把只有测试数字当作视觉完成。

四视角：艺术总监看到旧图的工作焦点消失在屏外；工程师看到持焦控件被裁成0×0，不能用“focused=true”代替可见；任务所有者在取消后找不到重开入口；读者在改型后看不见当前值12。修复后分别复查原蓝环、像素包含、Enter/Space重开、直接输入24，并检查后续滚轮。没有为二次检查强行改版。

**32张720×520原始PNG、16组深浅对照、2总览、2张4×局部**归档于本轮目录；原图来自Linux ui.Tester完整设置页。最小变化58766像素，最大235304像素（均含滚动后的内容变化），详见[实数](P13-ke/image-differences.json)。4×为最近邻放大，非原生高DPI截图。前图是对应事件执行后未修产品的真实结果，不是手绘效果图。

| 项 | 图上看到的一处变化 | 浅色 | 深色 |
| --- | --- | --- | --- |
| 1 参数撤回 | Cancel的蓝环在正文底部出现 | [对照](P13-ke/field-removed-light-comparison.png) | [对照](P13-ke/field-removed-dark-comparison.png) |
| 2 参数改型 | 当前数字12与插入点回到正文顶部 | [对照](P13-ke/field-kind-light-comparison.png) | [对照](P13-ke/field-kind-dark-comparison.png) |
| 3 提交不可用 | 可用Cancel与退淡Run一起出现 | [对照](P13-ke/submit-unavailable-light-comparison.png) | [对照](P13-ke/submit-unavailable-dark-comparison.png) |
| 4 取消表单 | 原Run入口蓝环回到正文顶部 | [对照](P13-ke/form-cancel-light-comparison.png) | [对照](P13-ke/form-cancel-dark-comparison.png) |
| 5 外部关闭 | 原运行入口可见，后续工具仍在同一滚动列 | [对照](P13-ke/form-closed-light-comparison.png) | [对照](P13-ke/form-closed-dark-comparison.png) |
| 6 参数清空 | 可执行的无参Run入口可见 | [对照](P13-ke/form-empty-light-comparison.png) | [对照](P13-ke/form-empty-dark-comparison.png) |
| 7 配置取消 | Configure蓝环与保留的mine草稿可见 | [对照](P13-ke/config-cancel-light-comparison.png) | [对照](P13-ke/config-cancel-dark-comparison.png) |
| 8 配置丢弃 | 收起后的Configure蓝环出现在正文顶端 | [对照](P13-ke/config-discard-light-comparison.png) | [对照](P13-ke/config-discard-dark-comparison.png) |

[浅色总览](P13-ke/overview-light.png)、[深色总览](P13-ke/overview-dark.png)。4/5/6有相同像素结果，但触发、草稿归属和下一次Enter的命令行为不同，分别有事件断言；未将深浅重复算作修复。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/黑/霓虹 | 无新增装饰，原语义主题保留 |
| A2 字体 | 中文系统字体规则优先，未为换风格换字体 |
| A3 斜体 | 无新增斜体 |
| A4 Hero | 功能设置页，不适用 |
| A5 卡片 | 原分组沿用，未加容器 |
| A6 假数据 | 天气工具与参数为测试夹具，不进产品 |
| A7 空词/感叹号 | 产品文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原设置标题保留，图中可见 |
| A10 Bento | Go设置表单，不适用 |
| A11 装饰竖线 | 无新增 |
| B1 设计读取/拨盘 | 功能表单、稳定3/即时2/紧凑7一致 |
| B2 主题 | 深浅分别锁定，不混主题 |
| B3 记忆点 | 退出之后看得见下一步键盘目标 |
| B4 动效气质 | 新增0ms，高频恢复即时 |
| C1 目的/频率 | 只伴随原返焦，一次显现，非当前刷新不追随 |
| C2 all/ease-in/scale/fade | 无新增 |
| C3 时长/按压 | 沿用原按压面，无新过渡 |
| C4 origin | 在原正文显现，不新增浮层 |
| C5 reduced-motion | 新事件使用ReduceMotion=true，无新动画 |
| C6 hover媒体 | Go自绘，不适用CSS媒体查询 |
| C7 布局动画 | 无新增 |
| C8 keyframes | 无新增 |
| C9 手势 | 原滚轮保持自由，无新手势 |
| C10 Framer | 不适用Go |
| C11 慢放 | 无新动画，只验确定帧，未声称真机慢放 |
| D2-1 字体 | 系统中文字体，无新增webfont |
| D2-2 字号/行高/字重 | Caption9.9、正文10.8仍不满足通用≥14px/1.5，沿用需拍板；本轮未擅改高度链 |
| D2-3 混排 | 产品文案零改，夹具不作为产品文案 |
| D2-4 标点/对比 | 原标点与墨保留，未声称全部次要墨达到7:1 |

项目缺陷沉淀为§6.258和事件pin，没有升级为通用审美偏好或改写全局skill。新系统、依赖、外部模型、外发消息均0。Windows/macOS仅编译验证，未做原生设备交互。

门槛过程记录：首次平台会话在linux vet/build与windows vet exit0之后以143退出，windows build尚无exit文件，不计完整通过；保留已有日志，剩余检查单独补跑。原因未由工具返回，不归因于产品或假称通过。

## 仍待

1. 下一轮首选已有运行结果的折叠/展开键盘出口与长输出选择：先真实事件复现，不改输出内容、保存与会话语义，不重复本轮参数/配置返回显现。
2. 生成工具编辑/转换覆盖、重复新包ID覆盖、重装事务、配置模板失败重试、另一端冲突、字号/行盒、accent禁用填充、未知多选/Radio、极小步长、176-9/161-4保存期草稿、终端S1b/S1c/S2尾继续需拍板。
3. 八条早期WIP原件保留；历史ImageCopy/Rescan/memo线索不弱化B断言、不加睡眠或容差掩盖。

## 最终门槛与边界

最终实测 **6327 PASS / 0 FAIL / 3477函数**，开盘6292/3468，增加 **35 PASS / 9函数**。普通全量与verbose全量均exit0；gofmt输出0字节，linux/windows/darwin的vet/build六项最终均exit0，文档CI exit0。原始日志与exit文件在本轮目录；首次平台会话143不算通过，补跑的三项均已实际结束。

M1–M4每组四个深浅修复场景实红，逐文件MD5还原、touch后-count=1复验。M4额外十个非当前刷新场景通过；最终产品MD5也再次核对成功。产品四批1c3c7f3c/fe136ecc/95396bfb/d78e5ba5均已推送，结案仅归档证据与交接。

完成时钟：2026-11-08 08:30:52 UTC。首个明确时钟08:08:04 UTC，阅读在此前；本轮短于45–75分钟目标，八项修复与全部门槛完成后不以额外改动或重复测试凑时长。仅对真实中断的剩余平台项目补跑，未重跑已通过的全量门槛。

Windows/macOS仅vet/build，未做原生设备交互；Linux自绘图覆盖八项×深浅的16组前后状态，未将像素变化等同于跨平台真机效果。需拍板项和八条早期WIP均保留。

证据：[基线](P13-ke/baseline.txt)、[普通全量](P13-ke/full-test.txt)、[verbose全量](P13-ke/full-verbose.txt)、[深色总览](P13-ke/overview-dark.png)、[浅色总览](P13-ke/overview-light.png)。
