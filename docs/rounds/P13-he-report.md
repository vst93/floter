# P13-he · 配置选项的身份与可读反馈

起点 `674724d0`，2026-11-05；Codex 独立执行，pull 已同步，开盘工作树空。最新前轮为 P13-hd，done.log 开盘不存在。测试函数开盘实测 2735；全量中文 locale 基线 **3648 PASS / 0 FAIL**。初次误用 C locale 的运行终止，未将环境差异视为产品问题。

## stash 清点

活动 stash 八条逐一 reverse-check 均失败；已核对各条 stat、P13-hd 清点与 stash-backlog。它们均为早期独立 WIP，不是任务书中的 stale-kill。原抢救稿已在历史轮次归档清理；不重放旧主题、键盘合约或 Tauri 发布链。没有新 orphan。

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | P7 keycap 与旧主题，3 文件 | 保留原件 |
| stash@{1} / 8af94a07 | 设置行距/滑块旧刻度，3 文件 | 保留原件 |
| stash@{2} / 7e4aa710 | 卸载旧布局与 scratch 探针，2 文件 | 保留原件 |
| stash@{3} / 1e3aec94 | 旧数字框与透镜稿，2 文件 | 保留原件 |
| stash@{4} / 363f5a42 | 旧确认带/快捷键稿，2 文件 | 保留原件 |
| stash@{5} / 9b8ce23c | P7 快捷键拒绝反馈稿，9 文件 | 保留原件 |
| stash@{6} / 90d314b2 | action bar 旧键盘合约，3 文件 | 保留原件 |
| stash@{7} / 7a9f6e13 | 旧 Tauri 发布链，1 文件 | 保留原件 |

## 增量验收

1. **集成多选换序后焦点跑到邻项**：文件与应用交换后焦点变为应用 → 每行以选项文本为身份 → 深浅实际焦点仍在文件。原始红串 `multiselect focus moved to another option after reorder`。
2. **插件多选换序后空格改错选项**：文件已选，交换顺序后按空格得到 `[apps files]` → Checkbox 以 option.Value 为身份 → 深浅都取消文件，值为 `[]`。原始红串 `plugin checkbox keyboard changed another option: [apps files]`。

键盘 Space 单独候选原实现已通过，保留为守卫但不计修复。第一批相关回归与 internal/ci 通过后提交；旧断言未改，V/B/S 0/0/0。所有原实现红串保存在 `/tmp/floter-P13-he/probe.log`。

第一批 **24 PASS / 0 FAIL**，internal/ci 通过，`e18a191b` 已推送。

3. **换序后 Shift 选中无关项**：以文件为锚，换序再 Shift 点书签得到 `[应用 文件 书签]` → 锚点保存选项值，使用时按当前顺序定位 → 深浅都只选 `[文件 书签]`。原始红串 `reordered range selected the wrong options: [应用 文件 书签]`。
4. **删除锚点后邻项继承范围起点**：删除文件后 Shift 点书签，应用也被选上 → 找不到原值即失效并清除锚点 → 深浅只选书签，下一次 Shift 从新的书签锚点正常扩展。原始红串 `removed anchor selected an unrequested replacement: [应用 书签]`。

第二批相关回归 **30 PASS / 0 FAIL**。一项守换序的延续，一项守删除后的失效与恢复；同一批一次修完，旧 B 断言零改。

第二批 `36fa4dee` 已推送。第一轮目视：浅色范围图里应用的误勾消失；深色插件图里焦点环跟随文件，两个框不再都被选中。工程视角选项身份已修；任务视角仍需修下拉伪报当前值；读者视角仍需补空标签；艺术视角检查保持单一 E1 面，不用额外警告框填空白。

5. **未知保存值伪装成第一项**：保存的是 bookmarks，触发器却显示文件 → choose 只在 ID 匹配时取标签，缺失时显示原值 → 深浅均显示 bookmarks，点击文件后写回 files。原始红串 `unknown stored option is displayed as the first choice: ["搜索配置" "搜索方式" "搜索方式" "文件"]`。
6. **插件下拉的空标签成为空白项**：有 Value=files、Label 为空时，触发器与菜单没有名字 → 显示回落到 Value → 深浅可见 files，应用→files 两次选择均写回对应 ID。原始红串 `select option with empty label has no visible name`。

第三批相关回归 **35 PASS / 0 FAIL**，`12638064` 已推送。前后图已目视：浅色触发器从错误的「文件」变为 bookmarks；深色空白触发器出现 files，胶囊、箭头与原材质位置保持。

7. **插件单选分段留下无名选择**：空标签画出能选中却没有名称的分段 → Radio 标签共用 Value 回退 → 深浅可见 files，应用→files 切换分别提交 apps/files。原始红串 `radio option with empty label has no visible name`。
8. **插件复选框只剩无名方框**：空标签仅画方框，不知道它控制什么 → Checkbox 标签共用 Value 回退 → 深浅可见 files，点击清除再点击选中都写回原 ID。原始红串 `checkboxes option with empty label has no visible name`。

