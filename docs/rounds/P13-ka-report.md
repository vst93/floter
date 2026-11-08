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
