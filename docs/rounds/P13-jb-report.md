# P13-jb：快捷键录制的退出与恢复

开盘 2026-11-07 12:46 UTC，起点 9d76d244。pull 同步，工作区干净。中文 locale 实测基线 5328 PASS / 0 FAIL，退出 0；cmd/internal 下 3201 个 Test 函数。上一轮“命令输入退出”线索引出录制器未随页面/窗口离开的问题，本轮以真实 UI 点击、按键和宿主退出接口验证。

## Stash 清点

八条均为已登记早期 WIP，无新的 stale-kill。与 P13-ja SHA 相同；反向检查失败本身不代表独有可接续工作，按 backlog 已有语义核对保留，未重入旧主题/Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | reverse=1，早期 keycap WIP | 保留 |
| stash@{1} | 8af94a07 | reverse=1，早期设置间距 WIP | 保留 |
| stash@{2} | 7e4aa710 | reverse=1，卸载指标/临时测试 WIP | 保留 |
| stash@{3} | 1e3aec94 | reverse=1，旧数字字段 WIP | 保留 |
| stash@{4} | 363f5a42 | reverse=1，旧插件字段 WIP | 保留 |
| stash@{5} | 9b8ce23c | reverse=1，旧快捷键校验 WIP | 保留 |
| stash@{6} | 90d314b2 | reverse=1，旧启动器动作栏 WIP | 保留 |
| stash@{7} | 7a9f6e13 | reverse=1，旧 main/Tauri WIP | 保留 |

done.log 开盘不存在，收尾创建，不伪造上轮记录。八项结果随每批追加。

## 第一批：切页结束不可见录制

1. **内置录制切页**：现象 → 点击 Sessions 后旧录制仍暂停召回，第一次 Escape 只取消看不见的录制；回 Shortcuts 重新显示录制提示。改动 → selectPage 离开 Shortcuts 时结束捕获并即时同步注册。验证 → 深浅真实点击，召回与自定义原键均恢复，返回不复活，Escape 一次关闭；未 Enter 的命令保持原样并可再次提交。
2. **自定义录制切页**：现象 → 编辑命令后录制该行，切页同样留下暂停与旧提示。改动 → 同一退出方法同时清除自定义捕获归属，不清草稿。验证 → 深浅事件确认原 Ctrl+Shift+B / say original 未被离开动作改写，say unfinished 回来仍可 Enter。

相关回归 274 PASS / 0 FAIL，settingsui/shell vet 净。M1 回注原 selectPage/录制实现实红：`invisible recorder still suspends global shortcuts`、`exit did not restore the summon and custom registrations`、`invisible recording swallowed page Escape`、`return resurrected the old recording`。MD5 恢复、touch 后 6 PASS / 0 FAIL。补充返回瞬间截图后重新实跑 M1，避免用 Escape 已消掉问题的图片作对比。既有 V/B/S 0/0/0，旧测试不改。

第一批 dc47d2f1 已推送。

## 第二批：关闭设置后释放录制

3. **内置录制关闭设置**：现象 → 点击页头关闭后到了启动器，召回仍被暂停；重开又显示旧录制。改动 → Open 离开设置时先结束捕获，再走原 Flush。验证 → 深浅真实关闭按钮，原注册立即恢复，重开无旧提示，草稿未保存且可继续 Enter。
4. **自定义录制关闭设置**：现象 → 自定义命令行改键期间关闭，已保存键没有恢复、返回抢焦点。改动 → 相同宿主出口释放自定义捕获归属。验证 → 原 Ctrl+Shift+B/say original 保持，say unfinished 回来可编辑提交，原召回与自定义注册都存在。

相关回归 280 PASS / 0 FAIL，包 vet 净。M2 回注缺少 Open 退出处理的旧分支，实红 `invisible recorder still suspends global shortcuts`、`exit did not restore the summon and custom registrations`、`reopen resurrected the old recording`；MD5 还原、touch 后 6 PASS / 0 FAIL。既有 V/B/S 0/0/0。
