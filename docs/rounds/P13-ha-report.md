# P13-ha · 运行表单与输出的状态闭环

2026-11-05 续接抢救稿，起点 `35751144`，分支 `mygo-rewrite`。工作树干净，pull --ff-only 已是最新。沿用原轮号完成八项，不把前次中断当作已验收。当前环境 done.log 不存在，收尾创建。

## stash 清点

开盘十条 reverse-check 全部失败。两条新增 orphan 的 tracked tree 与第三父 untracked tree 分别完全相同；属于独有、未提交的首批两项，已用 `git apply -3` 接续代码及文档，恢复第三父中的测试和 PNG，重新实测。原报告中未经本次复核的计数不沿用。

| 开盘 ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 92315415 | P13-ha 首批两项独有工作 | `c6d0d6d6` 单独提交；产品补丁 reverse-check 通过，原件归档后已 drop |
| stash@{1} / ff170912 | 与上一条代码及第三父完全相同 | 未重复套用，原件归档后已 drop |
| stash@{2} / 422483b4 | P7 keycap/旧主题 WIP | 沿用 P13-gq/gz 语义审核，保留原件 |
| stash@{3} / 8af94a07 | 行距/边缘旧刻度草稿 | 后续已有正式刻度，保留原件 |
| stash@{4} / 7e4aa710 | 卸载布局与 scratch 探针 | 保留原件 |
| stash@{5} / 1e3aec94 | 旧数字框/透镜草稿 | 保留原件 |
| stash@{6} / 363f5a42 | 旧确认带/快捷键草稿 | 保留原件 |
| stash@{7} / 9b8ce23c | P7 快捷键拒绝反馈草稿 | 保留原件 |
| stash@{8} / 90d314b2 | 旧 action bar，与现有键盘合约不同代 | 保留原件 |
| stash@{9} / 7a9f6e13 | 旧 Tauri release workflow | 保留原件，不恢复旧发布链 |

## 已完成（逐条，增量记录）

1. **运行按钮漏锁**：展开后复检、禁用、移除、卸载、开关写入中，或 stopped/broken/orphan 时，内层 Run 仍派发运行 → 同时检查已有 rowBusy 与运行可用状态，建造时禁用 → 两主题 × 八状态拒绝派发，解除后恰好增加一次。旧实现实跑：`unavailable repair form dispatched 1 runs`。
2. **文字参数漏锁**：文本、数字、路径参数在同行复检中仍获焦并写入 `123` → `!locked` 建材质，根 disabled、父行 Inert，保留草稿 → 两主题 × 三类型拒绝编辑，解除后恢复。旧实现实跑：`busy text input accepted editing: value="123" focused=true`。

上述红测为本次重新执行，日志 `/tmp/floter-P13-ha-rescue-before.log`。首批定向与 mutation 恢复后均 **65 PASS / 0 FAIL**；全量基线 **3546 PASS / 0 FAIL**，测试函数 **2705**。接续版本全量 settingsui 通过（313.312s）；其余包中 shell 的 `TestARescanAsksForAFreshDetectedList` 曾报 `the frame after a rescan started no pass`，单项重复 10 次也出现一次；它直接观察异步任务的瞬时 pending，未改其断言，shell 包整体复跑通过。三平台 vet/build 通过，gofmt 空。最终全量将覆盖全部八项。

正式 M1 把 runDisabled 恢复成只看 issues，16 场景红；正式 M2 将行锁读数替换为 false，6 场景红。原始串分别同上两项；每次恢复 `md5sum -c` 均 OK（settingsui.go `a4eec1370b22cb5622a559175292a0e5`），touch 后非缓存回归 65 PASS。V/B/S 台账 **0/0/0**，未改旧断言。

