# Floter 扩展平台开发计划

> 本文档整理自 2026-08-08 Codex 设计会话的完整对话记录。
> 会话目标：将 Floter 从「应用搜索 + 终端」升级为「终端能力平台」，支持第三方 CLI/TUI 工具作为扩展接入，以 `v` (github.com/vst93/v) 作为官方参考实现。
>
> 当前状态：阶段 1-7 全部完成。扩展协议（FEP-1~6）、Rust 后端、React 命令联想、插件管理页面、V Tools 静态适配器、端到端验证、动态 complete、NPM 签名分发、SDK 模板、权限模型、声明式配置和跨设备同步均已落地。

---

## 一、项目定位

Floter 定位为**终端能力平台**，而非传统插件运行时：

- Floter 负责搜索、联想、配置、进程调度和终端承载。
- 插件仍然是独立 CLI/TUI 程序（Go、Rust、Python、Bash 均可）。
- 接入层只描述「有哪些命令、参数是什么、如何启动」。
- 插件界面直接运行在 PTY 中，不提供 HTML 页面，也不嵌入 Floter 进程。
- 与 uTools 的本质区别：uTools 基于 Electron + 网页插件，Floter 基于 Tauri + 原生终端插件。

**边界澄清**：Floter 本身的启动器界面仍由 WebView (React) 渲染，但插件运行内容完全不基于网页，而是原生 CLI/TUI。

---

## 二、核心架构

### 2.1 整体数据流

```
输入框
  │
  ▼
命令解析器 ──► 统一命令目录 (Command Catalog)
  │               │
  │               ├─ 系统应用
  │               ├─ 系统命令
  │               ├─ 本地配置命令
  │               └─ 外部工具提供的命令（Provider）
  │
  ▼
执行计划 (Execution Plan)
  │
  ├─ 内置 PTY 执行
  ├─ 跳转系统终端
  ├─ 后台捕获输出
  └─ 打开 URL/文件
```

Floter 不关心插件是什么语言写的，只关心最终的结构化执行计划：

```
program: /path/to/v
args: [jv, -f, example.json]
mode: pty
cwd: current
```

用户输入 `jv -f example.json`，实际执行 `/path/to/v jv -f example.json`。`v` 前缀仅存在于执行层。

### 2.2 两层接入模型

核心设计决定：**Floter 只固定「如何询问 Provider」，从不固定「Provider 里面有哪些命令」。**

```
Bootstrap 注册层
只回答：如何找到并询问这个工具？
        │
        ▼
Provider 动态协议层
工具回答：我当前有哪些命令、参数、配置和执行方式？
```

对于 `v`：

```
Floter
  └─ 执行：v --floter describe --protocol 1
                │
                ▼
       v 动态遍历 Plugin.List()
                │
                ▼
       返回当前所有插件的 JSON 描述
```

`v` 新增插件时，只要它注册进 `Plugin.List()`，更新 `v` 二进制后，Floter 刷新目录就能自动发现，不需要更新 Floter，也不需要更新映射文件。

### 2.3 三种接入等级

三层体系，由低到高：

#### 等级 1：原生 Provider

第三方工具自己实现 Provider 协议：

```bash
foo --floter describe
foo --floter complete
```

优点是信息最准确、无需额外维护。`v` 就是官方参考实现。

#### 等级 2：中间适配器 (Wrapper)

不能修改的工具，由独立 Wrapper 实现 Provider 协议：

```
Floter
  └─ kubectl-floter-provider --floter describe
                                │
                                └─ 调用 kubectl
```

Wrapper 可以：
- 调用工具已有的 JSON 接口。
- 读取 Cobra/Clap/Click 等命令定义。
- 使用工具已有的 completion 脚本。
- 最后才考虑解析 `--help` 文本（降级方案，格式/语言/版本变化会不稳定）。

#### 等级 3：用户本地配置

给普通用户一个声明式适配器编辑器：

```json
{
  "id": "local.my-tool",
  "executable": "my-tool",
  "discovery": {
    "type": "static",
    "commands": [...]
  }
}
```

或者指向一个发现指令：

```json
{
  "id": "local.my-tool",
  "executable": "my-tool",
  "discovery": {
    "type": "command",
    "args": ["commands", "--json"]
  }
}
```

配置页面负责生成这些文件。简单工具不用开发 Wrapper，复杂工具再升级为 Provider。

**三种来源最终都转换成相同的内部 `CommandDescriptor`，搜索和执行层不区分来源。**

---

## 三、Floter Extension Specification (FEP)

规范拆成六个部分（全部已完成文档）：

| 编号 | 名称 | 状态 | 文件 |
|------|------|------|------|
| FEP-1 | Extension Package Manifest | ✅ 已完成 | `docs/extensions/FEP-1-package.md` |
| FEP-2 | Provider Runtime Protocol | ✅ 已完成 | `docs/extensions/FEP-2-provider.md` |
| FEP-3 | Installation Lifecycle & Security | ✅ 已完成 | `docs/extensions/FEP-3-lifecycle.md` |
| FEP-4 | NPM Registry Convention | ✅ 已完成 | `docs/extensions/FEP-4-npm-registry.md` |
| FEP-5 | Permissions and Security | ✅ 已完成 | `docs/extensions/FEP-5-permissions.md` |
| FEP-6 | Declarative Configuration | ✅ 已完成 | `docs/extensions/FEP-6-declarative-config.md` |

