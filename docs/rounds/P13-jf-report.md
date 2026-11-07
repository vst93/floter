# P13-jf：文件传输失败的原因与恢复

开盘 2c61b589，pull 已同步、工作树干净。基线于 2026-11-07 16:10:22 UTC 开始；Codex 独立执行。done.log 开盘不存在。仅实测完成项计入交付。

**最终八项全部完成**，四批每两项推送，最终产品 ee3a4b24；**5480 PASS / 0 FAIL / 3245 函数**（基线 5454 / 0 / 3237）。普通/verbose 全量、三平台 vet/build、gofmt、定向 race、两组 mutation 均通过。过程中的全量中止、计数笔误与调用预算偏差保留在下文。

## Stash 清点

八条均与已归档轮次相同，无新增 stale-kill。逐条导出 patch、reverse-check 全为 1；不是 HEAD 已覆盖，但为已审核早期独立 WIP，继续保留，不把旧主题或 Tauri 链盖回当前 Go 产品。

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

## 实施记录

旧 V/B/S 断言修改 0/0/0；无新增 Skip。

1. 现象：选择已被移走的集成文件，只提示无效文档。改动：接回 os.ReadFile 的路径与错误。验证：深浅两主题经已接线的 ImportIntegrations 动作与真实临时文件事件，原产品报 `file read failure hides its cause`；修后全文可见、busy 释放，补回文件后同入口成功产生报告。
2. 现象：导出遇到损坏仓库，也提示无效导入文件。改动：展示 ExportSync 返回的仓库读取原因。验证：原产品报 `repository export failure hides its cause`；修后保存选择器 0 次、无目的文件、busy 释放，修复仓库再导出 1 个集成。

第一批相关 **17 PASS / 0 FAIL**，shell vet 净；原产品真实红证据 before12.txt。初始指针夹具受整页高度影响，改用产品已接线 Actions + 真实文件事件，不声称系统原生选择器真机验收。正式 mutation 后续独立注入并 MD5 还原。

基线结束：**5454 PASS / 0 FAIL / 3237 测试函数**，与 P13-je 一致。第一批 1a6853f2 已推送。

3. 现象：不支持的版本和 JSON 语法错误只剩同一个标题。改动：保留原本地化标题，用已有 Toast.Description 接回实际解析原因；满高浮层补画原来遗漏的详情。验证：两种文档 × 深浅，原产品 `document refusal hides its cause`；修后标题与原因同时可见、不产生报告，改正文件可重试。
4. 现象：仓库损坏被当空仓库，合法导入误显示跳过 1 项。改动：ImportSync 在任何应用动作前返回 RepositoryErr，宿主显示实际错误。验证：两主题原产品 `unreadable repository was treated as empty` 与 `unreadable repository fabricated import report`；修后仓库字节不变、无虚假报告，恢复仓库后成功 1/跳过 0/失败 0。

第二批相关 **62 PASS / 0 FAIL**，extensions/shell vet 净。旧标题 B pin、原单行 toast 的结构和键盘规则未改；新增详情列以 2u 间隔排原文，无新增浮层。

第二批 9c0871c6 已推送。第一次目视：浅色浮层原标题下一行明确显示不支持 version 99；关闭键完整，单一浮面不再只有笼统拒绝。深色报告的普通多段路径本来就能换行，未计修复；进一步测试无断点文件名。

5. 现象：脚本保存选择器不可用只显示“导出失败”，无恢复线索。改动：保留原标题并显示选择器返回原因。验证：深浅两主题，原产品 `script-picker hides failure cause`；修后可见“restart the desktop portal”，恢复选择器后导出成功，原取消静默 B 测试仍绿。
6. 现象：脚本写入目录等真实磁盘错误也被压成笼统失败。改动：接回路径与文件系统原因。验证：原产品 `script-write hides failure cause`；修后原因可见，改用合法路径重试文件字节等于原脚本。

第三批相关 **16 PASS / 0 FAIL**，shell vet 净。没有改变导出脚本、选择器或成功提示的时机。

第三批 37cb7b6d 已推送。

