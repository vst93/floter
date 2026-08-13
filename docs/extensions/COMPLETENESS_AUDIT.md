# Floter 扩展平台基础功能完整度核查

核查日期：2026-08-13  
核查分支：`dev/extensions-platform`（`ed1943a`）

## 1. 核查范围与方法

本次核查以 `docs/DEVELOPMENT_PLAN.md`、FEP-1～FEP-6、中文总览与 Agent 打包指南、SDK 发布检查清单和两份 JSON Schema 为规格真源，逐项追踪到 Rust 扩展模块、Tauri command 注册、React 调用与最终 PTY 执行层，并检查现有测试是否覆盖关键成功及失败路径。

判定口径如下：

| 判定 | 口径 |
| --- | --- |
| ✅ 真实现 | 核心逻辑已经接线到真实调用路径，并有针对性测试覆盖。 |
| ⚠️ 半成品 | 有 API、界面或局部逻辑，但关键事务、分支、可信边界、产品接线或测试缺失。 |
| ❌ 未实现 | 计划/FEP 明确声称存在，但当前仓库没有对应实现或可运行产物。 |

验证结果：`cargo test` 通过（110 passed、0 failed、1 ignored）；`npm run build` 通过。仓库没有 React 单元测试、Playwright/E2E 测试；release workflow 只做六目标构建，不运行扩展协议、安装或 UI 流程测试。因此“构建通过”不能替代计划声称的三平台端到端验证。

计划文档本身也存在冲突：开头和阶段 1～7 均标称完成（`docs/DEVELOPMENT_PLAN.md:9`、`:1027-1043`），但附录仍把后端、联想、管理页、V 动态协议和三平台测试列为 pending，并写明“代码实现尚未开始”（`:1089-1103`）。本报告不以该附录直接下结论，仍以当前代码行为为准。

## 2. 判定结果总表

