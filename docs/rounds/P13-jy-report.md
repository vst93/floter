# P13-jy · 本地工具连接恢复

八项完成并分四批推送：**0ff958a2 / 08f2e09f / 961a75f4 / f132bc4c**。最终 **6069 PASS / 0 FAIL**，完整串行验收通过；并行失败与工具预算偏差如实记录，详见文末。

2026-11-08开盘，首个时钟04:04:15 UTC。HEAD6325a179，工作区干净，pull已最新。P13-jx已做完registry失败恢复，按其仍待继续本地工具连接。指定done.log不存在。规格见同目录P13-jy-spec.md。

## stash清点

本轮逐条实际reverse-check均1，完整SHA与交接相同。八条均是早期独立WIP，沿用交接审计保留，不用旧主题/旧Tauri链覆盖当前Go实现；无新增抢救稿、无drop。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap/主题 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/滑块 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42 | 旧字段/确认栏 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏 | 保留 |
| stash@{7} | 7a9f6e13 | 旧Tauri发布链 | 保留 |

## 增量记录

### 第四批：选择成功与提交后的键盘出口

7. **选择成功后不能立即修订路径**：现象→新路径已写入但焦点仍在选择按钮；改动→成功选择也返回程序输入；验证→键盘选择后路径为/usr/bin/forecast，直接全选键入得到/usr/bin/retry；新旧表单归属继续由已有回调测试守住。
8. **提交收起表单后键盘悬空**：现象→原提交按钮退场后没有可操作的焦点；改动→仅本次连接提交把焦点交给同表单收起后的添加入口，token变化则退役；验证→普通提交和等待提交两种、深浅两套，Enter直接打开下一份空草稿，完成等待不抢焦点。

定向59 PASS/0 FAIL/exit0、vet净。M4撤掉成功选择返焦点和提交返焦点，实红`picked picker did not return program focus`、`picked picker lost immediate keyboard edit: "/usr/bin/forecast"`、`submitted form lost keyboard exit: calls=1 open=false addFocus=false`；MD5还原并touch后59 PASS。旧V/B/S断言0/0/0、新增Skip0。

### 第三批：选择器错误与取消出口

5. **选择程序失败无反馈**：现象→原生选择器返回错误却被吞掉；改动→向已有宿主警告出口发LocalToolFailed，保留原路径，旧表单退出后不再发过期警告；验证→真实按钮回调返回os.ErrPermission，警告恰1条，完整宿主ui.Tester可见错误浮层；新编辑器替换后的错误不警告、不覆盖。
6. **取消选择后键盘留在按钮**：现象→取消后直接输入无法修订程序路径；改动→空选择返回程序字段；验证→Tab/Enter打开选择器、取消后直接全选输入，字段变为/usr/bin/retry，取消无警告。

定向51 PASS/0 FAIL/exit0，vet净。M3实红`program picker error warnings=[]`、`cancel picker did not return program focus`、`cancel picker lost immediate keyboard edit: "/usr/bin/weather"`及`program picker failure never reached host notice`；MD5还原、touch后51 PASS。旧V/B/S0/0/0、新增Skip0。第二批08f2e09f已推送。

中文基线完整完成：6029 PASS/0 FAIL/exit0、3413函数。第一批0ff958a2已推送。

### 第二批：提交失败与等待互斥

3. **提交失败**：现象→审核后声明文件消失，提交报错且草稿丢失；改动→Commit失败也走本表单恢复出口；验证→完整设置页选择程序、输入名称/命令、声明网络权限、Enter提交、审核中移除声明，失败后重新提交的请求严格相等。
4. **等待重复连接**：现象→第一次还在准备/审核，第二份完整草稿也能派发；改动→本地表单宿主入口互斥，折叠时在添加入口旁显示已有连接忙态，打开时提交按钮忙态；验证→等待中的完整第二份草稿不派发，完成后解锁；添加入口仍允许编辑新草稿，已检测工具的独立连接路径保持原样。

41 PASS/0 FAIL/exit0、定向vet净。M2撤回忙态和三处恢复，实红`busy connection dispatched 2 requests`与`commit recovery=0 settled=1, want one each`（准备/取消也再次实红）；两文件MD5还原、touch后41 PASS。前三个异常现已由完整设置页真实输入→磁盘/权限回调→重试覆盖，深浅出图。夹具最初把“程序”类型按钮当成同名输入，已改用实际选择程序入口；同名连接按钮改为从本表单用Tab到提交，不改产品来适配夹具。旧V/B/S断言0/0/0、新增Skip0。

首次基线LANG被LC_ALL=C.UTF-8覆盖，出现已记录的字体pin失败，已终止并用中文LC_ALL重跑；不修改旧断言。后续只记录完整退出的实测结果。

### 第一批：准备失败与权限取消

