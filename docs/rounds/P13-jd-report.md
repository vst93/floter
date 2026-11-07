# P13-jd：传输反馈与结果来源

起点 1a5d808e。开盘工作区干净、pull 已同步；实际基线 5381 PASS / 0 FAIL，3218 测试函数。done.log 开盘不存在。

## Stash 清点

下列八条与 P13-jc 和抢救 backlog 的早期 WIP 完全同 SHA；反向检查均退出 1。不是新增 stale-kill；历史语义核对为已替代的旧主题/字段/Tauri 工作，逐条保留，不强行覆盖当前产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse exit 1 | 保留 |
| stash@{1} | 8af94a07 | 旧间距，reverse exit 1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载指标/临时测试，reverse exit 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse exit 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse exit 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验，reverse exit 1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse exit 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse exit 1 | 保留 |

正常宿主每次只调用一次完成，未复现旧回调误清新锁；不为此引入代数或改关闭表单语义。实际选择传输结果与错误出口，详见规格。

## 第一批

1. 配置复制完成：现象 → 已报成功但剪贴板未写入，且绕过已有写入接口；旧代码探针报 `configuration copy bypassed the configured clipboard backend`、`configuration copy announced success before clipboard delivery`。改动 → 走原 writeClipboard，完成后才发成功与 done。验证 → 阻塞注入后端期间无成功、无完成；释放后成功，文本含 REDACTED、不含测试密钥。
2. 配置复制校验：现象 → 缺必填 endpoint 的草稿被误报为“不是集成导出文件”。改动 → toast 显示原校验器错误，不改字典。验证 → 深浅两帧均可读校验原因，失败不写剪贴板，回调错误等于校验器原错误。

第一批首次相关实际 5 PASS / 1 FAIL，shell vet 退出 0。原复制测试直连系统剪贴板，无桌面事件循环，完成移到写入后旧固定等待不再适用；给共享 provider 夹具接上已有 WriteClipboard 注入，所有旧行为断言保留。提交脚本未向 commit 传播测试退出码，02e87cbf 被提前提交推送，提交消息的“6 PASS”是错误记录；此处明确纠正，不冒充首次绿。后续所有提交使用 set -e 门禁。其余探针保留为非 Test helper，随各批启用，未将未修项算完成。

首批夹具修正后实际 6 PASS / 0 FAIL、shell vet 净，单独提交推送。

## 第二批

3. 配置导出校验：现象 → 缺必填 endpoint 的草稿误报导入文件格式。改动 → 导出校验失败展示实际字段错误。验证 → 两套外观中字段原因可读，回调携带原错误，文件选择器调用 0 次。
4. 配置导出选择器：现象 → 系统选择器不可用只留日志，用户看不到结果。改动 → 原错误进入宿主 toast；空路径且无错误仍为取消。验证 → 错误深浅可见、done 原样返回错误；再次执行空路径取消通知 0 条。

第二批累计相关 13 PASS / 0 FAIL，shell vet 净；旧断言 V/B/S 0/0/0，i18n 字典零改。

## 第三批

5. 集成导出选择器：现象 → 对话框系统错误被当成取消，无反馈。改动 → err 非空显示原错误；正常空路径单独返回。验证 → 深浅实帧可读系统原因，忙态清空，下一次取消不报错；原单任务 guard 和文件输出回归通过。
6. 集成导入选择器：现象 → 入口失败后直接消失，用户不知道为何没有导入。改动 → 同样区分错误和取消，仍走唯一宿主 toast。验证 → 深浅可见错误，忙态释放后可再次打开，正常取消 0 条新通知，导入原回归通过。

第三批累计相关 21 PASS / 0 FAIL，shell vet 净；旧 V/B/S 0/0/0。

## 第四批

7. 配置传输结果归属：现象 → 配置复制/导出成功与错误共用 toast ID，覆盖另一工具未处理的失败。改动 → 成功复用既有 transferNoticeID，同类成功仍替换。验证 → copy/export × 深浅四个设置页实帧同时显示成功与另一工具错误，未增加通知系统。
8. 导入来源可读：现象 → ImportReport.Path 已有真实来源，却无法从结果块读到。改动 → 报告块接原 tokens.Tip，仅有来源时出现。验证 → 成功/失败 × 深浅真实悬停 700ms 可读完整路径；移开消失，原报告矩形 0px 变化，清报告后内容与提示均撤回。

第四批首次 census 因 12 对旧 11 实红，按 S 明确重定到 12、transfer.go 新增 1 站点。另一个新夹具最初停在启动器，其合约只画最新一条 toast；改用真实设置表面后两条可见，不改产品启动器契约。修正后累计相关 39 PASS / 0 FAIL，两包 vet 净。旧断言 V/B/S 0/0/1；B 断言全部保留。总计八项，跨 shell 配置/集成传输和 settingsui 报告组件。

