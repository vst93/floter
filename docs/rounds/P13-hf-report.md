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
