# P13-it：设置表单的草稿、刷新与失败恢复

07:39 UTC 开盘，起点 `1be93d7a`；pull 最新，工作区干净。实际从 P13-is 接续；P13-hx 已完成旧任务书的确认条件刷新，不重复计数。Codex 独立执行，已读取指定设计 skill 与参考。

## Stash 清点

八条 SHA 与 P13-is 一致；逐条导出 patch、reverse-check 均退出 1。早期独立 WIP 的语义处置已由 backlog 与前轮审计确认，沿用保留，未重合旧主题/Tauri 链；没有新的 stale-kill。开盘 done.log 不存在，收尾按指定路径创建。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

基线全量实测 **5050 PASS / 0 FAIL / 3110 函数**，退出 0。

## 第一批

1. 现象：首次打开已保存的 45 秒自定义值，字段为空，Enter 写成 0。改动：初始化展开状态时同时给草稿种子。验证：深浅真实 Tab/复制读值/Enter，要求读回 45 且提交仍为 45；旧实现两主题均红。
2. 现象：自定义草稿 47 秒、菜单高亮 Off 时宿主改为 53 秒，旧 Enter 写成 0 且抹掉草稿。改动：菜单观察原始秒数，原值变更即退役旧菜单。验证：两主题旧菜单撤走、宿主 53 保留、草稿 47 保留、选择器焦点保留；旧实现两主题均红。

相关 **16 PASS / 0 FAIL**，包 vet 净。M1 回注旧文件 **6 FAIL**，原串 `stored custom duration opened as "", want 45`、`stale draft menu overwrote host duration with 0`；MD5 OK，touch 后 **6 PASS**。旧 V/B/S 0/0/0。新菜单测试最初用 HasText("关闭") 匹配到了说明文字，已改为只存在于菜单的 20 秒选项；数值、草稿、焦点断言全部保留。目视浅色：旧浮层消失，左侧自定义（53 秒）、右侧草稿 47，2px 蓝环仍在原入口。