### 3.1 FEP-1：扩展包与平台运行时

#### NPM 包入口

`package.json` 必须包含：

```json
{
  "keywords": ["floter-extension"],
  "floter": { "manifest": "floter.extension.json" }
}
```

`manifest` 必须是包根目录内的相对路径，不能包含 `..`。Host 解包时拒绝绝对路径、符号链接逃逸和写出安装目录的 tar 条目。

Floter 不执行 `npm install`，而是直接使用 Registry HTTP API 下载 `.tgz`、校验 `dist.integrity` 并解包。用户无需安装 Node.js。Floter 永远不执行 NPM 的 `preinstall`、`install`、`postinstall` 或包内 JavaScript。

#### 两种安装模式

**managed（Floter 托管）**

- Floter 从 NPM 下载对应平台二进制。
- 基础包保存清单、图标和文档，`platformPackages` 按当前目标选择实际二进制包。
- 同一个扩展版本的基础包与平台包版本必须一致。
- 平台包解压到扩展版本目录的 `runtime/` 下，`executable` 相对于该目录。
- Unix Host 在完整性验证后补充用户可执行位；Windows 不修改 ACL。
- Floter 知道安装目录，可以直接更新、回滚和删除，不污染系统 PATH。

**linked（关联现有工具）**

- Floter 检测 PATH 或用户指定路径中的工具。
- `executableNames` 按顺序探测；Windows 自动考虑 `.exe`、`.cmd` 和 `.bat`，Unix 只接受普通可执行文件。
- Floter 只管理关联关系。删除只解除注册，绝不删除外部程序。更新由工具自己的包管理器负责。
- 管理页面显示「外部管理」。

安装时让用户选择：

```
V Tools

● 使用现有安装
  /opt/homebrew/bin/v · 0.0.12

○ 由 Floter 管理
  从 NPM 下载 · 18.4 MB
```

#### 平台覆写

`platformOverrides` 可以覆写 Provider 参数、环境变量和最低系统版本，但不能覆写扩展 ID、发布者或权限。解析顺序：

1. 通用 `provider` 配置；
2. 当前 `<os>-<arch>` 的精确覆写；
3. 当前 `<os>-any` 的系统覆写。

环境变量只允许显式键值；Host 不展开 shell 表达式。所有参数以 argv 数组保存，不接受拼接的 shell 字符串。

#### 平台目标标识

| 目标 | 系统 | 架构 |
|------|------|------|
| `darwin-arm64` | macOS 11+ | Apple Silicon |
| `darwin-x64` | macOS 10.15+ | Intel |
| `linux-arm64` | Linux | AArch64 |
| `linux-x64` | Linux | x86-64 |
| `windows-arm64` | Windows 10+ | ARM64 |
| `windows-x64` | Windows 10+ | x86-64 |

#### 三平台差异要求

- **macOS**：必须区分 `darwin-arm64` 与 `darwin-x64`。托管包可以是未签名程序，但管理页必须显示签名状态；正式源应提供公证产物。
- **Linux**：托管包应尽量使用静态链接或声明 `diagnose` 检查系统动态库。Host 不假设发行版，不自动运行 `apt`、`dnf` 或 `pacman`。
- **Windows**：可执行入口可以是 `.exe`。第一版托管包不接受 `.cmd`/`.bat` 作为主入口，避免隐式 `cmd.exe` 和不可控转义；linked 模式可以识别它们，但执行时必须显式选择对应宿主。

#### 四层版本独立

```
io.github.vst93.v        Provider ID
@vst93/floter-v@0.0.12  NPM 扩展包版本
V Tools 0.0.12          工具版本
Provider Protocol 1.0   运行协议版本
```

### 3.2 FEP-2：Provider 运行协议

#### 调用约定

扩展包声明 `provider.argsPrefix`。Host 将操作名附加在后面。例如前缀为 `["--floter"]` 时：

```bash
v --floter describe --protocol 1
v --floter complete --protocol 1
v --floter diagnose --protocol 1
```

- Provider 必须把协议 JSON 写到 stdout，把诊断日志写到 stderr。
- 成功返回 0，协议错误返回 2，工具自身错误返回其他非零值。
- stdout 不能包含 ANSI 或额外文本。UTF-8 是唯一允许的编码。
- `describe` 必须在五秒内完成且不能修改用户配置。
- Host 缓存成功结果，在可执行文件路径、mtime、包版本或工具版本变化时刷新。

#### describe（发现能力）

输出包含 Provider 身份、工具版本和完整命令目录。命令的 `execution.program` 默认是 `self`，表示执行刚才响应 describe 的同一文件；也可以指向托管包 `runtime/` 下的相对路径。

命令参数必须使用结构化类型：

| kind | 说明 |
|------|------|
| `flag` | 不取值的开关 |
| `string` | 字符串标量 |
| `integer` | 整数标量 |
| `number` | 数字标量 |
| `path` | 由 Host 提供本地路径联想 |
| `directory` | 由 Host 提供目录联想 |
| `url` | URL 输入 |
| `enum` | 由 `values` 提供候选 |
| `command` | 由 Provider 的动态 complete 提供候选 |

执行模式：

| mode | 说明 |
|------|------|
| `pty` | 交互式 TUI，必须使用此模式 |
| `capture` | 旧版兼容值，当前归一化为 `pty` |
| `external` | 跳转系统终端 |

