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

第二批实测 59 PASS / 0 FAIL，提交 1e9472df。开盘基线完整退出 0：4585 PASS / 0 FAIL。基线命令结束后才数函数已包含本轮新增 4 个，因此输出 2991；基线 HEAD 的函数数按 git 对象单独计数，不能把这个末尾数冒充原基线。

## 第三批（5–6）

5. 现象：配置接口复用 Config 对象，写入回调重开后旧拒绝回滚到 true/10，旧成功覆盖成 true/80。改动：每次打开/关闭递增显示代次，写入结果只归派发时的显示期。验证：深浅 × 成功/拒绝，保留新表单 false/25、无旧失败，写入 1 次。
6. 现象：危险动作回调重开同对象后，旧失败显示在新面板。改动：动作返回同样核对显示代次。验证：深浅真实两次点击，重新打开无旧红字；再次第一击只询问，Escape 关闭面板且不执行。

M3 撤回显示代次核对，6 叶子实红，原串 `reopened sheet inherited old write: values=map[enabled:true limit:10] failed=true` / `reopened sheet inherited old action refusal: calls=1 failed="clear"`。MD5 还原和 touch 复验通过，相关 vet 净；精确 PASS 计数见归档 batch3-green.txt。

过程偏差：第三批新增夹具最初把 launcher Escape 误当作仅取消动作；实际既有契约是关闭配置面，已修正新断言，未改产品键盘模型或旧 B 断言。第一次 M3 复验失败期间曾启动第二次脚本，造成检查进程重叠；最终脚本退出与源码 diff 独立核对，不把失败算 PASS。第二批首次 push 被 GitHub 500 拒绝，重试成功；没有丢本地提交。

第三批最终 70 PASS / 0 FAIL，提交 6644d780 已推送。基线 HEAD f0ef7c33 的测试函数实测 2987。

## 第四批（7–8）

7. 现象：写入记录开关时回调新增保存格式，旧成功/拒绝返回整表值，新增 html 值丢失且上限被改回。改动：派发前复制整张字段约束，返回核对兄弟字段。验证：深浅 × 两种返回，新增 html、上限 25 和开关 false 均保留，只写 1 次。
8. 现象：同一写入期间移除保留上限，旧返回把 limit 键重新带回并可能报错。改动：整表约束失配时不回滚旧快照、不接纳旧整表成功值。验证：深浅 × 两种返回，limit 不复活、开关 false、无旧失败，写入 1 次。

M4 撤回整表核对，8 叶子实红；原串 `new sibling lost its current value: map[enabled:true limit:80]` / `retired sibling resurrected in values: map[enabled:true limit:10]`。MD5 还原、touch 复验通过，相关 vet 净；精确 PASS 数见 batch4-green.txt。所有八项均有独立事件断言；旧 V/B/S 0/0/0、Skip 新增 0。