| 功能项 | 声称来源 | 实际代码位置 | 判定 | 证据 |
| --- | --- | --- | --- | --- |
| Manifest v1/v2、Schema 校验与平台覆写 | FEP-1；计划阶段 2 | `manifest.rs:253-317,320-452`；两份 schema | ✅ | 使用 Draft 2020-12 validator，兼容 v1 迁移，校验组合、兼容版本、安全相对路径、平台 allow-list 和 exact/OS override；有解析、拒绝路径和覆写测试。 |
| Provider `describe`、缓存、超时与降级 | FEP-2；计划阶段 2 | `provider.rs:237-333,400-513,641-752` | ✅ | 缓存键包含路径、mtime、包版本和工具版本；限制输出、强制超时/kill，失败可回退缓存；Provider ID 和执行描述符均校验。 |
| 结构化执行计划与 PTY/外部终端接线 | FEP-2/5；计划阶段 2～3 | `provider.rs:516-598`；`mod.rs:121-177`；`commands/terminal.rs:34-100`；`App.tsx:1304-1333,2321-2346` | ✅ | program/argv/env/cwd 结构化传递，敏感计划以一次性 token 留在后端，真实 PTY spawn 使用还原后的计划；`capture` 归一化为 `pty`。 |
| 权限安装审批与升级审批 | FEP-5:21-26；计划阶段 7 | `install.rs:862-982,1248-1320,1472-1476,1509-1512`；`ExtensionsPanel.tsx:691-733,815-867` | ✅ | 首装要求精确权限集合；更新新增权限时后端再次要求批准；有拒绝未审批和权限升级测试。 |
| `environment` 执行层强制 | FEP-5:11-17,69-75 | `provider.rs:407-420,616-628,566-575`；`commands/terminal.rs:50-85`；`terminal/broker.rs` | ✅ | Provider/version 调用缺权限即 `env_clear()`；命令计划的 `inherit_environment` 最终进入 broker；测试验证宿主环境变量不可见。 |
| `process-spawn` 执行层强制 | FEP-5:11-17,69-75 | `provider.rs:516-553,682-714` | ✅ | 非 `self` program 在 describe 加载和执行计划生成两处都要求权限，只允许 bundled runtime 内普通相对路径并验证文件存在；有正反测试。 |
| 其余权限的披露边界 | FEP-5:11-19 | `manifest.rs:149-159`；`install.rs:924-982` | ✅ | filesystem/network/clipboard 明确只作披露，不宣称 OS 沙箱；manifest、审批 UI 和文档口径一致。 |
| NPM 解析、下载与 SRI 校验 | FEP-3/4；计划阶段 2/7 | `install.rs:1241-1360,1644-1754` | ✅ | 直接 Registry HTTP 下载，不执行 npm；支持版本/tag/range和 SHA-512/384/256 SRI；限制响应大小并要求 HTTPS。 |
| Ed25519 tarball 签名验证 | FEP-4:103-130；计划阶段 7 | `install.rs:1281-1283,1756-1867,2768-2824`；Cargo `ed25519-dalek` | ✅ | 对原始基础包 tarball 字节验签；HTTPS 下载、长度/Base64/公钥格式和错误签名均拒绝，并有成功、篡改、畸形材料测试。 |
| 官方签名索引与可信来源 | FEP-3 安全基线；计划 6.3、阶段 7“签名索引” | `install.rs:1195-1238`；`ExtensionsPanel.tsx` | ❌ | NPM 搜索结果无条件 `verified: false`；没有官方索引下载、索引签名验证、信任根/包名白名单或官方/社区源切换。manifest 自带公钥验签不能建立“官方可信”身份。 |
| tar 安全解包 | FEP-1/3/5；SDK checklist | `install.rs:1879-1967,2826-2843` | ✅ | 强制 `package/` 根，拒绝绝对路径、`..`、符号链接、硬链接和非常规条目，限制条目数与展开体积，使用 `create_new`；有逃逸与正常解包测试。 |
| executable/descriptor/script 安全相对路径 | FEP-1/2/3 | `manifest.rs:352-399,440-452`；`install.rs:2008-2024`；`provider.rs:531-553,694-710` | ✅ | manifest 路径只允许 Normal component；bundled 主入口必须存在；非 self program 被限制在 runtime root。 |
| NPM 安装/更新 8 步流程 | FEP-3；计划阶段 2 | `install.rs:1241-1470`；`lock.rs:137-161,197-236` | ⚠️ | 下载、SRI/签名、安全解包、manifest/兼容性、describe、同文件系统 rename、pointer/lock 写入均存在；但 phase struct 只是顺序断言，`current.json` 与 lock 是两个独立提交，崩溃窗口可产生不一致，回滚是 best-effort，且没有 crash recovery/journal。未执行可选 diagnose。 |
| 回滚与保留 previous version | FEP-3 | `install.rs:1088-1192` | ⚠️ | 可在 current/previous 间切换并重新 describe，pointer 写失败不改 lock、lock 写失败尝试还原 pointer；但仍是双文件 best-effort 事务，且只保留一个 previous 指针，没有清理/保留策略与崩溃恢复测试。 |
| 重装 | 计划阶段 4；管理页动作 | `ExtensionsPanel.tsx:746-770` | ⚠️ | 仅由前端串行调用 uninstall 再 install；第二步失败会删除原 lock/程序版本，违反 FEP-3“失败保留当前可用版本”。后端没有原子 reinstall command，也无该失败路径测试。 |
| 修复 | FEP-3“修复”；计划阶段 4 | `ExtensionsPanel.tsx:1305,1762-1775` | ❌ | 没有 repair Tauri command 或重新校验/补下载/describe 流程；所谓 repair/installTool 只打开 homepage，system runtime 另有 reconnect，但不等于托管扩展修复。 |
| 卸载与 system runtime 保护 | FEP-1/3；计划阶段 4 | `install.rs:1013-1086` | ✅ | 托管目录先 rename staging，lock 保存失败可恢复；system runtime 不删除外部程序；取消 completion，可选删除 data；有 system 文件保留测试。 |
| 固定版本、stable/beta channel、默认仅 patch 自动更新 | FEP-3；计划 3.3 | `lock.rs:99-104`；`install.rs:984-1011`；`ExtensionsPanel.tsx:497-518,1051-1075` | ⚠️ | lock 有 `pinned/channel` 字段且 update 会尊重 pinned，但没有设置 pin/channel 的 command/UI；检查更新和“全部更新”直接采用 registry 最新版本，没有 patch-only 策略或 major 确认。 |
| NPM deprecation 提示 | FEP-4:132-147 | `install.rs` Registry DTO/search；`ExtensionsPanel.tsx` | ❌ | 未解析 `floter.deprecated` 或 registry deprecation，也无安装警告。 |
| 静态 V Tools 适配器 | 计划阶段 5 第一步 | `src-tauri/extensions/v-tools/*`；`static_adapter.rs:10-229`；`catalog.rs:486-528` | ✅ | 内置 manifest/descriptor 声明 jv/diff/codec/genpwd/tt；仅用户连接并启用后入目录，PATH 可执行探测和 argv 计划有测试。 |
| V Tools 动态 Provider 参考实现 | 计划阶段 5 第二步、阶段 6 | 当前仓库无 V 源码或可执行 Provider 产物 | ❌ | 仓库只有静态适配器和文档示例；无法证明 `v --floter describe/complete/diagnose`、Plugin.List 动态发现或“V 新插件刷新即出现”已经落地。 |
| Catalog 搜索、冲突命名空间和执行 | FEP-2；计划阶段 2～3 | `catalog.rs:113-181,329-662,718-752`；`commands/extensions.rs:897-912`；`App.tsx:874-936,2390-2412` | ✅ | 合并应用、系统命令、本地命令和启用 Provider；支持命名空间、排序、后端保护执行计划并真实接入前端。相关排序/启停/执行计划有 Rust 测试。 |
| 静态与路径补全 | FEP-2；计划阶段 3 | `catalog.rs:183-327,664-715`；`App.tsx:874-936` | ✅ | 参数 kind 驱动静态、enum、文件/目录补全，合并去重并接入建议 UI；有静态与合并测试。 |
| 动态 `complete` 全链路 | FEP-2；计划阶段 7 | `provider.rs:335-380`；`catalog.rs:183-327`；`App.tsx:874-936` | ✅ | 前端 debounce/过期 generation、后端 100ms debounce、2s cwd+请求缓存、超时、取消（drop + kill_on_drop）及静态降级均存在；协议 argv/stdin、超时、失败、合并有测试。 |
| React 联想键盘和结构化执行体验 | 计划阶段 3 | `App.tsx:283-368,874-936,2390-2490` | ⚠️ | Tab/Enter/方向键和 external 执行已接线且能构建，但仓库没有任何 React/浏览器交互测试，不能满足“完整逻辑 + 覆盖测试”的严格 ✅ 口径。 |
| 扩展管理页基础功能 | 计划阶段 4 | `ExtensionsPanel.tsx`；`App.tsx:2740-2773`；`i18n.ts` | ⚠️ | 已安装/发现/更新、详情、配置、诊断、权限、启停、更新、回滚、卸载和双语均有真实调用；但可信源、repair、pin/channel 缺失，且无前端测试。 |
| 声明式配置模型、校验、UI 与注入 | FEP-6；计划阶段 2/4/7 | `config.rs:116-603`；`catalog.rs:426-468,507-508`；`ExtensionsPanel.tsx:1085-1145,1620-1700` | ✅ | host/tool owner、字段类型与约束、schema 迁移、默认值、env/argv 注入、密码 IPC/导出脱敏和工具配置 PTY 均接线；有校验、迁移、注入、脱敏测试。 |
| 配置持久化整体事务 | FEP-6:98-128；重点核查项 | `config.rs` | ✅ | secrets 先作为受限权限的不可变 generation 写入并 fsync，`config.json` 以 `secretGeneration` 作为唯一原子提交指针；配置写复用 mutation lock 串行化，启动修复悬空/损坏 generation。覆盖公开提交失败、chmod 失败、启动修复和并发保存测试。 |
| 跨设备同步数据过滤与文件导出 | 计划 5.3、阶段 7 | `sync.rs:85-203`；`config.rs:206-212`；`commands/extensions.rs:142-320` | ✅ | 导出安装项、启用状态、可移植 manifest/静态 descriptor/script 和非敏感配置；密码被脱敏，不导出本机 executable path；导出文件原子写，有格式/迁移/幂等测试。 |
| 文件导入整体事务与真正传输 | 计划阶段 7“跨设备同步” | `sync.rs`；`ExtensionsPanel.tsx` | ✅ | 本地 JSON 导入先生成完整 plan，在隔离 staging 中完成所有下载、SRI/签名、解包、Provider 与配置校验，再持 mutation lock 提交；失败通过 lock、版本目录、配置和本地文件快照整体回滚。Cloud/Git/WebDAV 未实现，管理页和计划文档已明确功能仅为本地导入/导出。 |
| SDK / Wrapper 模板 | 计划阶段 7；`sdk/README.md`、checklist | `docs/extensions/sdk/*.md` | ⚠️ | 内容是较完整的 TypeScript/Go/Rust 指南和代码片段，但没有可构建目录、`package.json`/`Cargo.toml`/`go.mod`、schema 校验脚本、测试或 CI；“模板”不能直接复制后构建/pack。 |
| Schema 与运行时一致性 | FEP-1/2/6；SDK checklist | `docs/extensions/schemas/*`；`manifest.rs`；`provider.rs`；`config.rs` | ⚠️ | 两份 schema 会被 Rust 运行时使用，但部分关键条件只在 Rust 二次校验（例如 config owner/schema/openCommand、参数 kind 与 values/takesValue 关系）；单独“通过 schema”不足以保证 FEP 合法。没有独立 schema fixture 测试矩阵。 |
| 旧 `commands.json` 迁移 | 计划阶段 2 | `catalog.rs:820-875`；`mod.rs:93-97` | ✅ | 启动时一次性迁移到结构化 `local-commands.json`，拒绝明显 shell 元字符并原子持久化。 |
| Linux/Windows/macOS 端到端验证 | 计划阶段 6；SDK checklist | `.github/workflows/release.yml:116-195` | ❌ | CI 矩阵只构建六目标，未运行 `cargo test`、Provider 协议、NPM 安装/更新/卸载或 UI E2E；当前本机仅完成 Linux Rust 测试与前端构建，无法支持“三平台测试已完成”的声明。 |

