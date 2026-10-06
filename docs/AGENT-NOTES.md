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

## 测试门控规则：CI 只有 Linux，平台相关断言必须 Linux 门控（2026-10 / R104）

1. **CI 只有 Linux**。`.github/workflows/pull-request.yml` 的两个 job（`frontend` / `rust`）都跑在 `ubuntu-latest` / `ubuntu-22.04`；**macOS 没有任何自动化测试覆盖**。macOS 上「本机绿」不代表任何东西——没有 CI 跑它。
2. **`#[cfg(unix)]` 不是平台门控，是平台假设**。凡断言依赖 `/proc`（`/proc/<pid>`、`/proc/<pid>/task/<pid>/children`）的测试，必须写 `#[cfg(target_os = "linux")]`，不得写 `#[cfg(unix)]`。在 macOS 上 `/proc` 恒不存在，`#[cfg(unix)]` 的测试会照常编译、照常运行、并**恒真通过**——这是假绿，比没有测试更糟，因为它让人以为有覆盖。先例：`src-tauri/src/process_launch.rs` 的 `/proc` 测试用 `#[cfg(target_os = "linux")]`；R104 修掉了 `extensions/probe_runner.rs`、`extensions/capability_probe.rs`、`extensions/run.rs` 三处。
3. **两种修法择一，优先门控**。要么给该断言/测试加 `#[cfg(target_os = "linux")]`，要么改用平台无关探测（如 `kill(pid, 0)`）。选门控的理由：CI 只在 Linux 跑，平台无关探测在 macOS 上无人验证，等于用一个未测代码路径换掉一个恒真断言。**不为测试新增依赖**（`sysinfo` 不在依赖树里，别加；`libc` 有，但别只为测试用它）。
4. **门控只动测试**。门控的是断言 / 测试函数 / 测试专用 helper，不是被测的生产代码；若门控会让某个 helper 在 macOS 上变成死代码，就把它的平台专属部分也一并门控（别留 `dead_code` 警告）。

## 反馈通道（全应用一套 Toast，2026-09-17 / R7-5 起约束）

> **2026-10 / R96 退役注记**：本节下面的 1-4 条描述的是已删除的插件页 bridge
> （`host-notify` / `notify-retry` / `createRetryRegistry`）。内置 iframe 页 R33 退役、
> R76 物理删除，对外协议与 hello-page 示例 R96 物理删除；今天没有任何页面侧生产者，
> 宿主 toast 只由宿主自身的代码调用。保留这些历史条目只为解释 `notify(kind, text, action?)`
> 的 action 槽与 `#floter-app-toasts` 单栈约束的来源。

**唯一反馈面**是宿主 toast（`src/components/ToastStack.tsx` + `src/toast-state.ts`）。任何表面——包括沙箱 iframe 里的插件页——都只发消息、不画自己的提示条。时长、位置、视觉只有一份来源：`TOAST_DISMISS_MS`（error 8s / success 4s）与 `#floter-app-toasts`。

1. **插件页不得自绘任何 notice/toast/banner**。页内提示条（如已删除的 `.clipboard-panel__notice`）与自有 dismiss 定时器都不再有位置；失败一律走 bridge 的 `host-notify` 消息（该 bridge 已于 R96 删除）。
2. **线上只传字典 key，不传文案**。`host-notify` 的 `messageKey` 由宿主用 `src/i18n.ts` 自己解析，未知 key 直接丢弃（`isMessageKey`）。宿主侧调用 `notify(kind, text, action?)` 时已是译文；插件页侧永远不持有宿主文案，也不增加宿主字典的耦合面。
3. **重试语义单向**：页说「这个失败了，可以重试」→ 宿主 toast 出一个动作按钮 → 用户按下 → 宿主回发 `notify-retry` → 页自己重跑。宿主绝不替页重放命令（它不知道页失败的是哪一次动作、也不持有页的状态）。**重试带关联 id**（2026-09-17 微修）：`host-notify` 由页生成 `id`，宿主的 `notify-retry` 原样回传；页内用 `createRetryRegistry()`（容量 = `MAX_TOASTS`）按 id 找 thunk，过期/未注册/已消费的 id 静默丢弃。此前单槽 `pendingRetry` 会让旧 toast 的 Retry 执行最新动作。
4. **位置按表面归属，不按表面另起一套**。`#floter-app-toasts[data-surface=…]` 只做「在这个窗口里落哪」的修正，不做第二套栈：全高窗口（settings / terminal / plugin）共用默认 `top:64px; right:16px`（插件页 28px 顶栏之下仍余 36px）；只有 ~58px 的 collapsed 需要专门规则。**新增表面默认复用默认值，除非实测遮挡**。

