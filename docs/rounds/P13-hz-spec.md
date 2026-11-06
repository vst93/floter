# P13-hz：确认退场后的可用键盘出口

意图：已失效的问题退出时，只把仍在问题内的焦点交给当前可用入口；入口缺失时回到既有侧栏。依据为 P13-hy 仍待第一条与下列当前源码，不改变确认/保存语义。

读取为面向工具使用者的原生设置界面，克制的轻便液态玻璃；视觉冒险/动效/密度 3/2/7。功能契约：一眼识别当前集成及可用动作，确认撤回后键盘可继续，不自动执行任何操作。

## 待复现并逐批实现

| 批/项 | 位置（开盘行号） | 现状线索 → 验收 |
| --- | --- | --- |
| 1-1 | settingsui.go:2935,3332 | 丢弃中 owner 变 tool，无焦点交接 → 可用“打开配置”接住；Space 只运行当前 openCommand |
| 1-2 | settingsui.go:3002 | 丢弃中变 Orphan，未覆盖 discard → Adopt 接住，不写草稿 |
| 2-3 | settingsui.go:3177 | schema 清空后丢弃消失，无 Configure → 同行 Check 接住 |
| 2-4 | settingsui.go:3333 | tool 无 openCommand 的配置入口不可用 → 同行 Check 接住，不聚焦禁用控件 |
| 3-5 | settingsui.go:3002 | 停用变 Orphan 且无 Adopt → 既有侧栏接住，删除不自动执行 |
| 3-6 | settingsui.go:3003 | 移除变 Orphan 且无 Adopt → 同上，移除不执行 |
| 4-7 | settingsui.go:3004 | 卸载浮层变 Orphan 且无 Adopt → 侧栏接住；浮层与旧选择退场 |
| 4-8 | settingsui.go:3174 | 停用变 Broken 且无 Check → 侧栏接住，不聚焦禁用开关 |

八项分别用真实 Click/Tab/Space 事件和深浅图确认；每项另做用户先移焦侧栏的对照，刷新不抢走焦点。若线索不复现不计修复。分批测试仅注册本批完成项，防止中间提交带入后续红测试。

## 红线与设计依据

高度链、键位、会话管理、设置写回、i18n、命令接线均保持原契约；3000ms 时限不变。修复属于既有问题的焦点恢复，不新设键盘模型；保存期间草稿归属等拍板项不动。

qiaomu-design 已读 SKILL、偏好、craft-loop、preflight、chinese-typography。Apple DESIGN 的单一交互蓝、2px 焦点环、8px 控件圆角与 1px 边映射本仓现有 token；不换色、不添装饰。craft-loop #3 保住焦点环、#4 交接 0ms/自动派发 0 次、#12 无效 E2 退场且新增材质 0 层。为什么更轻：无效问题退出后，只留下当前可操作的一层。

## 验证

开盘 6bcc6dd7；中文 locale 全量 verbose 基线实测 4242 PASS / 0 FAIL / 2921 测试函数。gofmt 空、三平台 vet/build、普通与 verbose 全量在最终产品各跑一次；每批提交前运行相关 settingsui 事件与 vet，失败不提交。计数不减，旧 V/B/S 断言零改。

Mutation ≥2 组：M1 回注配置交接遗漏，应令 owner/orphan/schema/unopenable 真实焦点 pin 红；M2 回注不可用出口遗漏，应令 disable/remove/uninstall/Broken pin 红。注入前 MD5，红后逐字节恢复、touch、复跑。PNG 从 ui.Tester 导出，深浅 before/after 入库并目视。
