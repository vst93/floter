# P13-jj · 传输结果的当前状态与过期反馈

起点 17a5059f，分支 mygo-rewrite，pull ff-only 完成，开盘干净。采用 qiaomu-design 的已有方向打磨路径。规格见 P13-jj-spec.md。本报告增量记录，未完成验证不标通过。

## Stash 清点

无活动 stale-kill；八条均与 P13-ji SHA 相同，reverse-check 退出 1。前轮已逐项区分为早期独立 WIP，继续保留，不重新合入旧主题与旧 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap WIP，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧设置间距/slider WIP，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/scratch WIP，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧插件数字框 WIP，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件样式 WIP，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 WIP，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏 WIP，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri WIP，reverse 1 | 保留 |

## 增量验收

1. 现象：同一已安装 ID 两次导入，配置按最后一条合并，但 enabled 留在中间状态，仍显示成功两项。改动：每条从当前 repository 读取 enabled；不改顺序与配置合并。验证：真实点击导入，深浅 × 初始开/关四场景，磁盘开关/两字段合并/成功计数均断言；原红 `repeated import reports success but enabled=true, want false`（另一极性同样红）。
2. 现象：同脚本较早选择器晚报失败，当前失败原因被旧原因覆盖。改动：共享错误槽拒绝同 owner 的较早版本。验证：两个真实异步选择器反序返回，深浅确认较新错误保留；原红 `script-late-failure: obsolete failure replaced current result`、`latest failure disappeared`。

第一批修前日志 batch1-red.txt 实红；修后门槛数字随提交记录。旧 V/B/S 断言修改 0/0/0。后六项 helper 仅准备，尚未计完成。

全量基线实测 5591 PASS / 0 FAIL、退出 0；测试函数 3284。第一批初验有一场新夹具滚动后未派发点击，旧 B 测试未红；增加点击前真实几何复核后再验，原日志保留，不把夹具失败算产品问题。

第一批 fb086bf1 已提交推送，相关 **18 PASS / 0 FAIL**，包 vet 净。提交正文误写 17，原日志为 18，以此处及 batch1-retest.txt 实测为准；不重写已推送历史。

3. 现象：集成导出已成功，再次导出选择器失败，上方仍留“已导出”成功浮面。改动：成功记录动作归属，后续同动作失败同步退役旧成功。验证：真实 Export 点击，深浅 `failed export retains obsolete success` 修前红；修后只保留当前原因，原备份仍在。
4. 现象：配置复制成功后输入无效文本重试，旧“已复制”与 UTF-8 错误同时显示。改动：复制成功带工具 ID 与动作归属，仅自身失败可以退役。验证：真实 Actions 交付剪贴板，再无效 UTF-8 拒绝；深浅 `failed copy retains obsolete success` 修前红，修后当前字段原因可读。

第二批相关 **66 PASS / 0 FAIL**、shell vet 净；旧通知/新恢复/跨目标测试均通过。亲眼复核：浅色导出原先上下两层成功和错误，现在只剩错误一层；深色脚本只显示较新 picker 原因。截图背景工具发现与滚动位置有差异，不能当作纯墨值比较；通知区域是本轮有效变化。

第二批 e6918d61 已提交推送。

5. 现象：配置导出成功后重试选择器失败，“配置已导出”继续压在当前错误上。改动：配置文件成功通知带配置动作和工具 ID。验证：深浅 `failed config-export retains obsolete success` 修前红；修后只保留失败，第一次写成的 JSON 与密码占位符不变。
6. 现象：脚本导出成功后再失败，旧“脚本已导出”仍在。改动：脚本成功通知记录脚本名归属。验证：两个真实异步选择器串行成功/失败，深浅 `failed script retains obsolete success` 修前红，修后错误可见，原脚本文件保留。

第三批首次相关运行 24 PASS，有一条前轮恢复测试在真实主机工具发现插入行时点击未派发。定位到 transferCauseFrame 接入了与传输无关的异步 Detected 列表；夹具固定为不显示主机候选，安装的真实 provider、点击与全部旧断言不动。该辅助夹具变更不计产品修复、不改 B 断言、不放宽容差。复验数字见下一记录。

