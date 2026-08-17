# 使用 Agent 打包 Floter 工具插件

这份文档供希望把现有 CLI/TUI 工具接入 Floter 的用户使用。你可以把下面的
任务模板交给编码 Agent，让它检查工具仓库、实现 Provider 协议、生成 NPM
扩展包并完成本地验证。

Agent 应把本目录中的 JSON Schema 和 FEP 文档作为最终依据，不应只根据本页
示例猜测协议。

用户也可以在 Floter 的“设置 > 集成 > 创建自定义集成”中连接可执行文件，或直接编写 JavaScript、Shell、PowerShell Provider 脚本。界面会自动填写 ID 和版本，并支持从当前 `PATH` 模糊查找命令。

## 平台限制与脚本 runtime

manifest 可用 `platforms` 声明支持的操作系统：`darwin`（macOS）、`linux`、`windows`。这是 allow-list；宿主不加载当前平台未被列出的插件。为了兼容旧包，字段省略时表示支持全部平台。

本地插件还可以使用脚本 runtime：

```json
{
  "schemaVersion": "2.0",
  "distribution": { "type": "local" },
  "runtime": {
    "type": "script",
    "language": "powershell",
    "path": "provider.ps1",
    "versionArgs": []
  },
  "platforms": ["windows"]
}
```

`js` 使用 `node`，`shell` 使用 `sh`，`powershell` 优先使用 `pwsh`、其次使用 `powershell`。解释器必须存在于 Floter 进程的 `PATH` 中。可视化创建会为脚本生成静态命令描述；如果需要动态补全、诊断或配置协议，再实现完整 Provider Protocol。

## 先选择接入方式

| 目标 | Manifest 组合 | 产物 |
| --- | --- | --- |
| 发布到 NPM，并由 Floter 安装工具 | `npm + bundled + executable` | 一个基础包，加每个平台一个原生运行时包 |
| 发布到 NPM，但复用系统已有工具 | `npm + system + executable` | 一个只含 manifest、README 等集成描述的基础包 |
| 只在本地接入已有工具 | `local + system + executable` | manifest；Floter 从 PATH 或指定路径寻找程序 |
| 只在本地编写脚本 Provider | `local + script + static-descriptor` | manifest、静态命令描述和 JS、Shell 或 PowerShell 脚本 |
| 工具暂时不能实现 Provider 协议 | Provider wrapper | 一个很薄的原生程序，负责协议响应并转发到原工具 |

需要公开搜索、安装和更新集成描述时，选择 NPM 分发。工具已有成熟的 Homebrew、
Cargo 或系统包安装方式时，优先选择 `npm + system`；只有希望 Floter 同时管理
工具二进制时才选择 `npm + bundled`。Floter 不执行 NPM 包内 JavaScript，也不
运行 `preinstall`、`install`、`postinstall`。

## 交给 Agent 前准备的信息

至少提供以下内容。未知项可以要求 Agent 先检查仓库再给出建议，但不要让它
擅自决定发布身份、权限或远端发布操作。

```text
工具仓库或本地路径：
工具名称：
工具当前版本：
可执行文件名称：
主要命令及典型调用：
是否包含交互式 TUI：
希望支持的平台和架构：
NPM 包名或 scope：
扩展稳定 ID：
发布者 ID 和名称：
项目主页：
许可证：
是否允许 Agent 修改工具源码：
是否已经登录 NPM：
是否需要 Ed25519 签名：
```

扩展 ID 应长期稳定，建议使用反向域名，例如 `com.example.acme-tools`。
manifest 的 `id` 与 Provider 输出的 `provider.id` 必须完全一致。

## 可直接复制的 Agent 任务模板

把尖括号内容替换成实际值，再将整段交给 Agent：

