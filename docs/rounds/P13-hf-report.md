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
