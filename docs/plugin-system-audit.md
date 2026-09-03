# Floter 插件系统全面审计与重新规划

审计基线：`main` 分支当前工作树，代码证据截至 2026-08-22。本文以 Rust/React 实现为准，FEP 和开发计划只用于核对“声明与实现是否漂移”。本次只读审计未修改任何源代码。

## 一、现状架构盘点

### 1.1 总体模块关系

真实调用链如下：

```text
React ExtensionsPanel
  -> @tauri-apps/api/core invoke()
  -> src-tauri/src/lib.rs invoke_handler 注册
  -> commands/extensions.rs（参数校验、用户确认、状态编排）
  -> extensions/{install,lock,transaction,config,sync,catalog,provider,...}
  -> 文件系统 / NPM Registry / 外部 runtime / provider 子进程
```

Rust 侧 `extensions/mod.rs:74-178` 创建单例 `ExtensionState`。它持有：

- `ExtensionPaths`：配置根、程序版本、用户数据、下载缓存、扩展 lock、工具绑定 lock、官方索引版本状态；
- `ProviderManager`：provider `describe/complete/diagnose/config` 子进程调用、超时、输出限制、缓存；
- `static_adapters`：编译/仓库内置的静态适配器；
- `mutation_lock`：进程内串行化安装、更新、配置和导入；
- `tool_lock`：用户为 system runtime 选择的可执行文件指纹绑定；
- `ExecutionPlanCache`：把带真实路径、argv、环境变量的执行计划留在 Rust 内，只把一次性 token 送往 IPC（`mod.rs:181-274`）。

IPC 命令在 `src-tauri/src/lib.rs:1193-1234` 注册。插件系统命令集中在 `commands/extensions.rs`，包括列表、搜索、安装/连接、启停、更新、pin/channel、reinstall、repair、rollback、describe/diagnose/health/reprobe、配置、导入导出、catalog search/complete。终端实际启动仍由宿主命令链处理，扩展命令只生成结构化计划或 launch 描述，不把任意 shell 字符串交给前端。

### 1.2 Manifest schema 与实际解析

主模型是 `ExtensionManifest`（`src-tauri/src/extensions/manifest.rs:15-45`），解析流程为 JSON -> bundled Draft 2020-12 schema -> serde `deny_unknown_fields` -> 相对路径与组合约束（`manifest.rs:237-251,336-463`）。当前字段分组：

| 分组 | 真实字段/语义 |
|---|---|
| 身份 | `schemaVersion`、`id`、`name`、`publisher`、homepage/icon |
| 兼容 | Floter semver、provider protocol semver；预发布宿主版本会去掉 pre-release 再匹配（`manifest.rs:304-333`） |
| 分发 | `npm`、`local`、`built-in`（`manifest.rs:48-54`） |
| runtime | `bundled`（平台 NPM 包+入口）、`system`（PATH/用户选择）、`script`（本地脚本） |
| provider | executable 或 static-descriptor、argv 前缀、超时、显式环境 |
| 平台 | OS allow-list、platform override、platform package 映射 |
| 完整性/信任 | NPM SRI、可选 Ed25519 tarball signature；官方身份由独立 signed index 追加判断 |
| 权限 | filesystem read/write、network、process spawn、clipboard、environment |
| 生命周期 | shell completion、配置模板、capability probes、launch config（`lifecycle.rs:17-28`） |
| artifacts | bundled runtime 中 provider/public/helper 二进制及 shim 元数据 |

Schema 本身覆盖类型、长度、枚举和路径形式（`docs/extensions/schemas/floter-extension.schema.json:16-340`），但部分跨字段规则只在 Rust 二次校验，例如 provider descriptor、artifact provider 唯一性、`program` 与 `process-spawn` 的关系。因此“通过 JSON Schema”不是完整的包合法性证明。

### 1.3 生命周期与状态机

持久状态只有 `enabled`、`disabled`、`broken`（`lock.rs:52-69`），安装阶段另由 transaction journal 表达：

```text
not-installed -> enabled <-> disabled
                         \-> broken

resolved -> downloading -> downloaded -> verified -> staged
          -> activated -> cleaned
```

实现的 transaction 状态和单向转移在 `transaction.rs:24-65`。启动时 `ExtensionState::from_paths_with_official_index` 先 `transaction::recover`，再恢复配置（`mod.rs:143-178`）。

实际操作路径：

