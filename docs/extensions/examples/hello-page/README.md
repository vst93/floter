# hello-page · 最小可跑插件页

这是 [插件页通信协议](../plugin-page-protocol.md) 的一份完整、无构建步骤的示例。
它做的事只有四件，也正是每个插件页都必须做的事：

1. **握手** —— 加载后立刻发 `{ floter: "frame-ready", protocol: 1 }`
2. **读透明度带** —— 首屏读 `terminal-opacity` 查询参数，之后听 `opacity` 消息
3. **注册拖动区** —— 空白处按下 → 发 `{ floter: "drag" }`
4. **发一条消息** —— `host-notify` 弹宿主提示、`close` 关闭自己

## 文件

| 文件 | 作用 |
| --- | --- |
| `index.html` | 页面结构与交互元素（含 `data-no-drag` 豁免示例） |
| `page.css` | 材料带写法：颜色通道用字面量、alpha 用变量（WebKit 的坑） |
| `protocol.js` | 消息名与构造函数的唯一来源；页面的「协议词表」 |
| `main.js` | 页面的全部逻辑（握手 / 收消息 / 拖动 / 提示） |
| `mock-host.html` | 一个浏览器里就能跑的假宿主，用来单独验证页面 |

## 方式一：在浏览器里直接跑（推荐先跑这个）

不需要 Floter，不需要构建：

```bash
cd docs/extensions/examples/hello-page
python3 -m http.server 8080
# 浏览器打开 http://localhost:8080/mock-host.html
```

`mock-host.html` 实现了协议 1 的宿主侧：握手比对、bootstrap 参数、`opacity` /
`theme` / `glass` / `visibility` / `reload` 推送，以及把页面发来的消息打印出来。
左侧面板可以实时改透明度和主题，能看到页面**不重建 iframe** 就地跟随。

想验证版本校验，把 `protocol.js` 里的 `PROTOCOL` 改成 `2` 再刷新：假宿主会像真宿主
一样拒绝握手，并把 `page=2 host=1` 显示出来。

## 方式二：加载进真实的 Floter

真实的宿主比假宿主多了两件事：页面必须由**宿主同源**提供，且要登记在宿主注册表里。
当前版本（0.3.5-preview）的外部插件页注册尚未开放，所以要用开发流程：

1. 把本目录复制到仓库的 `plugins/hello-page/`：

   ```bash
   mkdir -p plugins/hello-page
   cp index.html page.css protocol.js main.js plugins/hello-page/
   ```

   `protocol.js` / `main.js` 里的 `import` 相对路径不变，复制后仍然成立。

2. 让 Vite 把它当成一个独立入口，在 `vite.config.ts` 的
   `build.rollupOptions.input` 里加一行（每个插件页都是一份独立 HTML 文档）：

   ```ts
   input: {
     main: "index.html",
     "plugins/clipboard": "plugins/clipboard/index.html",
     "plugins/hello-page": "plugins/hello-page/index.html",
   },
   ```

3. 在宿主注册表 `src-tauri/src/plugin_pages.rs` 的 `DESCRIPTORS` 里登记它。
   `page` 必须是 `plugins/` 下的相对路径（后端有测试守着这一点），
   `allowed_commands` 是这份页面能通过 `invoke` 调用的命令白名单：

   ```rust
   static DESCRIPTORS: &[PluginPageDescriptor] = &[
       PluginPageDescriptor { /* builtin.clipboard … */ },
       PluginPageDescriptor {
           id: "example.hello",
           title_key: "settings.clipboardHistory", // 用宿主字典里已有的键
           description_key: "settings.clipboardHistoryHint",
           page: "plugins/hello-page/index.html",
           allowed_commands: &[],                  // 本示例不调宿主命令
       },
   ];
   ```

   本示例只发 `host-notify`，它的文案键 `plugin.exampleNotify` 已经在
   `src/i18n.ts` 里（中英各一条）—— 页面只能引用宿主字典里真实存在的键，
   不存在的键会被宿主静默丢弃。

4. `npm run tauri dev`，然后用启动器 / 快捷键 / `floter` CLI 打开这个页面。
   窗口里看到的 `plugin-page-host__topbar`（标题 + 关闭）是**宿主**画的，
   不是页面画的。

## 对照阅读

- 完整协议：`../plugin-page-protocol.md`
- 协议实现（纯逻辑，node 可直接驱动）：`src/plugin-pages.ts`
- 宿主侧：`src/plugins/PluginPageHost.tsx`
- 第一个真实消费者（比本示例多得多）：`src/plugins/clipboard/main.ts`
- 协议测试：`tests/plugin-page-protocol.test.ts`

## 常见错误

| 症状 | 原因 |
| --- | --- |
| 错误态 `Plugin failed to load` + 版本号 | `PROTOCOL` 与宿主 `PLUGIN_PAGE_PROTOCOL` 不一致，或忘了发 `frame-ready` |
| 页面一直显示 Loading | 握手消息没发出去（检查是否在第一行执行前抛错），或加载超时 10s |
| 提示不出现 | `host-notify` 的 `messageKey` 不在宿主字典里，被丢弃 |
| 颜色/透明度不生效 | `rgba()` 的颜色通道里用了 `var()`，WebKit 丢弃了整条声明，见 `page.css` 注释 |
| 拖动没反应 | 按下点落在 `DRAG_EXEMPT` 匹配的元素里；或没有 `preventDefault` |
| 模糊没效果 | 沙箱 iframe 的 `backdrop-filter` 模糊的是自己的文档，不是宿主像素——不要用 |
