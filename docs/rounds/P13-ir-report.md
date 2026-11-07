# P13-ir 报告：展开选择器刷新后的选项身份

开盘：2026-11-07 UTC。起点 `7c54e11d`，工作树净；中文 locale 基线为 5001 PASS / 0 FAIL，测试函数 3093。八条早期 WIP stash 均已登记，reverse-check 失败且保留。

## 进展

- 规格已写入 `P13-ir-spec.md`。读取为已有原生设置页，面向键盘使用者，延续轻便液态玻璃；稳定程度 2/10、动效 1/10、密度 6/10。先守“看见哪项就选中哪项”的任务契约。
- 首次批量替换未先复现，且 palette/residency 初值仍传 label，已全部撤回；一次包测试漏中文 locale 引发字体度量误报，未改旧断言。此过程不计完成项，原始 diff 在 `/tmp/floter-P13-ir/first-attempt.diff`。后续均按中文 locale 真实事件见红再修。

## Stash 清点

八条 SHA 与 backlog/前轮一致，无新增 stale-kill；逐条 reverse-check 均退出 1，沿既有语义审查保留旧 WIP，不恢复旧主题或 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 第一批

1. 现象：动作菜单展开并高亮浏览器后，宿主改为计算器，Enter 又写回浏览器。改动：复用现有 valueSelect 的外部值同步。验证：深浅真实 Tab/Down/Enter，刷新写入 0 次、Enter 保持计算器并保留焦点。
2. 现象：展开动作菜单重排后，Enter 按旧索引选中剪贴板。改动：高亮随值重排。验证：深浅真实事件，Enter 只写浏览器一次，触发器焦点仍在。

相关 **64 PASS / 0 FAIL**，包 vet 净；旧 V/B/S **0/0/0**。M1 恢复原 custom.go，**6 FAIL**，原串 `custom external selected "plugin:browser", want "plugin:calculator"`、`custom reorder selected "plugin:clipboard", want "plugin:browser"`；MD5 还原 touch 后 **6 PASS**。

第一批 `44dbb0ef` 已推送。

## 第二批

3. 现象：调色板菜单展开后，宿主从 inherit 改为 dracula，Enter 又写回 contrast。改动：复用当前值刷新高亮。验证：深浅真实 Down/Enter，外部值保持 dracula、没有多余回调、焦点留在触发器。
4. 现象：调色板选项重排后，Enter 落回 inherit。改动：保存选项值身份。验证：深浅真实事件，重排后只提交 contrast 一次，预览依旧跟随被选中的 ID。

其余四项已见红但测试待对应批次再启用；不提交未修复的红测试。旧 V/B/S **0/0/0**。

第二批 `d7989fcf` 已推送。相关 **19 PASS / 0 FAIL**，包 vet 净；M2 **6 FAIL**，原串 `palette external selected "contrast", want "dracula"`、`palette reorder selected "inherit", want "contrast"`；MD5 还原 touch 后 **6 PASS**。

## 第三批

5. 现象：两个动作标签相同时，按 Down 选择第二项会被去重成下一动作。改动：选择器值改为动作 ID，标签仅负责显示。验证：深浅真实键盘选择第二个同名项，只提交 plugin:browser 一次。
6. 现象：两个调色板同名时，第二项无法独立选中。改动：传入 palette ID，初值也改为 ID；预览继续取实际 ID。验证：深浅真实键盘选择第二个同名项，只提交 contrast 一次。

相关 **44 PASS / 0 FAIL**，包 vet 净；旧 V/B/S **0/0/0**。M3 回注按 label 选择的实现，**6 FAIL**，原串 `custom duplicate selected "plugin:calculator", want "plugin:browser"`、`palette duplicate selected "dracula", want "contrast"`；MD5 两文件还原 touch 后 **6 PASS**。
