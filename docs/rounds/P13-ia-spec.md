# P13-ia：异步入口随条件刷新消失时的键盘交接

起点 a7f1fb39。读取为面向工具使用者的原生设置界面，沿用轻便液态玻璃；视觉冒险 3（克制）、动效 2（即时反馈）、密度 7（保留现有信息量）。

意图：已有操作在飞或刚返回时，入口随配置/集成刷新消失，键盘仍落在当前可用动作上。沿 P13-hz 仍待线索，先用真实事件复现，不把推断记成修复。

## 八项候选与开盘代码证据

| 项 | 当前位置 | 现状线索 → 目标 |
| --- | --- | --- |
| 1 | config.go:538；settingsui.go:2935 | 保存按钮未记录交接句柄，owner 变 tool → 可用打开配置接焦 |
| 2 | 同上；settingsui.go:3174 | schema 清空后保存入口消失 → 可用 Check 接焦 |
| 3 | settingsui.go:3002 | 保存中目标变 Orphan → Adopt 接焦 |
| 4 | config.go:306,363；settingsui.go:2935,3174 | 保存时允许继续编辑；字段正持焦时变 tool 且无 openCommand → Check 接焦，禁用编辑入口不接焦 |
| 5 | settingsui.go:3918,3078 | Recheck 完成后 Broken 解除，入口消失 → Check 接焦 |
| 6 | settingsui.go:3918,3083 | Reconnect 完成后运行时可用，入口消失 → Check 接焦 |
| 7 | settingsui.go:3002,3918 | 修复在飞时目标变 Orphan → Adopt 接焦，迟到回调不抢新焦点 |
| 8 | settingsui.go:3135,4003 | 启用在飞时目标变 Orphan → Adopt 接焦，迟到回调不抢新焦点 |

每项先注入事件断言看红；不复现则换同族实际缺陷。深浅两套，另覆盖用户主动移到页头时不得抢焦、缺同行出口时页头返回可用、回调成功/失败后不重新派发。只分批注册已完成的两项。

## 红线

高度链、键位、会话管理、设置写回时机、i18n 文案、命令接线不变。3000ms 确认窗口不变；不处理保存期间草稿归属拍板项。仅修复旧控件消失时的焦点连续性，不新增操作。

## 设计依据

已读 qiaomu-design、偏好、craft-loop、preflight、chinese-typography。Apple DESIGN 的单一交互蓝/系统字体/胶囊操作映射本仓既有 accent、系统中文字体和按钮，不改色值/字阶。
craft-loop #3 保住原 2px 焦点环及 1px 间隙；#4 交接 0ms、刷新自动派发 0 次；#12 保留原 1px 受光边、新增材质 0 层。用户限定 bug 打磨，不为五项菜单加装饰。
为什么更轻：旧入口消失后，当前可用操作立即接住键盘位置，不留下看不见的交互层。

## 验收与 mutation

中文 locale / GOMAXPROCS=4；开盘函数数实测 2929，全量 verbose 基线已启动，结果落盘后补录。每批相关事件与 settingsui vet 通过后提交推送；最终 gofmt、三平台 vet/build、普通与 verbose 全量各跑一次。旧 V/B/S 断言零改、计数不减。

开盘全量已完成：4282 PASS / 0 FAIL，退出 0。

M1 撤回保存句柄或交接：owner/schema 的真实 Focused + Space pin 应红。M2 撤回修复/启用入口的交接：集成状态刷新后 pin 应红。每组先 MD5、注入、记录原始失败、还原、md5sum -c、touch 后重跑。
ui.Tester 深浅 before/after PNG 入 docs/rounds/P13-ia，代表图同时留 /tmp/floter-P13-ia-*-dark.png / light.png。