Host 永远以 argv 生成执行计划，不把用户输入直接拼进 shell 字符串。

#### complete（动态联想，可选）

请求 JSON 从 stdin 读取：

```json
{
  "command": "jv",
  "args": ["-f"],
  "cwd": "/home/user"
}
```

返回：

```json
{
  "completions": [
    { "label": "-file", "kind": "flag", "detail": "Read from file" }
  ]
}
```

- 动态补全默认应在 800ms 内返回，可通过 `completeTimeoutMs` 配置。
- Host 必须防抖、取消过期请求，并对 cwd 与输入建立短期缓存。
- Host 合并静态与动态结果；Provider 不支持、超时或失败时降级到静态结果。

#### diagnose（诊断，可选）

用于管理页健康检查。检查结果只能报告问题，不能自行安装依赖或修改系统。

```bash
v --floter diagnose --protocol 1
```

返回：

```json
{
  "status": "warning",
  "checks": [
    { "id": "clipboard", "status": "ok", "message": "Clipboard integration available" },
    { "id": "gencm-api", "status": "warning", "message": "API key is not configured" }
  ]
}
```

#### config（声明配置，可选）

Provider 可以声明配置由谁管理：

```json
{
  "configuration": {
    "owner": "tool",
    "openCommand": ["--floter", "config"]
  }
}
```

或者提供声明式表单：

```json
{
  "configuration": {
    "owner": "host",
    "schema": [
      { "key": "defaultLanguage", "type": "select", "options": ["zh", "en"] }
    ]
  }
}
```

对于 `v`，建议继续由 `v` 管理 `~/.v_tools/settings.ini`。Floter 不复制这些配置，只负责启动配置命令或展示工具返回的声明式配置。

### 3.3 FEP-3：安装生命周期与安全

#### 持久状态与操作阶段

```
not-installed -> enabled <-> disabled
                     \       /
                       broken
```

`enabled`、`disabled` 和 `broken` 写入 lock 文件。解析、下载、校验、安装、
更新、回滚和删除是 Host 操作或事务阶段，不作为扩展持久状态。安装事务内部按
`resolving -> downloading -> verifying -> installing -> complete` 推进；任一步失败
都保留原来的可用版本和状态。不要让插件自己实现 `install`、`update`、
`delete` 命令。

#### 各操作定义

| 操作 | Floter 行为 |
|------|------------|
| 安装 | 下载、校验、解包、验证 Manifest、运行 describe、原子启用 |
| 启用 | 将 Provider 命令加入搜索目录 |
| 禁用 | 移出搜索目录，但保留程序和配置 |
| 更新 | 下载到新版本目录、验证、切换 current 指针 |
| 回滚 | 恢复上一个已验证版本 |
| 删除 | 删除托管文件，可选择是否保留用户数据 |
| 解除关联 | 删除外部工具的注册信息，不删除外部程序 |
| 修复 | 重新校验、下载缺失文件、重新执行 describe |

#### 安装事务（8 步原子流程）

1. 从 NPM Registry 解析精确版本和 `dist.integrity`。
2. 下载基础包以及当前平台包。
3. 在临时目录验证 integrity 并安全解包。
4. 验证 Package Manifest、Host 版本、协议版本、OS 和架构。
5. 启动 Provider `describe`，校验 Provider ID 与扩展 ID 相同。
6. 可选执行 `diagnose`。
7. 原子移动到 `extensions/<id>/versions/<version>`。
8. 原子写入 lock 文件并启用命令目录。

任何一步失败都不得改变当前可用版本。

#### 目录结构

```
floter/
├─ extensions/
│  └─ io.github.vst93.v/
│     ├─ versions/
│     │  ├─ 0.0.11/
│     │  └─ 0.0.12/
│     └─ current.json
├─ extension-data/
│  └─ io.github.vst93.v/
└─ extensions.lock.json
```

程序文件与用户数据必须分离。删除程序时，用户可以选择「保留配置」或「同时删除数据」。

#### 更新与回滚

- 新版本安装到并列目录，通过 lock 文件的 current version 原子切换。
- 至少保留一个 previous version，直到新版本成功运行。
- 默认自动更新只允许 patch；major 更新需要用户确认。
- 用户可以固定版本或选择 stable/beta dist-tag。

#### 安全基线

- 不执行任何 NPM 生命周期脚本或包内 JavaScript。
- 所有 tar 路径在解包前规范化，拒绝绝对路径、`..` 和目录外链接。
- 使用 Registry 提供的 Subresource Integrity 校验 tarball。
- Provider ID、包名、版本和发布者与 lock 记录一致。
- 执行计划使用结构化 program/argv/cwd/env。
- Provider 超时后终止子进程，stdout 大小设上限。
- 官方索引必须是签名的包名白名单；普通 NPM 搜索结果标记为未验证。
- 权限是披露而不是沙箱。原生 CLI 仍拥有启动它的用户权限。

---

## 四、联想交互设计

### 4.1 输入阶段联想

```
j
├─ jv       JSON Viewer             来源：V Tools
└─ java     系统应用

jv -
├─ -f       格式化 JSON
├─ -c       压缩 JSON
├─ -file    从文件读取
├─ -url     从 URL 读取
└─ -tui     打开交互式终端界面

jv -file ./
├─ data.json
├─ package.json
└─ examples/
```

### 4.2 键盘行为

