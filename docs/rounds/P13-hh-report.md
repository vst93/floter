# P13-hh · 动态选择与可见边界

起点 628bf3c4，工作树干净、pull 同步；done.log 开盘不存在。已读取 qiaomu-design 打磨路径与项目两份视觉合约，按最新 P13-hg 仍待接续。

## stash 清点

首次开盘以下八条实际 reverse-check 均退出 1，stat 与前轮记录相同。原任务书 stale-kill 已由 P13-go/gq 等轮归档；首次开盘无新 orphan（后续接续见第二次接续表）。这些是早期独立 WIP，不能把 reverse 失败当成未接续功能，也不能用旧主题覆盖当前合约。

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | P7 keycap/旧主题，3 文件 | 保留原件 |
| stash@{1} / 8af94a07 | 旧行距与滑块，3 文件 | 保留原件 |
| stash@{2} / 7e4aa710 | 旧卸载布局及探针，2 文件 | 保留原件 |
| stash@{3} / 1e3aec94 | 旧数字框/透镜，2 文件 | 保留原件 |
| stash@{4} / 363f5a42 | 旧确认带/快捷键，2 文件 | 保留原件 |
| stash@{5} / 9b8ce23c | P7 快捷键反馈，9 文件 | 保留原件 |
| stash@{6} / 90d314b2 | action bar 旧键盘合约，3 文件 | 保留原件 |
| stash@{7} / 7a9f6e13 | 旧 Tauri 发布链，1 文件 | 保留原件 |

## 增量验收

1. **外部刷新为未知值后 Enter 写回 files** → 未知值使用独立菜单身份，更新时收起旧菜单 → 深浅真实 Enter 均保留 remote-source；原串 `Enter silently replaced unknown external value: files`。
2. **当前值和高亮同时移除后 Enter 误写 notes** → 失效菜单退役，保留原 files，重开后必须主动选 → 深浅 Enter 保值；原串 `removed current and highlight silently chose neighbour: notes`。

第一批相关回归 **71 PASS / 0 FAIL**；旧断言零改，V/B/S 0/0/0。开盘全量 **3737 PASS / 0 FAIL**，tracked `func Test` **2764**（接续按起点提交重新计数，初稿 2762 为笔误）。原始反证与深浅前后图已生成。

第一批 8375f206 已推送。

3. **选项短暂移除又回来后，Shift 复活旧范围、多选应用** → 每帧校验并退役消失锚点 → 结果为文件+书签；反证 `reappeared option resurrected its old range: [文件 应用 书签]`。
4. **插件复选长名称直抵卡片右边界、末字被切** → 原 Checkbox 保留，标签单行省略、控件最大宽为父容器 100% → 深浅标签右端落在原内边距内，下一项仍可点；原标签右端 704.6，图上文字贴边，修后留白与省略号可见。初始探针的 700 是估测，正式 pin 采用卡内边界 692；没有放宽旧断言。

第二批相关回归 **74 PASS / 0 FAIL**，V/B/S 0/0/0。目视浅色长复选名称已从硬裁切变为省略，勾选框和应用下一行保持。

第二批 efc1d0fa 已推送。

5. **单选换序后 Space 操作旧位置** → 插件 Radio 将真实 Value 传给原分段控件的身份 → 焦点留在应用、Space 保持 apps；原串 `radio focus moved to a different value after reorder` / `radio Space selected the old position: files`。
6. **移除聚焦单选后 Space 激活替代项** → 被删 Value 的焦点随元素退役 → 保留有效当前值 files；原串 `removed radio focus activated its replacement: notes`。

未知 Radio 的既有拍板语义未改；两项只修已存在的焦点错配。第三批相关 **109 PASS / 0 FAIL**，f9c81c1d 已推送。提交正文遗漏该数，报告补录原始实测。

7. **单选长名撑成 90px 高，邻项也跟着长高** → 原分段文字单行省略 → 原 30u 地板恢复，应用与长名保持同一基线；原串 `long radio label inflated or displaced segments: long={29.8 75.70001 329.30002 90} next={360.9 107.20001 329.30002 27}`。
8. **多选焦点换序后留在不可见的末项** → 仅位置变化时把聚焦行滚入视口 → 原 0×0 裁切目标重新可见，Space 仍操作文件。原串 `reordered focused option is outside its viewport: {159 442.5 0 0}`。

第二轮图审发现只 reveal checkbox 会切掉比方框更高的中文标签尾部 1.3px，改为 reveal 整行。新 pin 比较修改前自然字高与修改后可见字高，并相对真实字段标签中心检查 66px 视口；撤销最初误把视口固定为 y=20..86 的探针假设，原 0×0 仍必红。

