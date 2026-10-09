# AGENT NOTES — floter 迭代上下文

给在本仓库工作的 AI agent 看的项目级备忘：产品方向、磁盘格式约定、门槛与纪律。
构建/运行命令见 [`mygo-rewrite-status.md`](mygo-rewrite-status.md)，分轮决策记录见
[`mygo-rewrite-plan.md`](mygo-rewrite-plan.md)。

## 这是什么

floter = 一个常驻的浮动面板，一个快捷键唤出，面板里三件事：**启动器**（搜应用 / 文件 /
算式 / 扩展命令）、**终端**（ghostty 内核）、**设置**。用
[mygo](https://mygo.egoist.dev)（原生 GPU 自绘 UI，无 webview）+ Go 实现。

仓库里**只有 Go**：`cmd/floter` 入口，`internal/*` 各功能包。历史上曾用
Tauri + Rust + React 实现（见 `docs/archive/` 与 git 历史），那套代码已删除。

## 插件体系方向（2026-08-23 用户确认，仍然有效）

1. **去 NPM 强依赖**：NPM 分发不作为核心路径；围绕它建的信任栈优先级下降。
   （npm 安装能力保留，用于自建 registry / 私有包，但产品叙事不依赖它。）
2. **发现优先，用户介入越少越好**：PATH 扫描 + 约定位置 manifest + 本地连接都应能自动
   识别注入工具；不强制一套规则适配所有工具。
3. **v-tools 平权**：`extensions/v-tools` 只是"推荐工具"的参考包，和其他扩展走同一套代码路径。
4. **管理面板尽量轻**：安装/连接、开关、卸载；不做商店式 Discover。
5. **权限=诚实披露**：声明权限 + 明确告知非沙箱（`environment` / `process-spawn` 两条宿主
   真的拦，其余只披露，UI 上如实标注），不假装能强制执行。

## 磁盘格式：零迁移是硬约束

用户数据文件**格式不变**，换实现不能让用户重配。任何动这些文件的重构，先看这条：

| 文件 | 位置（`<config dir>` = `os.UserConfigDir()`） | 谁写 |
|---|---|---|
| `floter/settings.json` | 应用与终端全部设置 | 设置页 / 快捷键录制 |
| `floter/extension-repository.json` | 扩展权威状态（未知键原样保留） | 安装 / 启停 |
| `floter/extensions/<id>/` | 扩展包目录 | graft 安装 |
| `floter/extension-data/<id>/config.json` | 扩展配置（含密钥代数） | 配置注入 |
| `floter/clipboard-history/index.json` + `images/` | 剪贴板历史 | 剪贴板监听 |
| `floter/usage.json` | 启动次数（最近使用） | 每次启动应用/命令 |

**未知键逐层保留**是这套约定的实现方式：读进来 → 改自己认识的键 → 原样写回。
新增设置项时不要"重写整个文件"，也不要删掉不认识的值。

## 门槛（每次改动都要过）

```sh
gofmt -l cmd internal     # 必须为空
go vet ./...
go test -count=1 ./...    # 全绿；平台相关断言必须在任何平台都成立
GOOS=linux go build ./... && GOOS=windows go build ./...
go tool mygo build        # macOS 打包产物（可选，改动打包相关时跑）
```

**平台无关性**：CI 跑 ubuntu / macos / windows 三平台矩阵。任何平台相关的断言要么
按 `runtime.GOOS` 门控，要么把平台表写成 `(goos, home, ...)` 的**纯函数**，让每个平台的
布局在任何宿主上都能被断言（`internal/browser` 的 `chromiumBrowsers` 就是这么做的——
旧实现曾因"只在 macOS 编译"的路径表把 Edge 的目录写错而没人发现）。

**测试不碰真实系统资源**：剪贴板、登录项、窗口、原生库（libghostty-vt / sqlite）都要能注入
（`shell.Options` 里的 `WriteClipboard` / `RegisterShortcut` / `OpenAtLogin` / `OpenPinned` /
`ConfirmPermissions` / `Registry` / `NewTerminal` / `Paths`）。需要真资源的测试用环境变量
opt-in（`FLOTER_TERMINAL_TEST=1`、`FLOTER_SQLITE_TEST=1`、`FLOTER_REGISTRY_TEST=1`）。

## 反馈通道与三态

- **唯一反馈面是宿主 toast**（`launcher` 的 `toast` 字段 + 窗口底部提示）。任何表面都只发
  消息、不画自己的提示条；时长与视觉只有一份来源。
- **三态**（空 / 加载 / 错误）适用于任何会拉数据的表面：空态说"没有东西 + 为什么/怎么办"；
  加载态是**行内** spinner，有旧内容就不换掉它；错误态给"重试（重试真可能成功时才有）+ 关闭"，
  同一错误重复渲染要复用已画节点，否则重绘会丢焦点。

## 纪律

- 每轮改动自带测试；先跑门槛再提交。
- 报告里"已做 / 未做"要如实分开写，未做的不要写成半成品。
- 用户可见文案走 `internal/i18n`（en/zh 结构体，缺字段编译期报错），不要散落字面量。
