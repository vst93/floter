# P13-hh · 动态选择与可见边界

起点 628bf3c4，工作树干净、pull 同步；done.log 开盘不存在。已读取 qiaomu-design 打磨路径与项目两份视觉合约，按最新 P13-hg 仍待接续。

## stash 清点

以下八条实际 reverse-check 均退出 1，stat 与前轮记录相同。原任务书 stale-kill 已由 P13-go/gq 等轮归档；本轮无新 orphan。这些是早期独立 WIP，不能把 reverse 失败当成未接续功能，也不能用旧主题覆盖当前合约。

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

第一批相关回归 **71 PASS / 0 FAIL**；旧断言零改，V/B/S 0/0/0。开盘全量 **3737 PASS / 0 FAIL**，实际 tracked `func Test` **2762**（前轮报告 2764，当前重新计数，不照抄）。原始反证与深浅前后图已生成。

第一批 8375f206 已推送。

3. **选项短暂移除又回来后，Shift 复活旧范围、多选应用** → 每帧校验并退役消失锚点 → 结果为文件+书签；反证 `reappeared option resurrected its old range: [文件 应用 书签]`。
4. **插件复选长名称直抵卡片右边界、末字被切** → 原 Checkbox 保留，标签单行省略、控件最大宽为父容器 100% → 深浅标签右端落在原内边距内，下一项仍可点；原标签右端 704.6，图上文字贴边，修后留白与省略号可见。初始探针的 700 是估测，正式 pin 采用卡内边界 692；没有放宽旧断言。

第二批相关回归 **74 PASS / 0 FAIL**，V/B/S 0/0/0。目视浅色长复选名称已从硬裁切变为省略，勾选框和应用下一行保持。
