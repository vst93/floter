# 迭代计划：插件页保活 / 焦点恢复 / 剪贴板卡顿 / 通用 Toast

日期：2026-09-14
状态：任务 A 已完成（2026-09-14，经 pi 实现→审查→修复→nit 清理→终审 APPROVE 共 5 轮，node 111/111、tsc/build 全绿）；任务 B、C 待做
验证管线：`npx tsc --noEmit` · `npm run build` · `npm test`（node --test）· `cargo fmt --check` · `cargo test --lib`

---

## 背景

本轮用户反馈了四组问题：

1. 剪贴板插件打开报「插件加载失败」→ **已修复**（根因：`clipboard-history` 不在 Cargo 默认 feature，见下）。
2. 剪贴板面板「有点卡顿」→ **待做**（根因见任务 A）。
3. 退出插件回到搜索框后**输入焦点丢失** → **待做**（根因见任务 A，与卡顿同源）。
4. 集成页插件列表很长、滚到底部操作时，弹出提示必须滚回顶部才看得到 → **已修复**（改为 App 级 toast，见下）。

---

## 已完成（本轮已提交的改动）

### C1 · 剪贴板默认编译进产物
- 文件：`src-tauri/Cargo.toml`
- 变更：`default = []` → `default = ["clipboard-history"]`。
- 根因：整个 `clipboard_history` 模块 + 8 个 `clipboard_*` 命令被 `#[cfg(feature = "clipboard-history")]` 门控，而该 feature 不在默认集合里，导致发布/dev 默认构建**不含剪贴板**（监控线程不启动、命令未注册、终端复制粘贴的 `clipboard_read_text/write_text` 直接返回「feature disabled」）。页面本身能加载，但 bridge 调用 `clipboard_get_entries` 失败，于是渲染成「插件加载失败」，误导。
- 注意：**需要重新构建 + 重启应用才生效**。

### C2 · 剪贴板页错误态不再谎报「插件加载失败」
- 文件：`src/plugins/clipboard/main.ts`、`src/i18n.ts`、`src/plugins/clipboard/page.css`
- 变更：新增 `isBackendUnavailable(error)` 判定；后端不可用（feature 关闭/命令未注册/state 未 manage）时显示 `clipboard.pageUnavailable` + `clipboard.pageUnavailableHint`（提示去 设置 → 集成 打开），不再给无用的「重试」；瞬时错误才保留重试并显示 `clipboard.loadFailed`。

### C3 · 组件化卸载弹窗（上一轮遗留）
- 移除 `useTimedReset(uninstallDialogTarget, …)`（3 秒自动关闭多选弹窗）。
- `ComponentizedUninstallDialog.css` 用设计 token 重写并补齐 `.extensions-dialog-overlay/-dialog/-title/-hint/-actions`。

### C4 · legacy 卸载默认不再删用户数据
- 文件：`src-tauri/src/commands/extensions.rs`、`src-tauri/src/extensions/uninstall.rs`
- 新增 `UninstallRequest::from_legacy(id, remove_data)`：`remove_data` 缺省视为 `false`（仅删程序，保留 host config / tool data / artifacts），恢复历史契约，避免静默数据丢失。含单元测试。

### C5 · 操作进度条清理 + 缺失 CSS
- `ExtensionsPanel.tsx`：`runMutation` 的 `finally` 清除 `operationProgress[id]`（此前「Complete」会永久钉在行上）。
- `src/styles/extensions.css`：补 `.extension-row__progress`、`.extension-row__progress-text`、`.extensions-notice--warning`。

