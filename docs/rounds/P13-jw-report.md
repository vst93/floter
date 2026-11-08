# P13-jw · 安装确认的当前声明与等待出口

开盘 2cca8ae9，pull 已同步，工作树干净。仓库已到 P13-jv，本轮按最新遗留推进。Codex 独立执行。开盘指定 done.log 不存在。基线与分批证据增量记录。

## Stash 清点

八条均为已审计早期独立 WIP，SHA 与上轮相同，无新增 stale-kill；逐条 reverse-check 退出1，不等于独有工作。沿用既有语义审核，不把旧主题/Tauri覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

## 实施记录

待实际事件复现后逐条记现象→改动→验证。旧V/B/S断言保持0/0/0；不改旧断言、不新增Skip。

### 第一批：两类安装入口的等待态

1. 现象：本地安装审核未回答时，原目录入口仍可点击；深浅事件红串 `pending local-busy permits repeat: calls=1`。改动：本地入口在审核、安装、刷新期间使用既有忙态胶囊，并在宿主拒绝重复派发。验证：真实Click等待时派发0次，取消结束后原入口可再点1次。
2. 现象：推荐工具连接审核期间同样可重复点击，红串 `pending recommended-busy permits repeat: calls=1`。改动：按推荐ID占用其自己的入口，复用同一忙态材质。验证：深浅Click和直接重复宿主派发，审核仅1次，取消后恢复。

旧代码的四个深浅场景均实红，原始 before PNG 和 M1-before.txt 已生成。后续正式回注、MD5复原、门槛数字据实追加。

第一批相关回归 **53 PASS / 0 FAIL**，settingsui/shell vet净。M1回注两个原入口，四场景实红；两文件MD5逐字节还原，touch后 **6 PASS**。浅色图目视：推荐行“接管”原位出现等待环并退淡，本地“选择…”保持可用；没有遮罩或新卡层。完整基线仍运行中，提交只引用已完成的定向数字。

第一批 **18326336** 已推送。开盘全量完成：**5974 PASS / 0 FAIL，exit0**。

### 第二批：声明字节变化使旧回答退役

3. 现象：本地审核窗口打开后新增clipboard-read，确认仍安装并批准新增权限。改动：Prepared把审核摘要带到安装读盘边界，不一致退役；本地入口安静恢复。验证：真实选择Click→审核门控→磁盘变化→确认，仓库字节不变、旧通知0条，新审核仍可安装。
4. 现象：推荐连接审核后暂存声明改变，同样把旧回答用给新权限。改动：共用摘要保护，推荐入口处理失效结果并恢复。验证：推荐行真实Click及其独立暂存目录变化，深浅两套同验。

第二批相关 **86 PASS / 0 FAIL**，extensions/shell vet净。M2撤销摘要检查后四场景报 `obsolete local-permissions review rewrote repository` / `obsolete recommended-permissions review rewrote repository`；MD5还原、touch后 **6 PASS**。开盘树 `git grep -h '^func Test' 2cca8ae9 -- '*_test.go'` 实数 **3395**（上一轮报告写3393，本轮不沿用误差）。

第二批 **41dadd06** 已推送。

### 第三批：包入口变化使旧回答退役

5. 现象：本地审核时保留原manifest字节，只将package.json改指current.json，旧回答仍登记原声明。改动：提交前重新解析本地包入口、名称与版本；重新审核后记录实际manifest相对路径，避免读取旧文件。验证：真实Click→入口改指→确认，仓库不变；新审核成功且Inventory读到当前工具名。
6. 现象：推荐暂存包改入口也绕过摘要检查，仍登记旧声明。改动：推荐路径共用当前入口复核。验证：独立暂存包、深浅两套事件，失效回答不写盘/不发旧通知，重新点击推荐可正常安装原装包。

第三批原代码四场景均报 `obsolete local-package review rewrote repository` / `obsolete recommended-package review rewrote repository`。新审核后manifest路径也一并钉住，不单算第九项。

第三批相关 **92 PASS / 0 FAIL**，extensions/shell vet净。M3撤销入口复核，四场景实红；MD5还原、touch后 **6 PASS**。两次审核后的当前manifest名称有独立断言。