## 3. 高优先级未实现/半成品项

### P0：重装会先删除当前可用版本

**现状：** 管理页的重装由前端执行 `extensions_uninstall` 后再 `extensions_install`。下载、网络、签名、describe 或 lock 写入任一失败时，旧版本已经被删除。

**缺失点：** 与 FEP-3“任何一步失败都不得改变当前可用版本”直接冲突；没有后端事务命令和失败测试。

**建议实现：** 新增后端 `extensions_reinstall`，复用安装 staging，但允许同版本进入临时版本目录；完成所有校验后一次切换版本身份。旧目录在 lock/pointer 提交成功后再回收；增加下载失败、describe 失败、pointer/lock 失败和进程崩溃恢复测试。

### P0：安装/更新/回滚缺少崩溃一致事务

**现状：** 版本目录 rename、`current.json` 和 `extensions.lock.json` 都单独原子，但整体不是一个事务；代码只在收到 I/O error 时 best-effort 还原。

**缺失点：** 在 pointer 已替换、lock 未替换或回滚过程中进程/机器崩溃，会留下双真源不一致；没有启动恢复、事务日志、目录 fsync 或故障注入覆盖。`InstallationTransaction` 只验证枚举顺序，不承担资源回滚。

**建议实现：** 明确唯一提交真源（优先 lock）并让 pointer 可重建，或引入带 transaction id 的 journal；对 staged version、文件与父目录执行必要 fsync，启动时扫描未完成事务并恢复/完成提交。将 install/update/rollback/reinstall 统一到同一事务引擎。

