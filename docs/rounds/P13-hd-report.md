# P13-hd · 配置表单的输入、撤销与错误归属

起点 c8ee659e，2026-11-05；pull --ff-only 已同步，工作树空。Codex 独立执行。基线中文 locale 全量实测 **3624 PASS / 0 FAIL / 2727 测试函数**。done.log 开盘不存在。

## stash 清点

当前八条逐一 reverse-check 均未通过；已对照 P13-hc 报告与 stash-backlog：均为早期独立 WIP，原任务书 stale-kill 已由前轮归档清理，没有新 orphan。不将旧主题与旧 Tauri 发布链覆盖当前实现。

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | P7 keycap 旧主题稿 | 保留原件 |
| stash@{1} / 8af94a07 | 设置行距/滑块旧刻度 | 保留原件 |
| stash@{2} / 7e4aa710 | 卸载旧布局与 scratch 探针 | 保留原件 |
| stash@{3} / 1e3aec94 | 旧数字框与透镜稿 | 保留原件 |
| stash@{4} / 363f5a42 | 旧确认带/快捷键稿 | 保留原件 |
| stash@{5} / 9b8ce23c | P7 快捷键拒绝反馈稿 | 保留原件 |
| stash@{6} / 90d314b2 | action bar 旧键盘合约 | 保留原件 |
| stash@{7} / 7a9f6e13 | 旧 Tauri 发布链 | 保留原件 |

## 增量验收

1. **重排 schema 后编辑写错字段**：原 Directory 已全选，换序后输入把 Endpoint 改成 `/workspace/searchcalhost` → 每个 configField 以字段 key 标识 → 深浅实际输入恰为 Directory=`/workspace/search`，Endpoint 保持 `https://localhost`。原始红串 `reorder wrote into another field: map[directory:/workspace endpoint:/workspace/searchcalhost]`。
2. **切换中文后错误仍是英文**：无效数字 `1e` 后换语言，仍画 `a number is expected` → 三种本地校验错误按当前语言显示，宿主错误原样保留 → 深浅均画「这里要填数字」，随后键入 12 清除错误并更新值。原始红串 `field error kept previous language`。

第一批相关回归 **43 PASS / 0 FAIL**，internal/ci 通过。V/B/S **0/0/0**，旧断言零改。两场景深浅 before/after 已导出；浅色错误图已目视：中文错误紧贴原字段，未添加第二层面。

3. **换集成后 Undo 恢复上一集成路径**：编辑 search 后切到 notes，撤销得到 `/workspace/search` → 表单以现有 draft 对象区分控件历史 → notes 保持 `/workspace/notes`，自己的后续编辑仍可撤销。原始红串 `integration inherited another undo history: /workspace/search`。
4. **schema 新字段显示空值**：已有草稿刷新 schema，新增结果上限与服务地址没有读取默认 25 与已保存地址 → 仅初始化缺失 key，并同步该字段 baseline → 显示 25 与 `https://localhost:8080`，旧路径编辑保留；键入无效 `1e` 后不会被默认值覆盖。原始红串 `new schema fields lost their initial values: map[directory:/workspace/edited]`。

第二批相关回归 **42 PASS / 0 FAIL**，原始日志 `/tmp/floter-P13-hd/batch2-after.log`。V/B/S 0/0/0，旧断言零改。候选「关闭后隔帧重开继承撤销」原实现通过，未计入八项。首批 ae79c5f3 已推送。

5. **插件字段换序丢失未提交文本**：目录键入 `/workspace/edited` 后换序，Enter 仍留下 `/workspace/search` → 字段使用自己的 schema key → 深浅均将原编辑提交到 directory，endpoint 不变。原始红串 `plugin reorder lost or misdirected edit: map[directory:/workspace/search enabled:true endpoint:https://localhost]`。
6. **插件 Undo 带回另一插件的路径**：从搜索切便条后撤销恢复搜索路径 → 字段身份同时包含插件 ID → 便条保持自身值，便条自己的后续编辑仍能撤销。原始红串 `plugin inherited another undo history: /workspace/search`。

第三批 **94 PASS / 0 FAIL**，两项深浅 before/after 已导出，旧断言零改。第二批 6f717a67 已推送。

7. **字段成为首行后丢掉编辑**：删掉目录前的两个字段，目录成为首行，Enter 不再提交、焦点消失 → stackedRow 主体使用固定身份，分隔线有无不影响主体 → 深浅都保留 `/workspace/edited` 与原焦点。原始红串 `first row lost its active edit: /workspace/search, focused=false`。
8. **增减分组标题后丢掉编辑**：原无标题分组增加「搜索数据」，目录输入丢失；随后移除标题仍丢输入 → 分组卡片使用固定身份 → 增标题提交 `/workspace/edited`，移除标题后提交 `/workspace/without-heading`，焦点持续。原始红串 `section heading lost its active edit: /workspace/search, focused=false`、`removing section heading lost its edit: /workspace/search`。

