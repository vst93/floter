# P13-jc：快捷键恢复、菜单键盘与长键可读性

起点 4e967fd8，pull 已同步、工作区干净。实际基线 5352 PASS / 0 FAIL，cmd/internal 3209 测试函数；全部测试/截图使用 zh_CN.UTF-8。普通英文大字号八种布局探针全部过界限检查，未把它们算成修复；截图暴露录制中的菜单键盘失效及长键全文不可达。

## Stash 清点

八条早期独立 WIP 与 P13-jb SHA 一致；reverse-check 均退出 1。backlog 已确认旧主题、旧数字字段或 Tauri 链不能覆盖当前实现，逐条保留；没有新增活动 stale-kill，没有静默删除。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | reverse=1，早期 keycap | 保留 |
| stash@{1} | 8af94a07 | reverse=1，早期间距 | 保留 |
| stash@{2} | 7e4aa710 | reverse=1，卸载指标/临时测试 | 保留 |
| stash@{3} | 1e3aec94 | reverse=1，旧数字字段 | 保留 |
| stash@{4} | 363f5a42 | reverse=1，旧插件字段 | 保留 |
| stash@{5} | 9b8ce23c | reverse=1，旧快捷键校验 | 保留 |
| stash@{6} | 90d314b2 | reverse=1，旧启动器动作栏 | 保留 |
| stash@{7} | 7a9f6e13 | reverse=1，旧 main/Tauri | 保留 |

done.log 开盘不存在。按本轮实测创建交接，不追认历史日志。

## 第一批：召回恢复

1. 刷新拒绝反馈：现象 → 请求 K 被系统拒绝、旧 J 可用却没有提示。改动 → 当前请求失败可见，设置恢复 J 后清旧状态。验证 → 初始探针 `refused settings refresh is invisible` / `working settings refresh retained old refusal` 实红。
2. 旧键重新确认：现象 → 录入界面上的 J，设置仍存被拒的 K。改动 → no-op 同时核对持久化值与实际注册，重新注册同键后不把新句柄注销。验证 → 两套外观真录键，文件写入恰 1 次、重新 OpenStore 读到 J，原键继续注册、焦点可重试。

第一批相关 115 PASS / 0 FAIL，shell vet 净。M1 回注开盘源码，两项深浅全部实红，原串含 `refused settings refresh is invisible`、`working settings refresh retained old refusal`、`recording fallback did not save the working key`。MD5 还原、touch 后 6 PASS。旧 V/B/S 0/0/0。首次新测试错误地用内存 Store 断言文件写入次数，改为真实临时文件 Store 后验证，不放宽断言；未把夹具错误计为产品缺陷。

第一批 **49d14733** 已推送。

## 第二批：保存行菜单的键盘出口

3. 保存行动作选择：现象 → 录制时打开菜单，Home/Down/Enter 被录制框吃掉，动作不变且菜单不关。改动 → 记录既有选择器打开状态，保存行的已开菜单在本帧取回焦点。验证 → 真实按键把动作改为浏览器，关闭后 Ctrl+Shift+Z 仍录到原行，动作选择只提交 1 次。
4. 保存行菜单 Escape：现象 → Escape 取消底下录制，菜单仍留着，后续组合键不能完成原行。改动 → 同一菜单拥有键盘，Escape 只关闭菜单。验证 → Escape 写入 0 次，录制继续且聚焦，下一组合键完成原行。

相关 355 PASS / 0 FAIL，settingsui vet 净。M2 回注旧 custom.go 实红，原串 `recorder left action menu open after keyboard exit`、`saved menu did not commit keyboard-selected action`、`menu exit canceled or lost the original recorder`、`menu return cannot finish the original binding`；两文件 MD5 还原，touch 后 6 PASS。旧 V/B/S 0/0/0。实现时漏一处闭括号的编译错误已修，不把编译错误计入 mutation。

第二批 **5e724345** 已推送。

## 第三批：草稿菜单的键盘出口

5. 草稿动作选择：现象 → Add 自动录制时，动作菜单 End/Enter 不能切到命令模式。改动 → 草稿的已开菜单同样取得焦点。验证 → 深浅真实 End/Enter 显示命令输入框，下一组合键仍落原草稿，输入 say draft 后可绑定。
6. 草稿菜单 Escape：现象 → Escape 取消草稿录制而留下菜单，下一键不能录入。改动 → 菜单优先接收 Escape，关闭后归还原捕获。验证 → Escape 写入 0 次，原草稿仍在录制，下一键及 Bind 完成同一条绑定。

