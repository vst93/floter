# P13-ii · 复制反馈与当前可操作表面一致

开盘 2026-11-07 00:09:18 UTC，实际 HEAD d7556438（P13-ih），pull 最新且工作区干净。用户任务书中的 P13-hw 已被仓库后续迭代覆盖，本轮按最新仍待继续。done.log 不存在，收尾补建。单独由 Codex 执行。

## Stash 清点

八条 SHA 与 P13-ih 一致，无新增 stale-kill；早期抢救已经接续/归档。本次实跑 reverse-check 均为 1；原稿继续保留，避免旧主题与旧 Tauri 链覆盖当前产品。

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

## 选题核对

普通内容变更已经由 copiedContentValue 清理，不能重复记修复。ExportScript 生产实现异步导出点击时的内容，没有按钮确认可误写；保留对照，不计修复。候选集中到反馈离开所属面后的清理，以及没有 CopyText 时的真实可用性。基线、复现与后续结果按实测增量补录。

## 第一批（1–2）

基线完整退出 0：4639 PASS / 0 FAIL，开盘 3003 测试函数。

1. 现象：真实点击复制命令，宿主回调打开内容相同的 local.second，返回后新表单显示“命令已复制”。改动：复制返回核对原表单 key、展开状态和当前复制能力。验证：深浅新工具显示原始复制文案，主动再点才派发第二次且确认；旧 1600ms 常量不变。
2. 现象：宿主没有 CopyText，复制命令依旧取得焦点、可按却无结果。改动：内容按钮走现有 disabledActionButton，能力撤回时退役确认；恢复能力后原位可用。验证：深浅真实点击不再取得焦点、不确认；恢复后一次点击派发一次。

原版与 M1 注入均实红：`old copy returned success into new editor`、`unavailable content copy still accepts focus`。当批相关 38 PASS / 0 FAIL、包 vet 净；M1 6 FAIL，MD5 逐字节还原、touch 后 6 PASS / 0 FAIL。旧 V/B/S 修改 0/0/0，无新增 Skip。已目视浅色 after：local.second 的复制动作回到原始文案，未新增一层面。

## 第二批（3–4）

第一批 72173676 已推送。

3. 现象：复制命令或执行计划后，真实点击脚本模式再切回程序，旧成功文案复活。改动：操作组退出程序模式时退役其确认。验证：命令/计划分别在深浅主题复制→切换→返回，不重放剪贴板调用，保留字段值；再次明确复制才确认。
4. 现象：复制脚本后切换程序再回脚本，旧“脚本已复制”再次出现。改动：脚本组退出时同样退役。验证：深浅实际模式控件往返，脚本文本保持、当前复制可用；导出仍收到同一文件名和全文，且不抢占或延长复制反馈。

原串：`command copy success revived after mode roundtrip`、`plan copy success revived after mode roundtrip`、`script copy success revived after mode roundtrip`。相关 64 PASS / 0 FAIL、包 vet 净；M2 撤回本批补丁 8 FAIL，MD5 还原 touch 后 8 PASS / 0 FAIL。旧 V/B/S 0/0/0。已目视深色脚本 after：代码仍在，复制文案回到待操作状态，导出位置不动。

## 第三批（5–6）

第二批 a73f1250 已推送。

5. 现象：复制后点击“添加本机工具”折叠，再展开仍显示旧成功。改动：折叠时退役复制反馈。验证：深浅真实折叠/展开按钮保持 weather 的命令、程序和编辑 ID，复制次数仍为 1；再点一次才显示新成功。
6. 现象：复制后点击常规页再回集成页，已离场的成功文案复活。改动：实际离开集成页时清理该页复制反馈。验证：完整设置页深浅侧栏点击往返不再显示旧成功；重选当前页保持尚有效反馈，草稿与后续复制均正常。

原串：`copy-fold revived old copy success`、`copy-page revived old copy success`。相关 71 PASS / 0 FAIL、包 vet 净；M3 撤回后 6 FAIL，MD5 还原 touch 后 6 PASS / 0 FAIL。旧 V/B/S 0/0/0，无新增 Skip。

## 第四批（7–8）

第三批 d09bdf78 已推送。

7. 现象：关于页无复制回调，两个链接的复制入口仍可聚焦；能力撤回后旧成功还在。改动：既有文字动作支持禁用态，深链按当前能力传入；撤回时清确认。验证：深浅真实点击不取得焦点，不冒报成功；能力恢复后复制和成功通知各一次，随后撤回清理成功。没有添加一层胶囊或新提示。
8. 现象：关于页复制链接后切常规再返回，“链接已复制”重新出现。改动：实际离开关于页退役该页复制确认。验证：深浅完整侧栏事件无旧成功、无额外复制或通知；重选当前页保留反馈，返回后明确再复制正常，1600ms 保持。

原串：`unavailable link copy still accepts focus`、`withdrawn clipboard kept link success`、`About page revived old copy success`。相关 95 PASS / 0 FAIL、包 vet 净；M4 撤回后 6 FAIL，两个源码文件 MD5 还原 touch 后 6 PASS / 0 FAIL。旧 V/B/S 0/0/0，无新增 Skip。已目视浅色关于页：两个无能力入口显示灰色 Copy 墨，链接文本仍可读，原有行与空白节奏保留。