### C6 · 通用 App 级 Toast（解决「必须滚回顶部才看到提示」）
- 新增：`src/toast-state.ts`（纯逻辑）、`src/components/ToastStack.tsx`（`ToastHost` / `ToastStack`）。
- 根因（headless 实测确认）：`.extensions-toasts` 渲染在可滚动容器 `.settings-content` 内部。对最近祖先**可滚动**的 `position: fixed`，WebKit `top` 相对滚动内容而非视口——滚动后 toast 跑出视口（实测 `top=-1370`），只有滚回顶部才可见。
- 变更：toast 改为 portal 到 `.settings-card` 直接子节点（滚动容器之外），组件化为跨面板可复用的 `onNotify(kind, text)`；`ExtensionsPanel` 删除本地 `toasts` 状态与 `ExtensionsToast`，改调 `onNotify`；CSS 由 `.extensions-toasts*` 迁移为 `#floter-app-toasts` + `.app-toast*`（`z-index: 60`，`pointer-events` 仅按钮可点）。
- 测试：`tests/toast-state.test.ts`（队列上限/删除/超时区分）、`tests/extension-ui-surfaces.test.ts`（feature 默认值、各弹窗/进度/toast 的 CSS 存在性、剪贴板错误文案不复用 `plugin.pageError`）。

---

## 待做

### A · 插件页保活 + 关闭后焦点恢复（同时解决「卡顿」与「焦点丢失」）

**优先级：高** · **状态：已完成**（单实例 pluginLayer 跨四 mode 保活；ToastHost 按 data-surface 分表面定位；关闭后 focusCollapsedInput/focusTerminalView 抢回焦点 + iframe blur；plugin-layer 按 --window-radius 裁切、Windows inset:10px；测试 tests/plugin-page-persistence.test.ts 8 例。遗留备注：extensions.css 一处注释把假设前态写成了实际前态，纯注释问题）

**根因（已定位，未修）**
`src/App.tsx` 的四个 mode 分支各自 `return` 不同的根元素：
- `settings` → `.settings-shell`（~L1222）
- `plugin` → `.terminal-shell` + `.terminal-panel--plugin`（~L1396）
- `collapsed` → `.collapsed-shell`（~L1430）
- terminal → `.terminal-shell`（~L1686）

`PluginPageHost` 在 **3 处**出现：plugin 分支（`pluginId={pluginPageId}`）、collapsed 分支（`pluginId={null}`，`display:none` 包裹）、terminal 分支（`pluginId={null}`，`display:none` 包裹）。

由于四者根元素类型不同且 `PluginPageHost` 处于不同父节点，React 在 mode 切换时会**卸载旧树、挂载新树**，iframe 随之销毁重建——注释里宣称的「keep the iframe alive across toggles」并未生效。后果：
- **卡顿**：每次打开插件页都重新加载 iframe 文档（descriptor 请求 + 页面脚本 + `clipboard_get_entries` + 缩略图）。
- **焦点丢失**：关闭插件页时，持有焦点的 iframe 被移除，焦点回落到 `<body>`；`mode` 变 `collapsed` 时的 `focusCollapsedInput(90/140)` 又因 `autoFocus`/布局时序竞争而没稳定拿到焦点。
- 副产品：`App.tsx` L305 的对话框 `inert` 遍历里对 `extensions-toasts` 的排除是死代码（toast 已移出面板）。

**方案（代码结构提示）**
1. 在 `App.tsx` 顶层（四个 `if` 之前）定义一次 `pluginLayer`：
   ```tsx
   const pluginLayer = (
     <div className="plugin-layer" data-plugin-layer data-active={mode === "plugin" ? "true" : undefined}>
       <PluginPageHost
         pluginId={pluginPageId}
         language={language}
         theme={resolvedTheme}
         mainOpacity={normalizeOpacity(settings.main_opacity) / 100}
         terminalOpacity={normalizeOpacity(settings.terminal_opacity) / 100}
         onClose={closePluginPage}
       />
     </div>
   );
   ```
2. 四个分支都渲染同一实例：`<>{pluginLayer}<div className="…-shell">…</div></>`。要点：
   - `pluginLayer` 必须是所有分支返回树的**第 0 个兄弟节点**且类型一致，React 才会跨 mode 复用同一 iframe 实例（fragment 隐式保证）。
   - 删除三个分支里原有的 `PluginPageHost` 挂载点（plugin / collapsed / terminal）。
