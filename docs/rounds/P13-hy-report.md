# P13-hy：异步结果与条件刷新后的键盘恢复

开盘 405191bc；pull 已最新、工作树干净。独立 Codex 执行。基线实测 4218 PASS / 0 FAIL / 2913 函数。八项真实事件初次复现全部红。

## Stash 清点

八条 SHA 与 P13-hx 相同，逐条导出并 reverse-check 均退出 1。backlog 明确它们是早期非 stale-kill WIP，继续保留原件，不重新合入旧主题/Tauri；历史抢救已接续归档。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 逐项实施（增量）

1. 现象：保存失败插入错误带后 Save 失焦，Space 只派发 1 次。改动：配置动作行独立稳定 key。验证：深浅真实编辑、Tab、Space、延迟失败后保持 Save 焦点，Space 可派发第 2 次。
2. 现象：点击重试后 Saving 与上次 Save failed 同时出现。改动：开始新保存时清旧错误。验证：深浅新回调仍在飞、调用 2 次、错误带消失。

3. 现象：停用中目标损坏，失败后原开关不可用，键盘焦点丢失。改动：原答案仍持焦点时交给同行 Check。验证：深浅延迟回调后 Check 聚焦，Escape 正常退出，调用仍 1 次。
4. 现象：停用中目标变孤立目录，答案与原开关一起消失。改动：原答案持焦点时由 Adopt 接住，结果到达不抢别处焦点。验证：深浅刷新与回调事件，Adopt 聚焦且无自动派发。

5. 现象：移除中目标变孤立目录，原答案消失且结果不能恢复键盘位置。改动：同行 Adopt 接住原移除答案焦点。验证：深浅刷新/失败后 Adopt 聚焦、Escape 关闭恰好一次，移除仅一次。
6. 现象：普通移除仍在飞时目标变 npm，原确认解锁，Space 能打开第二层卸载问题。改动：busy 不再由最新类型豁免。验证：深浅刷新后 Space 不开浮层，原问题仍忙且调用恰好一次。

7. 现象：组件卸载中目标变孤立目录，浮层消失后焦点返到已不存在的原按钮。改动：记录原浮层 handle，同行 Adopt 接住其中的键盘焦点。验证：深浅刷新/回调后 Adopt 聚焦、Escape 可退出，无第二次卸载。
8. 现象：组件卸载失败时目标已变普通链接，失效浮层撤下后焦点悬空。改动：同一 handle 把焦点交给当前 Remove package。验证：深浅延迟失败后入口聚焦、问题撤回、busy 结束、调用一次。

过程偏差：首批 3fbc7069 在完整工作树相关 195 PASS 后提交，但把后续四项测试的注册提前带入，导致该中间提交独立检出会红；本批撤下尚未随产品提交的注册，后续各自随产品补回。旧测试零改，最终全量门槛以完整产品验证。规格也在原型实验后才落盘，均如实记录，不把它们称作符合先规格/独立绿提交的执行。

## 完整门槛与提交

最终产品 **36f11168**；四批 3fbc7069 / 985b3727 / 65e08024 / 36f11168 均推送。最终普通与 verbose 全量退出 0：**4242 PASS / 0 FAIL / 2921 函数**，比基线 +24 PASS / +8 函数。gofmt 输出 0 字节，linux/windows/darwin vet/build 全部退出 0。第二/三/四批相关分别 **183 / 189 / 195 PASS**，包 vet 净。旧 V/B/S 断言改动 **0/0/0**，无新增 Skip。

完整日志在 `/tmp/floter-P13-hy/`；入库 [普通全量](P13-hy/full-test.txt)、[相关事件](P13-hy/related.txt)、[初次复现](P13-hy/before.txt)、[源码 MD5](P13-hy/final-source.md5)。完整检查只在最终产品运行一次，未用重复全量填时间。

## Mutation 实跑

四组均注入后 exit=1，原件复制还原后 `md5sum -c` 四文件全 OK，touch 再跑 exit=0、每组 **6 PASS**。失败为事件断言，不是编译错误。

| 组 | 回注 | 原始报错 |
| --- | --- | --- |
| M1 | 去掉动作 key 与清错 | `save failure insertion lost keyboard retry focus`；`keyboard retry dispatched 1 calls`；`retry still paints the previous failed attempt` |
| M2 | 去掉停用 fallback | `changed disable broken result lost keyboard destination "Check"`；`changed disable orphan result lost keyboard destination "Adopt"` |
| M3 | 去掉移除 orphan 出口、恢复类型 busy 豁免 | `changed remove orphan result lost keyboard destination "Adopt"`；`kind refresh allowed a second removal answer while busy` |
| M4 | 去掉原浮层内焦点判定 | `changed uninstall orphan result lost keyboard destination "Adopt"`；`changed uninstall kind result lost keyboard destination "Remove package"` |

原始 red/restored/md5 均在 [证据目录](P13-hy)。早先另有一次单项 M1 探针红后还原，该次未事先保存 MD5，不计入上述四组正式 mutation。

## 偷了什么

真实读取 qiaomu-design SKILL、user-preferences、craft-loop、preflight、chinese-typography 与 Apple DESIGN。用户指定打磨路径优先，当前 Codex 独立执行，无四方向等待，无 K3。

