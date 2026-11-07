# P13-ip · 刷新后的提示、录制目标与命令草稿

2026-11-07 04:46 UTC 开盘，da989db0，pull 最新、工作树净。Codex 独立执行；无 goal 池也不等待选题。基线 **4937 PASS / 0 FAIL / 3077 函数**。done.log 开盘不存在，收尾创建。

## Stash 清点

SHA 与上轮及 backlog 一致，无新增 stale-kill。逐条 reverse-check 均退出 1；沿已审查结论保留早期独立 WIP，不重新覆盖当前主题/Tauri 已撤销路径。

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

## 实施记录

候选未实测前不计修复。旧 V/B/S 断言修改 0/0/0。同帧输入退出受 ui.Tester 每次事件自动 settle 限制，暂未取得真实批量事件证据，未改退出/写回时机。

### 第一批

1. 现象：别名从 taken 改 available、甚至清空后仍显示旧冲突。改动：Taken 仅在显示值与宿主 alias（TrimSpace 后）一致时显示。验证：深浅真实全选/输入/删除，改值立即清旧提示，改回 taken 保留有效提示；Flush 一次写入空串，180ms 不改。
2. 现象：快捷键命令输入切换内置动作再恢复后显示 say obsolete，Enter 覆盖宿主 say refreshed。改动：内置动作刷新时清对应命令草稿。验证：深浅真实输入、宿主刷新、复制读取、Enter，恢复后严格为 say refreshed。

相关 **94 PASS / 0 FAIL**，包 vet 净。M1 注入开盘两处产品：`old alias conflict labels a different pending draft`、`reopened command resurrected retired draft: "say obsolete"`、`Enter overwrote the new host action`。MD5 恢复、touch 后 **6 PASS**。旧 V/B/S 0/0/0，新 Skip 0。

### 第二批

3. 现象：第二条录制中宿主交换前两条，Ctrl+Shift+R 写进原第一条。改动：录制目标保留 key，绘制及捕获时重新定位。验证：深浅真实 Tab/Enter 启动录制、宿主交换、录入新键，只改原目标的 key，其他两个对象逐字段相同。
4. 现象：录制目标被宿主移除后，下一条仍显示录制态并接受新键。改动：目标 key 不再存在即退役录制状态。验证：深浅事件中原两条与草稿都不改变，录制提示消失。

相关 **34 PASS / 0 FAIL**，包 vet 净；M2 回注上一产品实红：`reordered recorder targets row 1, want 0`、`recorded key reached wrong binding`、`missing binding left a recorder on its neighbor`。MD5 恢复并 touch 后 **6 PASS**。最初新夹具 Tab 到第三条，已改为从命令框反向三次 Tab 确认第二条，再取真实产品失败；不使用夹具失败计修复。

### 第三批

5. 现象：录制重复键产生冲突后，宿主交换绑定，提示跟随旧行号落到另一条。改动：冲突保留绑定 key，刷新时重新定位。验证：真实录入 Ctrl+Shift+A 制造冲突，再重排，原对象仍有冲突、相邻对象无提示。
6. 现象：冲突所属绑定被宿主移除后，相邻行仍显示冲突。改动：冲突目标不再存在即退役。验证：深浅真实重复键与移除事件，提示及 customDuplicate 均清空，其他绑定未改。

相关 **40 PASS / 0 FAIL**、包 vet 净；M3 回注旧产品实红：`conflict followed row index instead of binding: row=1`、`missing binding left a conflict on its neighbor`。MD5 恢复、touch 后 **6 PASS**。旧 V/B/S 0/0/0。

### 第四批

7. 现象：输入 say obsolete 后宿主删除该绑定，再以同 key 新建 say new，旧草稿复活且 Enter 覆盖新值。改动：刷新时清除没有所属绑定的命令草稿。验证：深浅删除/重建后真实全选复制严格为 say new，Enter 不会恢复旧值。
8. 现象：命令尚在编辑，重新录制原 Ctrl+Shift+B 后，say unfinished 变回 say original。改动：等价 chord 保留草稿，规范化拼写时转移到返回 key；不同键仍沿原清理契约。验证：深浅真实录制后复制严格为 say unfinished，存储仍为 say original，仅 Enter 才提交草稿。