```text
请把下面的 CLI/TUI 工具制作成可安装的 Floter NPM 集成：

- 工具源码：<仓库 URL 或本地路径>
- 扩展 ID：<例如 com.example.acme-tools>
- 扩展名称：<显示名称>
- 发布者：<publisher id / name>
- NPM 基础包：<例如 @example/floter-acme-tools>
- 工具运行时：<system，复用系统安装；或 bundled，由 Floter 安装>
- 支持目标：<darwin-arm64, darwin-x64, linux-arm64, linux-x64,
  windows-arm64, windows-x64 中的实际目标>
- 主要命令：<命令和示例；不知道时请从源码及 --help 中整理>
- 允许修改工具源码：<是/否>
- 签名要求：<暂不签名/使用现有 Ed25519 密钥；不要自行创建或上传密钥>

请严格阅读并遵守 Floter 仓库中的：

- docs/extensions/FEP-1-package.md
- docs/extensions/FEP-2-provider.md
- docs/extensions/FEP-3-lifecycle.md
- docs/extensions/FEP-4-npm-registry.md
- docs/extensions/FEP-5-permissions.md
- docs/extensions/FEP-6-declarative-config.md
- docs/extensions/schemas/floter-extension.schema.json
- docs/extensions/schemas/provider-description.schema.json
- docs/extensions/sdk/checklist.md

工作要求：

1. 先检查工具的入口、命令树、参数、配置、外部依赖和各平台构建方式，给出
   简短的接入判断。不要根据工具名称猜命令。
2. 优先让原工具实现 Provider 协议；无法修改原工具时，再实现最薄的原生
   wrapper。不要把 Node.js 或 shell 脚本作为 bundled runtime 依赖。
3. 实现 `describe`；只有确有动态候选时才实现 `complete`，需要依赖检查时
   实现 `diagnose`，需要 Floter 管理配置时实现 `config`。
4. stdout 只输出协议 JSON，日志写 stderr。所有执行命令必须使用结构化
   program/argv，不拼 shell 字符串。
5. 始终生成基础 NPM 包。选择 bundled runtime 时再生成各平台 NPM 包，所有包
   使用相同 SemVer；选择 system runtime 时不打包外部工具，并在 README 写明
   安装命令和最低工具版本。先在本地构建和 `npm pack`，不要发布。
6. manifest 只声明工具真实需要的权限。权限是用户披露，不是系统沙箱。
7. 使用仓库 Schema 验证 manifest 和 Provider 描述，并实际启动每个可用平台
   产物验证协议。检查 tarball 内容、路径、可执行位和包版本一致性。
8. 更新或新增 README，写明安装方式、外部依赖、支持平台、权限理由和限制。
9. 完成后报告：改动文件、包结构、命令映射、权限理由、验证命令与结果、
   未验证平台、待用户决定事项。

授权边界：

- 可以读取源码、修改当前工作区、构建、测试、运行 `npm pack`。
- 未经我明确确认，不得执行 `npm publish`、创建 release、推送 Git、创建远端
  仓库、上传签名或生成/替换正式密钥。
- 如果缺少发布身份、平台构建环境、签名材料或工具行为信息，先完成其他可
  验证工作，再明确列出阻塞项。

验收标准：

- 两份 JSON 均通过仓库 Schema。
- Provider ID 与 manifest ID 相同。
- `describe` 在 5 秒内返回纯 UTF-8 JSON。
- bundled runtime 的基础包和平台包版本完全一致。
- bundled 平台包无需 Node.js 和生命周期脚本即可运行。
- system runtime 的 tarball 不包含外部工具，且可从 PATH 找到并验证真实工具。
- 每个命令的 argv、工作目录、执行模式和权限与工具真实行为一致。
- `npm pack` 后检查过实际 tarball，而不只是源目录。
- 没有把 token、私钥、用户配置或本机绝对路径放进包。
```

## bundled runtime 的包结构

推荐在工具仓库中建立独立打包目录，具体名称可以服从原仓库约定：

```text
packaging/floter/
├── base/
│   ├── package.json
│   ├── floter.extension.json
│   └── README.md
├── darwin-arm64/
│   ├── package.json
│   └── bin/<tool>
├── linux-x64/
│   ├── package.json
│   └── bin/<tool>
└── windows-x64/
    ├── package.json
    └── bin/<tool executable>
```

