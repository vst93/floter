# P13-hg · 选择控件的动态值、长文与重复项

意图：让现有设置选择控件在数据刷新与异常 schema 下仍准确、可读、可操作。起点 48e06621，干净树，pull 已同步。上一轮的仍待指向菜单动态值；本轮沿这一族扩展到多选列表、插件复选组，至少八项实际复现再修。

## 逐项计划（读当前实现后定位）

1. selectvalue.go:29：只检测列表换序，未检测外部当前值变化 → 打开菜单同步当前值，回车不撤销外部变化。
2. selectvalue.go:63：长菜单宽度实跑原实现已被框架裁切，保留守卫，不计修复；替换候选为动态列表失去高亮与当前值的组合，先复现。
3. config.go:1069：多选标签点击只切值，不传焦点 → 标签与勾选框指向同一个键盘目标。
4. config.go:1070：多选项未 SingleLine → 长名字保持一行和勾选框对齐。
5. config.go:683：重复字符串生成同键多选行，tickedToValues:1091 重复输出 → 每个真实值只出现并保存一次。
6. pluginsheet.go:358：重复 Value 生成同键复选框，提交重复 ID → 每个 ID 一个控件、一个提交值。
7. selectvalue.go:78：同 ID 重复选项画成多个相同操作 → 菜单稳定保留首个显示名。
8. pluginsheet.go:268、pluginOptionLabel：值和名称同时为空时控件没有可见名字 → 用已有本地化 ConfigNone 表达合法空值，仍提交原空 ID。

八条都是待实跑复现的独立验收，不把正常行为当修复。每两项完成即提交推送；若复现不成立，替换规格并记原因。

## 合约与设计

高度链、会话管理、设置写回时机、键盘按键集合和 i18n 词条不改；修复错误的选择归属不改变既有选择语义。维持 66px 多选箱、28px 配置字段地板、原 token 和单层浮面。无依赖、无新页面。

qiaomu-design 打磨路径，读取 SKILL、user-preferences、craft-loop、preflight、chinese-typography。Apple DNA：#0071e3 的单一交互色角色沿用 Accent；#f5f5f7/#1d1d1f 映射本仓近白/近黑；980px 胶囊沿用已有 999px。工艺菜单 3=2px focus、4=0ms 键盘反馈、6=1px 发丝边及单行编辑、9=单滚动区、12=1px 顶内受光边。项目 liquid-glass 为唯一锚，用户已指定方向，不做网页预览或外部模型。

## 门槛

统一 LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4。开盘全量 verbose 实测 3702 PASS / 0 FAIL，函数 2755。

每批跑新增与相关旧回归再提交。轮末一次 gofmt -l cmd internal（空）、三平台 go vet ./...、普通及 verbose go test -count=1 ./...、三平台 go build ./...；原始输出归档。V/B/S 旧断言预计 0/0/0。

## 反证与图

新增行为 pin 在原实现先红；ui.Tester 深浅 before/after PNG。至少两条正式 mutation：撤掉外部值同步，回车值 pin 红；撤掉标签 Focus，Space 归属 pin 红。每条备份、注入、实跑红、还原、md5sum -c、touch 后复跑。图看整体和局部，逐条报告现象→改动→验证。
