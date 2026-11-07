# P13-jf：文件传输失败的原因与恢复

意图：让现有集成导入/导出与本地脚本导出的失败能够解释原因，并保持原入口可重试。起点 2c61b589；最新 P13-je 的仍待优先于任务书内旧轮号。

设计读取：原生集成设置，面向管理本地工具的用户，克制的轻玻璃；VARIANCE/MOTION/DENSITY=2/1/6。三秒内辨认失败的动作和原因，通过原入口重试或关闭。

## 实施队列（产品改动前的代码位置）

1. shell/sync.go:116：文件读取失败只显示 IntegrationsTransferBad → 显示实际文件系统原因，释放 busy，允许重试。
2. shell/sync.go:69：导出读取仓库失败误报无效导入文件 → 显示实际仓库原因，不能打开保存选择器或写空备份。
3. shell/sync.go:122：文档解析失败丢掉版本/语法诊断 → 保留既有本地化标题，补真实原因。
4. extensions/sync.go:291：ImportSync 忽略 inventory.RepositoryErr → 事件确认误判后返回错误，宿主显示原因；不产生成功/跳过报告、不改磁盘。
5. shell/sync.go:186：脚本保存选择器失败无具体原因 → 保留 LocalToolExportFailed 标题并补原因，取消仍静默。
6. shell/sync.go:195：脚本写磁盘失败无路径与原因 → 同上；重试实际输出内容。
7. shell/sync.go:83：集成文档编码失败误报无效导入 → 损坏 descriptor 实际复现后显示编码原因，目的文件不被写坏。
8. settingsui/transfer.go:108：报告文字 Grow 而无收缩/换行约束 → 用长文件路径与失败原因实帧测量；若越界，约束文本列并换行，保留图标与现有原文。

只计真实复现并完成的项目。跨集成传输、脚本导出与报告组件；一至两项完成后提交推送。

## 红线与门槛

高度链、键盘模型、会话管理、保存时机、i18n 字典、命令接线不变；3000ms/5000ms、180ms/Flush 不变。旧 B pin 零修改，不引依赖、不加系统。错误详情是已有信息的接活。

中文 locale 基线实测 5454 PASS / 0 FAIL / 3237 函数，原始证据 /tmp/floter-P13-jf/baseline.txt。每批运行相关测试及包 vet；结尾运行 gofmt -l cmd internal、三平台 go vet ./...、普通与 verbose go test -count=1 ./...、三平台 go build ./...。PASS/测试函数只增不减。

Mutation 至少两组：M1 恢复集成读错误的通用文案，原因可见 pin 必红；M2 恢复脚本通用标题而去掉详情，脚本原因 pin 必红。注入后记录原始失败，逐字节 MD5 还原、touch，再测试。新增测试先在原产品跑红，导出深浅 before 图。

## 设计借用

已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography、Apple DESIGN 的颜色/字体段。Apple 单一交互蓝用于焦点，迁成本仓现有蓝；强调层 600/正文 400 原值不动，浮层只一层。craft #4：高频重试反馈 0ms 附加动画；#6：标题和原因两级阅读，文本列不越过关闭键；#12：继续 1px 受光边，新增材质层 0。不为五项密度添加装饰；以用户合约为先。
