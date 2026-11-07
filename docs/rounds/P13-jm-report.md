# P13-jm · 当前工具声明与状态归属

起点 022e27d7；ff-only pull 已最新，工作树干净，19:52 UTC 开盘。Codex 独立执行，本文增量记录实测。

## Stash 清点

没有活动 stale-kill。八条均与 P13-jl 及交接登记 SHA 相同，reverse-check 均退出1；沿用已登记语义结论，不将早期主题和 Tauri 链覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载/scratch，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 增量实施

已读取最新交接、用户反馈、视觉合约与 qiaomu-design 打磨资料。当前八项为代码线索，正在建立写盘刷新与实际界面的证据。

1. 现象：同路径 runtime.type 从不可解析的脚本运行时切换到可用系统程序，旧徽标仍说不可用。改动：事实身份纳入当前运行时类型。验证：真实 manifest 写盘、Store.Refresh、列表投影及深浅 ui.Tester；修前 `runtime-type: refreshed declaration kept obsolete runtime badge: got=false want=true`，修后可用。
2. 现象：脚本解释器改为本机可用语言，旧不可用状态仍留在卡片。改动：身份同时纳入语言。验证：真实声明刷新、ResolveRuntime 与卡片投影；修前 `runtime-language: refreshed declaration kept obsolete runtime badge: got=false want=true`，修后可用。

第一批相关 **8 PASS / 0 FAIL**，shell vet 退出0。旧V/B/S 0/0/0，无新增Skip。M1正式回注安排在八项完成后；修前两项深浅已实红。基线仍运行，暂不写数字。夹具最初一次编译因未用 import 和时钟字段名错误失败，已纠正；不将编译失败计入mutation。

第一批 **7e176690** 已推送。

3. 现象：系统程序候选名已经改成存在的程序，卡片仍显示不可用。改动：把有序候选名纳入当前事实身份。验证：真实 PATH 解析、manifest 写盘与 Store.Refresh，深浅修前 `runtime-names: refreshed declaration kept obsolete runtime badge: got=false want=true`；修后徽标撤下。
4. 现象：平台声明已不支持当前主机，旧可用状态仍保留。改动：事实身份同时记录 manifest 是否有效。验证：真实平台校验返回错误，深浅修前 `runtime-platform: refreshed declaration kept obsolete runtime badge: got=true want=false`；修后不可用徽标出现。原计划脚本越界会同时清空解析结果，证据独立性不够，改为保留运行时声明但平台校验失败的真实事件。

第二批原验收实际 **11 PASS / 3 FAIL**，候选程序夹具在组合运行中未改变可用性；被错误记录成14 PASS并由 **45aa492a** 提前提交推送。这是执行偏差，不能算通过，原日志保留。原因是测试临时改PATH不适合进程已初始化的路径探测。第一次改为绝对路径仍失败（候选名解析不支持绝对路径），最终改用本机原PATH中的平台原生解释器候选，补验后再提交。四项额外断言实际绘制标签，旧V/B/S 0/0/0，不放宽断言。

基线完整 verbose 实测 **5678 PASS / 0 FAIL**，退出0，起点函数数 **3311**。

5. 现象：健康记录已经失败，新鲜度详情仍显示成功，与下方健康状态矛盾。改动：当前事实身份纳入报告状态。验证：真实报告落盘/Store.Refresh，深浅修前 `health-result: refreshed report still paints old result: got=success want=failed`，修后必须同时看到失败文本。
6. 现象：健康检查时间已刷新到现在，上次探测仍显示前天。改动：当前事实身份纳入 CheckedAt。验证：真实时间字段更新、当前记录与可见“刚刚”联合断言；修前 `health-time: refreshed check still paints old timestamp`。

第三批及第二批补验：**20 PASS / 0 FAIL**、shell vet退出0。候选程序使用本机已有解释器名称，四种声明与两种健康刷新全部通过。正式M1将在最终代码上串行注入；夹具失败日志保留。

第三批 **aa13eb26** 已推送，修正后的候选夹具和健康结果同批提交。

7. 现象：同 ID 换到新包，单行新鲜度继续显示旧包的2条命令。改动：命中缓存前核对当前清单中的声明，路径变化立即重读本行。验证：两个真实包目录和sidecar、Store.Refresh、单行回调与可见命令数；修前 `row-path: new package still paints old command count`，修后9条。
8. 现象：独立行的健康报告更新后仍永久显示旧成功，等待本行重绘也不刷新。改动：同一检查覆盖当前报告；命中仅比较内存声明，不额外stat/read。验证：真实健康记录与单行界面，修前 `row-health: refreshed report still paints old result: got=success want=failed`，修后失败。

第四批相关 **27 PASS / 0 FAIL**、shell vet退出0，旧V/B/S 0/0/0，无新增Skip。保留列表与单行两种入口，额外验证连续60帧复用、partial补全和单行memo。八项已完成红绿复现，随后进行正式mutation和完整门槛。

第四批 **84405379** 已推送。

## Mutation

