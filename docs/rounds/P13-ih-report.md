# P13-ih · 获取与提交回调换表单后保住当前面

开盘 b6a77746（实际前轮 P13-ig），pull 最新，工作区干净。首个时钟 2026-11-06 23:34:48 UTC。done.log 不存在，收尾补建。基线运行中，数字只在完成后填写。

## Stash 清点

当前八条 SHA 与 P13-ig 及 backlog 一致，无新增 stale-kill；早期抢救已接续归档。以下早期独立 WIP 不覆盖新主题、不恢复旧 Tauri 链，逐条保留。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 过程

规格先于产品修改。读取 skill、偏好及 craft-loop / preflight / chinese-typography，按已定方向打磨；不走四方案。运行表单同步关闭再重开仍无真实公开路径，不把直接改私有布尔值当成产品复现。

## 第一批：配置获取（1–2）

1. 现象：配置获取内部通过公开 OpenConfig 再开一面，外层返回把新上限 25 改回 10。改动：获取前记录已有显示代次，返回失配即不接纳旧结果。验证：深浅真实配置界面保留新值；Escape 仍关闭、新请求可再开。
2. 现象：内层最新获取返回 nil，外层仍复活旧配置控件。改动：最新空结果也拥有显示期。验证：深浅旧“保留上限”控件消失，下一次正常打开成功。

原版四叶子实红：`fetch-new lookup overwrote latest configuration: open=true`、`empty latest lookup resurrected old controls`。M1 撤回同一产品 diff 实跑、MD5 还原、touch 复验；结果见 batch1 日志。旧 V/B/S 修改 0/0/0。

第一批最终定向 **50 PASS / 0 FAIL**，相关 vet 净，d546dad4 已推送。

## 第二批：文件选择返回（3–4）

3. 现象：选择器内换到 local.new，画面却保留 oldtool 和 selected-for-old 路径。改动：整帧字段回写前核对既有表单身份。验证：深浅真实“选择”点击后保留 newtool、/usr/bin/newtool、--newtool；下一次普通选择仍准确接纳一次。
4. 现象：选择器内结束编辑，关闭后又回写旧草稿，再展开新连接出现旧名字和路径。改动：已关闭或换代的表单不接纳旧帧局部值。验证：关闭时草稿为空，真实点击展开仍为空，再选择当前路径成功。

M2 原串 `picker overwrote new draft: editing="local.new" name="oldtool" program="/usr/bin/selected-for-old" args="--oldtool"`、`closed picker resurrected draft: open=false name="oldtool" program="/usr/bin/selected-for-old" command="oldtool"`。四叶子实红，MD5 还原、touch 复验，旧 V/B/S 0/0/0；精确 PASS 见 batch2-green.txt 与本批提交正文。

第二批 **50 PASS / 0 FAIL**，384d26de 已推送。基线完整退出 0：**4615 PASS / 0 FAIL**；开盘 b6a77746 对象实测 **2995 测试函数**。基线脚本完成后数到 2999 包含当时新增的四个，不能冒充开盘值。

## 第三批：提交收尾（5–6）

5. 现象：保存 local.old 的回调打开 local.new，旧保存收尾又关闭并清空 local.new。改动：收尾核对提交表单身份。验证：深浅真实“保存”后保留新编辑器、名字、路径、参数；再保存新目标仅调用一次并正常关闭。
6. 现象：新建连接回调打开编辑目标，同样被旧连接的 endEdit 清空。改动：连接与保存共用已有表单身份约束。验证：深浅真实“连接”后 local.new 可见、可保存，原提交只发一次。

M3 四叶子实红，原串 `save-new completion erased newer editor: open=false id="" name="" program=""`、`connect-new completion erased newer editor: open=false id="" name="" program=""`。MD5 还原与 touch 复验，相关 vet 净；精确 PASS 见 batch3 日志和提交正文。旧 V/B/S 0/0/0。

