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

首批定向实测 **31 PASS / 0 FAIL**，提交 `6b1a7678` 已推送。

3. **外部配置漏锁**：同一集成复检中仍能启动外部配置命令，浅色父行 230→206、深色 39→69 → 入口并入 rowBusy 状态 → 忙态点击回调 0 次、父行像素不变；解除忙态后恰好 1 次。旧实现原始失败：`busy tool configuration launched 1 commands`。
4. **禁用动作的父行误抬升**：Run 已禁用，父行还把按钮旁的浅色 221→198、深色 50→78 → 动作行忙态使用 Inert → 同一取样区域 rest==hover，保留子控件原有禁用淡化与按钮接线。原始失败：`busy action cluster lifted under disabled Run: {221 221 222 ...} -> {198 198 199 ...}`。

第二批定向 **8 PASS / 0 FAIL**，提交 `bfd437fa` 已推送。

5. **卸载动作按压缺面反馈**：静止取样处 hover/press 完全一样（浅 114/114、深 146/146）→ 按下改取 ControlPress，沿用 0.97 缩放与 120ms → 中性/危险 × 深浅四组按下均向地面回落，忙态仍保持禁用。
6. **危险动作图标掉语义色**：确认文字用 Warning，前导确认图标却是默认文字墨（图标区暖色像素 0）→ 图标与标签同取 ink → 深浅都能测到 Warning 图标，禁用仍只淡一次。

开工全量基线已完成：**3530 PASS / 0 FAIL / 2697 测试函数**。最早一遍误用 C locale 的测试已停止，不拿它计基线；中文 locale 完整通过。

第三批定向 **16 PASS / 0 FAIL**，提交 `8d1d072e` 已推送。

7. **命令开关漏锁**：卸载进行中命令开关仍派发 1 次写入 → 复用同行 rowBusy + disabledSwitch → 忙态 0 次写入，解锁后 1 次，开关仍显示原开关状态。原始失败：`busy command switch dispatched 1 writes`。
8. **命令别名漏锁**：复检中别名仍获焦并把 search 写入 debounce 队列 → 使用 rowBusy 锁输入，建面时传 !locked → 不获焦、不生成新草稿、Flush 不派发；解锁后输入正常保存。原始失败：`busy command alias accepted editing: focused=true pending=map[tool\x00scan:{tool scan search}]`。

截至第四批，旧测试断言没有改动，V/B/S 为 0/0/0。8 项跨会话行、集成动作行、命令编辑区、卸载确认组件；未增加页面、依赖或系统。