| 按键 | 行为 |
|------|------|
| `Tab` | 插入当前联想，不执行 |
| `Enter` | 在 Floter 内置终端运行 |
| `Cmd/Ctrl+Enter` | 直接交给系统终端 |
| `↑/↓` | 选择建议 |
| 参数缺值时 Enter | 优先补全，不直接执行 |

### 4.3 命令冲突处理

结果旁标注来源。发生命令冲突时，默认按用户使用频率和精确匹配排序，同时保留显式命名空间写法：

```
jv           # 按频率/精确匹配自动选择
v:jv         # 强制指定 V Tools 的 jv
system:jv    # 强制指定系统的 jv
```

普通情况下用户永远不需要输入命名空间。

---

## 五、配置管理

### 5.1 配置所有权

- `v` 自己已有的设置继续保存在 `~/.v_tools/settings.ini`。
- Floter 只保存启用状态、别名、命令优先级和可执行文件路径。
- 工具专属配置优先由工具自己管理，避免产生两份不一致的配置。

### 5.2 声明式配置表单

Manifest 可以声明配置字段，由 Floter 统一渲染：

- 文本、密码、路径
- 单选、多选
- 开关
- 数字
- 环境变量映射
- 命令参数映射

### 5.3 跨设备同步

三套生命周期独立：

| 内容 | 负责方 |
|------|--------|
| Provider/Adapter 包更新 | NPM 或 Floter Registry |
| 工具本身更新 | Homebrew、GitHub Releases、工具自己的更新机制 |
| 用户配置跨设备同步 | Floter Cloud、Git、WebDAV 等独立机制 |

跨设备同步时只同步：
- 安装了哪些 Provider
- 是否启用
- 用户自定义别名和优先级
- 非敏感的 Host 配置

**不应同步**：本机绝对路径、API Key、工具自己的私有配置、命令历史中的敏感参数。

---

## 六、NPM 分发与可信来源

### 6.1 包结构

```
@vst93/floter-v
├─ package.json
├─ floter.extension.json
├─ icon.png
└─ README.md
```

不同平台二进制拆成独立包（类似 esbuild 发布方式）：

```
@vst93/floter-v                  # 基础包（清单+图标+文档）
@vst93/floter-v-linux-x64
@vst93/floter-v-darwin-arm64
@vst93/floter-v-darwin-x64
@vst93/floter-v-windows-x64
```

包版本与工具版本一致。包中不包含固定的命令列表——安装后 Floter 调用 `describe` 动态获取。

### 6.2 NPM 的正确定位

NPM 适合做「Provider/Adapter 包分发仓库」，但不适合承担工具运行时或用户配置同步。Floter 不依赖 Node.js，直接通过 Registry HTTP API 下载 `.tgz`、校验 integrity 并解包。

第一版强烈建议 NPM 包只能包含声明文件和静态资源。允许包内执行任意 Node 脚本会立刻引入 Node 运行时、供应链攻击和跨平台兼容问题。后续需要代码的 Wrapper 时，优先发布为独立原生二进制，NPM 包只描述如何下载和校验它。

### 6.3 可信来源

两类源：

- **官方源**：Floter 维护签名索引，索引指向 NPM 包。只保存 `id`、`npm` 包名、`publisher`、`verified` 标记。
- **社区源**：直接搜索 NPM，明确标记「未经验证」。

安装时至少校验：
- NPM tarball integrity
- 包名与 Manifest ID
- `package.json` 与 Manifest 版本
- Floter 最低版本
- Provider 协议兼容范围
- 当前操作系统和架构
- 发布者是否与官方索引一致

---

## 七、插件管理页面

### 7.1 布局

三个标签页：

```
已安装  |  发现  |  更新
```

已安装页面采用紧凑列表（不是大卡片）：

```
V Tools             0.0.12    已启用    [开关] [更新] [更多]
Git Utilities       1.4.2     已禁用    [开关]        [更多]
Docker Provider     2.1.0     有更新    [开关] [更新] [更多]
```

### 7.2 详情页/侧边抽屉

展示：
- 插件名称、发布者、版本和来源
- 导出的命令列表
- Provider 协议版本
- 托管或外部关联状态
- 实际可执行文件路径
- 权限声明
- 配置入口
- 诊断结果
- 更新日志
- 自动更新、版本固定和发布通道
- 禁用、重新安装、删除、回滚

### 7.3 删除操作

```
删除 V Tools

● 删除程序，保留配置
○ 删除程序和全部数据

外部关联的工具只会从 Floter 中移除，不会删除系统文件。
```

---

## 八、V (github.com/vst93/v) 接入方案

### 8.1 现有基础

`v` 已经具备接入基础：

- 子命令分派集中在 `main.go:17`。
- 插件都有名称、版本、描述、命令和参数信息，接口位于 `service/plugin.go:17`。
- 例如 `jv` 已经声明了完整参数说明，`plugin/jv/jv.go:24`。
- `v` 的整体版本已经能够独立输出，`service/help.go:9`。
- 配置文件由 `setting/ini.go` 管理，存储在 `~/.v_tools/settings.ini`。
- `main.go` 中定义了快捷别名 `gp`、`gc`、`cc` 等，这些也应进入 Descriptor 成为单一信息源。

当前 `v` 的插件接口：

```go
GetName()
GetVersion()
GetDescription()
GetCommand()
GetArgs() map[string]string
```

#### 当前已知插件清单（Codex 审计确认）

