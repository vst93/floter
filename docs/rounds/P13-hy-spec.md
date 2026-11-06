# P13-hy：异步结果与条件刷新后的键盘恢复

起点 405191bc；pull 已最新、工作树干净。基线实测 4218 PASS / 0 FAIL / 2913 测试函数。

意图：让设置里的失败重试和条件变化都有可见、可继续操作的键盘出口。上一轮仍待第 1 项支持选题，八项已由真实 Click / Tab / Space、延迟回调与 View 刷新复现（before.log）。下表行号为开盘读取值，规格落盘前已完成原型实验，正式提交按两项分批。

设计读取：原生集成设置面，面向维护工具与配置的用户，沿用轻便液态玻璃；视觉变化 3 / 动效 2 / 密度 7，即克制、即时、保持原密度。功能契约：当前状态可读，保存失败可原位重试，无效确认不截留键盘。

| 项 | 开盘位置 | 现状 → 应改成 |
| --- | --- | --- |
| 1 | config.go:369、375 | 失败带插在动作行前导致 Save 身份改变，Space 无响应 → 动作行身份跨提示插入稳定 |
| 2 | config.go:543 | 再次保存时旧错误仍占据表单 → 新尝试开始撤旧错误，新失败正常显示 |
| 3 | settingsui.go:3150、3173 | 停用后目标 Broken，原 switch 不可用，失败撤带后焦点丢失 → 当前可用的 Check 接焦点 |
| 4 | settingsui.go:2984 | 停用中目标 Orphan，提前返回让答案消失 → 同行 Adopt 接原答案焦点 |
| 5 | settingsui.go:2984 | 移除中目标 Orphan，答案消失 → 同行 Adopt 接原答案焦点 |
| 6 | settingsui.go:6996 | plain removal 在飞时变 npm，!componentized 解锁答案 → busy 始终锁第二次确认 |
| 7 | settingsui.go:2984、uninstall.go:66 | 卸载中变 Orphan，浮层消失且旧 opener 不存在 → Adopt 接浮层内焦点 |
| 8 | uninstall.go:66、settingsui.go:3175 | 卸载失败时已变普通链接，浮层退役后焦点悬空 → 当前 Remove package 接焦点 |

跨配置动作、停用带、移除带、组件卸载四组件。每两项独立提交推送。不改变全局键盘模型、保存时机、保存期间草稿归属、3 秒窗口、会话管理、高度链、i18n 或命令执行语义。焦点已在其他控件时不抢回。

已读 qiaomu-design SKILL、user-preferences、craft-loop、preflight、chinese-typography 与 Apple DESIGN。取单一 accent 焦点角色、2px focus 环、8px 控件圆角与 1px 边；不移植网页纯黑/字号/负字距。菜单 #3 对应 2px 环恢复，#4 对应 0ms 状态反馈与单次在飞提交，#12 对应既有 E1/E2 层不增包装。

门槛：gofmt -l cmd internal 空；三平台 go vet ./... 与 go build ./...；普通与 verbose go test -count=1 ./...；统计 PASS 与 func Test 不下降。LC_ALL=zh_CN.UTF-8、GOMAXPROCS=4。每批相关测试与 vet，完整门槛本轮一次。

mutation：M1 回注旧 configActions 身份/旧错误保留；M2 去掉停用 fallback；M3 回注 remove busy 与 orphan 焦点旧行为；M4 去掉卸载 panel 焦点交接。各对应两项事件变红。每组备份、注入、实跑、还原、md5sum -c、touch、复跑，旧 V/B/S 断言零改。

ui.Tester 720×900 深浅 before/after 放 /tmp/floter-P13-hy-*，复制进轮次目录。更轻与空间感：旧错误退场不再重复占据表单，焦点环指出当前可操作层。