第三批 **9b5f1b01** 已推送。

### 第四批：取消答案不进入提交

7. 现象：深链审核取消仍进入Commit；审核期间声明变化后产生“链接失败”反馈（没有摘要保护时还能误装已不声明权限的包）。改动：拒绝答案直接退出。验证：真实HandleURL→权限门控→磁盘移除权限→取消，仓库不变、通知0条。
8. 现象：registry审核取消也进入Commit，变化后的声明变成“安装失败”。改动：拒绝答案直接释放拥有的下载暂存目录再退出，使用既有刷新回调唤醒页面。验证：真实输入包名/Click安装→本地HTTP注册表→审核→取消，深浅仓库不变、通知0条、下载目录消失。

测试夹具首次编译引用不存在的ToastText，已改为已有notice队列在渲染前的断言；该编译错误不是产品证据，不计mutation。

## 2026-11-08 接续清点

03:11 UTC 从 `9b5f1b01` 接续，工作树干净、pull已最新。依据 AGENT-NOTES 沿用 P13-jw：前六项已推送，本次接回剩余两项并补齐整轮验收，不声称本次新写了前六项。开盘指定 done.log 不存在。

| 开盘 ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | d74161692d | reverse=1，独有第四批代码、测试与9行报告 | `git apply -3` 四文件无冲突接回；门槛通过后单独提交、归档原对象再drop |
| stash@{1} | 422483b4 | reverse=1，早期keycap WIP | 沿用已归档语义审计，保留 |
| stash@{2} | 8af94a07 | reverse=1，早期rows/slider WIP | 保留 |
| stash@{3} | 7e4aa710 | reverse=1，早期卸载布局 | 保留 |
| stash@{4} | 1e3aec94 | reverse=1，早期数字字段 | 保留 |
| stash@{5} | 363f5a42 | reverse=1，早期文本槽 | 保留 |
| stash@{6} | 9b8ce23c | reverse=1，早期快捷键 | 保留 |
| stash@{7} | 90d314b2 | reverse=1，早期动作栏 | 保留 |
| stash@{8} | 7a9f6e13 | reverse=1，旧main/Tauri链 | 保留，不覆盖当前Go实现 |

接续首次第四批四个深浅场景通过（6 PASS），随后补强本轮新增测试：通知在渲染消费前取样，取消后本地源文件逐字节保留，忙态变异失败也等待工作协程结束。八项定向 **24 PASS / 0 FAIL**。这些仅修改本轮新测试，旧V/B/S断言仍0/0/0；抢救稿先前的M4-restored日志中途结束，不能据它声称通过，本次重新实跑。

接续M4拆为两次独立注入：M4a只删深链取消出口，深浅实红 `cancelled link-cancel published a result: notices=["无法打开该链接"] pending=0`；M4b只删registry取消出口，深浅实红 `cancelled registry-cancel published a result: notices=["review-current 安装失败"] pending=0`。两次red均exit1，各自恢复MD5为OK、touch后3 PASS且exit0。最终守住的是用户取消不应产生错误通知；仓库不变与本地源/下载目录归属均有额外断言。

三平台vet/build均exit0、gofmt为空。首次整组race日志虽已输出24 PASS和包PASS（133.491s），外层工具返回143，未写出结束状态文件；原因未确认，不能把这次当成命令成功。保留 `current-race-first.txt/.exit`，取消两路径单独重验并以实际退出状态收录。

接续HEAD `9b5f1b01` 的完整基线实测 **5992 PASS / 0 FAIL、exit0、3401函数**；本轮开盘 `2cca8ae9` 仍为 **5974 PASS / 0 FAIL、3395函数**。取消两路径race单独重验 **6 PASS / 0 FAIL、exit0**。抢救稿两份产品文件reverse-check通过，八条旧WIP保持原样；最终源码四文件MD5均OK。

最终普通全量 `go test -count=1 ./...` 已exit0；verbose全量仍在计数，未提前填入推算数字。整轮新增8个func Test、旧函数删除0、旧V/B/S断言修改0/0/0、新增Skip0。文档合约检查exit0。