相关 **99 PASS / 0 FAIL**，包 vet 净；M4 回注旧产品实红：`recreated binding resurrected deleted command draft: "say obsolete"`、`same-key recording discarded unfinished command: "say original"`。MD5 恢复并 touch 后 **6 PASS**。八项跨命令别名与自定义快捷键组件，全部有行为红绿；没有用文案修改凑项。

四批产品已推送：**bd7907c3 / ab6e3114 / 457f536e / 46c5d0be**。每批两项经相关事件与包 vet 后提交；完整门槛收尾，不把相关数字冒充全量。

## 偷了什么

本轮实际读 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography，及 Apple DESIGN 的角色表。用户已指定方向，直接打磨；不用四方向网页、不调 K3、不委派。

- craft-loop 工艺菜单 **#3 品牌 focus 环**：沿 Apple 单交互色角色，保留仓库 **2px** 环与原蓝色；录制重排 **0ms** 跟随原对象，额外焦点层 **0**。
- **#6 编辑细节**：保留 **1px** 发丝线与别名 **4px/8px** 内距；失主冲突行 **0** 条，错误挂到邻行的提示 **0** 条，草稿不在帧刷新时倒退。
- **#12 材质细节**：沿原命令板 **8px** 圆角、别名 **6px** 圆角和受光边；新增阴影/透明面 **0** 层。不为凑满五个签名动作加入装饰，优先任务书的已有功能修复。

为什么更轻、更有空间感：提示和焦点落回所属行；过时的错误句退场后，原有面与空隙重新承担分组，不需要额外状态面解释输入去了哪里。

## 图像证据与两轮复核

32 张原帧、16 张对照板、总览与像素差表均进仓。Linux ui.Tester 720px 窗口真实事件自绘；不是原生桌面截图，Windows/macOS 只有 vet/build。

第一轮四视角：艺术视角发现别名槽下有与输入无关的提示；工程视角发现重排后录制目标错位；任务视角发现 Enter 能把宿主新命令覆盖掉；读者视角发现冲突句指向无冲突的邻行。第二轮沿这些点核对全部深浅 before/after，未加装饰或推倒布局。

目视浅色冲突局部，红色句从 Ctrl+Shift+A 行移动到实际发起录制的 Ctrl+Shift+B 行；深色别名局部，available 槽下的旧冲突句消失，卡片下边回到原内容高度，焦点环和圆角没有重影。八项两外观像素差均非零，最小 **356 像素**（命令改型后的草稿恢复）。[总览](P13-ip/overview.png)、[像素差](P13-ip/image-differences.txt)。

