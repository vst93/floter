# P13-ko · 插件配置刷新后的可见编辑出口

16:35 UTC开盘，HEAD09a47ae4，pull已同步、工作区干净。P13-jx已完成任务书旧线索，按P13-kn仍待继续。Codex独立执行，已读设计skill与指定参考；done.log开盘不存在。规格先落盘。

## stash清点

权威清单八条，无新stale-kill。逐条实跑reverse-check均exit1，SHA与交接一致。原件保留：早期WIP与当前新主题/实现有语义差异，不能把reverse失败当作可直接合入的独有修复。既有抢救工作已由历史归档接续，不静默丢弃。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap与主题重写 | 保留原件 |
| stash@{1} | 8af94a07 | 旧行高/滑块pin | 保留原件 |
| stash@{2} | 7e4aa710 | 旧卸载布局 | 保留原件 |
| stash@{3} | 1e3aec94 | 旧数字字段与主题 | 保留原件 |
| stash@{4} | 363f5a42 | 旧禁用栏/设置WIP | 保留原件 |
| stash@{5} | 9b8ce23c | 旧快捷键/设置接线 | 保留原件 |
| stash@{6} | 90d314b2 | 旧启动器动作栏 | 保留原件 |
| stash@{7} | 7a9f6e13 | 旧Tauri发布链 | 保留原件 |

## 实施与验证（增量）

正在验证刷新布局后焦点仍在但控件离开视口的事件路径；只有实际变红的项计入任务量。开盘完整基线正在跑。

1. Radio：现象→失败行插入后轨道Y239.2/H30.6，组标题刷新后Y272.6，恢复提示后Y-9.8；焦点在但环不可见。改动→宿主布局输入改变时一次显现原16px净空矩形。验证→深浅三个阶段分别回到Y213.4/213.4/16，方向键仍只写当前字段，值刷新不抢滚轮。
2. Checkboxes：现象→失败行与分组把当前选项推到Y252.8/286.2，提示退役又推到顶部外。改动→每个原选项仅在自身持焦点时接收布局刷新显现。验证→深浅完整行盒断言、Space仍只改北方；没有扩大命中面积。

M1撤销刷新重显现，两项深浅真实失败；`radio failure refresh hides active ink: {X:28 Y:239.20001 W:504 H:30.6}`。恢复原文件MD5=OK，touch后定向与旧身份/活动边回归均通过；详细实数随批次归档。首次探索的Checkboxes文字查找返回裁切后的矩形，正式pin改用完整原行盒，未改任何旧B断言。

第一批45 PASS、包vet净，e7af1896已推送。首次提交门控时验收尚未结束，exit文件不存在，守卫拒绝提交；实际等退出0才提交，没有越过失败门槛。

3. Text：现象→失败/分组刷新后原输入槽底边超出视口，恢复又落到顶边外，键盘仍能写入不可见草稿。改动→文本外槽接入宿主布局刷新显现。验证→深浅失败/分组/恢复严格边界；continued草稿跨刷新逐字保留，Enter只提交当前字段一次。
4. Number：现象→失败后数字槽Y245.6/H25.2，刷新分组后Y279，恢复时Y-9.8。改动→数字原外槽同样一次显现。验证→半输入1e不被刷新丢弃、不派发非法值，继续输入24只写当前字段一次；值刷新保持用户滚动位置。

第二批旧产品两项深浅均实红，修复后事件与旧身份/编辑边回归通过；M2正在独立注入与逐字节还原。

第二批30 PASS、包vet净；M2独立撤回两条外槽接线，四个深浅场景实红，MD5恢复OK、touch后12 PASS。891700f7已推送。

5. Toggle：现象→错误行和分组刷新后开关仍持焦点，却被推入下淡边；恢复时上环被切。改动→开关原本体接入一次刷新显现。验证→深浅三个阶段完整活动墨，Space仍只切当前开关一次。
6. Select：现象→宿主刷新后下拉触发器离开当前视口，键盘仍会打开不可见的入口。改动→原触发器接回刷新显现。验证→深浅几何pin，Down/End/Enter仍选择南方，普通滚动和值刷新不抢位置。