| 插件 | 命令 | 说明 | 参数声明方式 |
|------|------|------|-------------|
| jv | `jv` | JSON 查看/格式化/编辑 | `GetArgs()` map |
| codec | `codec` | 编解码工具 | `GetArgs()` map |
| genpwd | `genpwd` | 密码生成 | `GetArgs()` map |
| diff | `diff` | 差异对比 (Myers 算法) | `GetArgs()` map |
| tt | `tt` | 时间戳转换 | `GetArgs()` map |
| json2excel | `json2excel` | JSON 转 Excel | `GetArgs()` map |
| gcm | `gcm` | Git Commit Message 生成 | `GetArgs()` map |
| vc | `vc` | 视频合并 | `GetArgs()` map |
| translate | `tr` | 翻译 | `GetArgs()` map |
| cp | `cp` | 复制增强 | `GetArgs()` map |

所有插件当前都使用 `GetArgs() map[string]string` 声明参数，迁移到 `DescriptorProvider` 接口时需逐个覆盖。第一版 V Adapter 静态适配器应优先覆盖 `jv`、`diff`、`codec`、`genpwd`、`tt` 这 5 个核心插件。

### 8.2 不足与改进

`GetArgs() map[string]string` 不足以长期支撑联想，因为缺少：
- 参数顺序
- 参数是否需要值
- 值类型：路径、URL、整数、枚举
- 参数是否可以重复
- 参数之间是否互斥
- 默认值
- 示例
- 动态补全方式

### 8.3 V 内部改进方案

#### 接入分两步走

**第一步：静态适配器，完全不改 `v`**

在 Floter 侧维护一个 V Adapter，检测 PATH 中的 `v`，把 `jv`、`diff`、`codec`、`genpwd`、`tt` 等导出成 Floter 顶级命令。这一步不需要修改 `v` 的任何代码，纯粹在 Floter 侧用静态 Manifest 声明命令和参数。

适用场景：快速验证端到端流程，用户只需要已安装 `v` 即可。

**第二步：给 `v` 增加 `--describe-json`**

现有 `GetArgs() map[string]string` 能提供初级联想，但因为 map 没有顺序，也缺少参数类型、是否需要值、枚举范围等信息。后续增加一个可选的结构化描述接口。原有 `PluginTemplate` 不需要破坏性修改。

#### CommandDescriptor 单一信息源

不要为 Floter 再维护第二套插件列表，而是让插件元数据成为唯一信息源：

```go
type CommandDescriptor struct {
    ID          string
    Name        string
    Version     string
    Description string
    Aliases     []string
    Arguments   []ArgumentDescriptor
    Execution   ExecutionDescriptor
}
```

插件注册、`v -h` 帮助输出和 `v --floter describe` 都从同一个 Descriptor 生成。

平滑迁移方案——可选接口：

```go
type DescriptorProvider interface {
    Descriptor() CommandDescriptor
}
```

处理逻辑：
- 新插件实现 `DescriptorProvider`，获得完整联想能力。
- 老插件继续使用现有 `GetArgs()`，V 自动转换成基础 flag 描述。
- 以后再逐步迁移老插件。
- `v -h` 也逐步改为根据 Descriptor 生成，避免帮助文档与实际参数不一致。
- 当前写在 `main.go` 中的 `gp`、`gc`、`cc` 等别名也应该进入 Descriptor，成为单一信息源。

这样 V 新增一个插件时，不需要理解 Floter，只需要遵守 V 自己的插件元数据规范。

### 8.4 V 的发布方式

V 每次发布新版本时同时发布：

```
@vst93/floter-v
@vst93/floter-v-linux-x64
@vst93/floter-v-darwin-arm64
@vst93/floter-v-darwin-x64
@vst93/floter-v-windows-x64
```

新增 V 插件的流程：
1. 新插件注册进 V 的插件目录。
2. `v --floter describe` 自动输出它。
3. 发布新的 V 和平台 NPM 包。
4. Floter 检测到更新。
5. 更新后重新执行 describe。
6. 新插件自动进入搜索目录。

整个过程中 Floter 不新增任何命令映射。

### 8.5 不建议的做法

- ❌ 解析 `v -h` 的 ANSI 文本（帮助输出不是稳定协议）
- ❌ 把 Go 插件编译成 Rust 动态库（ABI、CGO 和跨平台发布问题）
- ❌ 把 `v` 二进制打包进 Floter（破坏独立版本治理）
- ❌ 把 `v` 的每个子命令安装成独立扩展（它们共享一个 `v` 版本）
- ❌ Floter 维护 `jv -> v jv` 这种逐命令映射

---

## 九、现有 Floter 代码可复用部分

### 9.1 搜索结果模型

当前搜索结果模型已经预留了 `command` 类型（`src/App.tsx:65`），只是搜索流程目前只真正加入了应用和系统操作（`src/App.tsx:726`）。

### 9.2 执行路径

- 输入命令后创建内置终端（`src/App.tsx:2049`）
- Rust 接收初始命令并创建 PTY（`src-tauri/src/commands/terminal.rs:10`）
- 当前 PTY 还能继续移交给系统终端（`src-tauri/src/commands/terminal.rs:109`）

**真正需要新增的是「命令目录和解析层」，终端本身不需要重新设计。**

### 9.3 旧自定义命令（不直接扩展，需迁移）

