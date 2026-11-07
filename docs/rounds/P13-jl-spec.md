# P13-jl · 同步读取与配置刷新交错

意图：让设置页首读、单行补读期间已确认的配置和工具状态在当前帧保持有效。起点 087bee9f，19:28 UTC 开盘；最新 P13-jk 的“仍待”优先于任务书旧进度。Codex 独立执行。

读取为：桌面设置表单，服务已安装集成的用户，沿用克制的轻便液态玻璃。视觉冒险 2/10（保留布局）、动效 1/10（即时反馈）、密度 6/10（保留原字段与状态）。功能契约：三秒内能辨认当前配置及工具可用性，能继续编辑和运行；刷新不恢复过期警告。

## 逐条复现与落地计划

| 项 | 位置（开盘实读） | 现状与预期 |
| --- | --- | --- |
| 1 | internal/shell/configmemo.go:183 | 同步首读覆盖已导入地址 → 只发布读取时仍有效的结果，失效后重取当前值 |
| 2 | internal/shell/configmemo.go:183 | 同步读取失败覆盖刚保存的字段 → 保留保存后的实际值与可复制编辑内容 |
| 3 | internal/shell/configmemo.go:164 | 补读新行时合并的旧表覆盖另一行已保存值 → 合并基底必须仍属于当前代 |
| 4 | internal/shell/configmemo.go:164 | 补读自己的旧值覆盖已导入新值 → 补读结果也检查所属代 |
| 5 | internal/shell/integrationfacts.go:231 | 同步探测失败覆盖刷新确认的运行时 → 当前帧不再显示过期不可用 |
| 6 | internal/shell/integrationfacts.go:231 | 同步旧摘要复活批准警告 → 当前批准继续匹配当前摘要 |
| 7 | internal/shell/integrationfacts.go:218 | 补读新行时旧基底恢复另一工具的不可用 → 保留刷新后的原行状态 |
| 8 | internal/shell/integrationfacts.go:218 | 补读自己的旧摘要覆盖新批准 → 退役旧摘要，批准区域不回退 |

以上先用真实 App、provider、磁盘与 ui.Tester 复现，未红项目不计完成；读 I/O 的返回顺序由既有 compute 参数控制，不新增产品测试钩子。每两项单独提交推送，四批完成。

## 边界与设计依据

行为契约零改：高度链、键盘模型、会话管理、设置写回时机、i18n、命令接线；保留 2s TTL、3s 确认、既有草稿/忙态规则。不添加系统、依赖或页面。只修可见刷新错误，不做内部一致性清账。

qiaomu-design 已读 SKILL、偏好账本、craft-loop、preflight、chinese-typography。用户已定方向，直接打磨。Apple DESIGN 借用蓝色仅用于交互的角色、8px 控件圆角、2px focus、0.22 浅色外影；本仓对应 token 沿用，不换成 Apple 色值。craft-loop #4：反馈新增延时0ms；#6：无效状态条退役；#12：原1px受光边、额外玻璃0层；#5：浮面原两层阴影。少装饰优先于凑满五项。

## 门槛与 mutation

基线全量 verbose 实测5654 PASS / 0 FAIL，退出0；起点087bee9f树实测3303函数。每批定向事件和包 vet 通过再提交；全量门槛最终统一运行：gofmt -l cmd internal；go vet ./...；GOOS=windows/darwin go vet ./...；go test -count=1 ./...；verbose PASS计数；linux/windows/darwin go build ./...。正式测试统一 LC_ALL/LANG=zh_CN.UTF-8。

M1：撤回配置首读/补读代际保护，应让导入地址、保存字段及邻行合并事件变红。M2：撤回事实首读/补读保护，应让运行时与批准警告事件变红。每条实跑失败、逐字节 MD5 还原、touch 后重跑。旧 V/B/S 测试不改，不新增 Skip。深浅 before/after 用 ui.Tester 导出、入库并目检。