1. **安装 NPM**：解析 registry 精确版本，下载基础包，安全解包，读取 manifest，校验签名/官方索引、兼容性、权限、平台和 provider；bundled runtime 再下载同版本平台包；执行 `describe` 和声明的 required probes；写入版本目录并通过 `transaction::commit_version` 提交（`install.rs:1548-1934`）。
2. **连接 local/built-in**：把 manifest/descriptor/script 放入 `extension-data/<id>/integration`，system runtime 只记录外部路径；不复制外部可执行文件（`install.rs:762-869,1943-2140`）。
3. **更新**：仅 NPM；无显式版本时只接受 patch，pinned 拒绝自动更新，minor/major 需显式版本（`install.rs:1015-1059`）。
4. **reinstall**：读取当前 package/version/SRI，直接调用 `install_managed` 重新下载并以同版本进入 staging，未调用 uninstall（`install.rs:1062-1091`）。
5. **repair**：先做落盘 tree integrity、manifest identity、provider/runtime 检查；NPM 用锁定版本/SRI 重装，system runtime 重新发现并 describe（`commands/extensions.rs:1221-1261`、`install.rs:1136-1213`）。
6. **rollback**：要求 `previous_version` 目录存在并通过 previous content integrity，交换 current/previous 元数据后走 lock transaction（`install.rs:1319-1459`）。当前只保留一个 previous 版本。
7. **卸载**：NPM 扩展目录先 rename 到临时 removing 目录，再提交 lock，最后删除程序；system runtime 不删除外部程序；可选删除 data（`install.rs:1244-1317`）。

### 1.4 目录布局与数据归属

默认根为 `dirs::config_dir()/floter`（`mod.rs:85-110`）：

```text
<config>/floter/
├─ extensions.lock.json          # 宿主安装事实、版本、状态、信任摘要
├─ tool-lock.json                # system runtime 的用户选择/指纹
├─ official-index-state.json     # 官方索引最高接受版本
├─ extensions/
│  ├─ <id>/
│  │  ├─ versions/<version>/     # NPM 解包版本树
│  │  ├─ current.json            # lock 的 runtime-facing projection
│  │  ├─ shims/                  # public artifact 稳定入口
│  │  └─ .transactions/*.json
│  ├─ .staging/
│  └─ .transactions/
├─ extension-data/<id>/
│  ├─ integration/               # local/built-in manifest/descriptor/script
│  ├─ config.json                # host-owned 配置提交指针
│  ├─ config-secrets/<generation>.json  # 受限权限 secret generation
│  ├─ completions/               # 生命周期生成物
│  └─ sessions/, health.json
└─ extension-cache/
   ├─ providers/                 # describe 缓存
   ├─ NPM/源下载缓存
   └─ 导入回滚快照/临时文件
```

宿主拥有安装事实、启用状态、版本策略、工具绑定和 `owner=host` 配置。插件/工具拥有 `owner=tool` 配置及运行时自己的文件，但当前 provider 是原生进程，filesystem/network/clipboard 权限不是 OS 沙箱。local 生成集成的程序文件放在宿主 `extension-data`，这使“程序”和“数据”边界比 NPM 清晰度低。

### 1.5 Provider API 与信任边界

Provider 通过独立进程 stdin/stdout JSON 协议暴露：`describe`、`complete`、`diagnose`、可选 `config`（`provider.rs:245-405`）。Host 为每次调用清空环境（没有 `environment` 时）、注入 manifest/config 显式变量、限制 stdout/stderr、超时 kill（`provider.rs:408-490`）。`describe` 还校验 provider id 等于 extension id，并校验每个 command descriptor。

命令 descriptor 可声明 `program=self` 或 runtime root 内相对程序；后者必须 `process-spawn`，linked provider 只能执行 self（`provider.rs:526-587,696-728`）。带真实 program/argv/env/cwd 的计划由 `ExecutionPlanCache` 隐藏，IPC 只获得单次 token。这是当前最重要的 Host 执行边界，但 native provider 自己仍可绕过这些声明直接访问 OS。

### 1.6 前端面板与 IPC 交互

`ExtensionsPanel.tsx` 把体验分为 Installed/Discover/Updates 三个 tab，首次加载调用 `extensions_list`，详情并行调用 describe/diagnose/config/health（`ExtensionsPanel.tsx:613-737`）。安装/更新/reinstall 在调用命令前先获取权限摘要并用确认对话框；repair、rollback、pin/channel、启停、卸载由 `useExtensionActions` 串行化并刷新列表（`ExtensionsPanel.tsx:802-975`）。Discover 目前固定显示 NPM 来源（`ExtensionsPanel.tsx:1634-1641`），没有真正的 source/provider 选择器。面板还提供本地 manifest/package 连接、自定义 integration 编辑、配置复制/导出、扩展集合导入/导出。

## 二、问题清单

严重度定义：P0 = 可导致数据/代码执行边界失控或不可恢复；P1 = 用户可见错误状态、事务/信任/兼容性高风险；P2 = 功能缺口、维护性或文档漂移。

