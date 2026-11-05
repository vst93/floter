# P13-gz · 设置控件的可用状态与动作归属

起点 `b5073966`，分支 `mygo-rewrite`。本报告随每批修复增量更新。

开工树干净，pull --ff-only 已是最新。done.log 与 stash-backlog 在当前环境不存在；收尾创建 done.log。
禁用胶囊重测：浅色 face/crown/shade 238/238/231；深色 26/26/23，与既有契约一致，未重复计修复。

## stash 清点

任务书的 stale-kill 已由 P13-gq / P13-gy 归档接续。实际剩余八条均非 stale-kill，reverse-check 全部失败；没有静默删除或覆盖现有设计。

| 当前 ref / SHA | 判断 | 处置 |
|---|---|---|
| 0 / 422483b4 | P7 keycap / 旧主题 WIP，非当前设计 | 保留原件 |
| 1 / 8af94a07 | P7 行距 / 滚动边 WIP，后续刻度已有正式 pin | 保留原件 |
| 2 / 7e4aa710 | 卸载布局与 scratch 草稿 | 保留原件 |
| 3 / 1e3aec94 | 旧数字框 / 透镜草稿 | 保留原件 |
| 4 / 363f5a42 | P7 确认带与快捷键草稿 | 保留原件 |
| 5 / 9b8ce23c | P7 快捷键拒绝反馈草稿 | 保留原件 |
| 6 / 90d314b2 | 旧启动器 action bar 草稿，后续键盘合约已定 | 保留原件 |
| 7 / 7a9f6e13 | Tauri release workflow，当前 Go 分支无该路径 | 保留原件 |

独有工作的判断沿用 P13-gq 审核表，没有把 reverse-check 失败误判为可以合入。原件均仍可恢复。

## 已完成（逐条）

1. **恢复状态点错位**：原实现两套主题各有 20 个点像素落在恢复按钮之外 → 移到按钮 Children 中 → 点像素全部归于恢复按钮，按钮布局不变，恢复/删除接线保留。before 原始失败：`resume marker changed 20 pixels, 20 outside its button (20,20)-(45,45)`。
2. **只读会话标题误反馈**：active / exited / 缺恢复回调时标题不能恢复，却把浅色面 229 抬到 205、深色 39 抬到 69 → 用现有 rowFace.Inert 表达不可恢复 → 六组无动作状态保持静息，可恢复的两组仍有 hover 且单击只调用一次。原始失败：`read-only title feedback: available=false rest={229 229 230 ...} hover={205 205 206 ...}`。

首批台账 V 0 / B 0 / S 0（未改旧断言）。截图由新回归用 ui.Tester 生成深浅 before/after。