1. **准备失败**：现象→宿主拒绝不存在的程序后仅通知，表单已清空；改动→同步提交捕获本表单的恢复出口，失败在UI线程恢复原草稿；验证→真实磁盘失败调用恢复1次，ui.Tester程序/脚本两种表单逐字段保留（含参数、声明、平台、隐藏模式草稿和原始空白），再次提交请求严格相等。
2. **权限取消**：现象→拒绝权限后进入Commit并报“连接失败”，表单不可重试；改动→拒绝直接返回并恢复草稿；验证→真实PrepareCustom和拒绝回调，恢复1次、通知0条、仓库条目0；新输入、主动清空、折叠与另一个编辑器四种状态不被旧结果替换。

定向与既有回调契约共35 PASS/0 FAIL/exit0，定向vet净。M1撤回两条宿主恢复及表单出口，实红`failed connection lost complete draft: open=false name="" program="" script="" params=[]`、`prepare recovery=0 settled=1, want one each`、`cancel recovery=0 settled=1, want one each`；两文件MD5逐字节恢复，touch后35 PASS/exit0。旧V/B/S断言修改0/0/0、新增Skip0。夹具初次窗口1100px不足以点击1319.7px处提交，后改用既有1500px测试画布；另一初次把参数渲染前未初始化的key用于相等比较，已在首帧后取样。均不是产品mutation证据，未放宽产品断言。

## 验收过程限制

并行执行普通全量、verbose全量和race造成三条命令未通过：设置页两次默认10分钟超时，shell race也超时；超时栈仍在自绘文字渲染。verbose另有旧B测试TestAStaleMemoIsRefreshedBehindTheFrame实红 `the pass left the memo marked pending`。完整原始输出保存在parallel-attempt目录，不能把这些尝试算作通过。已保持源码和全部旧断言不变，改为串行重跑失败门槛；最终数字只在命令完整结束后补录。

工具预算偏差：本轮超过40次总工具调用；八项产品已在四批提交中保住，额外调用用于图像/完整验收、等待和失败后的重跑，没有继续增加产品选题。后续线索写入仍待。此偏差不包装为满足工具预算。

## 偷了什么

真实读取qiaomu-design的SKILL、user-preferences、craft-loop、preflight、chinese-typography，并读Apple DESIGN供体。读取为原生集成设置表单，视觉冒险3（可预期）、动效2（即时）、密度7（保留工具信息）。供体取单一交互蓝、8px矩形控件圆角、平面无影/浮层承载阴影三条职责，映射现有token；胶囊沿用自身圆角。明确方向下不做四方向预览，不调用外部模型。

- craft-loop工艺密度菜单 **#3 品牌focus环**：取消/选择成功返回1个程序输入，提交返回1个添加入口；额外等待 **0ms**，复用原accent焦点墨。
- **#4 签名交互**：同一连接表单在飞上限 **1次**，重复点击派发 **0次**；等待留在原连接按钮区域。
- **#12 材质细节**：延续 **1px**受光边，新增玻璃面和阴影 **0层**；忙态复用既有DimFace。

用户要求修已有功能优先，不为凑五项工艺添加装饰。**这轮为什么更轻、更有空间感**：草稿回到原表单、等待留在原按钮、取消不制造错误浮层；底、面、浮各承载有效内容，不再用一条通知替代整份丢失的上下文。

## 两轮图像复核

第一轮四视角：艺术总监看到准备/提交失败时只剩错误浮层、原表单消失；工程师看到输入与结果失联；任务用户无法连续输入；读者看到选择程序与路径编辑缺少连续焦点。第二轮收敛四处：原表单恢复、单处忙态、路径返焦点、提交返添加入口，没有推倒布局。

44张原始PNG、16组逐项对照、2张总览、6张最近邻4×局部；16组均有像素变化，最小 **552像素**。图来自Linux ui.Tester：完整宿主960×2400、表单720×1500；4×只是检查放大，不冒充原生高DPI。高画布用于检查已有长表单，不改变产品高度链。

肉眼复核：浅色取消后蓝环从选择按钮移到路径槽，光标仍在原路径末尾；深色提交后蓝环在“添加本机工具”；前三种失败的after均恢复原字段；选择器错误after才出现右上方原通知。忙态胶囊没有加玻璃层。字号与次要墨限制仍保留。

| 项 | 浅色前后 | 深色前后 |
| --- | --- | --- |
| 1 准备失败 | [图](P13-jy/connect-prepare-light-comparison.png) | [图](P13-jy/connect-prepare-dark-comparison.png) |
| 2 权限取消 | [图](P13-jy/connect-cancel-light-comparison.png) | [图](P13-jy/connect-cancel-dark-comparison.png) |
| 3 提交失败 | [图](P13-jy/connect-commit-light-comparison.png) | [图](P13-jy/connect-commit-dark-comparison.png) |
| 4 等待互斥 | [图](P13-jy/connect-wait-light-comparison.png) | [图](P13-jy/connect-wait-dark-comparison.png) |
| 5 选择错误 | [图](P13-jy/picker-error-light-comparison.png) | [图](P13-jy/picker-error-dark-comparison.png) |
| 6 选择取消 | [图](P13-jy/picker-cancel-light-comparison.png) | [图](P13-jy/picker-cancel-dark-comparison.png) |
| 7 选择成功 | [图](P13-jy/picker-picked-light-comparison.png) | [图](P13-jy/picker-picked-dark-comparison.png) |
| 8 提交出口 | [图](P13-jy/submit-keyboard-light-comparison.png) | [图](P13-jy/submit-keyboard-dark-comparison.png) |