第三批 **50 PASS / 0 FAIL**，0321fcec 已推送。

## 第四批：编辑工具获取（7–8）

7. 现象：点击旧工具“编辑”，获取回调里已打开 local.new，旧结果仍抢回 local.old。改动：beginEdit/endEdit 当场更新既有表单 key，获取返回核对该身份。验证：深浅真实编辑点击保留 local.new 与 newtool，后续明确点击可再编辑旧工具。
8. 现象：获取回调刷新目标使 Generated=false，旧结果仍打开本应不可编辑的表单。改动：返回按 ID 重读当前目标，核对存在、Generated、非 Orphan、写入能力和忙态。验证：深浅不出现失效编辑器；恢复资格只刷新不获取，下一次明确点击才打开且只调用一次。

M4 四叶子实红：`old tool lookup replaced newer editor: id="local.old" name="oldtool"`、`retired generated tool opened stale editor`。MD5 还原、touch 复验，相关 vet 净；精确 PASS 见 batch4 日志和本批提交正文。八项均有独立事件验收，旧 V/B/S 0/0/0、无新增 Skip。

第四批 **50 PASS / 0 FAIL**，d48412c7 已推送。四批每批两项，完整三平台 vet/build 已退出 0，普通及 verbose 测试进行中。

## 偷了什么

- craft-loop 工艺密度菜单 #3「品牌 focus 环」：从 Apple DESIGN.md 取交互蓝的职责和 2px focus，沿用本仓 2px 焦点，不为回调结果另加提示层。
- 菜单 #4「签名交互」：取即时反馈这一原则，旧返回覆盖次数从 1 降到 0，新面保留延迟 0ms；自动重提/重查 0 次，下一次明确点击才调用。八项事件全部断言调用次数。
- 菜单 #12「材质细节」：取 Apple 8px 控件圆角，对应本仓既有 8px；保留 1px 受光边，新增玻璃/投影 0 层。字体、行盒、材质 token 零改。

没有为凑满菜单五项添加装饰。用户指定既有方向高于四方案、HTML、Web 大屏和新增 DESIGN.md 流程；唯一设计来源继续是 liquid-glass。没有调用其他执行器或外部模型。

## 图像复核

第一轮四视角：艺术总监看到“当前 ID 是新工具、名字却是旧工具”的主次冲突；工程师看到隐藏草稿会在重新展开后泄漏；用户点击保存后新编辑器突然消失；读者看到无法编辑的行却配着编辑表单。第二轮分别验证身份和值一致、重新展开为空、保存保住新面、资格撤回后空态，保留正常恢复路径。

已目视浅色与深色对照：第 1 行开关开启/上限 10 变为关闭/25；第 3 行 local.new 下的 oldtool 与 selected-for-old 恢复成 newtool 和正确路径；第 4 行旧输入清空后出现原有占位文案；第 5/6 行新编辑器保持打开；第 8 行撤掉失效表单。