第一轮目视：浅色registry取消的before右上角仍有带阴影的“安装失败”浮层，占据页头；深色推荐等待图中，spinner仅出现在推荐行胶囊，本地选择按钮仍正常可用。局部沿用原受光边、圆角与字体，没有加新的玻璃层。

## 偷了什么

本次重新真实读取 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography；Apple DESIGN取单一交互蓝、控件8px圆角与1px边框的职责，映射本仓既有token。读取为桌面集成设置与安装审核；视觉冒险度3（可预期）、动效强度2（即时）、密度7（保留已有工具信息）。用户已指定轻便液态玻璃，不启用四方向选择或外部模型。

- craft-loop工艺菜单 **#4 签名交互**：同入口审核中的有效派发上限 **1次**，重复Click **0次**；取消后 **0ms** 解除等待，不额外插动画。
- **#6 编辑细节**：把状态留在发起位置，失效回答/取消的结果通知 **0条**，常驻提示增加 **0px**；取消不会错误地给用户一条失败信息。
- **#12 材质细节**：等待复用既有胶囊受光边 **1px** 与框架单次禁用透明度 **0.5**；新增玻璃面和阴影 **0层**。通用8px控件圆角继续沿用，胶囊仍为原全圆角，不把两者混为一谈。

用户要求修已有功能优先，未为凑菜单五项添加装饰。**这轮为什么更轻、更有空间感**：等待只占原按钮，取消和过期确认不会误装新工具、制造结果浮层；既有底/面/浮层各自只承载有效内容。

## 两轮图像复核

第一轮四视角问题：艺术总监看到取消后的错误浮层抢占页头；工程师看到安装声明变化仍落盘；任务用户无法判断哪个入口已在等待；读者看到“取消”被解释成“安装失败”。第二轮集中查四处：入口等待环原位、其他入口独立、旧审核不产生工具行、取消后页头恢复。原字号较小仍属于既有拍板项，不冒充已解决。

八项深浅同帧前后均有可测像素差，最小为推荐等待浅色 **1724像素**。原图32张，逐项对照16张，局部4×6张、总览2张。4×为最近邻检查放大，不冒充原生高DPI。图来自Linux `ui.Tester` 960×1600软件自绘；没有原生权限弹窗或Windows/macOS真机交互证据。

| 项 | 浅色前后 | 深色前后 |
| --- | --- | --- |
| 1 本地等待 | [图](P13-jw/local-busy-light-comparison.png) | [图](P13-jw/local-busy-dark-comparison.png) |
| 2 推荐等待 | [图](P13-jw/recommended-busy-light-comparison.png) | [图](P13-jw/recommended-busy-dark-comparison.png) |
| 3 本地声明变化 | [图](P13-jw/local-permissions-light-comparison.png) | [图](P13-jw/local-permissions-dark-comparison.png) |
| 4 推荐声明变化 | [图](P13-jw/recommended-permissions-light-comparison.png) | [图](P13-jw/recommended-permissions-dark-comparison.png) |
| 5 本地入口变化 | [图](P13-jw/local-package-light-comparison.png) | [图](P13-jw/local-package-dark-comparison.png) |
| 6 推荐入口变化 | [图](P13-jw/recommended-package-light-comparison.png) | [图](P13-jw/recommended-package-dark-comparison.png) |
| 7 深链取消 | [图](P13-jw/link-cancel-light-comparison.png) | [图](P13-jw/link-cancel-dark-comparison.png) |
| 8 registry取消 | [图](P13-jw/registry-cancel-light-comparison.png) | [图](P13-jw/registry-cancel-dark-comparison.png) |

