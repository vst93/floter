# P13-hg · 选择控件边界打磨

开盘 48e06621，干净树，pull 同步。done.log 不存在，将在收尾新建。前轮 P13-hf 已完成禁用态和下拉身份修复，本轮接其动态状态候选，采用 qiaomu-design 打磨路径。

## stash 清点

八条活动 stash 均实跑 reverse-check=1，stat 与 P13-hf 及 stash-backlog 一致。任务书的 stale-kill 原件已在前轮归档清理；当前是早期 WIP，不重放已被新设计替代的主题/行为。

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

1. **打开菜单期间值从文件刷新为便条，回车又写回文件** → 记录上帧实际值并同步高亮 → 深浅 Enter 均保留 notes；原串 `Enter overwrote the externally refreshed value: got files, want notes`。
2. **点击多选标签后空格不作用于刚点项** → 标签点击将焦点移交该行勾选框 → 深浅先点文件、再点应用、空格后仅剩文件；原串 `label click did not focus its checkbox`、`Space after label click changed the wrong option: [文件 应用]`。

第一批相关回归 63 PASS / 0 FAIL。长菜单宽度原实现已通过，只保留守卫，不计修复。开盘全量实测 3702 PASS / 0 FAIL，2755 个测试函数。原有断言未改，V/B/S=0/0/0。目视浅色菜单高亮从文件移到便条；深色多选的面和间距保持原刻度。

第一批 0755619f 已推送。

3. **多选长名字换行挤掉下一项** → 单行省略保持原行高 → 深浅长项高度和下一项间隔断言通过，下一项真实点击可提交；前后图中应用从视口外回到第二行。
4. **重复多选值让表单直接 panic** → 每个真实值保留一个行、一个范围位置 → 重复 schema 能完整打开并仅提交一次文件；原串 `duplicate` 的完整框架堆栈归档 batch2-before.txt。崩溃前无法导出重复项 before 图，明确缺证，不伪造截图。

第二批 78b44f84 已推送，但过程有一次门槛违规：第一批命令异步运行时追加了第二批测试，随后第二批计数 pin 仍红时过早提交。原始日志保留；原因是 Tester.Texts 同时包含可见词与勾选框的辅助功能 Label，应为一对 2 条而非 1 条。修正的是本轮新增断言的观测口径，旧测试未动。重复 schema 原实现仍会 panic，防崩与单次提交断言保持。

5. **插件复选组重复 ID 直接崩溃** → 构建前按 Value 保留首个选项 → 深浅能显示并选择文件，只提交一个 files；原串 `ui: two elements under one parent have the key "plugin.option.files", and share one state`。原实现无法出图，同样记录缺证。
6. **插件单选重复 ID 形成无法独立选择的分段** → 按 Value 保留首项及其名字 → 深浅移除“文件副本”，点击文件稳定保存 files；原串 `duplicate radio ID still draws an unselectable segment`。

第三批相关实测 49 PASS / 0 FAIL，包含第二批全部回归；b4faf46e 已推送。

7. **插件及集成下拉重复 ID 多占一格键盘位置** → 共享下拉按值保留第一名字且不修改调用方列表 → 深浅 Home/Down/Enter 均选择第二个真实值 apps；原串 `duplicate ID consumed a keyboard step: files`、`config duplicate consumed a keyboard step: files`。
8. **空值且空名称的选项没有可见名字** → 插件下拉/单选/复选和集成必填下拉/多选共五处复用本地化 ConfigNone，身份不改 → 五种控件×深浅都可点击“无 / None”且提交原空字符串；原串 `empty select choice has no visible name` 等十处。

第四批相关实测 90 PASS / 0 FAIL。最初新测试将枚举 Kind 当 string，被 vet 拒绝；修为显式测试名，重新以 Go overlay 加载 b4faf46e 三个原产品文件取得真实行为反证。batch4-before.txt 是编译通过后的原行为失败，未把编译错误算产品缺陷。所有既有断言零改。