## Mutation（全部实跑）

M1 回注开盘 sync.go，校验与三个选择器故障共五项深浅实红：`configuration validation hid its actionable cause`、`config picker failure is invisible`、`export picker failure is invisible`、`import picker failure is invisible`。MD5 逐字节还原、touch 后 15 PASS。

M2 注回通用 success ID 与无来源提示的 transfer.go，实红：`configuration success replaced the unrelated refusal`、`import report hides its source path on hover`。两文件 MD5 还原、touch 后 10 PASS。

M3 单独把成功通知提前到剪贴板写入前，实红：`configuration copy announced success before clipboard delivery`。MD5 还原、touch 后 1 PASS。全部 -count=1，原始红/绿日志保存；没有靠缓存或放宽断言消红。

## 偷了什么

实际读取 qiaomu-design/SKILL.md、user-preferences、craft-loop、preflight、chinese-typography，并选 Apple DESIGN 的色彩与控件段。用户给定方向，直接走打磨路径。

- craft-loop 工艺菜单 **#4 签名交互**：状态反馈新增动画 **0ms**；写入未返回时成功通知 **0 条**，取消通知 **0 条**。Apple 的单一交互色角色沿用原 accent，错误/成功沿用原语义墨。
- **#6 编辑细节**：已有来源路径通过悬停读全，原门槛 **600ms**，测试推进 **700ms**；常驻字符新增 **0**，报告几何变化 **0px**。
- **#12 材质细节**：原玻璃提示圆角 **8px**、受光边 **1px**；Apple 控件 8px 是供体，最终沿用仓库 token，不另造材质。
- **#5 多层阴影**：新来源提示直接复用 **2 层**浮面投影；组件内无新增阴影，没有玻璃叠玻璃。

为什么更轻、更有空间感：来源说明只在需要时浮起，静息报告不加行；状态信息发生在正确的完成时刻，错误与结果保留各自身份。不是一次主题改版，颜色与层级 token 零改。

## 两轮目视与证据

第一轮四视角：艺术总监看到报告没有可识别的来源；工程师看到成功先于实际交付；任务所有者碰到选择器故障后没有可用结果；读者遇到缺字段却读到“不是集成导出文件”。第二轮集中修复这四类问题，深浅实帧重看。夹具表面错误单列处理，未据此改启动器只显示最新通知的契约。

目视：浅色成功图中，右上方原错误仍在，其下是配置复制成功；深色校验图明确露出缺失的 endpoint。深浅来源图的提示悬在原报告句上方，移开即回到原来的单行结果。以原生分辨率检查边缘和全文；未宣称真机高 DPI 验收。

共 **36 原帧 + 18 深浅前后对照**，每对像素差非零；详见 `P13-jd/image-differences.txt`。shell 反馈画面 720×520、settingsui 报告 720×900。第 1 项为时序/接线修复，以阻塞后端事件验证，不伪造时序前后静态图。代表图 `/tmp/floter-P13-jd-transfers-{dark,light}.png`。