第三批复验 **26 PASS / 0 FAIL**，shell vet 净；源码与旧行为断言检查通过。7b5cee7d 已推送；提交正文误写 25，原始 batch3-retest.txt 为 26，报告已纠正，不改写已推送提交。

7. 现象：配置导出较早选择器失败，在较新导出已经写成后才返回，旧错误重新出现。改动：同成功槽记住当前完成版本，拒绝旧失败；先登记成功再解除旧错误，闭合回调交错窗口。验证：真实两个选择器反序返回，深浅修前 `config-late-success: obsolete failure replaced current result`。
8. 现象：脚本导出同样在新文件已写成后冒出旧 picker 错误。改动：同脚本版本保护，旧成功回调不降低记录版本。验证：真实并发脚本导出、磁盘文件存在，深浅修前 `script-late-success: obsolete failure replaced current result`。写入与回调照常完成，只修反馈归属。

第四批 **acff98ae** 已提交推送，相关 **48 PASS / 0 FAIL**，shell vet 净。八项完成，跨集成传输、配置工具条、脚本导出和通知组件；完整门槛正在准备。

第二轮 **15 PASS / 0 FAIL**：未绘制的旧成功被退役、其他动作/其他目标成功保留、无归属成功保留、关闭队列后新成功仍可见、旧成功不倒退版本、中文 720×520 深浅状态，以及重复导入四场景。新的 import 图滚到真实 Configured 卡，可直接看到开关与 Running/Disabled，避免只截报告却看不到结果。

## 偷了什么

- 从 craft-loop.md 工艺密度菜单取 **#4 签名交互**：有效结果交接新增等待 **0ms**，旧成功/错误的自然阅读窗口仍为 **4000/8000ms**，不以动画延迟事实。
- 取 **#6 编辑细节**：同一结果同时显示原因 **2→1**（旧/新失败），同一操作矛盾的成功/失败 **2层→1层**；原 i18n 标题、正文与错误 cause 保留，不另建说明卡。
- 取 **#5 多层阴影 / #12 材质细节**：复用既有 E2 的 **2层投影、1px 顶受光边**，新增玻璃 **0层**。这些值是沿用，不冒称重新调色。

为什么更轻、更有空间感：撤掉已经失效的浮面，让当前结果只占必要的一层，露出被旧提示挡住的表单。按用户范围不凑五项装饰；Apple 供体的单色角色、近白地面/近黑正文映射到既有 token，未更换字号。

## 两轮审查

第一轮四视角：艺术总监看到相互矛盾的双浮面；工程师看到 enabled 和按条合并的配置脱节；任务用户不知道该重试哪个错误；读者受旧失败原因误导。第二轮聚焦归属、事件先后、真实卡片状态与中文小窗口，增加以上 15 PASS，产品已通过八项定向门槛。

## Mutation 实跑

- M1 把逐条当前值比较退回开盘 inventory：四场景实红，原串 `repeated import reports success but enabled=true, want false` / `enabled=false, want true`。还原 MD5 OK，touch 后 **5 PASS / 0 FAIL**。
- M2 在 queueOwnedToast 清空归属：七入口/情形两主题实红，原串 `failed export/copy/config-export/script retains obsolete success`、`script-late-failure: obsolete failure replaced current result`、`config-late-success: obsolete failure replaced current result`、`script-late-success: obsolete failure replaced current result`。还原 MD5 OK，touch 后 **21 PASS / 0 FAIL**。

两组红退出 1，恢复后退出 0，逐字节校验记录见 [M1](P13-jj/M1-restore.txt) / [M2](P13-jj/M2-restore.txt)，完整原错误见对应 M1-red.txt / M2-red.txt。普通旧行为测试未删改断言，新增 Skip 0。

## 深浅证据

图为 Linux ui.Tester 软件自绘（720×1000），中文小窗口 720×520。before 来自真实注入缺陷，after 来自还原后的同一夹具；主机发现现已固定，临时文件路径仍可能不同，不把整图差异冒称纯视觉墨值。每组像素变化记录见 [image-differences.txt](P13-jj/image-differences.txt)。

