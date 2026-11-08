# P13-ka · 本地工具导出等待与表单拒绝后的编辑出口

开盘 2026-11-08 05:38:38 UTC，HEAD e91168f3，pull 已最新、工作区干净。仓库最新为 jz，指定安装恢复已在 jx 完成。指定 done.log 开盘不存在。Codex 独立执行，无其他代理、模型或依赖。

## stash 清点

逐条实跑反向校验均 exit1，SHA 与交接一致；八条早期独立 WIP 的语义审计沿用交接结论，不覆盖新主题或恢复旧 Tauri 链。本轮无新增 stale-kill、无 drop。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4e42eb518f4a9732465cc31be84002078 | 旧 keycap/主题 | 保留 |
| stash@{1} | 8af94a07586e56646fe74cc4da76355ecb0cc343 | 旧行距/滑块 | 保留 |
| stash@{2} | 7e4aa710feb96f0afdb6328418ded0ea3d39051d | 旧卸载布局 | 保留 |
| stash@{3} | 1e3aec9484cb2be99e73171beb05cd6806de0127 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42e2825109a5f2688831cb0495434e7444 | 旧字段/确认栏 | 保留 |
| stash@{5} | 9b8ce23cbd7a74c9fa9c92faae340b5881602070 | 旧快捷键 | 保留 |
| stash@{6} | 90d314b27296c6ee60ad29297ae1a26a8ea36165 | 旧动作栏 | 保留 |
| stash@{7} | 7a9f6e136bb2b7af5265f18ada33b408c9c4f6f7 | 旧 Tauri 发布链 | 保留 |

## 增量记录

规格已落盘，开盘全量测试运行中；后续只填写实测数字。当前未提交产品修改。

初次全量继承 LC_ALL=C.UTF-8，覆盖了 LANG=zh_CN.UTF-8，导致既有中文字体行盒 pin 失败。原始日志保留；不是本轮产品回归。显式 LC_ALL=zh_CN.UTF-8 后两个旧行盒测试通过，正在用开盘源码 overlay 重测全量。六类拒绝出口均已真实复现，编辑/连接 × 深/浅每类四组。

### 第一批：名称与命令

3. **名称拒绝后键盘落空**：现象 → 提交空名称后焦点留在提交按钮，直接键入不改名称；改动 → 将第一个名称错误关联现有输入焦点，只在提交拒绝时返回；验证 → 连接/编辑 × 深/浅四组直接输入“天气工具”，再次提交恰一次。
4. **命令拒绝后键盘落空**：现象 → 非法命令“bad command”被拒绝后无法直接修正；改动 → 返回既有命令字段；验证 → 四组直接改成 forecast，再提交恰一次，不改验证规则。

定向 **10 PASS / exit0**，settingsui vet exit0。M1 撤回两个目标后实红 `name refusal lost editable keyboard exit` / `command refusal lost editable keyboard exit`，以及 `refused direct keyboard correction: submissions=0`；MD5 还原成功，touch 后 **10 PASS / exit0**。V/B/S 旧断言修改 0/0/0，无新 Skip。

第一批 d2b4dcea 已推送。对照图肉眼看到名称输入获得蓝色焦点环，其余字段和面层保持不动。

### 第二批：版本与程序

5. **版本拒绝后键盘落空**：现象 → “version one”拒绝后直接键入无法修正；改动 → 返回当前版本输入；验证 → 连接/编辑 × 深/浅四组直接改为 2.3.4，再提交一次。
6. **空程序拒绝没有出口**：现象 → 空程序提交静默返回，输入仍落在按钮；改动 → 返回已有程序字段，不增加文案或改空值规则；验证 → 四组直接输入 /usr/bin/weather，再提交一次。

M2 撤回两个目标，实红 `version refusal lost editable keyboard exit` / `program refusal lost editable keyboard exit`；还原 MD5、touch 后累计 **20 PASS / exit0**，settingsui vet exit0，旧 V/B/S 0/0/0。

第二批 93b2a4ad 已推送，但发生提交时序错误：图片汇总后才运行的 git add 尚未结束，就开始第三批修改，导致该提交误带第三批的两个新测试及尚未还原的空焦点分支。版本/程序两项自身已过门槛；整份第二批树不满足新脚本/平台测试，不能声称其全绿。第三批立即补回两个焦点目标并验证，不重写远端历史。后续所有提交明确等待完成才改源码。

### 第三批：脚本与平台

7. **空脚本拒绝无法直接补写**：现象 → 空脚本拒绝后输入未进入代码编辑器；改动 → 返回原编辑器焦点，保留已有缓冲；验证 → 四组直接输入 print('sunny') 后提交一次。
8. **平台全清后没有键盘选择出口**：现象 → 提交后 Space 仍按提交按钮；改动 → 聚焦平台行第一个既有复选框；验证 → 四组 Space 补选平台后提交一次，不自动选择、不更改平台限制。

M3 撤回脚本/平台目标后实红 `script refusal lost editable keyboard exit` / `platform refusal lost editable keyboard exit`；MD5 还原、touch 后累计 **30 PASS / exit0**，settingsui vet exit0。旧 V/B/S 0/0/0。

第三批 41aab284 已推送，六类基本表单出口已全绿。中文 locale 开盘源码 overlay 的完整基线 **6101 PASS / 0 FAIL / exit0**。

### 第四批：导出等待与编辑出口

1. **导出等待重复打开保存窗口**：现象 → 连点导出派发两个保存对话框，换表单还能继续派发；改动 → 接通宿主完成通知，当前导出出口等待期间保留原几何并显示禁用面；验证 → 深浅两组连续点击只派发一次、换表单仍等待，完成后恰一次导出 forecast-script.py 与新脚本内容。
2. **导出取消或失败后不能接着写脚本**：现象 → 焦点停在动作按钮，直接输入落空；改动 → 派发后把自己持有的焦点交回原脚本，晚完成只解锁、不夺回用户已移动的焦点；验证 → 深浅原编辑器和主动改焦点四组直接输入正确落地，真实宿主 saveFile 的取消/选择失败/写入失败/成功四支各完成一次且可重试，取消无通知。

新旧定向回归 **146 PASS / 0 FAIL / exit0**。M4 撤回等待闸和返焦后实红 `waiting export dispatched 2 save dialogs` / `export completion lost current keyboard exit: 脚本` / `export return dropped keyboard correction`；MD5 还原、touch 后 **13 PASS / exit0**，settingsui/shell vet exit0。旧直接宿主导出的并发通知 B 测试保持通过。无新增动画、无几何变化；只复用禁用面，不新增 spinner。