现有 `custom.rs`（`src-tauri/src/commands/custom.rs:7`）可以作为早期原型参考，但不适合直接扩展成平台协议：
- 它目前只有命令字符串，并通过 `sh -c`/`cmd /C` 执行（`custom.rs:130`）
- 正式平台应保存 `program + args[]`，避免字符串拼接、转义差异和命令注入问题
- 旧自定义命令保持隔离，新扩展平台以独立 `extensions` 模块实现

**重要发现**：Codex 审计确认，`commands.json` 后端**目前根本没有接入前端**——`get_custom_commands` 在 `App.tsx` 中没有对应的调用逻辑，搜索流程只加入了应用和系统操作。这意味着旧自定义命令功能实际上是一个未完成的功能，迁移时不需要考虑前端兼容性，可以直接用新的扩展平台替代。

**迁移策略**：
1. 保留 `custom.rs` 代码但标记为 deprecated
2. 新建 `src-tauri/src/extensions/` 模块，完全独立实现
3. 如果用户有旧 `commands.json`，提供一个一次性迁移工具转换为本地 Adapter 配置
4. 确认无用户依赖后删除 `custom.rs`

### 9.4 前端集成点

Codex 审计了现有前端代码结构，以下是扩展平台需要接入的具体位置：

| 功能 | 文件 | 位置 | 说明 |
|------|------|------|------|
| 搜索结果类型 | `src/App.tsx:65` | `LauncherItem` 类型 | 已预留 `command` 类型，需补全搜索和渲染逻辑 |
| 搜索流程入口 | `src/App.tsx:726` | 搜索结果构建 | 目前只加入应用和系统操作，需接入 `catalog_search` |
| 终端创建 | `src/App.tsx:2049` | 命令执行 | 已有创建内置终端的逻辑，需改为接收结构化执行计划 |
| Settings 面板 | `src/App.tsx` | settings/settings-tab 模式 | 插件管理页面应作为 Settings 的一个新 tab 接入 |
| i18n | `src/i18n.ts` | 国际化 | 新增管理页面需要 en + zh 字符串 |

### 9.5 Cargo 依赖可用

Cargo.lock 中已有以下依赖，无需额外引入：
- `reqwest 0.13.4` — HTTP 客户端（NPM Registry API）
- `semver 1.0.28` — 版本比较
- `sha2 0.10.9` — integrity 校验
- `tar 0.4.46` — 解包
- `flate2 1.1.9` — gzip 解压

**注意**：Codex 额外检查了 `shell-words` crate（用于跨平台命令行参数解析/转义），在 Cargo 缓存中**未找到**。如果需要跨平台 argv 处理，需在 `Cargo.toml` 中新增此依赖，或使用 `std::process::Command` 直接传 argv（推荐后者，避免额外依赖）。

### 9.6 项目目录结构

Codex 审计了现有目录结构，扩展平台应新增的目录：

```
floter/
├─ src-tauri/src/
│  ├─ commands/
│  │  └─ extensions.rs        # 新增：Tauri commands 入口
│  ├─ extensions/             # 新增：扩展平台核心模块
│  │  ├─ mod.rs
│  │  ├─ manifest.rs          # Manifest 加载与校验
│  │  ├─ provider.rs          # Provider 调用与缓存
│  │  ├─ catalog.rs           # 统一命令目录
│  │  ├─ install.rs           # 安装生命周期
│  │  └─ lock.rs              # extensions.lock.json 管理
│  └─ lib.rs                  # 注册新 commands
├─ src/
│  ├─ App.tsx                 # 接入 catalog_search 和联想 UI
│  ├─ ExtensionsPanel.tsx     # 新增：插件管理页面
│  └─ i18n.ts                 # 新增 en + zh 字符串
└─ docs/extensions/           # ✅ 已完成：协议文档
```

---

## 十、缓存与刷新策略

动态读取不等于每输入一个字符都运行一次 `v`。

推荐策略：

1. 首次安装 Provider 时执行 `describe`。
2. 启动 Floter 时检测二进制路径、mtime 和版本。
3. 工具版本变化时重新执行 `describe`。
4. 用户可以手动刷新。
5. describe 结果落盘缓存，工具暂时不可用时仍能显示命令，但标记「未找到运行时」。
6. 动态 complete 设置 100～150ms debounce、超时和请求取消。

---

## 十一、实施计划（分 7 个阶段）

### 阶段 1：协议文档 ✅ 已完成

产出物：`docs/extensions/` 下的 8 个文件
- FEP-1 / FEP-2 / FEP-3 规范文档
- `floter-extension.schema.json` — 扩展包 Manifest 的 JSON Schema
- `provider-description.schema.json` — Provider describe 输出的 JSON Schema
- V Tools 参考示例（`floter.extension.json` + `provider-description.json`）

### 阶段 2：Rust 扩展后端 ✅ 已完成

新建 `src-tauri/src/extensions/` 模块，实现：

1. **Manifest 加载与校验**
   - 解析 `floter.extension.json`
   - 按 JSON Schema 校验
   - 平台目标解析（`<os>-<arch>`）
   - `platformOverrides` 合并

2. **Provider Manager**
   - 调用 `describe` 获取命令目录
   - 缓存结果（基于路径 + mtime + 版本）
   - 超时处理（5s）
   - stdout/stderr 分离
   - Provider 不可用时的降级（显示缓存命令，标记「未找到运行时」）

