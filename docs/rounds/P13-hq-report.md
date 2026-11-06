# P13-hq 报告：配置刷新后的编辑与校验

开盘 ef5e7b13，pull 最新，树干净。中文 locale 基线 3923 PASS / 0 FAIL / 2842 函数。done.log 不存在。

## 抢救 stash 清点

权威 git stash list 当前八条，reverse-check 全部退出 1；SHA 与 P13-hp 相同，stat 已逐项核对。backlog 的 stale-kill 抢救稿已接续/归档，这八条是早期独立 WIP，继续保留原件；不把 reverse 失败当作独有工作强行套回产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap 未完稿 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/滑块稿 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/探针 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段稿 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽稿 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键形状稿 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏行为稿 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 Tauri 发布稿 | 保留 |

## 增量记录

1. 前置分组后文本失焦、未提交内容丢失 → 分组身份按存活字段匹配 → 原实现深浅均红：`text-prepend lost focus on group refresh`、`lost draft: "", want "/local"`。
2. 数字跨分组后半输入丢失 → 优先沿用聚焦字段原分组身份 → 原实现深浅均红：`number-move lost focus on group refresh`、`lost draft: "", want "1e"`。

第一批两项通过，相关事件回归 **80 PASS / 0 FAIL**，settingsui vet 通过；旧 V/B/S 断言零改动。M1 回退 pluginsheet.go 后两条深浅场景确实报上述失焦/丢草稿，md5sum -c 两文件均 OK，touch 后复跑通过。第一次变异误将 App 新成员一起撤回导致编译失败，该次不计有效变异；已纠正注入范围并以真实断言红日志覆盖。文本保留后 Enter 只向自己的字段提交一次；数字夹具覆盖从非首行移入已有分组。

第一批提交 e2bd2905 正文将相关 PASS 误录为 93；原始 batch1.txt 实测为 80，本报告更正。基线 3923 与变异结果无误。

后续两组旧校验路径已有原实现红证据，移除重加路径待本批实测。完整门槛在八项完成后运行一次。

3. 已存文本按 Enter 显示长度错误后，外部纠正为 ok，仍留旧红句 → 未编辑字段接纳新宿主值时清理旧校验 → 深浅字段 clean、旧句消失。
4. 已存数字 25 按 Enter 显示范围错误后，外部纠正为 5，旧红句仍在 → 同一宿主刷新路径清旧校验 → 深浅值 5、状态 clean、旧句消失。

第二批相关 80 PASS / 0 FAIL、settingsui vet 净。M2 回退 config.go 两项均红：`text-correct retains obsolete validation: "outside its allowed length"`、`number-correct retains obsolete validation: "outside its allowed range"`；md5sum -c OK、touch 后复跑。V/B/S 0/0/0。

5. 存量文本按 Enter 报错后放宽长度，仍显示旧错误 → 对已显示的错误按当前 schema 重验，未验证值仍不主动验证 → 深浅放宽后旧句退出、值与 dirty 不变。
6. 存量数字按 Enter 报错后放宽范围，仍显示旧错误 → 同上覆盖数字 → 深浅放宽后旧句退出，不触发写回。

第三批相关 80 PASS / 0 FAIL、settingsui vet 净。M3 原文 `text-relax retains obsolete validation: "outside its allowed length"` 与 `number-relax retains obsolete validation: "outside its allowed range"`；两文件日志留存，MD5 还原 OK、touch 复跑。V/B/S 0/0/0。