| 项目 | 深色 | 浅色 |
| --- | --- | --- |
| 复制字段校验 | [对照](P13-jd/copy-validation-dark-comparison.png) | [对照](P13-jd/copy-validation-light-comparison.png) |
| 导出字段校验 | [对照](P13-jd/export-validation-dark-comparison.png) | [对照](P13-jd/export-validation-light-comparison.png) |
| 配置选择器失败 | [对照](P13-jd/config-dialog-error-dark-comparison.png) | [对照](P13-jd/config-dialog-error-light-comparison.png) |
| 集成导出选择器失败 | [对照](P13-jd/export-dialog-error-dark-comparison.png) | [对照](P13-jd/export-dialog-error-light-comparison.png) |
| 集成导入选择器失败 | [对照](P13-jd/import-dialog-error-dark-comparison.png) | [对照](P13-jd/import-dialog-error-light-comparison.png) |
| 复制成功保留错误 | [对照](P13-jd/copy-preserves-refusal-dark-comparison.png) | [对照](P13-jd/copy-preserves-refusal-light-comparison.png) |
| 导出成功保留错误 | [对照](P13-jd/export-preserves-refusal-dark-comparison.png) | [对照](P13-jd/export-preserves-refusal-light-comparison.png) |
| 成功报告来源 | [对照](P13-jd/report-source-dark-comparison.png) | [对照](P13-jd/report-source-light-comparison.png) |
| 失败报告来源 | [对照](P13-jd/failed-report-source-dark-comparison.png) | [对照](P13-jd/failed-report-source-light-comparison.png) |

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，原 token 不改 |
| A2 非系统字体 | 不采用；中文系统字体与仓库契约优先 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用原生设置组件 |
| A5 三等分/卡片海 | 无新增容器 |
| A6 假数据 | 测试只用可执行 provider 与可控错误，不进入产品指标 |
| A7 空泛文案 | 字典零改，错误展示原校验器/系统原因 |
| A8 编号眉批 | 产品无新增 |
| A9 标题行数 | 原页头完整，未改变 |
| A10 Bento | 不适用 |
| A11 左侧装饰线 | 无新增 |
| A2 工艺五项 | 四项具体做法；用户修 bug 优先，不为数量加装饰 |
| A4 功能契约 | 开始/等待/结果/失败/取消/重试已覆盖；无新保存时机 |
| B1 读取/拨盘 | 3/2/7，保守/即时/原密度 |
| B2 主题 | 深浅独立 Tester，无混用 |
| B3 记忆点 | 完成不抢跑、错误可读、来源按需浮起 |
| B4 气质 | 不加动效，复用现有轻浮面 |
| C1 高频操作 | 新增 0ms 动画，原异步工作不阻塞 UI |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原反馈不改，600ms 为悬停触发延迟 |
| C4 origin | tip 锚在报告块上方，实际帧已验 |
| C5 reduced-motion | 新帧使用 ReduceMotion=true |
| C6 hover 查询 | 原生指针路由；CSS media 不适用 |
| C7 布局动画 | 无新增 |
| C8 keyframes | 无新增 |
| C9 手势 | 不涉及拖拽/滑动 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 没有新动效，不声称原生慢放 |
| D2-1 字体 | 系统字体不改，无 webfont |
| D2-2 字号/行盒 | 旧 Caption 9.9/正文 10.8 不满足通用 ≥14px/1.5；仍待拍板 |
| D2-3 中西空格 | 字典零改，路径/系统错误按原文 |
| D2-4 标点/对比 | 原 token 不改；不声称原有安静墨全部达到 7:1 |

缺陷规则落本仓 §6.231，不擅自写全局偏好。原生桌面剪贴板/文件选择器均以现有注入接口验证；Windows/macOS 仅编译门槛，未宣称真机通过。

## 仍待

1. 配置复制/导出的目标在 provider 刷新后消失或撤回 host ownership 时，旧入口能否给准确结果：本轮只审到缺描述分支 finish(nil)，尚未构造实际刷新与按钮事件，不算确认缺陷。下一轮先复现，沿用现有交互。
2. 字号/行盒、accent 禁用填充、多草稿覆盖、未知多选/Radio、极小步长、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾继续需拍板。确认窗口、保存和会话语义未动。
3. 八条旧 stash 保留；原生系统剪贴板 API 的无错误返回限制沿用，当前成功表示写入调用已返回，不承诺平台未提供的交付回执。

## 过程偏差

首批命令串未传播测试失败，02e87cbf 提前推送且提交消息 PASS 计数错误，已用 22d1a0ec 修夹具并明确纠正。后续批次均以 set -e 和实际退出码控制提交。开盘读数输出过多、几次猜测测试文件名不存在，造成额外读取；本轮没有守住 40 次底层工具调用预算，后段停止新选题，仅修已暴露的红灯、执行完整门槛与归档。不能把这些偏差写成流程全部合规。

## 最终门槛与提交

最终普通全量、verbose 全量均退出 **0**；**5407 PASS / 0 FAIL**（基线 5381，增加 26），cmd/internal **3226 测试函数**（基线 3218，增加 8）。gofmt -l cmd internal 为空；Linux/Windows/Darwin 的 go vet ./... 与 go build ./... 六项全部退出 0。相关 race -count=3 退出 0（31.111s）。完整门槛在三组 mutation 还原后执行，未修改产品后复用旧门槛。

四批产品提交：02e87cbf、85a80bf7、72d86c5a、217b6dfd；首批夹具及错误记录单独修正 22d1a0ec，全部已推送。结案 SHA 由本报告最新 git log 与 done.log 定位。

原始证据：[基线](P13-jd/baseline.log)、[普通全量](P13-jd/full-test.txt)、[verbose 全量](P13-jd/full-verbose.txt)、[race](P13-jd/race.txt)、[门槛摘要](P13-jd/gates.txt)、[最终源码 MD5](P13-jd/final-source.md5)。M1/M2/M3 的 red/restored/md5 保留。原始 verbose 日志包含帮助文本的空格后 tab，保留原字节，日志 whitespace 检查只豁免这一项；源码文档检查仍正常。

基线记录开始于 14:22:59 UTC，最终完整门槛约 14:49 UTC 结束，其后只归档与推送。本轮完成八项，但执行时间短于 45–75 分钟目标，连同提交与调用预算偏差如实保留。
