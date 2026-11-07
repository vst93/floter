# P13-jg：导入结果完整性与备份失败恢复

意图：让集成传输明确告诉用户哪些已生效、哪些未生效，以及备份为何不能完整恢复。最新 HEAD 为 8835887e（P13-jf），不是任务书中的旧 P13-hw；按最新仍待接续。

设计读取：已有原生设置页，面向管理本地工具的用户，保持轻便液态玻璃；拨盘 3/2/7（克制、即时、高信息密度）。三秒内读到结果，必要诊断不能只留日志。

## 八项候选与真实位置（改前读取）

1. `internal/settingsui/transfer.go:71-90`：失败分支 return 藏掉成功/跳过数量 → 部分成功仍显示原汇总。
2. `internal/shell/sync.go:157-174`：Skipped 只有数量 → 将 ID 和原因带回报告，显示已有诊断。
3. `internal/settingsui/transfer.go:79-86`：第 4 条及以后失败只剩数量 → 保留前三条正文，既有提示入口接通完整详情。原“长来源路径越界”候选在深浅事件中均未复现，取消修改；不计完成项。
4. `internal/settingsui/transfer.go:65-70`：ExcludedFields.FieldCount 不可见 → 在既有详情提示中展示每个对象的排除数量，保持原警告正文。
5. `internal/extensions/sync.go:262`：负版本通过版本校验 → 拒绝不支持的负版本，保留 v1/v2。
6. `internal/extensions/sync.go:258`：JSON 解码可将非法 UTF-8 替换成 U+FFFD → 拒绝损坏文本，防止静默改写配置。
7. `internal/extensions/sync.go:164-168`：脚本读取失败静默略过 → 导出返回具体路径原因，保护已有备份。
8. `internal/extensions/sync.go:173-176`：静态 descriptor 读取失败静默略过 → 同上；不改变无包集成的兼容导出。

均先事件复现；不成立的候选不计完成。跨导入结果块、来源/排除详情浮层、宿主传输错误反馈。每完成两项运行定向门槛并 commit/push，最终全量门槛一次。

## 红线

高度链、键盘模型、会话管理、设置写回时机、覆盖策略、确认窗口与原 i18n 文案不变。不新增依赖、页面、系统。不改旧 B 断言；V/S 如需改必须单独注明。

## 门槛与变异

当前中文 locale 全量基线实测 5480 PASS / 0 FAIL；基线 HEAD 测试函数实数 3247（前轮报告 3245 未沿用）。使用 LC_ALL=zh_CN.UTF-8、GOMAXPROCS=4。
最终：gofmt -l cmd internal 空；三平台 go vet ./... 与 go build ./...；普通/verbose go test -count=1 ./...。测试函数不减少。

M1：恢复失败分支提前 return，应使部分成功真实导入 pin 变红。M2：恢复静默忽略必需文件读错，应使脚本/descriptor 导出 pin 变红。每次备份 MD5、注入、实跑红、还原、md5sum -c、touch 后复跑。

## 设计工艺

用户指定已有方向，不做四方案。craft-loop #6：结果主次与逐条诊断；#4：复用既有 600ms 提示触发而不增加结果反馈延迟；#12：浮层保持 1px 受光边，新增玻璃层 0；#5：沿用两层浮层阴影。Apple 供体提取 8px 控件圆角、1px 边、单一交互 accent 的角色分工，落在现有 token，不改正文尺寸。两轮 ui.Tester 深浅出图与 preflight 入报告。
