# 插件页通信协议（Plugin Page Protocol v1）

插件页（plugin page）是跑在 Floter 窗口里一个沙箱 iframe 中的一份 HTML 文档。
它拿不到 Floter 的 DOM，也拿不到任何 Tauri API：页面与宿主之间**只有一条
postMessage 通道**。这份文档就是那条通道的完整定义。

> 状态：**Protocol 1**，已实现。宿主与页面的版本常量分别是
> `src/plugin-pages.ts` 的 `PLUGIN_PAGE_PROTOCOL` 和本示例里的
> `PROTOCOL`。两边不一致时宿主拒绝进入交互，并显示带版本号的错误态。

## 0. 三十秒版本

1. 页面加载后**第一件事**：`postMessage` 一条握手消息
   `{ floter: "frame-ready", protocol: 1 }`。
2. 之后才能发 `invoke` / `host-notify` / `close` / `drag`。
3. 宿主回推 `result` / `opacity` / `theme` / `glass` / `visibility` / `reload`。
4. 每条消息都是一个普通对象，必带 `floter` 字段（下称「tag」）。
5. 页面认不出来的消息**直接忽略**；宿主对没通过握手的页面**不作任何回应**。

## 1. 最低运行要求

| 项目 | 要求 | 说明 |
| --- | --- | --- |
| Floter | 0.3.5-preview 或更高 | 本协议随该版本的插件页机制一起发布 |
| WebView | WebKit / WebView2 / WebKitGTK，支持 `postMessage`、`MessageEvent.source`、可选链 | 都是各平台系统 WebView 的基线能力，无需 polyfill |
| 页面侧 API | 原生 `window.parent.postMessage`、`window.addEventListener("message", …)` | 无构建步骤；纯静态 HTML/JS 即可 |
| iframe sandbox | 宿主决定 | 外部页面为 `allow-scripts`（不透明源）；内置受信页面额外 `allow-same-origin` |
| 网络 | 不要求 | 页面本身应由宿主同源提供；所有宿主能力都走 `invoke` 桥 |

因为 iframe 默认是**不透明源（opaque origin）**，页面里不能用
`localStorage`、`document.cookie`、`window.parent.document`，也读不到宿主的
CSS 变量。需要什么就从 `invoke` 或 bootstrap 查询参数里拿。

## 2. 消息总表

方向 `page → host` 表示页面发给宿主，`host → page` 反之。所有消息都带 tag。

| 方向 | 消息 | Payload | 何时发 |
| --- | --- | --- | --- |
| page → host | `frame-ready` | `{ protocol: 1 }` | 页面脚本执行后立刻，且只发一次；这是握手，必须早于任何其他消息 |
| page → host | `invoke` | `{ id: number, session?: string, command: string, args?: object \| null }` | 需要宿主执行一个白名单命令时；`id` 由页面生成，用于配对回复 |
| page → host | `close` | `{}` | 页面要求关闭自己、回到之前的界面时 |
| page → host | `drag` | `{}` | 用户在页面空白区按下鼠标、想把窗口拖走时；payload 必须为空 |
| page → host | `host-notify` | `{ id: number, kind: "error" \| "success", messageKey: string, retryable?: boolean }` | 页面要弹一条反馈提示时；`messageKey` 必须是宿主字典里的键 |
| host → page | `result` | `{ id: number, session?: string, ok: true, value: unknown }` 或 `{ id: number, session?: string, ok: false, error: string }` | 回复一条 `invoke`；`id`/`session` 原样回带 |
| host → page | `opacity` | `{ mainOpacity: number, terminalOpacity: number }` | 页面加载完成（握手通过）后推一次；之后每次透明度滑块变动都推 |
| host → page | `theme` | `{ theme: "dark" \| "light" }` | 握手通过后推一次；之后主题切换时推 |
| host → page | `glass` | `{ glassStep: "frosted" \| "regular" \| "liquid" }` | 握手通过后推一次；之后玻璃档位变化时推 |
| host → page | `visibility` | `{ visible: boolean }` | 页面被显示 / 隐藏时（含 `hidden → shown`），用来暂停或恢复轮询 |
| host → page | `reload` | `{}` | 页面重新可见、或同一个页面被再次打开时；页面应刷新数据 |
| host → page | `notify-retry` | `{ id: number }` | 用户点了某条提示的「重试」；`id` 是该提示自己的 id |

字段约定：

- tag 字段名固定为 `floter`，值就是上表「消息」列里的字符串。
- `invoke.id` 是页面自己发的自增整数；宿主只负责原样回带，不自己造 id。
- `session` 可省略。带上它时，宿主会在 `result` 里回带同一个值；页面重载后
  可以用它丢弃上一个文档的迟到回复（见 §7）。
- `host-notify.messageKey` 是**宿主字典的键**（如 `clipboard.actionFailed`），
  不是给用户看的文案。宿主翻译，键不存在就丢弃。
- 未在表中出现的 tag，两端都必须忽略，不得报错。

## 3. 握手时序（文字版）