| 位置 | 严重度 | 问题描述 | 用户影响 |
|---|---|---|---|
| `src-tauri/src/extensions/transaction.rs:469-503,511-547` | P1 | lock、`current.json`、journal 是三个独立 durable 写入；代码用 journal/recovery 处理崩溃，但“同一事务”仍是 best-effort，提交后 pointer 写失败可能让 runtime projection 暂时落后。 | 崩溃/断电窗口可能出现 lock 与 shim/pointer 不一致；恢复依赖启动扫描而非单一提交记录。 |
| `src-tauri/src/extensions/transaction.rs:310-373` | P1 | recovery 以 lock 与 journal 内容相等推断 committed；对跨文件写入没有故障注入覆盖每个 fsync/rename 点，且 `current.json` 仍是可见真源之一。 | 极端 I/O 故障下难以证明“旧版本始终可用”，问题排查困难。 |
| `src-tauri/src/extensions/install.rs:1062-1091` | P1 | reinstall 已去掉“先 uninstall”的明显错误，但同版本 target 仍依赖 backup/lock/pointer 多步提交；没有面向用户的“旧目录保留直到新版本完全可运行”契约测试。 | reinstall 下载、describe 或权限失败时通常可恢复，但回归风险与成功/失败语义没有稳定 API 保证。 |
| `src-tauri/src/extensions/lock.rs:74-120` | P1 | lock 保存权限只有当前 manifest 的 `permissions` 间接表达，没有 approved set、批准时间、批准来源或审计事件。 | 无法解释“谁在何时批准了新增权限”；导入/更新后的合规审计和 UI 对比不足。 |
| `src-tauri/src/extensions/install.rs:934-950` | P1 | 权限批准是一次 IPC 请求中的完整数组比较，不绑定 manifest digest、包版本或用户确认上下文。 | 同一进程内若请求材料变化，批准对象的可追溯性不足；未来异步安装/队列化容易误用旧批准。 |
| `src-tauri/src/extensions/manifest.rs:193-203`、`provider.rs:408-490` | P1 | 只有 `environment` 和 descriptor-driven `process-spawn` 在 Host 执行层强制； filesystem/network/clipboard 仅披露，native provider 可直接绕过。FEP 已承认这是 disclosure 而非 sandbox，但产品面板容易被理解为“权限控制”。 | 恶意/被攻破的原生插件仍拥有宿主用户权限；权限复选框不能降低实际破坏半径。 |
| `src-tauri/src/extensions/install.rs:1558-1613`、`official_index.rs:130-176` | P2 | 官方签名索引、pinned root、过期和 anti-rollback 已实现；但每次安装都在线 fetch，网络不可用不使用编译内置 payload，`official_verified` 会变成 false。 | 离线环境看不到官方身份；用户可能把“签名通过”与“官方发布”混为一谈。 |
| `src-tauri/src/extensions/official_index.rs:108-127` | P2 | index 只绑定 extension id/package/publisher/signing key，不声明版本范围、撤销状态、发布渠道或包 tarball digest。 | 官方信任不能表达撤销/恶意版本/渠道策略，仍要依赖 NPM SRI 与 publisher key。 |
| `src-tauri/src/extensions/lock.rs:52-69`、全仓库 `rg ExtensionStateKind::Broken` 只有枚举/转移测试 | P1 | `broken` 是持久状态但没有实际写入路径；repair/diagnose 失败只返回错误，列表无法稳定显示“已损坏”。 | 用户看到的是“操作失败”而不是可恢复的 broken 状态，启动后也没有明确恢复入口。 |
| `src-tauri/src/commands/extensions.rs:1405-1474` | P1 | `extensions_reprobe` 硬编码 `--version`/`--help`，不执行 manifest 中声明的 lifecycle probes；安装时却执行声明 probes（`install.rs:1752-1803`）。 | 安装健康检查与手动重探针结果不一致，required capability 可能被误判。 |
| `src-tauri/src/extensions/lifecycle.rs:17-28`、`commands/extensions.rs:1478-1559` | P1 | manifest 有 launch/cwd/terminal/restore policy，但 `extensions_launch` 使用硬编码 `InheritActiveSession`、reattach 和终端 profile，未读取扩展 lifecycle 配置。 | 扩展声明的启动策略不生效；协议字段和产品行为漂移。 |
| `src-tauri/src/extensions/config.rs:125-191,820-892` | P2 | host config 的 secret generation + `config.json` 指针已解决双 JSON 原子性；但配置提交与扩展安装/lock 提交不在同一事务。 | 更新 manifest/schema 与配置迁移跨步骤失败时，版本和配置可能分属不同代。 |
| `src-tauri/src/extensions/sync.rs:313-428,734-810` | P1 | 本地导入具备 preflight、快照和全量回滚；快照是递归复制，restore 再分别写目录、lock、current pointer，仍非 crash-consistent 单事务。 | 进程在导入恢复中崩溃可能留下部分目录；大扩展导入也会显著放大 I/O。 |
| `src-tauri/src/extensions/sync.rs:65-218`、`ExtensionsPanel.tsx:812-840` | P2 | “sync” 实际是 JSON 文件导入/导出，没有远端 transport、冲突解决、加密或设备身份。代码和面板目前已按本地文件表达，但历史计划仍使用跨设备同步措辞。 | 用户预期跨设备自动同步却只能手工传文件；密码被脱敏后不能自动恢复。 |
| `src-tauri/src/extensions/install.rs:1244-1317` | P1 | 卸载先提交 lock，再删除 staged removal 目录；删除失败会返回错误但 lock 已无扩展，后续恢复依赖人工 repair/残留扫描。 | I/O 故障时可能出现“列表已卸载但磁盘残留”，空间泄漏和错误提示不一致。 |
| `src-tauri/src/extensions/install.rs:1215-1242`、`transaction.rs:209-250` | P2 | 只保留 current + 一个 previous，成功更新后立即清理更老版本；没有 retention policy、用户选择或多级回滚。 | 连续更新/坏发布后只能回退一代，恢复能力弱于 VS Code 等成熟生态。 |
| `src-tauri/src/extensions/catalog.rs` 与 `commands/extensions.rs` 多处 | P2 | catalog、inventory、tool-lock、provider cache、manifest/lock 是多个子系统，各自有缓存/状态和刷新逻辑；没有统一 ExtensionRepository/RuntimeBinding 聚合根。 | 需求变更需同时修改多个模块，状态字段容易重复和漂移。 |
| `src-tauri/src/extensions/manifest.rs:304-333`、`lock.rs:117-123`、`ExtensionsPanel.tsx:920-930` | P2 | semver compatibility、NPM channel、tool version、Floter version 分属不同策略；pin/channel 有命令/UI，但 lock 只记录字符串，不记录解析后的约束/更新来源。 | 用户难以知道“锁定的是版本、dist-tag 还是升级策略”；跨宿主升级兼容边界不透明。 |
| `docs/DEVELOPMENT_PLAN.md:6,158-167,852-1043,1088-1103` 与现状代码 | P2 | 文档同时声称 FEP/阶段 1-7 全部完成，又保留“代码实现尚未开始”、动态 V、三平台 E2E pending；FEP 文件本身仍是 Draft。 | 后续迭代会按过时完成声明排期，无法区分协议、实现和验证证据。 |