第四批八项及旧选择相关回归 **44 PASS / 0 FAIL**，internal/ci 通过；旧断言全部零改，V/B/S 0/0/0。八项跨集成多选、插件多选、下拉与分段控件。单选未知值的旧夹取规则继续留在拍板队列。第四批提交正文先写了 43，此处据完成日志纠正为 44。

第四批 `74cd4306` 已推送，四批均完成 1–2 项即提交，没有攒到轮末。

## 正式 mutation

1. M1 移除两类多选的稳定身份：`multiselect focus moved to another option after reorder`、`reordered checkbox keyboard changed another option: [应用 文件]`、`plugin checkbox keyboard changed another option: [apps files]`。深浅都红，退出 1；恢复后 6 PASS。
2. M2 将范围锚点退回数组索引：`reordered range selected the wrong options: [应用 文件 书签]`、`removed anchor selected an unrequested replacement: [应用 书签]`。深浅都红，退出 1；恢复后 6 PASS。
3. M3 去掉未知值与空标签回退：`unknown stored option is displayed as the first choice: ["搜索配置" "搜索方式" "搜索方式" ""]`、`select option with empty label has no visible name`、`radio option with empty label has no visible name`、`checkboxes option with empty label has no visible name`。深浅四组都红，退出 1；原始输出归档 `P13-he-evidence/mutation3.log`。恢复后本轮全部回归 **27 PASS / 0 FAIL**。

每条都复制原件、注入缺陷、实跑、还原，三文件 `md5sum -c` 均 OK，再 touch 并 `-count=1` 复跑。指纹：config.go `0c4c41065324ba6eae999d1859d0c6e7`；pluginsheet.go `66aef56b72909d70f666c51cb2f6dc7f`；settingsui.go `f3ec73e0c2e486e0a4e69bdd89760d85`。

## 深浅对比与目视

ui.Tester 原图：集成配置 720×320，插件浮层 720×400。八项 × 深浅 × 前后共 32 张，另有 16 张并排对比、4 张审稿联系图和 2 张 4× 局部图，共 **54 张 / 729452 字节**。原件在 `P13-he-images/`；所有 16 组都已目视。没有把无头截图称为三平台真机手测。

| 项 | 浅 / 深 | 图上看到的变化 |
|---|---|---|
| 集成多选焦点 | [浅](P13-he-images/multi-focus-light-compare.png) / [深](P13-he-images/multi-focus-dark-compare.png) | 空格后两枚错误勾选消失，焦点环跟着文件 |
| 插件多选焦点 | [浅](P13-he-images/plugin-checkbox-focus-light-compare.png) / [深](P13-he-images/plugin-checkbox-focus-dark-compare.png) | 应用不被误勾，焦点从左框移到右侧文件 |
| 重排范围 | [浅](P13-he-images/multi-range-light-compare.png) / [深](P13-he-images/multi-range-dark-compare.png) | 应用恢复空框，文件与书签保持选中 |
| 删除锚点 | [浅](P13-he-images/removed-anchor-light-compare.png) / [深](P13-he-images/removed-anchor-dark-compare.png) | 原先两项都选中，现只勾书签 |
| 未知下拉值 | [浅](P13-he-images/unknown-select-light-compare.png) / [深](P13-he-images/unknown-select-dark-compare.png) | 触发器从伪报文件变为真实 bookmarks |
| 下拉空标签 | [浅](P13-he-images/unlabelled-select-light-compare.png) / [深](P13-he-images/unlabelled-select-dark-compare.png) | 空白胶囊出现 files，箭头与圆角连续 |
| 单选空标签 | [浅](P13-he-images/unlabelled-radio-light-compare.png) / [深](P13-he-images/unlabelled-radio-dark-compare.png) | 无名亮片出现 files，原分段宽度保持 |
| 复选空标签 | [浅](P13-he-images/unlabelled-checkboxes-light-compare.png) / [深](P13-he-images/unlabelled-checkboxes-dark-compare.png) | 方框后出现 files，下一选项按原间距让位 |

第二轮审稿：焦点细节图显示 2px 环与方框角连续；Radio 局部图显示名称落在原 9u 内缩中，没有增加第二条边。修复前范围图暴露的多余勾选、无名控制和伪报值均已消除。66px 多选视口的第三行仍进入原有软边，这是现有高度合约，不为截图改尺寸。

## 偷了什么

供体 Apple DESIGN.md 的三个具体 DNA：#0071e3 单一交互强调色 → 本仓 Focus/Accent；#f5f5f7/#1d1d1f 中性表面 → 本仓 #F7F7F8/#0E0E10；980px 胶囊 → 现有 999px。沿用映射，不替换项目 token。

- 从 craft-loop 工艺密度菜单 **第 3 项「品牌 focus 环」**取一个当前对象 + 2px 环；落在集成与插件多选，换序后仍属于原值。
- **第 4 项「签名交互」**取即时操作反馈；空格与 Shift 在 1 帧内落值，新增动画 0ms，原按键语义保持。
- **第 6 项「编辑细节」**取 1px hairline；分段/复选/下拉保留原边，名称回退不加说明框。此项是复核，不计新修复。
- **第 9 项「自定义滚动条」**保留 66px 多选视口、单一内部滚动与原软边；选项换序不改变高度。此项是复核，不计新修复。
- **第 12 项「材质细节」**保留 E1 的 1px 内受光边，新增阴影 0 层；不会用叠玻璃强调选项。此项是复核，不计新修复。

