# P13-hf · 下拉选项的身份、清空与可用状态

开盘 f8433521，工作树干净；pull 已同步。原 done.log 不存在。前轮两条补档提交仍待推送，已开始重试；结果稍后据实补记。

## stash 清点

八条活动 stash 均 reverse-check 退出 1，逐条 stat 已复核，并与 P13-he 的语义判断及 stash-backlog 对照。这些是早期 WIP，不是 stale-kill；原抢救稿已归档清理，无新 orphan。不重放过期主题/行为或 Tauri 发布链。

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | P7 keycap 与旧主题，3 文件 | 保留原件 |
| stash@{1} / 8af94a07 | 设置行距/滑块旧刻度，3 文件 | 保留原件 |
| stash@{2} / 7e4aa710 | 卸载旧布局与探针，2 文件 | 保留原件 |
| stash@{3} / 1e3aec94 | 旧数字框与透镜，2 文件 | 保留原件 |
| stash@{4} / 363f5a42 | 旧确认带/快捷键，2 文件 | 保留原件 |
| stash@{5} / 9b8ce23c | P7 快捷键拒绝反馈，9 文件 | 保留原件 |
| stash@{6} / 90d314b2 | action bar 旧键盘合约，3 文件 | 保留原件 |
| stash@{7} / 7a9f6e13 | 旧 Tauri 发布链，1 文件 | 保留原件 |

## 增量验收

1. **同名插件选项无法切换**：两个“搜索”分别是 files/apps，Home + Enter 原来仍为 apps → 下拉使用真实 ID，文字仅绘制 → 深浅都写入 files；原始红串 `same-name option was not selected by ID: got apps, want files`。
2. **旧 ID 等于合法标签时无法修正**：存储“文件”，选择标签“文件”仍保存非法旧值 → ID 与显示名分离 → 深浅都提交 files；原始红串 `unknown ID matching a label could not be replaced: got 文件, want files`。

第一批新增与相关旧回归 **56 PASS / 0 FAIL**。旧 V/B/S 断言零改。复现即为两项原实现反证；正式 mutation 轮末另跑。采用框架 SelectBase，保留原键盘模型、触发器尺寸和原菜单墨；菜单滚动使用公开 Scroll API。

开盘全量实测 **3674 PASS / 1 FAIL**；失败为未改 shell 的 `TestDeepLinkConnectAndRegister` 临时目录清理与异步连接写入竞态：`TempDir RemoveAll cleanup: unlinkat ... directory not empty`。函数实测 2746。不能把此前 3675/0 冒称本轮基线。第一批提交前复跑该项并记录结果。

前轮补档推送已成功：远端 bb89669a → f8433521。

第一批 `e7b4201d` 已推送；CI 与身份回归通过。基线清理竞态单项复跑 3 次均通过。

3. **打开后换序把 Enter 交给另一项**：应用高亮时换到首行，原来 Enter 选择文件 → 保存高亮的真实值并在新列表发布后定位 → 深浅均选应用。原串 `menu reorder moved the keyboard choice: got files, want apps`。
4. **删除高亮项后误选邻居**：删除应用后原索引落到便条 → 原高亮消失时回到仍有效的当前值 → 深浅保留文件，再主动 End/Enter 能选便条。原串 `removed highlight selected an unrequested neighbour: got notes, want files`。

第二批选择相关回归 39 PASS / 0 FAIL（四项新修复 12，前轮守卫 27）。旧断言零改。

第二批 `47e868fb` 已推送。第一轮目视：浅色换序菜单高亮从文件移回应用；深色删除菜单高亮回到文件。艺术视角继续保持单一列表面；工程视角关注动态列表身份；任务视角下一步补清空出口；读者视角同名的清空动作与真实值需要可辨认。

5. **实际值与“未设置”同名时无法清空**：旧实现为避碰直接删除清空入口 → 清空以独立 none 身份表示，真实同名值加引号区分 → Home/Enter 写 nil，再 Down/Enter 可写真实词条。首次夹具误写“Not set”而非实际词条，旧实现通过，未计修复；改为读取 a.copy().ConfigNone，使用 Go overlay 实跑 HEAD 原文件复现红。
6. **schema 空项是一行无效空白**：可选字段 files 上按 Up/Enter 落到空项却仍为 files → 空 schema 值与已有清空入口合并 → 深浅都写 nil，菜单少一行空白。原串 `empty schema choice did not clear the optional field: got "files", want nil`。

第三批新增与相关回归 **59 PASS / 0 FAIL**；旧断言零改，V/B/S 0/0/0。保留原有 null 清空语义和字段 28px 地板。

第三批 `0193a958` 已推送。同名清空的真实原串为 `reserved display name removed the clear action: got "None", want nil`。

7. **没有选项仍弹空菜单**：空集合接受焦点并画空浮层，选项到来后旧浮层还自动出现 → 空集合触发器禁用、熄受光边，值仍可读 → 深浅拒绝空菜单，补入选项后重新点击能提交 apps。原串 `empty select still accepts focus and opens a blank menu`。
8. **锁定后菜单仍悬在页面上**：触发器变淡但已开浮层继续显示，解锁复活旧菜单 → 不可用时退役活动触发器及菜单，恢复后从关闭态开始 → 深浅锁定帧不见 apps，解锁再开正常提交。原串 `locked field left its live option popup on screen`。

