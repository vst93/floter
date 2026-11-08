# P13-jz · 本地工具选择与内容回调的草稿归属

开盘：2026-11-08，HEAD 8a6cf2df，pull已最新，原工作区干净。任务书进度停在jw，仓库已完成jx/jy，因此接续jy“同表单选择回调”线索。指定done.log不存在。本轮唯一执行者Codex，未调用其他模型或代理。

## stash清点

八条逐条实跑reverse-check均exit1，SHA与交接一致；它们是早期独立WIP，不是尚未接续的stale-kill。沿用交接语义审计保留，不恢复旧主题或Tauri链；无drop，无遗漏的新抢救稿。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4e42eb518f4a9732465cc31be84002078 | 旧keycap/主题 | 保留 |
| stash@{1} | 8af94a07586e56646fe74cc4da76355ecb0cc343 | 旧行距/滑块 | 保留 |
| stash@{2} | 7e4aa710feb96f0afdb6328418ded0ea3d39051d | 旧卸载布局 | 保留 |
| stash@{3} | 1e3aec9484cb2be99e73171beb05cd6806de0127 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42e2825109a5f2688831cb0495434e7444 | 旧字段/确认栏 | 保留 |
| stash@{5} | 9b8ce23cbd7a74c9fa9c92faae340b5881602070 | 旧快捷键 | 保留 |
| stash@{6} | 90d314b27296c6ee60ad29297ae1a26a8ea36165 | 旧动作栏 | 保留 |
| stash@{7} | 7a9f6e136bb2b7af5265f18ada33b408c9c4f6f7 | 旧Tauri发布链 | 保留 |

## 增量记录

开盘中文locale全量实测6069 PASS / 0 FAIL / exit0，原3425测试函数。基线日志在本轮目录。首轮未修夹具提前取了英文label，后三组导航/复制复现因此失败；这不是产品缺陷证据，修正语言初始化后单独重新复现，原日志保留。

### 第一批

1. **选择返回撤销新输入**：现象→同一表单改模式、改写字段或主动清空路径后，旧选择覆盖整份局部字段；改动→操作前捕获原始字段与表单身份，返回时不一致便退役当前帧；验证→真实选择按钮事件，回调同步注入模式/脚本、名称/路径/命令/参数/版本/说明和清空三种更新，两套外观都保留最新值。
2. **选择器错误键盘断路**：现象→报错后焦点仍在选择按钮，直接输入丢失；改动→保留一次既有警告并返回原程序输入，沿用焦点归属检查；验证→Tab/Enter触发权限错误后直接全选键入，路径成为/usr/bin/retry，深浅均通过。

M1撤回两处保护后实红`picker rewound newer mode`、`picker rewound newer fields`、`picker refilled cleared path`、`picker error lost editable exit`、`picker error dropped keyboard correction`。MD5还原并touch后定向26 PASS / 0 FAIL / exit0，vet exit0；旧V/B/S断言修改0/0/0，无新增Skip。
