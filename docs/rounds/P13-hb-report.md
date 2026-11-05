# P13-hb · 本地工具草稿、编辑焦点与内容反馈

起点 `72d291ff`，2026-11-05；工作树干净，pull --ff-only 已同步。当前 done.log 不存在，收尾创建。执行者为当前 Codex，无外部模型、无子代理。

## stash 开盘清点

活动 stash 8 条，全部实际 reverse-check 失败；均为早期非 stale-kill WIP。P13-gq/ha 已归档清理本任务书所指抢救稿，此次没有新 orphan。以下沿用已入库语义审核，不恢复旧主题/发布链，也不把 reverse-check 失败误报为独有改动。

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | P7 keycap 与旧主题稿 | 保留原件 |
| stash@{1} / 8af94a07 | 设置行距/滑块旧刻度 | 保留原件 |
| stash@{2} / 7e4aa710 | 卸载旧布局与 scratch 探针 | 保留原件 |
| stash@{3} / 1e3aec94 | 旧数字框与透镜稿 | 保留原件 |
| stash@{4} / 363f5a42 | 旧确认带/快捷键稿 | 保留原件 |
| stash@{5} / 9b8ce23c | P7 快捷键拒绝反馈稿 | 保留原件 |
| stash@{6} / 90d314b2 | action bar 旧键盘合约 | 保留原件 |
| stash@{7} / 7a9f6e13 | 旧 Tauri 发布链 | 保留原件 |

基线：中文 locale 全量 **3592 PASS / 0 FAIL**，函数数 **2711**（`^func Test` 同口径）。首次 C locale 字体 pin 红，已按 AGENT-NOTES 使用中文 locale 复核，未修改断言。下文逐批增量记录实测，不预填最终门槛。

## 已完成（逐条）

1. **删除脚本又冒出模板**：全选删除后两帧回到 264 字节模板，提交甚至成功一次 → 模板只在首次新建初始化，用户清空保持空 → 深浅两套实际键盘删除后为空，提交 0 次并显示既有空脚本错误。原始红串：`deleted script was repopulated: 264 bytes`、`empty script submit: calls=1 error=false`。
2. **撤销窜回上一工具**：编辑第二工具时 Undo 出现 `first edited` → beginEdit/endEdit 释放旧 editor.State，载入稿由 NewBuffer 初始化 → 第二工具无历史撤销，自己的新编辑仍可撤销。原始红串：`new tool inherited undo: "first edited\n"`、`undo escaped current tool: "first edited\n"`。

首批 V/B/S = 0/0/0，旧测试零改。红测日志 `/tmp/floter-P13-hb-pair1-before.log`，通过日志 `pair1-after.log`。深浅截图已通过 ui.Tester 导出，图上可见脚本编辑器从自动补回的模板回到仅第 1 行空稿；完整尺寸和错误验证保留。

首批定向回归 **24 PASS / 0 FAIL**；internal/ci 通过。旧 copy/export 回归曾发现点击切换到 Script 的同帧未初始化模板，已将一次初始化归到 editor 创建路径修复，未改旧断言。

3. **新建工具继承后台输出**：上一工具选后台输出并完成后，新表单仍选后台 → endEdit 清除 toolOutput → 旧工具收到 background，新表单恢复终端默认。原始红串：`fresh connection inherited output "background"`。
4. **另一工具未经复制就显示已复制**：复制第一工具后切到第二工具，按钮仍确认成功 → beginEdit/endEdit 清除旧确认与到期时间 → 深浅两主题新工具都显示可复制标签。原始红串：`new tool inherited copied confirmation`、`completed tool kept copied confirmation`。

第二批相关回归 **21 PASS / 0 FAIL**，V/B/S 0/0/0，旧断言零改。第一批提交 `9e32937d` 已推送。

5. **脚本文字上下重叠**：截图发现字号 10.8px、实际行距仅 8px，中英文行墨互相覆盖 → 编辑器内部改为字号 ×1.5 向上取整（默认 17px），外框高度不动 → 深浅两套 LineTop 与截图确认四行不再重叠。原始红串：`script line advance 8.00 is below readable 16.20`。这是截图发现的真实可读性问题，替换规格中尚未证实的主表单焦点候选；不改变面板高度链。
6. **修改内容仍声称已复制**：复制 weather 后改成 forecast，按钮仍显示成功 → 确认同时绑定 action 与复制值，值不同立即清除 → 深浅两套旧实现 `changed content still claims copied` 变绿；保持原 1600ms 期限和按钮宽度。

第三批相关回归 **33 PASS / 0 FAIL**，V/B/S 0/0/0。未改旧断言。前两批 `9e32937d`、`5ebc90b6` 均已推送。