| 项 | 浅色 | 深色 |
| --- | --- | --- |
| 1 重复导入开关 | [图](P13-jj/repeat-enabled-light-comparison.png) | [图](P13-jj/repeat-enabled-dark-comparison.png) |
| 2 较新脚本失败 | [图](P13-jj/script-late-failure-light-comparison.png) | [图](P13-jj/script-late-failure-dark-comparison.png) |
| 3 集成导出旧成功 | [图](P13-jj/failure-export-light-comparison.png) | [图](P13-jj/failure-export-dark-comparison.png) |
| 4 配置复制旧成功 | [图](P13-jj/failure-copy-light-comparison.png) | [图](P13-jj/failure-copy-dark-comparison.png) |
| 5 配置导出旧成功 | [图](P13-jj/failure-config-export-light-comparison.png) | [图](P13-jj/failure-config-export-dark-comparison.png) |
| 6 脚本导出旧成功 | [图](P13-jj/failure-script-light-comparison.png) | [图](P13-jj/failure-script-dark-comparison.png) |
| 7 配置旧失败晚到 | [图](P13-jj/config-late-success-light-comparison.png) | [图](P13-jj/config-late-success-dark-comparison.png) |
| 8 脚本旧失败晚到 | [图](P13-jj/script-late-success-light-comparison.png) | [图](P13-jj/script-late-success-dark-comparison.png) |

代表图 `/tmp/floter-P13-jj-transfer-{light,dark}.png`。通知细节图以最近邻 2× 放大便于核对，不冒充真实高 DPI。

最终目视：浅色重复导入对照里，左侧明明写 Imported 2 却为 Disabled/灰开关，右侧为 Running/蓝开关且配置入口恢复；深色配置导出对照左侧两块矛盾浮面，右侧仅当前失败；中文深浅 720×520 的错误全文两行可读，关闭按钮完整。**18 组对照像素差均非零**，其中重复 ID 的页面高度变化来自真实启停状态与配置入口，不冒称间距改版。中文图：[浅](P13-jj/chinese-compact-light-after.png) / [深](P13-jj/chinese-compact-dark-after.png)。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 紫蓝渐变/纯黑/霓虹/渐变字 | 未新增，沿用原深浅材质；透明窗外不是纯黑产品面 |
| A2 非系统字体 | 服从原生 Go 与中文合约，保留系统字体，不执行网页换字体建议 |
| A3 斜体 | 无新增，原中文不伪斜 |
| A4 Hero | 不适用设置功能面 |
| A5 卡片海 | 新卡片/页面 0 |
| A6 假数据 | 产品未加假指标，测试使用真实 POSIX provider 夹具 |
| A7 空词/感叹号 | 无新增产品文案，保留 i18n 标题与真实原因 |
| A8 眉批 | 新增 0 |
| A9 标题行数 | 原标题保持单行，中文小窗口已导出 |
| A10 Bento | 不适用 |
| A11 左色条 | 新增 0 |
| B1 读取/拨盘 | 2/1/6，原布局与密度，即时交接 |
| B2 主题锁定 | 深浅独立 Tester，无同页混主题 |
| B3 记忆点 | 最后导入状态正确，当前反馈不被旧结果盖住 |
| B4 气质 | 高频反馈不加动画，保持克制 |
| C1 动效目的/频率 | 新等待 0ms，原通知时长不改 |
| C2 all/ease-in/scale(0)/fade | Go 自绘无 CSS，未新增这些效果 |
| C3 ≤300ms/按压 | 原 220ms toast 入场与控件按压不改 |
| C4 origin | 原右上浮面位置不改，无新弹窗 |
| C5 reduced motion | 深浅及中文截图实际 ReduceMotion=true |
| C6 hover | 必需结果不依赖悬停，无 CSS 媒体查询 |
| C7 transform/opacity | 未新增布局或其他属性动画 |
| C8 快速重触发 | 复用原 toast ID 交接，不重播新关键帧 |
| C9 手势 | 不适用，本轮无手势改动 |
| C10 Motion | 不适用，未引入依赖 |
| C11 慢放 | 无新动效；没有慢放/真机帧率证据，不冒称验证 |
| D2-1 中文系统字体 | 沿用原系统字体，中文深浅图复核 |
| D2-2 ≥14px/1.5行高 | 仍未满足通用建议；Caption 9.9/正文10.8/行盒为已登记拍板项，用户禁止擅改 |
| D2-3 混排空格 | 未新增中文文案，现有 i18n 保留 |
| D2-4 标点/7:1 | 原标点不改；未重新审计全套 AAA，默认次要墨不冒称全部达到7:1 |

