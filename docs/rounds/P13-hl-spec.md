# P13-hl · 禁用墨：八处现有控件的状态细节

起点 `1765f2cd`，2026-11-06；开盘工作区为空且已 pull。接续 `e0f58d758c` 抢救稿，原稿数字有误，以下以本次复测取代。基线日志 **3806 PASS / 0 FAIL / 2797 测试函数**，进程退出状态的异常记录见报告。

## 意图与证据

让同一种中性禁用状态可辨且可读：弱文字适度增强，过强的禁用文字退让；沿用已有 E0/E1/E2 和单次 0.5 禁用衰减。现有薄面不变，文字只保留一条清晰的状态层级。

设计读取：桌面设置控件和输出窗口工具条，面向高频键盘用户，方向为轻便液态玻璃。冒险度 2/10（稳定）、动效 2/10（沿用）、密度 7/10（紧凑）。用户需立即辨认动作名与当前可用性。

## 八项规格（开盘已读代码行号）

| 项 | 位置 | 现状 → 目标 |
|---|---|---|
| 1 | rows.go:616 extensionActionFace | 禁用胶囊 105/106 → 统一中性墨；普通、忙态、配置入口一并修 |
| 2 | rows.go:764 extensionIconButton | 透明扩展图标 69/70 → 同一中性禁用墨 |
| 3 | rows.go:1095 sessionIconButton | 会话图标 59/61 → 清楚且弱于启用态 |
| 4 | settingsui.go:6476 barButton | 中性确认标签 105/106 → 同一中性禁用墨；危险色保留 |
| 5 | settingsui.go:6543 图标子树 | 警告字旁边是主墨图标 → 图标明确取标签的 ink |
| 6 | uninstall.go:335 uninstallActionButton | 忙态中性胶囊 105/106 → 同一中性禁用墨 |
| 7 | uninstall.go:444 uninstallPartRow | 固定卡提示单独取样仅 54/53 → 提示和标题都使用 Disabled |
| 8 | shell/detach.go:398 detachedBar | 运行中重跑键对比 50/45 → 中性禁用墨，仍弱于可用态 |

前七处在 internal/settingsui，第八处在 internal/shell。theme.go:247/667/703/838 新增语义墨是实现手段，不独立计项。
最终 token 为浅 **#57575B** / 深 **#B5B5BD**（框架衰减前）。草稿 #58585C/#B4B4BC 会让导入/导出旧严格相等 pin 红；校准产品颜色后原断言不动。

## 行为红线与交付

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线不动；面差、受光边、阴影、圆角、间距不动。Disabled/lockedControl 仍只负责既有禁用位和单次衰减。
每两项提交推送，相关回归每批过；最终 gofmt 空、三平台 vet/build 净、普通及 verbose 全量测试通过，PASS 与函数数量不减。V pin 明确旧值→新值，B/S 不改。

截图使用 ui.Tester 的实际构造器：两套主题、两个组件组、同几何的 before/after。以开盘提交的原始产品文件重拍 before，再 MD5 还原当前代码；无合成假 UI。

## Mutation 计划与落地

- M1：胶囊墨退回 TextStrong → TestDisabledActionLabelsShareInk 红；还原 MD5 + touch 后复跑。
- M2：移除确认图标 TextColor → TestConfirmationIconUsesLabelSemantics 四态红；同上。
- M3：固定卡退回旧提示墨 → TestFixedUninstallHintKeepsReadableInk 的英文/中文、深浅四处红；同上。
- M4：输出窗口退回 TextTertiary → TestTheDisabledBarKeyDimsOnce 深浅精确值红；同上。

## Skill 的具体使用

读取 qiaomu-design、user-preferences、craft-loop、preflight、chinese-typography，走既定方向打磨。
Apple 供体的 48% 退让墨转译为框架已有 0.5；近黑/近白延续 #1B1B1D/#F7F7F8；交互蓝不加到禁用态。
工艺菜单 #6 语义文字层级、#11 图标语义色、#12 禁用面受光边为 0；具体数值与逐条 preflight 结论见报告。不为凑五项增加装饰，用户范围优先。
