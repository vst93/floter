# P13-hp 报告：配置编辑的冲突与恢复

开盘 035ca0bd，pull 最新，树干净。中文 locale 基线 3893 PASS / 0 FAIL / 2832 测试函数（全仓 Test 声明统计）。
qiaomu-design 与指定参考已读；不调用子代理或外部模型。done.log 开盘不存在。

## 抢救 stash 清点

八条 reverse-check 均退出 1，SHA 与 ho 相同，git stash show --stat 已核对。
backlog 明确这八条是早期独立 WIP，stale-kill 抢救稿已归档/接续；不把 reverse 失败当作应强行覆盖产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | P7 keycap 未完稿 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/滑块稿 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/探针 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段稿 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽稿 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键形状稿 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏行为稿 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 Tauri 发布稿 | 保留 |

## 进行中

1. 文本先本地编辑，再收到外部冲突，回到外部值仍显示未保存 → 冲突只保留草稿、saved 更新为新宿主值 → 深浅真实输入，回到 external 为 Saved、回到 old 为 Unsaved。
2. 数字 1e 遇外部 40，恢复默认 5 却显示 Saved → 同一宿主基准用于默认恢复和合法数值比较 → 半输入保留、恢复默认仍 dirty、输入 40.0 后 clean。

M1 回退 config.go，两项深浅全红；原文 `returning to external text still says Unsaved changes`、
`restoring obsolete default falsely says Saved`。MD5 还原成功，touch 后相关测试与 vet 通过；
日志 mutation1.txt、batch1-after.txt。V/B/S 0/0/0，旧断言零改动。
