# P13-ht：集成动作退出与刷新后的恢复

开盘 `a1674426`，pull 最新，工作树干净。基线实测 **4007 PASS / 0 FAIL / 2870 测试函数**。done.log 开盘不存在。

## Stash 清点

活动八条 SHA 与 P13-hs 完全相同，逐条 reverse-check=1，属于早期独立 WIP；抢救 stale-kill 已有归档/接续记录，不重复试合，不覆盖当前 Go/主题实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实施记录

候选与真实行号见规格；下面随批次记录实测，未完成项不冒充修复。

### 第一批：参数随清单同步

1. 现象：表单已开后新增格式/数量参数，默认 `json`/`12` 没有显示。改动：按当前参数定义补齐缺少的默认答案，保留存量手改值。验证：深浅真实帧，旧实现报 `new parameters did not receive defaults without replacing edits: map[limit: target:my edit]`；修复后 target 仍为 my edit，format=json、limit=12。
2. 现象：删除参数后运行仍携带旧 key，重新加入也复活旧答案。改动：按当前完整 ID 集清除退役答案（含无默认值参数，不能用 defaults 的 key 集代替 schema）。验证：真实点击 Run 只派发当前 target，重加采用 fresh；旧实现报 `removed parameter reached command: map[obsolete:old target:current]` 与 `re-added parameter resurrected old answer: "old"`。

V/B/S 0/0/0，旧测试零改。M1 用 HEAD 产品回注两缺陷，红/绿原始日志与 MD5 校验落轮次目录；深浅新增参数前后图已导出。

第一批 `364ef2b8` 已推送；run 相关回归 69 PASS，M1 缺陷注入退出 1、恢复后 6 PASS，settingsui vet 净。

### 第二批：编辑与已提交答案分开

3. 现象：文字参数输入 24 后改成数字，Cmd/Ctrl+Z 将答案撤销回旧的 8。改动：参数编辑器身份包含 kind，改型时退役旧编辑历史，值仍保留 24。验证：两主题真实键盘事件；旧实现 `new number editor resurrected text undo: "8"`，新实现撤销不再引入旧答案；原参数重排保焦回归仍通过。
4. 现象：运行回调拿到表单 map，后续输入 second 把已提交 first 改掉；回调修改 map 又污染当前输入。改动：提交时拷贝字符串答案 map。验证：真实 Run 后继续编辑，命令快照仍 first、表单仍 second，派发恰好 1 次；原红串 `later edit rewrote submitted command: map[target:second]` / `command map mutated live form: "host-owned"`。

V/B/S 0/0/0。M2 回退本批产品变动，实际红；恢复后 MD5 校验、touch 复跑，原始日志随本批入库。

第二批 `dfa8ef77` 已推送；相关回归 73 PASS，M2 恢复后 4 PASS。

### 第三批：配置折叠确认的键盘出口

5. 现象：取消确认后焦点落空，下一次 Space 无法再打开确认。改动：确认退场当帧将焦点交回同一配置入口。验证：深浅真实取消后 Configure 有焦点，Space 再次打开确认，edited 草稿保留、保存次数 0。原红串 `config-cancel lost focus instead of returning to Configure`。
6. 现象：每帧强制聚焦取消，Tab 到不了丢弃；即便鼠标点丢弃，折叠后入口也无焦点。改动：取消只在确认初次出现时 AutoFocus，丢弃后沿同一出口归还入口。验证：Tab→Space 丢弃、Space 重开得到 stored，保存次数 0；原红串 `Tab could not reach Discard`。不是新增键盘模型，而是让既有可聚焦按钮可达。

M3 HEAD 回注两缺陷实跑红，MD5 恢复与 touch 后配置族回归。V/B/S 0/0/0。

候选修正：卸载“取消后重开旧错”探针绕过真实入口；真实 removalConfirmation:6957 已清错误，排除该候选、不改产品、不计修复。第 7 项改查真实 runForm 取消焦点（已复现），第 8 项继续卸载失败重试。

第三批 `546de9d0` 已推送；配置族 42 PASS，settingsui vet 净。

### 第四批：运行退出与卸载重试

