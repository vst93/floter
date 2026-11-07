# P13-jn · 诊断结果的完整呈现与当前归属

开盘8a4a7dc1，ff-only pull已最新，工作树干净。20:20 UTC开始；Codex独立执行。基线正在跑，未写未经完成的计数。

## Stash清点

八条早期独立WIP与P13-jm SHA一致，无活动stale-kill。逐个导出并reverse-check退出1，沿用已有语义审计，保留旧主题/Tauri原稿，不重复覆盖新实现。第一遍显示值因shell命令替换覆盖退出码误打印0，第二遍单独保存退出码核对，原错误日志均保留。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载/scratch，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

## 增量实施

正在用真实提供者进程、声明写盘、Store.Refresh和ui.Tester建立事件证据。

1. 现象：提供者返回 maintenance，总状态在诊断标题中缺失。改动：列表接出已实现的 DiagnosisStatus。验证：真实提供者子进程与深浅ui.Tester，修前 `provider verdict never reaches the diagnosis heading`；修后精确显示 Maintenance。
2. 现象：提供者返回网络正常和账号警告，玩家只见摘要，成功检查完全消失。改动：列表接出有序 DiagnosisChecks。验证：两个原始ID及顺序、两个可见消息联合断言；修前 `provider checks never reach the diagnostic grid`，修后全绿。

第一批相关7 PASS / 0 FAIL，shell vet退出0，旧V/B/S 0/0/0。原测试首稿误要求界面显示原始ID与小写状态，读取真实网格合约后修为检查ID数据顺序和既有首字母大写/消息显示；没有修改旧断言。正式mutation留到最终产品统一执行。

第一批bb8d9631已推送。

3. 现象：诊断结束后健康报告刷新为异常，摘要仍被旧诊断盖回健康。改动：健康块保留当前持久报告的投影。验证：真实仓库写盘、Store.Refresh和深浅界面，修前 `diagnosis overwrote current health: "健康" want "异常"`；修后异常文本与状态枚举一致。
4. 现象：诊断后由中文切英文，健康标签仍为“健康”。改动：同一投影只用当前语言，诊断不再拥有健康译文。验证：Store.Update触发实际语言切换，修前 `diagnosis health kept previous language: "健康" want "Healthy"`；修后Healthy可见。

第二批相关14 PASS / 0 FAIL，shell vet退出0，旧V/B/S 0/0/0。首轮目检浅色完整图：两个检查项已进入原有54px状态列与消息列，成功项不再消失；没有额外卡片或阴影。

第二批89107bb7已推送。全量基线日志完成：5702 PASS / 0 FAIL，起点3319测试函数。

5. 现象：同ID的工具换包后，新包卡片继续显示旧包诊断。改动：结果记录包路径、程序、版本与声明身份；显示前不匹配则退役。验证：真实包路径变更与Store.Refresh，修前 `owner-package: current tool inherited another declaration's diagnosis`；修后旧检查撤下，新包再检查能显示自己的结果。
6. 现象：检查已启动，提供者声明在等待期间变化，旧进程晚到仍污染当前诊断。改动：写回前核对当前声明，失效结果不发布。验证：真实子进程启动标记/释放门、声明写盘刷新、完整worker完成等待；修前 `owner-pending: current tool inherited another declaration's diagnosis`，修后旧结果被拒且新检查可用。

第三批相关20 PASS / 0 FAIL，shell vet退出0。把原异步闭包提为同一worker以便测试等待完整完成，公共动作仍异步；没有改诊断协议、超时或持久健康写回。命令重探测的失败也携带来源，兼容原有共用诊断出口。旧V/B/S 0/0/0。

第三批b8e6ac7f已推送。

7. 现象：连续检查时第二次先成功，第一次晚到的失败覆盖新诊断。改动：入口记录本工具请求序号，写回只接受当前序号。验证：两个真实子进程以文件门精确控制先后，修前 `older check overwrote the latest completed diagnosis`；修后只见最新成功，旧失败不复活。
8. 现象：同一顺序下界面先收到工作正常，随后又显示旧检查“报告了问题”。改动：失效结果在通知之前退出，不派发旧完成反馈。验证：深浅当前toast与队列共同断言，修前 `superseded check replaced current completion feedback: ["检查工具 工作正常" "检查工具 报告了问题"]`；修后只保留一条当前成功。

第四批相关26 PASS / 0 FAIL，shell vet退出0。首稿没有提供panelOnScreen seam导致通知进入系统出口，读实际路由后补用既有seam并重新得到上述真实两条toast失败；未修改产品通知路由。八项均已独立红绿复现，旧V/B/S 0/0/0、新增Skip0。随后统一进行正式变异与完整门槛。

第四批8284165d已推送。

## 正式Mutation

最终产品上串行执行四组；每组均6个测试节点实红、还原源文件与测试文件MD5全OK，touch后6 PASS / 0 FAIL。