相关 361 PASS / 0 FAIL，settingsui vet 净。M3 回注上一批 custom.go，原串 `draft menu could not select command mode`、`menu exit canceled or lost the original recorder`、`menu return did not record into the same draft` 实红；MD5 还原、touch 后 6 PASS。旧 V/B/S 0/0/0。

第三批 **83aa9b08** 已推送。

## 第四批：长键读全

7. 保存行长键：现象 → 大字号 720px 设置页显示 `Ctrl+Alt+S…`，没有全文入口。改动 → 原 recorder 接到现有 tokens.Tip。验证 → 深浅实际悬停 700ms，完整文本从 1 份变 2 份；移开恢复 1 份，录制钮矩形不变，随后可正常改键。
8. 草稿长键：现象 → 未绑定的长组合键同样截断。改动 → 草稿复用同一 recorder 提示。验证 → 真实 Add/录键/选命令/输入后悬停读全，读键写入 0 次，随后 Bind 恰提交 1 次且保留完整键。

相关 373 PASS / 0 FAIL，settingsui vet 净。M4 去掉新 tip 实红 `truncated shortcut has no full hover text: got 1 copies, want 2`；MD5 还原、touch 后 6 PASS。V 0 / B 0 / S 1：仅现有 tip 站点 census 从 10 → 11、settingsui.go 6 → 7，注释写明结构新增；不放宽像素/行为断言，不加 Skip。

第四批 **37332a45** 已推送。

## 收尾补齐同族跨行问题

9. 上方菜单与下方录制：现象 → 前两批菜单在本行拿回焦点后，被随后绘制的下方保存行/草稿录制器再次抢走，方向键仍无效。改动 → 视图局部记住已开菜单，整组绘制结束后再交给它键盘，不依赖行顺序。验证 → 两类下方录制者 × 深浅，点击上方菜单、Home/Down/Enter 成功改上方动作，下一组合键仍只改变下方原目标。

相关 378 PASS / 0 FAIL，包 vet 净。M5 回注第四批源码，四场景 `lower recorder stole upper menu keyboard selection` 实红；MD5 还原、touch 后同排/跨排共 17 PASS。新增 S/B/V 0/0/0。

完整普通测试首次退出 1：`fatal error: concurrent map read and map write`，栈顶 config.go:512 / TestTheConfigToolbarHoldsOneOperationAtATime，原始日志保留。已定位并修复，见第 10 项；未把首轮完整门槛写成通过。

## 完整门槛发现的两处故障

10. 配置工具栏回调崩溃：现象 → 后台 copy/export 回调 delete(configOperations)，绘制工具栏/保存按钮同时读 map，完整普通测试发生并发读写 fatal。改动 → 这一张忙态表用 RWMutex 统一保护，所有生产读入口走 configOperationNow；外部回调与 Invalidate 在锁外执行，禁用/保存时机不变。验证 → 原 TestTheConfigToolbarHoldsOneOperationAtATime 断言零改，`go test -race -count=10` 退出 0。
11. 浮窗重跑绕过现有动作：现象 → 首次 verbose 全量报 `a rerun did not mark the window as running`；测试注入的 RunCommandCaptured 未被使用，真实快速失败抢先结束在飞状态。改动 → rerunDetached 使用既有 Launcher.Actions.RunCommandCaptured；缺失回调时仍回落宿主实现。验证 → 原断言零改，定向重复 20 次与 Detached/Captured 相关回归通过；默认产品动作仍指向原宿主方法，不改会话或命令参数语义。

这两项是为关闭实测红灯增加的修复，不作为新系统选题。超过 40 次调用预算后已停止新选题，仅处理失败、复验和归档；这是本轮过程偏差，不声称预算合规。首轮普通/verbose 均失败，最终门槛必须在修正后重新执行，首次日志单独保留。

第 9、10 项 **d58567cd** 分批提交，第 11 项 **50065697** 单独提交，均推送到 mygo-rewrite。全轮共 11 项，跨 settingsui/shell；四批八项之后未把三项收尾修复攒成一次产品提交。

## 最终门槛

中文 locale、GOMAXPROCS=4，修正后普通及 verbose 完整测试均退出 0，**5381 PASS / 0 FAIL**（基线 5352，增加 29）；cmd/internal **3218 测试函数**（基线 3209，增加 9）。三平台 go vet ./...、go build ./... 全部退出 0；gofmt -l cmd internal 输出为空。最终六个产品源文件的 MD5 逐字节核对通过。

