# Floter 插件开发者文档

面向**插件作者**：你写一个工具，Floter 把它的输出画进搜索框。

这份文档回答三件事：

1. **你提供什么** —— 一份声明（命令清单）和命令的输出；以及输出应当长什么样。
2. **框架给你什么** —— 搜索框内的插件模式（scope 图标、过滤器、列表/文本双形态、
   分页、快捷键、驻留期、三级 dismiss）、schema 驱动的配置弹层、集成页的开关。
3. **边界在哪** —— 哪些是框架的（渲染、快捷键、窗口高度、键盘、执行计划），哪些是
   你的（命令语义、输出内容、行内容）。

> 状态：本文描述的实现对应仓库 R39（`plugin_command_switches`、外部插件命令模式）。
> 内置插件（clipboard / browser）与外部插件走**同一套**契约，区别只在触发词与开关的
> 归属（见 §6）。

---

## 0 · 三十秒版本

1. 你的插件是一个**集成（extension）**：一个 manifest + 一个 provider descriptor。
2. descriptor 里的 `commands[]` 就是你的**命令清单**。每个命令在设置 → 集成页拿到
   一个开关；开关打开后，用户可以在搜索框输入 `命令名 ` 进入该命令的插件模式。
3. 进入模式后，输入框里的内容**就是命令行参数**；回车执行。
4. 你的命令往 stdout 打印：
   - 一个 JSON 数组（符合 §3 的列表协议）→ Floter 画成和内置插件一样的列表；
   - 其它任何内容 → 原样文本，最小 3 行；超过十行列表的高度后块内滚动。
5. 你不需要写 HTML、不需要碰 Tauri API、不需要自己画行、不需要处理快捷键。

---

## 1 · 架构总览

### 1.1 两种宿主表面

| 表面 | 谁拥有 | 插件作者需要做什么 |
| --- | --- | --- |
| **插件模式**（搜索框内） | 框架 | 声明命令；打印输出 |
| **配置弹层**（schema 驱动） | 框架 | 声明配置字段（可选） |
| **集成页开关** | 框架 | 无（命令清单来自你的 descriptor） |

插件模式是「搜索框被某个插件占用」的状态：输入框左侧出现 scope 图标和插件名，输入框
内容是该插件的 needle 或命令行参数，下方是插件的输出。

### 1.2 数据流

```text
用户输入 "search rust async"
        │
        ▼
App 的输入框 onChange
        │  先问内置触发器（browser / clip …），再问外部命令触发器
        ▼
enabledExternalCommands(registry, plugin_command_switches)   ← 命令级开关门禁
        │  命中的命令（commandId 或 alias + 空格）
        ▼
pluginMode = { scope: "external", extensionId, commandId }    ← 模式成为显式状态
query      = "rust async"                                    ← 字段只剩 needle / argv
        │
        ▼
useLauncherCatalog：把 (mode, run 状态) 变成 PluginEmission
        │
        ▼
resolvePluginView(emission)                                   ← 能力层，唯一的判定点
        │
        ├── output 符合列表协议 → { form: "list", tier, items, page }
        │        └─ 走 LauncherResults：编号快捷键、分页、视口、行图标
        │
        └── 否则 → { form: "text", metrics: { min, max, scrolls } }
                 └─ 走 PluginTextView：最小 3 行、上限为十行列表高度、超出滚动
        │
        ▼
回车（当视图没有可交互列表时）→ external_plugin_run
        │
        ▼
Rust：provider::execution_plan → run::execute_plan_background（无 shell）
        │
        ▼
PluginCommandOutput { success, exitCode, stdout, stderr, truncated }
        │
        └── 回到 PluginEmission（stdout，stderr 兜底）
```

### 1.3 平台边界

