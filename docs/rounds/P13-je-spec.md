# P13-je：配置刷新后的动作有效性

意图：在原有配置表单与宿主动作之间补齐条件刷新后的拒绝与恢复，避免静默成功、崩溃和越权写入。起点 353032fa，工作树干净。最新队列为 P13-jd 的 provider 刷新线索；P13-hw 的确认条件问题已由后续轮完成，不重复。

开盘基线实跑 5407 PASS / 0 FAIL；cmd/internal 3226 测试函数（全仓另有 2 个，不混用）。中文 locale。8 条旧 stash SHA 与 P13-jd 一致，reverse-check 均退出 1，保留，明细见报告。done.log 不存在。

## 逐项计划（实现前行号）

| 项 | 位置 | 现状 → 目标 | 事件验收 |
| --- | --- | --- | --- |
| 1 | shell/sync.go:316 | 复制时描述/配置消失 finish(nil) → 可见拒绝、非 nil 完成 | 原复制按钮触发前真实 provider refresh，无剪贴板写入，恢复后可重试 |
| 2 | shell/sync.go:356 | 导出时描述/配置消失 finish(nil) → 可见拒绝、非 nil 完成 | 同一竞态经导出按钮，不打开路径选择器 |
| 3 | shell/sync.go:322 | 复制不检查 tool owner → 遵循现有 host ownership | 真实 provider 改 owner 后复制拒绝，草稿保留 |
| 4 | shell/sync.go:362 | 导出不检查 tool owner → 同上 | 改 owner 后无文件或选择器副作用 |
| 5 | shell/shell.go:2386 | 保存解引用可为空 Configuration → 非崩溃拒绝 | 移除配置后原保存事件返回错误，已存文件字节不变 |
| 6 | shell/shell.go:2386 | 保存仍接受 tool owner → 拒绝写入工具持有配置 | 改 owner 后原保存事件返回错误，原数据/密钥保留 |
| 7 | shell/sync.go:399（第五批修改前） | 配置导出写入失败误报导入格式 → 显示实际路径/原因 | 真实目录写入失败，换路径可重试 |
| 8 | shell/sync.go:89（第五批修改前） | 集成导出写入失败误报导入格式 → 同上 | 忙态释放，换路径可重试 |
| 9 | shell/notices.go:421（目视后追加） | 长路径撑走关闭键 → 文本列 MinWidth(0) 换行 | 旧关闭坐标 x=1085.3 超出720窗口，修后全文保留、关闭可点 |

每项先在旧实现事件实红，证据不足则不记完成。每 1–2 项定向 test/vet 后提交并推送；全量门槛集中一次。两类缺口都在已有配置功能，不新增系统/依赖。

## 红线

全量证据修正（15:38 UTC）：既有 B pin 要求 busy 时复制仍是键盘出口，因此撤回计划 7/8 的 rowBusy 锁定，旧断言不改。最终 7/8 改为配置导出与集成导出的写文件失败原因：sync.go 的两处 os.WriteFile 把真实路径错误写成 IntegrationsTransferBad；用选择器返回真实目录复现，改为显示原错误并验证正常路径重试。新增两条忙态测试转为保护原出口，不计修复项。M2 忙态假设作废；新增 M3 回注旧文件写入错误映射。

高度链、键盘模型、会话、3000ms 确认、180ms/Flush、保存时机、i18n 字典和命令接线不改。最终修复失效目标接受旧动作、文件错误误报与长路径关闭入口越界；行忙态工具条保留原键盘出口，保存期字段编辑归属仍待拍板，不涉及。无颜色/半径重定基线；旧 V/B/S pin 预期 0/0/0。

## 工艺与门槛

qiaomu-design 打磨路径；读取 SKILL、偏好账本、craft-loop、preflight、chinese-typography，Apple DESIGN 的单一交互色、8px 控件半径与 1px 边只作为既有 token 的供体参照。拨盘 2/1/6。工艺 #4 即时反馈 0ms、#6 错误只占原反馈出口、#12 沿用错误带 1px 边与 8px 圆角、#3 既有焦点路径不变；原计划忙态 DimFace 已撤回。不得为凑五项加装饰。

为什么更轻：失效动作只给一处准确反馈，完整原因在原浮面内换行，不增加常驻文案或卡片层。行忙态工具条遵循既有键盘出口，最终不熄灯。

门槛：gofmt -l cmd internal 空；go vet ./... 与 GOOS=windows/darwin go vet ./...；go test -count=1 ./...；go test -count=1 -v ./... 的 PASS 不低于 5407；GOOS=linux/windows/darwin go build ./...。全部 LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4。

Mutation ≥2，按最终执行编号：M1 回注旧复制/导出/保存条件，六个刷新入口 pin 应红；M3 回注旧文件写入错误映射，配置/集成导出 pin 应红。M2 忙态锁定假设已作废，不计最终组数。每组恢复 MD5、touch、-count=1 复验。深浅 before/after 由 ui.Tester 导出，真实系统选择器与 Windows/macOS 运行时不冒充已验。