[像素差](P13-jw/image-differences.txt)、[浅色总览](P13-jw/overview-light.png)、[深色总览](P13-jw/overview-dark.png)。前三批before/after来自被中断前已落盘的真实测试，第四批本次单独注入两条缺陷重新渲染；没有拼造UI。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，主题沿用现有近黑/浅色 |
| A2 非系统字体 | 中文系统字体规则优先；不为网页通则换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置页 |
| A5 三等分/万能卡片 | 无新增卡片 |
| A6 假数据 | 产品无假指标；工作区工具是隔离磁盘事件夹具 |
| A7 文案空词 | i18n零改；取消时去除错误结果 |
| A8 眉批 | 无新增 |
| A9 H1换行 | 既有集成页标题完整可见 |
| A10 Bento | 不适用 |
| A11 左侧竖线 | 无新增装饰 |
| B1 读取/拨盘 | 3/2/7与既有功能面一致 |
| B2 主题 | 每个Tester明确设置深/浅，未混主题 |
| B3 记忆点 | 等待与回答只属于实际审核过的安装 |
| B4 动效气质 | 状态即时更新，未增加高频动画 |
| C1 目的/频率 | 原忙态环表达确有操作在飞；不加键盘动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 沿用原颜色与按压；没有新增长过渡 |
| C4 origin | 等待环在原胶囊；结果复用原宿主浮层 |
| C5 reduced-motion | 新事件全用ReduceMotion=true；原降级仍在 |
| C6 hover查询 | Go自绘，不适用CSS媒体查询；原指针路由不变 |
| C7 布局动画 | 无新增 |
| C8 重播keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用Go自绘 |
| C11 慢放 | 无新动效；只验静态确定帧，不声称真机慢放 |
| D2-1 字体 | 系统字体，无webfont |
| D2-2 字号/行高/字重 | 通用≥14px/1.5未满足；既有Caption9.9/正文10.8与行盒需拍板，未越权改 |
| D2-3 混排空格 | i18n文案零改，无新中文句子 |
| D2-4 标点/对比 | 原墨与标点不变，不声称所有次要墨达到7:1 |

项目可复现缺陷写入§6.250和事件测试；没有把单次修复写成全局审美偏好。用户指定已有方向与不扩张规则优先于通用skill的五项工艺数量、浏览器viewport与多方案要求。

## 仍待

1. 下一轮从已有安装表单的失败后恢复继续查：registry输入名/版本在失败、取消与重新提交之间的归属，以及本地选择期间取消后的键盘出口；先真实事件复现，不把代码线索算作bug。不重复本轮审核摘要/入口变化与深链取消。
2. 生成工具编辑/转换覆盖、重复新包ID覆盖与重装事务、配置模板失败重试、另一端冲突、字号/行盒、accent禁用填充、未知多选/Radio、极小步长、176-9/161-4保存期草稿、终端S1b/S1c/S2尾继续需拍板。
3. 旧ImageCopy/Rescan并行时序线索保留，不改旧B断言、不加睡眠或容差消红。Windows/macOS仅编译，原生权限弹窗未截图。

## 最终门槛

2026-11-08 03:29 UTC 完成。最终 **5998 PASS / 0 FAIL、3403个测试函数**；相对本轮开盘5974/3395为 **+24 PASS、+8函数**，相对接续HEAD5992/3401为 **+6 PASS、+2函数**。普通、verbose完整测试均exit0，gofmt输出空，linux/windows/darwin的vet/build全部exit0，文档合约检查exit0。

整轮M1/M2/M3/M4四组实红；接续M4分两次独立注入，均MD5逐字节还原、touch复验通过（3+3 PASS）。本次八项定向24 PASS，新增两项race6 PASS、exit0；整组race外层143的限制保留在上文，不能把该次包装为干净退出。旧V/B/S断言修改 **0/0/0**，旧测试函数删除 **0**，新增Skip **0**。

本次从03:11到完整门槛约18分钟，接续已有三批六项，未把中断等待计作开发时长，也未增加无关项目凑时长。首次实施三批分别推送 `18326336 / 41dadd06 / 9b5f1b01`；第四批独立接续提交随本报告推送，之后只补交接SHA与stash处置。

原始证据：[本轮开盘](P13-jw/round-baseline.txt)、[接续基线](P13-jw/resume-baseline.txt)、[普通全量](P13-jw/full-test.txt)、[verbose全量](P13-jw/full-verbose.txt)、[取消race](P13-jw/cancel-race.txt)、[M4a红](P13-jw/M4a-red.txt)、[M4b红](P13-jw/M4b-red.txt)、[源码MD5](P13-jw/final-source.md5)、[抢救原patch](P13-jw-orphan.diff)。每个平台完整日志及exit文件也在本轮目录，不只保留终端口述数字。