所有原图由 Linux ui.Tester 生成：配置 720×420，工具 720×1100，含集成行 720×1400。对照板保留 1:1 像素、每项取顶部 420px，不是完整窗口；完整帧在同目录。八项深浅均有非零像素差，见 image-differences.txt。Windows/macOS 只有编译门槛，无真机交互证据。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，既有深浅 token 不改 |
| A2 非系统字体 | 中文系统字体与产品合约优先，不为网页规则换字体 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，现有桌面表单 |
| A5 卡片海/三等分 | 无新增容器 |
| A6 假数据 | 产品无假数据，oldtool/newtool 仅为事件夹具 |
| A7 空词/感叹号 | 产品 i18n 零改 |
| A8 编号眉批 | 产品无新增，图板编号仅对应验收 |
| A9 H1 | 原有页头与工具身份可见 |
| A10 Bento 空洞 | 不适用，自绘表单 |
| A11 左色条 | 无新增 |
| B1 读取/拨盘一致 | 克制 3、动效 2、密度 7，行为修复不改变视觉方向 |
| B2 主题锁定 | 每个夹具显式固定 Store 和 Tester 的深浅主题 |
| B3 记忆点 | 当前工具 ID、字段值与操作保持对应 |
| B4 动效气质 | 高频回调结果即时归属，新增动画 0 |
| C1 目的/频率 | 高频点击和键盘不加动效 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原有按压与过渡不改 |
| C4 origin | 不新增浮层，当前表单原位保留 |
| C5 reduced-motion | 新夹具均 ReduceMotion=true |
| C6 hover 媒体查询 | Go 自绘不适用 CSS，原指针路由保留 |
| C7 布局动画 | 无新增 |
| C8 keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer 高负载 | 不适用 Go 自绘 |
| C11 慢放 | 无新增动画；测试真实点击和后续帧，不声称真机慢放 |
| D2-1 字体 | 保留系统中文字体，无新增 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px/1.5 与现有行盒冲突，沿用拍板项，不声称通过 |
| D2-3 混排 | 产品文案零改 |
| D2-4 标点/对比度 | 原标点与墨不变，未声称所有安静墨满足 AAA |

可复现缺陷已记录于本仓 §6.209 和事件测试，不把局部状态问题升格成全局审美规则。

## 验证边界与仍待

1. 回调重入通过现有可注入的宿主接口驱动，并在真实 ui.Tester 控件/公开 OpenConfig 入口中复现。工具选择对应 shell.chooseProgram 的原生选择器接口；编辑获取对应 toolForEdit 的描述符读取。这里不声称已经在实体 OS 对话框中重现多窗口切换。
2. 下一格首选 CopyText / ExportScript 返回与“已复制”反馈的归属，只是代码线索，尚未复现；先事件验证，保持 1.6s 窗口、导出/保存时机。
3. 运行回调同步关闭再重开仍未找到真实公开路径，不新造私有生命周期。字号/行盒、accent 禁用填充、未知多选/Radio、极小数字步长、176-9/161-4 保存期间草稿、终端 S1b/S1c/S2 尾继续需拍板。
4. 八条早期独立 WIP 原样保留；无新系统、页面、依赖，唯一执行者为当前 Codex。

## 收尾复验

最终四批产品合并后的八项事件为 **24 PASS / 0 FAIL**。新增测试函数 8 个，旧测试文件未修改，新增 Skip 0。全部深浅原图与比较板已入 P13-ih 目录，原始基线、分批红/绿、mutation 补丁与还原日志同步归档。

完整门槛最终 **4639 PASS / 0 FAIL**（基线 4615，增加 24），函数 **3003**（基线 2995，增加 8）。普通和 verbose 全量均退出 0；三平台 vet/build 均通过，gofmt 空、git diff --check 净。最终四份源码 MD5 校验通过。统一 LC_ALL=zh_CN.UTF-8、GOMAXPROCS=4，完整门槛使用磁盘 TMPDIR=/home/tar/.fih。

原始证据：[基线](P13-ih/baseline.log)、[普通全量](P13-ih/full-test.txt)、[verbose 全量](P13-ih/full-verbose.txt)、[三平台门槛](P13-ih/gates.txt)、[源码校验](P13-ih/final-source.md5)、[像素账](P13-ih/image-differences.txt)。前后对照：[浅色](P13-ih/comparison-light.png)、[深色](P13-ih/comparison-dark.png)。四批产品分别 d546dad4 / 384d26de / 0321fcec / d48412c7，每批两项，均已推送。

执行偏差：首个时钟 23:34:48 UTC，完整门槛于 00:02 左右完成，约 28 分钟，短于 45–75 分钟目标；没有用无关修改填时长。工具调用超过 40 次，前期读取过宽和门槛等待增加调用；八项完成后停止扩展范围，只做必要验收与归档。未跳测、未放宽断言，没有新增需拍板的行为改变。
