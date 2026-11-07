# P13-ij · 复制与导出能力变化后的键盘归处

2026-11-07 01:04 UTC 左右开盘，pull 为最新，工作区干净。实际起点07cba797（P13-ii），按仓库最新仍待接续。基线全量实测4665 PASS / 0 FAIL，退出0；原始日志已落盘。独立由Codex执行，无模型切换或子代理。

## Stash 清点

原八条早期WIP与上轮SHA一致，没有新的stale-kill。逐条实际reverse-check均为1；旧主题/Tauri工作已有语义核对记录，本轮不误当新工作合入，原稿完整保留。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

## 复现与范围

已有按钮由实际点击取得焦点，再用Tab往返使环可见；宿主Actions撤回后检查对应编辑内容。关于页从侧栏用Tab定位真实链接，撤回时检查原生selectAll/copy，恢复时检查Tab/Shift-Tab方向和复制出的实际URL。原版七个候选实红，首链接恢复的正向路径原已正确，补测反向出口，未把原本正确路径计修复。新测试最初写错ui.ModShift导致编译错误，已改为框架实际ui.Shift，错误不算产品缺陷。

为什么更轻、更有空间感：让唯一焦点环留在仍可用的内容上，操作退场后不留下空的键盘层；不增加卡片、通知、阴影或材质层。

## 第一批（1–2）

1. 现象：实际复制命令后撤回CopyText，禁用药丸失焦而输入框未接回。改动：按当前App/表单/动作保存来源句柄，只在原按钮持有焦点时回命令框。验证：深浅点击→Tab往返→撤回，命令框持焦点；Enter不复制，恢复能力不抢焦点，明确再点只多一次复制。
2. 现象：复制执行计划后撤回能力，键盘同样无编辑归处。改动：计划动作独立绑定程序框。验证：两主题回程序而非命令框，恢复后计划复制次数恰好+1，导出次数不变。

当批相关41 PASS / 0 FAIL，包vet净。M1撤回当批settingsui.go补丁实红6条FAIL记录（4个深浅叶子），原串 `command withdrawal lost source-field focus: want 命令`、`plan withdrawal lost source-field focus: want 程序`；MD5还原一致，touch后6 PASS / 0 FAIL。V/B/S旧断言0/0/0，无新增Skip。浅色after目视：命令槽有明确焦点环，灰色复制药丸保留原位置，未增加外框或浮层。

补测首链接恢复后的Shift-Tab确实不能回侧栏，已保留原红日志。测试函数数按全仓tracked `*.go` 实测3011；之前cmd/internal口径3009少算根目录2条，后续统一全仓口径。