- M1：从身份中移除类型、语言、候选名、manifest有效状态及健康报告字段。前六项深浅共 **18 FAIL** 测试节点，包含 `runtime-platform: refreshed declaration kept obsolete runtime badge: got=true want=false`、`health-result: refreshed report still paints old result: got=success want=failed`。还原后三文件MD5全OK，touch复验 **18 PASS**。
- M2：单行命中恢复为只看ID与非空key。后两项深浅共 **6 FAIL** 节点，包含 `row-path: new package still paints old command count`、`row-health: refreshed report still paints old result: got=success want=failed`。还原后三文件MD5全OK，touch后八项一起 **24 PASS**。
- 原始日志、MD5核对与修前/修后结果位于 `P13-jm/`。最初截图画布720×1000未覆盖已安装卡，目检后改为960×1600并重跑正式变异，最终原图均完整覆盖该卡。测试函数数 **3319**，相对基线 **+8**。

## 偷了什么

使用 qiaomu-design 的既定方向打磨路径，实读 SKILL、全部用户偏好、craft-loop、preflight、chinese-typography及Apple DESIGN，Codex独立实现。

- craft-loop **#4 签名交互**：已确认的声明到本帧显示，额外等待 **0ms**；外部磁盘变化原 **2s TTL** 不动。
- **#6 编辑细节**：工具已可用时错误徽标 **1→0**；真实失败从旧“成功”回到原强墨“失败”，附加说明卡 **0个**。
- **#12 材质细节**：原 **1px** 受光边与 **8px** 控件圆角沿用，额外玻璃 **0层**。
- **#5 多层阴影**：浮层原 **2层** 投影沿用，状态行新增外影 **0层**。这些是保留的具体值，不冒称本轮改了材质token。
- Apple DESIGN取交互专用accent、8px圆角、2px focus和浅色0.22外影的角色，映射现有token；没有抄品牌色或增加装饰来凑五项。

**这轮为什么更轻、更有空间感**：撤去过期的不可用徽标，让状态层只承载当前事实；错误回到已有的结果墨阶，界面不需要再加一层解释卡来调和矛盾。

## Preflight逐条

| 检查 | 结论 |
| --- | --- |
| A1 紫蓝渐变/纯黑/外发光/渐变字 | 未新增，沿用既有中性面 |
| A2 非系统字体且与上次不同 | 不适用：中文系统字体和本仓合约优先，不换字体 |
| A3 斜体 | 无新增斜体 |
| A4 Hero构图 | 非Hero表面，不适用 |
| A5 三等分卡片/万能卡片 | 沿用集成卡；新增容器0个 |
| A6 假人名/假精确数字 | 无；命令数2→9来自实际sidecar |
| A7 空洞文案/感叹号 | 未新增文案 |
| A8 编号眉批 | 未新增 |
| A9 H1换行 | 沿用单行“集成” |
| A10 Bento空洞 | 无Bento，不适用 |
| A11 左侧装饰竖线 | 未新增 |
| B1 读取与拨盘 | 2/1/6，维持紧凑功能表面与即时状态 |
| B2 单主题 | 深浅分别运行并目检，不混主题 |
| B3 记忆点 | 工具当前状态与刚完成的检查一致 |
| B4 动效气质 | 无新动画，反馈即时 |
| C1 目的/频率 | 高频刷新不加动画 |
| C2 all/ease-in/scale(0)/纯fade | 无新增 |
| C3 时长/按压 | 新延迟0ms，原按钮按压沿用 |
| C4 浮层origin | 无新增浮层，不适用 |
| C5 reduced-motion | 测试显式ReduceMotion，原机制保留 |
| C6 hover媒体条件 | Go自绘，非CSS；未改hover |
| C7 动画属性 | 未动画化布局属性 |
| C8 高频重播 | 无新增keyframes |
| C9 手势/弹簧 | 未修改，不适用 |
| C10 Framer/Motion主线程 | 无该依赖，不适用 |
| C11 慢放/逐帧 | 无新动画；用真实前后帧检查状态变化 |
| D2-1 中文系统字体 | 通过，locale统一zh_CN.UTF-8 |
| D2-2 行高≥1.5/正文≥14/字重400/500/600 | 不能宣称通过；既有Caption9.9/正文10.8与580/620等合约保留，沿用字号/行盒需拍板项 |
| D2-3 混排空格/单位 | 未新增文案；原文与时间格式保留 |
| D2-4 全角标点/暗色7:1 | 未改文案、墨值；默认全部次级墨并不承诺7:1，沿用现有高对比合约，不冒称新测 |

不适用项与既有合约例外均明示；用户已要求既定方向、修现有功能优先，不走四方向点选或skill外部模型。

## 仍待