更新本轮报告与当前进度后，`go test -count=1 ./internal/ci` 再验通过（2.794s），`git diff --check` 为空。约 13:16 UTC 开盘，产品、完整门槛与文档契约在 14:03 UTC 完成，约 47 分钟；其后只做证据归档、推送与交接。六批产品提交全部已推送，结案提交可从本文件的最新 git log 查得，done.log 记录最终 SHA。

归档 staging 后，三个原始 verbose 日志中的命令帮助输出触发 Git 的 space-before-tab 提示。保留日志原字节；源码和文档仍做默认 whitespace 检查，仅这三个日志检查时允许原有空格后接 tab，不修改全局 Git 配置或产品测试。

- [最终普通全量](P13-jc/full-final-test.txt)、[最终 verbose 全量](P13-jc/full-final-verbose.txt)、[门槛摘要](P13-jc/gates-final.txt)、[最终源码 MD5](P13-jc/final-source.md5)。
- [首次普通崩溃](P13-jc/first-full-failure.txt)、[首次 verbose 失败](P13-jc/first-verbose-failure.txt)、[配置并发 race 复验](P13-jc/config-race.txt)、[浮窗运行态 20 次复验](P13-jc/detached-fixed.txt)。
- M1–M5 的 injected-red/restored 日志与各阶段 MD5 均在本轮目录；各次还原后 touch 复验为 **6/6/6/6/17 PASS**。阶段 MD5 对应当时源码，后续合法修正以 final-source.md5 为准。
- 旧断言台账 **V 0 / B 0 / S 1**，唯一结构变更是 tip 站点 census。新增 Skip **0**；没有删函数、删断言、放宽容差或将行为 pin 改成非空判断。

## 偷了什么

实际读取 qiaomu-design 的 SKILL、偏好账本、craft-loop、完整 preflight、chinese-typography，以及 Apple DESIGN 的颜色与控件段。用户指定已有方向优先于四方案 HTML、外部模型等泛化要求。

- craft-loop 工艺菜单 **#3 品牌 focus 环**：Apple 单一交互色和 2px focus 的角色映射到现有控件；菜单/录制器焦点交接 **0ms**，一帧结束后只由当前上层菜单持有键盘。
- **#4 签名交互**：高频 Home/Down/End/Enter/Escape 不加动画；菜单 Escape 写入 **0 次**，保存行动作选择 **1 次**；继续使用现有交互模型。
- **#6 编辑细节**：截断组合键保留原等宽标签与省略号；**600ms** 悬停门槛后读全文，测试推进 **700ms**，移开消失；矩形变化 **0px**，悬停保存 **0 次**。
- **#12 材质细节**：原 E2 tip 的 **8px** 圆角、**1px** 受光边与 **2 层**投影直接复用，只有窗口层透明度参与；不加玻璃嵌套和渐变白边。Apple 8px 控件圆角是角色供体，最终数值以本仓 token 为准。

为什么更轻、更有空间感：键盘优先级现在与画面中的菜单层级一致，退出菜单后回到原行；长键全文只在悬停时借已有浮层出现，控件宽度与常驻文案密度不增加。恢复成功时旧拒绝正确消退。未为凑工艺数量加装饰。

## 两轮目视与图像

第一轮四视角：艺术总监看到上层菜单还开着、下层录制框却消失；工程师发现强制 focus 按绘制顺序覆盖；用户按 End/Enter 无法切换到命令输入；读者只读到 Ctrl+Alt+S… 无法确认完整键。第二轮逐一复核状态、焦点归属、可操作出口和全文可读性，额外检查跨行顺序并实修第 9 项。

浅色召回图：失败后原 Ctrl+Alt+J 仍在，新增的是既有拒绝条与原行脚注；深色重新录入图：旧拒绝退场，原键仍在。深色保存行 Escape 图：残留菜单消失，Press keys… 返回原行。浅色草稿选择图：菜单收起、命令输入框出现。长键图：原省略键帽尺寸不变，浮层可读 `Ctrl+Alt+Shift+Super+PageDown`；4× 最近邻局部用于检查受光边与间距，不冒充高 DPI 真机。

