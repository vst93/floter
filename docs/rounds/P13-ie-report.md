# P13-ie · 表单拒绝与动作确认的条件刷新

开盘 2cfe705d，pull 最新、工作树干净。实际承接 P13-id。首个时钟 21:44:03 UTC。done.log 开盘不存在，结束创建。已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple 供体。

## Stash 清点

八条均与交接账本及上轮 SHA 一致，没有新增 stale-kill；reverse-check 全部退出 1。它们是已审核的独立旧 WIP，不能据反向应用失败覆盖当前主题或 Go 发布链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap；reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider；reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局；reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段；reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段；reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键；reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏；reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri；reverse=1 | 保留 |

## 验证记录

完整基线正在运行；后续只写实测数字。旧 V/B/S 断言暂为 0/0/0，无新增 Skip。

## 第一批（1–2）

1. 现象：运行拒绝后改正 Target，旧红字仍说参数无效。改动：拒绝记录绑定提交值；该值改变后退役，编辑 Note 保留 Target 的错误。验证：真实点击运行、点击输入、Cmd+A/输入；深浅均通过，刷新派发 0 次。
2. 现象：被拒绝的 Target 移除后，底部仍责备不存在的字段，重加还会复活。改动：拒绝随该参数值槽退役。验证：真实运行失败后移除/重加，错误消失且默认值恢复；深浅通过。

定向回归 **65 PASS / 0 FAIL**，settingsui vet 退出 0。M1 用 HEAD 原产品注入两项缺陷，四个深浅叶子均红；原串 `edited parameter left obsolete refusal visible`、`removed parameter left obsolete refusal visible`、`re-added parameter resurrected refusal`。cp 还原后 `md5sum -c` 为 OK，touch 后以上 65 PASS。最初测试夹具有语法及辅助函数重名，两次编译失败均修正后才取得实红，未把编译失败算 mutation。

目视浅色 before：输入已经是 fixed，底部仍有红色旧错误；修复只撤掉失效反馈，不改变输入框和操作行。

## 第二批（3–4）

3. 现象：同一参数从自由文字改为可选值，原来的后台拒绝仍挂在新控件下面。改动：拒绝快照保存类型、必填、参数旗标与选项副本；校验条件变化即退役。验证：运行拒绝 → 原值仍在的 Select schema → 不留旧错、不自动运行，深浅通过。
4. 现象：同一参数改名后，错误仍点名已不存在的 Target。改动：保存结构化拒绝，按当前字段标签与语言重新解释，保留错误本身。验证：Target → Destination 后只出现当前名称的拒绝，深浅通过。

定向回归 **71 PASS / 0 FAIL**，settingsui vet 退出 0。M2 撤回这批产品后四叶子实红：`constraints parameter left obsolete refusal visible`、`renamed parameter left obsolete refusal visible`、`renamed parameter lost current refusal wording`；MD5 OK、touch 后以上 71 PASS。旧 V/B/S 0/0/0。
