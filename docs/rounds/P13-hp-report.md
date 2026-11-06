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

第一批相关 93 PASS；0c0d38d5 已推送。

3. 插件文本改走再改回原文、不按回车，外部刷新被旧草稿挡住 → 按上一代宿主值辨认撤回 → 深浅复制得到 external、0 次保存且焦点仍在。
4. 外部已写入同一份插件草稿，后续值仍被旧文字挡住 → 新宿主代际同值确认后退役草稿 → 下一代 latest 可见，真正冲突的 unsent 继续保留。

M2 回退 pluginsheet/settingsui 两文件，原文 `withdrawn plugin text masks host`、
`host-confirmed plugin text masks next value`；MD5 恢复并 touch，相关回归与 vet 通过。V/B/S 0/0/0。

第二批相关 99 PASS；060ae37e 已推送。

5. 集成数字存 1e308 却展开 309 个字符，槽内只见尾零 → 采用已有数字显示格式 → 深浅复制完整 1e+308，dirty=false，宿主值不变。
6. 插件无步长数 1e-308 展开 310 个字符 → 非零绝对值小于 1e-6 用科学计数法 → 完整 1e-308，最小次正规数、负数和阈值逐值往返解析，0 次写入。

M3 原文 `integration number expands to 309 characters, want 1e+308`、
`tiny number expands to 310 characters, want 1e-308`。MD5 还原、touch 后相关 105 PASS，vet 通过。
V/B/S 0/0/0；第三批只改显示，保存精度与步长不变。

侦察撤回：关闭插件浮层两帧后重开，文本/数字 Ctrl+Z 均不复活旧输入，四场景已 PASS；不计修复。
