# P13-kf · 运行结果的阅读与退出恢复

开盘352f8cc4，pull已同步且工作树干净。Codex独立执行，未委派、未换模型。给定done.log开盘不存在。最新P13-ke仍待指向输出阅读；jw安装恢复已经后续轮完成，不重复。

## Stash清点

权威活动清单8条，无新增stale-kill。交接账登记为早期独立WIP；各项反向校验exit1，并非当前待接续抢救稿。保留原件，不将旧主题与Tauri发布链覆盖HEAD。

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

基线与事件复现进行中，只按实测补录。旧V/B/S断言目标0/0/0。

1. 展开后再次Enter无效（旧槽位复用造成看似持焦）→ 折叠入口显式身份，展开交接到原收起键 → 深浅可见断言与两次Enter往返通过。
2. 收起后查看入口失焦 → 收起键持焦时返还原查看入口并一次显现 → 深浅入口完整可见，Enter可立即重开。

第一批定向6 PASS / 0 FAIL，settingsui vet净、gofmt空。M1撤回本批产品实红：`disclosure cannot reverse with keyboard`与`run outlet outside viewport: label="查看输出" focused=false`；MD5还原、touch复验6 PASS。V/B/S旧断言0/0/0。完整开盘门槛尚在运行，不填写未完成的全量数字。

第一批87326334已推送。开盘完整日志最终6327 PASS / 0 FAIL，go test exit0，3477函数（包装命令末尾rg无FAIL匹配返回1，不是测试失败）。

3. 阅读中的结果被外部折叠后丢失焦点 → 输出面记录现有阅读焦点，折叠时回到原查看入口 → 深浅入口完整可见、Enter重新展开，输出原字节不变。
4. 正文被空结果替换后键盘落空 → 可选择正文退役时交接原收起键 → 深浅空态可退出；存储结果仍为空，不复制旧结果。

第二批新增场景6 PASS，连同第一批12 PASS / 0 FAIL，settingsui vet净、gofmt空。前图实红均为`run outlet outside viewport`且focused=false。首次夹具没有滚到正文，只记夹具错误；补真实阅读位置后才计复现。V/B/S旧断言0/0/0。M1已实跑，第二个正式mutation留给结果选区隔离。

第二批41da540a已推送。

5. 正在阅读的集成变为孤立包，结果面退役但焦点落空 → 按现有退役规则回到页面退出入口 → 深浅Close持焦且可见，Enter恰好关闭一次，历史字节保留。
6. 正在阅读的集成从列表移除，输出未登记退出路径 → 纳入现有missing-notice恢复登记 → 深浅Close持焦可用，原结果记录不删除。

第三批累计18 PASS / 0 FAIL，settingsui vet净。首次夹具漏接Actions.Close，不作为产品证据；接回真实页面出口后M2回注第二批产品，四场景实红`retired output lost page keyboard exit`，两产品文件MD5还原、touch后18 PASS。V/B/S旧断言0/0/0。

第三批4ee579c0已推送。

7. 新运行以前次结果为前缀或完全相同，复制会沿用前次选区 → 每次SetRunResult给予正文独立身份，当前读者焦点交接到新正文 → 深浅×相同/前缀四场景复制前残余选区为0；重新全选复制完整新结果，普通重绘保留当前选区。
8. 截断标记新增或撤回会改变头部槽位，收起键失焦 → 收起键使用独立稳定身份 → 深浅×标记增删四场景保留原焦点，Enter恰好收起。

第四批累计43 PASS / 0 FAIL（含14个用户已转移到Close的保护场景），settingsui vet净、gofmt空。M3回注第三批产品，八个场景实红`new run inherited previous result selection`或`run outlet outside viewport: label="收起输出" focused=false`；MD5还原、touch后43 PASS。旧V/B/S断言0/0/0。

第四批5c37bd22已推送。三平台vet/build六项均exit0，gofmt输出空。完整普通/verbose验收运行中，最终实数随后补录。

## 偷了什么