- craft-loop 工艺菜单 **#3 品牌 focus 环**：取 Apple 的单一交互蓝角色与 **2px** 环，映射本仓 accent；在五条刷新出口恢复该环，交接 **0ms**，不用新高亮装饰。
- **#4 签名交互**：即时反馈落成 **1 次**在飞派发，刷新后 Space 派发 **0 次**额外操作；原有效问题时限仍为 **3000ms**。
- **#12 材质细节**：Apple **8px** 圆角 / **1px** 边映射既有控件刻度；不增加玻璃层。错误重试时提示带 **1 → 0 条**，错误第二层卸载浮面 **1 → 0 层**。

这轮为什么更轻、更有空间感：过时错误不重复占高，在飞操作不再叠新浮层，焦点明确指出当前可操作的一层。未为满足菜单五项添装饰；本轮仅三项适用，字号和材质 token 不重定基线。

## 两轮图像复核

首轮旧事件图：艺术总监看到保存中仍有高对比旧警告；工程师看到错误带插入导致身份漂移；任务用户 Space 无法重试；读者看到普通移除未结束却盖上第二层问题。第二轮集中解决这四类，重拍八项深浅图。

32 张原始 **720×900** PNG 在本目录（before 前缀 floter-P13-hy，after 为简短名称）；[浅色对照](P13-hy/comparison-light.png)、[深色对照](P13-hy/comparison-dark.png)。每组像素变化非零，最小 **420 像素**，详细 [像素差](P13-hy/image-differences.txt)。图是 Linux ui.Tester 软件自绘，Windows/macOS 只有编译检查。

1. save-focus：相同 Space 事件后，旧图仍停在失败，新图已显示 Saving。
2. save-retry：旧图同时有 Saving 与红错误带，新图只留 Saving，动作行上移。
3. disable-broken：浅/深 Check 外均出现现有蓝焦点环。
4. disable-orphan：Adopt 环恢复，原确认不复活。
5. remove-orphan：同目标的 Adopt 接到焦点。
6. remove-componentized：第二个卸载浮层与 scrim 不再出现，原确认仍为禁用忙态。
7. uninstall-orphan：浮层退场后 Adopt 环可见。
8. uninstall-kind：当前 Remove package 的完整焦点环可见。

局部放大用于检查边缘，没有声称高 DPI/真机验证。第一次拼图裁掉动作行且标题错误，已重新生成可读对照；原始 PNG 未改。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹 | 无新增，沿用原主题 |
| A2 字体 | 系统中文与本仓合约优先；不按网页规则换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用原生设置面 |
| A5 卡片海 | 无新增容器 |
| A6 假数据 | 无产品假指标，Workspace Tools 仅事件夹具 |
| A7 空词/感叹号 | 产品文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 原标题无变动 |
| A10 Bento | 不适用 |
| A11 装饰竖线 | 无新增 |
| B1 读取/拨盘 | 3/2/7，克制即时，密度沿用 |
| B2 主题 | 深浅分开指定，无混主题 |
| B3 记忆点 | 错误可重试，当前动作接住键盘 |
| B4 动效气质 | 即时反馈符合任务，无新动画 |
| C1 目的频率 | 高频键盘反馈 0ms |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 沿用既有 120ms/0.97，无改动 |
| C4 origin | 内联问题仍随原对象，modal 原位置 |
| C5 reduced-motion | 新事件均 ReduceMotion=true |
| C6 hover 查询 | Go 自绘非 CSS；保留原指针路径 |
| C7 布局动画 | 无新增 |
| C8 keyframes 重播 | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用 |
| C11 慢放 | 无新增动效，不声称真机慢放 |
| D2-1 系统字体 | 保留，无 webfont |
| D2-2 字号/行高/字重 | ≥14px/1.5 通用规则未满足；Caption 9.9/正文 10.8 与既有字重继续待拍板 |
| D2-3 混排 | 产品 i18n 零改 |
| D2-4 标点/对比 | 沿用原墨，不宣称旧次要墨均达到 7:1 |

缺陷固化到本仓 §6.200 与事件 pin，不改全局 skill 偏好账本。

## 仍待与流程限制

下一轮先事件验证：替代 Check/Adopt 未接线或不可用时的出口，以及用户已主动移焦后刷新不抢焦点。实现用原 handle 的 Focused/FocusWithin 限制，但本轮新增事件只覆盖原答案持焦点，未声称新增别处焦点对照已跑。

失效条件下迟到错误仍按原契约随问题退役；保留新错误面/改变持续时间需拍板。保存期间草稿归属 176-9/161-4、字号/行盒、未列举多选、accent 禁用填充、终端 S1b/S1c/S2 尾继续待拍板。参数全移除后恢复仍未复现。

本轮约 70 分钟；调用次数超过 40 上限，停止新增产品工作并收尾。四批 commit 的 mutation 字段如实标待跑，最终四组已完成；首批未独立绿、规格后落盘偏差见上文，不能把最终全绿等同于过程全部合规。八条 stash 保留，done.log 原路径开盘不存在，本轮新建追加记录。
