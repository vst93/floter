# P13-jb：快捷键录制的退出与恢复

开盘 2026-11-07 12:46 UTC，起点 9d76d244。pull 同步，工作区干净。中文 locale 实测基线 5328 PASS / 0 FAIL，退出 0；cmd/internal 下 3201 个 Test 函数。上一轮“命令输入退出”线索引出录制器未随页面/窗口离开的问题，本轮以真实 UI 点击、按键和宿主退出接口验证。

## Stash 清点

八条均为已登记早期 WIP，无新的 stale-kill。与 P13-ja SHA 相同；反向检查失败本身不代表独有可接续工作，按 backlog 已有语义核对保留，未重入旧主题/Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | reverse=1，早期 keycap WIP | 保留 |
| stash@{1} | 8af94a07 | reverse=1，早期设置间距 WIP | 保留 |
| stash@{2} | 7e4aa710 | reverse=1，卸载指标/临时测试 WIP | 保留 |
| stash@{3} | 1e3aec94 | reverse=1，旧数字字段 WIP | 保留 |
| stash@{4} | 363f5a42 | reverse=1，旧插件字段 WIP | 保留 |
| stash@{5} | 9b8ce23c | reverse=1，旧快捷键校验 WIP | 保留 |
| stash@{6} | 90d314b2 | reverse=1，旧启动器动作栏 WIP | 保留 |
| stash@{7} | 7a9f6e13 | reverse=1，旧 main/Tauri WIP | 保留 |

done.log 开盘不存在，收尾创建，不伪造上轮记录。八项结果随每批追加。

## 第一批：切页结束不可见录制

1. **内置录制切页**：现象 → 点击 Sessions 后旧录制仍暂停召回，第一次 Escape 只取消看不见的录制；回 Shortcuts 重新显示录制提示。改动 → selectPage 离开 Shortcuts 时结束捕获并即时同步注册。验证 → 深浅真实点击，召回与自定义原键均恢复，返回不复活，Escape 一次关闭；未 Enter 的命令保持原样并可再次提交。
2. **自定义录制切页**：现象 → 编辑命令后录制该行，切页同样留下暂停与旧提示。改动 → 同一退出方法同时清除自定义捕获归属，不清草稿。验证 → 深浅事件确认原 Ctrl+Shift+B / say original 未被离开动作改写，say unfinished 回来仍可 Enter。

相关回归 274 PASS / 0 FAIL，settingsui/shell vet 净。M1 回注原 selectPage/录制实现实红：`invisible recorder still suspends global shortcuts`、`exit did not restore the summon and custom registrations`、`invisible recording swallowed page Escape`、`return resurrected the old recording`。MD5 恢复、touch 后 6 PASS / 0 FAIL。补充返回瞬间截图后重新实跑 M1，避免用 Escape 已消掉问题的图片作对比。既有 V/B/S 0/0/0，旧测试不改。

第一批 dc47d2f1 已推送。

## 第二批：关闭设置后释放录制

3. **内置录制关闭设置**：现象 → 点击页头关闭后到了启动器，召回仍被暂停；重开又显示旧录制。改动 → Open 离开设置时先结束捕获，再走原 Flush。验证 → 深浅真实关闭按钮，原注册立即恢复，重开无旧提示，草稿未保存且可继续 Enter。
4. **自定义录制关闭设置**：现象 → 自定义命令行改键期间关闭，已保存键没有恢复、返回抢焦点。改动 → 相同宿主出口释放自定义捕获归属。验证 → 原 Ctrl+Shift+B/say original 保持，say unfinished 回来可编辑提交，原召回与自定义注册都存在。

相关回归 280 PASS / 0 FAIL，包 vet 净。M2 回注缺少 Open 退出处理的旧分支，实红 `invisible recorder still suspends global shortcuts`、`exit did not restore the summon and custom registrations`、`reopen resurrected the old recording`；MD5 还原、touch 后 6 PASS / 0 FAIL。既有 V/B/S 0/0/0。

第二批 1e2f5bc6 已推送。

## 第三批：隐藏前恢复全局键

5. **内置录制隐藏窗口**：现象 → Hide 已执行但召回键仍未注册，用户不能再用它打开窗口。改动 → Hide 在停止绘制前即时结束捕获并恢复注册。验证 → 调用真实 Hide 出口、注入窗口隐藏钩子且只调用一次；深浅下原两键立即注册，重开没有录制提示，草稿不变。
6. **自定义录制隐藏窗口**：现象 → 行改键过程中隐藏，同样释放了全局键却没有恢复。改动 → Hide 对两类捕获统一退役。验证 → 保存行未改键/改命令，未 Enter 文本仍在，重开可提交；没有新会话或原命令重跑。

相关回归 286 PASS / 0 FAIL，包 vet 净。M3 回注旧 Hide，实红 `invisible recorder still suspends global shortcuts`、`exit did not restore the summon and custom registrations`、`reopen resurrected the old recording`；MD5 还原、touch 后 6 PASS / 0 FAIL。既有 V/B/S 0/0/0。隐藏的原生窗口调用由钩子计数验证，没有冒充原生桌面真机。