第四批插件、配置身份、本地表单与堆叠行相关回归 **149 PASS / 0 FAIL**，原始日志 `/tmp/floter-P13-hd/batch4-after.log`。V/B/S 0/0/0，八项均为真实行为修复，跨集成配置、插件浮层及堆叠字段组件；旧测试全部未改。第三批 62efd2a4 已推送，第四批 bb36af62 也已推送。

## 正式 mutation

1. M1 撤销两种表单的字段身份：`reorder wrote into another field: map[directory:/workspace endpoint:/workspace/searchcalhost]`、`plugin reorder lost or misdirected edit: map[directory:/workspace/search enabled:true endpoint:https://localhost]`；深浅均红，退出 1。
2. M2 撤销配置 draft 的控件身份：`integration inherited another undo history: /workspace/search`；深浅均红，退出 1。
3. M3 撤销堆叠行主体与分组卡身份：`first row lost its active edit: /workspace/search, focused=false`、`section heading lost its active edit: /workspace/search, focused=false`、`removing section heading lost its edit: /workspace/search`；深浅均红，退出 1。

每次都还原后执行 md5sum -c，三文件均 OK，再 touch 并复跑。最终新增八测试 × 两外观加父测试计 **24 PASS / 0 FAIL**。恢复指纹：config.go `c3ebb293aaa09b551a4f43f71a857c9e`；pluginsheet.go `141fb0058324c2207ee28393dc1bb190`；rows.go `9712c9309c5c7c3eaa7099153e943c86`。原始日志 `/tmp/floter-P13-hd/mutation{1,2,3}.log`、`restored.log`。

## 深浅图与两轮审稿

ui.Tester 原图为 720×320（集成配置）或 720×400（插件浮层）；32 张原图、16 张 before/after 对比和 2 张 4× 局部图，共 **50 张 / 793007 字节**，归档 `P13-hd-images/`。全部 16 张对比已目视；4× 复核错误字形与字段边连续。这里只使用原生软件渲染和真实键盘事件，没有浏览器/HTML 样机，未做三平台桌面手测。

| 场景 | 浅 / 深对比 | 图上看到的一处变化 |
|---|---|---|
| 配置换序 | [浅](P13-hd-images/config-reorder-light-compare.png) / [深](P13-hd-images/config-reorder-dark-compare.png) | 焦点从错的 Endpoint 回到 Directory，服务地址恢复完整 |
| 错误语言 | [浅](P13-hd-images/config-error-language-light-compare.png) / [深](P13-hd-images/config-error-language-dark-compare.png) | 字段下方英文错误变为「这里要填数字」，原行位置不动 |
| 集成撤销 | [浅](P13-hd-images/config-integration-history-light-compare.png) / [深](P13-hd-images/config-integration-history-dark-compare.png) | 路径保留 notes，状态恢复 Saved，没有伪造未保存改动 |
| 新增字段 | [浅](P13-hd-images/config-added-fields-light-compare.png) / [深](P13-hd-images/config-added-fields-dark-compare.png) | 灰色占位符变为真实的 25 与已保存服务地址 |
| 插件换序 | [浅](P13-hd-images/plugin-reorder-light-compare.png) / [深](P13-hd-images/plugin-reorder-dark-compare.png) | 插入点随目录上移，edited 留在正确的输入槽 |
| 插件撤销 | [浅](P13-hd-images/plugin-history-light-compare.png) / [深](P13-hd-images/plugin-history-dark-compare.png) | 便条配置保持 notes，未重新出现搜索路径 |
| 字段变首行 | [浅](P13-hd-images/plugin-first-row-light-compare.png) / [深](P13-hd-images/plugin-first-row-dark-compare.png) | 首行没有多余分隔线，插入点和 edited 文本仍在 |
| 分组标题 | [浅](P13-hd-images/plugin-heading-light-compare.png) / [深](P13-hd-images/plugin-heading-dark-compare.png) | 「搜索数据」标题出现后，目录仍显示 edited 与插入点 |

四视角审稿：艺术视角看到多余「未保存」警报来自串历史，修第 3 项；工程视角看到首行/标题变化的身份断裂，第二轮补第 7–8 项；任务视角发现新字段空白使用户无法核对配置，修第 4 项；读者视角看到中文界面遗留英文错误，修第 2 项。两轮都保留原生布局与材质，没有用装饰掩盖错误。

## 偷了什么

从 Apple DESIGN.md 取三条具体 DNA：单一交互强调色（供体 #0071e3 → 本仓 Focus/Accent）、近白/近黑中性面（供体 #f5f5f7/#1d1d1f → 本仓 #F7F7F8/#0E0E10）、胶囊圆角（供体 980px → 已有 999px）。这些是沿用的项目映射，未替换本仓 token。