第三批36 PASS、包vet净，b083e5ea已提交。开盘完整基线已exit0；计数与函数数将由原始日志及开盘SHA核准。

7. Slider：现象→失败/分组刷新把当前轨道推到下边外，读数与轨道失去可见操作位置。改动→原滑轨接入布局变化后的单次显现。验证→深浅三阶段活动墨边界、Right从12写到13且只派发一次，后续滚动自由。
8. Action：现象→失败提示、分组及恢复后，持焦点的清理胶囊落到视口外，下一次Enter仍会装填。改动→原动作按钮接入一次刷新显现，确认/取消复用原按钮。验证→深浅活动边、Enter仅进入armed且不运行命令，恢复不产生写入。

第四批63 PASS、包vet净，c7554491已推送。四批分别e7af1896/891700f7/b083e5ea/c7554491；开盘实测6570 PASS / 0 FAIL / exit0，3553函数。八项跨八类现有控件，旧V/B/S断言0/0/0，新Skip0。

窄面补验3 PASS / exit0：320×180长选项的同一底层slice原地首尾重排，焦点身份保留，完整17px行盒显现，Space与Click共4次回调/主题均只修改csv，MaxY不变。该项是八项的边界补验，不另凑第九项。

## 偷了什么

本轮已实际读取qiaomu-design、完整偏好账本及craft-loop、preflight、chinese-typography；供体Linear DESIGN取三级中性面、0.05–0.08发丝边、单一交互accent的角色，沿本仓token落地，未复制紫色或字体。设计读取为紧凑原生配置浮层，拨盘3/2/7，当前用户的明确方向优先于四方向预览和外部模型规则。

- craft-loop菜单#3 品牌focus环：保留原3px外扩环，三个刷新阶段均完整落在11px淡边内侧。
- #4 签名交互：宿主布局改变才触发一次显现，默认16px净空、0ms；值刷新触发0次，普通滚轮保持用户位置。
- #12 材质细节：新增玻璃面、白边与阴影均0层；原受光边与8px字段圆角保持，动作胶囊仍为半高圆角。

**为何这轮更轻、更有空间感**：当前编辑出口不会随错误与分组刷新掉进浮层软边，原有层次继续承载有效的操作位置；不增加常驻提示或装饰面。依用户修复优先规则，没有为了工艺菜单五项额度添加无关装饰。

## 两轮图像复核

首轮四视角：艺术总监看到活动蓝环被底淡边切断；工程师看到焦点身份保留却位置出界；任务用户能继续键入但看不到草稿；读者在恢复后找不到顶部外的当前输入。第二轮集中核对完整活动边、草稿保留、上下双向恢复、窄面重排，深浅都复渲染并目视。

浅色单选before只有蓝环上沿，after完整显示“北方”与下沿；深色数字before的1e压在底淡边，after回到完整槽内；开关、下拉、滑轨与动作都在原面上露出完整边界。128张原图、48张状态对照、32张4×局部及2张总览已生成；16组静息差异0像素，刷新变化最小47798像素。4×是最近邻检查放大，不冒充原生高DPI。

图来自Linux ui.Tester 560×260与窄面320×180，测试数据是隔离schema；Windows/macOS只做编译与vet，不声称原生真机交互。没有新增动效轨迹或模糊材质。

