# Tool Binding 设计：从"扩展分发平台"到"工具接入"

状态：方向已确认（2026-08-23），本文取代 `docs/plugin-system-audit.md` 中 Phase 3-8 的旧规划。
产品原则见 `docs/AGENT-NOTES.md`。

## 一句话

**Extension = ToolBinding**：一份描述（manifest，可自动生成）+ 一个可执行入口 + 开关。
PATH 发现、约定位置 manifest、手动连接、内置推荐只是四种不同的"找到它"的方式，
进入系统后完全同质。NPM 分发冻结为 legacy，不再维护。

## 现状盘点（2026-08-23 侦察结论）

可复用的核心（保留）：

| 能力 | 位置 | 说明 |
|---|---|---|
| Provider 动态协议 | `provider.rs` describe/complete/diagnose | 差异化能力，保留 |
| 结构化执行计划 | `provider.rs:526` + ExecutionPlanCache | 安全边界干净 |
| Catalog 搜索/评分/命名空间 | `catalog.rs` | launcher 本体 |
| 本地连接全通路 | `install.rs:334 create_custom_integration` | 已实现"本地程序→manifest→lock"，通用工具一键接入直接复用 |
| PATH 扫描引擎 | `inventory.rs` ToolInventory（TTL 缓存） | `extensions_search_tools` 已暴露 |
| system 绑定指纹校验 | `tool_lock.rs` | 保留 |

要消除的不通顺：

1. **双轨制**：built-in 走 `static_adapter.rs` 内嵌 JSON 特殊路径；其他走 lock+manifest。
   同一件事两套代码。
2. **发现结果无处安放**：`ToolCandidate` 只服务于自定义集成表单的路径选择器；
   面板没有"检测到这些工具，一键接入"的主路径。
3. **broken 不进 catalog 过滤**：运行期工具消失只 eprintln，面板仍显示 enabled。
4. reprobe/launch 忽略 manifest 声明（audit P1，暂缓到 provider 收敛时一并修）。

## 目标架构

```text
发现层
  ├─ PATH 扫描            (inventory.rs，已有)
  ├─ 推荐清单              (v-tools 清单=数据，不再是代码特例)
  ├─ 约定位置 manifest     (~/.config/floter/tools/*.json，远期)
  └─ 手动选择文件          (现有 local 连接)
        ↓ 全部归一
ToolBinding (= ExtensionLockEntry, 单一模型)
  ├─ id / name / executable / 来源
  ├─ enabled / broken(含原因，诚实呈现)
  └─ 权限声明（纯披露）
        ↓
Provider 层（静态描述式为主，动态协议可选增强）
        ↓
Execution Plan → PTY（现状保留）
```

关键决策：

- **不新建 ToolBinding 数据结构**——`ExtensionLockEntry` 已经能表达全部信息，
  新建第二套结构等于制造 audit 里批评过的"多真源"。改造对象是写入路径和 UI。
- **v-tools 平权**：删除 `static_adapter::load_bundled()` 编译期内嵌特例，
  v-tools 变成随 app 附带的推荐清单数据；接入后与其他工具走同一条
  custom-integration 通路。（分步执行，先加平权通路再拆特例。）

## 分阶段落地

### Phase A（本次）：通用工具一键接入

- 后端：`connect_tool` 命令——给定 `ToolCandidate`（来自 PATH 扫描），
  复用 `create_custom_integration` 内部逻辑生成 manifest + descriptor
  （命令 id = 可执行文件名），权限默认最小集
  （environment/process-spawn/filesystem-read，诚实披露），写入 lock。
- 前端：Installed tab 顶部新增「检测到的工具」区块：列出未接入的 PATH 工具，
  一键接入；已接入的不重复出现。
- i18n en/zh 同步。

验收：PATH 上有 `v` 时，面板可见 → 一键接入 → 搜索框敲 `v --version`
或其子命令即可执行，与手动创建的自定义集成行为完全一致。

补充（connect-time 参数提示）：一键接入在生成 descriptor 时会对可执行文件运行一次
带总超时（约 3s）的 `--help` 探测（复用 `capability_probe`，接受任意退出码），并用通用
逐行解析器 `src-tauri/src/extensions/help_args.rs` 尽力提取选项定义（兼容 clap/argparse/
cobra/go flag 风格），写入静态描述的 `arguments`。解析失败或无输出时保持 `[]`，
绝不阻断连接；launcher 补全（`static_completions`）因此能直接提示已接入工具的参数与说明。

补充（子命令展开）：当顶层 `--help` 是「子命令/插件列表」而非选项列表时（如 v 的插件清单、
cobra 的 Available Commands 段落），解析器会额外尽力提取每个子命令的名称、别名与描述，并对其
逐一探测二级帮助（先 `<sub> --help`，无输出再试 `<sub> -h`；最多探测 12 个，整体约 4 秒预算，
并发执行、结果按原顺序归位）。接入时除根命令外，还会为每个子命令生成一个独立的描述符命令
（别名保留，`argsPrefix` 追加子命令名，与推荐工具的多命令 descriptor 形状一致，上限 40 个），
launcher 因此能按子命令补全其专属 flag。全部步骤仍为尽力而为：任一失败只回退为根命令
（或无参数），绝不阻断连接。探测结果同时以 `help-probe.json` 旁车文件记录在集成目录中，
并在重新启用集成时与详情抽屉的「重新探测命令」按钮上复用同一推导管线刷新描述符。