为什么更轻、更有空间感：焦点与勾选稳定占据所属选项的位置，空白控制有可辨认内容；恢复操作连续性，避免靠额外提示层补救。

## preflight 逐条

项目事实与用户明确的行为/高度/字号红线优先。新缺陷与复现归档本仓规格、合约和回归测试；未改外部 skill 的全局偏好账本。

| A 反套路项 | 结论 |
|---|---|
| A1 渐变/纯黑/霓虹/渐变字 | 通过；保留近黑近白 token，无新增装饰 |
| A2 字体不应为系统默认 | 按中文专属规则与项目红线保留系统 CJK；不为了换字体破坏量尺 |
| A3 禁斜体 | 通过；没有斜体 |
| A4 Hero 构图 | 不适用；原生配置表单 |
| A5 卡片海/三等分 | 通过；没有增加容器 |
| A6 假数据 | 通过；夹具为明确的文件、应用、书签与真实 ID 形状 |
| A7 空洞文案/感叹号 | 通过；没有新宣传句，标签直接来自 schema |
| A8 眉批 | 通过；没有新增眉批 |
| A9 H1 行数 | 不适用；沿用单行插件标题 |
| A10 Bento 空洞 | 不适用；没有网格改造 |
| A11 左侧装饰竖线 | 通过；选中使用已有勾选与焦点环 |

| B 设计读取 | 结论 |
|---|---|
| B1 方向与拨盘 | 通过；稳定 2 / 即时 2 / 紧凑 6 与规格一致 |
| B2 全页主题 | 通过；每帧一个主题，分别出深浅图 |
| B3 记忆点 | 焦点、范围与当前值都属于实际选项 |
| B4 动效气质 | 通过；没有新增拖延键盘操作的动画 |

| C 动效抽查 | 结论 |
|---|---|
| C1 目的/高频输入 | 通过；高频输入即时反馈 |
| C2 all/ease-in/scale(0) | 通过；没有 CSS 或新增入场 |
| C3 时长/按压 | 保留既有 120ms 色过渡与按压反馈；新增动画 0ms |
| C4 浮层 origin | 保留框架 Select 从触发器展开，测试实际点击开合 |
| C5 reduced motion | 深浅 ui.Tester 都设置 ReduceMotion=true，状态仍完整 |
| C6 hover 媒体查询 | 不适用；原生指针事件，无 CSS |
| C7 动画属性 | 通过；没有新增几何动画 |
| C8 快速触发 | 通过；没有新 keyframes 或重播机制 |
| C9 手势 | 不适用；未改拖拽/手势 |
| C10 Framer | 不适用；Go 自绘 |
| C11 慢放/逐帧 | 本轮为离散状态修复，逐帧键鼠输入和前后截图已覆盖；未声称真机动效复核 |

| D2 中文页 | 结论 |
|---|---|
| 系统字体栈 | 通过；沿用系统 CJK，未下载字体 |
| 行高/14px/字重 | **既有例外未过**：Body 10.8px、Caption 9.9px 与 14px 要求冲突；既有 580/620 字重也不在 400/500/600 内。用户要求保留字号与行高链，仍列拍板，不冒充全过 |
| 中西文/数字空格 | 没有新增混排句；schema 的 Value 按原样显示，不擅自改 ID |
| 标点/暗色 7:1 | 本轮无新增标点；沿用语义墨，未重新承诺所有次要墨都到 7:1，既有对比度限制仍沿用 |

## 最终门槛

统一 `LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4`。普通与 verbose 两次全量均退出 0；最终 **3675 PASS / 0 FAIL**（基线 3648，+27）。测试函数 **2744**（基线 2735，+9；其中 Space 原本正常的守卫不计修复）。gofmt 输出空；Linux/Windows/macOS vet 与 build 六项全部通过，七份日志均为空。文档更新后的 internal/ci 再验也通过。没有新增 Skip、没有删除测试、没有修改旧 V/B/S 断言。

普通全量原始输出归档 `P13-he-evidence/test.log`；汇总在 `P13-he-evidence/gates.txt`。较大的 verbose 原始日志保留 `/tmp/floter-P13-he/test-verbose.log`。代码四批已经推送，最终档案提交包含本报告、规格、§6.181、当前进度、54 张图及 mutation 原始证据。

## 仍待

1. 本轮八项没有遗留尾项。下一轮候选：下拉存在相同显示名或菜单打开时选项集合变化后的提交归属；先复现，不把代码猜测记为缺陷。
2. 保留 176-9、161-4 保存时机、未知 Radio 值夹取、字号/行盒与终端 S1b/S1c/S2 尾拍板项；未擅自改变。
3. 八条早期非 stale-kill stash 保留原件，逐条表见上。