第一轮目视：浅色自定义关闭对照中，旧的蓝色 Press keys… 已换回 Ctrl+Shift+B；右侧 say unfinished 保持完整。没有增加玻璃层或提示占高。

第三批 804b1055 已推送。

## 第四批：内置拒绝仍属于原行

7. **开始邻行录制**：现象 → New command 录到 Paste 已占用键后出现拒绝；点击另一内置行录制时，原行拒绝无故消失。改动 → 只有重试被拒的那一行才清其拒绝。验证 → 深浅真实点击/冲突按键，邻行仍进入录制且写入 0 次，原行提示保留，回原行重试可清除并成功改键。
8. **邻行录制成功**：现象 → 即使保住开始时的拒绝，邻行成功接受新键后仍清掉原行提示。改动 → recordShortcut 的成功/同键分支只结束当前 action 的拒绝。验证 → 单修第 7 项后，第 8 项独立实红；两处完成后，邻行成功写入恰 1 次，原行原键与拒绝保持，原行可继续重试成功。

相关回归 295 PASS / 0 FAIL，包 vet 净。M4 回注两处无条件清空，实红 `neighbor recording erased the original row refusal`；MD5 还原、touch 后 6 PASS / 0 FAIL。既有 V/B/S 0/0/0；首次新测试常量拼写错误导致编译失败，修正后才计入事件红灯，不把编译错误算 mutation。

原候选的宿主页跳转没有当前设置面可点击触发证据，因此未计修复、未加防护；其测试分支也已删除。最后两项是同一快捷键表面真实可达的状态归属问题，不扩展拒绝历史模型。

第四批 92cbe6c4 已推送，开始普通/verbose 全量与三平台 vet/build。最终数字只在完整进程退出后填写。

## 偷了什么

真实读取 qiaomu-design 的 SKILL、完整偏好账本、craft-loop、preflight、chinese-typography，以及 Apple DESIGN 的颜色/控件段。用户已定方向，Codex 自行实现；未生成方向预览、未调用 K3。

- craft-loop 工艺密度菜单 **#3 品牌 focus 环**：Apple 的单一交互蓝与 **2px** focus 角色映射到本仓现有 token；旧录制退出后不再以蓝色捕获框抢回键盘，额外焦点动画 **0ms**。
- **#4 签名交互**：结束捕获与恢复注册延迟 **0ms**，隐藏调用 **1 次**；不等下一帧，也不增加计时器。高频键盘操作即时反馈。
- **#6 编辑细节**：未 Enter 命令原样保留，退出触发的命令保存 **0 次**；邻行开始录制写入 **0 次**，邻行接受新键写入 **1 次**；不增加常驻成功文案。
- **#12 材质细节**：Apple 控件圆角角色沿用本仓 **8px**，原 **1px** 受光边保持，新增玻璃/阴影 **0 层**。改的是状态何时存在，不堆材质。

为何更轻、更有空间感：已退出的捕获框不再复活，键盘回到当前可见表面；尚未解决的拒绝稳定留在原行。这里的“轻”是减少状态错位，不声称提高了材质对比。按用户修复优先与 craft-loop 的优先级，不为工艺密度凑五项装饰。

## 两轮图像复核

第一轮四视角：艺术总监看到设置重开后蓝色录制框意外成为焦点；工程师确认隐藏后注册表仍空；任务用户切到 Sessions 后第一次 Escape 没有关闭；读者看到原行拒绝随邻行操作消失。第二轮集中核对返回键帽、草稿完整、Escape 出口、拒绝归属四点。

目视深浅总览和原帧：前六项的 Press keys… 换回原键，say unfinished 没有被保存值覆盖；深色邻行接受图中 Ctrl+Alt+Y 已显示，New command 下的原拒绝仍可见。保留原 warning 的上下占位是现有反馈结构，不加新容器。局部 4× 为最近邻检查图，不冒充高 DPI 真机。

| 项 | 浅色前后 | 深色前后 |
| --- | --- | --- |
| 1 内置切页 | [图](P13-jb/builtin-page-light-comparison.png) | [图](P13-jb/builtin-page-dark-comparison.png) |
| 2 自定义切页 | [图](P13-jb/custom-page-light-comparison.png) | [图](P13-jb/custom-page-dark-comparison.png) |
| 3 内置关闭 | [图](P13-jb/builtin-close-light-comparison.png) | [图](P13-jb/builtin-close-dark-comparison.png) |
| 4 自定义关闭 | [图](P13-jb/custom-close-light-comparison.png) | [图](P13-jb/custom-close-dark-comparison.png) |
| 5 内置隐藏 | [图](P13-jb/builtin-hide-light-comparison.png) | [图](P13-jb/builtin-hide-dark-comparison.png) |
| 6 自定义隐藏 | [图](P13-jb/custom-hide-light-comparison.png) | [图](P13-jb/custom-hide-dark-comparison.png) |
| 7 开始邻行 | [图](P13-jb/builtin-refusal-start-light-comparison.png) | [图](P13-jb/builtin-refusal-start-dark-comparison.png) |
| 8 接受邻行 | [图](P13-jb/builtin-refusal-accepted-light-comparison.png) | [图](P13-jb/builtin-refusal-accepted-dark-comparison.png) |