3. **布尔参数漏锁**：忙态点开关或标题仍把 false 改成 true，父行浅色 246→220、深色 15→49 → 两入口遵守 rowBusy，控件根禁用、父行 Inert → 深浅 × 两入口保持 false 和静息像素；解锁后一次点击变 true。原始红测：`busy boolean changed through switch` / `busy boolean changed through label`。
4. **分段参数漏锁**：忙态点击/方向键仍可把 fast 改为 careful，父行也抬升 → 整个分段根禁用，写值与父行都遵守行锁 → 两主题拒绝点击/方向键，解除后正常选择 careful。原始红测：`busy select changed to "careful"`。

第二批相关回归 **73 PASS / 0 FAIL**（`choice-after.log`）。所有旧断言零改动。首批提交 `c6d0d6d6` 已推送；两个相同 orphan 的产品补丁 reverse-check 通过，已创建 `refs/archive/floter/P13-ha/<完整 SHA>` 保存原始报告/图片/测试，再按开盘 ref 1、0 顺序 drop；活动清单回到八条早期 WIP。

5. **错误跨表单串用**：A 的失败在 B 出现，取消或运行 B 又清掉/替换 A 的错误 → runError 按 integration.ID 存，取消、重试与入口展开只清自己 → 深浅均验证错误不串用，A/B 的失败可同时保留。原始串：`first run failure leaked into second form`、`canceling second form erased first failure`、`second run replaced first failure`。
6. **参数重排写错字段**：Target 输入 first 后参数顺序改变，继续输入 -tail 却写入 Note → 表单根以集成 ID、参数行以参数 ID 定位 → 两主题均保留 Target 焦点，最终 Target=`first-tail`、Note 为空。原始串：`parameter reorder lost edit target: target="first" note="-tail"`。原规格“前行错误插入”只是线索，没有算修复；此项以真实重排复现替代。

第三批相关回归 **79 PASS / 0 FAIL**，日志 `owner-{before,after}.log`。第二批提交 `eb0d857c` 已推送。