第四批八项与相关回归 **65 PASS / 0 FAIL**；额外加入 80 项长菜单的 End 滚入视口、Enter 选择与 Escape 不写值守卫，不计第九项修复。旧断言零改。锁定触发器只走框架一次 0.5 淡化；未改变保存期间字段是否可编辑的拍板项。

第四批 `cab4c474` 已推送。长菜单守卫曾发现内滚动未受窗口限制：第 80 项 Y=2288.3015，超出 400px 窗口；修为实际触发器上/下剩余空间的最大值减菜单边距，守卫通过。第四批最终 **68 PASS / 0 FAIL**。这项修正阻止本轮下拉实现带入滚动回归，不冒充原产品第九项缺陷。

## 正式 mutation

| 组 | 注入 | 实跑红串 | 还原 |
|---|---|---|---|
| M1 | 选项身份改为 Label | `unknown ID matching a label could not be replaced: got 文件, want files`；同名组也红 | settingsui.go 逐字节 OK，touch 后重跑 |
| M2 | 禁用高亮值迁移 | `menu reorder moved the keyboard choice: got files, want apps`；`removed highlight selected an unrequested neighbour: got notes, want files` | selectvalue.go 逐字节 OK，touch 后重跑 |
| M3 | 强行开启不可用控件 | `empty select still accepts focus and opens a blank menu`；`locked field left its live option popup on screen`；`unlock resurrected the old popup` | selectvalue.go 逐字节 OK，touch 后重跑 |

三组命令均退出 1，产品文件恢复后本轮 9 个测试函数共 **27 PASS / 0 FAIL**（8 个修复 + 1 个长菜单守卫，各深浅子项）。原始日志与指纹归档 P13-hf-evidence；变异时 settingsui.go=`6b668a919af88967fc155baeb99bea72`，selectvalue.go=`0a477dc3dc19dceb65aa437fcbeba533`，config.go=`a57675cfd0043cc174b1028bef02b265`。终审另外保持辅助技术读出的 Value 为可见文字，并让无候选的配置字段走 DimFace；最终全量覆盖这两处收尾。

## 图片与两轮复核

ui.Tester 原图 48 张、并排比较 24 张、联系图 4 张、4× 局部 2 张，共 **78 PNG**。插件图 720×400，集成配置图 720×320。原图由 `/tmp/floter-P13-hf-<场景>-<主题>-<阶段>.png` 入库到 P13-hf-images；这些是自绘无头实帧，不是三平台真机截图。

| 组 | 深浅比较 | 目视结论 |
|---|---|---|
| 换序 | [浅](P13-hf-images/reordered-menu-light-compare.png) / [深](P13-hf-images/reordered-menu-dark-compare.png) | 蓝色高亮从错误的文件移回应用，当前保存值仍有原中性面 |
| 删除 | [浅](P13-hf-images/removed-menu-light-compare.png) / [深](P13-hf-images/removed-menu-dark-compare.png) | 便条不再继承高亮，文件重新获得高亮 |
| 同名清空 | [浅](P13-hf-images/reserved-menu-light-compare.png) / [深](P13-hf-images/reserved-menu-dark-compare.png) | None 与真实字符串 "None" 分成可辨认的两项 |
| 空项 | [浅](P13-hf-images/empty-option-menu-light-compare.png) / [深](P13-hf-images/empty-option-menu-dark-compare.png) | None 与 files 之间的空白行消失，浮层变短 |
| 空集合 | [浅](P13-hf-images/empty-menu-light-compare.png) / [深](P13-hf-images/empty-menu-dark-compare.png) | 触发器变淡、受光边熄灭，下面的无内容浮条消失 |
| 锁定 | [浅](P13-hf-images/locked-menu-light-compare.png) / [深](P13-hf-images/locked-menu-dark-compare.png) | 已开的选项浮层退出，恢复按钮不再被盖住 |

前两项 ID 修复的闭合触发器文字可以相同，图片不证明 ID 已写对；由真实 Home/Enter 事件后的保存值断言验收，不编造像素差。第二轮复核覆盖全部六组可见变化；4× 图显示胶囊边、箭头与选项圆角连续，没有额外描边。艺术视角压住装饰性提示，工程视角补上长列表滚动边界，任务视角验清空与解锁恢复，读者视角用引号区分真实字符串。用户的既有字号问题仍保留例外。

## 偷了什么

已实际阅读 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography，并读取 Apple DESIGN：单一交互色 #0071e3 映射本仓 Accent/Focus；#f5f5f7/#1d1d1f 映射现有近白/近黑；980px 胶囊沿用现有 999px。没有照抄网页 blur。

