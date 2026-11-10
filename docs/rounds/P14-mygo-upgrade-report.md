# mygo 升级记录

2026-11-10：确认最新正式版 v0.8.3；完成 v0.7.4 至 v0.8.3 发布说明核对。模块已通过 goproxy.cn 下载并取得 Go 校验和；官方 proxy.golang.org 连接超时。

发现必须适配：v0.8.0 将单独修饰键作为 KeyUnknown 事件交给 HandleInput，启动器会因此取消已进入的历史删除确认。快捷键录制器保持捕获无效键以防窗口菜单抢走焦点，继续等待有效组合键。

## 已实现

- `go.mod` / `go.sum`：v0.8.3；`go mod tidy` 同时将实际直接导入的 purego 从 indirect 归入直接依赖，版本不变。Go 版本与其余依赖版本不变。
- `internal/launcher/fieldkeys.go`：过滤 `KeyUnknown`；左右 Ctrl / Command / Shift / Alt 均用新版 `Tester.CodeDown/CodeUp` 走真实事件分发验证，组合键第二次确认仍删除选中行，普通文字与 Escape 仍取消确认。
- `internal/terminalui/terminalui.go`：创建会话时启用 `ShellIntegration`。框架只对自己启动的 zsh 注入临时启动脚本，保留用户启动文件；其他 shell 不改。现有生产路径直接使用 `terminal.New`，因此不是只设置一个无效的 Conn 选项。
- `internal/launcher/launcher.go`：普通反馈标为 RoleStatus 并仅在消息变化时 Announce；警告使用新增 RoleAlert，由框架即时播报。回归验证无重复状态播报、不抢搜索框焦点。
- `internal/motion`、`internal/shell`：自绘循环、面板入场与 toast 入场读取 `Context.Instant()`，与 `DisableAnimations` 同步；仍遵循 ReduceMotion。toast 的入场时钟在停用时记为完成，下一帧恢复动画也不会倒退重播。
- `mygo.json`：新增 `license: GPL-3.0-only`，为新生成的 PKGBUILD 提供项目许可证。README 中英文补齐本地打包产物。

## 中间版本的新特性取舍

| 版本 / 能力 | floter 的处理 |
| --- | --- |
| 0.7.4：隐藏窗口提前绘制、Edit 菜单按焦点启用、macOS 空闲省电 | 随升级获得；现有窗口与菜单继续使用框架入口 |
| 0.7.4：Context.Instant、Arch PKGBUILD | 接入自绘动画；配置许可证并验证实际打包 |
| 0.8.0：物理键事件 | 修复历史删除确认；快捷键录制器本来就拒绝无效键并继续等待有效组合键，无需改成单修饰键快捷键 |
| 0.8.0：终端 ShellIntegration | 已启用；同时获得 Option-click 定位、中文路径识别、AltGr/CJK 输入与同步重绘修复 |
| 0.8.0：无障碍 RoleAlert / Tester.Access | 接入启动器反馈并验证辅助技术树；内置 Toast 原本已有 Announce，不再叠加播报 |
| 0.8.0：Opacity 作为整体合成、仅主键拖动、失活窗口强调色、取消按压不提交 | 接受新版行为；产品没有自定义拖放提交处理器，不需要模拟旧行为 |
| 0.8.1：菜单富内容、HoldPlace | 现有菜单没有预览/动态行内容，输出列表没有 FollowEnd 折叠场景；暂不引入 |
| 0.8.3：裁剪与 GPU 内存修复、更新删除已撤下文件 | 随升级获得；包括 Linux/Windows 更新时清理新版本不再携带的文件 |
| 0.8.0–0.8.3：WebView Liquid Glass、GPUProcessID、Markdown GitHub 扩展、音频能力 | 当前应用使用原生 Go UI、CLI 扩展与终端，没有相应用点 |
| 0.8.0：permissionflow、代码编辑器 snippets | 尚无需要请求的系统权限或代码补全提供器；保留现有扩展权限披露与脚本模板流程 |