## 三态规范（空 / 加载 / 错误，2026-09-17 / R7-5 起）

适用对象：任何会拉取数据的表面（插件页、面板、抽屉）。三态都必须能互相切换，且不闪布局。

1. **空态** = 标题 + 说明 + （可选）主操作。标题说「没有东西」，说明说「为什么 / 怎么办」（如 `clipboard.empty`），有可执行的下一步才放按钮；不要用空态承载错误（错误有自己的态）。形状用宿主的 `.settings-empty` 基元（`SettingsEmpty`：`__title` / `__hint` / `__icon` / `__action`）；插件页自绘时沿用同一套 class 名，不要另发明形状。
2. **加载态** = **行内 spinner，不是整块替换内容**。只要有旧内容在场就不换掉它（后台轮询、刷新、重试一律如此），只有“首屏还没东西可显示”才画 spinner 行；行本身要有固定高度（避免到达时跳）。判据落成代码：一个 `loaded` 标志 + 「已是 spinner 就不重建节点」（重建会重启动画）。
3. **错误态** = 标题 + 为什么 + **两个控件槽：重试 + 关闭**。重试只在「重试真的可能成功」时出现（后端被关掉、命令不存在 → 只给说明和关闭，不给死路按钮）；关闭永远在，且语义是「不再画这个态」，不是假装加载成功了。同一错误重复渲染要复用已画节点（`data-failure-state`），否则重绘会丢焦点并重放进入动画。

参考实现：宿主侧 `src/settings/SettingsRows.tsx` 的 `SettingsEmpty`（`.settings-empty` 家族，样式在 `src/styles/settings.css`）。旧的 `src/plugins/clipboard/main.ts` 参考实现已随 R76 物理删除；后续新增表面照宿主基元沿用，不要另发明形状。

### 微修补记（2026-09-17，R7-5 复核后）

- **错误态节点复用 ⇒ 不要手动 disable 控件**。`renderLoadFailure` 的节点靠 `data-failure-state` 跨重绘复用，所以 `retry.disabled = true` 会活过点击：重试再失败时节点不重建，按钮永久禁用。重入由 `reload()` 的 `reloadPending` 去重兜住，数据态决定按钮外观。新增任何「复用已画节点」的态都适用此条。
- **自动轮询的失败要按 key 去重**。`createFailureDeduper()` 默认 30s 窗口；只给 `clipboard.loadFailed`（2s 轮询那条）用，用户手势失败（copy/delete/clear）每次都报，成功加载后 `clear()` 重新武装。
- **M4 不改（有意语义）**：错误态 dismiss 后若列表为空会画空态（「Nothing copied yet」）。这是「不再画这个错误态」的可辩语义，不是假装加载成功；2s 后轮询仍失败会弹回错误态并（去重后）再报一次。若未来要更诚实，应给「已关闭的错误」单独记忆，而不是把它并入空态。
- **N1 不改（有意取舍）**：`messageKey` 形状门允许任意宿主字典 key（页可令宿主弹 `settings.*` 等文案）。这是「宿主 own words + 未知 key 丢弃」的设计取舍，非注入；未来收紧可换 per-plugin key 允许清单。

## HIG-2 记（2026-09-17）：elevation 接线 / accent 预算 / 层纪律 / a11y 兜底 / 浅色主题

