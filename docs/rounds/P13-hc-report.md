# P13-hc · 本地工具主表单的输入归属与恢复

起点 cb345a63，2026-11-05；pull --ff-only 已同步、工作树空。唯一执行者 Codex，无外部模型和子代理。done.log 开盘不存在，收尾创建。

## stash 清点

以下均已实际 reverse-check，全部失败。按 P13-gq/ha/hb 入库的语义审核，属于早期独立 WIP，不能把旧主题和旧 Tauri 发布链覆盖当前实现；保留原件。任务书所指 stale-kill 已由前轮归档处理，当前没有新 orphan。

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | P7 keycap 与旧主题稿 | 保留原件 |
| stash@{1} / 8af94a07 | 设置行距/滑块旧刻度 | 保留原件 |
| stash@{2} / 7e4aa710 | 卸载旧布局与 scratch 探针 | 保留原件 |
| stash@{3} / 1e3aec94 | 旧数字框与透镜稿 | 保留原件 |
| stash@{4} / 363f5a42 | 旧确认带/快捷键稿 | 保留原件 |
| stash@{5} / 9b8ce23c | P7 快捷键拒绝反馈稿 | 保留原件 |
| stash@{6} / 90d314b2 | action bar 旧键盘合约 | 保留原件 |
| stash@{7} / 7a9f6e13 | 旧 Tauri 发布链 | 保留原件 |

## 增量验收

当前中文 locale 全量基线 **3608 PASS / 0 FAIL**，旧测试函数实测 **2719**。原始日志 `/tmp/floter-P13-hc-baseline.log`。

1. **类型切换后命令输入丢失**：选中 Command 后切换脚本模式，继续输入仍是 weather → Command 行使用稳定语义身份 → 深浅真实输入均变成 forecast。原始红串 `mode switch lost command edit: "weather"`。
2. **类型切换后版本输入丢失**：选中 Version 后切换模式，输入仍是 1.2.3 → Version 行保持自己的身份 → 输入恰为 2.0.0。原始红串 `mode switch lost version edit: "1.2.3"`。

第一批相关回归 **33 PASS / 0 FAIL**，internal/ci 通过。V/B/S 0/0/0，旧断言零改。深浅 Command 前后 PNG 已实际导出。校验提示插入和 Advanced 展开两候选实测通过，未计入修复。

3. **撤销带回上一工具名称**：第二个工具点 Undo 得到 First → 表单每次 beginEdit/endEdit 分配独立编辑身份 → 第二工具旧历史为空、自己的编辑仍能撤销回 Second。原始红串 `new tool inherited text undo: "First"`。
4. **界面翻译变化后参数输入丢失**：英文参数 Label 全选后切换中文，输入仍是 Target → 六个参数字段使用不依赖翻译的语义 key → 深浅两套均接受 Destination，类型切换与删除参数的旧回归保持。

第二批相关回归 **37 PASS / 0 FAIL**；旧断言零改，V/B/S 0/0/0。首批 a70e682f 已推送。参数语言切换深浅前后图已导出。

5. **删前一条后录制改写错误绑定**：原第二条录制中删除第一条，Ctrl+Shift+R 被写到 clipboard 那条 → 删除/转为命令草稿时同步录制行索引 → 键只写到原 say original 绑定，clipboard 保持 Ctrl+Shift+C。原始红串 `recording moved to another binding: [{Key:Ctrl+Shift+B Action:say original} {Key:Ctrl+Shift+R Action:plugin:clipboard}]`。
6. **删除正在录制的行仍继续录制**：原第一条被删，后续按键改写存活的第二条 → 删除录制对象立即取消录制 → 录制提示消失，余下两条键值与草稿都保持。原始红串 `removed binding left recorder armed`、`removed recorder rewrote surviving binding`。

第三批相关快捷键回归通过，具体计数见提交正文与最终日志。删除和「转为命令草稿」两条移除路径共用修复，键盘格式/持久化回调保持。V/B/S 0/0/0。
