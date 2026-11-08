# P13-kl · 安装输入与配置字段的完整活动边

13:49 UTC开盘，pull已同步且工作树空。实际最新P13-kk，P13-jx已修安装恢复。已读qiaomu-design、偏好、craft-loop、preflight、中文排版和Linear供体；Codex独立执行。

## 开盘抢救

031f454f53有独有P13-kk收尾文档与日志，通过git apply -3接回；8a268ffae9的tracked树与HEAD一致，第三父树有补充exit/计数文件。合并后发现verbose日志只有1637 PASS、pass-count为空，不能用exit0声称完整通过；正在重新跑acc02b1d完整基线。产品MD5与P13-kk已验收源码一致。逐条stash处置和最终数字后续补录。

## 进度

已写八候选与真实代码位置；先事件复现再实施。旧断言零改，新Skip零。

开盘完整基线6485 PASS / 0 FAIL / exit0、3527函数；P13-kk收尾独立提交e0240437并推送。两对象已保留完整archive引用后drop，活动10→8。

| 开盘ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 8a268ffae9 | tracked等于HEAD，无patch（reverse128）；第三父树有独有exit文件 | 接回第三父树，完整对象归档refs/archive/floter/P13-kk/完整SHA后drop |
| stash@{1} | 031f454f53 | reverse1，独有收尾文档与普通测试尾部 | git apply -3接回；完整对象归档同上后drop；报告补验后reverse不通过，不冒称原patch完全相同 |
| stash@{2} | 422483b4 | reverse1，旧keycap/主题 | 原件保留，不覆盖当前主题 |
| stash@{3} | 8af94a07 | reverse1，旧rows/slider | 原件保留 |
| stash@{4} | 7e4aa710 | reverse1，旧卸载尺寸/探针 | 原件保留 |
| stash@{5} | 1e3aec94 | reverse1，旧数字字段 | 原件保留 |
| stash@{6} | 363f5a42 | reverse1，旧禁用bar/表单 | 原件保留 |
| stash@{7} | 9b8ce23c | reverse1，旧快捷键/i18n | 原件保留 |
| stash@{8} | 90d314b2 | reverse1，旧动作栏 | 原件保留 |
| stash@{9} | 7a9f6e13 | reverse1，旧main/Tauri发布链 | 原件保留，不混入Go链 |

## 第一批

1. registry包名：现象→Shift+Tab返回顶边落在Y51.4的视口起点、深色蓝红差21；改动→原输入显现19px净空，Grow留在外层保持292px宽；验证→原事件/失败恢复共25 PASS，编辑只改包名、滚轮可自由离开。
2. registry版本：现象→同一行独立Tab入口同样落在淡区；改动→独立一次显现19px，固定160px；验证→通过真实Tab从包名进入，以真实光标确认位置（原无显式label），继续输入只改版本。

M1原样撤销两处接线，真实日志见M1-red；还原MD5与touch复验见M1-restore/M1-restored。无旧V/B/S修改、新Skip0；本批包vet通过。

第一批1257c70a已推送。M1原报错：`registry-version top active edge missing: blue-red=21`；还原MD5 OK、touch后6 PASS。目视浅色版本图：原框顶部贴住页头被洗淡，修后蓝边完整、文字与160px宽保持。

## 第二批

3. 配置文本：现象→返回时原1px活动边落在淡区，深色蓝红差21；改动→控制列内一次显现19px；验证→深浅像素、继续编辑本字段、邻项值与后续滚轮。
4. 配置密码：现象→与文本相同的顶部边界缺损；改动→同列显现净空，Password遮蔽调用不动；验证→深浅事件与原配置表单/身份/恢复回归，旧断言零改。

分批条件仅用于每两项及时交付，最终覆盖单控件配置字段；嵌套多选列表保留自己的滚动链。