| 组 | 注入 | 原始报错摘录 |
| --- | --- | --- |
| M1 | 撤回状态和检查投影 | `provider verdict never reaches the diagnosis heading`；`provider checks never reach the diagnostic grid` |
| M2 | 恢复旧Health覆盖 | `diagnosis overwrote current health: "健康" want "异常"`；`diagnosis health kept previous language: "健康" want "Healthy"` |
| M3 | 撤掉显示和写回的声明保护 | `owner-package: current tool inherited another declaration's diagnosis`；`owner-pending: current tool inherited another declaration's diagnosis` |
| M4 | 撤掉序号保护 | `older check overwrote the latest completed diagnosis`；`superseded check replaced current completion feedback: ["检查工具 工作正常" "检查工具 报告了问题"]` |

完整门槛与并发定向race验收随后运行；八项原始深浅前后图已按最终夹具重新生成。

## 偷了什么

实读qiaomu-design的SKILL、偏好账本、craft-loop、preflight、chinese-typography与Apple DESIGN。读取为既有功能表面的轻便液态玻璃打磨，视觉变化幅度2、动效1、密度6；没有外部模型、四方向或HTML预览。

- craft-loop菜单 **#4 签名交互**：结果在当前帧呈现，额外等待 **0ms**；过期完成派发 **0次**，当前完成 **1次**。
- **#6 编辑细节**：接活已有 **54px** 状态列、**8px**列间距与 **1px** 行分隔；成功检查也保留在原顺序。旧问题块退役后额外占高 **0px**。
- **#12 材质细节**：沿用 **1px** 受光边与 **8px**控件圆角，新增玻璃 **0层**。
- **#5 多层阴影**：保留浮层环境/接触 **2层**，诊断行新增外影 **0层**；浅色环境影的 **0.22** 角色继续由本仓token承担。
- **#3 品牌focus环**：沿用交互accent与 **2px** focus；诊断接线不添加焦点节点，键盘出口额外动画 **0ms**。

Apple供体仅提取交互专用accent、8px圆角、2px focus及浅色0.22环境影的角色；本轮不改这些已有值，不冒称重新设计材质。工艺取舍服从用户“修现有问题”的优先级。

**为什么更轻、更有空间感**：旧工具的诊断区收回，完成浮层只承载当前结果；完整检查直接进入已有网格，既不添加一层卡片，也不让旧警告与当前内容争夺注意力。

## 两轮图像复核

第一轮实看960×1600浅色完整图。艺术总监：旧摘要把结论与原因混成一行；工程师：状态字段和逐项结果在宿主接线中丢失；任务使用者：健康更新和语言切换后标签互相矛盾；读者：原Caption偏小、次要墨偏弱，属既有拍板项。本轮前三处以完整投影/当前数据来源修复，第四处不越权改变字号。

第二轮在第一轮基础上集中收掉换包旧块、声明变化后的旧结果、倒序结果和过期通知。正式变异重出32张原图，并目检深浅四张对照总览：浅色“健康”→“异常”，深色旧工具诊断整块撤回，最新检查保持Ok，右上角只显示工作正常。没有截断当前检查；对照裁切为局部，完整原图仍保留。

| 项 | 浅色对照 | 深色对照 | 目检变化 |
| --- | --- | --- | --- |
| 1 总状态 | [图](P13-jn/status-light-comparison.png) | [图](P13-jn/status-dark-comparison.png) | 摘要上方有Maintenance结论 |
| 2 逐项检查 | [图](P13-jn/checks-light-comparison.png) | [图](P13-jn/checks-dark-comparison.png) | 连接正常与需要重新登录分行呈现 |
| 3 健康刷新 | [图](P13-jn/health-refresh-light-comparison.png) | [图](P13-jn/health-refresh-dark-comparison.png) | 异常状态不再带健康字样 |
| 4 语言 | [图](P13-jn/health-language-light-comparison.png) | [图](P13-jn/health-language-dark-comparison.png) | Health行内显示Healthy |
| 5 换包 | [图](P13-jn/owner-package-light-comparison.png) | [图](P13-jn/owner-package-dark-comparison.png) | 旧问题块撤回、操作行上移 |
| 6 检查中更新 | [图](P13-jn/owner-pending-light-comparison.png) | [图](P13-jn/owner-pending-dark-comparison.png) | 迟到旧工具提示不出现 |
| 7 连续检查 | [图](P13-jn/latest-result-light-comparison.png) | [图](P13-jn/latest-result-dark-comparison.png) | Problem变为当前Ok与连接正常 |
| 8 完成反馈 | [图](P13-jn/latest-notice-light-comparison.png) | [图](P13-jn/latest-notice-dark-comparison.png) | 右上角只显示工作正常 |

16个局部对照都有像素变化，最小 **190像素**（深色健康字样），最大70389像素（旧块收回）。原图为Linux ui.Tester软件自绘；没有宣称Windows/macOS实机交互。原路径 `/tmp/floter-P13-jn-<场景>-<dark|light>-<before|after>.png`；[像素记录](P13-jn/image-differences.txt)。

## Preflight A / B / C / D2逐条