### 2.1 历史包袱逐项判定

| 历史问题 | 判定 | 代码证据与结论 |
|---|---|---|
| current.json 指针 + lock 两次落盘 | 🟡 半成品 | 有 journal、fsync、启动 recovery（`transaction.rs:253-373`），但 commit 仍在 `lock.save` 后另写 pointer（`transaction.rs:483-503`）；不是单一提交真源。 |
| 删除再重装 | ✅ 已解决（实现层） | `reinstall` 直接复用锁定版本的 staging/install（`install.rs:1062-1091`），未调用 uninstall；同版本故障恢复测试基础已存在，但应补 API 级故障注入。 |
| env_clear、非 self 程序限制 | ✅ 已解决（声明范围内） | `provider.rs:408-430` 清环境；`provider.rs:526-587,696-728` 强制 process-spawn 与 bundled runtime root。filesystem/network/clipboard 仍明确不是沙箱。 |
| 包自带 key+sig 无官方可信源 | ✅ 已解决（当前实现） | pinned development root、signed index、publisher key allow-list、过期和 anti-rollback 在 `official_index.rs:13-63,245-314`；在线索引不可用时不会伪造 official。 |
| 配置双 JSON 原子性 | ✅ 已解决（host config） | secret generation 先受限写入并 fsync，`config.json` 只提交 generation 指针（`config.rs:820-892`），启动可修复悬空 generation（`config.rs:906-960`）。 |
| “跨设备同步”实为导入导出 | ✅ 已澄清为本地导入/导出；远端未实现 | `sync.rs:65-218,313-428` 是文件格式和本地事务；没有 Cloud/Git/WebDAV transport。产品文案仍需统一。 |
| repair 只是打开首页 | ✅ 已解决 | `extensions_repair` 先 verify，NPM 按锁定 SRI 重装，system runtime reconnect（`commands/extensions.rs:1221-1261`）。 |
| FEP 文档与代码漂移 | ❌ 未解决 | `DEVELOPMENT_PLAN.md` 的“全部完成”和 pending/“代码实现尚未开始”并存；FEP 仍标 Draft，且 reprobe/launch 行为已与 lifecycle 声明分叉。 |

## 三、与成熟方案的对比差距

