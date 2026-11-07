# P13-jh · 配置传输的文本与备份完整性

起点 ac396892。读过 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography；方向沿用，单人执行。完整门槛与逐项修复尚在进行，不把计划当结果。

## Stash 清点

无新增 stale-kill；下列 SHA 与 P13-jg 登记相同，reverse-check 均为 1。早期 WIP 已有语义归档，不把旧主题/旧 Tauri 链重合到当前产品；无静默删除。

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

基线函数 3258；基线 verbose 实测 **5513 PASS / 0 FAIL**、退出 0。八项事件均在原产品复现，原始红日志见 P13-jh/before-red.txt；原产品 18 个主题/入口场景均未保护损坏输入或旧备份。

1. 现象：配置 endpoint 含 0xff 时复制/导出成功，JSON 静默把原字节替换。改动：脱敏后验证公开字符串与多选列表的 UTF-8，再编码；错误定位字段。验证：原始 `configuration transfer silently repaired invalid text: <nil>`；事件覆盖复制/文件、深浅、旧备份不变，修正为中文及合法 U+FFFD 后成功且密码仍脱敏。
2. 现象：磁盘 config.json 含非法 UTF-8，集成导出把损坏文本读成合法值并覆盖备份。改动：原始配置解析拒绝非法字节，导出错误附 config.json 路径。验证：原始 `stored-text export replaced a good backup with unrestorable content`；修正文件后同入口重试，中文/U+FFFD 原样保留。

第一批定向 **20 PASS / 0 FAIL**，extensions/shell vet 净。旧 V/B/S 断言修改 0/0/0，无新增 Skip。未来候选的失败测试先作为复现日志落盘，各批实现时再启用对应函数，保证每个提交不会带已知红测试。浅色首轮图确认字段名与 UTF-8 原因两行可读，关闭按钮完整；没有新增层级。