第一轮四视角复核：艺术视角看到重复选项使同一动作占两块面，已去重；工程视角发现重复键 panic，已覆盖真实构建；任务视角发现外部刷新后 Enter 撤销新值，已同步；读者视角长名字把下一项藏起来，已单行裁切。第二轮目视深浅：菜单少一行、radio 从三格变两格，点击文件后的选中面与焦点环回到同一格；空复选项现在有“无”字，勾选框仍在原位置。

第四批 ce4a031d 已推送，CI 合约通过。四批共八项，覆盖共享下拉、集成多选、插件复选和插件单选四种组件。

## 正式 mutation

| 组 | 注入 | 实跑原始红串 | 还原 |
|---|---|---|---|
| M1 | external 强制 false | `Enter overwrote the externally refreshed value: got files, want notes` | selectvalue.go MD5 OK、touch |
| M2 | 取消 label click 的 Focus | `label click did not focus its checkbox`；`Space after label click changed the wrong option: [文件 应用]` | config.go MD5 OK、touch |

两组测试进程退出 1，均深浅变红。指纹：selectvalue.go=`57af3becfd2c07d92472125a47591fc0`，config.go=`07711b7dd0b598f591046ce4513492f3`。逐字节还原后本轮九个函数（八项修复 + 一个长菜单守卫）**35 PASS / 0 FAIL**，未使用缓存，原始日志见 P13-hg-evidence。

## 图片

ui.Tester 52 张原图、24 张并排图、2 张联系图、4 张 4× 像素放大，共 **82 PNG**，均归档 [P13-hg-images](P13-hg-images)。插件帧 720×400，集成帧 720×320；4× 图是原帧裁切放大，未冒称高 DPR 或真机截图。两个崩溃场景没有 before 图，因为原实现构建失败；after 图、原始 panic 和真实操作测试齐全。

| 场景 | 深浅对比 | 实际看见 |
|---|---|---|
| 外部值刷新 | [浅](P13-hg-images/external-value-menu-light-compare.png) / [深](P13-hg-images/external-value-menu-dark-compare.png) | 蓝色高亮从文件移到便条，与触发器当前值一致 |
| 多选长名字 | [浅](P13-hg-images/multi-long-light-compare.png) / [深](P13-hg-images/multi-long-dark-compare.png) | 原先 56px 多行墨变单行，原下一项 0×0 裁切变可见可点 |
| 重复单选 | [浅](P13-hg-images/radio-duplicate-chosen-light-compare.png) / [深](P13-hg-images/radio-duplicate-chosen-dark-compare.png) | 三段变两段，焦点环与文件选中面同格 |
| 重复下拉 | [浅](P13-hg-images/select-duplicate-light-compare.png) / [深](P13-hg-images/select-duplicate-dark-compare.png) | 文件副本消失，菜单缩短一行 |
| 无名复选 | [浅](P13-hg-images/empty-name-checkboxes-light-compare.png) / [深](P13-hg-images/empty-name-checkboxes-dark-compare.png) | 孤立勾选框旁出现“无”，有明确点击目标 |

4× 局部已目视：深色菜单四角和选择面圆角连续；浅色长项末尾省略号没有压住右侧勾选框。标签焦点的主要证据为键盘事件后的值断言，截图不代替行为验证。

## 偷了什么

已读 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography 与 Apple DESIGN。Apple 的 #0071e3 单一交互色角色映射现有 Accent/Focus；#f5f5f7/#1d1d1f 映射本仓近白/近黑；980px 胶囊沿用现有 999px。项目方向已定，不生成 HTML，不调用外部模型。

- craft-loop 工艺密度菜单 **第 3 项「品牌 focus 环」**：沿用 **2px** 焦点环；标签点击让一个焦点交给一个复选框。
- **第 4 项「签名交互」**：键盘与外部刷新 **1 帧**响应、新增动画 **0ms**。
- **第 6 项「编辑细节」**：多选名字由多行变 **1 行**省略，原 **1px** 发丝边不变，消除同 ID 重复段。
- **第 9 项「自定义滚动条」**：维持 **1 个**列表滚动区与 **66px** 外框；长名字不再吞掉下一行。
- **第 12 项「材质细节」**：沿用 **1px** 顶内受光边、E1 面和 E2 菜单；新增材质层 **0**，去掉重复操作面。

