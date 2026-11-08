# P13-jx · 安装表单失败恢复

2026-11-08 03:32 UTC开盘。HEAD 762606e1，pull已最新，工作区干净；done.log在给定路径不存在。按上轮“仍待”继续registry表单与本地选择出口。全量基线实测5998 PASS / 0 FAIL、3403函数、exit0。

## stash清点

以下均本轮实际reverse-check=1，SHA与P13-jw相同。它们是早期独立WIP，不是待接续stale-kill；沿用交接语义审计，不能用旧主题覆盖现有设计。没有新增独有抢救稿、没有drop。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap/主题 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/滑块 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42 | 旧字段/确认栏 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏 | 保留 |
| stash@{7} | 7a9f6e13 | 旧Tauri发布链 | 保留 |

## 增量实施记录

候选与mutation计划见spec。每两项完成即记录、提交、推送；旧测试断言不动。

### 第一批：失败与取消恢复

1. **下载/解析失败**：现象→HTTP 503后再次点击派发空包名/空版本；改动→失败回到UI线程恢复本次输入，编辑revision保护后续草稿；验证→真实HTTP + 点击、Tab输入版本、完成后再次点击，断言恢复`review-current / ^1.0.0`，深浅两套。
2. **权限取消**：现象→拒绝审核后输入被丢弃；改动→取消清理下载目录后恢复本次输入，仍不发失败通知；验证→本地registry包与权限回调、取消后重新点击，输入完整、通知数0，深浅两套。

M1实跑：撤回两条宿主恢复接线，两个场景深浅均红，原始串`download lost retry draft: "" / ""`、`cancel lost retry draft: "" / ""`。MD5还原OK，touch复跑6 PASS，旧提交即清空B测试1 PASS，定向vet无输出。第一次夹具误把无Label的版本占位符当Click目标，未触发业务；已改为真实Tab进入版本，初次输出保留但不计mutation证据。旧V/B/S修改0/0/0。

### 第二批：registry等待与空态

3. **等待期间仍可重复提交**：现象→本地/推荐已有锁而registry仍亮着可按；改动→同一安装入口复用packageInstallBusy，显示原忙态环、禁用重复点击，宿主同步拒绝重复派发；验证→等待期间真实点击派发0次、完成后重试1次，原取消宿主回归仍通过。
4. **空包名仍是Tab停靠点**：现象→空白包名的安装按钮看起来可用、能拿焦点，却无动作；改动→空白包名走disabledActionButton，输入有效包名即解锁；验证→Tab跳过、点击派发0次、填入后派发1次，深浅均验。

M2将表单还原到上一批产品，实红`busy registry permits repeat: 1`与`blank install remains a keyboard stop`；MD5还原OK、touch后6 PASS。宿主恢复与旧取消回归9 PASS，相关vet净。第一批已推送e5c74f50；V/B/S旧断言修改仍0/0/0。

### 第三批：提交失败与registry键盘

5. **审核后提交失败**：现象→准备成功、审核批准，但manifest在落盘前消失，结果报错且两字段仍丢失；改动→提交阶段所有错误也走本次草稿恢复；验证→在真实权限回调移除下载目录manifest，看到`extensions: no manifest`，完成后重新点击派发原名/版本，只有一个失败通知，深浅均验。
6. **registry等待丢键盘**：现象→Enter提交后焦点留在禁用按钮，继续输入无效；改动→只在原安装按钮持有焦点时，把键盘交回已有包名字段；验证→Enter提交、等待中直接输入`next-package`成功，完成不抢回焦点，焦点已在关闭按钮时也不抢。深浅均验。

M3a撤回提交失败恢复，深浅均报`commit lost retry draft: "" / ""`，还原后3 PASS；M3b撤回焦点接线，深浅均报`waiting registry lost keyboard focus`，还原后3 PASS。两次都MD5核对、touch复跑。新草稿保护另有name/version/输入后清空三条真实编辑事件，共4 PASS；定向vet净。键盘夹具起初漏接Close回调，补上后负向不抢焦点也通过，旧B测试未动。

### 第四批：本地入口退出与错误反馈

7. **本地审核取消后误重试**：现象→Enter提交、等待并取消审核后，再按Enter会再次安装（实测close=0、installs=2）；改动→原按钮持有键盘时，等待态把焦点交给现有关闭按钮，完成不抢回；验证→实际Tab/Enter、等待/取消/Enter链，close=1、installs=1，深浅两套。单纯文件选择器空结果取消原本已能保留入口焦点，不把该正常路径算bug。
8. **选择器错误静默**：现象→系统选择器失败没有任何结果，用户无法区分取消和失败；改动→复用既有LocalToolFailed宿主warning，不新增文案或浮层；验证→失败一次warning、取消不新增、重新选择成功派发一次，真实shell接线在深浅帧均看见提示。

M4撤回本地两条接线，实红`folder waiting lost keyboard focus`、`cancelled folder exit: close=0 installs=2`、`picker failure warning=[]`；MD5还原后touch复跑6 PASS。宿主警告3 PASS，相关vet净。八项均完成，四批产品提交；后续只补完整门槛、图像复核和交接。