3. `ToastHost` 同理需在四个分支各挂一次（当前只在 settings/terminal 分支）——toast 也要跨 mode 保活；这要求它同样作为稳定的兄弟节点。
4. CSS（`src/styles/terminal.css`）：
   ```css
   .plugin-layer {
     position: fixed;
     inset: 0;
     z-index: 30;                /* 高于普通面板，低于 toast(60) */
     display: none;
   }
   .plugin-layer[data-active="true"] { display: block; }
   ```
   plugin 模式的 shell 仍渲染在 layer 之下（保留 `.terminal-shell` 背景/圆角），layer 覆盖其上。
5. **焦点恢复**：`closePluginPage`（`src/App.tsx` L442）除 `setPluginPageId(null)` / `setMode(...)` 外，在模式落地后显式抢回焦点：
   - 返回 collapsed：复用 `focusCollapsedInput()` 多次（参考现有 20/80/140ms 节奏）。
   - 返回 terminal：`focusTerminalView()`。
   - 关键：iframe 现在不卸载，关闭时应主动 `iframeRef.current.blur()` 或把焦点交给宿主输入，避免焦点留在已隐藏的 iframe 上。可在 `PluginPageHost` 的 `pluginId: null` 分支补 `contentWindow.blur()`，或在 `closePluginPage` 里 `focusCollapsedInput(0/90/140)`。

**验证**
- 打开/关闭剪贴板页 ≥5 次，观察网络面板无重复文档请求（表示 iframe 未重建）；输入过滤词后关闭再打开，过滤词/选中/滚动位置保留。
- 关闭后立刻键入，字符应进入搜索框（焦点在位）。
- `npx tsc --noEmit`、`npm run build`、`npm test` 全绿。

**注意/坑**
- `App.tsx` 四个分支的根元素类名不同（`collapsed-shell`/`terminal-shell`/`settings-shell`）。若担心 React 仍重建，可给四个 shell 统一加 `key`，或把 pluginLayer 的 `key` 固定。
- macOS 是 NSPanel（accessory，非激活），焦点语义需真机验证；headless 只能验证 DOM 结构与 scroll 定位。
- 改完删掉 `useDialogFocus` 里对 `extensions-toasts` 的排除（L305，已无该 class）。

---

### B · 剪贴板面板滚动/交互顺滑度

**优先级：中**

**候选根因（待实测确认）**
1. `render()` 每次都 `content.replaceChildren()` 全量重建所有行（`src/plugins/clipboard/main.ts` ~L526 起）。237+ 条记录时每次按键/选择变化都整表重排。方案：给行加 `data-id`，只 diff 变更行；或对长列表做窗口化。
2. `render()` 内 `filtered.forEach(...)` 对所有行 `renderRow`，其中 `formatClipboardAge` → 行内多次 `Date` 计算。可缓存 `now`（已做）并避免为不可见行建 DOM。
3. 定时刷新 `setInterval(reload, 2000)`（~L735）在页面可见时持续拉 `clipboard_get_entries`；若 `sameClipboardSnapshot` 判定为「变化」会触发整表重建。确认 snapshot 比较是否稳定。
4. 缩略图：每次 reload 对未缓存的 image/files 预览发起 `clipboard_read_image` / `clipboard_read_file_preview`，并发上限 4。首屏多图时会有批量解码。可懒加载（仅可视行）。

**建议先做**：测量面板在 200+ 条时的单次 `render()` 耗时（`performance.now()` 包一层），再决定是 diff 还是窗口化。

---

### C · 其它待清理项（低优先级，来自 `.pi-task-floter-audit.md`）
- 托盘菜单语言切换即时刷新、加速键提示。
- 设置侧边栏在 body 焦点下 ↑/↓ 切换页面。
- 退出确认框替换 `window.confirm`。
- `settingsSaveFailed` 提示的触发路径接线。

---

## 提交建议

本轮改动建议拆两个 commit：
1. `fix(clipboard): ship clipboard-history in default features; truthful page error state`
2. `feat(ui): reusable card-level toast stack (pinned outside scroll container)`

待做的任务 A 单独一个 commit：`fix(plugin-page): keep iframe alive across mode switches; restore launcher focus on close`。
