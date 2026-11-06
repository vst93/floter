# P13-hw：确认在刷新与忙态下仍属于原对象

意图：修复既有确认的到期调度与列表刷新边界，让过时问题及时退场、留下的问题和键盘仍属于原集成。P13-hv 的“仍待”是本轮证据入口；不改 3000ms 窗口。

设计读取：原生集成设置，面向管理本地工具的用户，沿用克制的轻便液态玻璃。拨盘 3/2/7（低视觉变化、即时反馈、原信息密度）。三秒内辨认当前工具、正在回答的问题与可用动作；取消不派发破坏性调用。

## 逐项计划（先事件复现，红了才计入）

| 项 | 位置（开盘行号） | 现状线索 → 应有行为 |
| --- | --- | --- |
| 1 | settingsui.go:6805–6816 | 停用忙态的旧时间参与 next → 其他问题仍按自己的期限唤醒 |
| 2 | settingsui.go:6817–6828 | 移除忙态同族 → 同上 |
| 3 | settingsui.go:1159,6917 | 集成消失仍留停用确认 → 不吞 Escape、不在重现时复活 |
| 4 | settingsui.go:1159,6922 | 集成消失仍留移除确认 → 同上 |
| 5 | settingsui.go:2890,6913 | 集成消失留下丢弃确认 → 只撤问题，保留草稿与保存语义 |
| 6 | settingsui.go:6928 / uninstall.go:65 | 集成消失留下卸载浮层的 open → 不复活旧浮层、不吞 Escape |
| 7 | settingsui.go:1912,2745 | 安装列表按位置构建 → 刷新重排后停用答案保持目标与焦点 |
| 8 | settingsui.go:1912,2745 | 同族移除答案 → 刷新重排后只对原目标派发 |

每完成两项跑定向事件回归与包 vet 后立即 commit/push；全量门槛在产品定稿后集中一次。新增测试必须通过真实 Click/Tab/Space、刷新与 Tester 的调度时钟复现；不改旧 B 断言，不减少测试函数。

## 红线与设计取舍

高度链、键盘模型、终端会话、设置写回、i18n 与命令接线不变。只恢复现有确认的归属与生命周期；不新增系统/页面/依赖，不动正在执行的操作，不把删除确认等同删除草稿。未知的保存归属继续待拍板。

已读 qiaomu-design SKILL、user-preferences、craft-loop、preflight、chinese-typography；既定方向不走四方向预览。Apple DESIGN 提取三个 DNA：单一交互蓝角色、2px focus 环、8px 控件圆角，映射现有 token 而不换配色。菜单 #3 focus 对象保持、#4 取消 0 次派发/3000ms 现有回答期限、#6 去掉失效问题的编辑噪声、#12 原 1px 受光边与 E1/E2 材质；新增装饰层为 0。

## 门槛与变异

中文 locale 与 GOMAXPROCS=4 统一；开盘全量在运行，实测结束即补基线。函数数开盘 2896。

`gofmt -l cmd internal` 空；linux/windows/darwin `go vet ./...` 净；普通与 verbose `go test -count=1 ./...` 全绿；三平台 `go build ./...`。PASS 不下降。

M1：回注忙态参与最早期限的旧分支，两个真实静止页面测试应报 later discard missed its deadline。M2：撤销不存在目标的确认清理，刷新/重现测试应报 stale question revived / invisible question swallowed Escape。M3：撤销行 key，重排测试应报 answer lost its owner 或回调 ID 不符。每条红后恢复原文件、md5sum -c、touch 后复验。

Linux ui.Tester 导出深浅 before/after 至 /tmp/floter-P13-hw-*.png，并归档至 docs/rounds/P13-hw；目视检查整页和焦点局部。macOS/Windows 仅编译，明确限制。