- craft-loop 工艺菜单 **第 3 项「品牌 focus 环」**：保留 2px 环与一个高亮对象；高亮随真实值移动。
- **第 4 项「签名交互」**：键盘与锁态在 1 帧内反馈，新增动画 0ms。
- **第 6 项「编辑细节」**：保留 1px 边，不加解释卡；同名实际字符串用一对引号区分。
- **第 9 项「自定义滚动条」**：菜单一个滚动区，上限为触发器上/下可用空间最大值减 Space(3)+2px；80 项末项确实滚入 400px 窗口。
- **第 12 项「材质细节」**：E1 顶内边 1px；不可用触发器熄边且只退 0.5 一档，新增玻璃层 0。浮层投影从框架内部的固定 0.18 转为现有 ShadowAmbient token（深 0.62 / 浅 0.22），没有新增 token 或阴影层。

为什么更轻、更有空间感：消掉空白项与无内容浮条，锁定后收回不应悬浮的菜单；高亮只占据实际操作的那一项，保留底/面/浮层的任务层次。

## preflight 逐条

| A 反套路 | 结论 |
|---|---|
| A1 渐变/纯黑/霓虹/渐变字 | 通过，沿用项目近黑近白，无新增装饰 |
| A2 非默认字体 | 按中文专属规则与项目字号红线保留系统 CJK，属于明确项目优先例外 |
| A3 斜体 | 通过，无斜体 |
| A4 Hero | 不适用，原生表单 |
| A5 卡片海/三等分 | 通过，无新容器系统 |
| A6 假数据 | 通过，测试用文件/应用/便条与明确 ID |
| A7 空词与感叹号 | 通过，无新增宣传文案 |
| A8 眉批 | 通过，无新增 |
| A9 H1 行数 | 不适用，原有单行标题 |
| A10 Bento | 不适用，无网格改造 |
| A11 装饰竖线 | 通过，仅现有高亮面 |

| B 设计读取 | 结论 |
|---|---|
| B1 拨盘与方向 | 通过，稳定 2 / 即时 2 / 紧凑 6 |
| B2 全页主题 | 通过，每帧单主题，深浅分别出图 |
| B3 记忆点 | 显示词、保存值与高亮各自对应实际对象 |
| B4 动效气质 | 通过，选择即时反馈 |

| C 动效抽查 | 结论 |
|---|---|
| C1 目的与高频 | 通过，键盘即时 |
| C2 all/ease-in/scale(0)/纯 fade | 通过，无新增动画或 CSS |
| C3 时长与按压 | 新增动画 0ms，保留现有触发器反馈 |
| C4 浮层 origin | 通过，SelectBase 从触发器展开 |
| C5 reduced motion | 通过，两套 Tester 均 ReduceMotion=true，功能完整 |
| C6 hover 媒体查询 | 不适用，原生指针事件 |
| C7 动画属性 | 通过，无新增几何动画 |
| C8 快速触发 | 通过，未引入重播 keyframes |
| C9 手势 | 不适用，未改手势/拖拽 |
| C10 Framer | 不适用，Go 自绘 |
| C11 逐帧 | 真实键鼠帧与前后截图覆盖；未声称真机动效测评 |

| D2 中文页 | 结论 |
|---|---|
| 系统字体 | 通过，沿用系统 CJK，无字体下载 |
| 行高/14px/字重 | 既有例外未过：Body 10.8px、Caption 9.9px 与 14px 冲突，580/620 字重沿用；用户禁止擅改高度链，仍列拍板 |
| 混排空格 | 无新增混排句；Value 原样保存，测试来源编号带空格 |
| 标点/暗色 7:1 | 同名实际值采用原样字符串的引号表现；沿用语义墨，未把默认次要墨冒称全部达到 7:1 |

## 门槛与仍待

首轮全量发现两条源码合约红：`configSelect no longer hands its face to fieldFace/fieldFaceEdge` 与 `valueSelect calls Disabled(true) ... table does not name this site`。产品恢复 configSelect 直接调用 fieldFace，并复用既有 lockedControl 禁用入口；旧断言一字不改。原始全量输出归档 test-first.txt。修复后重跑完整门槛，结果如下。没有新增 Skip、删测试或改旧断言。终端会话语义、保存期间草稿归属与字号/行盒拍板项沿用。

最终统一 `LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4`：普通与 verbose 两次全量退出 0；**3702 PASS / 0 FAIL**，相对开盘 3674/1 增加 27 个新增 PASS 与 1 个恢复通过的基线用例。测试函数 **2755（基线 2746，+9）**。gofmt 输出空；Linux、Windows、macOS 的 vet/build 六项均通过。最终原始输出在 P13-hf-evidence/test.txt、test-verbose.txt 与 gates.txt，78 张图片共 1089485 字节。

V/B/S 旧断言修改台账 **0/0/0**；新增九个测试函数保留，未新增 Skip。四批产品每两项即提交推送；最后补齐合约接线与证据归档。本轮工具调用超过 40 次，超出部分用于已完成八项的全量失败修复、门槛与交付，没有继续扩题。执行约 48 分钟，处于目标 45–75 分钟内。

下一轮候选：选择器打开时当前值外部改变，以及选项消失后当前值也不在集合内的组合状态；尚未复现，不计已知缺陷。本轮开盘 shell TempDir 清理竞态已记录，后续复现时固定异步生命周期，不能放宽断言。