7. 现象：已安装包的 descriptor 损坏，导出编码失败却称无效导入文件。改动：显示 document.Marshal 的真实原因。验证：两主题真实损坏 descriptor 触发 `encoding failure hides its cause`；修后原因可读、已有备份字节不变、busy 释放；修复 descriptor 后同入口输出可解析文档。
8. 现象：导入报告中的无断点长文件名越过文本列；旧右沿 360，允许右沿 328，越界 **32px**。改动：报告文字增加 MinWidth(0)，按真实剩余列宽换行。验证：深浅旧版均报 `import failure overflows the report column`；修后全文节点留在列内，后续内容不重叠。普通可断路径原本正常，不另凑一项。

第四批相关 **15 PASS / 0 FAIL**，shell/settingsui vet 净。旧 V/B/S pin 修改 0/0/0；所有八项均有原产品失败和修后验收。该批提交正文误沿用了第三批的 16，原日志实测是 15，以本条与 batch4-after.txt 为准，不改写已推送历史。

第四批 ee3a4b24 已推送；产品修改集中在 sync.go 两层、notices.go 和 transfer.go。

## Mutation

M1 回注六类原因丢失：读取文件、读取导出仓库、文档解析、脚本选择器、脚本写入、编码。注入退出 1，六个函数的全部主题/文档场景均实红；恢复后三个文件 md5sum -c 全部成功，touch 源码后 **20 PASS / 0 FAIL**。

原始失败串包含：`file read failure hides its cause: open .../removed-backup.json: no such file or directory`、`repository export failure hides its cause: extensions: invalid repository ...`、`document refusal hides its cause: extensions: import version 99 is not supported`、`script-picker hides failure cause: file picker unavailable: restart the desktop portal`、`script-write hides failure cause: open ...: is a directory`、`encoding failure hides its cause: json: error calling MarshalJSON for type *jsontext.Value: invalid character 'b' looking for beginning of value`。含临时路径的完整未删原文在 [M1-red.txt](P13-jf/M1-red.txt)。

M2 去掉导入仓库错误保护并去掉报告列宽约束。注入退出 1，原文 `unreadable repository was treated as empty`、`unreadable repository fabricated import report`、`import failure overflows the report column: {X:33.6 Y:12 W:326.4 H:45} right=328`；深浅均红。恢复后三个文件 MD5 全部成功，touch 后 **6 PASS / 0 FAIL**。完整证据 [M2-red.txt](P13-jf/M2-red.txt)、[M1 还原](P13-jf/M1-md5.txt)、[M2 还原](P13-jf/M2-md5.txt)。

## 偷了什么

实际读取 qiaomu-design SKILL、偏好账本、craft-loop、preflight、chinese-typography，并读取 Apple DESIGN 的颜色和字体段。使用用户指定的打磨路径，Codex 独立执行；已有仓库合约优先，没有四方向预览或外部模型。

- 从 craft-loop 菜单 **#6 编辑细节**取标题/补充文字的层级：原标题保持 Warning，实际原因使用 TextSecondary，两者间距 **t.Space(0.5)=2u**，同用既有正文字号；无断点报告的文本列最小宽度 **0**，不再超出分配列宽 **32px**。
- 从 **#4 签名交互**取即时反馈与恢复：新增动画 **0ms**；失败释放既有 busy，修复文件后从原入口重试 **1 次**即产生实际文件或报告，正常取消提示 **0 条**。
- 从 **#12 材质细节**取“同一浮面内建立层级”：沿用 E2 的 **10px** RadiusMD、**1px** 受光边与原两层投影；详情仍在原浮层内，新增透明面 **0 层**，不为两行内容再套卡。
- Apple 的三个具体 DNA 转译：单一交互蓝留给原焦点/动作；近黑/近白继续用本仓主题；400 正文角色用于原因，不借营销页大字。没有覆盖既有 token，也不把未修改的焦点环冒充新增修复。

为什么本轮更轻、更有空间感：一个浮层同时交代动作与原因；长报告不再横向侵占相邻空间。用户要求修已有功能优先，不为菜单凑五项增加装饰。

## 两轮图像复核

第一轮直接从原产品导出 before：四视角问题分别是笼统错误缺乏信息层级（设计）、无断点长文件名越界（工程）、仓库损坏却称跳过（任务用户）、导出失败没有恢复线索（读者）。普通多段路径正常，未当作缺陷。第二轮只复核这四处：标题下增加原因、长文件名完整换行、错误状态替代虚假结果、关闭键完整。

