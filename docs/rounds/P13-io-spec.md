# P13-io · 编辑状态与键盘边界

开盘 bb8c6567，2026-11-07 04:18 UTC；pull 最新、工作树干净。上一轮实际为 P13-in，按其“仍待”继续检查命令整行忙态及未写回别名，不重做 P13-hw。

意图：刷新与短暂锁定后，输入仍显示用户刚写的内容，键盘仍有明确归处。读取为现有设置功能页，面向键盘用户，沿轻便液态玻璃；视觉冒险 2/10（稳定）、动效 1/10（即时）、密度 6/10（保留功能布局）。3 秒任务是辨认当前编辑位置并继续输入或退出。

## 逐条候选与证据

以下行号为开盘源码实读；仅实际事件变红的条目计入交付。

1. `settingsui.go:3049`：命令持焦板只检查存在，没有检查整行忙态。别名锁定后应归还既有 Close。
2. `settingsui.go:3639`：命令开关仅 Disabled；忙态后应有相同键盘出口，解锁不抢焦。
3. `settingsui.go:3664`：别名每帧直接从 command.Alias 取值；180ms 未写回窗口里连续输入和普通重绘应保留完整草稿。
4. `settingsui.go:3664` / `debounce.go:45`：命令短暂撤回/重新声明时，新输入节点应从已有待写队列恢复草稿；空字符串也必须恢复，保存时机不改。
5. `textinput.go:187`：别名绕过通用字段的上下方向键保护；编辑时 ↑/↓ 应留在本页。
6. `textinput.go:187`：同一路径遗漏用户 new-command 快捷键；别名内应遵守现有关闭设置契约。
7. `parameditor.go:213`：参数选项 TextArea 没有通用字段的快捷键处理；既有快捷键应先于文本输入，普通多行编辑保留。
8. `scripteditor.go:88`：脚本编辑器的同类快捷键先做宿主集成事件复现；若已正确则不计修复，另选已存在编辑路径。

## 红线与验收

高度链、键盘既有模型、会话管理、设置写回、i18n 与命令接线不改。仅修遗漏接线与显示状态；180ms debounce、Flush、3 秒确认、终端语义全部保留。无新页面/依赖/系统。旧 V/B/S 断言零改，新增事件 pin。

基线完整 verbose 实测 **4888 PASS / 0 FAIL / 3068 测试函数**。每两项先跑相关事件、旧 B 与包 vet，过后立即 commit/push；最终集中执行 `gofmt -l cmd internal`、三平台 `go vet ./...`、普通与 verbose `go test -count=1 ./...`、三平台 `go build ./...`，记录真实 PASS 与测试函数数。

## Mutation 与图

- M1 回注存在即保留的命令焦点判断，别名/开关忙态出口 pin 应红。
- M2 回注只读 command.Alias，连续输入/重新声明恢复 pin 应红。
- M3 回注别名原始 TextInput 路径，上下键及用户快捷键 pin 应红。
- M4 回注多行编辑旧接线，对应真实宿主快捷键 pin 应红。

每组实跑红 → 恢复 → md5sum -c → touch → 复验，原串保留。ui.Tester 在中文 locale 导出深浅 before/after PNG；图存 `/tmp/floter-P13-io-*` 并归档仓库。Linux 自绘事件是证据，Windows/macOS 只承诺 vet/build。

## 设计依据

真实读取 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN 的色彩/交互角色。从 craft-loop #3 取单一品牌 focus 环（沿本仓 2px、恢复 0ms），#6 保留 1px 发丝线与编辑内容，#12 保留 8px 板圆角、6px 别名角及原面差；新增材质/阴影 0 层。用户已定方向，不做四方案或 HTML 预览。为什么更轻：用原有焦点环与输入内容明确状态，不靠常驻解释条增加视觉重量。