只创建实际支持的平台目录。基础包的最小 `package.json`：

```json
{
  "name": "@example/floter-acme-tools",
  "version": "1.0.0",
  "description": "Acme Tools integration for Floter",
  "license": "MIT",
  "keywords": ["floter-extension"],
  "files": ["floter.extension.json", "README.md"],
  "floter": {
    "manifest": "floter.extension.json"
  }
}
```

对应的最小 `npm + bundled` manifest：

```json
{
  "schemaVersion": "2.0",
  "id": "com.example.acme-tools",
  "name": "Acme Tools",
  "description": "Acme command line tools",
  "homepage": "https://example.com/acme-tools",
  "publisher": {
    "id": "example",
    "name": "Example"
  },
  "compatibility": {
    "floter": ">=0.3.0",
    "providerProtocol": "^1.0"
  },
  "distribution": {
    "type": "npm"
  },
  "runtime": {
    "type": "bundled",
    "platformPackages": {
      "darwin-arm64": "@example/floter-acme-tools-darwin-arm64",
      "linux-x64": "@example/floter-acme-tools-linux-x64",
      "windows-x64": "@example/floter-acme-tools-windows-x64"
    },
    "executable": "bin/acme"
  },
  "provider": {
    "type": "executable",
    "argsPrefix": ["--floter"],
    "describeTimeoutMs": 5000,
    "completeTimeoutMs": 800
  },
  "permissions": []
}
```

平台包的最小 `package.json`：

```json
{
  "name": "@example/floter-acme-tools-linux-x64",
  "version": "1.0.0",
  "description": "Acme Tools runtime for Floter on Linux x64",
  "license": "MIT",
  "files": ["bin/acme"]
}
```

`runtime.executable` 只有一个跨平台值，因此所有平台包必须在这个相对路径提供
可启动文件；Agent 应在打包阶段复制或重命名平台构建产物，而不是把本机绝对
路径写入 manifest。

每个平台包至少包含正确的 `name`、`version`、`files` 和可执行文件。基础包与
平台包必须使用相同版本。平台包名称遵循：

```text
<base-package>-<os>-<arch>
```

例如：

```text
@example/floter-acme-tools
@example/floter-acme-tools-darwin-arm64
@example/floter-acme-tools-linux-x64
@example/floter-acme-tools-windows-x64
```

## system runtime 的 NPM 包结构

工具已经通过 Homebrew、Cargo 或系统包管理器分发时，只需要基础包：

```text
packaging/floter/base/
├── package.json
├── floter.extension.json
└── README.md
```

`package.json` 与前面的基础包相同。最小 `npm + system` manifest：

```json
{
  "schemaVersion": "2.0",
  "id": "com.example.acme-tools",
  "name": "Acme Tools",
  "publisher": {
    "id": "example",
    "name": "Example"
  },
  "compatibility": {
    "floter": ">=0.3.0",
    "providerProtocol": "^1.0"
  },
  "distribution": {
    "type": "npm"
  },
  "runtime": {
    "type": "system",
    "executableNames": ["acme", "acme.exe"],
    "versionArgs": ["--version"]
  },
  "provider": {
    "type": "executable",
    "argsPrefix": ["--floter"],
    "describeTimeoutMs": 5000,
    "completeTimeoutMs": 800
  },
  "permissions": []
}
```

这里 NPM 版本是集成版本，`versionArgs` 探测的是工具版本，两者可以不同。Floter
更新或卸载该 NPM 集成时只处理集成文件，不升级或删除系统工具。README 必须
写明工具的安装方式、可执行文件名和版本要求。

## Provider 最小契约

如果 manifest 中配置：

```json
{
  "provider": {
    "argsPrefix": ["--floter"]
  }
}
```

Host 会按需调用：

