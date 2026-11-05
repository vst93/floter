# P13-hd · 配置表单的输入、撤销与错误归属

意图：schema 与页面状态变化时，用户的输入和反馈仍属于原字段。接续 P13-hc「仍待」第 2 条；起点 c8ee659e，pull --ff-only 同步、工作树空。禁用/只读族已在后续轮次收口，不重复旧任务书的已完成选题。

读取为原生配置表单，面向配置插件的用户，沿用轻便液态玻璃。视觉冒险度 2（可预期），动效强度 2（输入即时），信息密度 6（现有密度）。三秒内辨认当前字段、错误和保存入口；能编辑、纠错、提交和关闭。

## 开工规格（代码行号为起点实读）

| 项 | 位置 | 现状 → 验收目标 |
|---|---|---|
| 1 | config.go:213、559 | 字段按 schema 次序构建 → 重排后键盘仍编辑原字段 |
| 2 | config.go:827 | 错误储存翻译后的句子 → 切换语言时错误用当前语言 |
| 3 | config.go:205 | 表单复用同一个控件身份 → 换集成不继承撤销历史 |
| 4 | config.go:205；settingsui.go:3216 | 关闭丢弃草稿但 UI 历史仍在 → 重开不撤销回已丢弃内容 |
| 5 | pluginsheet.go:242、251 | 插件字段按位置构建 → 重排后未提交输入保持原字段 |
| 6 | pluginsheet.go:101、251 | 不同插件共享控件身份 → 撤销不恢复另一插件的值 |
| 7 | rows.go:1731 | stackedRow 首行分隔线改变主体次序 → 删除前行后当前输入仍有效 |
| 8 | pluginsheet.go:229、241 | 分组标题有无改变卡片次序 → 标题增减不丢当前输入 |

每项必须先真实键盘复现红测；不复现则替换同族实际问题，不计完成。本轮跨集成配置、插件配置、堆叠字段组件。

## 红线、门槛与 mutation

高度链、窗口键盘模型、会话管理、设置写回协议、i18n 文案和命令接线保持。本轮修已有控件的对象归属，不新增页面、系统、依赖。旧 V/B/S pin 不删、不放宽、不 Skip。

基线：LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4 verbose 全量实测 **3624 PASS / 0 FAIL**，测试函数 **2727**，日志 /tmp/floter-P13-hd/baseline.log。每 1–2 项相关回归通过即 commit && push；轮末一次 gofmt -l cmd internal、三平台 go vet ./...、普通及 verbose go test -count=1 ./...、三平台 go build ./...。

Mutation：M1 撤销字段身份，使重排输入 pin 红；M2 撤销表单会话身份，使跨对象/重开撤销 pin 红；M3 撤销 stackedRow 主体身份，使首行边界 pin 红。每条实跑、还原、md5sum -c、touch 后复跑。

截图由 ui.Tester 导出 /tmp/floter-P13-hd-<场景>-<light|dark>-<before|after>.png，深浅前后与放大图入库。更轻/更有空间感来自焦点和错误只落到所属位置，保留现有面差、不额外叠玻璃。

## skill 转译

已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography。Apple 供体取单一交互强调色、近黑/近白中性底、胶囊按钮三条，映射到本仓既有 Accent/Background/999px 半径；不引入营销站字号与阴影。craft-loop #3 焦点归属、#4 即时反馈、#6 1px hairline 与字段节奏、#9 既有独立滚动、#12 一条受光边用于本轮审稿；实际新修项与沿用值分别报告。