| 是**框架**的 | 是**插件**的 |
| --- | --- |
| 窗口高度（离散 band 表，见 `result-budget.ts`） | 输出内容与行内容 |
| `⌘1`-`⌘9` + `⌘0` 的分配（纯按序，见 §3.3） | 行的顺序与分组 |
| 键盘（方向键、回车、Esc / ⌘W） | 命令的参数语义 |
| 列表/文本的**判定**（`resolvePluginView`） | 输出的形状（数组 or 文本） |
| 分页窗口与滚动加载 | `cursor` 的语义（框架只回传，不解释） |
| 驻留期（R35）与 dismiss（R31） | 无 |
| 执行计划与权限（`execution.program`、`process-spawn`） | 命令声明的 argv 前缀 |

一句话：**你提供数据，框架决定它长什么样**。

---

## 2 · 你的插件是什么

外部插件在 Floter 里是一个**集成**（integration / extension），由两部分组成：

1. **manifest**（`floter.extension.json`）——身份、runtime、权限、provider 声明。
2. **provider descriptor**（`description.json`，或由 `describe` 子进程返回）——命令清单。

manifest 与 descriptor 的完整 schema 见 `docs/extensions/`（FEP 系列与
`plugin-page-protocol.md`）。本文只重复与搜索框插件模式相关的部分。

### 2.1 命令清单：`provider.commands[]`

命令清单**就是**你已经在 `description.json` 里写的 `commands`。框架不新增声明文件格式：

```json
{
  "protocolVersion": "1.0",
  "provider": { "id": "local.mytool", "name": "My Tool", "version": "1.0.0" },
  "commands": [
    {
      "id": "search",
      "name": "Search",
      "description": "Search the local index",
      "aliases": ["s", "find"],
      "execution": {
        "program": "self",
        "argsPrefix": [],
        "mode": "capture"
      }
    }
  ]
}
```

| 字段 | 必填 | 作用 |
| --- | --- | --- |
| `id` | 是 | 命令 id。它同时是**开关的内层键**和**搜索框的触发词**。 |
| `name` | 是 | 人类可读名。集成页的开关用它做标签；插件模式的 scope 名用它。 |
| `description` | 否 | 一行说明。集成页与开关的辅助文本。 |
| `aliases` | 否 | 额外触发词（同样需要「词 + 空格」才进入模式）。 |
| `execution` | 是 | 执行描述。`program: "self"` 表示跑插件自己的可执行文件；`argsPrefix` 是固定的前置参数；`mode` 是 `pty` / `capture` / `external`。 |

### 2.2 命令级开关（R39）

集成页（设置 → 集成 → 选中一个集成 → 命令列表）为**每个**命令渲染一个开关。

- 开关状态持久化在 `AppSettings.plugin_command_switches`：
  `extensionId -> commandId -> enabled`（Rust 侧 `BTreeMap<String, BTreeMap<String, bool>>`）。
- **缺省即关闭**。没有条目的命令从未被用户打开过，因此不可呼出；显式 `false` 与缺省
  同义。用户的原话是「打开后就可以允许在搜索框内呼出插件」。
- 一个插件**没有任何**开启的命令 → 它在搜索框里**不出现**（触发器词落回普通搜索）。
- 开关只影响「插件模式」这条通路（呼出 + 执行）。命令搜索（catalog）是框架的另一个
  既有能力，不受这个开关控制。

### 2.3 权限与执行路径

- 你的命令由 Rust 侧构建执行计划（`provider::execution_plan`）并**直接 spawn**：
  没有 shell，没有字符串拼接，每个参数是一个独立的 argv 项。
- `execution.program` 不是 `self` 时，需要 `process-spawn` 权限，且路径必须是
  runtime root 下的相对路径（不允许绝对路径或 `..`）。
- 需要宿主环境变量时声明 `environment` 权限。
- 输出有超时与字节上限（`RUN_TIMEOUT` / `MAX_RUN_OUTPUT_BYTES`），超限截断并置
  `truncated: true`。

---

## 3 · 列表协议

### 3.1 怎么算「列表」

你的命令往 stdout 打印一个 **JSON 数组**（字符串形式即可），数组的**每一项**都符合
下面某一族的形状 → 框架画成列表。**只要有任意一项不符合，整份输出都当文本**
（`resolvePluginView` 的规则，见 §4）。