7. **删前一参数后丢掉剩余输入的焦点与选择**：第二项 Label 已全选，前一项移除后输入不再落到第二项 → 参数板分配只存内存的稳定身份，不使用可编辑 ID 或数组下标 → 原始红串 `remaining parameter lost edit: "Second"`，修后值恰为 `Remaining`。
8. **类型切换把默认值输入写进 flag**：全选 Default 后将参数变成布尔，placeholder 消失使后续输入按序号串位 → 每个字段按自己的标签设定身份 → 原始红串 `kind switch moved default edit: default="old" flag="truearget"`，修后 Default=`true`、Flag=`--target`。

第四批相关回归 **34 PASS / 0 FAIL**，V/B/S 0/0/0，全部旧断言零改。至此八项实际修复，跨脚本编辑器、表单、参数板和复制动作四个组件。

## 正式 mutation

- M1 恢复每帧空脚本模板填充：两主题红，`deleted script was repopulated: 264 bytes`；提交守卫也红，`empty script submit: calls=1 error=false`。退出 1。
- M2 删除 beginEdit 的 editor 状态重置：`new tool inherited undo: "first edited\n"`、`undo escaped current tool: "first edited\n"`。退出 1。

每次恢复后 `md5sum -c /tmp/floter-P13-hb-product.md5` 三文件均 OK，并 touch 改动文件后启动 `-count=1` 回归。原始日志 `/tmp/floter-P13-hb-M1.log`、`M2.log`；没有通过测试缓存消红。

恢复后新增回归 **16 PASS / 0 FAIL**。MD5：settingsui.go `1a9f57efae3da7bf447373dee8743b6f`，scripteditor.go `cf85ab192cf4f13ca9d1a2461772d4f0`，parameditor.go `7902a11b95c492b173e2681610c3ac54`。

## 偷了什么

已实际读取 qiaomu-design 的 SKILL、用户偏好、craft-loop、preflight、chinese-typography 及 Linear 供体。设计读取为原生紧凑工具表单，拨盘 2/2/6；用户指定唯一方向，直接走打磨路径。

- 从 craft-loop 工艺密度菜单 **#3 品牌 focus 环**取「焦点必须对应当前控件」：参数板与字段两层身份，始终只有 1 个输入拥有焦点；原有主题焦点墨保留。
- 从 **#4 签名交互**取有明确对象的反馈：确认期限仍为 **1600ms**，切工具或内容改变即复位为 **0 个旧确认**；不增加常驻提示。
- 从 **#6 编辑细节**取可读行节奏：脚本字号 **10.8px**，行距 **8px → 17px**；外框仍为 `Space(18)`。先发现实际像素重叠，再调整唯一行距来源。
- **#9 滚动条**沿用编辑器原生细条；新的 editor.State 不继承上一文档的滚动和选择。未另造滚动条，未把沿用项计成第九项修复。
- **#12 材质细节**沿用 **1px ControlEdge** 与现有 E1 受光边；这一轮不增加高光和投影，未把保留值冒充新增工艺。

Linear 供体的三档中性文字、代码等宽字族、1px 细边映射至项目既有 token。没有照搬紫色、营销字阶或 Web blur。菜单只有前三项有本轮直接改动，其余为保留与核对；不为凑工艺数量添加装饰。

## 深浅图与两轮看图

全部原图由 ui.Tester 导出，720×1500；复核用局部比较图直接取原始像素，上为 before、下为 after。原图及对比图均随报告入库。

| 场景 | 深色 | 浅色 |
|---|---|---|
| 脚本行墨重叠 | [前](P13-hb/floter-P13-hb-script-lines-dark-before.png) / [后](P13-hb/floter-P13-hb-script-lines-dark-after.png) / [局部对比](P13-hb/script-lines-dark-compare.png) | [前](P13-hb/floter-P13-hb-script-lines-light-before.png) / [后](P13-hb/floter-P13-hb-script-lines-light-after.png) / [局部对比](P13-hb/script-lines-light-compare.png) |
| 删除脚本 | [前](P13-hb/floter-P13-hb-empty-script-dark-before.png) / [后](P13-hb/floter-P13-hb-empty-script-dark-after.png) | [前](P13-hb/floter-P13-hb-empty-script-light-before.png) / [后](P13-hb/floter-P13-hb-empty-script-light-after.png) |
| 新工具复制确认 | [前](P13-hb/floter-P13-hb-new-tool-copy-dark-before.png) / [后](P13-hb/floter-P13-hb-new-tool-copy-dark-after.png) | [前](P13-hb/floter-P13-hb-new-tool-copy-light-before.png) / [后](P13-hb/floter-P13-hb-new-tool-copy-light-after.png) |
| 修改内容后的确认 | [前](P13-hb/floter-P13-hb-changed-content-dark-before.png) / [后](P13-hb/floter-P13-hb-changed-content-dark-after.png) | [前](P13-hb/floter-P13-hb-changed-content-light-before.png) / [后](P13-hb/floter-P13-hb-changed-content-light-after.png) |