| 项 | 结论 |
| --- | --- |
| A1 紫蓝渐变/纯黑/霓虹/渐变字 | 无新增；使用已有中性主题 |
| A2 换非系统字体 | 本仓中文系统字体合约优先，不换字体 |
| A3 斜体 | 无新增 |
| A4 Hero居中 | 非Hero表面，不适用 |
| A5 三等分/万能卡片 | 新容器0个，接回原网格 |
| A6 假数据 | 无产品假指标；测试提供者输出有明确断言 |
| A7 空词/感叹号 | i18n文案零改 |
| A8 眉批 | 无新增 |
| A9 H1换行 | 保留单行集成标题 |
| A10 Bento空洞 | 非Bento，不适用 |
| A11 左侧竖线 | 无新增 |
| B1 设计读取/拨盘 | 2/1/6，克制、即时、原功能密度 |
| B2 主题 | 深浅分别渲染，未混主题 |
| B3 记忆点 | 当前工具显示自己最新的完整检查 |
| B4 动效气质 | 新增动画0，符合高频检查 |
| C1 目的/频率 | 没有给高频刷新加动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 既有按压保留，新增延迟0ms |
| C4 origin | 保留宿主toast位置，未加浮层 |
| C5 reduced-motion | 两主题测试显式ReduceMotion=true，原机制不变 |
| C6 hover媒体条件 | Go自绘，非CSS；指针路由不改 |
| C7 布局属性动画 | 无新增；旧块退出是事实变化 |
| C8 keyframe重播 | 无新增 |
| C9 手势/弹簧 | 未涉及 |
| C10 Framer/Motion主线程 | 无此依赖，不适用 |
| C11 慢放 | 无新动效；用真实前后帧验状态，不冒称真机慢放 |
| D2-1 系统中文字体 | 沿用系统字体，统一zh_CN.UTF-8，无webfont |
| D2-2 行高/14px/字重 | 不宣称通过通用门槛；既有Caption9.9/正文10.8与580/620沿用，字号行盒需拍板 |
| D2-3 混排空格/单位 | 无新增产品文案，原时间格式不变 |
| D2-4 全角标点/暗色7:1 | 未改原文与墨值；不宣称次要墨全面满足7:1 |

自发现缺陷沉淀到§6.241与事件测试，未将一次性修复写成全局审美规则。用户固定方向优先于四方案和工艺数量等泛化要求。

## 仍待

1. 下一轮先复现“当前有效诊断失败”的反馈：本轮旧代码图里失败句仍带成功勾，notifyCompletion固定走panelFeedback(false)。在当前有效失败和命令重探测恢复路径上复现后再修，不把过期失败修复重复算一项。
2. 健康持久写回另有边界：extensions/probes.go的recordHealthReport以ID写回，检查期间声明变化是否可能写到新对象尚未实测。当前轮修的是内存诊断与显示；不声称已经修复持久健康报告归属。
3. 既有字号/行盒、accent禁用填充、未知选项、多草稿/转换覆盖、176-9/161-4保存期草稿、终端S1b/S1c/S2尾继续需拍板。无新增系统、依赖或设置项。
4. Linux事件与软件自绘有证据；其他平台仅编译门槛。八条早期WIP按表保留。

## 完整门槛执行记录

三平台vet/build均退出0，gofmt输出0字节；定向race **25 PASS / 0 FAIL**，退出0。首次普通与verbose全量并行运行均被信号终止，工具返回 **143**，未写到退出码文件，也没有测试失败行；不记为通过。中断日志保留，改为顺序重跑并持续轮询。源码与测试MD5仍全OK，未以修改断言规避中断。

顺序重跑完成：普通全量退出 **0**，verbose全量退出 **0**，实测 **5727 PASS / 0 FAIL**，基线 **5702（+25）**。测试函数 **3319→3328（+9）**：8个独立验收测试与1个跨平台子进程提供者入口；额外入口的空运行PASS不算第九项修复。旧V/B/S断言修改 **0/0/0**，旧函数减少0，新增Skip0。最终产品8284165d，最终两源文件MD5保持一致。

每两项定向事件与shell vet成功后立即commit/push，四批均已推送；完整三平台与全量门槛最终集中执行。没有提交红的定向批次。首次全量中断后的顺序重跑是为补齐未完成门槛，没有重复已成功的三平台检查。普通全量设置界面包耗时256.565s。

证据：[门槛](P13-jn/gates.txt)、[普通全量](P13-jn/full-test.txt)、[verbose全量](P13-jn/full-verbose.txt)、[基线](P13-jn/baseline.txt)、[race](P13-jn/race.txt)、[M1](P13-jn/M1-red.txt)、[M2](P13-jn/M2-red.txt)、[M3](P13-jn/M3-red.txt)、[M4](P13-jn/M4-red.txt)、[最终MD5](P13-jn/final-md5.txt)。两份中断日志一并保留，不作通过依据。

20:20 UTC开盘，20:48 UTC完整门槛完成，约28分钟，短于45–75分钟目标；八项、四批推送、四组mutation、32张原图与完整门槛均已完成，没有以无关修改或重复已通过检查填时间。工具预算主要用在实际实现、复现与中断重跑等待；收尾仅归档、文档检查、推送和交接，不再扩题。