三种行族：

| `family` | 谁用 | 必填字段 |
| --- | --- | --- |
| 省略 / `"plugin"` | **外部插件（你）** | `id`, `title` |
| `"browser"` | 内置浏览器插件 | `id`, `title`, `url`, `profileKey` |
| `"clipboard"` | 内置剪贴板插件 | `id`, `title` |

### 3.2 通用行（`family` 省略或 `"plugin"`）

```json
[
  {
    "id": "row-1",
    "title": "First result",
    "subtitle": "one line of context",
    "icon": "star",
    "group": "Favorites",
    "kind": "entry",
    "disabled": false,
    "action": { "type": "open", "url": "https://example.com" }
  },
  { "id": "row-2", "title": "Plain information row" }
]
```

| 字段 | 类型 | 必填 | 语义 |
| --- | --- | --- | --- |
| `id` | string | 是 | 行身份。分页合并按它去重（先出现的胜出）。 |
| `title` | string | 是 | 行的主文案。 |
| `subtitle` | string | 否 | 副文案。省略或空串 → 行折叠成单行。 |
| `icon` | string | 否 | 字形名，**封闭词表**：`link` / `file` / `folder` / `globe` / `star` / `clock` / `text` / `image` / `command`。其它值 → 整份输出当文本。缺省用通用字形。 |
| `group` | string | 否 | 分组名。相邻的同一 `group` 行画成一块，组名只印一次。 |
| `kind` | `"status"` | 否 | 状态行：画成列表里的弱化说明，不占编号、不可回车。用于空态/错误态。 |
| `disabled` | boolean | 否 | 标记为不可执行（与 `kind: "status"` 一致；两者都表示「不是门」）。 |
| `action` | object | 否 | 回车执行的动作，见下。**没有 `action` 的行是信息**。 |

**`action`**（可选，决定行是否可交互）：

```json
{ "type": "open",   "url":  "https://example.com" }
{ "type": "copy",   "text": "text to put on the clipboard" }
{ "type": "insert", "text": "text to put back in the search field" }
```

| `type` | 框架执行 |
| --- | --- |
| `open` | 把 `url` 交给系统打开（`open_url`） |
| `copy` | 把 `text` 写进系统剪贴板（`clipboard_write_text`） |
| `insert` | 把 `text` 放回搜索框，不执行任何东西 |

未知的 `action.type`、或已知类型缺字段 → **整行不合法 → 整份输出当文本**（不做部分解析）。

### 3.3 交互层级（tier）与快捷键

- 列表有两个层级：`interactive`（行可选中、可回车、带 `⌘N` 角标）与 `display`
  （纯信息，无选中、无快捷键、无回车）。
- 框架的默认判定：
  - 通用行列表：**只要至少一行有 `action`** → `interactive`；否则 `display`。
  - 内置状态行列表：全部 `disabled` → `display`。
- 你可以在 `PluginEmission.tier` 里显式声明，框架会尊重它。
- **编号是纯按序分配的**：`⌘1`-`⌘9` 给视口内的前九个可运行行，`⌘0` 给第十个。
  没有任何保留位。滚动会按当前视口重新编号（R34）。

### 3.4 分页 / 滚动加载

`PluginEmission.page`：

```ts
{ cursor: string | null, hasMore: boolean }
```

- `cursor` 是**你的不透明续页令牌**。框架从不解释它，原样回传。
- `hasMore` 是框架唯一读的位：它画底部页脚，并在用户滚到底部时请求下一页。
- 不提供 `page` → 框架把整份输出当作一页，但**自己**在内存里按视口分页
  （每页 `MAX_RESULTS = 10` 行，首屏两页，滚到底部再加一页）。内置插件走的就是这条。
- 自己分页时：`hasMore: false` + `cursor: null` 表示结束；框架不会替换你的 `cursor`。

