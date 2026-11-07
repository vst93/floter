# P13-jm · 当前工具声明与状态归属

意图：已安装工具同 ID 的声明或健康记录刷新后，运行时徽标与 freshness 详情立即对应当前对象。起点 022e27d7，2026-11-07 19:52 UTC。当前工作树干净，唯一执行者 Codex。

读取为：已安装集成的桌面管理表面，沿用克制的轻便液态玻璃。视觉冒险 2/10（保留布局）、动效 1/10（即时反馈）、密度 6/10（保留信息层级）。三秒内必须看清当前可用性和最近检查结果；现有操作入口继续可用。

## 逐条计划

开盘实读：internal/shell/integrationfacts.go:100 的身份只包括可执行路径、manifest 路径和批准时间；shell.go:2886 单行查询未核对当前对象。真实写盘、Store.Refresh、ui.Tester 先红后绿，未红不计完成。

| 项 | 位置 | 现状 → 预期 |
| --- | --- | --- |
| 1 | integrationfacts.go:100 | 同路径 runtime.type 更新仍沿用旧可用徽标 → 纳入当前类型 |
| 2 | integrationfacts.go:100 | 解释器语言改变仍显示旧可用状态 → 纳入当前语言 |
| 3 | integrationfacts.go:100 | 系统程序候选名更新仍显示旧运行入口 → 纳入候选名 |
| 4 | integrationfacts.go:100 | 脚本路径变为越界后仍显示可用 → 纳入当前脚本路径 |
| 5 | integrationfacts.go:100 | 已刷新健康报告的结果仍是旧成功 → 纳入报告状态 |
| 6 | integrationfacts.go:100 | 最新检查时间仍显示旧记录 → 纳入报告时间 |
| 7 | shell.go:2886 | 单行同 ID 换包路径后继续显示旧命令数 → 命中前核对当前身份 |
| 8 | shell.go:2886 | 单行健康报告更新后旧结果常驻 → 命中前核对当前报告 |

每两项定向事件和包 vet 过后立即 commit/push，完整门槛在最终产品集中运行一次。跨运行时徽标/操作入口与 freshness 详情组件，不新增系统、页面、依赖。

## 红线与设计依据

高度链、键盘模型、会话管理、设置写回、i18n、命令接线均不改；2s TTL、3s/5s 确认窗口、180ms 保存时机沿用。只修当前声明已刷新却画旧事实的可见错误，不改后台探测频率。

已读 qiaomu-design 的 SKILL、全部偏好账本、craft-loop、preflight、chinese-typography。用户固定方向高于四方向选择；无 HTML 预览和外部执行器。Apple DESIGN 提取交互专用 accent、8px 控件圆角、2px focus、浅色 0.22 外影，均映射本仓已有 token。craft-loop #4：当前刷新额外延迟 0ms；#6：矛盾状态 1→0；#12：1px 受光边与新增玻璃 0层；#5：浮层原2层投影，面内额外投影0层。修 bug 优先于凑装饰。

## 验收

基线全量 verbose 已启动；数字按实际完成日志补录，不引用旧报告冒充实测。环境 LC_ALL=zh_CN.UTF-8、GOMAXPROCS=4。门槛：gofmt -l cmd internal 空；三平台 go vet ./...；普通 go test -count=1 ./...；verbose PASS 数；linux/windows/darwin go build ./...。

M1 删除运行时/健康声明的身份项，预期前六条变红；M2 恢复单行无条件使用旧 ID 的命中路径，预期后两条变红。每条独立注入、实红、还原、md5sum -c、touch 复验。旧 V/B/S 断言不改，无新增 Skip。深浅前后 PNG 必须入库并目检。
