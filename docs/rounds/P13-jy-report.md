# P13-jy · 本地工具连接恢复

2026-11-08开盘，首个时钟04:04:15 UTC。HEAD6325a179，工作区干净，pull已最新。P13-jx已做完registry失败恢复，按其仍待继续本地工具连接。指定done.log不存在。规格见同目录P13-jy-spec.md。

## stash清点

本轮逐条实际reverse-check均1，完整SHA与交接相同。八条均是早期独立WIP，沿用交接审计保留，不用旧主题/旧Tauri链覆盖当前Go实现；无新增抢救稿、无drop。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap/主题 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/滑块 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42 | 旧字段/确认栏 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏 | 保留 |
| stash@{7} | 7a9f6e13 | 旧Tauri发布链 | 保留 |

## 增量记录

首次基线LANG被LC_ALL=C.UTF-8覆盖，出现已记录的字体pin失败，已终止并用中文LC_ALL重跑；不修改旧断言。后续只记录完整退出的实测结果。

### 第一批：准备失败与权限取消

1. **准备失败**：现象→宿主拒绝不存在的程序后仅通知，表单已清空；改动→同步提交捕获本表单的恢复出口，失败在UI线程恢复原草稿；验证→真实磁盘失败调用恢复1次，ui.Tester程序/脚本两种表单逐字段保留（含参数、声明、平台、隐藏模式草稿和原始空白），再次提交请求严格相等。
2. **权限取消**：现象→拒绝权限后进入Commit并报“连接失败”，表单不可重试；改动→拒绝直接返回并恢复草稿；验证→真实PrepareCustom和拒绝回调，恢复1次、通知0条、仓库条目0；新输入、主动清空、折叠与另一个编辑器四种状态不被旧结果替换。

定向与既有回调契约共35 PASS/0 FAIL/exit0，定向vet净。M1撤回两条宿主恢复及表单出口，实红`failed connection lost complete draft: open=false name="" program="" script="" params=[]`、`prepare recovery=0 settled=1, want one each`、`cancel recovery=0 settled=1, want one each`；两文件MD5逐字节恢复，touch后35 PASS/exit0。旧V/B/S断言修改0/0/0、新增Skip0。夹具初次窗口1100px不足以点击1319.7px处提交，后改用既有1500px测试画布；另一初次把参数渲染前未初始化的key用于相等比较，已在首帧后取样。均不是产品mutation证据，未放宽产品断言。
