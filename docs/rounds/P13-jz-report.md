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

第一批d80da969已推送。

### 第二批

3. **复制把旧内容确认到新草稿**：现象→复制期间更新命令/执行计划/脚本，旧帧覆盖新输入且显示已复制；改动→三类内容操作统一复核原始草稿，失效后停止旧帧写回与确认；验证→三个真实复制按钮分别触发同步更新，深浅均保留全部新字段、没有旧确认，再次复制严格得到新内容。
4. **导出后新脚本倒退**：现象→导出回调期间修改命令/脚本，回到表单又变旧，第二次导出仍是旧快照；改动→导出返回也复核草稿并停止旧帧写回，已派发内容不变；验证→连续两次导出分别为weather-script.py/旧脚本和forecast-script.py/新脚本，两套外观均严格相等。

第二批与既有回调/复制生命周期/内容出口回归88 PASS / 0 FAIL / exit0，vet exit0。M2退回仅校验form身份，实红`command/plan/script copy rewound newer draft`、`copy confirmed stale bytes`、`export rewound newer draft`；MD5还原并touch后88 PASS / exit0。旧V/B/S断言修改0/0/0、新增Skip0。

第二批aa358d54已推送。

### 第三批

5. **折叠重开后接收旧选择**：现象→选文件期间真实点击添加入口折叠再打开，同一表单指针/字段均没变，旧路径仍写入；改动→回调身份加入既有toolDisclosure代次；验证→第二个ui.Tester视图泵入两次真实折叠按钮事件，旧选择不落地，重开后的新选择正常。
6. **折叠重开后复活旧复制确认**：现象→折叠已清确认，晚回的复制重新显示已复制；改动→相同代次保护覆盖内容操作；验证→真实折叠往返后确认为空，显式重新复制才点亮，深浅均通过。

第三批新旧回归94 PASS / 0 FAIL / exit0，vet exit0。M3将代次固定为0后实红`picker fold accepted departed result`、`copy revived after fold departure`；MD5还原、touch后6 PASS / exit0。旧V/B/S断言修改0/0/0、新增Skip0。

第三批ea3a2950已推送。

### 第四批

7. **跨页返回后接收旧选择**：现象→选择器等待期间离开集成页又回来，旧选择写入当前表单；改动→只在离开集成页时推进内容回调的页面代次，与连接失败恢复代次独立；验证→真实侧栏一般页/集成页往返，旧路径不落地，重新选择正常；重选当前页不退役有效操作。
8. **跨页返回后复制确认复活**：现象→导航清除的已复制状态被旧回调重新点亮；改动→内容操作也验证页面代次；验证→真实侧栏往返后无旧确认，新的复制恰一次成功，两套外观通过。

第四批新旧定向回归100 PASS / 0 FAIL / exit0、vet exit0。M4固定页面代次为0，实红`picker page accepted departed result`与`copy revived after page departure`；MD5还原并touch后6 PASS / exit0。八项已分别复现，不将夹具错误计为缺陷；旧V/B/S断言修改0/0/0、新增Skip0。
