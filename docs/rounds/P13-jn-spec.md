# P13-jn · 诊断结果的完整呈现与当前归属

意图：让已有诊断网格、健康摘要与完成反馈只讲当前工具的完整结果。起点 8a4a7dc1；最新 P13-jm「仍待」指向 diagnoses 写回链。Codex 独立执行。

读取为：面向工具使用者的集成设置，沿用轻便液态玻璃与紧凑功能布局。视觉变化幅度2/10、动效1/10、信息密度6/10。三秒内应能分清诊断结论、逐项检查和持久健康记录。

## 逐项计划（真实代码位置）

1. shell.go:1848：诊断只投影摘要，提供者总状态丢失 → 接出 DiagnosisStatus。
2. shell.go:1848：DiagnosisChecks 已计算却没接给网格 → 接出完整有序检查。
3. shell.go:1849：诊断开始时的 Health 覆盖最新报告 → 健康块只取当前持久报告。
4. shell.go:2005：Health 译文缓存于诊断，切换语言不更新 → 用当前语言投影健康。
5. shell.go:1847：同 ID 换包沿用旧诊断 → 已完成诊断绑定包身份。
6. shell.go:2021：检查期间声明刷新，迟到结果写入新工具 → 写回核对当前声明。
7. shell.go:2021：同一工具连续检查，较旧结果覆盖新结果 → 请求顺序限定结果归属。
8. shell.go:2023：已过期检查仍发送完成反馈 → 只为当前有效结果通知。

计划5–8仍须事件证实，不把代码线索当完成。跨诊断网格、健康摘要、宿主反馈三个组件；每两项定向回归和 vet 通过即提交推送。

## 红线

不改高度链、键盘模型、会话语义、设置写回、i18n文案与命令接线；3s/5s确认、180ms保存、2s TTL不变。不开系统/页面/依赖。已有总状态与检查行接活，不另做诊断页面。

## 设计依据

实读 qiaomu-design/SKILL.md、user-preferences、craft-loop、preflight、chinese-typography。已有方向，直接打磨。Apple DESIGN 的交互专用 accent、8px控件圆角、2px focus 映射既有 token，保留浅色0.22环境影角色。craft-loop #4：当前结果额外延迟0ms；#6：过期状态/重复反馈1→0；#12：1px受光边和新增玻璃0层；#5：浮层原2层投影、诊断行外影0层。不为凑数新增装饰。

## 验收

中文 locale LC_ALL=zh_CN.UTF-8，GOMAXPROCS=4；起点3319测试函数。基线全量verbose正在执行，只在完成后填数字。
最终运行 gofmt -l cmd internal；三平台 go vet ./...；普通与verbose go test -count=1 ./...；三平台 go build ./...。旧V/B/S断言零改，无新增Skip。

Mutation：M1撤回两项网格投影；M2恢复旧Health覆盖；M3撤掉声明归属检查；M4撤掉请求顺序/反馈保护。每组真实注入→红→恢复→md5sum -c→touch→复验。深浅真实ui.Tester前后PNG入库，目检。
