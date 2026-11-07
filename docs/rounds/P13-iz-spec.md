# P13-iz：快捷键编辑的拒绝归属与可触达性

意图：保住正在编辑的对象、拒绝说明与键盘出口；沿 P13-iy 最新交接实测，不重复旧 P13-hw。开盘 HEAD 038f2cb6，11:40 UTC，工作树干净、pull 已最新。

设计读取：原生快捷键设置，面向高频键盘用户，沿轻便液态玻璃；拨盘 3/2/7。三秒内应能辨认当前键、动作及失败原因；现有录制、选择、提交、删除均应可达。

候选位置（读过当前代码后列出，只有真实事件变红才计修复）：

1. `internal/settingsui/custom.go:380`：Add 无条件清 local rejection → 只清自己的草稿拒绝。
2. `internal/settingsui/custom.go:174`：另一保存行开始录制清原行拒绝 → 只清本行的拒绝。
3. `internal/settingsui/custom.go:215`：邻行成功录键清原拒绝 → 保持拒绝归属。
4. `internal/settingsui/custom.go:389`：取消草稿后系统拒绝残留 → 核对重开同键是否错误继承。
5. `internal/settingsui/custom.go:125`：录制 prompt 在窄列无收缩 → 测大字号与邻按钮遮挡。
6. `internal/settingsui/custom.go:323`：保存行警告无明确换行 → 测警告边界。
7. `internal/settingsui/custom.go:373`：草稿警告同族 → 测警告与 Bind 的空间归属。
8. `internal/settingsui/settingsui.go:4237`：内置录制框 → 测满提示与窄控件列。

宿主 `internal/shell/custom.go:38/97` 只保存成功项；若修复要求保留拒绝的保存绑定或新增恢复草稿，列需拍板，不更改该契约。初选若不成立，继续同一设置表面的实际边界问题。

红线：高度链、键盘模型、3000/5000ms、180ms/Flush、会话管理、设置写回时机、i18n 字符串、命令接线不改。修现有状态归属与溢出；旧 B 断言零改。

门槛：中文 locale，gofmt 空；三平台 vet/build；普通和 verbose 全量；测试函数不减。基线实测 **5274 PASS / 0 FAIL / 3183 函数**。每批定向回归/vet 后 commit/push；完整门槛本轮集中一次。

Mutation：M1 撤 Add/邻行录制的归属保护，事件 pin 应丢失原拒绝；M2 撤几何收缩/换行，邻控件或内容边界 pin 应红。每组红后 MD5 恢复、touch、-count=1 复验。

Skill：已读 qiaomu-design、user-preferences、craft-loop、preflight、chinese-typography 与 Apple DESIGN。从 Apple 取单蓝交互角色、2px focus、8px 圆角三项，沿本仓 token。菜单 #3 焦点、#4 即时交接 0ms、#6 编辑细节、#12 既有 1px 受光边；不为凑数添加材质。失效/错位说明退场，焦点与控件各有归属，所以更轻且空间关系更清楚。
