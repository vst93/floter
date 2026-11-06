# P13-hm · 配置输入的值与状态

起点 2fa36d8e，已 pull，工作区为空。done.log 开盘不存在。规格见 P13-hm-spec.md。

## Stash 清点

| 开盘 ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | reverse-check 1，早期 keycap WIP | 保留 |
| stash@{1} / 8af94a07 | reverse-check 1，早期行距 WIP | 保留 |
| stash@{2} / 7e4aa710 | reverse-check 1，早期卸载探针 WIP | 保留 |
| stash@{3} / 1e3aec94 | reverse-check 1，早期数字控件 WIP | 保留 |
| stash@{4} / 363f5a42 | reverse-check 1，早期插件配置 WIP | 保留 |
| stash@{5} / 9b8ce23c | reverse-check 1，早期快捷键 WIP | 保留 |
| stash@{6} / 90d314b2 | reverse-check 1，早期启动器 WIP | 保留 |
| stash@{7} / 7a9f6e13 | reverse-check 1，旧 Tauri 分支 | 保留 |

活动 stash 无 stale-kill。任务书所列抢救稿已有 P13-gq/ha/hh/hk/hl 的归档与接续记录，最新 e0f58d758c 已归档并 drop；不重新套用过时草稿。reverse-check 失败不等于独有工作，本轮没有静默删除。

## 增量验收

开盘代码确认：sheetNumberSlot 无条件 clampNumber（无界成为 0）；写回使用 boundedValue 而显示仅 clamp，存在双值。先用真实输入事件复现，再分批修正。

基线完整退出 0：**3816 PASS / 0 FAIL / 2808 测试函数**。前轮报告记 2806，本轮以当前 HEAD 实测为准。

### 第一批（1–2）

1. 现象：无边界的数字框输入 42 后显示并派发 0。改动：用现有 boundedValue 的“仅声明过界才夹界”规则。验证：深浅真实键入均派发/显示 42，M1 旧产品原串 `unbounded input stored 0, want 42`。
2. 现象：Step=1 的框输入 3.7，宿主拿到 4，框仍是 3.7。改动：草稿也用同一量化结果。验证：深浅显示/派发均为 4，随后半输入 1e 保留且不写回；M1 原串 `snapped input displays "3.7", want 4`。

浅色 after 图肉眼看到右侧槽内是单个 4，取代 3.7；行高和单位位置未变。框架 Tester.Texts 不含编辑器内部文字，初版探针对此作了错误假设，已改为验证真实事件驱动的草稿与宿主值，并导出像素；没有修改旧测试或产品以迎合探针。
M1 注入原产品，两个新测试均红，恢复后 md5sum -c OK、touch 复跑。V/B/S 0/0/0，完整门槛留待末批，当前仅称受影响回归通过。

第一批受影响回归 **14 PASS / 0 FAIL**，settingsui vet 与 diff-check 净；8bd9c36d 已推送。

### 第二批（3–4）

3. 现象：数字框聚焦编辑为 25 后，schema 将 Max 改成 10，仍显示 25。改动：重绘时仅将可解析草稿按当前约束重算，半输入保持。验证：从真实编辑器全选复制得到 10；不因 schema 刷新派发写回；1e 保留、后续 6 正常写入。旧代码原串 `refreshed bounds display "25", want 10`，深浅均红。
4. 现象：Step=20、存量 30 时读数为 40，但滑块停在 30。改动：轨道初值使用与读数相同的 boundedValue。验证：深浅整帧逐字节等于存量 40 的参考帧，未触发 Change。旧代码原串 `slider says 40 but its position differs from the 40 frame`。

两项均由 ui.Tester 导出深浅 before/after。V/B/S 0/0/0；旧断言未动。M2 将两条产品路径回退到第一批，再跑本批两条 pin，恢复时按 MD5 核对并 touch；精确结果随门槛日志归档。