3. **统一命令目录 (Command Catalog)**
   - 合并系统应用、系统命令、本地配置命令和 Provider 命令
   - 命令冲突解决（频率排序 + 命名空间）
   - 启用/禁用控制

4. **结构化执行计划**
   - `program + args[]` 格式（不用 `sh -c`）
   - 支持 `pty` / `external`；旧版 `capture` 兼容性归一化为 `pty`
   - workingDirectory 处理
   - 环境变量传递
   - 使用 `std::process::Command` 直接传 argv，不引入 `shell-words` 依赖

5. **安装生命周期**
   - NPM Registry API 调用（下载 `.tgz`）
   - integrity 校验
   - 安全解包（拒绝路径逃逸）
   - managed / linked 两种模式
   - 原子安装/更新/回滚
   - `extensions.lock.json` 管理
   - 状态机实现

6. **声明式配置支持**
   - 解析 Provider `config` 输出的声明式 schema
   - Host 侧渲染表单并保存配置值
   - 工具自管理配置时只提供「打开配置」入口

7. **旧自定义命令迁移**
   - 保留 `custom.rs` 但标记 deprecated
   - 提供一次性迁移工具：旧 `commands.json` -> 本地 Adapter 配置
   - 新扩展平台完全独立，不依赖旧代码

8. **Tauri Commands**
   - `extensions_list` - 列出已安装扩展
   - `extensions_install` - 安装扩展
   - `extensions_uninstall` - 删除扩展
   - `extensions_enable` / `extensions_disable`
   - `extensions_update` - 更新扩展
   - `extensions_describe` - 获取 Provider 命令
   - `extensions_diagnose` - 运行诊断
   - `extensions_search` - NPM 搜索
   - `extensions_config_get` / `extensions_config_set` - 声明式配置读写
   - `catalog_search` - 统一命令搜索（合并系统 + 扩展）
   - `catalog_complete` - 参数联想（静态 + 动态 complete）

### 阶段 3：React 前端 - 命令联想 ✅ 已完成

1. **输入框联想 UI**
   - 命令联想（输入 `j` -> 显示 `jv`、`java`...）
   - 参数联想（输入 `jv -` -> 显示 `-f`、`-c`、`-file`...）
   - 路径联想（输入 `jv -file ./` -> 显示文件列表）
   - 来源标注和冲突处理
   - 键盘行为（Tab/Enter/Cmd+Enter/↑↓）

2. **命令目录集成**
   - 将 `catalog_search` 结果集成到现有搜索流程（`App.tsx:726`）
   - `command` 类型结果渲染（`App.tsx:65` 已有类型预留，需补全逻辑）

3. **结构化执行集成**
   - 将执行计划传给现有 PTY 终端（`App.tsx:2049`）
   - `external` 模式跳转系统终端（`terminal.rs:109`）
   - 保持现有 TUI、鼠标、剪贴板功能不受影响

4. **i18n 集成**
   - 在 `src/i18n.ts` 中新增扩展相关的 en + zh 字符串
   - 联想来源标注（如「来源：V Tools」/「Source: V Tools」）
   - 冲突提示、状态标记等文案

### 阶段 4：插件管理页面 ✅ 已完成

插件管理页面作为 Settings 面板的新 tab 接入（复用现有 settings/settings-tab 模式），新建 `src/ExtensionsPanel.tsx` 组件。

1. **已安装标签页**
   - 紧凑列表布局
   - 开关、更新、更多操作
   - 状态显示（enabled/disabled/broken/有更新）

2. **发现标签页**
   - NPM 搜索
   - 官方源 / 社区源切换
   - 安装按钮

3. **更新标签页**
   - 有更新的扩展列表
   - 批量更新

4. **详情侧边抽屉**
   - 插件信息展示
   - 命令列表
   - 诊断结果
   - 配置入口（工具自管理时启动 `openCommand`；Host 管理时渲染声明式表单）
   - 删除/回滚/重新安装

5. **i18n**
   - 管理页面全部文案双语（en + zh）
   - 状态名称、操作按钮、确认对话框

### 阶段 5：V Provider 参考实现 ✅ 已完成

分两步走，先验证端到端流程，再实现动态协议。

#### 第一步：静态适配器（不改 `v` 代码）

在 Floter 侧维护一个 V Adapter（linked 模式），检测 PATH 中的 `v`，用静态 Manifest 声明 `jv`、`diff`、`codec`、`genpwd`、`tt` 这 5 个核心插件的命令和参数。

管理页将其显示为“内置集成 · 系统工具”。检测只表示可连接；用户确认连接并
写入 lock 后，命令才进入目录。用户断开连接时不删除 `v`。

1. **编写 V Adapter Manifest**
   - `runtime.type = "linked"`
   - `executableNames = ["v", "v.exe"]`
   - 静态声明 5 个核心插件的命令、参数和执行方式
   - 参数信息参考 Codex 审计结果（`plugin/jv/jv.go`、`plugin/diff/diff.go` 等）

2. **验证闭环**
   - 安装 Adapter 后输入 `jv -` 能看到参数说明
   - 按 Enter 后在终端运行 `v jv ...`
   - TUI、鼠标、剪贴板正常

#### 第二步：给 `v` 增加 `--floter` 子命令（动态协议）

在 `v` 仓库 (github.com/vst93/v) 中：

