# P13-ic：仍在场的配置入口，刷新后的可用焦点

开盘 c0678fea，2026-11-06 20:15 UTC；pull 最新、树干净。按 P13-ib 最新仍待接续，由 Codex 独立执行。

意图：配置编辑器、传输工具栏与外部编辑入口发生局部刷新后，键盘仍落在一个当前可用的控件上。完整 ui.Tester 事件已证实以下失败；Copy-owner / Export-schema 在飞刷新已通过，作为对照保留，不算修复。

## 八项计划（改动前真实行号）

| 项 | 位置 | 现状 → 应改成 |
| --- | --- | --- |
| 1 | config.go:370 / :652 | 持焦字段被 schema 移除，表单仍在而焦点悬空 → 当前 Configure 接住 |
| 2 | config.go:725 | 字段同 key 改型后旧编辑器退场 → Configure 接住，不自动修改新字段值 |
| 3 | config.go:423 | Copy callback 撤回，持焦图标被禁用 → 可用 Export 接住；否则回 Configure |
| 4 | config.go:430 | Export callback 撤回 → 可用 Copy 接住；否则回 Configure |
| 5 | settingsui.go:3367 | tool openCommand 撤回，按钮仍持焦但不能做事 → 当前 Check 接住，缺失则回页头 |
| 6 | settingsui.go:3367 | tool OpenConfiguration callback 撤回 → 同一可用出口；不执行已撤回命令 |
| 7 | settingsui.go:2950 | owner tool→host 且无 schema 时外部编辑按钮消失 → 当前 Check 接住，缺失则回页头；有 schema 的对照已通过 |
| 8 | settingsui.go:2941 | 点击配置行标题关闭干净表单，焦点留在非键盘标题 → 既有 Configure 接住；覆盖 Copy/Export 在飞的折叠/重开 |

不同表面不扩张，跨三个现有组件：字段网格、传输工具栏、配置入口行。同族八项分四批，每批两项；未完成批次的失败复现测试不混入可提交门槛。

## 红线与设计读取

读取为：面向工具使用者的原生集成设置页，轻便液态玻璃；视觉冒险 3（克制）、动效 2（即时键盘）、密度 7（沿用紧凑布局）。三秒内看清配置归属、是否有未保存项及下一步可操作位置。功能契约是继续编辑、复制/导出、打开工具配置或安全返回。

高度链、键盘按键/顺序、会话、设置写回时机、草稿归属、i18n、命令语义、3000ms 确认期限全部保持。只修失效节点的焦点交接；不新增页面、依赖或系统。

qiaomu-design 已读取 SKILL、偏好账本、craft-loop、preflight、chinese-typography；打磨路径不做四方向预览。Apple DESIGN 的三项职责转译：单一交互色→保留本仓深 #8FB7FF / 浅 #064AA6；药丸 CTA→保留 30px 高、15px 半径；阴影只表达层级→本轮新增投影 0 层。craft 菜单 #3 用已有 2px 环/1px 间隙；#4 焦点 0ms 交接且刷新派发 0 次；#12 保持已有 1px 受光边，组件不叠玻璃。

## 门槛与变异

基线全量 verbose 实测退出 0，4397 PASS / 0 FAIL，测试函数 2946。环境 LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4。每批定向事件、旧 Notice/Pending/Missing/重复 ID 回归及包 vet 通过后 commit/push；最终一次 gofmt -l、三平台 vet、普通/verbose 全量、三平台 build。

- M1 撤字段网格交接，让移除/改型的 focus pin 变红。
- M2 撤传输能力失效交接，让 Copy/Export pin 变红。
- M3 撤工具入口交接，让 openCommand/callback/owner 刷新 pin 变红。
- M4 撤标题折叠后的归还，让折叠与在飞重开 pin 变红。

每条先备份 MD5，注入并记录真实红串，再逐字节还原、md5sum -c、touch 后复验；深浅 before/after 用 ui.Tester PNG，入本轮目录并目视。
