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

第一批相关测试 **32 PASS / 0 FAIL**（包含新增两项深浅测试），V/B/S 0/0/0，旧断言零改。初稿 36 为抄录笔误，以 batch1-after 原始输出的 32 为准。全量基线仍在运行，未借用旧报告数字。

3. 数字上限放宽后红色错误行不消失 → 每帧对已有编辑按新边界校验 → 放宽 10→20 消错、收紧至 8 报错；12 与焦点均保留，输入 6 恢复。原串：`relaxed number bound left a stale error line`。
4. 文字长度放宽后仍显示超长 → 已有文本按当前长度约束校验 → 2→6 消错、6→3 报错；“工作索引”与焦点保留，输入“索引”恢复。原串：`relaxed text length left a stale error line`。

目视首批深浅图：旧文字字段仍带 `1e` 与数字错误红边；改型后回到中性槽面、下方错误行消失，动作带上移到正确间距。未改字号、行盒与材质。

第二批相关 **28 PASS / 0 FAIL**，提交 ebfcf611 已推送。开盘完整基线实测 **3764 PASS / 0 FAIL / 2773 函数**。

5. 插件文字字段暂时改成开关再恢复，旧未提交草稿仍被保留 → 离开 Text 类型即清草稿，类型进入控件身份 → 深浅变型周期退役 `/workspace/unsent`，恢复后可输入并提交 `/workspace/recovered`。原串：`retired text kind kept its unsent draft for the next text field`。
6. 插件数字字段改成开关再恢复仍保留 `1e` → 离开 Number 类型即清半输入，恢复读取宿主值 8 → 深浅恢复后可输入并提交 9。原串：`retired number kind kept its half-input for the next number field`。
