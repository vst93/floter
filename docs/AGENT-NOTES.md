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

## HIG-2 记（2026-09-17）：elevation 接线 / accent 预算 / 层纪律 / a11y 兜底 / 浅色主题

1. **elevation 阶梯是唯一阴影来源**。`--elev-0`（平面上控件：inset edge+rim）、`--elev-1`（按平，none）、`--elev-2`（浮起 pane，`--glass-raised-shadow` 的基座）、`--elev-3`（浮层：drawer/dialog/toast/menu）四档；派生档 `--elev-hover(-soft)`、`--elev-3-edge/-compact`、`--elev-bar`、`--elev-track`、`--elev-keycap`、`--elev-halo` 都写成 `var(--elev-*)`/已有 token 的组合。宿主表里不得再出现阴影字面量：tint 必须是 token，带 cast 的层必须点名 rung（`tests/hig-craft.test.ts` 的 wiring 断言）。唯二例外：`.platform-*` 的窗口服务器边框阴影（另一套 shadow authority，已 token 化）、以及 `var(--glass-field-shadow)` 这类纯 token 别名（在定义处审计）。
2. **accent 预算 = `--accent-budget: 2`**，单位是「accent 色**填充**的面」（background 为 `--accent`/`--accent-tint`/`--glass-raised` 家族）。文字/描边/焦点环/状态点/进度条不算填充，不耗预算。「选中的分段控件」（theme/language/cursor/glass step）是**状态不是主操作**，一律走 `--glass-raised-quiet`（中性 raised pane + accent 1px keyline）；同一个视图可能有 4 个选中态，若都给 tint 就爆预算。**计数按选择器计，不按实例计**（同一选择器可多实例同屏，如 6 个 switch 可同时 on；实例级由像素面积取证兜底）。`tests/accent-budget.test.ts` 做**全表扫描**：解析 `src/styles/*.css` 全部规则体、按 file→view 映射聚合，除逐视图 ≤ budget 的正面断言外，还有**补集断言**——凡命中 accent 填充家族、非 hover/active/focus、非伪元素、非状态点/进度条的选择器，若不在任何 view 的清单里即红。所以「新增一个 accent 面」会直接红，而不是被封闭清单漏掉。新增任何 accent 填充前先跑该测试。
3. **层纪律的真断言**。`tests/hig-craft.test.ts` 的 `the material stays on the functional layer` 现在断言**内容面画的是 standard material**（`--surface-*`/控件阶梯或 transparent），且内容面规则体不得出现 `backdrop-filter` 或 `--glass-tint*`/`--glass-float`（frame/floater 的 tint）；**`background-image` 一并解析**——内容面只允许 `var(--scroll-edge-band*)` 这类 token 化值，raw 色/hex/inline gradient → 红。同一选择器的**每条**规则都检查（后置重复规则不能绕）。这取代了旧版「规则体不含 backdrop-filter 字样」的同义反复断言——旧版对几乎任何规则都成立，无法因它声称的原因变红。
4. **a11y 三兜底必须覆盖新表面**。R7-4/R7-5 新增的 `plugin-page-host__frame/-topbar/-loading/-error`、`.app-toast`、`.settings-save-alert--toast` 已补进 RT（`--surface-opaque` + 去 blur）、IC（`--stroke-contrast` 描边/边框）、RM（`animation: none`）三个块；`tests/a11y-backstops.test.ts` 逐一断言 + 「宿主表里每个带 animation 的选择器都要在 RM 块里被 neutralize」的结构断言。**扫描范围是宿主目录 `src/styles/` + `src/extensions/`**（`ComponentizedUninstallDialog.css` 这类组件私有但消费宿主 token 的表也在内），`src/plugins/clipboard/page.css` 除外（页边界）。新增动画表面时必须同步三块。
5. **浅色主题是「起步」不是完整设计**。真实调色板在 `[data-theme="light"]`（App.tsx 写入）；`@media (prefers-color-scheme: light) { html:not([data-theme]) { … } }` 是首帧兜底（`auto` 为默认，属性落盘前不能闪深色）。两个块的取值由 `tests/light-theme.test.ts` 钉死一致；MUST_COVER 清单（文字/表面/描边/accent/terminal 五组）必须全覆盖，palette-independent 清单（radius/type/duration/elev/step）不得重复。**可读性红线一句话**：light 下 primary/secondary/muted/accent 全部 ≥4.5:1（muted 本轮从 0.74 提到 0.78，复算 4.98:1 on recess、4.57:1 on hover 面，不再是 AA 正文边缘）。**未覆盖**：完整浅色设计、第三方案例、plugin page 自己的 light 媒体查询（其 `[data-theme="light"]` 已存在，只是没有 media-query 首帧兜底；页面自带 bootstrap 参数，首帧风险低）、窗台平台阴影的浅色微调。列为后续独立轮。
6. **`--glass-raised-quiet` 是中性 raised pane**（暗= `--glass-control-hover`，亮同左），用于所有「选中/激活状态」以及同类状态/通知面（`.extension-status--recommended`、`.extension-health__tag`、`.extension-row__progress` 等）。`--glass-raised`（= accent tint）从此只留给真正的 accent pane（launcher 选中行、clipboard 选中行、sidebar 当前页）。旧的 `--glass-raised-quiet-rim` 因全仓零消费已删。