[深色总览](P13-jy/overview-dark.png) · [浅色总览](P13-jy/overview-light.png) · [像素差](P13-jy/image-differences.json)。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿用近黑/浅色token |
| A2 非系统字体 | 中文系统字体优先，不为网页通则换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置表单 |
| A5 三等分/万能卡片 | 无新增卡片 |
| A6 假数据 | 产品无假指标；工具为隔离磁盘夹具 |
| A7 空词 | i18n零改，取消去掉误报 |
| A8 眉批 | 无新增 |
| A9 标题 | 既有标题完整可见 |
| A10 Bento | 不适用 |
| A11 装饰竖线 | 无新增 |
| B1 读取/拨盘 | 3/2/7与功能面一致 |
| B2 主题 | 深浅Tester分别设置，无混用 |
| B3 记忆点 | 失败回来仍是自己的草稿 |
| B4 动效气质 | 状态即时，沿用加载环 |
| C1 目的/频率 | 忙态表达实际在飞操作，键盘不新增动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 新增过渡0ms；原加载循环沿用，不等同于界面转场 |
| C4 origin | 忙态在原入口，错误走原宿主浮层 |
| C5 reduced-motion | 新截图设置ReduceMotion=true，原降级沿用 |
| C6 hover查询 | Go自绘不适用CSS媒体查询，指针模型未改 |
| C7 布局动画 | 无新增 |
| C8 重播keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用Go自绘 |
| C11 慢放 | 无新动效，只验确定静态帧，不声称真机慢放 |
| D2-1 字体 | 系统中文字体，无webfont |
| D2-2 字号/行高/字重 | 通用≥14px/1.5未满足；原Caption9.9/正文10.8和行盒需拍板，未越权改 |
| D2-3 混排空格 | i18n零改，无新产品句子 |
| D2-4 标点/对比 | 原标点与墨不变，不声称所有次要墨≥7:1 |

缺陷沉淀为§6.252和事件测试，不升级为全局审美规则。用户指定Go自绘、单一方向、不扩张优先于skill的浏览器、多方案与工艺数量通则。原生系统权限/选择器窗口未截图；Windows/macOS仅编译，不声称真机交互通过。

## 仍待

1. 下一轮先复现同一连接表单内，选择器回调期间切换程序/脚本或改写其他字段的草稿归属。本轮覆盖换表单/关闭，不把同表单重入线索当已确认bug，不重复失败恢复和返焦点。
2. 生成工具编辑/转换覆盖、重复新包ID覆盖、重装事务、配置模板失败重试、另一端冲突、字号/行盒、accent禁用填充、未知多选/Radio、极小步长、176-9/161-4保存期草稿、终端S1b/S1c/S2尾继续需拍板。
3. 取消后的预生成目录保留，沿用PrepareCustom“拒绝后可再审核”的既有约定；未改重试复用/清理语义。健康成功只解除probe_failed及旧ImageCopy/Rescan线索沿用。

## 最终验收（串行复跑）

2026-11-08 04:59:52 UTC 完成。最终 **6069 PASS / 0 FAIL / 3425函数**，相对开盘6029 PASS、3413函数为 **+40 PASS、+12函数**。普通与verbose全量均exit0；定向race **40 PASS / 0 FAIL / exit0**；三平台vet/build均exit0，gofmt为空，最终三份产品源码MD5复核全部OK。旧V/B/S断言修改0/0/0、旧测试函数删除0、新增Skip0。

失败的并行验收后，完全相同的源码和旧断言通过串行完整重跑，未增加超时、容差或Skip。旧memo pending断言在串行完整测试通过；并行出现的一次失败仍保留观察，不把它包装成已修复的第九项。原始失败日志在[parallel-attempt](P13-jy/parallel-attempt/)。

四批每两项均已推送：**0ff958a2 / 08f2e09f / 961a75f4 / f132bc4c**。本次自首个时钟04:04:15到最终验收约56分钟（包括失败验收与串行重跑）。工具调用超预算的过程偏差保留于上文。

证据：[基线](P13-jy/baseline-zh.txt)、[普通全量](P13-jy/full-test.txt)、[verbose全量](P13-jy/full-verbose.txt)、[race](P13-jy/focused-race.txt)、[源码MD5](P13-jy/final-source.md5)、[门槛退出](P13-jy/gates.exit)。对应txt/exit、44原图、16对照和4组mutation证据均进仓库。本机的完整Go自绘测试宜串行，避免本轮同类资源竞争。