来源：[v0.7.4](https://github.com/egoist/mygo/releases/tag/v0.7.4)、[v0.8.0](https://github.com/egoist/mygo/releases/tag/v0.8.0)、[v0.8.1](https://github.com/egoist/mygo/releases/tag/v0.8.1)、[v0.8.3](https://github.com/egoist/mygo/releases/tag/v0.8.3)；本地 Go 模块缓存中的各版本源码。v0.8.2 未出现在查询到的正式发布列表中。

## 验证进度

新增回归（物理修饰键、反馈可访问性、终端选项传递、禁用动画）通过。mygo vet 与 Linux/Windows/macOS 的 go vet、go build 通过。

深浅色、720×580 / 620×420、启动器/终端/设置各页的无头预览全部生成。目视检查了浅色启动器、深色常规设置、深色终端、浅色关于页，关于页正确显示 mygo 0.8.3。

macOS arm64 应用包（adhoc 签名，跳过 dmg）与 Linux amd64 的 deb、tar.gz、install.sh、PKGBUILD 均打包成功，PKGBUILD 的 license 实际为 GPL-3.0-only。没有发布、安装或签署更新清单。

首次全量对比：独立 HEAD `4f9f3f1b30ed150167cb5c6e6afcd3217e2ac073` / v0.7.3 基线 105 个顶层失败，升级版 109 个（启动器首次编译遇到测试常量拼写错误，修正后单独重跑该包补齐结果）。两版均在相同的两个旧像素采样测试里越界 panic。4 条新增失败已逐一处理：

- `TestALauncherCompletionKeepsTheDockedRow`：新反馈容器同时有 Label 和子文本，Tester.Texts 重复计数。把 RoleStatus/RoleAlert 移到消息文本本身，保留无障碍语义且仅有一个名称来源。
- `TestSettingsRowsAnswerThroughTheVeil` / `TestEscapeAnswersTheSettingsQuestionFirst`：新 Tester.Click 校验发现具名 ListItem 与真正处理点击的子行是不同节点。测试改为在该行真实中心点发送指针，仍断言切页和 Escape 优先级，产品路径未改。
- `TestQueuedCustomFailedRegistrationCannotDispatch`：原测试把弹层后的同名已保存控件误作目标；现在用打开弹层的 End/Enter 选择草稿命令，再 Tab 到草稿输入框。真正命中草稿后暴露了原有缺陷：失败注册的回调也持有当前 generation。`internal/shell/custom.go` 增加逐次注册的 accepted 标记，只有成功注册后才允许派发；原有生命周期、移除、替换、暂停、保存失败回滚用例一并通过。

42 项针对性顶层回归全部通过（`logs/focused.log`），快捷键队列的 `-race` 检查通过（`logs/race-shortcuts.log`）。最后对 toast 恢复动画的边界补充断言，3 项入场相关测试通过（`logs/instant-motion.log`）。

最终普通全量测试：**106 个顶层失败，exit 1**。105 项均在独立 v0.7.3 基线复现；额外名称为 `TestDeepLinkConnectAndRegister`，本次报 `TempDir RemoveAll cleanup: unlinkat … directory not empty`。这条清理与后台连接写入的竞态已有历史证据：`P11-cr-report.md:176`、`P13-hf-report.md:27`。单跑也观察到连接超时，仍属未解决的异步测试问题，不能声称全量全绿。

对该深链用例在旧、新两版各连续运行 10 次：v0.7.3 失败 7 次，v0.8.3 失败 3 次，均复现连接时序问题（`logs/deeplink-baseline-10.log` / `logs/deeplink-repeat-10.log`）。最终观察到的失败均有旧版或历史记录依据，没有遗留已确认由本次升级引入的失败。

比较清单见 [`test-comparison.json`](P14-mygo-upgrade/test-comparison.json)。两个版本都在 launcher / settingsui 的既有像素采样测试里 panic，后续用例未能执行，因此上述比对只覆盖实际执行的测试。最后的 toast 时钟边界改动以 `instant-motion.log` 专项复验；最新版代码再次通过 mygo vet，并重新生成 macOS/Linux 包。

验证证据位于 [`P14-mygo-upgrade/`](P14-mygo-upgrade/)：压缩的旧版基线与初次升级日志、最终门槛日志、发布说明原文、生成的 PKGBUILD、4 张已目视的界面预览。三平台 vet/build 为本机交叉检查；没有把 Windows/Linux 实机交互、VoiceOver 实际朗读或交互式 zsh 调整窗口写成已实测。未安装、发布或推送。
