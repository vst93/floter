# P13-hs：配置动作的确认、恢复与状态归属

意图：把已有配置动作在取消、失败、字段退役与回调返回后的状态接完整。P13-hr 仍待第 1 条明确点名动作生命周期；本轮事件探针已复现取消与失败后 Space 无法重试。保留轻便玻璃表面，让当前操作的焦点和确认状态清楚、连续，减少失去键盘位置造成的空间跳跃。

读取为：现有原生配置浮层，面向键盘与鼠标用户，克制、即时反馈，沿用 liquid-glass。拨盘：视觉冒险 2（不换方向）、动效 1（即时）、密度 6（原尺寸）。功能契约：三秒辨认当前配置与确认/取消，明确操作后才运行，失败能直接重试。

## 落地队列

以下行号取本轮开盘源码。未复现的候选不计修复；同族路径一起覆盖。

| 项 | 位置 | 现状 → 验收目标 |
| --- | --- | --- |
| 1 | settingsui/pluginsheet.go:780–805 | 取消按钮消失后焦点丢失 → 回到同一动作，Space 可再次确认，Run=0 |
| 2 | settingsui/pluginsheet.go:796–802 | 失败句插入改变按钮身份 → 失败后保留焦点，Space 能重试，首次 Run=1 |
| 3 | launcher/config.go:168 | schema 移除动作只关闭旧控件，未退役 Armed/ActionFailed → 重加前清掉旧确认与失败 |
| 4 | launcher/config.go:168 | 动作改为值字段仍留动作状态 → 改型后再变回需重新确认 |
| 5 | launcher/config.go:149 | RunConfig 未核对 Armed → 无确认不执行，拒绝后仍能正常两步执行 |
| 6 | launcher/config.go:129–153 | Arm/Run 未核对当前字段与对应 key → 旧字段回调不夺取或消费另一个动作的确认 |
| 7 | launcher/config.go:102–122 | ConfigChange 返回后写入当前 a.config → 回调关闭/切换时不得崩溃或覆盖另一配置 |
| 8 | launcher/config.go:149–161 | ConfigAction 返回后错误写入当前 a.config → 回调关闭/切换时不得崩溃或污染另一动作 |

## 红线与验证

高度链、键位、两步确认模型、会话管理、设置文件格式、写回时机、i18n 文案与命令接线不改。只修生命周期违背现有契约的路径。V/B/S 旧断言不改、不放宽容差。已有纯视觉值不重定基线。

开盘 `LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4 go test -count=1 -v ./...` 真实基线 3971 PASS / 0 FAIL；函数由 git grep 统计为 2858。每批两项先跑定向事件与 vet 后 commit/push，整轮固定源码后一次全量 gofmt、三平台 vet、普通/verbose test、三平台 build。采用用户小步提交优先规则，全量门槛在最后统一执行。

Mutation：M1 回退取消后的焦点交接，取消 Space pin 必须红；M2 回退失败句前的稳定主按钮 key，失败重试 pin 必须红。后续 owner 改动另跑缺陷回退。每次恢复用 MD5 清单逐字节核验，touch 后 count=1 复跑。ui.Tester 深浅 before/after 均导出 PNG，真实事件重放，不以文档图代替。

Skill：已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography。Apple 供体的单一交互蓝、弱化墨 .48、柔和圆角映射现有 accent、Disabled=.5、RadiusSM=8；不换色值。工艺菜单 #3 焦点只属于一个控件，#4 额外等待 0ms，#12 增加材质层 0。预检字号/行盒旧拍板项如实保留。
