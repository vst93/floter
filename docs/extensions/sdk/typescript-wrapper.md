# TypeScript Wrapper 模板

TypeScript 在这里用于生成描述文件、组织 NPM 包和发布脚本，不会在用户机器上
执行。实际 Provider 必须是平台包内可直接启动的程序。

## 目录布局

```text
floter-acme-tools/
  package.json
  floter.extension.json
  provider-description.json
  README.md
  packages/
    linux-x64/package.json
    linux-x64/bin/acme-provider
    darwin-arm64/...
    windows-x64/bin/acme-provider
```

发布时基础目录和每个 `packages/<target>` 目录是独立 NPM 包。平台包只携带
对应目标的二进制；基础包携带 manifest、README 和可选图标。

## package.json

基础包至少要有 `name`、`version`、`keywords`、`files` 和
`floter.manifest`。`keywords` 必须精确包含 `floter-extension`。

```json
{
  "name": "floter-acme-tools",
  "version": "1.0.0",
  "description": "Acme command tools for Floter",
  "license": "MIT",
  "keywords": ["floter-extension"],
  "files": ["floter.extension.json", "provider-description.json", "README.md"],
  "floter": {
    "manifest": "floter.extension.json"
  }
}
```

平台包的 `name` 和 `version` 必须与 registry 记录相符，版本必须与基础包一致：

```json
{
  "name": "floter-acme-tools-linux-x64",
  "version": "1.0.0",
  "license": "MIT",
  "os": ["linux"],
  "cpu": ["x64"],
  "files": ["bin/acme-provider"]
}
```

不要依赖 `preinstall`、`install` 或 `postinstall`：Floter 不会运行这些脚本，
也不会解析 `dependencies` 来组装运行时。

## floter.extension.json

```json
{
  "schemaVersion": "1.0",
  "id": "com.example.acme-tools",
  "name": "Acme Tools",
  "description": "Search and transform Acme resources",
  "homepage": "https://example.com/acme-tools",
  "publisher": {
    "id": "acme",
    "name": "Acme"
  },
  "compatibility": {
    "floter": ">=0.2.3 <1.0.0",
    "providerProtocol": "^1.0"
  },
  "runtime": {
    "type": "managed",
    "platformPackages": {
      "darwin-arm64": "floter-acme-tools-darwin-arm64",
      "darwin-x64": "floter-acme-tools-darwin-x64",
      "linux-arm64": "floter-acme-tools-linux-arm64",
      "linux-x64": "floter-acme-tools-linux-x64",
      "windows-arm64": "floter-acme-tools-windows-arm64",
      "windows-x64": "floter-acme-tools-windows-x64"
    },
    "executable": "bin/acme-provider"
  },
  "provider": {
    "argsPrefix": ["--floter"],
    "describeTimeoutMs": 5000,
    "completeTimeoutMs": 800
  },
  "permissions": ["filesystem-read", "network-fetch"],
  "signatures": {
    "url": "https://example.com/releases/floter-acme-tools-1.0.0.sig",
    "publicKey": "ed25519:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
    "algorithm": "ed25519"
  }
}
```

不发布某个平台时，删除对应的 `platformPackages` 键。`executable` 总是相对于
平台包解压后的根目录，且每个平台包必须提供相同的相对路径。需要 `.exe` 后缀
时可在所有平台包中统一使用该文件名；Unix 不依赖扩展名判断可执行文件。

签名可省略。启用后，URL 必须返回对 `npm pack` 产生的原始 `.tgz` 字节所作的
64 字节 Ed25519 签名，以标准 Base64 文本编码。不要解压、重压或修改 tarball
后再签名。

## provider-description.json

```json
{
  "protocolVersion": "1.0",
  "provider": {
    "id": "com.example.acme-tools",
    "name": "Acme Tools",
    "version": "2.4.0",
    "description": "Acme CLI integration"
  },
  "commands": [
    {
      "id": "search",
      "name": "Search Acme",
      "description": "Search resources by query",
      "aliases": ["find"],
      "keywords": ["acme", "resource"],
      "execution": {
        "program": "self",
        "argsPrefix": ["search"],
        "mode": "capture",
        "workingDirectory": "current"
      },
      "arguments": [
        {
          "names": ["--type"],
          "kind": "enum",
          "description": "Resource type",
          "takesValue": true,
          "values": ["project", "task"]
        },
        {
          "names": ["query"],
          "kind": "string",
          "description": "Search text",
          "required": true
        }
      ]
    }
  ]
}
```

`execution.program: "self"` 代表运行 Provider 本身；其他值必须是平台包
`runtime/` 内的安全相对路径。交互式命令使用 `pty`，短任务使用 `capture`，
交给系统应用打开的操作使用 `external`。

## 静态适配器还是动态 Provider

| 场景 | 选择 |
| --- | --- |
| 命令和参数长期固定，不需要读取环境 | 轻量 Provider，在 `describe` 中输出内嵌的 `provider-description.json` |
| 候选项依赖 cwd、用户输入、账号或远端状态 | 动态 Provider，实现 `describe` 和 `complete` |
| 需要检查外部 CLI、登录状态或系统依赖 | 动态 Provider，再实现 `diagnose` |
| 希望无需安装即可随 Floter 发布 | Host 内置静态适配器，需要向 Floter 仓库贡献代码和描述文件 |

第三方 NPM 包不能直接注册 Host 内置静态适配器。即使描述内容完全静态，平台
包仍需要一个可执行 wrapper 来响应 `--floter describe --protocol 1`。
