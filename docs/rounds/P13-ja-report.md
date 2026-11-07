# P13-ja：快捷键变更后的排队回调归属

开盘 8df8ec68，pull 最新、工作树干净。按实际最新 P13-iz 接续。Codex 独立执行。

## Stash 清点

八条均 reverse-check 退出 1，SHA 与 P13-iz 相同；这是早期独立 WIP，先前 stale-kill 已归档，不重合旧主题/旧 Tauri。自动回收清单尾部 aa5e214d48 不在活动 stash，不冒充本轮可合入的新稿。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse 1 | 保留 |

## 增量记录

首次 C.UTF-8 基线出现字体相关失败，按 AGENT-NOTES 改用 zh_CN.UTF-8；保留原日志，不改旧视觉 pin。正式基线运行中。

### 第一批

1. 现象：删除保存绑定后，已排队的原生回调仍执行 `say original`。改动：每批自定义键注册持有代次，替换批次先退役旧回调。验证：真实点击 Remove 后投递保留的旧回调，深浅两套原代码均红 `retired custom callback dispatched: [say original]`，修后派发 0 次。
2. 现象：命令 Enter 保存替换后，旧回调仍执行旧命令。改动：同一代次保护阻断旧闭包；当前注册的新动作继续可用。验证：真实输入并 Enter 后旧派发 0 次、新派发恰好 1 次且是 `say replacement`。

第一批相关 71 PASS / 0 FAIL，shell vet 净。M1 撤代次判断实跑红，MD5 恢复、touch 后 6 PASS。V/B/S 0/0/0，旧断言未改。图已导出；这两项是不可见命令派发的行为修复，设置帧前后相同属于预期，不冒充视觉变化。

第一批 e208f9f3 已推送；中文基线退出 0，5301 PASS / 0 FAIL。

### 第二批

3. 现象：宿主暂停全局键后、录制者状态尚未到达时，自定义旧回调仍派发命令。改动：暂停立即递增代次，并检查宿主暂停位。验证：暂停→投递旧回调→恢复→投递新回调，旧 0 次、新 1 次；原串 `suspended native callback dispatched: [say original]`。
4. 现象：真实点召回录制器后，已排队召回回调仍进入 Toggle，可能切走窗口。改动：四条召回注册路径统一检查暂停与录制状态。验证：深浅 Click→旧回调→Escape→新回调，旧调用不进入 Toggle，新调用仍到达原防连发门；原串 `queued summon reached Toggle = 1, want dispatch false`。

“注册被拒的回调”草稿探针原实现已过，不计修复。仅宿主暂停位测试是直接宿主生命周期入口，其余路径均真实 UI 输入。召回探针停在原防连发门，未创建系统窗口；不把它称为原生桌面验证。