第四批相关回归 **115 PASS / 0 FAIL**；八项产品修复均有深浅原实现失败证据。V/B/S 0/0/0，无旧测试删除/放宽。

## 正式变异

M1：未知值隔离条件强制 false，实跑退出 1，四个深浅场景红：`Enter silently replaced unknown external value: files`、`removed current and highlight silently chose neighbour: notes`。

M2：撤掉逐帧锚点校验，实跑退出 1，深浅两场景红：`reappeared option resurrected its old range: [文件 应用 书签]`。两个注入点分别跑各自测试，恢复后 MD5 与 touch 复跑结果另记。

## 接续验收（2026-11-06）

在 `fdf0edf6` 接续：pull 已同步、工作树为空，八项产品修复已经分四批推送。遵守 `docs/AGENT-NOTES.md` 的“同一轮被打断后重派沿用轮号”，本次完成 P13-hh 的余下验收，不把已完成八项重新算成下一轮。本轮四批提交为 8375f206 / efc1d0fa / f9c81c1d / fdf0edf6。

重新清点八条 stash，SHA 与上表逐项相同，reverse-check 均退出 1；无新增 stale-kill，保留原件的判断不变。done.log 在接续开始时仍不存在。

两条原始变异输出及还原记录均在：`md5sum -c` 接续复核两文件均 OK；selectvalue.go 为 `9d1460a7986e8b9a3b214a0dfcbef3bf`，config.go 为 `e212705751a5140c23437438448e972b`。恢复后的八个测试 × 深浅，加父测试共 **24 PASS / 0 FAIL**。原始失败和逐字节恢复记录随证据目录归档。

接续第一轮图审：浅色长复选名字从贴卡片右边硬切，变成内边距内的省略号；深色单选的两段回到同一行高；深色多选移位后“文件”的完整字高与右侧勾选框同时可见。gofmt 空、Linux/Windows/macOS vet 和 build 均通过，完整普通/verbose 测试运行中，未提前填写最终数字。

## 第二次接续（2026-11-06，恢复出口）

以上接续又被中断，产生 `stash@{0}` / `e63176d377f28512a8c81003fa93173dd2a6b91b`。本次已用 `git apply -3` 接回独有的 3 文件改动，并从第三父树恢复截图与原始日志。接回后 reverse-check=0；其余八条早期 WIP SHA 不变、reverse-check=1，保留原件。当前八条 WIP 的 ref 是上表编号加 1；新抢救稿会在验证提交后归档原始对象并 drop，清理后的编号恢复上表。

| 本次开盘 ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / e63176d3 | 3 文件独有恢复出口测试/报告及第三父截图日志 | apply -3 接回；补修出口，验证后单独提交；原对象另留 archive 引用 |
| stash@{1} / 422483b4 | 旧 keycap/主题 WIP，reverse 1 | 保留原件 |
| stash@{2} / 8af94a07 | 旧行距/滑块 WIP，reverse 1 | 保留原件 |
| stash@{3} / 7e4aa710 | 旧卸载布局/探针，reverse 1 | 保留原件 |
| stash@{4} / 1e3aec94 | 旧数字框/透镜，reverse 1 | 保留原件 |
| stash@{5} / 363f5a42 | 旧确认带/快捷键，reverse 1 | 保留原件 |
| stash@{6} / 9b8ce23c | 旧快捷键反馈，reverse 1 | 保留原件 |
| stash@{7} / 90d314b2 | action bar 旧键盘合约，reverse 1 | 保留原件 |
| stash@{8} / 7a9f6e13 | 旧 Tauri 发布链，reverse 1 | 保留原件 |

补验真实复现第 1/2 项的恢复出口缺陷：`correcting unknown value lost the select keyboard focus`、`replacing removed choice lost the select keyboard focus`，深浅四场景失败。原因是主动选择有效值后，原未知值 key 被替换。改为每控件自己的生命周期编号：仅未知值/选项变化或可用性变化退役菜单；主动修正保留同一个触发器。零尺寸绝对定位状态节点不参与布局、不接收指针，不包裹字段；同父控件使用独立身份。补验连续键盘选择、同父两个菜单、另一个菜单更新时不夺焦点，不另计为第九项。

接续相关回归 **119 PASS / 0 FAIL**（settingsui 的 Choice/Select 与 control 的 Segment）；原有断言没有修改，救回的恢复出口断言只增加连续键盘验收。新增一个同父选择器隔离测试，防止菜单生命周期串扰。

最终实现重新实跑三条 mutation（原件 `P13-hh-evidence/final-mutation{1,2,3}.txt`）：