### P1：官方签名索引/可信来源未实现

**现状：** 可验证 manifest 自声明公钥的 Ed25519 签名，但任何发布者都能同时提供包、签名和公钥；搜索结果始终是未验证社区包。

**缺失点：** 没有官方索引、索引签名、公钥 pin/轮换、包名+publisher+extension id 白名单，也没有官方/社区 UI。当前签名只证明“包与它自己声明的键一致”，不能证明 Floter 官方认可。

**建议实现：** 定义并内置最小根公钥，拉取带版本/过期时间的签名索引；索引绑定 extension id、NPM package、publisher 和允许的 signing key。安装时先验证索引再验证 tarball，UI 将官方验证和发布者自签分开展示，并测试篡改、过期、key rotation 和降级攻击。

### P1（已修复）：同步导入不是事务，且没有同步传输层

**现状：** 手工文件导出/导入可用，但逐扩展顺序执行并返回 succeeded/failed/skipped；单个扩展内也是安装、配置、启用分步提交。

**缺失点：** 配置或启用恢复失败时，已有扩展可能已更新；本地同步内容的 manifest/descriptor/script 可部分写入。功能名称“跨设备同步”目前实际只是可移植备份文件。

**建议实现：** 先生成完整 reconcile plan 并完成下载/验证，再以每扩展事务提交；保存旧 lock、版本与配置快照，后续步骤失败时恢复。若产品确实承诺跨设备自动同步，再实现明确 transport/provider 接口、冲突策略、加密和远端版本控制；否则把产品文案改为“导入/导出”。

**落实：** 导入现在先在隔离 staging 中完成全量预检，随后在同一 mutation lock 下提交；任何项失败均恢复本次导入前的完整快照，不返回部分成功。产品文案已改为本地文件导入/导出，远端 transport 仍明确留待后续工程。

### P1（已修复）：配置的公开值与 secrets 不是同一事务