| 验收项 | 浅色对照 | 深色对照 |
| --- | --- | --- |
| 1 刷新拒绝 | [图](P13-jc/fallback-refused-light-comparison.png) | [图](P13-jc/fallback-refused-dark-comparison.png) |
| 2 重新录旧键 | [图](P13-jc/fallback-recorded-light-comparison.png) | [图](P13-jc/fallback-recorded-dark-comparison.png) |
| 3 保存行选择 | [图](P13-jc/saved-menu-select-light-comparison.png) | [图](P13-jc/saved-menu-select-dark-comparison.png) |
| 4 保存行 Escape | [图](P13-jc/saved-menu-escape-light-comparison.png) | [图](P13-jc/saved-menu-escape-dark-comparison.png) |
| 5 草稿选择 | [图](P13-jc/draft-menu-select-light-comparison.png) | [图](P13-jc/draft-menu-select-dark-comparison.png) |
| 6 草稿 Escape | [图](P13-jc/draft-menu-escape-light-comparison.png) | [图](P13-jc/draft-menu-escape-dark-comparison.png) |
| 7 保存行全文 | [图](P13-jc/saved-key-tip-light-comparison.png) | [图](P13-jc/saved-key-tip-dark-comparison.png) |
| 8 草稿全文 | [图](P13-jc/draft-key-tip-light-comparison.png) | [图](P13-jc/draft-key-tip-dark-comparison.png) |
| 9 上方菜单/下方保存录制 | [图](P13-jc/upper-menu-saved-recorder-light-comparison.png) | [图](P13-jc/upper-menu-saved-recorder-dark-comparison.png) |
| 9 上方菜单/下方草稿录制 | [图](P13-jc/upper-menu-draft-recorder-light-comparison.png) | [图](P13-jc/upper-menu-draft-recorder-dark-comparison.png) |

原帧为 Linux ui.Tester 的 960×1100、720×440、720×1100。像素差见 [image-differences](P13-jc/image-differences.txt)，代表图 `/tmp/floter-P13-jc-shortcuts-{dark,light}.png`。第 10/11 项是崩溃/执行接线修复，以运行日志和原断言验证，不伪造崩溃前后视觉差。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，主题 token 不改 |
| A2 非系统字体 | 不采用；中文系统字体规则与仓库契约优先 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用原生设置/浮窗 |
| A5 三等分/卡片海 | 无新增包装 |
| A6 假数据 | say original/draft 为事件夹具，产品无虚构指标 |
| A7 空泛文案 | i18n 文案零改 |
| A8 编号眉批 | 产品无新增 |
| A9 标题行数 | 截图页头完整可见 |
| A10 Bento | 不适用 |
| A11 左侧装饰线 | 无新增 |
| A2 工艺密度五项 | 未凑五项；按 craft-loop 优先级与用户修复要求，四项具体做法优先于装饰数量 |
| A4 功能契约 | 录制/选动作/取消/读键/绑定和失败恢复已验；无新入口和保存时机 |
| B1 读取/拨盘 | 3/2/7：保守、即时、原密度 |
| B2 主题锁定 | 每个 Tester 固定深或浅 |
| B3 记忆点 | 上层菜单拿键盘、完整键按需出现 |
| B4 动效气质 | 高频键盘零新增动画，与既有控件一致 |
| C1 目的/频率 | 新增焦点交接 0ms，不阻塞输入 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原 120ms 颜色反馈沿用；tip 600ms 是触发延迟，不是动画时长 |
| C4 origin | tip 锚在原录制钮上方；菜单沿用原触发器 |
| C5 reduced-motion | 所有新增 UI 事件用 ReduceMotion=true 验证 |
| C6 hover 查询 | Go 自绘原指针路由，不适用 CSS media |
| C7 布局动画 | 无新增 width/height 或边距动画 |
| C8 keyframes | 无新增重播 |
| C9 手势 | 不涉及拖拽/swipe/sheet |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新动效，不声称原生慢放验证 |
| D2-1 字体 | 沿用系统字体，无 webfont |
| D2-2 字号/行盒 | 旧 Caption 9.9/正文 10.8 不满足通用 ≥14px/1.5；仍需拍板，不改 |
| D2-3 空格 | i18n 零改；完整键保留规范组合键拼写 |
| D2-4 标点/对比 | 原标点与墨不改；不宣称旧次要字全部过 7:1 |

可复现规则落本仓 §6.230，不改全局 skill 偏好。原生热键占用由注入注册器模拟；Linux 自绘有实际事件与 PNG，Windows/macOS 仅编译检查，未宣称原生桌面验收。

## 仍待

1. 下一轮从配置复制/导出的完成与退出边界开始，检查目标重建后旧回调是否影响新操作；仅代码线索，先实测。不重复本轮菜单、长键或常规恢复。
2. 沿用需拍板：多草稿/转换覆盖、另一端冲突历史、字号/行盒、accent 禁用填充、未知多选/Radio、极小步长、176-9/161-4 保存期间草稿、终端 S1b/S1c/S2 尾。确认窗口与保存时机不改。
3. 八条旧 WIP 仍保留；原生桌面热键抢占与其他平台运行时未验。不会用本轮软件截图代替真机结论。
