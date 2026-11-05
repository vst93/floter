# P13-hi · 配置 schema 刷新后的编辑状态

开盘 2ed2e7a8，工作树干净；备用 SSH 443 已同步。done.log 开盘不存在。规格先落盘，验收随每批追加。

## stash 清点

八条实际 reverse-check 均退出 1；SHA 与 P13-hh 结案表完全一致，无新 stale-kill。早期 WIP 原件继续保留，不把 reverse 失败当作必须覆盖现实现的理由。

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | 旧 keycap 与主题 | 保留原件 |
| stash@{1} / 8af94a07 | 旧行距与滑块 | 保留原件 |
| stash@{2} / 7e4aa710 | 旧卸载布局与探针 | 保留原件 |
| stash@{3} / 1e3aec94 | 旧数字框与透镜 | 保留原件 |
| stash@{4} / 363f5a42 | 旧确认带与快捷键 | 保留原件 |
| stash@{5} / 9b8ce23c | 旧快捷键反馈 | 保留原件 |
| stash@{6} / 90d314b2 | action bar 旧键盘合约 | 保留原件 |
| stash@{7} / 7a9f6e13 | 旧 Tauri 发布链 | 保留原件 |

## 增量验收

1. 数字改文字后仍显示半输入与数字错误 → 改型清掉旧 live 和错误 → 深浅数字输入 `1e` 后改文字，旧提示消失，重新输入“搜索”成功。原串：`text field inherited the number half-input or error`。
2. 文字改数字后撤销写回旧文字历史的 12 → 编辑器身份包含类型 → 深浅 Ctrl/Cmd+Z 不改已有值，重新输入 56 得到数字。原串：`number field inherited text undo and wrote an unrequested value: 12`。

第一批相关测试 36 PASS / 0 FAIL（包含新增两项深浅测试），V/B/S 0/0/0，旧断言零改。全量基线仍在运行，未借用旧报告数字。