| 维度 | Floter 当前 | VS Code Extensions | Obsidian Plugins | Raycast Store | 差距/取舍 |
|---|---|---|---|---|---|
| 包格式 | NPM tarball + 自定义 manifest，支持 bundled/system/script | VSIX，manifest + extension host API | 社区 GitHub/zip，manifest.json | 商店源码/构建产物与审核元数据 | Floter 运行时类型更多，但分发模型更复杂，缺统一 package identity/发布元数据。 |
| 执行隔离 | 独立 provider 进程；仅 env/process-spawn Host 强制，原生权限不受限 | Extension Host 进程，权限主要靠 API 面和信任模式 | 同进程 JavaScript，明确“社区代码可执行” | 沙箱/API 约束更强，商店审核 | Floter 诚实披露了 native 风险，但缺 OS sandbox 和可审计 API grant。 |
| API 面 | describe/complete/diagnose/config + command descriptor；Host 只暴露结构化执行计划 | 稳定的 host API、事件、存储、权限、版本化 | Plugin API、workspace/vault/events | 命令、表单、网络、持久化等 SDK | Floter API 面窄而适合 CLI，但没有稳定 SDK ABI、事件/迁移/取消语义。 |
| 信任 | NPM SRI；可选 publisher Ed25519；signed official index | Marketplace publisher、签名/审核、trust prompts | 社区审核，用户手工安装 | 官方商店审核、签名/发布渠道 | Floter 的密码学根已具备，但缺撤销、版本范围、审核状态和离线信任缓存。 |
| 生命周期 | enabled/disabled/broken；install journal；一代 rollback | install/activate/deactivate/uninstall，版本兼容与自动更新成熟 | enable/disable，手工更新/回退 | install/update/disable，商店驱动 | Floter journal 更强但状态边界混杂，broken/rollback UX 和 retention 不足。 |
| 更新策略 | patch 自动；pin/channel 字段和命令 | 自动更新、兼容性、预发布 channel | 用户控制、社区更新 | 商店版本/审核发布 | Floter 缺发布 channel 语义、撤回/回滚策略、更新来源审计。 |
| 配置归属 | host/tool owner；secret generation；本地 export/import | global/workspace/extension storage、secret storage | data.json/插件自管 | Preferences/secure storage | Floter 已有清晰 owner 概念，但安装版本与配置没有统一事务；tool-owned schema 只能间接协商。 |
| 错误恢复 | journal recovery、repair、snapshot import rollback | extension host restart、disable incompatible extension | 手工删除/禁用 | 商店重装/诊断 | Floter 底层恢复较强，产品层没有统一 operation id、progress、可重试错误分类。 |
| 目录/缓存 | extensions、data、cache、tool-lock、official state 多文件 | 扩展目录 + global storage + logs | vault/.obsidian/plugins | 应用管理目录 | Floter 目录职责可解释但聚合根缺失，跨文件事务复杂度明显更高。 |
| 审核与发现 | NPM 搜索，官方来源 badge；Discover source 固定 NPM | Marketplace 分类/评分/兼容性 | 社区目录 | 官方商店、分类、审核 | Floter 缺分类、版本兼容展示、评分/审核、撤销和多源 UI。 |
| SDK/验证 | wrapper 文档和 fixture；无可证明的 V 动态参考实现，CI 三平台 E2E 不完整 | 官方 TypeScript API/运行时测试 | JS API/样例丰富 | JS/TS SDK 与商店校验 | Floter 协议文档多但契约测试、生成器和跨平台证据不足。 |

### 3.1 设计自洽性结论

- **自洽的部分**：manifest -> provider descriptor -> structured execution plan 这条链边界清楚；NPM 版本树、SRI、tar 安全解包、provider 超时和 config secret generation 也形成了可验证闭环。
- **职责重叠**：`ExtensionsLock` 同时保存安装事实、版本策略、信任摘要、运行时路径和 UI 状态；`tool-lock` 又保存 runtime 绑定；`current.json` 再投影 current version。三个对象共同描述一个 extension，却没有单一聚合根。
- **过度设计**：platform/artifacts/static adapter/inventory/resolver/source bundle/health/session/lifecycle 全部进入同一扩展域，但没有清晰的 capability 分层；对当前 NPM+CLI 主路径而言，source inference、多个 locator 类型和 `Target` 抽象尚未形成统一可用产品。
- **欠设计**：权限审计记录、撤销/升级策略、broken 状态写入、操作进度/取消、跨平台 E2E、SDK contract tests、配置与安装联合提交。
- **信任边界矛盾**：文档正确地说权限不是 sandbox，但 UI 把权限统一呈现为同一种 checkbox；`environment/process-spawn` 是执行控制，其他五类只是告知，用户很难区分其实际约束强度。
- **数据归属矛盾**：host-owned config、tool-owned config、local integration files、NPM extension data 都位于 `extension-data/<id>` 下；卸载的 `removeData` 同时可能删除配置、session、health 和用户生成脚本，缺少按数据类别的保留策略。
- **版本兼容矛盾**：manifest Floter/provider semver、NPM package version、tool version、release channel、pinned 和 previous version 都存在，但 lock 没有记录“本次选择由何种约束产生”，也没有扩展 API/宿主 capability negotiation。
- **错误恢复矛盾**：底层有 journal/snapshot，前端却把大多数错误压成一条 notice；`broken` 没落盘，repair 结果没有统一 error code/operation id，用户无法判断应重试、回滚、重新连接还是批准权限。