```text
<tool> --floter describe --protocol 1
<tool> --floter complete --protocol 1
<tool> --floter diagnose --protocol 1
<tool> --floter config --protocol 1
```

Provider 必须遵守：

- 协议 JSON 写 stdout，日志写 stderr。
- stdout 必须是单个 UTF-8 JSON，不含 ANSI、前缀文字或调试日志。
- 参数以 argv 解析，不能依赖 shell 拼接。
- `describe` 不修改用户配置。
- 交互命令使用 `pty`，移交系统终端使用 `external`。
- 旧 `capture` 值只会被当前 Host 兼容性归一化为 `pty`，新扩展不要使用。

最小 `describe` 输出可参考：

```json
{
  "protocolVersion": "1.0",
  "provider": {
    "id": "com.example.acme-tools",
    "name": "Acme Tools",
    "version": "2.4.0",
    "description": "Acme command line tools"
  },
  "commands": [
    {
      "id": "search",
      "name": "Search",
      "description": "Search Acme resources",
      "aliases": ["find"],
      "keywords": ["acme", "resource"],
      "execution": {
        "program": "self",
        "argsPrefix": ["search"],
        "mode": "pty",
        "workingDirectory": "current"
      },
      "arguments": [
        {
          "names": ["--type"],
          "kind": "enum",
          "description": "Resource type",
          "takesValue": true,
          "values": ["project", "task"]
        }
      ]
    }
  ]
}
```

完整字段定义见
[`provider-description.schema.json`](schemas/provider-description.schema.json)。

## Manifest 检查重点

Agent 生成 `floter.extension.json` 时应逐项确认：

- `schemaVersion` 新包使用 `2.0`；Host 仍兼容现有 `1.0` manifest。
- `id` 与 Provider ID 一致且不会随包名或版本变化。
- `compatibility.floter` 和 `compatibility.providerProtocol` 使用真实兼容范围。
- `distribution.type` 表示集成从 `npm`、`local` 或 `built-in` 分发。
- `runtime.type` 为 `bundled` 时，每个 `platformPackages` 值指向真实平台包；
  为 `system` 时，工具由用户的系统包管理器维护。
- 完整 Provider Protocol 集成使用 `provider.type: executable`；只需把普通命令或
  脚本映射成固定命令目录项时可使用 `static-descriptor`。可视化创建器会自动生成
  descriptor，Agent 打包时也必须随包携带并校验对应 JSON。
- `runtime.executable` 是平台包内的安全相对路径。
- `provider.argsPrefix` 与实际入口一致。
- `platformOverrides` 只处理真实的平台差异。
- `permissions` 没有为了省事而全选。
- 签名字段只引用用户提供或明确批准的正式材料。

权限选择依据：

| 权限 | 何时声明 |
| --- | --- |
| `filesystem-read` | Provider 或命令读取用户文件、目录 |
| `filesystem-write` | 创建、修改或删除文件 |
| `network-fetch` | 发起出站网络请求 |
| `process-spawn` | 命令描述要求 Host 运行 `self` 以外的程序；工具自行启动子进程只能披露和审计，Host 无法拦截 |
| `clipboard-read` | 读取剪贴板 |
| `clipboard-write` | 写入剪贴板 |
| `environment` | 需要继承 Host 环境变量 |

这些权限是披露和用户确认边界，不是内核沙箱。Agent 必须在 README 中解释每项
权限对应的实际行为。

## 本地验收建议

Agent 至少应执行等价于以下检查，并报告真实输出。命令需要按工具实际入口和
项目工具链调整。

### 1. 协议冒烟测试

```bash
./bin/acme --floter describe --protocol 1 | jq .
printf '%s' '{"command":"search","args":["--type",""]}' \
  | ./bin/acme --floter complete --protocol 1 \
  | jq .
./bin/acme --floter diagnose --protocol 1 | jq .
```

未实现的可选操作可以省略测试，但 Agent 必须明确说明。还应单独捕获 stderr，
确认 stdout 没有混入日志。

### 2. Schema 验证