**现状：** 两个 JSON 文件分别原子写，但先提交公开文件，再提交 secrets；无配置级 mutex。

**缺失点：** 第二次写、权限设置或并发保存失败时可能产生跨版本组合；没有 generation/checksum 或恢复逻辑。

**建议实现：** 将一个 generation 的公开值与 secrets 写入同一临时目录/版本文件，再原子切换单一 pointer；或使用系统 secret store 并让公开文件引用已提交 secret generation。串行化每扩展写入，增加两阶段故障注入与并发测试。

**落实：** `config.json` 现在引用已完整写入、chmod 并 fsync 的不可变 secret generation，自身是唯一原子提交指针；配置写通过 mutation lock 串行化，启动时安全修复缺失、损坏或 generation 不匹配的 secret 引用，并有两阶段失败与并发覆盖。

### P1：修复流程不存在

**现状：** “安装工具/修复”动作只打开 homepage；仅 system runtime 有重新探测 PATH 的 reconnect。

**缺失点：** 没有 FEP-3 所述重新校验 integrity、补下载缺失文件、重新 describe 和恢复 broken 状态。

**建议实现：** 后端新增 repair command：先验证 lock/manifest/版本文件和 Provider；bundled 缺失或损坏时按锁定 integrity 重新下载到 staging 并事务替换；system runtime 只重新定位并 describe。输出结构化 repair report，并覆盖离线、损坏包和 Provider 失败。

### P1：三平台端到端完成声明无证据

**现状：** release CI 有六目标构建矩阵，但不运行测试；前端无自动化测试。

**缺失点：** Windows `.cmd/.bat` host、macOS/Linux executable 权限、NPM 安装事务、动态 complete、PTY/external、管理 UI 均没有跨平台可审计结果。

**建议实现：** 增加普通 PR CI：六目标至少编译并运行可用单测，三 OS 原生 runner 运行 mock Provider 协议与本地/NPM fixture 生命周期；Playwright 驱动联想、权限、安装、配置、回滚/重装失败流程。发布检查清单输出实际平台矩阵 artifact。

## 4. 中低优先级项

### 中优先级

- **SDK 只有指南，没有工程模板。** 提供可构建的 Go/Rust wrapper 和打包 workspace，带 fixture、schema validator、`npm pack --dry-run` 与 CI；否则应改称“实现指南”。
- **V 动态参考实现不可审计。** 将 V Provider 的固定版本、源码链接和协议 fixture 纳入仓库/CI，或撤销“已完成”声明；当前只能确认静态 V Adapter。
- **版本治理 UI 未完成。** 增加 pin/unpin、stable/beta/dist-tag 选择和 major 更新确认；自动/批量更新默认限制 patch。
- **NPM deprecation 未处理。** 解析 deprecation 元数据，发现页和安装确认中提示，不静默删除已安装版本。
- **Schema 约束不完整。** 用 `if/then` 表达 configuration owner 条件及 argument kind 条件，并增加 valid/invalid fixture 矩阵，保证 SDK checklist 的“Schema 通过”有足够含义。
- **权限审计记录不足。** 当前批准结果主要由已安装 manifest 间接表达，lock 没有 approved permission set、批准时间/来源。建议追加不可变审计字段，便于解释更新权限变化。

### 低优先级

- **管理页与联想缺少前端测试。** 逻辑已经接线并可构建，但键盘、焦点、确认框、批量更新中断和配置控件回归只能人工发现。
- **可选 diagnose 未进入安装 preflight。** FEP 将其定义为可选步骤；可提供安装选项或官方包策略，在提交前运行并把 warning 展示给用户。
- **文档状态需要校准。** `DEVELOPMENT_PLAN.md` 的“全部完成”和附录 pending 同时存在；应按本报告拆分为已实现、部分实现、未实现，避免继续把协议能力、产品能力和验证证据混为一谈。

## 5. 结论

扩展平台不是“只有文档”：Manifest/Provider、结构化执行、两项 Host 强制权限、安全解包、SRI/Ed25519、静态 V Adapter、Catalog、动态 complete、声明式配置和文件导入导出都有实质代码与 Rust 测试。高风险问题主要集中在**跨文件/跨步骤事务边界和产品声明过度**：重装会破坏旧版本，安装状态无法保证崩溃一致，配置与同步只有局部原子性，官方可信索引、repair、V 动态参考实现和三平台 E2E 并未落地，SDK 也仍是文档指南而非可运行模板。因此“Phase 1-7 全部完成”的总体声明不成立。
