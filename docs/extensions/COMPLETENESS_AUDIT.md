# 复核（2026-08-22）

> 本节是对 2026-08-13 初核结论的增量复核，基线为 `main` 分支最新代码。
> 初核正文保留在下文，其中部分 P0/P1 已在后续迭代中修复；本节逐项更新判定，
> 并补充初核之后新增能力的证据。架构层面的系统性问题与重构路线图见
> `docs/plugin-system-audit.md`（2026-08-22），该报告是当前权威的审计文档。

## 1. 初核遗留项的当前状态

| 初核判定 | 项目 | 当前状态 | 证据 |
| --- | --- | --- | --- |
| ❌ | 官方签名索引与可信来源 | ✅ **已实现** | pinned development root、签名索引版本状态与 anti-rollback：`official_index.rs:13-63,108-176,245-314`；安装时校验官方身份：`install.rs:1558-1613`。遗留：索引每次在线拉取，离线时 `official_verified` 变 false 且不使用内置 payload；索引不声明版本范围/撤销/渠道。 |
| ⚠️ | NPM 安装/更新事务 | 🟡 **改善但未闭环** | 引入 transaction journal + fsync + 启动恢复（`transaction.rs`）；但 lock、`current.json`、journal 仍是三个独立持久写入，"同一事务"为 best-effort，无故障注入测试覆盖每个提交点。 |
| ⚠️ | 回滚与 previous version | 🟡 未变 | rollback 需 previous 目录存在并通过完整性校验（`install.rs:1319-1459`）；仍只保留一个 previous 版本，无 retention 策略。 |
| ⚠️ P0 | 重装先删除可用版本 | ✅ **已修复** | 后端 `extensions_reinstall` 直接复用锁定版本的 staging/install，不再调用 uninstall（`install.rs:1062-1091`）。遗留：多步提交仍非单一事务，缺 API 级故障注入测试。 |
| ⚠️ | pin/channel UI 与 patch-only 更新 | ✅ **已实现** | pin/channel 有命令与 UI（`ExtensionsPanel.tsx:920-930` 一带）；更新仅接受 patch 自动化、pinned 拒绝自动更新、minor/major 需显式版本（`install.rs:1015-1059`）。 |
| ❌ | NPM deprecation 提示 | ✅ **已实现** | Registry DTO 解析 deprecation（`install.rs:64,92,163,186,194,223,878-895`）；发现页 SearchCard 与安装确认对话框均展示弃用原因并需二次确认（`ExtensionsPanel.tsx:857-863,1666,1725`）。 |
| ❌ | 三平台 E2E 验证 | ❌ **未解决** | CI 仍是构建矩阵而非测试矩阵；三平台生命周期/协议/UI 自动化验证缺失。这是「完成声明」目前最大的证据缺口。 |
| ⚠️ | SDK 工程模板 | ❌ 未变 | 仍为文档指南（`docs/extensions/sdk/*.md`），无可构建目录、fixture 或 CI。 |
| ❌ | V 动态参考实现 | ❌ 未变 | 仓库只有静态适配器；`v --floter describe/complete/diagnose` 无可审计产物。 |

## 2. 新增能力（初核之后落地）

| 能力 | 状态 | 证据 |
| --- | --- | --- |
| repair 流程 | ✅ | `extensions_repair` 先做落盘版本树 integrity + manifest 身份 + provider 检查，NPM 按锁定 SRI 重装、system runtime 重连（`commands/extensions.rs:1221-1261`、`install.rs:1136-1213`）。 |
| 配置 secret generation 原子性 | ✅ | 受限写入 + fsync 的不可变 generation，`config.json` 为唯一提交指针，启动修复悬空 generation（`config.rs:820-892,906-960`）。 |
| 导入整体事务 | ✅（本地） | preflight + 快照 + 全量回滚（`sync.rs:313-428`）；快照恢复仍非 crash-consistent 单事务。 |
| env_clear / process-spawn 强制 | ✅（声明范围内） | `provider.rs:408-430,526-587,696-728`；filesystem/network/clipboard 仍为披露非沙箱。 |
| 终端命令行解析修复 | ✅ | launcher 结构化路径支持带参命令、环境变量前缀、shell 运算符分流（commit `652775e`）。 |
| 搜索命令默认隐藏 | ✅ | `show_commands_in_search` 设置默认关闭，集成页 opt-in（commit `dcf5511`）。 |

## 3. 当前真实缺口（按优先级）

1. **P1 · 无聚合根**：lock / tool-lock / current.json 三处共同描述一个扩展，职责重叠导致状态字段重复和漂移——重构路线图 Phase 3 的目标。
2. **P1 · `broken` 状态已落盘**（✅ Phase 2 已完成）：mark_broken/clear_broken 方法（`lock.rs:277-334`）在 repair 流程中调用（`commands/extensions.rs:1315,1463,1509,1563`、`catalog.rs:444`），有幂等性测试（e174e51）。
3. **P1 · 权限审计记录已落盘**（✅ Phase 2 已完成）：lock 记录 approved_permissions/approved_at/approved_manifest_digest（`lock.rs:121-134`），安装时写入（`install.rs:1610-1612`），有 digest 绑定验证（`lock.rs:360-375`）与测试（549d233, e174e51）。
4. **P1 · reprobe/launch 忽略 manifest**：`extensions_reprobe` 硬编码 `--version/--help`（`commands/extensions.rs:1405-1474`）；`extensions_launch` 忽略 lifecycle launch/cwd/restore 声明（`commands/extensions.rs:1478-1559`）。
5. **P2 · 文档漂移**：FEP-1~6 全部停留 Draft 状态未随实现推进；本文初核正文的旧判定以第 1 节表格为准。

## 4. 测试基线（2026-08-22，Linux）

- `cargo test`：276 tests, 275 passed, 1 ignored
- `tsc --noEmit`：通过
- `vite build`：通过
- 前端 node:test 单测：30 passed
- 三平台 E2E：无（见第 3 节第 6 行）

---

# Floter 扩展平台基础功能完整度核查

核查日期：2026-08-13  
核查分支：`dev/extensions-platform`（`ed1943a`）

> ⚠️ 下文为初核快照，其中 P0/P1 的修复状态以本文开头《复核》章节为准。