| 项 | 浅色失败刷新 | 深色失败刷新 |
| --- | --- | --- |
| Radio | [对照](P13-ko/radio-failure-light-comparison.png) | [对照](P13-ko/radio-failure-dark-comparison.png) |
| Checkboxes | [对照](P13-ko/checkboxes-failure-light-comparison.png) | [对照](P13-ko/checkboxes-failure-dark-comparison.png) |
| Text | [对照](P13-ko/text-failure-light-comparison.png) | [对照](P13-ko/text-failure-dark-comparison.png) |
| Number | [对照](P13-ko/number-failure-light-comparison.png) | [对照](P13-ko/number-failure-dark-comparison.png) |
| Toggle | [对照](P13-ko/toggle-failure-light-comparison.png) | [对照](P13-ko/toggle-failure-dark-comparison.png) |
| Select | [对照](P13-ko/select-failure-light-comparison.png) | [对照](P13-ko/select-failure-dark-comparison.png) |
| Slider | [对照](P13-ko/slider-failure-light-comparison.png) | [对照](P13-ko/slider-failure-dark-comparison.png) |
| Action | [对照](P13-ko/action-failure-light-comparison.png) | [对照](P13-ko/action-failure-dark-comparison.png) |

[浅色总览](P13-ko/overview-light.png) · [深色总览](P13-ko/overview-dark.png) · [像素差](P13-ko/image-differences.json)。同目录heading/recovery对照覆盖分组刷新与提示撤回。

## Preflight A / B / C / D2

| 检查 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿用近黑与浅色主题 |
| A2 非系统字体通则 | 中文系统字体优先，未换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用原生配置浮层 |
| A5 三等分/万能卡片 | 无新增卡片 |
| A6 假数据 | 产品无假指标；截图使用明确隔离schema |
| A7 文案空词 | 产品i18n零改 |
| A8 眉批 | 无新增 |
| A9 H1换行 | 原标题未改，窄面标题完整 |
| A10 Bento | 不适用 |
| A11 左侧装饰线 | 无新增 |
| B1 设计读取 | 3/2/7与原紧凑功能面一致 |
| B2 主题 | 每个Tester独立明确深/浅 |
| B3 记忆点 | 刷新后仍能看见自己正在操作的控件 |
| B4 气质 | 一次即时显现，原色彩/按压保留 |
| C1 目的/频率 | 只在焦点进入或宿主布局变化时显现 |
| C2 all/ease-in/scale(0)/纯fade | 无新增 |
| C3 时长/active | 新显现0ms，原颜色120ms与按压保持 |
| C4 origin | 原控件是显现目标，未增加新浮层 |
| C5 reduced-motion | 测试启用ReduceMotion，新增显现本身即时 |
| C6 hover查询 | Go自绘无CSS媒体查询，原指针语义保留 |
| C7 布局动画 | 未动画宽高/间距 |
| C8 keyframes | 无新增 |
| C9 手势 | 本轮不涉及拖拽/swipe |
| C10 Framer | 不适用Go自绘 |
| C11 慢放 | 无新运动轨迹，检查确定帧，不冒充真机慢放 |
| D2-1 字体 | 保留系统中文字体，无webfont |
| D2-2 字号/行高/字重 | Caption9.9/正文10.8仍不满足通用14px/1.5；沿用需拍板，不越权改 |
| D2-3 混排空格 | 产品文案零改，测试中文/英文混排有空格 |
| D2-4 标点/对比 | 原标点与墨色不变，不声称所有次要墨达到7:1 |

自发现缺陷写入项目§6.268与事件pin，不发布为skill全局审美规则。原生自绘、既定方向与功能修复优先于网页四预览、1280px和五项新增装饰要求。

## 仍待

1. 下一轮先复现插件配置浮层的当前字段被宿主删除或改型后的键盘出口；本轮只处理身份仍存在时的布局刷新，不把线索直接算bug。也核对禁用/撤回选项时原出口是否可见，至少八项跨组件。
2. 字号/行盒、accent禁用填充、未知多选、保存期间草稿、输出首尾键/选区、终端S1b/S1c/S2尾仍需拍板；3秒窗口、保存和会话语义不改。
3. 八条早期WIP原件保留；Windows/macOS原生交互未验。

## 最终门槛（增量）

