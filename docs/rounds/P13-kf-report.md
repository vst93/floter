# P13-kf · 运行结果的阅读与退出恢复

开盘352f8cc4，pull已同步且工作树干净。Codex独立执行，未委派、未换模型。给定done.log开盘不存在。最新P13-ke仍待指向输出阅读；jw安装恢复已经后续轮完成，不重复。

## Stash清点

权威活动清单8条，无新增stale-kill。交接账登记为早期独立WIP；各项反向校验exit1，并非当前待接续抢救稿。保留原件，不将旧主题与Tauri发布链覆盖HEAD。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap/主题，reverse1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri发布链，reverse1 | 保留 |

## 增量实施

基线与事件复现进行中，只按实测补录。旧V/B/S断言目标0/0/0。

1. 展开后再次Enter无效（旧槽位复用造成看似持焦）→ 折叠入口显式身份，展开交接到原收起键 → 深浅可见断言与两次Enter往返通过。
2. 收起后查看入口失焦 → 收起键持焦时返还原查看入口并一次显现 → 深浅入口完整可见，Enter可立即重开。

第一批定向6 PASS / 0 FAIL，settingsui vet净、gofmt空。M1撤回本批产品实红：`disclosure cannot reverse with keyboard`与`run outlet outside viewport: label="查看输出" focused=false`；MD5还原、touch复验6 PASS。V/B/S旧断言0/0/0。完整开盘门槛尚在运行，不填写未完成的全量数字。
