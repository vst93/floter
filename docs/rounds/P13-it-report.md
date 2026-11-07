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

第一批 `6d0d46fa` 已推送。

## 第二批

3. 现象：45 秒原值下输入 30，切换 Off 写文件失败后，输入仍退场，草稿变回 45。改动：存储接受预设后才收起输入。验证：真实下拉键盘选择 + 本地文件 rename 失败，原值 45、原草稿 30、重试入口保留；恢复文件后 Enter 恰再写一次成功。
4. 现象：直接 Enter 提交 30 秒失败也提前关闭并覆盖输入。改动：存储接受后再规范化草稿及决定展开状态。验证：两主题真实输入、失败、恢复后重试，写次数 1 → 2，未提前改写原值。

相关 **30 PASS / 0 FAIL**，包 vet 净。M2 回注旧文件 **6 FAIL**，原串 `preset-save-failure discarded retry draft: open=false draft="45"`、`custom-save-failure discarded retry draft: open=false draft="45"`；MD5 OK，touch 后 **6 PASS**。旧 V/B/S 0/0/0。参数 Required 与即时校验探针均为绿，不计修复、不改实现。

第二批 `06d75e0d` 已推送。

## 第三批

5. 现象：输入 4294967296 秒、-5、45.6、1e2 均错误回落到 10 秒。改动：依据参考 `surface-residency.ts` 的既有数值规范，有限数值先限幅再舍入；保留 never 哨兵与 NaN/Inf 默认值。验证：深浅真实 Type/Enter，分别为 86400/0/46/100；哨兵、非有限数保护均绿。此族算一项。
6. 现象：输入闭合时宿主更新为 53 秒，自定义菜单可见却无法选择同值打开编辑器。改动：闭合态收到 custom 时种入原值并打开输入，不触碰活动草稿。验证：两主题真实 Down/End/Enter，53 可读可编辑且未产生写入。

相关 **46 PASS / 0 FAIL**，包 vet 净。旧 V/B/S 0/0/0。上述全部数值先在旧实现上见红；沿用前两组正式 mutation，不虚称本批额外变异。最初尝试的历史路径 `src/components/settings/GeneralPage.tsx` 不存在，随后通过该分支树清单定位 `src/settings/GeneralPage.tsx`；未做全盘搜索。

第三批 `fd06c6bd` 已推送。

## 第四批

7. 现象：新快捷键草稿绑定时磁盘写入失败，整行消失、键位及命令清空，未提示失败。改动：检查本次 Store 写入结果，失败时保留草稿并沿用 save alert；系统注册拒绝语义不变。验证：真实 Add/录制/输入/Bind，原实现深浅均红，修后恢复文件能直接重试原命令。
8. 现象：已存命令编辑 Enter 写失败，原编辑缓存被清，下一次 Enter 只能存回旧值。改动：接受写入后再清缓存。验证：深浅真实命令编辑、文件故障、Enter、恢复后再次 Enter，原文本与焦点保留，最终提交 unfinished。

相关 **94 PASS / 0 FAIL**，包 vet 净；旧 V/B/S 0/0/0。原始红串 `failed add discarded draft: open=false key="" command=""`、`failed command discarded command draft: map[]`；本批先见红再修，正式 mutation 仍为 M1/M2 两组。

第四批 `ceff09ac` 已推送。

## 第五批

9. 现象：把已存动作改为命令行时写文件失败，原绑定还在，却又出现同键的新草稿。改动：接受移除后才初始化命令草稿。验证：两主题真实菜单 End/Enter + 磁盘失败，原绑定/选择器仍在、重复草稿不出现。
10. 现象：删除有未提交命令的绑定时写失败，行仍在却丢掉编辑内容，焦点误回 Add。改动：接受删除后才清编辑/重排录制归属；确认原键已消失才交 Add。验证：真实 Tab/Enter，原 unfinished 文本与删除入口焦点保留。

旧实现原串 `failed command-mode switch duplicated saved binding as draft`、`failed remove discarded command draft: map[]`、`failed remove lost original keyboard outlet`。相关实测计数见 batch5-test；包 vet 结果见 batch5-vet。旧 V/B/S 0/0/0。