| 项 | 浅色前后 | 深色前后 |
| --- | --- | --- |
| 1 别名提示 | [图](P13-ip/alias-feedback-light-comparison.png) | [图](P13-ip/alias-feedback-dark-comparison.png) |
| 2 命令改型 | [图](P13-ip/command-type-refresh-light-comparison.png) | [图](P13-ip/command-type-refresh-dark-comparison.png) |
| 3 录制重排 | [图](P13-ip/recorder-reorder-light-comparison.png) | [图](P13-ip/recorder-reorder-dark-comparison.png) |
| 4 录制消失 | [图](P13-ip/recorder-removed-light-comparison.png) | [图](P13-ip/recorder-removed-dark-comparison.png) |
| 5 冲突重排 | [图](P13-ip/conflict-reorder-light-comparison.png) | [图](P13-ip/conflict-reorder-dark-comparison.png) |
| 6 冲突消失 | [图](P13-ip/conflict-removed-light-comparison.png) | [图](P13-ip/conflict-removed-dark-comparison.png) |
| 7 删除重建 | [图](P13-ip/command-removed-return-light-comparison.png) | [图](P13-ip/command-removed-return-dark-comparison.png) |
| 8 同键录制 | [图](P13-ip/command-same-key-light-comparison.png) | [图](P13-ip/command-same-key-dark-comparison.png) |

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹 | 无新增，沿原主题 |
| A2 字体 | 保留中文系统栈，用户既定方向优先于每轮换字体规则 |
| A3 斜体 | 无新增 |
| A4 Hero | 原生设置页，无 Hero |
| A5 卡片模板 | 新容器 0，保留原行板 |
| A6 假数据 | 工具与命令名仅本地事件夹具 |
| A7 文案空词 | 产品 i18n 零改 |
| A8 眉批 | 新增 0 |
| A9 标题 | 原页头与行盒不变 |
| A10 网格空洞 | 无新布局，退役错误句收回原行高 |
| A11 装饰竖线 | 新增 0 |
| B1 拨盘一致 | 2/1/6，稳定、即时、原密度 |
| B2 明暗 | 深浅独立 Tester，单图不混主题 |
| B3 记忆点 | 正在录制/编辑的对象与提示一致 |
| B4 动效气质 | 新动画 0，状态即时落位 |
| C1 目的频率 | 高频键盘处理即时 |
| C2 all/ease-in/scale/fade | 无新增 |
| C3 时长/按压 | 0ms 状态更新，原按压沿用 |
| C4 origin | 无新浮层 |
| C5 reduced-motion | 新事件全部 ReduceMotion=true |
| C6 hover 媒体查询 | Go 自绘非 Web，原指针路由保留 |
| C7 布局动画 | 新增 0 |
| C8 keyframes | 新增 0 |
| C9 手势 | 本轮无手势新增 |
| C10 Motion shorthand | 非 Web，不适用 |
| C11 慢放 | 无新动画，逐帧事件与静帧复核；未声称真机慢放 |
| D2-1 中文字体 | zh_CN.UTF-8，系统 CJK，无 webfont |
| D2-2 字号/行高/字重 | 既有 9.9/10.8 与通用 ≥14px 冲突，仍待拍板，不宣称达标 |
| D2-3 混排 | i18n 零改 |
| D2-4 标点/对比 | 原标点/墨值沿用，通用 7:1 本轮未重测 |

可复现问题沉淀到合约 §6.217 与事件 pin；不写成新的全局审美偏好。

## 仍待

1. 下一轮先事件复现快捷键冲突的另一端变化：发生重复之后，对方绑定被移除或改键，旧冲突是否仍挂着；以及宿主回写命令字符串后的草稿展示。仅代码线索，不改保存时机。
2. 同帧输入后退出尚无批量原生事件证据；ui.Tester Type/Key 各自 settle，不能假装测过同帧。180ms/Flush 与退出时机未改。
3. 字号/行盒、accent 禁用填充、未知多选/Radio、极小步长、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾继续需拍板。原生 macOS Option/输入法与 Windows 真机未测。
4. 八条早期 WIP 继续保留，未新建抢救 stash。

## 完整门槛与交接

最终产品 **46c5d0be**。普通与 verbose 全量均退出 **0**；实测 **4961 PASS / 0 FAIL**（基线 4937，+24），测试函数 **3085**（基线 3077，+8）。gofmt 空，Linux/Windows/macOS vet/build 六项退出 0、原始日志均 0 字节。文档 CI 通过。旧 V/B/S 断言改动 **0/0/0**，旧测试零删除、新 Skip **0**，最终产品源码 MD5 校验一致。

| 变异 | 注入后实测 | MD5 恢复、touch 后 |
| --- | --- | --- |
| M1 旧提示/改型草稿 | 6 FAIL | 6 PASS |
| M2 录制行号 | 6 FAIL | 6 PASS |
| M3 冲突行号 | 6 FAIL | 6 PASS |
| M4 失主草稿/同键清理 | 6 FAIL | 6 PASS |

[普通全量](P13-ip/final-test.txt)、[verbose 全量](P13-ip/final-verbose.txt)、[门槛汇总](P13-ip/gates.txt)、[源码 MD5](P13-ip/final-source.md5)。完整基线保存在 /tmp/floter-P13-ip/baseline.log；三平台原始日志同目录。

04:46 UTC 开盘，05:10 UTC 完成全量与交接，约 24 分钟，短于 45–75 分钟目标；八项已验收，不用无关改动补时。计入等待回收的工具调用超过 40，超过后只完成已有工作门槛、归档与交接，没有扩展新选题。单轮全量普通/verbose 各跑一次，包级回归与 mutation 单独计数。