使用支持 JSON Schema Draft 2020-12 的验证器分别校验：

```text
floter.extension.json
  against docs/extensions/schemas/floter-extension.schema.json

provider-description.json 或 describe 的实际输出
  against docs/extensions/schemas/provider-description.schema.json
```

不要只验证手写示例；必须验证 Provider 实际输出。

### 3. NPM 包内容

在每个包目录运行：

```bash
npm pack --dry-run
npm pack
tar -tzf <生成的 tgz>
```

检查：

- 没有源码秘密、测试凭据、用户配置、私钥或无关大文件。
- 基础包包含 manifest 和 README。
- 平台包包含 `package.json` 和 manifest 指定的可执行路径。
- Unix 产物可执行。
- tarball 中没有绝对路径、`..`、符号链接或硬链接逃逸。

### 4. 平台矩阵

Agent 的最终报告应使用明确矩阵，不能用“应该支持”代替测试：

| 目标 | 构建 | Provider 启动 | describe | 命令执行 | 结果 |
| --- | --- | --- | --- | --- | --- |
| `darwin-arm64` | 已验证/未验证 |  |  |  |  |
| `darwin-x64` | 已验证/未验证 |  |  |  |  |
| `linux-arm64` | 已验证/未验证 |  |  |  |  |
| `linux-x64` | 已验证/未验证 |  |  |  |  |
| `windows-arm64` | 已验证/未验证 |  |  |  |  |
| `windows-x64` | 已验证/未验证 |  |  |  |  |

无法在当前机器或 CI 验证的平台必须标成未验证，不能仅凭交叉编译成功标成运行
通过。

## 发布前的确认点

本地验证完成后，Agent 应停下来向用户展示：

1. 最终基础包名和集成版本；bundled runtime 还要列出平台包及统一版本。
2. `npm pack` 生成的文件清单和包大小。
3. 支持及未验证的平台矩阵。
4. 权限列表及逐项理由。
5. 签名方案和公钥来源。
6. 拟执行的准确 `npm publish` 命令和发布顺序。

用户明确批准后才能发布。bundled runtime 按“平台包优先、基础包最后”的顺序；
system runtime 只发布基础包。发布后还应从 registry 下载实际 tarball，重新
验证版本、integrity、安装、运行、更新和卸载。

## 本地 system runtime 的简化任务模板

如果工具已经由 Homebrew、Cargo、系统包管理器或用户手工安装，不需要发布
NPM 平台包，可以交给 Agent：

```text
请为 <工具路径或仓库> 制作 Floter 本地系统工具集成 manifest。

1. 检查工具是否已经实现 Provider 协议；没有时，在允许修改源码的前提下实现
   describe，并按需实现 complete、diagnose、config。
2. 使用 schemaVersion 2.0、distribution.type = local、runtime.type = system、
   provider.type = executable；executableNames 按优先级填写真实命令名，
   versionArgs 使用不会修改状态的版本查询参数。
3. 使用 Floter 仓库 Schema 验证 manifest 和 Provider 实际输出。
4. 实际从 PATH 找到程序并完成 describe 与至少一个命令的冒烟测试。
5. 报告 manifest 路径、需要的权限、未验证行为，以及通过“设置 > 集成 >
   连接本地工具”选择 manifest 的操作步骤。
6. 不发布 NPM，不修改外部程序安装，不执行任何远端操作。
```

系统工具集成在界面中使用“断开连接”；该操作只解除 Floter 注册，不应删除
外部工具。

## 相关资料

- [扩展包与平台运行时](FEP-1-package.md)
- [Provider 运行协议](FEP-2-provider.md)
- [安装生命周期与安全](FEP-3-lifecycle.md)
- [NPM Registry Convention](FEP-4-npm-registry.md)
- [权限与安全](FEP-5-permissions.md)
- [声明式配置](FEP-6-declarative-config.md)
- [第三方 SDK](sdk/README.md)
- [发布检查清单](sdk/checklist.md)