## 四、重新规划方案

### 4.1 目标架构

目标是把扩展系统收敛为四个明确边界：

```text
Package + Trust        # 解析、来源、签名、撤销、manifest digest
        |
Extension Repository   # 唯一安装聚合根和事务日志
        |
Runtime Binding        # bundled/system/script、tool locator、provider process
        |
Host Services          # command catalog、config store、health、UI/IPC
```

目标持久模型：一个 `extensions.state.json`（或等价 SQLite）作为唯一提交真源，包含安装 entry、current/previous 多版本、approved permission audit、source/trust、runtime binding、config generation 引用和 operation journal。`current.json`、shim、provider cache 都是可重建投影，不再参与业务真相判断。所有安装、更新、reinstall、rollback、uninstall、import 使用相同 transaction engine；事务提交采用 `prepare -> verify -> activate -> commit state -> rebuild projections -> cleanup`，启动只按 journal 完成/回滚。

权限拆成两层：

1. **Host-enforced capabilities**：environment、process-spawn、未来 filesystem/network/clipboard broker；每次 API 调用由 Host 检查；
2. **Native disclosure**：明确标注“无法沙箱”，并提供可选 OS sandbox backend（macOS seatbelt、Linux bubblewrap、Windows AppContainer/Job）作为长期能力。

配置拆成 `host settings`、`tool-owned settings`、`runtime data`、`generated artifacts` 四类，各自有保留/导入策略。Provider API 固定 protocol version、capabilities、cancel、diagnose/repair report 和错误码。UI 只消费结构化 `ExtensionOperation` 事件，不自行拼接卸载+安装流程。

### 4.2 短期修正（1-2 个 phase，目标是止血）

#### Phase 1：文档与真实状态对齐

- **改动**：更新 `docs/DEVELOPMENT_PLAN.md`、FEP 状态和扩展 README（本审计阶段不修改这些文件；后续 phase 才执行）；建立“实现/测试/文档”三列能力矩阵；明确 local import/export，不称 cloud sync。
- **涉及文件**：`docs/DEVELOPMENT_PLAN.md`、`docs/extensions/FEP-*.md`、`docs/extensions/COMPLETENESS_AUDIT.md`、本报告。
- **工作量**：S。
- **风险**：低；主要风险是遗漏历史声明。
- **验收标准**：每个 FEP 条目都有代码入口、测试命令和证据链接；不存在“全部完成”与“代码尚未开始”并存；产品文案不再承诺远端同步。

#### Phase 2：状态/错误/权限审计补齐

- **改动**：为 lock entry 增加 `approvedPermissions`、manifest/content digest、approvedAt、approvalSource、lastErrorCode、lastOperationId；所有 verify/describe/probe/install 错误按 `broken` 状态和结构化 error code 落盘；前端显示“声明权限”和“Host 实际强制范围”。
- **涉及文件**：`src-tauri/src/extensions/lock.rs`、`install.rs`、`commands/extensions.rs`、`src/ExtensionsPanel.tsx`、`src/extensions/ExtensionRow.tsx`（注：ExtensionRow.tsx 实际存在于 `src/extensions/` 而非内联）、相关 schema/测试。
- **工作量**：M。
- **风险**：中；lock schema migration 与旧安装兼容。
- **验收标准**：新增权限必须绑定同一 manifest digest 才可提交；重启后 broken 状态可见；repair 成功清除 broken 并记录 operation；UI 明确 native disclosure 不是 sandbox；旧 schema 自动迁移且不丢 pinned/channel。

### 4.3 中期重构（3-5 个 phase，统一事务和领域边界）

#### Phase 3：单一 ExtensionRepository 与投影重建

- **改动**：以 repository/state 文件或 SQLite 取代 lock + current pointer 多真源；current pointer、shims、tool-lock projection 可从 state 重建；统一 install/update/reinstall/rollback/uninstall 的 transaction engine 和故障注入。
- **涉及文件**：`extensions/lock.rs`、`transaction.rs`、`install.rs`、`artifacts.rs`、`tool_lock.rs`、`mod.rs`。
- **工作量**：L。
- **风险**：高；升级迁移、跨平台 rename/fsync 语义、旧残留目录。
- **验收标准**：kill/断电故障注入覆盖每个 commit 点；任意恢复后 state、runtime projection、shim 三者可重建且一致；reinstall 失败保持旧版本；uninstall 删除失败不会先丢失可恢复事实；至少保留可配置的 2 个 previous 版本。