1. **新增 `--floter` 子命令**
   - `v --floter describe --protocol 1` — 输出完整命令目录
   - `v --floter diagnose --protocol 1` — 输出诊断信息
   - `v --floter complete --protocol 1` — 动态补全（可选，第一版可不做）

2. **CommandDescriptor 结构**
   - 新增 `CommandDescriptor` / `ArgumentDescriptor` / `ExecutionDescriptor` 类型
   - 新增可选 `DescriptorProvider` 接口
   - 新插件实现此接口，老插件自动从 `GetArgs()` 转换
   - `v -h` 帮助输出也从 Descriptor 生成

3. **describe 实现**
   - 遍历 `Plugin.List()`
   - 对每个插件生成 `CommandDescriptor`
   - 输出符合 `provider-description.schema.json` 的 JSON
   - stdout 纯 JSON，无 ANSI

### 阶段 6：端到端验证 ✅ 已完成

1. 输入 `jv -` 能看到参数说明
2. 按 Enter 后在现有终端里真正运行 `v jv ...`
3. TUI、鼠标、剪贴板和移交系统终端都保持正常
4. 卸载或升级 `v` 不影响 Floter 自身版本
5. 三平台测试：Linux、Windows、macOS

### 阶段 7：开放平台 ✅ 已完成

1. ✅ 动态 `complete` 协议完整实现（`1fabb44`）
2. ✅ NPM 分发和签名索引（`7d65efb`）
3. ✅ 第三方 SDK / Wrapper 模板（`7d65efb`）
4. ✅ FEP-4 (NPM Registry Convention)（`f759caa`）
5. ✅ FEP-5 (Permissions and Security)（`f759caa` + `26120ce`）
6. ✅ FEP-6 (Declarative Configuration)（`f759caa` + `26120ce`）
7. ✅ 跨设备同步（`374d961`）

---

## 十二、第一阶段验收目标

这个闭环完成后，再扩展开放平台会稳很多：

1. ✅ 输入 `jv -` 能看到参数说明（`-f` 格式化、`-c` 压缩、`-file` 路径...）
2. ✅ 按 Enter 后在现有终端里真正运行 `v jv ...`
3. ✅ TUI、鼠标、剪贴板和移交系统终端都保持正常
4. ✅ 卸载或升级 `v` 不影响 Floter 自身版本
5. ✅ `v` 新增插件后，Floter 刷新即可自动发现

---

## 十三、Codex 会话中的关键决策记录

以下是设计过程中用户提出的补充要求和 Codex 的响应，作为开发约束：

1. **用户要求：不要写成固定映射关系。**
   → 设计为 Provider 动态协议，Floter 只固定「如何询问 Provider」，不固定 Provider 里面有哪些命令。

2. **用户要求：扩展管理（安装/开关/删除/更新）要在协议中完善。**
   → FEP-3 将持久状态与操作阶段分开，安装/更新/删除都是 Host 操作，不让插件自定义。

3. **用户要求：通过 NPM 平台来管理和安装，让第三方插件制作 NPM 包。**
   → NPM 承担索引和分发，Floter 直接用 Registry API 下载校验，不执行 npm install，不依赖 Node.js。包中只含声明文件和静态资源，不执行 JS。

4. **用户要求：考虑 Linux、Windows、macOS 三平台差异。**
   → 协议中原生设计平台覆写、平台包拆分、平台特定路径/权限/执行逻辑。Windows 不沿用 Unix PATH/单引号/权限逻辑，macOS 区分 Intel/Apple Silicon，Linux 不假设发行版。

5. **用户要求：先规划不用着急开发。**
   → 第一轮只做了代码与架构分析，没有修改两个仓库。后续确认方向后才开始产出协议文档。

6. **用户要求：v 作为官方工具插件，也是一个探索逻辑。**
   → V 作为参考实现，验证整套协议可行性。V 保持独立项目和独立版本，通过 Provider 协议接入而非编译进 Floter。

7. **Codex 确认：现有终端和 PTY 可直接复用，不需要重新设计。**
   → 真正需要新增的是「命令目录和解析层」。`custom.rs` 后端未接入前端，不影响新平台实现。

8. **Codex 确认：Cargo.lock 已有 reqwest/semver/sha2/tar/flate2 依赖。**
   → 安装生命周期所需的核心依赖齐备，无需额外引入。`shell-words` 缺失但可用 `std::process::Command` 替代。

---

## 附录：Codex 会话 plan 跟踪

Codex 在会话中维护的 plan（最终状态）：

| 步骤 | 状态 |
|------|------|
| 审计当前 Floter 架构、工作树与三平台测试基线，确定扩展平台接入边界 | ✅ completed |
| 编写正式扩展规范、跨平台 JSON Schema 与 V Provider 参考协议 | ✅ in_progress (文档已产出) |
| 实现 Rust 扩展包、Provider 发现、缓存、启停、更新与删除后端 | ⬜ pending |
| 实现统一命令目录、参数联想和跨平台结构化 PTY 执行 | ⬜ pending |
| 实现插件发现、安装、管理、更新、诊断与配置页面 | ⬜ pending |
| 为 V 增加动态 describe/complete/diagnose 参考实现并接入 | ⬜ pending |
| 补齐 Linux、Windows、macOS 测试、构建与端到端验证，完成需求审计 | ⬜ pending |

> Codex 在完成协议文档产出后 API 中断，步骤 2 的文档部分实际已完成，代码实现尚未开始。
