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

### Phase B：v-tools 平权

- `connect_bundled` 改为调用与 `connect_tool` 相同的通路；
- `bundled_static_provider_commands` 特殊分支删除；
- 推荐 UI 从"内置 tab"改为 Installed 页的推荐卡片（数据驱动）。

### Phase C：broken 状态贯通

- catalog 加载时对 binding 失败的条目标记 broken（带错误码落盘）而非仅 eprintln；
- 面板行内显示 broken 徽标 + "重新检测"操作（替代 repair/reinstall 四套按钮）。

### Phase D（远期）：约定位置 manifest 与分发渠道

- `~/.config/floter/tools/*.json` 自动发现；
- 届时按真实需求评估任何新分发渠道（含 NPM 解冻与否）。

## 冻结区

以下代码不再维护、新功能不得依赖（Phase C 完成后评估物理删除）：
`official_index.rs`、`download.rs` 的 SRI 部分、`registry.rs` NPM 解析、
Updates tab 及 update/rollback/reinstall/pin/channel 命令链。