开盘6570 PASS / 0 FAIL / 3553函数；最终普通/verbose与三平台门槛正在运行，未提前填入推算数字。现有测试函数删除0，新增9函数（八类事件与窄面重排）；旧V/B/S断言修改0/0/0，新Skip0。

M2原始红串：`number failure refresh hides active ink: {X:478 Y:245.60002 W:46.8 H:25.199999}`。M1与M2均exit1，原件MD5恢复OK、touch后分别45/12 PASS。四批45/30/36/63 PASS、包vet均净；窄面补验3 PASS。

4×局部与窄面二次目视：数字的1e和完整下沿清楚保留；单选蓝环下边没有新的描边叠层；csv原地移动到末行与回首行后仍有完整焦点框且对应原值。132张原始PNG含4张窄面，48张状态对照、32张检查放大、2张总览；静息16组零差。

第一次nohup验收启动只创建空日志即无存活进程，没有退出记录，不计作通过；已改为前台受管会话重跑完整命令，没有改变源码、测试断言或超时。

受管脚本的三平台vet/build及gofmt均exit0，普通全量尚未完成时外层返回143，没有full-test.exit；原输出留存first-full-test.txt，不算完整通过。改用与开盘基线相同的独立命令逐个执行，产品源码与测试断言不变；仅复跑未完成的全量测试。

16:59 UTC，独立普通全量已完整exit0；最终三份源码MD5均OK。verbose全量正在独立计数。开盘settingsui包485.091s，最后一次普通全量也实际等待完整退出，没有把部分包的输出当作全量通过。

17:07 UTC最终verbose完整exit0，**6597 PASS / 0 FAIL / 3562函数**；相对开盘6570/3553净增27 PASS、9函数。普通全量exit0，gofmt空，Linux/Windows/Darwin的vet/build全部exit0，最终源码MD5三份均OK。没有后续产品修改。

| 门槛 | 最终实测 |
| --- | --- |
| gofmt -l cmd internal | 空，exit0 |
| go vet ./... | linux/windows/darwin全部exit0，日志空 |
| go build ./... | linux/windows/darwin全部exit0，日志空 |
| go test -count=1 ./... | 完整exit0，settingsui 301.810s |
| go test -count=1 -v ./... | 完整exit0，6597 PASS / 0 FAIL |
| 测试函数 | 3553 → 3562，旧函数删除0 |
| 四批定向 | 45 / 30 / 36 / 63 PASS，包vet净 |
| 窄面原地重排 | 3 PASS，四次正确Space/Click回调/主题 |
| mutation | M1/M2实红，MD5还原、touch后45/12 PASS |
| V/B/S与Skip | 旧断言0/0/0，新Skip0 |
| 图像 | 132原始PNG、48对照、32局部、2总览；16组静息差异0像素 |

执行用时从16:35到17:07约32分钟，低于45–75分钟目标；八项跨组件任务量已满足，没有加无关改动凑时长。受管脚本143与首次空日志均如实保留，独立完整测试通过才结案。工具调用通过批处理安排，完成后只归档证据、文档检查与推送。

原始证据：[开盘基线](P13-ko/baseline.txt)、[普通全量](P13-ko/full-test.txt)、[verbose全量](P13-ko/full-verbose.txt)、[M1红](P13-ko/M1-red.txt)、[M2红](P13-ko/M2-red.txt)、[最终源码MD5](P13-ko/final-source.md5)、[stash清单](P13-ko/stash-list.txt)、[像素差](P13-ko/image-differences.json)。每条门槛与变异的txt/exit、逐字节恢复记录均随报告归档。

收尾文档契约检查internal/ci再次exit0；最终图同时保留`/tmp/floter-P13-ko-<radio|checkboxes|text|number|toggle|select|slider|action>-<dark|light>.png`入口，完整before/after在本轮档案。八条stash最终SHA未变、reverse均1；没有新增或丢弃原件。