页脚状态（`pluginFooterState`）：`loading`（加载中）/ `more`（还有更多，滚动触发是
唯一的提示）/ `end`（没有更多）/ `null`（这份列表不分页）。

### 3.5 空态约定

- 空数组 `[]` 是「没有话说」：框架**什么都不画**（不会把 `[]` 当文本打印）。
- 想在空结果时给用户一句话 → 发一个**状态行**：

```json
[{ "id": "empty", "title": "No matches in the index", "kind": "status", "disabled": true }]
```

- 框架自己的空态/关闭态（例如「插件已关闭」）也用状态行，因此列表的渲染路径只有一条。

---

## 4 · 文本形态

不符合 §3 的输出（普通文本、单行、非法 JSON、部分符合的数组……）→ **文本形态**。

- 判定点只有一个：`resolvePluginView(emission)`。它先试列表协议，失败就退文本。
- 文本永远 `display` 层级：没有行可选中。
- 高度（`--u` 单位，`launcher/plugin-mode.ts`）：

| 常量 | 值 | 含义 |
| --- | --- | --- |
| `PLUGIN_TEXT_LINE_UNITS` | `24` | 一行的高度 |
| `PLUGIN_TEXT_MIN_UNITS` | `24 × 3 = 72` | **最小 3 行**：一行输出不能缩成一条缝 |
| `PLUGIN_TEXT_MAX_UNITS` | `10 × 42 = 420` | **上限 = 十行列表的高度**（`MAX_RESULTS × ROW_HEIGHT_TWO_LINE`）；约 17 行文本，再长就滚动 |

- 超过上限 → 文本块**内部滚动**，窗口不随输出变高（`scrolls: true`）。
- 文本块的行数折进窗口的离散 band 表（`pluginViewRows`），因此一个插件输出永远不会
  逐键改变窗口高度。
- 输出是 `null` / `undefined` / 空串 → 什么也不画。

---

## 5 · 配置弹层（schema 驱动）

插件模式的 gear 图标打开一个由**声明**驱动的配置弹层（`PluginConfigOverlay` +
`plugins/controls.tsx`）。你不提供像素、不接线事件，只提供一个有序字段表。

```ts
type PluginConfigSchema = {
  pluginId: string;
  titleKey: MessageKey;        // 弹层标题（宿主字典键）
  fields: readonly PluginConfigField[];
};
```

### 5.1 通用组件（R29 清单）

| `type` | 控件 | 额外字段 |
| --- | --- | --- |
| `toggle` | 开关 | — |
| `select` | 下拉 | `options` |
| `radio` | 单选 | `options` |
| `checkboxes` | 多选 | `options` |
| `slider` | 滑块 | `min`, `max`, `step?`, `unitKey?` |
| `number` | 数字输入 | `min`, `max`, `step?`, `unitKey?` |
| `text` | 文本框 | `placeholderKey?` |
| `action` | **按钮**（R38） | `confirmKey`, `cancelKey`, `failedKey`, `command` |

每个字段都可以带 `labelKey` 与 `helpKey`（都是宿主字典键，**不传文案**）。

### 5.2 `action` 的两步确认

`action` 不保存值（`normalizeConfigValue` 返回 `null`，不写回配置块）。它渲染成一个按钮：

1. 第一次按下 → 进入「已武装」状态，显示 `confirmKey` 文案与确认/取消；
2. 第二次按下确认 → 调用 `command`（一个宿主允许的命令），失败时用 `failedKey` 报错。

这是**弹层自己的两步确认**，不是系统对话框。R38 的「清空剪贴板历史」就是这么做的。

### 5.3 值

- 值对象由 `configValues(schema, raw)` 归一化；`configDefaults(schema)` 给缺省值。
- `applyConfigChange` 只改一个字段，其余原样保留。
- 值以插件自己的配置块持久化（例如 browser 的 `browser_plugin`）；`action` 字段永不
  回写。

参考实现：`CLIPBOARD_CONFIG_SCHEMA`、`browserConfigSchema`（`plugins/config-schema.ts`）。