7. **新输出沿用旧滚动**：读过长输出后，新成功/失败结果均停在 Y=300，首行不可见 → SetRunResult 只重置本集成既有 ScrollState → 两主题 × 成功/失败均 Y=0，其他集成 Y=37 原样保留，已展开状态保留。原始红测：`new output starts at stale Y=300`。
8. **ANSI 原串泄露**：捕获区把 ESC[31m、ESC[0m、ESC[2K 当作文本 → 复用已有 ansi 解析与 OutputView.TextSpans/RichText → 两主题得到红色 ALERT 与普通正文，复制为 `ALERT plain`，LastRun 原始字节不变；空串及只有控制序列时保留原空态。原始红测：`captured output exposes ANSI bytes`。

第四批相关回归 **107 PASS / 0 FAIL**。初次替换 RichText 时空串被 Parse 表示为一个空 span，既有空态两条 B 断言报红；已修产品的可见文本判定，两条原断言保持原样转绿。第三批提交 `08586e80` 已推送。

## 设计与验证计划

读取为：面向桌面工具用户的原生参数表单与输出凹槽，轻便液态玻璃。拨盘 2/2/6，保留高度链、键盘、会话、写回与 i18n。三秒内能分清可操作项、错误所属表单和最新输出的开头。

为什么更轻、更有空间感：不可操作的面退回静息层，只有可操作控件承担受光反馈；输出沿用现有凹槽，不叠加玻璃或阴影。

已实际读取 qiaomu-design / user-preferences / craft-loop / preflight / chinese-typography，以及 Apple 供体的颜色、字阶、按钮章节。当前 Codex 独立执行，未调用 K3 或子代理。

## 偷了什么

- `craft-loop.md` **#12 材质细节**：文字参数建面时 `DimFace` 熄受光边，根由框架乘 **0.5**，保留 **1px** 外边与底侧线；布尔/分段根同样只淡一次。
- **#4 签名交互**：保留既有 **120ms** 指针反馈，但忙态父行附加 hover 为 **0**；两主题布尔/分段父行实测回到 **246/15** 的静息红通道值，点击不改值，解锁后一次生效。
- **#3 focus**：复用现有 focus 面，禁用文本参数不获焦；表单/参数分别按对象 ID 定位，重排后焦点仍为 Target，`first` + `-tail` 完整落到同一输入。没有新增焦点样式。
- **#6 编辑细节**：每个集成独立保存 **1** 份运行错误；参数表单与输出仍沿用 **8px** 控件圆角、**1px** 发丝线，不新增状态卡片。借的是信息归属和现成刻度，不是装饰编号。
- **#9 滚动细节**：复用已有 **180px** 最大输出视口、**12px** 上下软边；新文档滚动 **300→0**，其他集成的 **37** 原样保留。没有另造滚动条，也没有改既有刻度。
- Apple 供体取单一动作色、胶囊与 1px 边：保留项目 accent；Run 的 **30px** 高对应 **15px** 半高圆角，框线 **1px**。未移植官网大字号、纯黑或 blur。

## 图片与两轮复核

本轮 **48 张原始 PNG**（24 个深浅场景 × before/after）在 `rounds/P13-ha/`，ui.Tester 软件渲染；没有原生窗口或 Windows/macOS 真机验收。标准 `/tmp/floter-P13-ha-<表面>-<dark|light>.png` 指向本次 after 副本。

| 表面 | 深色前后 | 浅色前后 |
|---|---|---|
| 忙态文本参数 | [前](P13-ha/floter-P13-ha-input-text-dark-before.png) / [后](P13-ha/floter-P13-ha-input-text-dark-after.png) | [前](P13-ha/floter-P13-ha-input-text-light-before.png) / [后](P13-ha/floter-P13-ha-input-text-light-after.png) |
| 忙态布尔参数 | [前](P13-ha/floter-P13-ha-boolean-switch-dark-before.png) / [后](P13-ha/floter-P13-ha-boolean-switch-dark-after.png) | [前](P13-ha/floter-P13-ha-boolean-switch-light-before.png) / [后](P13-ha/floter-P13-ha-boolean-switch-light-after.png) |
| 参数身份 | [前](P13-ha/floter-P13-ha-input-identity-dark-before.png) / [后](P13-ha/floter-P13-ha-input-identity-dark-after.png) | [前](P13-ha/floter-P13-ha-input-identity-light-before.png) / [后](P13-ha/floter-P13-ha-input-identity-light-after.png) |
| 输出首行 | [前](P13-ha/floter-P13-ha-output-reset-success-dark-before.png) / [后](P13-ha/floter-P13-ha-output-reset-success-dark-after.png) | [前](P13-ha/floter-P13-ha-output-reset-success-light-before.png) / [后](P13-ha/floter-P13-ha-output-reset-success-light-after.png) |
| ANSI 输出 | [前](P13-ha/floter-P13-ha-output-ansi-dark-before.png) / [后](P13-ha/floter-P13-ha-output-ansi-dark-after.png) | [前](P13-ha/floter-P13-ha-output-ansi-light-before.png) / [后](P13-ha/floter-P13-ha-output-ansi-light-after.png) |

第一轮四视角：艺术——忙态父行整带提亮抢注意；工程——参数重排后焦点跟错对象；任务——新输出丢首行；读者——ANSI 控制串抢占行内文字。第二轮分别关闭父行假反馈、定位参数身份、归零新输出、接回已有 ANSI 样式。自行看图看到：深色输入的亮灰整带已退掉而 Target 标签保留；新输出的 `NEW RUN FIRST LINE` 回到顶端，滚动条回顶；深浅 ANSI 图上均只保留红色 ALERT 和普通 plain，控制码消失。

## preflight A / B / C / D2

| 条目 | 结论 |
|---|---|
| A1 渐变/纯黑/霓虹 | 无新增；沿用项目近黑与近白，通过 |
| A2 每轮换非系统字体 | 不采用；中文系统字族及原有高度链优先 |
| A3 斜体 | 产品 UI 无新增斜体；程序 ANSI 样式按已有输出契约解析 |
| A4 Hero | 不适用，原生参数与输出组件 |
| A5 卡片模板 | 无新增容器，通过 |
| A6 假名/假数字 | 产品无假数据；Runner/Target 为测试夹具 |
| A7 空词/感叹号 | i18n 产品文案未变 |
| A8 眉批 | 无新增 |
| A9 H1 | 不适用 |
| A10 网格空洞 | 未改布局；既有行盒和输出几何断言保留 |
| A11 左装饰线 | 无新增 |
| B1 读取/拨盘 | 2/2/6，与既定轻玻璃和紧凑密度一致 |
| B2 主题锁定 | 深浅分别渲染并复核 |
| B3 记忆点 | 可操作控件承担受光，结果各归其主 |
| B4 动效气质 | 沿用短反馈，无新动效 |
| C1 目的/频率 | 无新增键盘动画 |
| C2 all/ease-in/scale(0) | 无新增 |
| C3 时长/按压 | 保留既有 120ms；新锁定状态拒绝按压反馈 |
| C4 origin | 无新浮层，不适用 |
| C5 reduced-motion | 回归使用 ui.Preferences.ReduceMotion；既有框架降级保留 |
| C6 hover 媒体条件 | 原生指针系统，不适用 Web 媒体查询 |
| C7 属性 | 无新增布局动画 |
| C8 快速触发 | 沿用 transition，不重播 keyframes |
| C9 手势 | 无新增，不适用 |
| C10 Framer Motion | 不适用 |
| C11 逐帧/真机 | 已检静息、点击、禁用和恢复；未宣称真机动效验收 |
| D2-1 字体栈 | 中文系统字体保留；未下载字体 |
| D2-2 行高/字号/字重 | 现有 Body 10.8px、Caption 9.9px 不达 ≥14px，580 标签字重也超出该建议；继续列入既有拍板项，不能宣称全过 |
| D2-3 混排空格 | 无新产品文案，沿用 i18n；输出是程序自带文本 |
| D2-4 标点/对比 | i18n 标点未变；项目既有安静墨与字号限制保留，不宣称全达 7:1 |

## 最终 mutation 与台账

M1/M2 的原始串与恢复证据见前文。后续在完整产品上额外实跑：

- M3 删除参数行 `c.Key(param.ID)`：两主题红，`parameter reorder lost edit target: target="first" note="-tail"`。
- M4 删除 SetRunResult 的 scroll 清零：四场景红，`new output starts at stale Y=300`。

两次恢复 `md5sum -c` 均 OK，最终产品 MD5 **750b6422b6b536afc8a07d69c30e5046**，touch 后启动全量非缓存复跑。另尝试只删布尔根 Disabled 时写入保护仍阻止变值，该试验未红，不计入四条正式 mutation。

最终 **V 0 / B 0 / S 1**：全量发现 `TestTheEmptyCaptureStaysAheadOfTheTruncationAside` 的源码 pin 钉着 `ui.Text`，实际输出已改为 RichText 但仍取 runOutputFont。按 S 类改为精确匹配 RichText → Font(runOutputFont) 链，并写明旧结构→新结构；其余字体、行高、选择与空态断言不变。没有新增 Skip、放宽容差、删除测试或弱化成非空。

## 最终门槛

中文 locale、`GOMAXPROCS=4` 下：`gofmt -l cmd internal` 空；`go vet ./...` 与 `GOOS=windows/darwin go vet ./...` 净；Linux/Windows/macOS `go build ./...` 通过；普通 `go test -count=1 ./...` 与 verbose 全量均 **3592 PASS / 0 FAIL**。函数数由接续前 **2705** 增至 **2713**（新增 8 个实测回归函数），没有减少测试。最终产品 MD5 `internal/settingsui/settingsui.go` = `750b6422b6b536afc8a07d69c30e5046`。

## 仍待

- 本轮八项全部落地；未变更终端 S1b/S1c/S2 尾、176-9/161-4 保存语义及字号/行盒拍板项。
- 下轮沿设置输入通用队列，先复现本地工具编辑表单的运行时探测切换与错误恢复；只计真实可见缺陷，不开新系统。
- shell 的 rescan 测试瞬时 pending 偶发红已记录，后续若处理须固定可控时序，不能通过放宽断言消红。