1. **elevation 阶梯是唯一阴影来源**。`--elev-0`（平面上控件：inset edge+rim）、`--elev-1`（按平，none）、`--elev-2`（浮起 pane，`--glass-raised-shadow` 的基座）、`--elev-3`（浮层：drawer/dialog/toast/menu）四档；派生档 `--elev-hover(-soft)`、`--elev-3-edge/-compact`、`--elev-bar`、`--elev-track`、`--elev-keycap`、`--elev-halo` 都写成 `var(--elev-*)`/已有 token 的组合。宿主表里不得再出现阴影字面量：tint 必须是 token，带 cast 的层必须点名 rung（`tests/hig-craft.test.ts` 的 wiring 断言）。唯二例外：`.platform-*` 的窗口服务器边框阴影（另一套 shadow authority，已 token 化）、以及 `var(--glass-field-shadow)` 这类纯 token 别名（在定义处审计）。
2. **accent 预算 = `--accent-budget: 2`**，单位是「accent 色**填充**的面」（background 为 `--accent`/`--accent-tint`/`--glass-raised` 家族）。文字/描边/焦点环/状态点/进度条不算填充，不耗预算。「选中的分段控件」（theme/language/cursor/glass step）是**状态不是主操作**，一律走 `--glass-raised-quiet`（中性 raised pane + accent 1px keyline）；同一个视图可能有 4 个选中态，若都给 tint 就爆预算。**计数按选择器计，不按实例计**（同一选择器可多实例同屏，如 6 个 switch 可同时 on；实例级由像素面积取证兜底）。`tests/accent-budget.test.ts` 做**全表扫描**：解析 `src/styles/*.css` 全部规则体、按 file→view 映射聚合，除逐视图 ≤ budget 的正面断言外，还有**补集断言**——凡命中 accent 填充家族、非 hover/active/focus、非伪元素、非状态点/进度条的选择器，若不在任何 view 的清单里即红。所以「新增一个 accent 面」会直接红，而不是被封闭清单漏掉。新增任何 accent 填充前先跑该测试。
3. **层纪律的真断言**。`tests/hig-craft.test.ts` 的 `the material stays on the functional layer` 现在断言**内容面画的是 standard material**（`--surface-*`/控件阶梯或 transparent），且内容面规则体不得出现 `backdrop-filter` 或 `--glass-tint*`/`--glass-float`（frame/floater 的 tint）；**`background-image` 一并解析**——内容面只允许 `var(--scroll-edge-band*)` 这类 token 化值，raw 色/hex/inline gradient → 红。同一选择器的**每条**规则都检查（后置重复规则不能绕）。这取代了旧版「规则体不含 backdrop-filter 字样」的同义反复断言——旧版对几乎任何规则都成立，无法因它声称的原因变红。
4. **a11y 三兜底必须覆盖新表面**。`.app-toast`、`.settings-save-alert--toast` 等宿主表面已补进 RT（`--surface-opaque` + 去 blur）、IC（`--stroke-contrast` 描边/边框）、RM（`animation: none`）三个块；`tests/a11y-backstops.test.ts` 逐一断言 + 「宿主表里每个带 animation 的选择器都要在 RM 块里被 neutralize」的结构断言。**扫描范围是宿主目录 `src/styles/` + `src/extensions/`**（`ComponentizedUninstallDialog.css` 这类组件私有但消费宿主 token 的表也在内）。R76 删除了退役插件页及其宿主 chrome（`.plugin-page-host*` / `.clipboard-panel*`），扫描范围不再需要页边界例外。新增动画表面时必须同步三块。
5. **浅色主题是「起步」不是完整设计**。真实调色板在 `[data-theme="light"]`（App.tsx 写入）；`@media (prefers-color-scheme: light) { html:not([data-theme]) { … } }` 是首帧兜底（`auto` 为默认，属性落盘前不能闪深色）。两个块的取值由 `tests/light-theme.test.ts` 钉死一致；MUST_COVER 清单（文字/表面/描边/accent/terminal 五组）必须全覆盖，palette-independent 清单（radius/type/duration/elev/step）不得重复。**可读性红线一句话**：light 下 primary/secondary/muted/accent 全部 ≥4.5:1（muted 本轮从 0.74 提到 0.78，复算 4.98:1 on recess、4.57:1 on hover 面，不再是 AA 正文边缘）。**未覆盖**：完整浅色设计、第三方案例、窗台平台阴影的浅色微调。列为后续独立轮。
6. **`--glass-raised-quiet` 是中性 raised pane**（暗= `--glass-control-hover`，亮同左），用于所有「选中/激活状态」以及同类状态/通知面（`.extension-status--recommended`、`.extension-health__tag`、`.extension-row__progress` 等）。`--glass-raised`（= accent tint）从此只留给真正的 accent pane（launcher 选中行、clipboard 选中行、sidebar 当前页）。旧的 `--glass-raised-quiet-rim` 因全仓零消费已删。