这轮更轻、更有空间感的原因：每个操作只占一块选择面，长名字不吞下一行，高亮与真实值同位；空间用于真实可选内容，没有增加装饰层。现有 token 与高度链继续由 liquid-glass 合约约束。

## preflight 逐条

第 0 步：偏好账本已读；缺陷沉淀在项目 §6.183、九个测试函数与原始证据，没有将一次性修复写成跨项目审美禁令。Codex 独立执行。

| A 反套路 | 结论 |
|---|---|
| A1 渐变/纯黑/霓虹/渐变字 | 通过，无新增，沿用项目近黑近白 |
| A2 非默认字体 | 项目与中文规则优先，保留系统 CJK，明确例外 |
| A3 斜体 | 通过，无斜体 |
| A4 居中 Hero | 不适用，原生设置表单 |
| A5 三等分/卡片海 | 通过，减少重复段，未加容器系统 |
| A6 假数据 | 通过，文件/应用/便条均为可理解的测试选项 |
| A7 空词/感叹号 | 通过，复用已有本地化词条 |
| A8 眉批 | 通过，新增 0 |
| A9 H1 行数 | 不适用，原有单行标题 |
| A10 Bento | 不适用，无网格改造 |
| A11 装饰竖线 | 通过，选中使用原整块面 |

| B 设计读取 | 结论 |
|---|---|
| B1 拨盘方向 | 通过，稳定 2 / 即时 2 / 紧凑 6 |
| B2 主题锁定 | 通过，每帧一个主题，深浅分别验证 |
| B3 记忆点 | 当前值、焦点和选项始终指向实际对象 |
| B4 动效气质 | 通过，选择即时反馈 |

| C 动效抽查 | 结论 |
|---|---|
| C1 目的与频率 | 通过，高频键盘新增 0ms |
| C2 all/ease-in/scale(0)/纯 fade | 通过，无新增动画 |
| C3 时长与按压 | 通过，保留原控件反馈 |
| C4 浮层来源 | 通过，沿用 SelectBase 触发器锚定 |
| C5 reduced motion | 通过，全部新 Tester 用 ReduceMotion=true |
| C6 hover 媒体查询 | 不适用，Go 原生指针 |
| C7 几何动画 | 通过，未新增 |
| C8 快速触发 | 通过，无重播 keyframes |
| C9 手势 | 不适用，未改拖拽/手势 |
| C10 Framer Motion | 不适用，Go 自绘 |
| C11 逐帧证据 | 真实键鼠帧与截图覆盖；未声称三平台真机测试 |

| D2 中文页 | 结论 |
|---|---|
| 字体 | 通过，系统 CJK，无下载装饰字体 |
| 行高/字号/字重 | 既有例外：Body 10.8px、Caption 9.9px 小于 14px，580/620 字重沿用；用户禁止擅改高度链，保留拍板 |
| 混排空格 | 没有新增混排句，真实选项名与 ID 保持原文 |
| 标点/暗色 7:1 | 无新增标点句；沿用语义墨，未冒称所有次要墨达到 7:1 |

## 门槛与仍待

统一 `LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4`，普通与 verbose 两次全量退出 0，最终 **3737 PASS / 0 FAIL**（基线 3702/0，+35）。测试函数 **2764**（基线 2755，+9）。Linux、Windows、Darwin 三平台 vet/build 六项均通过，gofmt 输出空，git diff --check 通过。V/B/S **0/0/0**，旧断言未改，没有新增 Skip 或删测试。原始 baseline、test、test-verbose、各平台 vet/build、gates 与变异日志全部进 P13-hg-evidence。

本轮约 32 分钟，低于任务书目标 45–75 分钟；八项和全部门槛已完成。小步提交过程的提前提交偏差见上文，最终状态已完整复验。没有添加新页面、系统或依赖；没有引入终端会话管理变更。

下一轮首选：动态列表中当前值与高亮同时消失时如何保留未被用户替换的旧值；本轮仅修复更新为有效候选的情况，未宣称未知值组合已处理。沿用字号/行盒、保存期草稿、终端 S1b/S1c/S2 尾拍板项，不改会话语义。