1. 下一轮首选同ID工具刷新期间，诊断/健康检查的晚到结果是否仍归属当前声明（shell.go的diagnoses[id]写回链）。只是代码线索，先真实事件复现；不重复本轮已完成的事实身份与单行命中。
2. 重复新包覆盖、模板失败补跑、多草稿/转换覆盖、另一端冲突、字号/行盒、accent禁用填充、未知多选/Radio、极小步长、176-9/161-4保存期间草稿、终端S1b/S1c/S2尾继续需拍板；3000ms/5000ms/180ms与会话语义不改。
3. Linux真实写盘、Store.Refresh、运行时解析与ui.Tester已验；声明事件由测试更新本地文件，未冒称用户人工重装或原生选择器验证。Windows/macOS仅vet/build；八条早期WIP保留。
4. 第二批提前提交失败夹具属于本轮执行偏差，已在aa13eb26更正并补验，历史记录保留。

## 图像与两轮复核

八项×深浅×前后共32张960×1600原图，16张1472×404对照板，全部在`P13-jm/`。原图路径`/tmp/floter-P13-jm-<场景>-<light|dark>-<before|after>.png`；对照只裁出已安装卡，完整原图保留。截图由ui.Tester真实渲染，不是HTML或图像生成。

第一轮四视角：艺术总监指出可用工具的暖色旧徽标抢主任务注意力；工程师指出720×1000画布未覆盖目标卡；任务用户看到健康异常却仍被告知探测成功；读者指出单行夹具健康标签未跟随记录投影，造成无关矛盾。第二轮分别核对当前身份、扩展测试画布、当前结果/时间、单行其他健康字段用当前报告投影。第二次M2在最终夹具上再实跑 **6 FAIL**，最终三文件MD5全OK，touch复验 **6 PASS**；没有改产品规则以迎合截图。

| 项 | 浅色 | 深色 | 亲眼看到的一处变化 |
| --- | --- | --- | --- |
| 1 类型 | [图](P13-jm/runtime-type-light-comparison.png) | [图](P13-jm/runtime-type-dark-comparison.png) | 右上角“工具不可用”撤下 |
| 2 语言 | [图](P13-jm/runtime-language-light-comparison.png) | [图](P13-jm/runtime-language-dark-comparison.png) | 解释器可用后暖色旧徽标消失 |
| 3 候选名 | [图](P13-jm/runtime-names-light-comparison.png) | [图](P13-jm/runtime-names-dark-comparison.png) | 新候选可解析后不可用徽标消失 |
| 4 平台 | [图](P13-jm/runtime-platform-light-comparison.png) | [图](P13-jm/runtime-platform-dark-comparison.png) | 不支持本机后出现不可用徽标，与下方诊断对应 |
| 5 结果 | [图](P13-jm/health-result-light-comparison.png) | [图](P13-jm/health-result-dark-comparison.png) | 探测“成功”改为强墨“失败” |
| 6 时间 | [图](P13-jm/health-time-light-comparison.png) | [图](P13-jm/health-time-dark-comparison.png) | “前天”改为“刚刚” |
| 7 换包 | [图](P13-jm/row-path-light-comparison.png) | [图](P13-jm/row-path-dark-comparison.png) | 命令数2→9、无变化→+7，状态点同步变化 |
| 8 单行健康 | [图](P13-jm/row-health-light-comparison.png) | [图](P13-jm/row-health-dark-comparison.png) | 失败结果与当前健康异常同时可见 |

前后生成时真实墙钟相差数分钟，原检查时间随fixture变化；核心状态变化由精确文本/数据断言钉住，不把墙钟差算产品修改。全部16张对照已目检。

## 完整门槛

三平台vet/build六项均退出0，gofmt文件0字节，diff --check净。最终产品 **84405379**；最后修改仅截图夹具与文档。普通全量退出 **0**；verbose全量退出 **0**，实测 **5702 PASS / 0 FAIL**，基线 **5678**，增加 **24**。测试函数 **3311→3319（+8）**；旧V/B/S断言修改 **0/0/0**，新增Skip **0**。

第一次普通全量进程退出143（信号终止），没有测试失败行，也未启动后续verbose；原日志保存为full-test-interrupted.txt，不算通过。20:10 UTC重新分别启动普通与verbose两遍完整门槛，停止实现扩张，只补齐验收与收尾。工具预算优先用于这个未决检查，不再开新题。

两遍重跑均已完成。证据：[门槛](P13-jm/gates.txt)、[普通全量](P13-jm/full-test.txt)、[verbose全量](P13-jm/full-verbose.txt)、[基线](P13-jm/baseline.txt)、[M1实红](P13-jm/M1-red.txt)、[M1还原](P13-jm/M1-restored.txt)、[M2最终实红](P13-jm/M2-final-red.txt)、[M2最终还原](P13-jm/M2-final-restored.txt)、[最终MD5](P13-jm/final-md5.txt)。没有新页面、系统或依赖，保留原保存/确认/会话语义。

开盘19:52 UTC，完整门槛约20:17 UTC完成，约25分钟，短于45–75分钟目标。八项、分批推送、两组变异、32张原图和全量门槛均完成；没有以无关修改或重复成功检查填时间。实现预算到40次后只等待信号中断重跑、归档及推送；等待与收尾调用超过40次，作为执行偏差明确记录，不冒称严格满足预算。
