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