## 独立窗口（detached plugin window）路线裁决（2026-10，R84-R86）

- R84 `19acc29`：external 插件输出可钉独立窗口（label `plugin-detached`，二次 Pin 替换内容）；R85 `bdddb2b`：几何/位置持久化（拔副屏回退默认位、size 仍恢复）。
- **内建模式 Pin 明确不做**：内建 iframe 页 R33 已退役，descriptor 的 `page` 字段与测试锁在 R96 一并物理删除，内建模式是交互式搜索 UI 非答案面；snapshot 便宜但无用（动作全丢），live 需为三数据源新建变更事件通道（触碰「不为边际功能新开通道」边界）。**Pin 保持 external-only。**
- 多实例、设置广播：暂缓，触发条件见 R86 报告（`/tmp/floter-r86-report.md`）。
- 若未来需要「钉住单条内容」：snapshot-text 最小切法（复用 `PluginTextView`，~80-120 行 / 3 文件，无新命令无事件，退役干净）。

## 依赖政策：声明宽度、收紧判定、豁免与 npm 侧纪律（2026-10 / R110）

R105→R109 把供应链的**现状**清完了（rustls patch、6 个死依赖出清、rusqlite 0.40、audit 归零），
留下的是**政策债**：`src-tauri/Cargo.toml` 的声明面普遍是裸 major（`"1"`/`"3"`/`"5"`）或裸 minor
（`"0.4"`/`"0.28"`），`cargo update` 没有任何「已审计下限」挡着。R110 把高风险声明收到 patch 位并
落成守卫 `tests/r110-deps-policy.test.ts`。以下是此后新增/修改依赖时的规则。

1. **新依赖默认宽度 = 带 patch 位的 caret**（`"0.28.1"`、`"1.53.1"`），不写裸 major/minor。
   理由不是「更安全」，是**把已审计的版本记进声明**：裸 `"1"` 让 `cargo update` 的下限是 1.0.0，
   一次 update 可以把整条 1.x 线拉走；带 patch 位后下限就是我们编译并跑过测试的那个版本。
   **禁止**默认写 `=X.Y.Z`——等号 pin 是维护税，不是安全；只在下面第 2 条命中且确有需要时才用。
   **Cargo caret 语义要点**（决定「收紧」到底收什么）：`^1.53.1` = `>=1.53.1, <2.0.0`（1.x 的
   ceiling 挡不住跨 minor，只有 `=` 或显式范围能挡）；`^0.28.1` = `>=0.28.1, <0.29.0`（0.x 的
   ceiling 本来就把 minor 钉住了，加 patch 位只抬下限）。所以对 1.x crate，「收紧」的实际效果是
   **抬下限 + 记录审计版本**，不是封 minor；报告里必须如实这么写，不要假装封住了。
2. **收紧判定 = 命中以下任一条**，否则不动（避免把全表升级成 `=X.Y.Z`）：
   - **传递依赖树大**：tokio、serde、alacritty_terminal、reqwest 这类一次 update 能牵动几十个
     `[[package]]` 的；
   - **历史上有破坏性 minor**：chrono 0.4 线、crossterm 0.26→0.28、base64 0.21→0.22 这类
     0.x 上真发生过 breaking 的；
   - **安全敏感**：`libc`（unsafe FFI）、`png`（解码剪贴板里的不可信图像）、`jsonschema`
     （解析不可信插件 manifest）、`serde_json`（IPC/配置的不可信输入）、reqwest/rusqlite
     （TLS 栈 / 解析不可信数据库）。
   R110 实际收紧 8 条：serde、serde_json、chrono、tokio、crossterm、jsonschema、libc、png
   （≤8 是当轮预算，守卫把条数钉住）。