#### Phase 4：Provider/Runtime API 收敛

- **改动**：把 provider protocol、runtime binding、catalog command descriptor 分成稳定接口；`extensions_reprobe` 改为执行 manifest lifecycle probes；`extensions_launch` 读取 manifest launch/cwd/restore/terminal；加入 cancel、operation progress、标准错误码和 capability negotiation。
- **涉及文件**：`provider.rs`、`lifecycle.rs`、`registry.rs`、`catalog.rs`、`commands/extensions.rs`、`session_restore.rs`、前端 hooks。
- **工作量**：L。
- **风险**：中高；协议兼容和现有静态适配器迁移。
- **验收标准**：同一 probe 集合用于 install、repair、reprobe；launch 行为由 manifest 驱动并有 v1 compatibility fallback；provider 超时/取消不会残留子进程；catalog 只能执行 repository 返回的已验证 command descriptor；旧 provider protocol 给出明确降级错误。

#### Phase 5：配置/数据归属与导入导出边界

- **改动**：建立 host settings、tool settings、runtime data、generated artifacts 四类目录/策略；安装事务提交 config migration generation；导入导出 schema 标明 secret、设备特定路径、版本约束；把快照恢复改成 transaction engine 的 prepare/commit，而非递归复制后补写 lock。
- **涉及文件**：`config.rs`、`sync.rs`、`install.rs`、`commands/extensions.rs`、`ExtensionsPanel.tsx`、schema/docs。
- **工作量**：L。
- **风险**：高；密码迁移、用户脚本和现有 local integration 数据兼容。
- **验收标准**：更新失败不会出现新 manifest + 旧 config 混合；导入崩溃恢复后不产生部分成功；导出明确“本地移植包”，secret 永不明文导出；卸载可分别选择程序、host config、tool data、generated artifacts。

### 4.4 长期演进（6-8 个 phase，建立生态能力）

#### Phase 6：可选 OS sandbox 与 Host capability broker

- **改动**：将 filesystem/network/clipboard 从声明升级为 broker API；为 Linux/macOS/Windows 提供可插拔 sandbox backend，native unrestricted mode 必须显式标注并默认警告；定义权限升级、撤销和审计事件。
- **涉及文件**：新增 `extensions/sandbox/*`、`provider.rs`、`commands/extensions.rs`、capability schema、UI。
- **工作量**：L。
- **风险**：极高；三平台系统权限、性能、兼容性和用户体验。
- **验收标准**：在 sandbox mode 下 provider 无法读取宿主 lock/其他 extension data；每个 broker API 有 deny/allow 测试；无 backend 时 UI 明确降级为 native disclosure；权限撤销立即阻止下一次 API 调用。

#### Phase 7：官方来源治理与商店发现

- **改动**：官方 index 增加版本范围、channel、撤销、tarball digest、审核时间和 publisher rotation；实现签名索引缓存/离线使用策略；Discover 增加 source/filter/category/compatibility/review 状态，不再固定 NPM 单源。
- **涉及文件**：`official_index.rs`、`install.rs`、`registry.rs`、`ExtensionsPanel.tsx`、`SearchCard.tsx`、official-index schema/scripts。
- **工作量**：M/L。
- **风险**：中；信任根轮换和旧包兼容。
- **验收标准**：撤销版本无法安装/更新；离线使用最近未过期索引且显示 stale；官方/社区/本地来源在 UI 中可区分；publisher key rotation、index rollback、digest mismatch 都有测试。

#### Phase 8：SDK、兼容性与跨平台验证

- **改动**：从 schema 生成 typed SDK/manifest validator；提供可执行 Go/Rust/TypeScript provider harness；把 V 动态 provider 参考实现纳入仓库；CI 运行 Linux/Windows/macOS 的 install/update/reinstall/repair/rollback/uninstall、权限、配置和 provider contract tests。
- **涉及文件**：`docs/extensions/sdk/*`、新增 SDK/harness、`.github/workflows/*`、fixtures、`extensions/v-tools/*`。
- **工作量**：L。
- **风险**：高；平台 runner、外部工具和发布凭据。
- **验收标准**：干净环境可按 fixture 完成安装到卸载；每个 provider operation 有 golden contract；三平台至少一条真实 bundled 和一条 system runtime 流程；文档完成声明只在 CI 证据存在时标记完成。

### 4.5 后续 codex phase 拆分总表