7. 现象：参数表单取消后没有键盘落点，Space 不能重新展开。改动：原动作行绘制时，若同帧由表单取消而收起，将焦点交回 Run 入口。验证：真实集成页取消派发 0 次，随后 Space 重开，edited 参数仍在；原红串 `cancelled run form lost disclosure focus` / `keyboard reopen lost parameter draft`。
8. 现象：卸载局部 Handle 每帧新建，FocusWithin 永远读不到已提交身份，Tab 八次仍困在取消；失败插入错误带还会改变动作行 ID。改动：查询当前 panel Element 的 FocusWithin，保留延迟 focus 请求；给动作行稳定 key。验证：从真实 opener 打开仍先聚焦 Cancel，Tab 可达卸载，失败句插入后焦点仍在卸载，同步与延迟失败均可 Space 重试；错误原文保留、两次确认恰好派发两次。原红串 `Tab did not reach uninstall`，逐帧探针八次 `cancel=true`。

V/B/S 0/0/0，旧行为断言零改。八项跨运行参数、配置确认、卸载对话框三个组件；未新增页面/系统/依赖。

运行取消还需固定 `integration.actions` 行身份：仅同帧 Focus 会在下一帧因上方表单消失而失效，真实键盘 pin 抓到后补齐。M4 首次还原校验发现卸载注释从 Binding 改成 Querying 没进入备份；按 MD5 恢复该注释后再次逐字节核对和 touch 复跑，不隐瞒中间失败。

第四批 `e5bea624` 已推送，相关回归 **100 PASS / 0 FAIL**，M4 还原、MD5 通过与 touch 后 **9 PASS**，settingsui vet 净。四批各两项，完整门槛在固定产品源码后集中执行，未声称每批都跑全部平台。

## 偷了什么

读取 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography。已有唯一方向，按用户要求直接打磨，不生成四方向网页。Apple DESIGN 的交互单蓝、2px focus、8px 常规圆角只取语义，落到本仓既有 accent/框架焦点环/RadiusSM=8；没有复制其模糊、负中文字距或整页纯黑。

- craft-loop 菜单 **第 3 项「品牌 focus 环」**：可操作焦点始终 1 个；配置取消回到 Configure，卸载失败留在 Uninstall，额外交接等待 **0ms**。
- **第 4 项「签名交互」**：取消运行派发 **0 次**，一次提交保存 **1 份**独立答案，卸载两次明确按键派发 **2 次**；不加动画遮掩状态断裂。
- **第 12 项「材质细节」**：新增材质层 **0**、新增容器 **0**，原 1px 边与 E0/E1/E2 保留；确认退场不再留下无法继续的焦点。

没有为凑五项密度加装饰；临时主线与“先修已存在的功能”高于通用菜单。可复现规则落本仓 §6.195 与事件 pin，不改全局 skill 账本。

## 两轮看图与预检

第一轮旧实现截图与事件定位了空默认值、旧撤销值、丢焦点与 Tab 困住。第二轮发现只恢复当帧焦点不足，参数表单收起会改变动作行身份，补稳定 key 后重新截图与键盘验收。

四视角：艺术总监看是否因修复增加层/影（新增 0）；工程师看默认格式与真实答案是否一致、动作身份是否稳定；任务使用者看取消后 Space 与失败后 Space 能否继续；读者看错误原文是否保留。仍有一个独立视觉线索：卸载失败带在右边缘有裁切迹象，留给下一轮先量，未擅改全族 noticeBand 几何。

这张图上看到的变化：新增参数浅色图中选中项从 text 回到 json，数量从空槽变成 12；配置深色图的 Configure 重新有完整蓝环；卸载深色图的蓝环从 Cancel 移到 Uninstall，失败句留在原面上。运行取消使用指针事件，前后静态墨可以相同，键盘落点与后续 Space 由事件断言证明，不用相同图片冒充视觉变化。

12 张左右对照图与 24 张原始 PNG 已入库，源文件在 `/tmp/floter-P13-ht-*`。代表图：[新增参数浅色](P13-ht/new-parameter-light-comparison.png)、[深色](P13-ht/new-parameter-dark-comparison.png)；[配置取消浅色](P13-ht/config-cancel-light-comparison.png)、[深色](P13-ht/config-cancel-dark-comparison.png)；[卸载重试浅色](P13-ht/uninstall-retry-light-comparison.png)、[深色](P13-ht/uninstall-retry-dark-comparison.png)。卸载旧图在 Tab 不可达时停止，新图能运行至失败态，这是同一事件序列的不同可达结果。