- 从craft-loop.md工艺密度菜单 **#3 品牌focus环**取“当前动作有一个可见目标”：原控件矩形必须完整进入正文视口，容差 **0px**，沿用原蓝环；不把该矩形断言等同于外扩描边完全无裁切。
- 从 **#4 签名交互**取一次显现：焦点交接时一次最小ScrollIntoView，新增动画 **0ms**。用户已转去Close时，七种刷新×深浅共十四场景的页面 **100px** 偏移严格不变。
- 从 **#2 selection定制**取选区归属：每份新运行结果旧选区 **0字**，同一结果普通重绘保留全文选择，选中墨继续用原主题。未引入新颜色或选区样式。
- Apple供体只转译单一交互accent、**1px**边职责与原**8px**矩形圆角；新增材质/阴影 **0层**。不为通用菜单五项配额添加纹理、插图或装饰卡片。

这轮为什么更轻、更有空间感：结果阅读、收起与退出仍发生在原内容层，当前可继续的目标保持明确，用户无需回到列表顶端重新找入口；没有增加遮罩与框线。

## 两轮采样与视觉复核

每批先用旧产品运行真实事件得到前图，再修复并重拍；M1/M2/M3重新撤回缺陷确认pin实红，MD5还原、touch后复验。两处夹具错误已如实记录，不算产品失败。当前40张原始图来自Linux ui.Tester完整720×520设置页，20对照含同文新结果与截断增删分支。4×图为最近邻放大，非原生高DPI。

已亲自查看深浅总览、展开原始1×前后与收起键4×局部。艺术总监视角：旧图当前动作缺少焦点记号，蓝环恢复后只强调一个控件；工程师视角：槽位复用会让focused读数与Enter能力脱节，所以补连续两次Enter；任务所有者视角：退出后能在原入口重开，孤立/移除后能退出页面；读者视角：新结果不应看起来沿用了上次选择，修后旧选中底色消失。第二轮复核发现底边外扩焦点环的裁剪仍值得独立测量，已进入仍待，未以扩大容差掩盖。

| 项 | 图上观察及验证边界 | 浅色 | 深色 |
| --- | --- | --- | --- |
| 1 展开 | 收起键蓝环出现，页面最小上移 | [对照](P13-kf/expand-light-comparison.png) | [对照](P13-kf/expand-dark-comparison.png) |
| 2 收起 | 查看输出整行出现蓝环 | [对照](P13-kf/collapse-light-comparison.png) | [对照](P13-kf/collapse-dark-comparison.png) |
| 3 外部折叠 | 当前鼠标模态像素相同；Enter可重开是行为证据 | [对照](P13-kf/external-fold-light-comparison.png) | [对照](P13-kf/external-fold-dark-comparison.png) |
| 4 空结果 | 当前鼠标模态像素相同；Enter可收起空态 | [对照](P13-kf/empty-result-light-comparison.png) | [对照](P13-kf/empty-result-dark-comparison.png) |
| 5 孤立包 | 当前鼠标模态像素相同；Close接焦且Enter关闭一次 | [对照](P13-kf/orphan-light-comparison.png) | [对照](P13-kf/orphan-dark-comparison.png) |
| 6 集成移除 | 当前鼠标模态像素相同；页面出口可用，历史结果保留 | [对照](P13-kf/removed-light-comparison.png) | [对照](P13-kf/removed-dark-comparison.png) |
| 7 新结果 | 正文不再沿用旧选中底色，相同文本分支亦验证 | [对照](P13-kf/new-result-light-comparison.png) | [对照](P13-kf/new-result-dark-comparison.png) |
| 8 截断刷新 | 标记增删后收起键保留蓝环 | [对照](P13-kf/truncation-added-light-comparison.png) | [对照](P13-kf/truncation-added-dark-comparison.png) |

