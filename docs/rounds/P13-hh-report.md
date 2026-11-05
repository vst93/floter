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
