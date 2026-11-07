# P13-jh · 配置传输的文本与备份完整性

意图：已存在的复制、导出入口遇到损坏内容时解释原因、保住旧备份，修正内容后能原路重试。起点 ac396892；工作树干净，pull ff-only 已完成。依据最新 P13-jg 仍待，旧 P13-hw 条件刷新已被后续轮次处理。

读取为：集成设置中的工具配置工具条与全局传输组件，面向管理本机工具的用户，沿用轻便液态玻璃。视觉变化幅度 2/10（沿用布局）、动效 1/10（即时反馈）、密度 6/10（原紧凑页面）。功能契约：能分辨成功与拒绝、知道坏在哪个文件、修复后重试。

## 候选与真实位置（先事件复现再落地）

1. extensions/config.go:590 ExportConfiguration：JSON 静默替换非法文本 → 拒绝复制/导出损坏值；密码仍脱敏。
2. extensions/config.go:142 parseStoredConfiguration / sync.go:510 loadRawConfiguration：损坏 UTF-8 配置被重编码 → 原始读取拒绝并携带文件路径。
3. extensions/sync.go:156 exportIntegration：有登记包却 manifest 读坏时静默省略包 → 拒绝不完整备份；无包的兼容记录保留。
4. extensions/sync.go:165：空脚本导出成功而导入要求非空 → 导出拒绝不可恢复内容。
5. 同处：非 UTF-8 脚本被 JSON 改写 → 导出前拒绝并定位文件。
6. extensions/sync.go:175：空 descriptor 被 omitempty 删掉 → 拒绝不完整备份。
7. 同处：非法 UTF-8 descriptor 导出的 JSON 自己无法导入 → 拒绝并定位文件。
8. extensions/sync.go:245 Marshal：导出大于读取上限 4 MiB → 写入前拒绝，包含最终换行，保住现有备份。

每项均用真实磁盘或已接线的 App.Actions 复现；跨配置工具条、集成传输组件及既有 script/static descriptor 包。相同入口的深浅主题与复制/导出不重复计数。未复现候选不计完成。

## 红线与门槛

不改窗口高度链、键盘模型、会话管理、保存时机、格式、确认时限、i18n 既有文案或命令接线；只纠正损坏内容的反馈与备份写入。未知字段/缺包兼容保留；不增依赖/系统/设置，不作架构重构。

基线 HEAD 测试函数实数 3258；中文 locale 全量 verbose 已实测 5513 PASS / 0 FAIL、退出 0。四批每两项跑相关事件/原行为回归及包 vet 后提交推送；轮末只跑一次完整门槛，失败按证据修复后必要复验。

`LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4`：gofmt -l cmd internal 为空；三平台 go vet ./...；普通与 verbose go test -count=1 ./...；三平台 go build ./...。不减少旧函数，不改旧 B pin，不增 Skip。

Mutation：M1 撤销配置非法文本检查，应使复制/导出事件把错误内容送出而红；M2 撤销空必需文件检查，应使空 script/descriptor 覆盖旧备份而红。每条独立备份、实红、还原、md5sum -c、touch 后复跑。

## 设计选择

采用 qiaomu-design 打磨路径。craft-loop #6 用原错误文本层级指出内容/路径；#4 结果新增等待 0ms；#5/#12 复用现有 toast 两层影和 1px 受光边，新增容器 0。Apple DNA 取单一交互 accent、中性状态墨与 1px 边的角色，映射到本仓 token，不抄站点字号。用户指定方向与原生 Go 合约优先于网页预览、5项装饰或更换字体要求。深浅前后图 ui.Tester 导出，第二轮复核可读性与恢复。
