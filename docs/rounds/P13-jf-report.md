# P13-jf：文件传输失败的原因与恢复

开盘 2c61b589，pull 已同步、工作树干净。基线于 2026-11-07 16:10:22 UTC 开始；Codex 独立执行。done.log 开盘不存在。仅实测完成项计入交付。

## Stash 清点

八条均与已归档轮次相同，无新增 stale-kill。逐条导出 patch、reverse-check 全为 1；不是 HEAD 已覆盖，但为已审核早期独立 WIP，继续保留，不把旧主题或 Tauri 链盖回当前 Go 产品。

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

旧 V/B/S 断言修改 0/0/0；无新增 Skip。

1. 现象：选择已被移走的集成文件，只提示无效文档。改动：接回 os.ReadFile 的路径与错误。验证：深浅两主题经已接线的 ImportIntegrations 动作与真实临时文件事件，原产品报 `file read failure hides its cause`；修后全文可见、busy 释放，补回文件后同入口成功产生报告。
2. 现象：导出遇到损坏仓库，也提示无效导入文件。改动：展示 ExportSync 返回的仓库读取原因。验证：原产品报 `repository export failure hides its cause`；修后保存选择器 0 次、无目的文件、busy 释放，修复仓库再导出 1 个集成。

第一批相关 **17 PASS / 0 FAIL**，shell vet 净；原产品真实红证据 before12.txt。初始指针夹具受整页高度影响，改用产品已接线 Actions + 真实文件事件，不声称系统原生选择器真机验收。正式 mutation 后续独立注入并 MD5 还原。

基线结束：**5454 PASS / 0 FAIL / 3237 测试函数**，与 P13-je 一致。第一批 1a6853f2 已推送。

3. 现象：不支持的版本和 JSON 语法错误只剩同一个标题。改动：保留原本地化标题，用已有 Toast.Description 接回实际解析原因；满高浮层补画原来遗漏的详情。验证：两种文档 × 深浅，原产品 `document refusal hides its cause`；修后标题与原因同时可见、不产生报告，改正文件可重试。
4. 现象：仓库损坏被当空仓库，合法导入误显示跳过 1 项。改动：ImportSync 在任何应用动作前返回 RepositoryErr，宿主显示实际错误。验证：两主题原产品 `unreadable repository was treated as empty` 与 `unreadable repository fabricated import report`；修后仓库字节不变、无虚假报告，恢复仓库后成功 1/跳过 0/失败 0。

第二批相关 **62 PASS / 0 FAIL**，extensions/shell vet 净。旧标题 B pin、原单行 toast 的结构和键盘规则未改；新增详情列以 2u 间隔排原文，无新增浮层。

第二批 9c0871c6 已推送。第一次目视：浅色浮层原标题下一行明确显示不支持 version 99；关闭键完整，单一浮面不再只有笼统拒绝。深色报告的普通多段路径本来就能换行，未计修复；进一步测试无断点文件名。

5. 现象：脚本保存选择器不可用只显示“导出失败”，无恢复线索。改动：保留原标题并显示选择器返回原因。验证：深浅两主题，原产品 `script-picker hides failure cause`；修后可见“restart the desktop portal”，恢复选择器后导出成功，原取消静默 B 测试仍绿。
6. 现象：脚本写入目录等真实磁盘错误也被压成笼统失败。改动：接回路径与文件系统原因。验证：原产品 `script-write hides failure cause`；修后原因可见，改用合法路径重试文件字节等于原脚本。

第三批相关 **16 PASS / 0 FAIL**，shell vet 净。没有改变导出脚本、选择器或成功提示的时机。
