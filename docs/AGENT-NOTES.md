# AGENT NOTES — floter 迭代上下文

给在本仓库工作的 AI agent（Hermes / Codex / pi 等）看的项目级备忘。项目通用操作流程（构建、测试、验证管线）见 Hermes 侧 skill `software-development/floter-iteration`，这里只记产品方向和约定。

## 插件体系方向（2026-08-23 用户确认）

1. **去 NPM 强依赖**。NPM 分发不稳定，不作为核心路径；围绕它建的信任栈（SRI/Ed25519/官方签名索引）优先级下降，未来分发方案待定。（2026-09-15 补记：NPM 分发及其 SRI/Ed25519 校验栈已于 `350e2d6`（后端）/`96870c4`（前端）物理移除；仅官方签名索引 `official_index.rs` 以半冻结状态保留，唯一消费方是 `extensions_refresh_official_status`。）
2. **发现优先，用户介入越少越好**：PATH 扫描 + 约定位置 manifest + 本地连接都应能自动识别注入工具；开发者负担也要小，不强制一套规则适配所有工具。
3. **v-tools 平权**：内置 V Tools 不做特殊逻辑，只是"推荐工具"，和其他扩展走同一套代码路径。
4. **管理面板尽量轻**：安装/连接、开关、卸载；更新/回滚/修复等尽量自动化，不做商店式 Discover。
5. **权限=诚实披露**：声明权限 + 明确告知非沙箱，不假装能强制执行。

## 工作方式（2026-08-24 更新）

- 2026-08-24 起：Hermes 负责规格、派发、审查、验证、提交；代码修改由 pi 执行（优先 openrouter 的 xAI 模型，无余额时用 tar-sub2api + deepseek-v4-flash）。
- 历史调整可作参考但不必延续其逻辑——`docs/plugin-system-audit.md` 的 Phase 划分是旧方向下的产物，与上面第 1-3 条冲突时以本文件为准。
- 2026-09-15：`docs/plugin-system-audit.md` 已在顶部标注 Phase 3-8 为历史参考，并新增「能力矩阵校准 / 冻结区/待删区 / 已知缺口索引（G1-G7）」三节；方向仍以本文件与 `docs/tool-binding-design.md` 为准。
- 每次改动后：`git pull --rebase origin main` → 验证管线全绿 → commit → push。

## 反馈通道（全应用一套 Toast，2026-09-17 / R7-5 起约束）

**唯一反馈面**是宿主 toast（`src/components/ToastStack.tsx` + `src/toast-state.ts`）。任何表面——包括沙箱 iframe 里的插件页——都只发消息、不画自己的提示条。时长、位置、视觉只有一份来源：`TOAST_DISMISS_MS`（error 8s / success 4s）与 `#floter-app-toasts`。

1. **插件页不得自绘任何 notice/toast/banner**。页内提示条（如已删除的 `.clipboard-panel__notice`）与自有 dismiss 定时器都不再有位置；失败一律走 bridge 的 `host-notify` 消息（`src/plugin-pages.ts`）。
2. **线上只传字典 key，不传文案**。`host-notify` 的 `messageKey` 由宿主用 `src/i18n.ts` 自己解析，未知 key 直接丢弃（`isMessageKey`）。宿主侧调用 `notify(kind, text, action?)` 时已是译文；插件页侧永远不持有宿主文案，也不增加宿主字典的耦合面。
3. **重试语义单向**：页说「这个失败了，可以重试」→ 宿主 toast 出一个动作按钮 → 用户按下 → 宿主回发 `notify-retry` → 页自己重跑。宿主绝不替页重放命令（它不知道页失败的是哪一次动作、也不持有页的状态）。**重试带关联 id**（2026-09-17 微修）：`host-notify` 由页生成 `id`，宿主的 `notify-retry` 原样回传；页内用 `createRetryRegistry()`（容量 = `MAX_TOASTS`）按 id 找 thunk，过期/未注册/已消费的 id 静默丢弃。此前单槽 `pendingRetry` 会让旧 toast 的 Retry 执行最新动作。
4. **位置按表面归属，不按表面另起一套**。`#floter-app-toasts[data-surface=…]` 只做「在这个窗口里落哪」的修正，不做第二套栈：全高窗口（settings / terminal / plugin）共用默认 `top:64px; right:16px`（插件页 28px 顶栏之下仍余 36px）；只有 ~58px 的 collapsed 需要专门规则。**新增表面默认复用默认值，除非实测遮挡**。

## 三态规范（空 / 加载 / 错误，2026-09-17 / R7-5 起）

适用对象：任何会拉取数据的表面（插件页、面板、抽屉）。三态都必须能互相切换，且不闪布局。

1. **空态** = 标题 + 说明 + （可选）主操作。标题说「没有东西」，说明说「为什么 / 怎么办」（如 `clipboard.emptyHint`），有可执行的下一步才放按钮；不要用空态承载错误（错误有自己的态）。形状用 `.clipboard-panel__empty` + `__empty-title` + `__empty-hint`（宿主侧沿用 `.extensions-empty` 家族）。
2. **加载态** = **行内 spinner，不是整块替换内容**。只要有旧内容在场就不换掉它（后台轮询、刷新、重试一律如此），只有“首屏还没东西可显示”才画 spinner 行；行本身要有固定高度（避免到达时跳）。判据落成代码：一个 `loaded` 标志 + 「已是 spinner 就不重建节点」（重建会重启动画）。
3. **错误态** = 标题 + 为什么 + **两个控件槽：重试 + 关闭**。重试只在「重试真的可能成功」时出现（后端被关掉、命令不存在 → 只给说明和关闭，不给死路按钮）；关闭永远在，且语义是「不再画这个态」，不是假装加载成功了。同一错误重复渲染要复用已画节点（`data-failure-state`），否则重绘会丢焦点并重放进入动画。

参考实现：`src/plugins/clipboard/main.ts` 的 `renderEmpty` / `renderLoading` / `renderLoadFailure`，样式在 `src/plugins/clipboard/page.css`（`.clipboard-panel__empty(-title|-hint|-actions)` / `__loading` / `__spinner`）。后续新增插件页照此沿用，不要另发明形状。

### 微修补记（2026-09-17，R7-5 复核后）

- **错误态节点复用 ⇒ 不要手动 disable 控件**。`renderLoadFailure` 的节点靠 `data-failure-state` 跨重绘复用，所以 `retry.disabled = true` 会活过点击：重试再失败时节点不重建，按钮永久禁用。重入由 `reload()` 的 `reloadPending` 去重兜住，数据态决定按钮外观。新增任何「复用已画节点」的态都适用此条。
- **自动轮询的失败要按 key 去重**。`createFailureDeduper()` 默认 30s 窗口；只给 `clipboard.loadFailed`（2s 轮询那条）用，用户手势失败（copy/delete/clear）每次都报，成功加载后 `clear()` 重新武装。
- **M4 不改（有意语义）**：错误态 dismiss 后若列表为空会画空态（「Nothing copied yet」）。这是「不再画这个错误态」的可辩语义，不是假装加载成功；2s 后轮询仍失败会弹回错误态并（去重后）再报一次。若未来要更诚实，应给「已关闭的错误」单独记忆，而不是把它并入空态。
- **N1 不改（有意取舍）**：`messageKey` 形状门允许任意宿主字典 key（页可令宿主弹 `settings.*` 等文案）。这是「宿主 own words + 未知 key 丢弃」的设计取舍，非注入；未来收紧可换 per-plugin key 允许清单。
