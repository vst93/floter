# P13-hv：内联确认的退出、超时与并存

开盘 `d0fd58d4`，pull 最新、工作树干净。done.log 不存在；已读取实际存在的 stash-backlog。基线正在实跑，完成后补录。开始时间 14:29 UTC 左右。

## Stash 清点

既有 stale-kill 抢救已在前轮归档接续；以下活动 stash SHA 与 P13-hu 一致，本轮逐条 reverse-check 均为 1，保留独立旧 WIP，不覆盖新主题或恢复 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实施记录

仅完成且通过事件验收的项计入；旧 V/B/S 断言修改暂为 0/0/0。

### 第一批：外部收起的键盘出口

1. 现象：停用确认在 Escape 或 3 秒到期后丢失焦点，Space 不能重开。改动：用跨帧 Handle 识别正在退出的答案，仅在它拥有键盘时归还原开关。验证：深浅 × Escape/timeout × 焦点仍在/已移到检查按钮，共八场景；取消派发 0 次，原入口可以 Space 重开。
2. 现象：移除确认同样丢失原入口。改动：同族按归属回到移除按钮，进入组件卸载时保留浮层焦点。验证：同样八场景，并回归已有卸载与显式取消路径。

基线全量实测 **4083 PASS / 0 FAIL / 2888 测试函数**。第一批相关回归 **97 PASS / 0 FAIL**。M1 回注旧产品：`external exit lost its keyboard opener`、`Space cannot reopen the cancelled question`，8 个场景红；还原后 MD5 通过、touch 复跑 **18 PASS**，settingsui vet 通过。V/B/S 旧断言修改 **0/0/0**。

第一批 `8412ad8a` 已推送。

### 第二批：配置丢弃的 Escape 与到期

3. 现象：丢弃问题打开时 Escape 执行设置 Close，问题没有取消。改动：把已有 configDiscard 加入全局确认取消集合，保留草稿、表单展开状态，不写回。验证：深浅 × 中英文 × 焦点仍在/已移开，Close=0、保存=0、值仍为 edited，Space 可重开。
4. 现象：配置丢弃在 3 秒到期后丢失 Configure 焦点。改动：沿用跨帧答案 Handle，仅在离场答案持有键盘时还给入口。验证：同样八场景，检查按钮上的焦点不受影响。

相关回归 **103 PASS / 0 FAIL**；M2 原始红串 `discard exit closed settings or lost draft: closed=1 armed=true open=true value=edited saved=0`、`discard exit lost Configure focus`；恢复 MD5 与 touch 后 **18 PASS**，settingsui vet 净。V/B/S 0/0/0，旧断言不改。

第二批 `83a8b09f` 已推送。

### 第三批：慢失败的阅读与重试窗口

5. 现象：停用操作等待 5 秒后失败，下一帧立即撤掉错误，用户既读不到原因也不能 Space 重试。改动：仅问题仍开着时，从失败到达时重新开始原有 3 秒回答窗口。验证：两主题等待 5 秒、失败显示、2 秒后仍显示、Space 第二次派发、3 秒后正常撤回；已 Escape 取消的迟到失败不复开。
6. 现象：移除操作同样被提问时钟吞掉失败。改动：同族重启回答窗口，不改执行与取消语义。验证：同上四场景，实际回调计数 1→2，取消场景不显示迟到失败。

相关回归 **112 PASS / 0 FAIL**；M3 回注旧时钟，原始红串 `slow failure vanished before it could be read`、`slow failure cannot retry from keyboard: calls=1`，四场景红。MD5 还原并 touch 后 **10 PASS**；settingsui vet 净。V/B/S 0/0/0。

第三批 `d9924ea6` 已推送。

### 第四批：两种确认并存的身份

7. 现象：先开停用、2 秒后再开移除并 Tab 到确认；停用到期后移除按钮失焦，Space 派发 0 次。改动：两条确认使用各自稳定的根 key，移除不再继承前一条的节点。验证：深浅完整页面真实事件，停用撤回、移除仍开且 Space 恰好移除 1 次，停用 0 次。
8. 现象：先开移除再点开关询问停用，新问题默认焦点却落在旧移除的取消；Space 取消了错误的问题。改动：给新停用独立身份，首次 AutoFocus 作用于它自己的 Cancel。验证：深浅 Space 只收起停用，移除仍保留，两种破坏性回调均 0 次。

相关回归 **118 PASS / 0 FAIL**；M4 四场景实跑红：`earlier question expiry lost removal answer focus`、`new question default cancels the wrong band: disableOpen=true removeOpen=false disable=0 remove=0`。MD5 校验、touch 后 **6 PASS**，settingsui vet 净。第二轮看图将第 8 项截图时点移到 Space 之后，让图上直接看见留下的是哪个问题；断言不变。