1. M1 禁用 unknownChanged：退出 1，原串 `Enter silently replaced unknown external value: files`、`removed current and highlight silently chose neighbour: notes`，两个深浅场景均红。
2. M2 移除逐帧锚点校验：退出 1，原串 `reappeared option resurrected its old range: [文件 应用 书签]`，深浅均红。
3. M3 把未知/有效状态重新放进触发器身份：退出 1，原串 `correcting unknown value lost the select keyboard focus`、`recovered select could not continue with the keyboard`，深浅均红。

三条逐一还原。最终 `md5sum -c final-mutation.md5`：selectvalue.go `45bddfee1cdf31a4bffee9138907000d`、config.go `e212705751a5140c23437438448e972b` 均 OK；touch 后 `-count=1` 复跑 **27 PASS / 0 FAIL**。旧 mutation.md5 记录的是补修前文件，selectvalue.go 与该旧摘要不同是预期，不冒称新文件通过旧校验。

## 偷了什么

本次重新读取 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography 与 Apple DESIGN。读取为桌面配置表单，稳定 2 / 即时 2 / 紧凑 6；功能是显示真实选项、明确选择、保持焦点可继续操作。Apple 的 2px 焦点标记、单交互色 #0071e3、近白/近黑 #f5f5f7/#1d1d1f 是供体，转译到现有主题语义，不直接覆盖已落地颜色。

1. craft-loop 菜单 **3（focus 环）**：保留框架 2px 焦点反馈，把其归属钉回真实选项；移除值不把焦点传给邻项，恢复有效选择后仍能连续操作。
2. 菜单 **4（签名交互）**：动态更新和键盘选择即时生效，新增等待 **0ms**；只有聚焦多选的位置变化才滚入视口。
3. 菜单 **6（编辑细节）**：长复选和长单选固定 **1 行**，单选不再膨胀至 **90px**，保持原 **30u** 地板；标签右端不越过 **692px** 卡内边界。
4. 菜单 **9（滚动区）**：沿用 **66px** 多选视口，滚入整行而非方框，保留完整自然字高；未换序时仍允许普通滚轮越过焦点。
5. 菜单 **12（材质细节）**：沿用既有 **1px** 边与 E0/E1/E2；新增材质层 **0**。失效菜单收回，不用额外浮条补偿状态错误。

第 3/9 项是本轮可见改进，3/4 是焦点行为落地，12 是约束核验；不把沿用 token 算作新增产品修复。为什么更轻、更有空间感：失效浮层退出，长文让出邻项空间，焦点与整行文字回到同一可见区域。

## 两轮图审与 preflight

真实产物来自 Go `ui.Tester`，视口为插件面 **720×400**、配置区 **720×320**；不是网页或浏览器截图。四张 `review-{light,dark}-{1,2}.png` 左旧右新，全部已目视，`focus-detail-*` 为 4× 局部。最后恢复出口另有 `recovered-focus-{light,dark}.png`。

第一轮四视角：艺术总监看到长单选形成厚重的多行胶囊；工程师看到换序焦点在视口外；任务使用者在未知值后会被 Enter 改写；读者看到长复选贴边硬切。第二轮已收住长名、退役失效菜单、滚入整行，并补修主动选择后的连续键盘出口。浅图上长复选右端出现省略号与留白；深图上“文件”整行与勾选框一起可见。焦点归属与恢复路径由按键断言验收，静态图不冒充交互证据。