---

## 6 · 内置插件实例（参考实现）

内置与外部走同一套契约；差别只在「触发器词从哪来」和「开关是谁的字段」。

| | clipboard | browser |
| --- | --- | --- |
| 模式类型 | `{ scope: "clipboard", filter }` | `{ scope: "browser", kind }` |
| 触发词 | `clip` / `clipboard` / `剪贴板` / `粘贴板` + 空格 | `browser` / `书签` / `history` … + 空格 |
| 开关字段 | `clipboard_history_enabled` | `browser_plugin.enabled` |
| 输出模块 | `plugins/clipboard/mode.ts` | `plugins/browser/mode.ts` |
| 数据来源 | `clipboard_get_entries` 一次取回，内存过滤 | `browser_search_*` 一次取回，内存过滤 |
| 空态 | `clipboardStatusRow(...)` | `browserStatusRow(...)` |
| 过滤器（chips） | R38 六片：全部/收藏/文字/图片/链接/文件 | R32 四种范围：all/bookmarks/history/tabs |

共同点，也是你可以照抄的部分：

- 输出模块只做「把领域数据变成 `PluginRow[]`」，**不碰 `LauncherItem`、不决定高度**；
- 一次取回、内存过滤（typing 不触发 IPC，不触发窗口 resize）；
- 空结果发状态行，不发空数组；
- 配置只写自己的字段。

外部插件的区别：

- 模式类型是 `{ scope: "external", extensionId, commandId }`；
- 触发器词来自 descriptor 的 `commands[].id` / `aliases`，且**受命令开关门禁**；
- 输入框内容是该命令的 **argv**（按引号规则切分），回车执行；
- 输出直接来自命令的 stdout（stderr 兜底）。

---

## 7 · 生命周期：进入、驻留、离开

- **进入**：输入 `命令名 `（词 + 空格）。词大小写不敏感；裸词不进入（它是普通查询）。
  内置触发器优先于外部命令触发器。
- **驻留（R35）**：进入插件模式会启动一个驻留时钟（`surface_residency_seconds`，
  默认 10 秒）。在时钟内，应用**不会自动**把界面退回搜索框（例如窗口失焦再唤起）。
  显式手势（Esc / ⌘W、回车运行、关闭按钮）永远不被时钟拦截。
- **离开（R31）**：
  - `Esc` / `⌘W`（`Ctrl+W`）离开模式，字段里的文本**保留**为普通查询；
  - 字段已空时再按一次退格离开（「退格删空即停」）；
  - 三级 dismiss：先关配置弹层 → 再退出插件模式 → 最后隐藏窗口。

---

## 8 · 你不需要做的事（反清单）

- 不要自己画行、按钮、图标（`icon` 走封闭词表）。
- 不要自己决定窗口高度（框架的离散 band 表）。
- 不要自己分配快捷键（`⌘1`-`⌘9`、`⌘0` 纯按序）。
- 不要自己实现分页 UI（给 `cursor`/`hasMore` 或什么都不给）。
- 不要在输出里塞宿主文案以外的控制序列；未知消息/未知字段一律被忽略或降级为文本。
- 不要期望 `[]` 显示成文字（它表示「无话可说」）。

---

## 9 · 快速自检

1. `description.json` 的 `commands[]` 能被集成页列出，每个命令有开关。
2. 打开开关后，搜索框输入 `命令名 ` 进入模式，scope 名显示 `name`。
3. 输入参数回车 → 命令以独立 argv 收到参数（不是一整条 shell 字符串）。
4. 命令打印 JSON 数组 → 列表（有 `action` 的行可回车、带 `⌘N`）。
5. 命令打印普通文本 → 文本块，最少 3 行；超过十行列表的高度后在块内滚动。
6. 命令打印空数组或什么都不打印 → 无输出提示（状态行），不是空白框。
7. 关闭开关 → 触发器词落回普通搜索，命令不可呼出。