目视证据：浅色版本错误图从一行笼统标题变为标题 + version 99 诊断；深色脚本失败保留原标题，原因包含实际路径和 is a directory；报告深浅两套的长文件名从右边被截断变为完整多行，Next integration 随正文高度下移，不再重叠。4× 最近邻只用于检查边缘，不冒充真机高 DPI。

| 项 | 浅色前后 | 深色前后 |
| --- | --- | --- |
| 1 文件读取 | [图](P13-jf/import-read-light-comparison.png) | [图](P13-jf/import-read-dark-comparison.png) |
| 2 导出仓库 | [图](P13-jf/export-repository-light-comparison.png) | [图](P13-jf/export-repository-dark-comparison.png) |
| 3 文档版本 | [图](P13-jf/import-version-light-comparison.png) | [图](P13-jf/import-version-dark-comparison.png) |
| 3 文档语法（同项） | [图](P13-jf/import-syntax-light-comparison.png) | [图](P13-jf/import-syntax-dark-comparison.png) |
| 4 导入仓库 | [图](P13-jf/import-repository-light-comparison.png) | [图](P13-jf/import-repository-dark-comparison.png) |
| 5 脚本选择器 | [图](P13-jf/script-picker-light-comparison.png) | [图](P13-jf/script-picker-dark-comparison.png) |
| 6 脚本写入 | [图](P13-jf/script-write-light-comparison.png) | [图](P13-jf/script-write-dark-comparison.png) |
| 7 导出编码 | [图](P13-jf/export-encoding-light-comparison.png) | [图](P13-jf/export-encoding-dark-comparison.png) |
| 8 报告列宽 | [图](P13-jf/report-bounds-light-comparison.png) | [图](P13-jf/report-bounds-dark-comparison.png) |

[总览](P13-jf/overview.png)、[深色 4×](P13-jf/report-detail-dark.png)、[浅色 4×](P13-jf/report-detail-light.png)。36 张原帧保留，宿主图 720×1000、报告组件 360×320；18 张对照裁取反馈区域。宿主扫描结果与临时路径会变，因此像素差仅作图像有变化的记录，**不是确定性视觉 pin**；验收由文字、文件字节、实际状态与几何断言承担。全量日志、测试夹具使用 Linux ui.Tester；原生桌面与 Windows/macOS 运行时没有实测。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，使用既有近黑/近白 token |
| A2 非系统字体 | 不采用网页模板要求；中文系统字体与仓库合约优先 |
| A3 斜体 | 无新增 |
| A4 Hero | 原生设置，无 Hero，方向预览不适用 |
| A5 三等分/卡片海 | 无新增卡片，详情放在原浮层 |
| A6 假数据 | 真实临时文件、损坏仓库与 descriptor；没有产品假指标 |
| A7 空词 | 没有营销词，系统错误保留原文 |
| A8 眉批 | 无新增 |
| A9 标题 | 既有页头不改，错误长标题换行 |
| A10 Bento | 不适用 |
| A11 左装饰线 | 无新增 |
| B1 读取/拨盘 | 2/1/6，保守、即时、原信息密度 |
| B2 主题锁定 | 深浅独立实帧，无混用 |
| B3 记忆点 | 失败能说清原因，修正后原入口能重试 |
| B4 动静气质 | 沿用原浮层进入，高频重试不加动效 |
| C1 目的/频率 | 新增动效 0ms |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原按钮与浮层时限不改 |
| C4 origin | 原右上角 toast、流内报告，不新开 modal |
| C5 reduced-motion | 新帧均 ReduceMotion=true |
| C6 hover 查询 | 原生 Go 指针路由；CSS 查询不适用 |
| C7 布局动画 | 无新增；文字自然换行 |
| C8 keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新增动效，不宣称真机慢放验收 |
| D2-1 字体 | 系统字体不改，无 webfont |
| D2-2 字号/行盒 | Caption 9.9/正文 10.8 不满足通用 ≥14px/1.5；既有拍板项，不擅改 |
| D2-3 混排空格 | 原 i18n 字典不变；技术错误保留英文原文 |
| D2-4 标点/对比 | 原 token 与本地化标题不变；不宣称默认安静墨全部达到 7:1 |

