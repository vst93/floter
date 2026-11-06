# P13-ic：配置局部刷新后的可用焦点

2026-11-06 20:15 UTC 开盘，HEAD c0678fea，pull 最新且工作树干净。无新增用户待修反馈；接续 P13-ib 的配置折叠与传输刷新线索。done.log 开盘不存在。Codex 独立执行。

## Stash 清点

八条 SHA 与已审核早期 WIP 一致，无新 stale-kill；每条 reverse-check 均退出 1。保留原稿，不重放已被新材质及 Go 实现取代的旧链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 事件复现

持焦字段移除/改型、Copy/Export callback 撤回、工具入口 openCommand/callback 撤回均在深浅 entry 失败，elsewhere 对照通过。Copy-owner / Export-schema 两种在飞刷新已绿，不冒充修复。标题点击会主动改变焦点，故其对照要按真实点击语义验收，不能要求还停在此前页面复选框。

原始日志保留 reproduce.txt 与 owner-reproduce.txt；完整基线仍在运行，数字完成后补录。V/B/S 旧断言修改 0/0/0，未新增 Skip。

## 第一批：字段网格

开盘完整基线已结束：**4397 PASS / 0 FAIL / 2946 测试函数**，verbose 退出 0。

1. 现象：Token 持焦时仅该字段移除，Region 与表单仍在，Tab 原点留在已不存在的编辑器。改动：只记住真实持焦字段网格，当前构建缺席后归还 Configure。验证：Tab 到 Token → schema 删 Token → Configure 持焦；用户主动移到页内复选框时不抢焦。原串 `field-remove lost usable focus "Configure"`。
2. 现象：Token 从文本改为布尔时旧编辑器键盘身份消失，用户无法判断下一步位置。改动：字段本来按 key+type 区分身份，沿该身份检测退场并归还同一配置入口，不自动切换新开关值。验证：Tab 到 Token → 同 key 改型 → Configure；elsewhere 对照不变。原串 `field-type lost usable focus "Configure"`。

M1 强制视字段仍有效，两个主题的两个 entry 场景重新报上述错误；恢复原源码、MD5 核验、touch 后复验。具体回归数字按落盘日志补录。新增样式 0；图中 Configure 重新有可见键盘位置，旧字段消失后不残留交互层。

首批相关回归 **340 PASS / 0 FAIL**，包 vet 退出 0；M1 MD5 还原、touch 复验 **10 PASS / 0 FAIL**。已目视浅 after 与深 before：Region 和操作行位置保持，Configure 外新增既有蓝色焦点环，未添加容器。V/B/S 旧断言仍为 0/0/0。首批报告曾笔误写 342，原 batch1.txt 始终为 340，本批纠正。

## 第二批：传输工具栏

第一批 **a48ae567** 已推送。以下只处理能力撤回，Copy/Export 正常在飞锁定契约继续保留。

3. 现象：Copy 持焦期间宿主撤下 CopyConfiguration，图标变灰却继续接住键盘，Space 无结果。改动：忙态之外检测 capability 撤回，交给仍可用的 Export；两项都撤回则 Configure。验证：Tab 到 Copy → 撤 callback → Export 可见焦点 → Space 正好导出一次；双撤回与用户主动移焦均有对照。原串 `copy-unavailable lost usable focus "Export configuration"`。
4. 现象：Export 的宿主 callback 撤下时同样留下不可用焦点。改动：对称归还当前 Copy，双撤回时归还 Configure；不因 transfer busy 误认为能力消失。验证：Export → callback nil → Copy → Space 正好复制一次，Copy-owner / Export-schema 在飞对照继续绿。原串 `export-unavailable lost usable focus "Copy configuration"`。

M2 撤回本批两处交接代码再跑单撤回/双撤回，之后 MD5、touch 复验。旧 V/B/S 断言 0/0/0；新增场景检验真实派发次数与不可用焦点出口。

第二批相关回归 **371 PASS / 0 FAIL**，包 vet 退出 0；M2 八个 entry（单撤回/双撤回 × 深浅 × Copy/Export）实跑红，MD5 还原、touch 后 **21 PASS / 0 FAIL**。
