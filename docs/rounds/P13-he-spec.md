# P13-he · 配置选项的身份与可读反馈

2026-11-05，起点 `674724d0`，工作树干净且已 fast-forward pull。沿 P13-hd「仍待」第 2 条，检查配置下拉/多选的选项变化。原任务书禁用胶囊已经由后续轮次修复，不重复做。

意图：让选项的焦点、范围选择与显示值始终对应用户正在操作的内容。为什么现在做：`config.go:41,92–109` 仅记录多选索引；`:668` 以位置创建选项；`pluginsheet.go:364` 同样按位置创建 Checkbox；`settingsui.go:4731` 使用找不到时返回 0 的 optionIndex。它们是上轮字段身份修复的内部同类边界。

读取为：原生桌面配置表单，面向配置插件与集成的用户，沿用轻便液态玻璃；视觉冒险 2/10（稳定骨架）、动效 2/10（即时反馈）、密度 6/10（保留紧凑布局）。三秒内必须辨认当前值和焦点，完成单选、多选及恢复。不开新页面、系统或依赖。

## 落地项（先复现；未红的候选不计修复）

| 项 | 位置（开盘行号） | 现状 → 目标 |
|---|---|---|
| 1 | config.go:92–109 | 删除范围锚点后 Shift 错选下一项 → 删除的锚点失效，不由邻居继承 |
| 2 | config.go:668 | 选项换序后焦点沿位置漂移 → 随选项值保持 |
| 3 | config.go:92–109 | 范围锚点沿旧索引 → 换序随原值、删除后失效 |
| 4 | pluginsheet.go:364 | 插件多选键盘焦点沿位置 → 随 option.Value 保持 |
| 5 | settingsui.go:4731–4740 | 未知保存值显示第一项 → 显示原值，直到用户主动选择 |
| 6 | pluginsheet.go:651 | 下拉空 Label 显示空白 → 回落 Value，仍提交原 Value |
| 7 | pluginsheet.go:326 | Radio 空 Label 是无名分段 → 回落 Value，仍提交原 Value |
| 8 | pluginsheet.go:364 | Checkbox 空 Label 只留无名方框 → 回落 Value，仍提交原 Value |

最终按同类两项分四批：集成/插件焦点、换序/删除锚点、未知下拉值/下拉空标签、Radio/Checkbox 空标签。八项均在原实现深浅实跑变红后修复；旧有 Space 操作正常，只作守卫。

行为红线：窗口高度链、键盘模型、会话管理、保存时机、i18n 文案与命令接线保持。修复只恢复现有控件的身份与可用性；不改未知 Radio 值的既有夹取语义（仍属拍板项）。旧 B 测试零改；V/S 若无变化均记 0。

## 验证

以 `LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4` 运行（AGENT-NOTES 的字体环境要求）；开盘测试函数实测 2735，全量实测 **3648 PASS / 0 FAIL**。初次误用 C locale 的运行已终止，不用它的结果重钉视觉断言。键盘 Space 候选原实现通过，不计修复，替换为删除锚点的边界。

每 1–2 项先跑新增回归与相关旧测试、internal/ci，再 commit + push；全量门槛轮末一次：gofmt -l cmd internal 空、三平台 vet、普通与 verbose go test -count=1 ./...、三平台 build。计数不减。截图由 ui.Tester 导出 `/tmp/floter-P13-he-<场景>-<light|dark>-<before|after>.png`，归档到本轮目录并目视。

mutation：M1 移除选项稳定身份，焦点/键盘 pin 应红；M2 把值锚点退回索引，换序范围 pin 应红；M3 去掉未知值或空标签回落，相应显示与写值 pin 应红。每条保存原件、注入、实跑、还原、md5sum -c、touch 后复跑。

## 设计取用

已读 qiaomu-design SKILL、user-preferences、craft-loop、preflight、chinese-typography，Apple DESIGN 作为供体：单一 focus 色 #0071e3 映射本仓 Focus/Accent；#f5f5f7/#1d1d1f 映射现有 #F7F7F8/#0E0E10；980px 胶囊映射现有 999px。不改语义 token。

工艺菜单取 #3 焦点环（一个当前对象，现有 2px 环）、#4 交互（键盘反馈 1 帧，新增动画 0ms）、#6 编辑细节（原有 1px 分隔线）、#9 滚动（多选 66px 视口，单一内滚动）、#12 材质（E1 受光边 1px，新增影 0）。后三项是保留与复核，不冒充新增修复。

为什么更轻、更有空间感：焦点和勾选只占据正确选项的空间，空白控制变成有明确名称的操作；不增加提示层或玻璃层来补救歧义。