- craft-loop 工艺密度菜单 **第 3 项「品牌 focus 环」**：1 个当前输入拥有焦点；字段现有 1px 焦点边、键盘环 2px/外扩 3px 的几何保留，换序和标题增减后不跳到邻居。
- **第 4 项「签名交互」**：输入/校验在 1 帧内反馈，新增动画 0ms；撤销只操作当前对象的历史，Enter 提交次数保持 1 次。
- **第 6 项「编辑细节」**：首行分隔线数量 0，其余行保持 1px hairline；分隔线消失后字段主体身份不变，不再以丢焦制造空间跳变。
- **第 9 项「自定义滚动条」**：沿用 1 个 PluginScroll，与浮层同宽的 720px 验收帧；本轮没有新增滚动层。此项为审稿复核，不计新修复。
- **第 12 项「材质细节」**：保留 E1 的 1px 受光边、外投影 0 层，E2 阴影仍由既有实现承担；控件身份修复不增加表面。此项为审稿复核，不计新修复。

为什么更轻、更有空间感：焦点和错误稳定占据所属字段，额外警报消失，新字段能读到真实内容；无需新面、额外阴影或动画补偿状态丢失。

## preflight（逐项）

| 项 | 结论 |
|---|---|
| A1 色彩套路 | 沿用近黑/近白，无新增渐变、霓虹、纯黑面或大标题效果 |
| A2 字体 | 原生中文系统字体保留；skill「不能系统默认」与中文规范及本仓合约冲突，按后两者执行 |
| A3 斜体 | 无新增斜体 |
| A4 Hero | 原生配置组件无 Hero，不适用 |
| A5 卡片模板 | 既有分组卡，未新增容器或三列卡片 |
| A6 假数据 | 截图数据为本地目录、实际形状的服务地址与参数值；未注入产品数据 |
| A7 文案 | 复用 en/zh 校验文案，无营销词或新增感叹号 |
| A8 眉批 | 无新增编号眉批 |
| A9 标题行数 | 截图页名/组名均一行；不增加 H1 |
| A10 网格 | 字段仍占既有两列，无新增空洞 |
| A11 侧边装饰 | 无新增左侧竖条或强调线 |
| B1 读取/拨盘 | 2/2/6：保留紧凑配置密度，改对象状态归属 |
| B2 主题锁定 | 16 张对比逐张检查；每帧单一明暗主题 |
| B3 记忆点 | 操作中的字段始终还是同一个字段 |
| B4 动效气质 | 输入即时，表面不添加表演性过渡 |
| C1 目的/频率 | 高频键盘处理 0ms，真实输入回归通过 |
| C2 动效禁令 | 未新增 transition-all、ease-in、scale(0) 或纯 fade |
| C3 时长/按压 | 沿用 ColorStep=120ms，小于 300ms；按钮现有按压保持 |
| C4 浮层原点 | 本轮未改 popover/modal 的构建或原点 |
| C5 减少动态 | 截图夹具 ReduceMotion=true，输入与状态完整 |
| C6 hover 设备 | 原生鼠标状态处理，无 CSS 媒体查询；没有新增 hover 路径 |
| C7 动画属性 | 未新增几何动画 |
| C8 快速触发 | 不新增 keyframes，键盘状态直接更新 |
| C9 手势 | 未改手势/拖拽/sheet 跟手，不适用 |
| C10 Motion 主线程 | 原生 Go，不使用 Framer/Motion，不适用 |
| C11 逐帧 | ui.Tester 按事件逐帧验证焦点/提交，无新增时间插值需要慢放 |
| D2-1 中文字体 | 系统中文字体，无外部字体资产 |
| D2-2 行高/字号/字重 | 现有 Body 10.8px、Caption 9.9px 和字重不满足 skill ≥14px/400–600；沿用用户拍板项，本轮不改高度链，不宣称此项通过 |
| D2-3 混排空格 | 产品文案不改，路径/命令保留用户原文 |
| D2-4 标点/对比 | 现有中文标点保持；次要墨并非全部 7:1，沿用已有限制，未擅自重定主题 |

## 最终门槛与仍待

中文 locale、GOMAXPROCS=4：普通与 verbose 全量均退出 0，**3648 PASS / 0 FAIL**（基线 3624，增加 24）。gofmt 输出 **0 字节**；Linux/Windows/Darwin vet 与 build 全部退出 0。测试函数 **2727 → 2735**，旧测试文件零修改、无新增 Skip。V/B/S **0/0/0**。

原始门槛日志：`/tmp/floter-P13-hd/final-test.log`、`final-verbose.log`、`gofmt.log`、`vet-{linux,windows,darwin}.log`、`build-{linux,windows,darwin}.log`。三条 mutation 全红、三文件 md5 一致，touch 后新增 24 场景全绿。

- 八项已完成，四批代码提交 ae79c5f3 / 6f717a67 / 62efd2a4 / bb36af62 已推送；最后提交收录合约、进度、报告与截图。
- 下一轮先查配置下拉/多选在选项变化后的焦点、选中值和范围锚点归属；尚未复现，不预报为缺陷。
- 保存期间草稿归属、字号/行盒、终端 S1b/S1c/S2 尾沿用已有拍板项，本轮不改其语义。
- 八条早期独立 WIP 原件保留，逐条清点在本报告。旧 stale-kill 已由前轮归档清理，无新抢救稿。
