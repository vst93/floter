# P13-ji · 导入部分生效与传输重试的结果归属

起点 da23e1b8。17:55 UTC 开盘，Codex 独立执行。已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN；保持既定方向。done.log 开盘不存在，收尾建立。基线与候选正在实测，不把计划记成完成。

## Stash 清点

活动清单无 stale-kill；以下八条与 P13-jh SHA 相同，reverse-check 全为 1。沿用已审核的早期 WIP 保留结论，不把旧主题或 Tauri 链重新覆盖现产品。每条原 patch 与检查日志暂存 /tmp/floter-P13-ji。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 增量记录

真实 UI 的 transferRow 点击已清空旧报告（settingsui.go:2219/2231），所以“新导入保留旧报告”线索不成立，不改产品、不计修复。下一步只验证部分生效与已解决通知。

基线中文 locale verbose 全量实测 **5555 PASS / 0 FAIL**、退出 0；全仓 Go 测试函数 **3270**（git grep '^func Test' -- '*.go'；只数 cmd/internal 的窄路径会漏掉两条，不能混口径）。八项候选均已在原产品复现：部分生效三阶段遗漏事实，五入口成功仍显示旧错。初次新夹具部分场景按钮在滚动区外，补真实 Scroll 后全部复现；初次日志保留，不拿夹具错误计产品缺陷。

1. 现象：导入已把原工具停用，随后损坏配置解析失败，报告只显示 JSON 原因。改动：已写回启用状态后遇到配置失败，补充真实 enabled 值与未完成阶段，保留原始 cause。验证：深浅真实 Click 导入、磁盘 enabled=false、破损配置字节未变；原始红 `partial enabled hides committed state`，修复配置可同入口重试。
2. 现象：新包已经安装并出现在仓库里，配置错误却让报告只说失败。改动：安装完成后的配置错误加“package was installed”，不改变写入顺序或回滚语义。验证：深浅 Click、新包记录存在/已停用、失败计数 1 成功 0；原始红 `partial package hides committed state`，修复后导入值真正落盘。

第一批相关回归与 vet 结果见 batch1-test.txt / batch1-vet.txt；完整全量留到轮末。旧 V/B/S 断言修改 0/0/0。

第一批定向 **13 PASS / 0 FAIL**，extensions/shell vet 净。新夹具首次有一次未等 Click 的下一帧派发就检查磁盘，补事件派发断言和 Frame 后通过；原失败日志保留。未来六项目前仅保留复现 helper，逐批启用相应测试，避免提交已知红测试。

第一批 **19c4a8e5** 已提交推送。

3. 现象：新包的配置模板目录被同名文件阻塞，包和仓库已落盘，报告只给 mkdir 错误。改动：保留 Commit 返回的已安装 entry 事实，说明包已安装、配置模板未完成。验证：深浅真实导入，仓库有新包且仍 enabled=true（尚未执行后续停用），失败数 1；原始红 `partial template hides committed state`。不自动补跑模板或回滚。
4. 现象：集成备份因仓库损坏而失败，修好后导出成功仍和旧错误同时浮在页面。改动：通用错误槽记录操作归属，导出成功只退役本入口的旧错；成功原 ID 和时长保留。验证：深浅真实 Click、文件确实写出、旧错误消失；原始红 `successful export retry retains its resolved error`。另验已绘制/排队中的外部错误及其他操作错误均不被清除。

第二批定向 **27 PASS / 0 FAIL**，extensions/shell vet 净。V/B/S 0/0/0；没有改变旧断言。

第二批 **9be9b5db** 已提交推送。

5. 现象：配置复制遇到损坏文本失败，修正后剪贴板已有正确文档，但旧错还在。改动：复制操作按动作+集成 ID 登记错误，实际剪贴板交付后退役。验证：深浅宿主 Actions 与 ui.Tester 通知事件，原始红 `successful copy retry retains its resolved error`；剪贴板含修正 endpoint 且无原密码，旧错误消失、成功可见，旧延迟交付测试通过。
6. 现象：配置导出选择器失败后恢复，文件已写出却同时显示旧错误。改动：同样按导出动作+集成 ID 退役，仅发生在成功写入后，取消不算恢复。验证：深浅选择器失败→成功写盘，原始红 `successful config-export retry retains its resolved error`；旧错消失、新结果可见，原验证失败与磁盘错误回归通过。

第三批定向 **22 PASS / 0 FAIL**，shell vet 净。V/B/S 0/0/0。
