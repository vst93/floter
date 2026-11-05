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

第四批八项及旧选择相关回归 **43 PASS / 0 FAIL**，internal/ci 通过；旧断言全部零改，V/B/S 0/0/0。八项跨集成多选、插件多选、下拉与分段控件。单选未知值的旧夹取规则继续留在拍板队列。
