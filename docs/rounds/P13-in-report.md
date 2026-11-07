# P13-in · 集成刷新后的键盘目标

开盘 bc606b67，工作树净，pull 已最新。Codex 独立执行；已读 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography，沿既有方向打磨。基线实测 4821 PASS / 0 FAIL / 3057 测试函数；done.log 开盘不存在。

## Stash 清点

早期 stale-kill 已在前轮归档。当前八条均与 P13-im 及 backlog SHA 一致，无新抢救稿；实跑 reverse-check 均 1。沿用语义审查结论，不把旧主题/Tauri 工作覆盖到当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse 1 | 保留 |

## 复现与第一批（1–2）

旧产品七条候选失败（含三个命令退场子场景），忙态同帧改型已能到 Cancel，仅保留保护测试，不计修复。原始事件与深浅图已落盘。补查运行入口禁用作为第八条候选。

1. 现象：运行参数持焦时整张集成卡撤回，Enter 无法退出。改动：整卡退场清单加入 run，归还既有 Close。验证：深浅真实 Tab → 撤回 → Enter，关闭恰好一次，答案 12 保留、运行零次。
2. 现象：卡片成为 orphan 且不再声明参数，原表单消失后 Enter 无出口。改动：orphan 分支接入 run 焦点回收，优先 Adopt。验证：深浅刷新时零动作、Enter 才 Adopt 一次，保留答案。

相关门槛 **233 PASS / 0 FAIL**，settingsui vet 净。M1 撤回两文件到开盘实跑失败，原串 `removed run lost keyboard destination "Close settings"`、`orphan run lost keyboard destination "Adopt"`；MD5 一致并 touch 后 **6 PASS**。旧 V/B/S 断言修改 **0/0/0**。

## 第二批（3–4）

3. 现象：重排后继续输入别名，回调收到邻居 build。改动：命令板身份绑定 ID + 同 ID 出现序号。验证：深浅原 JSON Viewer 的焦点保留，输入 jview 后 Flush 只写 jv。
4. 现象：重排后按 Space 切换邻居 build。改动：同一身份覆盖板内开关。验证：深浅焦点仍跟随 JSON Viewer，Space 只切 jv；重复 ID 的 Build 开关仍能使用，无 key 冲突。

相关 **65 PASS / 0 FAIL**，包 vet 净。M2 撤回身份接线实跑红，原串 `reordered alias edited wrong command: id="build" value="jview"`、`reordered switch toggled wrong command "build"`；MD5 恢复、touch 后 **6 PASS**。旧 V/B/S 0/0/0。已看浅色前后全幅：蓝色输入边从错误的 Build 回到 JSON Viewer；深色开关环跟着原命令，几何不变。

## 第三批（5–6）

5. 现象：命令移除、Declared 撤回或整张卡移除后，键盘停在消失的别名框。改动：命令板登记当前持焦成员，退场回页头 Close；命令区有稳定身份。验证：三个路径 × 深浅，刷新零回调，Enter 关闭一次。
6. 现象：命令列表因卡片变 orphan 消失，键盘无出口。改动：orphan 分支把命令持焦位置交给既有 Adopt。验证：深浅刷新不执行操作，Enter 才 Adopt 一次。

相关 **277 PASS / 0 FAIL**，包 vet 净。M3 撤回本批两文件实跑红，原串 `removed command lost keyboard destination "Close settings"`、`undeclared command lost keyboard destination "Close settings"`、`card-removed command lost keyboard destination "Close settings"`、`orphan command lost keyboard destination "Adopt"`；MD5 恢复、touch 后 **13 PASS**。旧 V/B/S 0/0/0。

第八项补查成立：运行入口停止/破损/忙态禁用均无法 Enter 退出；表单提交键的停止/破损/忙态/验证失败共四类同样失焦。深浅共 14 个叶子在旧产品全红，下一批修这两个控件。

## 第四批（7–8）

7. 现象：已聚焦的表单提交键因停止、破损、忙态或参数验证刷新而禁用，Enter 不能取消。改动：提交键记住当前表单的持焦身份，失效时回原 Cancel。验证：四类 × 深浅，刷新不运行、不清答案，Enter 取消。
8. 现象：卡片 Run 入口禁用或撤回后失焦。改动：禁用时回页头 Close；整卡移除、能力撤回同样回 Close，变为 orphan 时回 Adopt。验证：六类 × 深浅，刷新零动作，Enter 只到对应出口；orphan 仍声明参数时也覆盖，不改变既有表单保留语义。

最终相关 **142 PASS / 0 FAIL**，包 vet 净；额外不抢焦与重复 ID 检查 **7 PASS / 0 FAIL**（也包含在 142 内）。M4 初版 **23 PASS** 还原；补全入口撤回后最终变异覆盖十组深浅场景。旧 V/B/S 0/0/0，新增 Skip 0。所有八项均有事件复现，忙态改型只是既有正确行为保护。

M4 最终撤回两文件实跑红，原串 `submit-stopped run lost keyboard destination "Cancel"`、`disclosure-stopped run lost keyboard destination "Close settings"`、`disclosure-orphan run lost keyboard destination "Adopt"`；MD5 恢复、touch 后 **32 PASS**。最终两套 before/after 独立保存，mutation 恢复帧明确用 after 后缀。

## 偷了什么