| 门禁 | 逐条结论 |
|---|---|
| A1 颜色 | 沿用近黑/近白与语义蓝，无新增紫蓝渐变、纯黑背景、霓虹或渐变字 |
| A2 字体 | 通用“每轮换字体”不适用：中文优先级与项目合约要求保留系统字体 |
| A3 斜体 | 新改动无斜体；原有字重 token 不变 |
| A4 Hero | 桌面表单无 Hero，不适用 |
| A5 卡片 | 沿用原字段卡，不增三等分或嵌套卡 |
| A6 数据 | 测试使用真实 schema 形状，无营销假精确数据 |
| A7 文案 | 无新增夸张文案或感叹号，i18n 词条零改 |
| A8 眉批 | 无新增眉批或编号装饰 |
| A9 标题 | 原搜索配置标题保持单行 |
| A10 网格 | 无新增网格空洞；零尺寸状态节点绝对定位、不占布局 |
| A11 左竖线 | 无新增左侧强调线 |
| B1 读取一致 | 保持稳定、紧凑的设置表单，修复状态归属 |
| B2 明暗 | 每帧单一主题，深浅分别出图 |
| B3 记忆点 | 当前真实值、焦点和可见选项在同一处 |
| B4 动静一致 | 高频键盘操作即时反馈，既有轻材质保留 |
| C1 目的/频率 | 本轮新增动画 0；键盘和 schema 更新不等待动画 |
| C2 禁用形式 | 无 transition:all、ease-in、scale(0)、纯 fade 入场 |
| C3 时长/按压 | 沿用 120ms 颜色反馈及已实现按压态，未超过 300ms |
| C4 来源 | 原 SelectBase 菜单仍锚定触发器；失效后收起 |
| C5 减弱动效 | ui.Preferences.ReduceMotion=true 的全部深浅夹具通过 |
| C6 hover | 原生指针状态，CSS media 不适用；未加触摸 hover |
| C7 属性 | 未增加布局尺寸动画 |
| C8 快速触发 | 复用原控件状态机，无重播 keyframes |
| C9 手势 | 本轮无拖拽、弹簧、sheet 手势，不适用 |
| C10 Motion | 未使用 Web/Framer Motion，不适用 |
| C11 抽查证据 | 键盘逐帧测试与 ReduceMotion 截图；无新增动画需慢放 |
| D2-1 字体 | 系统中文字体，未引入字体资源 |
| D2-2 字号/行高/字重 | **不全通过，沿用需拍板**：正文 10.8px、Caption 9.9px 低于 14px，部分既有 580/620 字重不在 400/500/600；不越权改字号或行盒 |
| D2-3 混排 | 产品词条未改；测试中“来源 14”保留中数空格 |
| D2-4 标点/对比 | 无新增产品句子或标点；沿用 token。未新增逐 CTA 的 7:1 像素审计，不冒称全面符合 skill 对比度要求 |

用户的现有合约与授权优先于 skill 通用建议；上述字号与字重例外公开保留，不因此停止已授权修复。没有新增页面、依赖、设置项或会话语义。

最终图对照：8 个状态 × 深浅共 **16 PNG**，最终补修图对四批提交后的 after 图逐像素相同（`final-image-compare.txt` 的 difference bbox 均为 None）；全套旧/新图继续有效。恢复选择后的两张图另外归档，不把焦点修复假充几何改版。标准导出路径另留 `/tmp/floter-P13-hh-<状态>-{dark,light}.png`。

## 仍待

最终门槛实测：`LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4`；**3764 PASS / 0 FAIL**（基线 **3737**，+27），全部 tracked Go 文件的行首 `func Test` **2773**（基线 **2764**，+9）。8 个原修复测试与 1 个隔离回归各有深浅子测试；不以新增回归测试充当第九项产品修复。

`gofmt -l cmd internal` 空，Linux/Windows/macOS 的 `go vet ./...` 与 `go build ./...` 全部退出 0；普通和 verbose 两次 `go test -count=1 ./...` 均退出 0。新增报告表格后 `internal/ci` 单独复核通过，`git diff --check` 空。所有原始输出在 `P13-hh-evidence/`，含 56 PNG、三条最终 mutation、MD5、逐像素对照与门槛脚本。

V/B/S：**0/0/0**。既有视觉 pin、行为断言与结构断言未修改，未删测试、未加 Skip、未放宽容差。新下拉实现新增零尺寸绝对定位状态节点，不改变既有控件盒或可见容器；16 个关键状态的最终图逐像素相同。Windows/macOS 是交叉 vet/build 验证，真实设备未运行，不冒称真机验收。

1. 下一轮首选现有配置输入控件的“schema 原地改型/选项清空再恢复”组合，按真实复现选至少八项；当前只完成动态选择这一族，不把未测路径当作修复。
2. 176-9、161-4 保存期间草稿归属、未知 Radio 值表现、字号/行盒与终端 S1b/S1c/S2 尾沿用需拍板；不擅自实现。
3. 八条早期独立 WIP 继续保留。新的 e63176d3 抢救稿接续本轮验收；旧 stale-kill 归档不重复试合。
4. shell rescan 瞬时 pending 与 DeepLink TempDir 异步清理的历史竞态仍按实测处理，禁止放宽断言。

## 提交与抢救结案

四批产品提交 `8375f206 / efc1d0fa / f9c81c1d / fdf0edf6` 均已在远端；本次接续修复和全部证据提交 **`f7c243f1`** 已推送 `origin/mygo-rewrite`。补修保持同一轮编号，完整交付原有八项。

新抢救稿 `e63176d377f28512a8c81003fa93173dd2a6b91b` 已单独接续提交，清理前确认 HEAD 等于 origin，并确认 archive ref 与原 stash SHA 完全相同后 drop。原对象留在 `refs/archive/floter/P13-hh/e63176d377f28512a8c81003fa93173dd2a6b91b`；活动 stash **9 → 8**，剩余八条编号恢复报告首次开盘表。没有静默丢弃独有内容。