```
页面文档开始加载
        │
        ├─ 宿主 onLoad 触发，启动 10s 握手超时
        │
  页面脚本执行
        │
        ├─ page  → host ：{ floter: "frame-ready", protocol: 1 }
        │
        ├─ 宿主比对 protocol：
        │     与本机版本相同  → accepted
        │     字段缺失/非数字 → missing
        │     数字不相同      → mismatch（错误态里写明 page=X host=1）
        │
        ├─ accepted：
        │     host → page ：opacity / theme / glass（初始值）
        │     host → page ：visibility { visible: true }（页面是当前活动页时）
        │     之后页面才可以发 invoke / host-notify / close / drag
        │
        └─ missing 或 mismatch：
              宿主显示错误态 + 「重试」按钮
              页面发来的 invoke / host-notify / drag 一律被丢弃，不回复
```

握手超时 10s 与页面侧 `invoke` 超时一致：两边在同一个时钟上放弃静默的对方。

**为什么缺失版本也算失败**：如果宿主把「没带 `protocol`」当成老页面来兼容，
那么一个写错字段名的页面会静默地以为自己握手成功，而宿主在另一套假设上
服务它。宁可显式拒绝，也不要猜。

## 4. 启动参数（bootstrap query params）

宿主在 iframe 的 `src` 上追加查询参数，让页面在**第一帧之前**就能拿到设置。
这与 §2 的推送消息是同一批值的两条通道：查询参数负责首屏，消息负责首屏之后的
变化（这样滑块移动不会重建 iframe、不会丢掉页面的筛选文本和滚动位置）。

| 参数 | 取值 | 含义 |
| --- | --- | --- |
| `lang` | `en` \| `zh` | 界面语言 |
| `theme` | `dark` \| `light` | 主题 |
| `main-opacity` | `0`–`1` | 主面板透明度 |
| `terminal-opacity` | `0`–`1` | 终端面板透明度 |
| `glass-step` | `frosted` \| `regular` \| `liquid` | 玻璃材料档位 |

页面必须**同时**读参数、听消息，并对缺失值有合理默认值（旧宿主可能一个都不发）：

```js
const params = new URLSearchParams(window.location.search);
const theme = params.get("theme") === "light" ? "light" : "dark";
const step = params.get("glass-step") ?? "regular"; // 默认 Regular
```

## 5. CSS 材料带注入（重要坑）

宿主把玻璃材料的 token 作为 CSS 自定义属性写在**宿主自己的容器**上
（`.plugin-page-host`）。沙箱 iframe 的文档**不会继承宿主的 `:root`**，所以
页面必须自己按收到的档位写自己的变量。宿主推给你的只有档位 id，数值表由
`glass-material.ts` 统一提供，不要在自己的 CSS 里抄数字。

用 `var()` **可以**放在 alpha 通道，放在颜色通道会被 WebKit 拒绝：

```css
/* ✅ 颜色通道是字面量，alpha 是变量：WebKit 接受 */
--field: rgba(17, 18, 20, var(--glass-content-alpha));
--row:   rgba(17, 18, 20, var(--glass-row-alpha));

/* ❌ 颜色通道也用了变量：WebKit 直接判无效，整条声明被丢弃 */
--page-bg: rgba(var(--page-rgb), var(--opacity));
```

需要「整条颜色都由变量算出来」时，用 `setProperty` 从 JS 写完整值，或者像
内置剪贴板页那样把 RGB 三元组拆开、只让 alpha 走变量：

```js
const root = document.documentElement.style;
root.setProperty("--glass-content-alpha", String(alpha));
root.setProperty("--glass-row-alpha", String(rowAlpha));
```

另外：**沙箱 iframe 的 `backdrop-filter` 模糊的是它自己文档的背景，不是宿主的
像素**，所以页面里加 `backdrop-filter` 只会浪费一个滤镜配额、什么也看不出来。
页面只需要画一层带 alpha 的填充，宿主那侧的玻璃由窗口本身提供。

## 6. 拖动桥接（isClipboardDragTarget 模式）

页面在沙箱里收不到宿主的 `mousedown`，`data-tauri-drag-region` 属性也只对
宿主文档里的元素有效。所以拖动是页面主动「报告意图」，不是宿主监听事件：

```js
// 只有落在非交互元素上的主键按下才算拖动意图。
const DRAG_EXEMPT = "button, input, textarea, select, a, [data-no-drag]";
const isDragTarget = (target) => !!target && !target.closest(DRAG_EXEMPT);

panel.addEventListener("mousedown", (event) => {
  if (event.button !== 0) return;               // 只认左键
  if (!isDragTarget(event.target)) return;      // 滚轮区/按钮/输入框豁免
  event.preventDefault();                        // 别让按下变成选中文本
  window.parent.postMessage({ floter: "drag" }, "*");
});
```

要给某个区域豁免（例如列表滚动条），在它的选择器上加 `data-no-drag`。
宿主收到 `drag` 后会走和标题栏完全相同的窗口拖动路径 —— 同一条平台分支、
同一套 Windows 失焦宽限逻辑 —— 所以页面不需要、也不应该自己去调任何窗口 API。