### Phase B：v-tools 平权

- `connect_bundled` 改为调用与 `connect_tool` 相同的通路；
- `bundled_static_provider_commands` 特殊分支删除；
- 推荐 UI 从"内置 tab"改为 Installed 页的推荐卡片（数据驱动）。

### Phase C：broken 状态贯通

- catalog 加载时对 binding 失败的条目标记 broken（带错误码落盘）而非仅 eprintln；
- 面板行内显示 broken 徽标 + "重新检测"操作（已完成：行内异常徽标悬停显示原因，broken 行提供统一"重新检测"动作，复用 `extensions_repair`）。

### Phase D（远期）：约定位置 manifest 与分发渠道

- `~/.config/floter/tools/*.json` 自动发现；
- 届时按真实需求评估任何新分发渠道（含 NPM 解冻与否）。

## 冻结区

以下代码不再维护、新功能不得依赖（Phase C 完成后评估物理删除）：
`official_index.rs`、`download.rs` 的 SRI 部分、`registry.rs` NPM 解析、
Updates tab 及 update/rollback/reinstall/pin/channel 命令链。

## 插件 HTML 页面机制（2026-08-30 新增）

剪贴板面板曾是一个硬编码的 ViewMode，违背了「搜索+终端」的基础原则。现在 floter 有一套
**通用的插件页面机制**：任何插件（内置基础插件、未来的外部集成）都可以声明一个 HTML 页面，
打开时替换整个画布区域，复用终端页面的窗口几何与外壳。

### 声明

- 内置插件：注册在 `src-tauri/src/plugin_pages.rs` 的静态 registry 中——每条
  `PluginPageDescriptor` 含稳定 id、i18n 标题/描述 key、页面资源路径
  （如 `plugins/clipboard/index.html`，随前端 dist 打包）与 **命令白名单**。
- 外部集成（远期）：在其集成目录放置 `page.html`，由 descriptor 引用；白名单随 descriptor
  一并声明。机制本身不感知剪贴板。
- 扩展生态可见性：`builtin_plugins_list` 命令把基础插件列进 Installed 页，开关就在那里
  （这是唯一的开关位；旧的 General 设置项已移除）。插件的持久化状态即其原有 settings 字段
  （`clipboard_history_enabled`，serde default 保证旧文件可读）——**没有第二个僵尸开关**。
  启动时按该状态 reconcile 监听器与热键（`clipboard_history::sync_runtime`）：
  开 → 监听 + 全局热键注册；关 → 双双拆除。

### 渲染

- `show_plugin_page` 命令（原 `show_clipboard` 泛化改名）：页面占用的就是终端页面的同一
  窗口，走同一套 saved-size + clamp + resize 机器。尺寸所有权完全在后端，前端在页面存续期
  绝不 setSize。PTY 在底下继续存活。
- 同一时间只允许一个插件页面打开；Esc / Cmd+W / Ctrl+W / 再次调用关闭并回到记忆中的表面。
- 实现选择 **sandbox iframe（无 allow-same-origin）** 而非 srcDoc 注入：外部插件 HTML 的
  可信度低于我们自己的代码。不透明 origin 意味着页面无法触碰宿主 DOM、没有任何 Tauri IPC
  面，其唯一能力是白名单桥允许的事。内置剪贴板页也走同一条管线（dogfood）。

### Bridge

页面与宿主之间的全部通信是一条极简 postMessage 协议（`src/plugin-pages.ts`，纯逻辑、有
node 测试）：

```
page → host: {floter:"invoke", id, command, args?}   // 白名单校验后由宿主 invoke()
host → page: {floter:"result", id, ok:true, value} | {…ok:false, error}
page → host: {floter:"close"}                        // 关闭页面回到记忆表面
```

宿主侧 `src/plugins/PluginPageHost.tsx` 校验 `event.source === iframe.contentWindow` 与
每插件命令白名单后才执行调用；现有权限模型日后可在同一接缝上继续收紧。启动参数（语言、主题、
透明度）通过 URL query 传入——沙箱页无法读取存储或宿主文档。

### 生命周期与调用路径

- **一条内部路径，多个触发源**：`plugin_pages::open_plugin_page`（总是打开）与
  `toggle_plugin_page`（热键专用：同页已开则隐藏）。触发源：全局热键（默认 Alt+V）、
  CLI `floter clip`、launcher 系统条目、冷启动请求（setup 记入 pending，前端挂载后取走；
  已运行实例经 single-instance 回调或 Wayland 控制 socket 的 `clip` 命令转发）。
- `floter clip` 冷启动 = 正常启动后直接落到剪贴板页；其他未知参数行为不变。

### 现状

- 第一个用户：剪贴板历史（`plugins/clipboard/index.html` +
  `src/plugins/clipboard/main.ts`），已从 React 面板提取为独立 HTML+JS，经 Vite 多入口
  构建（`vite.config.ts` rollupOptions.input），复用共享样式表与纯逻辑模块。
- CSP 相应放宽了一处：`img-src` 增加 `blob:`（缩略图字节过桥后在沙箱页内转 blob URL 渲染）。