用户本轮修复边界优先于通用 skill 的全项通过措辞；不足如上明示。可复现状态问题沉淀到 §6.237 与事件测试，不修改全局审美偏好。

## 仍待

1. 下一轮首选：导入失败与部分生效时，现有配置面板的已保存值/错误是否跟随外部导入刷新；先真实事件复现，不改变编辑期间草稿归属或保存时机。仅线索，不冒称缺陷。
2. 重复新包 ID 的包覆盖规则、安装模板失败后重试是否补跑仍需拍板；本轮只修已有 ID 的顺序状态，不重装/回滚/重跑模板。
3. 既有多草稿、转换覆盖、字号/行盒、accent 禁用填充、未知多选/Radio、176-9/161-4 保存期归属、终端 S1b/S1c/S2 尾沿用待定。原生选择器与 Windows/macOS 真机未验，八条早期 WIP 保留。

4. 本轮并行全量遇到旧 `TestTheImageCopyLeavesTheDrawingThread` 的时序假设：后台 goroutine 可能在调用返回后立即完成，测试的非阻塞 channel 读取会误判为绘制线程执行。产品 `copyClipboardEntry` 的 `go func` 路径本轮字节未改，普通全量通过；保留旧 B 断言，后续需要以明确同步验证异步边界，不能靠增加等待时间或放宽断言解决。

验收过程如实记录：首个汇总进程退出143、未完整结束，随后普通全量退出0，verbose 为5626 PASS/1 FAIL（上述旧时序测试）。这次不能算完整通过，已停止扩项，独立重跑完整 verbose。调用预算超过40批，超出部分只处理已有验收与交接，未追加产品修复。所有失败与中断日志保留。

## 完整门槛与交接

| 门槛 | 实测 | 证据 |
| --- | --- | --- |
| 基线全量 | 5591 PASS / 0 FAIL，退出0 | [日志](P13-jj/baseline-verbose.txt) |
| 最终普通全量 | 退出0 | [日志](P13-jj/full-test-retry.txt) |
| 最终 verbose | **5627 PASS / 0 FAIL**，增加36，退出0 | [日志](P13-jj/full-verbose-final.txt) |
| 测试函数 | **3294**，基线3284，增加10 | [计数](P13-jj/test-functions.txt) |
| gofmt | 输出0字节 | [日志](P13-jj/gofmt.txt) |
| 三平台vet/build | 六项退出0 | [汇总](P13-jj/gates.txt) |
| Mutation | 两组实红、MD5 OK，touch后5/21 PASS | M1/M2原始日志 |
| 旧V/B/S断言 | 0/0/0；新增Skip 0 | git diff 17a5059f |

旧剪贴板测试单独检查为1 PASS，最终完整 verbose 为5627 PASS/0 FAIL；它的调度时序隐患仍如实留在“仍待”，不以偶然绿掩盖。普通与最终 verbose 对应同一产品/测试源码；失败后未修改剪贴板产品、未弱化断言、未新增等待。最终产品仍为第四批 acff98ae；后续只补第二轮验收、合约、图片与报告。

过程偏差已保留：两次提交正文少记1 PASS、点击夹具发现源不稳定、首次完整汇总中断、旧剪贴板测试偶发红，以及超过40调用预算。全部修复有前后事件证据；不冒称完整门槛只运行一次。实际用时未达到45分钟目标，不以重复检查或装饰补时长。八项产品修复、两组变异及完整门槛已完成。