[总览](P13-jb/overview.png)、[像素差](P13-jb/image-differences.txt)。16 组均有变化，范围 **983–314149 像素**。原帧分别 960×1100、720×900；Linux ui.Tester 软件自绘。代表图 `/tmp/floter-P13-jb-shortcuts-{dark,light}.png`。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，原主题保持 |
| A2 非系统字体 | 原生中文系统字体规则优先，不为网页要求换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置面 |
| A5 三等分/卡片海 | 无新增包装 |
| A6 假数据 | say unfinished 是事件夹具；产品无新增虚构指标 |
| A7 空泛文案 | i18n 零改 |
| A8 编号眉批 | 无新增 |
| A9 H1 行数 | 原页头完整可见 |
| A10 Bento | 不适用 |
| A11 左侧装饰线 | 无新增 |
| A2 工艺五项 | 未凑满；用户修复优先，按 craft-loop 保两轮验证、不加装饰 |
| B1 读取/拨盘 | 3/2/7 保守、即时、原密度 |
| B2 主题 | 每个 Tester 固定深或浅，没有混主题 |
| B3 记忆点 | 退出后原键可用，反馈仍属于原行 |
| B4 动效气质 | 新状态交接 0ms，适合高频键盘 |
| C1 目的/频率 | 没给高频操作增加动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 沿用原 120ms 颜色反馈与按压；无新时长 |
| C4 origin | 拒绝仍在原行；无新增弹层 |
| C5 reduced-motion | 全部新增事件使用 ReduceMotion=true |
| C6 hover 查询 | Go 自绘，无 CSS；原指针路由保留 |
| C7 布局动画 | 无新增；拒绝保留的是原有静态占位 |
| C8 keyframes 重播 | 无新增 |
| C9 手势 | 不涉及 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新动画，不声称做了原生慢放检查 |
| D2-1 字体 | 沿用系统字体，无 webfont |
| D2-2 字号/行盒/字重 | 通用 ≥14px/1.5 未满足；旧字号与行盒仍待拍板，未改 |
| D2-3 混排空格 | 产品文案不改 |
| D2-4 标点/对比 | 原墨与标点保留；不宣称旧次要墨全部过 7:1 |

没有把一次性修复写成全局 skill 审美偏好；可复现规则落本仓 §6.229 和事件 pin。用户明确方向优先于四方案/浏览器 viewport 等网页泛化要求。

## 仍待

1. 下一轮查设置里的“恢复注册被系统拒绝”与窗口重开组合：焦点出口、拒绝归属、可再次录制；先复现，不再扩写迟到回调假设或重复本轮三条退出路径。
2. Add 草稿被另一保存行转换为命令覆写，仍是既有需拍板项；未加多草稿或覆盖确认。另一端冲突变化历史、字号/行盒、accent 禁用、未知多选/Radio、极小步长、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾继续保留。
3. 原生桌面热键抢占与 hide-on-blur 的系统事件顺序未真机验，Windows/macOS 仅编译检查；8 条早期 WIP 逐条保留。

## 完整门槛与交接

产品提交 **92cbe6c4**。最终完整 verbose **5352 PASS / 0 FAIL**，基线 5328，增加 **24**；Test 函数以同一 cmd/internal 命令计数 **3201 → 3209**。上轮报告的 3203 不作为本轮未经实测的基线。普通/verbose 全量均退出 0，gofmt 空，linux/windows/darwin vet/build 全部退出 0，追加合约后的 internal/ci 检查通过。

四批均通过相关测试与包 vet 后提交推送；完整门槛集中执行，旧测试断言未改、无新增 Skip。四组 mutation 全部实红并逐字节 MD5 还原、touch 后各 6 PASS；最终源码 MD5 再核对全部一致。

证据：[门槛](P13-jb/gates.txt)、[普通全量](P13-jb/full-test.txt)、[文档契约](P13-jb/docs-contract.txt)、[最终源码 MD5](P13-jb/final-source.md5)、[M1 红](P13-jb/M1-red.txt)、[M2 红](P13-jb/M2-red.txt)、[M3 红](P13-jb/M3-red.txt)、[M4 红](P13-jb/M4-red.txt)、[独立第八项红](P13-jb/refusal-start-only.txt)。完整 verbose 与各平台原始日志保留在 /tmp/floter-P13-jb/，没有把编译失败算作事件复现。

八项已完成，包含两类录制器与三个退出组件；后续仅交接已记录线索，不扩展系统。12:46 UTC 开盘，13:14:45 UTC 完整门槛与首次结案推送完成，约 **29 分钟**，低于 45–75 分钟目标；八项均已实测完成，不重复测试或添加装饰填时长。最后补录这条实测耗时，产品与验收证据不变。