第一轮四视角：艺术——脚本密度像重影；工程——参数类型变换后焦点串位；任务——删空内容被偷偷补回、错误提交；读者——修改内容却仍被告知已经复制。第二轮落地行距、身份、初始化与确认归属四项修正。自行看图：浅色脚本的 FIRST/SECOND 从相互压线变成完整分开的行；深色第四行仍在固定编辑框里，下方 Command 行未移位。

为什么本轮更轻、更有空间感：行墨不重叠，焦点与反馈留在当前对象上；保留既有面差与材质，不再用错误的状态抢注意力。

## preflight A / B / C / D2

| 条目 | 结论 |
|---|---|
| A1 渐变/纯黑/霓虹 | 未新增，沿用项目近黑/近白与中性面 |
| A2 更换非系统字体 | 不采用；用户既有合约与中文系统字体优先，不能每轮换字族 |
| A3 斜体 | 无新增 UI 斜体 |
| A4 Hero | 不适用，原生编辑表单 |
| A5 卡片模板 | 保留原分组，没有新增卡片 |
| A6 假名/假数字 | 产品无假数据，Weather 等仅测试夹具 |
| A7 空词/感叹号 | 产品 i18n 文案未改 |
| A8 编号眉批 | 无新增 |
| A9 H1 | 无营销标题，不适用 |
| A10 网格空洞 | 参数原有自适应网格保留，类型切换只修身份 |
| A11 装饰侧线 | 无新增 |
| B1 读取/拨盘 | 2/2/6 与现有轻玻璃工具界面一致 |
| B2 主题 | 深浅分别渲染与核对，无混色 |
| B3 记忆点 | 内容、焦点、反馈各归当前对象 |
| B4 动效气质 | 保留短反馈，未新增动效 |
| C1 目的/频率 | 高频输入不加动画 |
| C2 all/ease-in/scale(0) | 无新增 |
| C3 时长/按压 | 既有 120ms 控件反馈保持；1600ms 为复制文字驻留时间，不是动画 |
| C4 origin | 未增弹层，不适用 |
| C5 reduced-motion | 测试采用 ReduceMotion，未修改框架降级 |
| C6 hover 媒体条件 | 原生指针系统，不适用 Web 媒体查询 |
| C7 属性 | 无布局动画 |
| C8 快速触发 | 保留 transition，无重播 keyframes |
| C9 手势 | 无新增手势 |
| C10 Framer Motion | 不适用 |
| C11 真机/逐帧 | ui.Tester 验证静态帧与输入序列；未宣称真机 GPU 或动态播放验收 |
| D2-1 字体栈 | 中文系统字体与代码 monospace 保留，无字体下载 |
| D2-2 行高/字号/字重 | 脚本行距已达 1.5；全局 Body 10.8px、Caption 9.9px 仍不达 14px，既有字号/字重拍板项沿用，不宣称全过 |
| D2-3 混排空格 | 新测试内容有空格；产品文案不变，程序内容按原文显示 |
| D2-4 标点/对比 | 文案不变；现有安静墨并非全达 7:1，沿用已记录限制 |

## 仍待

- 八项修复全部落地，四批代码已推送：`9e32937d`、`5ebc90b6`、`df950050`、`78df327b`。
- 下一轮首选本地工具主表单在模式切换、校验提示插入后的输入焦点与错误恢复；本轮规格第 5 项仅为候选，未将未复现的问题计入完成。
- 176-9/161-4 保存期间草稿语义、字号/行盒、终端 S1b/S1c/S2 尾沿用拍板项。本轮不改其契约。
- 开盘八条早期 WIP 原件保留，未清掉独立历史工作。

## 最终门槛

`gofmt -l cmd internal` 输出 **0 字节**；Linux/Windows/macOS 三平台 `go vet ./...` 与 `go build ./...` 均退出 0。普通全量 `go test -count=1 -timeout=20m ./...` 退出 0（settingsui 311.365s）；verbose 全量 **3608 PASS / 0 FAIL**（基线 3592，增加 16；settingsui 274.761s）。测试函数 **2711 → 2719**，没有新增 Skip、删除测试或放宽断言；V/B/S **0/0/0**。本轮约 60 分钟。

完整日志：`/tmp/floter-P13-hb-baseline-zh.log`、`/tmp/floter-P13-hb-final-test.log`、`/tmp/floter-P13-hb-final-verbose.log`。16 张原图与两张局部对比已入库；原始 PNG 720×1500、对比 680×310。

4× 局部复核同样通过：脚本前两行的中文/英文字形不再共享同一行墨；第四行在固定槽底部正常裁切，滚动条仍可继续读后续内容。不能把固定视口里可见行数下降说成高度回归，它来自修正内部行距。