`drag` 消息是**空 payload**：坐标、目标窗口、尺寸都不许带，它们会被宿主丢弃。

## 7. 一个完整的客户端骨架

把这段放进页面即可跑（`my_command` 换成宿主白名单里的命令名，反馈文案用宿主
字典里真实存在的键）：

```html
<!doctype html>
<html lang="en">
  <head><meta charset="utf-8" /><title>My Plugin Page</title></head>
  <body>
    <div id="app">Loading…</div>
    <script type="module">
      const PROTOCOL = 1;                         // 与宿主 PLUGIN_PAGE_PROTOCOL 对齐
      const TAG = "floter";
      const session = crypto.randomUUID();        // 用来丢弃上一个文档的迟到回复
      const pending = new Map();                  // id -> {resolve, reject, timer}
      let nextId = 1;

      // ① 握手：第一件事，先于任何其他消息。
      window.parent.postMessage({ [TAG]: "frame-ready", protocol: PROTOCOL }, "*");

      // ② 调用宿主白名单命令。
      const invoke = (command, args = {}) => new Promise((resolve, reject) => {
        const id = nextId++;
        const timer = setTimeout(() => { pending.delete(id); reject("bridge timeout"); }, 10_000);
        pending.set(id, { resolve, reject, timer });
        window.parent.postMessage({ [TAG]: "invoke", id, session, command, args }, "*");
      });

      // ③ 发一条反馈提示（messageKey 由宿主翻译，页面不发文案）。
      const notify = (messageKey, { kind = "error", retryable = false } = {}) =>
        window.parent.postMessage(
          { [TAG]: "host-notify", id: nextId++, kind, messageKey, retryable }, "*");

      // ④ 收宿主的消息。
      window.addEventListener("message", (event) => {
        if (event.source !== window.parent) return;         // 只信宿主窗口
        const data = event.data;
        if (!data || typeof data !== "object" || data[TAG] === undefined) return;

        if (data[TAG] === "opacity") {
          // 透明度变了：重算自己的材料带，别重建 iframe。
          document.documentElement.style.setProperty(
            "--field-alpha", String(pageFieldAlpha(data.terminalOpacity)));
          return;
        }
        if (data[TAG] === "theme") {
          document.documentElement.dataset.theme = data.theme;
          return;
        }
        if (data[TAG] === "visibility") {
          data.visible ? startPolling() : stopPolling();
          return;
        }
        if (data[TAG] === "reload") { void reload(); return; }
        if (data[TAG] === "result" && data.session === session) {
          const call = pending.get(data.id);
          if (!call) return;
          pending.delete(data.id);
          clearTimeout(call.timer);
          data.ok ? call.resolve(data.value) : call.reject(data.error);
          return;
        }
        // 其余（含未知 tag）一律忽略。
      });

      // ⑤ 收尾：关闭自己。
      const closePage = () => window.parent.postMessage({ [TAG]: "close" }, "*");

      // 首屏读查询参数，之后靠消息；缺失值给默认。
      const params = new URLSearchParams(location.search);
      document.documentElement.dataset.theme = params.get("theme") ?? "dark";
      const pageFieldAlpha = (t) => 0.25 + 0.7 * Math.min(1, Math.max(0, Number(t) || 0));

      let polling = 0;
      const stopPolling = () => { clearInterval(polling); polling = 0; };
      const startPolling = () => { if (!polling) polling = setInterval(reload, 2_000); };

      async function reload() {
        try {
          const value = await invoke("my_command");
          document.getElementById("app").textContent = JSON.stringify(value);
        } catch {
          notify("plugin.loadFailed", { retryable: true }); // ← 换成宿主字典里的真实键
        }
      }
      void reload();
    </script>
  </body>
</html>
```

### 可运行的最小示例

`examples/hello-page/` 是一份无构建步骤的完整页面，演示握手、读透明度带、
注册拖动区、发一条 `host-notify`。见该目录的 `README.md`。

## 8. 版本策略

- 只有当消息**形态**发生老页面无法忽略的变化时才递增
  `PLUGIN_PAGE_PROTOCOL`（新增可选字段不算）。
- 宿主只接受**与自己相等**的版本号，不做范围协商。
- 版本不匹配时宿主显示错误态，文案里同时给出 `page=页面版本` 与
  `host=宿主版本`，让作者一眼看出该更新哪一侧。
- 页面没有握手、或握手超时（10s），等价于 `missing`：同样拒绝，错误态同理。

## 9. 相关实现

| 位置 | 内容 |
| --- | --- |
| `src/plugin-pages.ts` | 协议常量、消息类型、校验谓词、握手判定、URL 构造（纯逻辑，可被 node 测试直接驱动） |
| `src/plugins/PluginPageHost.tsx` | 宿主侧：iframe 挂载、消息路由、握手状态机、错误态 |
| `src/plugins/clipboard/main.ts` | 第一个消费者：内置剪贴板页的完整客户端用法 |
| `tests/plugin-page-protocol.test.ts` | 握手三态、消息表一致性、示例 payload 的结构测试 |