| Phase | 独立交付物 | 依赖 | 规模 |
|---|---|---|---|
| 1 | 文档/能力矩阵校准 | 无 | S |
| 2 | 权限审计、broken、错误码 | 1 | M |
| 3 | 单一 repository/事务与投影恢复 | 2 | L |
| 4 | Provider/runtime/lifecycle API 收敛 | 3 | L |
| 5 | 配置归属与导入事务 | 3、4 | L |
| 6 | OS sandbox/capability broker | 4、5 | L |
| 7 | 官方索引治理与多源发现 | 2、3 | M/L |
| 8 | SDK、V reference、三平台 E2E | 4、6、7 | L |

优先级建议：先执行 Phase 1-3，消除“声明完成但不可证明”和多真源事务风险；再执行 Phase 4-5，稳定协议与数据归属；只有在这两层稳定后，才投入 Phase 6 的 OS sandbox 和 Phase 7-8 的生态扩张。

## 五、能力矩阵（Phase 1 交付）

三列口径：**实现** = 代码入口（file:line）；**测试** = 覆盖该能力的测试命令与位置；**文档** = 声明该能力的规范及其状态。判定基线 2026-08-22 `main`。

| 能力 | 实现 | 测试 | 文档 |
|---|---|---|---|
| Manifest v2 解析/校验/平台覆写 | `manifest.rs:15-45,237-251,304-463` | `cargo test`（manifest 模块） | FEP-1 · 已实现 |
| Provider describe/diagnose/超时/缓存 | `provider.rs:245-405,408-490` | `cargo test`（provider 模块） | FEP-2 · 已实现 |
| 动态 complete 全链路 | `provider.rs` + `catalog.rs:189-232` + `App.tsx:863-938` | `cargo test`（complete 合并/超时/取消）；无前端测试 | FEP-2 · 已实现 |
| 结构化执行计划 → PTY | `provider.rs:526-587`、`ExecutionPlanCache`（`mod.rs:181-274`）、`commands/terminal.rs` | `cargo test`；手动验证 | FEP-2/5 · 已实现 |
| NPM 下载/SRI/安全解包 | `install.rs:1548-1934` | `cargo test`（下载/解包逃逸用例） | FEP-3/4 · 已实现 |
| Ed25519 tarball 验签 + 官方签名索引 | `install.rs:1756-1867`、`official_index.rs:13-63,108-176,245-314` | `cargo test`（篡改/过期/key rotation） | FEP-4 · 已实现（索引治理遗留 Phase 7） |
| 安装/更新/reinstall/rollback/repair/uninstall | `install.rs:1015-1459,1548-1934`、`commands/extensions.rs:1221-1261` | `cargo test`（274+ 用例含失败路径）；无故障注入覆盖每个提交点 | FEP-3 · 部分实现 |
| 事务 journal 与启动恢复 | `transaction.rs:24-65,253-373,469-547` | `cargo test`（枚举顺序/恢复推断）；缺 kill/断电故障注入 | FEP-3 · 半成品（Phase 3 目标） |
| 权限审批与执行层强制 | `lock.rs:121-134,360-375`、`install.rs:1610-1612`、`provider.rs:408-430,526-587,696-728` | `cargo test`（env 隔离/spawn 限制/digest 绑定/空权限集绑定：549d233, e174e51） | FEP-5 · 部分实现（审计记录已落盘；OS sandbox 待 Phase 6） |
| 声明式配置 + secret generation | `config.rs:125-191,820-892,906-960` | `cargo test`（两阶段失败/并发/启动修复） | FEP-6 · 已实现 |
| 本地导入/导出事务 | `sync.rs:65-218,313-428,734-810` | `cargo test`（preflight/回滚/幂等） | 计划 §5.3 · 已实现（明确为本地文件移植） |
| Catalog 搜索/命名空间/冲突 | `catalog.rs:115-187,329-662` | `cargo test`（排序/启停/冲突） | FEP-2 · 已实现 |
| 管理面板（Installed/Discover/Updates） | `ExtensionsPanel.tsx`、`hooks/useExtensionActions.ts` | 仅构建级验证；无浏览器交互测试 | 计划阶段 4 · 已实现 |
| V Tools 静态适配器 | `static_adapter.rs:10-229`、`src-tauri/extensions/v-tools/*` | `cargo test`（探测/argv 计划） | 计划阶段 5 · 已实现 |
| V 动态 Provider 参考实现 | — 无代码产物 | — | 计划阶段 5 第二步 · ❌ 未落地 |
| 三平台 E2E 验证 | `.github/workflows/release.yml`（仅构建矩阵） | — 无 | 计划阶段 6 · ❌ 未解决 |
| SDK 可构建模板 | — 仅文档指南 `docs/extensions/sdk/*.md` | — | 计划阶段 7 · ⚠️ 指南非模板 |

> 维护规则：本矩阵是「声明 vs 实现」的唯一对照表。后续 phase 改动能力状态时同步更新此表；
> DEVELOPMENT_PLAN.md 的阶段勾选仅作历史记录，不再作为完成度依据。

