# P13-hr：配置刷新后的非文本控件

起点 d3b1961b，工作树干净，pull --ff-only 已最新。禁用墨已在 P13-hl/hm 结案，本轮沿最新待办检查选择控件；不是重复执行过期任务书行号。

## 抢救 stash 清点

当前八条均非 stale-kill，SHA 与 P13-hq 表一致，reverse-check 均退出 1；此前 stale-kill 原件已归档并接续，不重复合入。保留原件，不以 reverse-check 失败推断可覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 早期 keycap WIP，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 早期 rows/slider WIP，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载布局与旧探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧 number-field WIP，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段 WIP，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 WIP，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏 WIP，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri 实现，reverse=1 | 保留 |

## 实施记录

规格八项均先跑真实事件；门槛、变异、图与每批 SHA 随完成增量记录。V/B/S 暂为 0/0/0，旧断言不改。

### 第一批：下拉与开关

1. 现象：下拉跨组后菜单消失，待选 apps 丢失，Enter 后仍是 files。改动：choose 返回原 trigger，插件将其绑定到已有字段焦点句柄。验证：真实 Click→Down→移组→Enter，深浅均选 apps、保留焦点；原实现报 `group move closed the active select menu` / `group move lost pending choice: files, want apps`。图上可见移组后的“应用”仍高亮，菜单位于新触发器下方。
2. 现象：开关跨组后焦点消失，Space 不再切换。改动：checkbox 返回原开关，绑定同一分组焦点路径。验证：点击→移组→Space 后 true 回 false，深浅均保焦点；原实现报 `group move lost toggle keyboard: value=true focus=false`。

相关回归 **146 PASS / 0 FAIL**，settingsui vet 净。M1 回退 pluginsheet.go 两条绑定，新增两函数六个 PASS 项变红，恢复后 MD5 OK、touch 后 **6 PASS**。日志和深浅图在 P13-hr/。旧 V/B/S 断言修改数 0/0/0。