- craft-loop 第 **3** 项品牌 focus 环：沿用 **2px** 环，让 **1 个**当前目标持焦；退出与刷新 **0ms** 归还，非持焦控件 **0 次**抢焦。深浅提交键图里，蓝环从失效 Run 移到可用 Cancel。
- 第 **6** 项编辑细节：保留 **1px** 发丝分隔与命令板 **8px** 圆角，命令重排后仍是原输入/开关；新增标签和常驻提示 **0 条**。
- 第 **12** 项材质细节：沿用原面差和受光边，新增面/阴影 **0 层**。以上是保留的具体边界，不冒充材质改版；不为五签名配额添加无关装饰。

这轮更轻、更有空间感：焦点跟随当前对象和可用控件，退场后没有无主焦点；用现有环表达位置，不增加解释性面板。

## 两轮图像复核

第一轮：视觉视角看到输入边跑到灰色邻居；工程视角看到撤回后 Enter 没有接收者；任务视角看到输入与开关误改 build；读者视角看到不可运行的提交键仍戴环。第二轮围绕这四处修正，重新导出深浅图。已亲眼复核深浅命令重排、深浅提交失效对照及 4× 开关局部：原几何保持，唯一环落回正确对象；圆角处环完整，无新增材料边。

[命令深色](P13-in/command-alias-reorder-dark-comparison.png) · [命令浅色](P13-in/command-alias-reorder-light-comparison.png) · [提交深色](P13-in/run-submit-stopped-dark-comparison.png) · [提交浅色](P13-in/run-submit-stopped-light-comparison.png)。左为修前，右为修后；原帧均为 720×900，自绘全页而非 HTML 预览。

## Preflight

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹 | 无新增；现有主题 token 保留 |
| A2 字体 | 保留系统 CJK，D2 优先；不执行每轮换字体的网页规则 |
| A3 斜体 | 无新增 |
| A4 Hero | 功能设置页，不适用 |
| A5 三栏/卡片 | 未新增容器，命令板保留既有语义 |
| A6 假数据 | 本地测试工具，不宣称业务数据 |
| A7 套话 | 无新产品文案 |
| A8 眉批 | 新增 0 |
| A9 标题 | 既有标题和行盒保留 |
| A10 网格 | 无新增布局 |
| A11 装饰竖线 | 未引入 |
| B1 读取/拨盘 | 功能页 2/1/6，一致 |
| B2 主题 | 深浅独立渲染 |
| B3 记忆点 | 当前对象只有一个键盘位置 |
| B4 动静一致 | 即时恢复，新增动画 0 |
| C1 目的/频率 | 高频键盘操作即时反馈 |
| C2 all/ease-in/scale | Go 自绘，无新增 |
| C3 时长/按压 | 原按压反馈不变，恢复 0ms |
| C4 原点 | 无新浮层 |
| C5 减少动效 | 事件与图均 ReduceMotion=true |
| C6 hover | 非 CSS，沿原指针机制 |
| C7 布局动画 | 新增 0 |
| C8 keyframes | 新增 0 |
| C9 手势 | 本轮不涉及 |
| C10 Motion shorthand | 非 Web，不适用 |
| C11 慢放 | 无新动效；逐帧事件核对 |
| D2-1 字体 | 中文 locale，沿系统 CJK |
| D2-2 字号/行高/字重 | 通用 ≥14px/1.5 与现有尺寸合约冲突，继续待拍板 |
| D2-3 中西文空格 | i18n 未修改 |
| D2-4 标点/对比度 | 原语义墨保留，默认主题通用 7:1 本轮未重测，不宣称通过 |

用户已定方向与仓库唯一来源优先于网页供体、五签名配额和风格预览规则。Linux 为 ui.Tester 实际事件/PNG，Windows/macOS 仅 vet/build；未冒充三平台真机。无新系统、页面、依赖或终端语义。

## 仍待

1. 下一格先复现命令列表在整行忙态开始/结束时，别名输入和命令开关是否保留可用出口；同时检查未触发写回的别名草稿在重排/重新声明后的恢复。仅代码线索，180ms 与 Flush 契约不改，不重复本轮的重排误派与撤回出口。
2. 运行参数忙态同帧改型已正常，本轮保护测试证明，不当成下一轮 bug。
3. 字号/行盒、accent 禁用填充、未知多选/Radio、极小步长、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾继续待拍板；八条早期 WIP 仍保留。

## 门槛进度

四批两项产品已推送 **3536dbb8 / e4c16e4d / a115f3e9 / e89d8b14**。Linux/Windows/macOS 三平台 vet/build 六项均退出 0，七份 gofmt/vet/build 输出均 **0 字节**；更新设计合约后文档 CI 通过。最终普通/verbose 全量均退出 **0**：**4888 PASS / 0 FAIL**（基线 4821，+67），**3068 测试函数**（基线 3057，+11）。最终三份源码 MD5 全部一致；旧测试函数未减少、旧 V/B/S 断言修改 0/0/0、新 Skip 0。

完整证据：[普通全量](P13-in/final-test.txt)、[verbose 全量](P13-in/final-verbose.txt)、[三平台门槛](P13-in/platform-summary.txt)、[源码校验](P13-in/final-source.md5)。小批提交前使用相关测试与包 vet，完整三平台/全量门槛集中一次；没有用中间结果冒充最终代码验收。

图片 **78 张全幅原帧 / 4 张对照板 / 2 张 4× 局部**。额外已看整卡移除浅色与 orphan 命令深色：前者环落到页头 Close，后者落到 Adopt；原空态/卡片布局未挪位。
