# AGENT NOTES — floter 迭代上下文

给在本仓库工作的 AI agent（Hermes / Codex / pi 等）看的项目级备忘。项目通用操作流程（构建、测试、验证管线）见 Hermes 侧 skill `software-development/floter-iteration`，这里只记产品方向和约定。

## 插件体系方向（2026-08-23 用户确认）

1. **去 NPM 强依赖**。NPM 分发不稳定，不作为核心路径；围绕它建的信任栈（SRI/Ed25519/官方签名索引）优先级下降，未来分发方案待定。
2. **发现优先，用户介入越少越好**：PATH 扫描 + 约定位置 manifest + 本地连接都应能自动识别注入工具；开发者负担也要小，不强制一套规则适配所有工具。
3. **v-tools 平权**：内置 V Tools 不做特殊逻辑，只是"推荐工具"，和其他扩展走同一套代码路径。
4. **管理面板尽量轻**：安装/连接、开关、卸载；更新/回滚/修复等尽量自动化，不做商店式 Discover。
5. **权限=诚实披露**：声明权限 + 明确告知非沙箱，不假装能强制执行。

## 工作方式（2026-08-24 更新）

- 2026-08-24 起：Hermes 负责规格、派发、审查、验证、提交；代码修改由 pi 执行（优先 openrouter 的 xAI 模型，无余额时用 tar-sub2api + deepseek-v4-flash）。
- 历史调整可作参考但不必延续其逻辑——`docs/plugin-system-audit.md` 的 Phase 划分是旧方向下的产物，与上面第 1-3 条冲突时以本文件为准。
- 每次改动后：`git pull --rebase origin main` → 验证管线全绿 → commit → push。