3. **豁免：tauri 家族（`tauri`、`tauri-build`、`tauri-plugin-*`、`tauri-nspanel`）一律不 pin**。
   理由：Tauri 自己的 release train 背 semver，官方 pin 会与未来 C3 升级轮直接冲突；`tauri-nspanel`
   等 git 依赖已用 `rev` 钉死（比任何 caret 都紧）。守卫**反向**断言这族不得出现 `=` 等号 pin，
   npm 侧 `@tauri-apps/*` 同样保持 `^`/`~` 范围、不得精确 pin。**`portable-pty` 同理零触碰**
   （它是 vendored `qscreen-daemon` 的依赖，不在本仓声明面）。rusqlite 保持 `"0.40"` 不加等号
   （0.x 已锁 minor，加 patch 位只抬下限，不值得动；R109 守卫已钉死这一行）。
4. **npm 侧：`npm update` 必须限定到 dev 闭包**（R106 教训）。裸 `npm update` 实测会同时升
   `react`/`react-dom`/`lucide-react`/`@tauri-apps/api` 四个**运行时**依赖，vite 产物从 671,272 B
   涨到 704,492 B（+33 KB），直接撞「前端零改动 / js/css 逐字节不变」红线。修补 dev advisory 时
   把包名逐个列出来（`npm update vite esbuild postcss …`），**永不**裸跑 `npm update`，也**不**跑
   `npm audit fix`（它会顺手动 lock 的运行时边）。判据：`git diff package.json` 必须为空、
   `dist/assets/*` 必须逐字节不变。

## Tauri 2 命令 panic 传播语义（R111/R112）

R111 只读普查了生产代码的 panic 面（10 处），结论是全部落在 S5「合同式」上，唯一例外风险是
`clipboard_history/mod.rs` 的 `mutate_history`——那里的 `expect` 落在**同步命令链**上，R112 已
消除。以下是为什么「同步命令链上的 panic」比普通 panic 严重，以及此后写命令的规则。

1. **同步命令（79 个）panic ⇒ 进程 abort**。证据链（tauri 2.11.5 / tauri-macros 2.6.3 /
   wry 0.55.1 / webkit2gtk 2.0.2，即 lock 现值）：
   - `tauri-macros` 的 `wrapper.rs:404-433` `body_blocking` 直接调用命令函数并
     `kind.block(result, resolver)`，没有任务边界；
   - 这个 wrapper 由 `Webview::on_message`（`tauri/src/webview/mod.rs:1742`，
     `manager/mod.rs:471` 的 `run_invoke_handler`）在**同步**路径上执行；
   - 入口是自定义协议处理器 `tauri/src/ipc/protocol.rs:75` 的 `webview.on_message(...)`；
   - 它在 Linux 上最终落到 webkit2gtk 的 `unsafe extern "C" fn callback_func`
     （`webkit2gtk-2.0.2/src/auto/web_context.rs:534`，`register_uri_scheme` 的 C 回调）。
   panic 从 `extern "C"` 帧里逃逸**不能 unwind**，只能 abort；R111 已实测进程 `exit 134`
   （SIGABRT）。前端拿到的是连接断开，不是错误。
2. **异步命令（38 个）panic ⇒ 该命令 promise 永久挂起**。`wrapper.rs:361-395` 的 `body_async`
   走 `respond_async_serialized`（`tauri/src/ipc/mod.rs:343-380`），最终
   `async_runtime::spawn`（`ipc/mod.rs:375`）把 future 交给 tokio；panic 在 tokio task 边界被
   吞掉，`return_result` 永不执行——前端 `invoke` 既不 resolve 也不 reject，**连错误提示都没有**。
3. **规则**：**同步命令链上禁止新增 `expect`/`unwrap`**（要么 `ok_or`/`?` 返回 `Err`，要么用
   `lock_or_recover` 这类 S5 兜底）；**异步命令优先返回 `Err`，不要用 panic 把 promise 挂死**。
   `clippy::unreachable` 在 `src-tauri/src/commands/*` 上的命中是 `tauri-macros`
   `wrapper.rs:221-229` 里 `if false` 类型检查宏的 span 错位，不是本仓代码，别照着它改。
   源码级守卫：`tests/r112-panic-surface.test.ts`（同步链唯一 abort 点零出现 + 本节锚点在场）。
