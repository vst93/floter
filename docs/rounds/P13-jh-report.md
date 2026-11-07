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

3. 现象：已登记包的 manifest 损坏，导出仍成功却丢掉整个包。改动：明确登记包的读取/解析失败返回路径原因；平台不匹配重新读原 manifest，不误当损坏；无包兼容记录保留。验证：原始 `manifest export replaced a good backup with unrestorable content`；坏文件不覆盖旧备份、修正后导出含完整包。
4. 现象：零字节脚本成功导出，但已有导入器拒绝空脚本。改动：导出时拒绝空必需脚本并指出路径，保留原有导入约定。验证：原始 `empty-script export replaced a good backup with unrestorable content`；写回合法脚本后同入口备份原文相等。

第二批定向 **22 PASS / 0 FAIL**，extensions/shell vet 净。深色 manifest 错误路径折行、原因尾部与关闭按钮可见；原有导入安装/合并/权限拒绝、缺脚本/descriptor 全绿。

5. 现象：含 0xff 的脚本经过 JSON 字符串导出变成 U+FFFD，备份不再是原脚本。改动：读取脚本后编码前校验 UTF-8，报实际源路径。验证：原始 `script-text export replaced a good backup with unrestorable content`；旧备份保持，修复后原文相等。
6. 现象：零字节静态 descriptor 经 omitempty 被省略，成功备份实际不能恢复工具。改动：读取后拒绝空描述文件。验证：原始 `empty-descriptor export replaced a good backup with unrestorable content`；修复文件后导出 JSON 描述内容相等，busy 释放。

第三批首次测试发现本轮夹具把 descriptor 中的 `script` 子串误认成脚本，错误要求 ScriptContent 等于描述 JSON。改为两个精确脚本场景名；描述文件自身的严格 JSON 等值断言不变。首次日志保留，没有修改旧 B pin，也没有放宽任何容差。

第三批修正夹具后 **21 PASS / 0 FAIL**，extensions/shell vet 净。前六项完成，正式 mutation 与全量门槛仍待。

7. 现象：descriptor 的非法 UTF-8 原样进入导出 JSON，自己的 ReadSync 随后拒绝。改动：读取 descriptor 时检查编码并定位源路径。验证：原始 `descriptor-text export replaced a good backup with unrestorable content`；修复后 JSON 原文一致。
8. 现象：脚本源文本不到 4 MiB，JSON 转义后超过导入上限，导出仍覆盖好备份。改动：编码完成后以最终字节数（包括文件末尾换行）检查已有 4194304 字节上限。验证：原始 `oversize export replaced a good backup with unrestorable content`；新增恰好上限可读、多一字节拒绝与转义膨胀边界。

第四批 **29 PASS / 0 FAIL**，extensions/shell vet 净。兼容测试确认无包记录仍可导出、其他平台的包仍完整携带、自定义 manifest 文件名正确保留、明确登记但缺失的包拒绝。公开 text/path/multiselect/key 的坏字节拒绝，已脱敏的密码不误拒，合法中文/U+FFFD/emoji 保留。八项四批完成，完整门槛尚待。
