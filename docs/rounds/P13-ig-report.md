# P13-ig · 表单换代后旧结果的退场

开盘 HEAD f0ef7c33，pull 最新、工作树干净。实际前轮 P13-if。首个时钟 2026-11-06 23:00:58 UTC；done.log 不存在，收尾创建。基线正在运行，后续只填实测。

## Stash 清点

八条 SHA 与 P13-if/backlog 相同，无新增 stale-kill；已接续抢救稿不重做。当前都是已登记早期独立 WIP，反向检查均退出 1，不把旧主题或旧 Tauri 发布链强行合回。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 过程记录

规格先于产品改动。旧 V/B/S 修改 0/0/0，尚未修改旧断言。

## 第一批（1–2）

1. 现象：真实点击运行，宿主同步关闭表单，旧拒绝仍被写入；再次展开后红字复活。改动：返回时确认表单仍打开、运行入口仍存在。验证：深浅两套真实点击，原版各 22 个构建帧带旧错误；修后 0 帧，remembered 答案保留、运行 1 次。
2. 现象：回调替换整个参数列表，旧拒绝仍在当帧出现一次。改动：有集成数据源时按 ID 重取当前参数列表，比较类型/必填/flag/选项，标签与排序仍可刷新。验证：原版深浅各 1 个旧反馈帧，修后 0；fresh 默认值 current 保留、运行 1 次。

定向回归 31 PASS / 0 FAIL，settingsui/launcher vet 退出 0。M1 撤回产品 diff，原串 `closed run published stale refusal in 22 frames`、`replaced run published stale refusal in 1 frames`，深浅四叶子实红；MD5 还原通过，touch 后 31 PASS。参数替换是瞬时反馈，稳定终帧截图不冒充捕获了那一帧；逐帧观察断言已钉住。

## 第二批（3–4）

3. 现象：后端报告 fresh 未知，清单随后新增 fresh，仍显示旧未知参数错误。改动：未匹配到字段的拒绝绑定整次提交的参数条件。验证：真实运行后新增字段，深浅旧红字退场，新字段默认值 current 保留，运行 1 次。
4. 现象：同类未知参数拒绝在用户修改答案后仍留在表单底部。改动：未匹配拒绝同时绑定独立答案快照，编辑后退役。验证：真实文本选中与输入 corrected，深浅旧红字退场、答案保留；未编辑的对照帧仍保留有效错误。

M2 撤回第二批产品修改，原串 `schema refresh retained obsolete unknown-parameter refusal` / `answer refresh retained obsolete unknown-parameter refusal`，四叶子实红；MD5 还原和 touch 复验通过。相关 vet 净，PASS 数详 batch2-green.txt（随收尾归档）。