[浅色总览](P13-kf/overview-light.png)、[深色总览](P13-kf/overview-dark.png)。[像素实数](P13-kf/image-differences.json)中12对非零、8对为0；没有把同一修复的深浅或分支重复算作独立修复。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹 | 无新增装饰，原语义底色保留 |
| A2 字体 | 中文系统字体规则优先，不为换风格换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 功能设置页，不适用 |
| A5 卡片 | 原分组不增加容器 |
| A6 假数据 | 天气工具为事件夹具，未写入产品数据 |
| A7 空词/感叹号 | 产品文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原设置标题保留 |
| A10 Bento | Go表单，不适用 |
| A11 装饰竖线 | 无新增 |
| B1 设计读取/拨盘 | 功能阅读区、稳定3/即时2/紧凑7一致 |
| B2 主题 | 深浅独立，不混主题 |
| B3 记忆点 | 当前结果可以直接读、收起和退出 |
| B4 动效气质 | 新增0ms，恢复即时 |
| C1 目的/频率 | 仅退役/替换时交接，不连续追随焦点 |
| C2 all/ease-in/scale/fade | 无新增 |
| C3 时长/按压 | 原按压面保留，无新增过渡 |
| C4 origin | 原位置恢复，不加浮层 |
| C5 reduced-motion | 事件夹具ReduceMotion=true，无新增动画 |
| C6 hover媒体 | Go自绘，不适用CSS查询 |
| C7 布局动画 | 无新增 |
| C8 keyframes | 无新增 |
| C9 手势 | 原滚轮与正文选取模型保留 |
| C10 Framer | 不适用Go |
| C11 慢放 | 确定帧复核，无新增动效；未声称真机慢放 |
| D2-1 字体 | 现有系统中文字体，无新增webfont |
| D2-2 字号/行高/字重 | Caption9.9、正文10.8及输出1.45行高不满足通用14px/1.5；沿用需拍板，不擅改高度链 |
| D2-3 混排 | 产品文案零改，夹具含真实中文/数字混排 |
| D2-4 标点/对比 | 原标点与主题墨保留，未声称全部次要墨达到7:1 |

项目缺陷沉淀为§6.259与事件pin，没有把项目行为修复写成通用审美偏好。新系统/依赖/外部模型/外发消息均0。Windows/macOS仅编译，未做原生设备交互。

## 全量首跑与S类处置

完整普通/verbose首跑均exit1，verbose实测6369 PASS / 1 FAIL。唯一失败为`TestTheEmptyCaptureStaysAheadOfTheTruncationAside`：`the captured text no longer wears runOutputFont`。实查是源码精确调用链pin仍要求无key的RichText后直接接Font；实际字体仍为runOutputFont，新增的是结果key与reader绑定。按S类更新精确完整调用链，原字体、旁白墨、像素断言不动；最终台账 **V 0 / B 0 / S 1**，不加容差、不删断言。首跑日志保留为initial-full-*，随后重跑完整门槛，不将6369包装为全绿。

## 仍待

1. 下一轮首选运行结果焦点环在正文视口底边的完整度，以及长输出键盘选择到末行的可见性。先测外扩环与裁剪，不把矩形包含当作完整光栅证明；不改高度链与全局键盘模型。
2. 生成工具编辑/转换覆盖、重复包ID覆盖、重装事务、配置模板失败重试、另一端冲突、字号/行盒、accent禁用填充、未知多选/Radio、极小步长、176-9/161-4保存期草稿、终端S1b/S1c/S2尾继续需拍板。
3. 八条早期WIP保留；历史ImageCopy/Rescan/memo线索不放宽B断言、不加睡眠掩盖。

## 最终门槛与边界

最终普通与verbose全量均exit0，**6370 PASS / 0 FAIL / 3486函数**；相对开盘6327 PASS/3477函数增加43 PASS/9函数。gofmt空，三平台vet（含更新后的_test.go）及三平台build均exit0，文档CI exit0。完整首跑约550秒，重跑仅将测试超时上限设为20分钟以容纳并行验收，未改变测试内容或断言。

M1/M2/M3三组实红并逐字节MD5还原、touch复验；结构pin修正后定向44 PASS。最终V/B/S旧断言台账 **0/0/1**，旧测试函数删除0、新增Skip0。最终产品MD5与第四批一致，S类只更新精确调用链断言。

八项四批产品87326334 / 41da540a / 4ee579c0 / 5c37bd22均已推送；本次结案提交归档结构pin、完整门槛与交接。40张原图、20对照、深浅总览及4×局部均已入库。Windows/macOS未做原生交互，不把编译通过当作真机验收。

本轮从2026-11-08T08:36:00.160Z到最终证据收集约36.3分钟，短于45–75分钟目标；八项与门槛完成后没有以额外改动凑时长。顶层工具调用收敛在40次，最后一次编排的单次进程等待均不超过50秒，并持续输出验收进度。

证据：[开盘](P13-kf/baseline.txt)、[首跑失败](P13-kf/initial-full-verbose.txt)、[最终普通全量](P13-kf/full-test.txt)、[最终verbose全量](P13-kf/full-verbose.txt)、[结构pin复核](P13-kf/structure-pin-recheck.txt)。