| 检查 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿用原生近黑背景 |
| A2 非系统字体 | 不适用；用户既有字体与中文 D2 优先 |
| A3 斜体 | 无新增 |
| A4 居中 Hero | 不适用，桌面表单 |
| A5 卡片海/三等分 | 无新增卡片或布局容器 |
| A6 假数据 | 无产品假指标；夹具为参数与错误的事件输入 |
| A7 空词/感叹号 | 产品文案零改 |
| A8 编号眉批 | 无新增 |
| A9 H1 行数 | 现有页头完整，未改 |
| A10 Bento 空洞 | 不适用 |
| A11 侧边竖线强调 | 无新增 |
| B1 读取/拨盘 | 克制、即时、保持原信息密度 |
| B2 主题 | 深浅分别渲染，无混搭 |
| B3 记忆点 | 当前答案与当前动作始终对应 |
| B4 动效一致 | 焦点/答案交接额外 0ms |
| C1 目的/频率 | 高频键盘没有新增动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原 120ms 颜色与 0.97 按压沿用 |
| C4 浮层来源 | 原 modal 居中，未新增浮层 |
| C5 reduced-motion | 图与事件使用 ReduceMotion=true，原降级保留 |
| C6 hover 媒体查询 | 自绘无 CSS 媒体查询；沿用指针输入路径 |
| C7 布局动画 | 无新增；原卸载 Position 过渡未改 |
| C8 重播 keyframes | 无 |
| C9 手势/拖拽 | 未涉及 |
| C10 Framer Motion | 不适用，Go 自绘 |
| C11 不确定动效 | 本轮固定 ReduceMotion 图验状态，不声称慢放或三平台真机证据 |
| D2-1 中文字体 | 沿用系统中文字体，不加 webfont |
| D2-2 字号/行高/字重 | 未通过通用 ≥14px 要求；仓内 Caption 9.9/Body 10.8 与旧行高是既有需拍板项，保留不改 |
| D2-3 混排空格 | i18n 文案零改，不新增混排问题 |
| D2-4 标点/暗色对比 | 文案/token 零改；不声称全部旧文字达到 7:1，延续字号/对比度拍板账 |

## 仍待

下一轮先量卸载失败带的右沿及其按钮焦点环裁切（本轮截图可见线索），连同已有 noticeBand 的错误/确认边界同族检查。运行参数全部移除再恢复时的会话记忆生命周期可作为后续真实事件线索，未复现不计 bug。字号/行盒、保存期间草稿归属和终端 S1b/S1c/S2 尾仍沿用需拍板。macOS/Windows 只做编译与 vet，不宣称真机交互。

## 门槛（最终）

完整普通与 verbose 全量均退出 **0**；最终 **4032 PASS / 0 FAIL**（基线 4007），测试函数 **2879**（基线 2870）。三平台 vet/build 均退出 0；gofmt 列表空，文档合约检查退出 0。V/B/S 旧断言修改 **0/0/0**，无新增 Skip。

产品代码固定于第四批。收尾看图发现复用的 runStateFrame 用 auto 主题，深色画布下部分墨仍按浅色计算；本轮新增夹具改为显式 light/dark（旧测试不动）。补图使用 Go `-overlay` 编译开盘产品，旧缺陷三场景六子场景仍红，当前产品文件 MD5 未变；修正夹具后的全部新增事件 **25 PASS / 0 FAIL**，三平台 settingsui vet 净。完整全量计数来自夹具修正前，产品源码与最终一致；没有冒充修改测试后重跑了整轮门槛。

证据：[普通全量](P13-ht/full-test.txt)、[最终定向](P13-ht/final-targeted.txt)、[门槛摘要](P13-ht/gates.txt)、[源码 MD5](P13-ht/final-source.md5)。verbose 完整日志在 `/tmp/floter-P13-ht/full-verbose.txt`，基线在 `/tmp/floter-P13-ht/baseline.log`。

过程偏差：13:18 UTC 开盘，13:50 UTC 完成完整门槛与图复核，约 32 分钟，低于 45–75 分钟目标；工具调用超过 40 次预算，主要用于真实事件定位与验收，未继续扩大产品选题。八项已按每两项分批推送；没有为凑时长加入无关改动。M4 中间 MD5 注释差异与错误主题图均如实记录并修正。

4× 局部复核：[Configure 焦点环](P13-ht/focus-detail.png)，蓝环完整且未压到按钮文字。其余深浅对照已逐组复核；未把指针路径的无像素差当成失败。