发现的产品缺陷写入本仓 §6.233，未写成全局 skill 审美规则。通用五项密度、字体轮换与四方向流程不适用于用户已明确的轻玻璃打磨车道。

## 仍待

1. 下一轮首选部分成功的导入报告：transfer.go 失败分支提前 return，sync.go 把 Skipped 只映射数量；成功/失败/跳过并存时是否缺少必要结果信息，仅代码线索，先真实文件事件复现。不要改变覆盖策略、保存时机或确认窗口。
2. 字号/行盒、accent 禁用填充、多草稿覆盖、未知多选/Radio、极小步长、176-9/161-4 保存期草稿与终端 S1b/S1c/S2 尾继续需拍板。
3. 八条早期 WIP 继续保留。原生系统选择器、Windows/macOS 运行时未验；本轮采用接线动作与真实磁盘事件，不冒充跨平台真机交付。

## 完整门槛

普通/verbose 全量、三平台 vet/build 与定向 race 已启动；只按结束日志填写最终数字。产品已分四批推送，文档与截图随归档提交。

已结束的门槛：gofmt 输出 **0 字节**，linux/windows/darwin vet/build 六项均退出 **0**；新八项相关 race 退出 **0**（shell 46.166s、settingsui 2.999s）。未改旧测试函数或断言，新增 Skip **0**。完整全量仍待结束。

首轮并行普通/verbose 全量进程均返回 **143**，没有正常结束标记，故不计通过；中止日志保存在 /tmp/floter-P13-jf/full-{test,verbose}-aborted.txt。未据此放宽断言或跳测，改为顺序重跑两个完整命令。三平台与 race 的已完成结果保留。

顺序普通全量已正常退出 **0**。最终测试函数 **3245**（基线 3237，+8）；verbose 计数正在运行。开盘过宽输出曾被截断，初始指针夹具被真实发现列表撑出视口，最终使用接线 Actions + 磁盘事件；不把这些夹具失败记成产品缺陷。调用预算已超 40，后续只完成必要门槛重跑和归档，不声称过程全部合规。

## 最终验收与交接

| 门槛 | 实测结果 | 证据 |
| --- | --- | --- |
| verbose 全量 | **5480 PASS / 0 FAIL**，比基线 +26；Go 退出 0 | [full-verbose.txt](P13-jf/full-verbose.txt) |
| 普通全量 | Go 退出 **0** | [full-test.txt](P13-jf/full-test.txt) |
| 测试函数 | **3245**，基线 3237，增加 8，未删除旧函数 | [汇总](P13-jf/gates.txt) |
| gofmt | 输出 **0 字节** | [gofmt.txt](P13-jf/gofmt.txt) |
| 三平台 vet/build | Linux、Windows、Darwin 六项全退出 **0** | [platforms.txt](P13-jf/platforms.txt) |
| 新八项定向 race | 退出 **0** | [race.txt](P13-jf/race.txt) |
| 文档契约 | 退出 **0** | [docs-ci.txt](P13-jf/docs-ci.txt) |
| Mutation | 两组实红、MD5 还原、touch 后 **20/6 PASS** | M1/M2 原始日志与 MD5 |
| 旧 V/B/S / 新 Skip | **0/0/0 / 0** | Git diff；八个新增测试函数 |

最终源码 MD5 与四批产品保持一致。verbose 的 Go 命令退出 0；其包装脚本最后 `grep -c '--- FAIL'` 匹配数为 0，因而包装脚本退出 1，这不是测试失败，独立保存的 full-verbose.exit 为 0。

基线启动 16:10:22 UTC，最终全量结果确认于 **16:42:30 UTC**，间隔 **32 分 8 秒**；加上开盘读取与归档仍低于 45–75 分钟目标，没有用无关修改或重复测试凑时长。八项已独立验收，产品四次推送为 1a6853f2 / 9c0871c6 / 37cb7b6d / ee3a4b24，最终报告与深浅图随归档提交。跨平台运行时证据仍限 Linux，自绘图不冒充原生桌面截图。
