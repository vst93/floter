# P13-jk · 刷新后的配置与状态不倒退

意图：让已打开的配置表单与集成状态忠实呈现最新落盘结果。P13-jj 的仍待要求验证导入后配置刷新；本轮从真实导入/保存和可控后台返回顺序复现，不改变草稿所有权。

读取为：面向桌面工具用户的设置状态页，沿用轻便液态玻璃。视觉冒险度 2（保留布局）、动效 1（即时状态）、密度 6（原紧凑表单）。三秒内辨认当前保存值、当前可用状态与真正需要处理的错误。

## 落地计划（复现成立才计完成）

| 项 | 动手前位置 | 现状 → 应改成 |
| --- | --- | --- |
| 1 | internal/shell/configmemo.go:169 | 旧后台读取可覆盖导入后的配置 → 已失效结果不再发布 |
| 2 | internal/shell/configmemo.go:169 | 旧读取失败可清空新保存字段 → 新保存不被失败快照覆盖 |
| 3 | internal/shell/integrationfacts.go:237 | 旧运行时探测可盖住修复后状态 → 最新刷新拥有状态 |
| 4 | internal/shell/integrationfacts.go:237 | 旧摘要可盖住重新批准的状态 → 已退役探测不能复活告警 |
| 5 | internal/settingsui/config.go:178 | 外部值满足当前草稿后旧保存错误仍留 → 仅确认已保存的当前表单撤旧错 |
| 6 | internal/settingsui/config.go:229 | 已验证的数字被外部无效值替换后提示被清掉 → 按现有边界重新验证 |
| 7 | internal/settingsui/config.go:229 | 已验证的文字被外部过长值替换后提示被清掉 → 按现有长度重新验证 |
| 8 | internal/settingsui/config.go:194 | 多选草稿被外部保存确认后旧范围锚点仍在 → 外部接受结束旧 Shift 手势 |

八项跨配置表单、集成运行/批准状态两个组件，分四批每两项提交推送。若复现不成立，记录并用同族真实缺陷替换。

红线：不改高度链、键盘模型、会话管理、保存时机、i18n 文案、命令接线、通知时长、TTL、保存中草稿规则。只修既有刷新结果的归属与显示。

## 验收

开盘 HEAD 8213e4d2，工作区干净。全量 verbose 基线运行于 /tmp/floter-P13-jk/baseline.txt，实测数字完成后补录，不引用历史数字冒充。

基线完成：5627 PASS / 0 FAIL，退出 0；3294 个测试函数。

每批：新增事件 pin 修前红、修后相关回归与包 vet。收尾一次运行 gofmt -l cmd internal；三平台 go vet ./...；普通与 verbose go test -count=1 ./...；三平台 go build ./...；函数数不下降、旧 B 断言不改。统一 LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4。

Mutation M1：撤销配置缓存过期结果保护，导入/保存事件 pin 应红。M2：撤销表单外部更新的错误/锚点交接，相关真实输入 pin 应红。逐次备份、实跑、还原、md5sum -c、touch 再验。

## 设计取值

qiaomu-design 已有方向打磨路径，读偏好账本、craft-loop、preflight、chinese-typography。Apple DESIGN 的中性文字层级、8px 控件圆角和阴影 alpha 0.22，映射并沿用本仓 TextStrong/TextSecondary、RadiusSM=8、浅色 ShadowAmbient=0.22；不覆盖既有 token。工艺菜单 #4 状态交接新增延时 0ms、#6 过期错误条 1→0、#12 既有 1px 受光边；#5 原两层浮影保留。少装饰优先于凑满五种效果。

两轮：先深浅真实 ui.Tester 图定位回退和旧错误；再复核最新值、失败保留、焦点与 Shift 手势。按 preflight A/B/C/D2 逐条记录限制。
