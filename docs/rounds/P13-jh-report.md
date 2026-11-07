# P13-jh · 配置传输的文本与备份完整性

起点 ac396892。读过 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography；方向沿用，单人执行。完整门槛与逐项修复尚在进行，不把计划当结果。

## Stash 清点

无新增 stale-kill；下列 SHA 与 P13-jg 登记相同，reverse-check 均为 1。早期 WIP 已有语义归档，不把旧主题/旧 Tauri 链重合到当前产品；无静默删除。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 增量记录

基线函数 3258；基线 verbose 实测 **5513 PASS / 0 FAIL**、退出 0。八项事件均在原产品复现，原始红日志见 P13-jh/before-red.txt；原产品 18 个主题/入口场景均未保护损坏输入或旧备份。

1. 现象：配置 endpoint 含 0xff 时复制/导出成功，JSON 静默把原字节替换。改动：脱敏后验证公开字符串与多选列表的 UTF-8，再编码；错误定位字段。验证：原始 `configuration transfer silently repaired invalid text: <nil>`；事件覆盖复制/文件、深浅、旧备份不变，修正为中文及合法 U+FFFD 后成功且密码仍脱敏。
2. 现象：磁盘 config.json 含非法 UTF-8，集成导出把损坏文本读成合法值并覆盖备份。改动：原始配置解析拒绝非法字节，导出错误附 config.json 路径。验证：原始 `stored-text export replaced a good backup with unrestorable content`；修正文件后同入口重试，中文/U+FFFD 原样保留。

第一批定向 **20 PASS / 0 FAIL**，extensions/shell vet 净。旧 V/B/S 断言修改 0/0/0，无新增 Skip。未来候选的失败测试先作为复现日志落盘，各批实现时再启用对应函数，保证每个提交不会带已知红测试。浅色首轮图确认字段名与 UTF-8 原因两行可读，关闭按钮完整；没有新增层级。

3. 现象：已登记包的 manifest 损坏，导出仍成功却丢掉整个包。改动：明确登记包的读取/解析失败返回路径原因；平台不匹配重新读原 manifest，不误当损坏；无包兼容记录保留。验证：原始 `manifest export replaced a good backup with unrestorable content`；坏文件不覆盖旧备份、修正后导出含完整包。
4. 现象：零字节脚本成功导出，但已有导入器拒绝空脚本。改动：导出时拒绝空必需脚本并指出路径，保留原有导入约定。验证：原始 `empty-script export replaced a good backup with unrestorable content`；写回合法脚本后同入口备份原文相等。

第二批定向 **22 PASS / 0 FAIL**，extensions/shell vet 净。深色 manifest 错误路径折行、原因尾部与关闭按钮可见；原有导入安装/合并/权限拒绝、缺脚本/descriptor 全绿。

5. 现象：含 0xff 的脚本经过 JSON 字符串导出变成 U+FFFD，备份不再是原脚本。改动：读取脚本后编码前校验 UTF-8，报实际源路径。验证：原始 `script-text export replaced a good backup with unrestorable content`；旧备份保持，修复后原文相等。
6. 现象：零字节静态 descriptor 经 omitempty 被省略，成功备份实际不能恢复工具。改动：读取后拒绝空描述文件。验证：原始 `empty-descriptor export replaced a good backup with unrestorable content`；修复文件后导出 JSON 描述内容相等，busy 释放。

第三批首次测试发现本轮夹具把 descriptor 中的 `script` 子串误认成脚本，错误要求 ScriptContent 等于描述 JSON。改为两个精确脚本场景名；描述文件自身的严格 JSON 等值断言不变。首次日志保留，没有修改旧 B pin，也没有放宽任何容差。

第三批修正夹具后 **21 PASS / 0 FAIL**，extensions/shell vet 净。前六项完成，正式 mutation 与全量门槛仍待。

7. 现象：descriptor 的非法 UTF-8 原样进入导出 JSON，自己的 ReadSync 随后拒绝。改动：读取 descriptor 时检查编码并定位源路径。验证：原始 `descriptor-text export replaced a good backup with unrestorable content`；修复后 JSON 原文一致。
8. 现象：脚本源文本不到 4 MiB，JSON 转义后超过导入上限，导出仍覆盖好备份。改动：编码完成后以最终字节数（包括文件末尾换行）检查已有 4194304 字节上限。验证：原始 `oversize export replaced a good backup with unrestorable content`；新增恰好上限可读、多一字节拒绝与转义膨胀边界。

第四批 **29 PASS / 0 FAIL**，extensions/shell vet 净。兼容测试确认无包记录仍可导出、其他平台的包仍完整携带、自定义 manifest 文件名正确保留、明确登记但缺失的包拒绝。公开 text/path/multiselect/key 的坏字节拒绝，已脱敏的密码不误拒，合法中文/U+FFFD/emoji 保留。八项四批完成，完整门槛尚待。

## 正式 mutation

M1 撤销公开配置文本检查：真实复制/导出四场景报 `configuration transfer silently repaired invalid text: <nil>`，注入退出 1；还原 config.go、MD5 OK、touch 后 **5 PASS**，退出 0。

M2 撤销脚本与 descriptor 空文件检查：报 `empty-script export replaced a good backup with unrestorable content` 与 `empty-descriptor export replaced a good backup with unrestorable content`；注入退出 1，还原 sync.go、MD5 OK、touch 后 **6 PASS**，退出 0。日志和 MD5 清单在轮次目录。

第二轮把“恢复成功”验收到实际安装：每个修正后的备份都导入新的数据目录，必须成功安装一个工具、无失败/跳过。补 720×520 中文深浅图与同入口恢复检查，避免只凭 720×1000 英文页声称中文小窗口验证。

第二轮定向 **42 PASS / 0 FAIL**，包 vet 净。12 个新增测试函数，当前函数总数 **3270**（基线 3258）；无新增 Skip，旧测试未删改。四批产品提交 **d1840391 / 521683bc / 655cdf4c / 5766da2b** 已推送。

## 偷了什么

- craft-loop **#6 编辑细节**：字段名或文件路径与原因在原浮层内自然折行，状态容器新增 **0**；正文使用现有 Caption，不提高文案密度。
- **#4 签名交互**：错误到达后的额外等待 **0ms**；失败→修正→重试共用原动作，新增按钮和确认步骤 **0**。
- **#5 多层染色阴影**：复用 E2 两层影（基准 alpha **0.30 / 0.20**，原 10u/26u 环境影及 1u/2u 接触影），面板内新增阴影 **0**。
- **#12 材质细节**：复用原浮层 **1px** EdgeHighlight 与 RadiusMD，不叠玻璃。Apple DESIGN 的单一交互 accent / 中性正文 / 细边三种角色映射到本仓语义，不替换既有品牌值。

遵从用户“修现物、不开系统”的上位要求，没有为菜单凑满五项加装饰或另造 DESIGN.md。**这轮为什么更轻、更有空间感**：损坏内容的错误只在原 E2 浮面准确出现，取消虚假的成功反馈，页面无需新增状态框或嵌套玻璃。

## 两轮复核

首轮四视角：艺术总监——长路径可能把一个小错误撑成大块，保持单层、沿原文本列折行；工程师——只验证 JSON 可读还不足，第二轮实际导入空目录；任务用户——错误必须指到字段/源文件且能重试，18 个入口/主题场景验证；读者——英文大窗口不足以代表中文页面，第二轮补 720×520 中文深浅帧。

第二轮聚焦四件事：真实恢复、中文小窗口、公开 Unicode 与脱敏不误拒、最终大小/跨平台包边界。未发现需要新增装饰或更改字阶的证据。浅色图中原“复制成功”变成两行 endpoint/UTF-8 拒绝，右侧关闭按钮完整；深色 manifest 图中完整原因在路径下方可读，没有越过浮层底边。原始前后图为 Linux ui.Tester 软件自绘，不是桌面原生截图。

深浅前后 **18 组**（第 1 项含复制和文件导出两个入口），像素差全部非零，见 [浅色总览](P13-jh/overview-light.png) / [深色总览](P13-jh/overview-dark.png) / [像素差](P13-jh/image-differences.txt)。路径中的临时目录每次不同，不能把整张图的变化量当作纯墨值变化。原始 PNG 同目录保留，比较图只截上部 230px 并标明前/后，未生成替代界面。

| 项 | 浅色对照 | 深色对照 |
| --- | --- | --- |
| 1 配置文本 | [复制](P13-jh/copy-light-comparison.png)、[导出](P13-jh/config-export-light-comparison.png) | [复制](P13-jh/copy-dark-comparison.png)、[导出](P13-jh/config-export-dark-comparison.png) |
| 2 磁盘配置 | [图](P13-jh/stored-text-light-comparison.png) | [图](P13-jh/stored-text-dark-comparison.png) |
| 3 manifest | [图](P13-jh/manifest-light-comparison.png) | [图](P13-jh/manifest-dark-comparison.png) |
| 4 空脚本 | [图](P13-jh/empty-script-light-comparison.png) | [图](P13-jh/empty-script-dark-comparison.png) |
| 5 脚本乱码 | [图](P13-jh/script-text-light-comparison.png) | [图](P13-jh/script-text-dark-comparison.png) |
| 6 空描述文件 | [图](P13-jh/empty-descriptor-light-comparison.png) | [图](P13-jh/empty-descriptor-dark-comparison.png) |
| 7 描述文件乱码 | [图](P13-jh/descriptor-text-light-comparison.png) | [图](P13-jh/descriptor-text-dark-comparison.png) |
| 8 超限备份 | [图](P13-jh/oversize-light-comparison.png) | [图](P13-jh/oversize-dark-comparison.png) |

中文 720×520：[浅色](P13-jh/chinese-compact-light.png)、[深色](P13-jh/chinese-compact-dark.png)。代表图 `/tmp/floter-P13-jh-transfer-{light,dark}.png`。亲眼复核确认中文页面保留标题与导航，短错误完整显示在原浮层；并不把底层英文诊断误记为新增中文翻译。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A-1 渐变/纯黑/霓虹 | 通过；不加渐变、glow 或纯黑面，截图外部透明区不算界面材质 |
| A-2 字体非系统默认 | 按用户既有 Go/中文合约保留系统字体；通用换字体建议不执行 |
| A-3 斜体 | 通过；未新增斜体，中文图已检查 |
| A-4 Hero / A-5 卡片模板 | 不适用 Hero；未新增卡片/容器 |
| A-6 假名假数 / A-7 空话 | 通过；真实文件事件与字段，诊断为具体原因 |
| A-8 眉批 / A-9 H1 | 通过；不增眉批；页面标题单行 |
| A-10 Bento / A-11 左色条 | 不适用 Bento；无新增装饰左条 |
| B-1 读取与拨盘 | 通过；2/1/6 沿用既有布局、即时结果和原密度 |
| B-2 单主题 | 通过；深浅各自渲染，不混主题 |
| B-3 记忆点 | 通过；拒绝时明确坏文件/字段，原备份仍在 |
| B-4 动效气质 | 通过；新增动效 0，原浮面负责结果 |
| C-1 目的频率 | 通过；按操作结果反馈，无额外延时 |
| C-2 CSS all/ease-in/scale(0) | 不适用原生 Go；未增这些效果 |
| C-3 ≤300ms/按压 | 沿用原按压与通知动效，本轮不改时长 |
| C-4 origin | 无新 popover/modal，toast 沿用原固定位置 |
| C-5 reduced motion | 通过；实际用 ReduceMotion=true 绘制所有对照及中文小窗口 |
| C-6 hover media | 不适用 CSS；错误信息不依赖悬停 |
| C-7 属性 / C-8 快触发 | 新增动画属性/重播 0，不改已有按键行为 |
| C-9 手势 / C-10 Motion | 不涉及手势或该依赖 |
| C-11 慢放 | 本轮未修改运动，不声称设备帧率验收；静态结果与恢复事件已验 |
| D2-1 中文字体 | 通过；既有系统字体，外部字体下载 0 |
| D2-2 ≥14px/1.5 行高 | 未达通用建议；旧 Caption 9.9/正文10.8 与行盒仍待拍板，按用户红线保留 |
| D2-3 混排空格 | 沿用中文页面；新的底层诊断是英文原文与文件名，未拼接中文单位 |
| D2-4 全角/AAA | 既有中文标点保留；底层错误维持原样英文，Warning 墨未重测 AAA，不冒称满足 7:1 |

## 仍待

1. 下一轮先复现“导入单条时仓库 enabled 已写回，随后配置读取/写入失败”的部分生效反馈；sync.go applySyncEntry 的顺序是代码线索，不把它记成已确认 bug。不得自行改变覆盖、保存或事务语义；若只能改交互语义则记拍板并换题。
2. 字号/行盒、accent 禁用填充、多草稿覆盖、未知多选/Radio、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾继续待拍板。
3. 原生选择器、Windows/macOS 运行时未验；八条早期 WIP 继续原样保留。

## 完整门槛

| 门槛 | 实测结果 | 证据 |
| --- | --- | --- |
| 基线 verbose | 5513 PASS / 0 FAIL，退出 0 | [日志](P13-jh/baseline-verbose.txt) |
| 最终普通全量 | 退出 0 | [日志](P13-jh/full-test.txt) |
| 最终 verbose 全量 | **5555 PASS / 0 FAIL**，+42，退出 0 | [日志](P13-jh/full-verbose.txt) |
| 测试函数 | **3270**，基线 3258，+12 | [计数](P13-jh/test-functions.txt) |
| 格式 | gofmt 输出 0 字节 | [日志](P13-jh/gofmt.txt) |
| Linux/Windows/Darwin vet/build | 六项均退出 0 | [汇总](P13-jh/platform-gates.txt) |
| 文档契约 | 退出 0 | [日志](P13-jh/docs-ci.txt) |
| Mutation | 2 组均实红，MD5 OK，touch 后 5/6 PASS | [M1](P13-jh/M1-red.txt)、[M2](P13-jh/M2-red.txt) |
| V/B/S | **0/0/0**，旧测试零改；新增 Skip 0 | git diff ac396892 |

完整普通与 verbose 首次全量均通过，无须修改旧 pin 或重跑完整门槛。只有第三批新夹具错误做过必要定向重跑，过程如上。源码与新增测试文件最终 MD5 全部一致。完整日志进仓库，不只留 /tmp。

图像总览与关键浮层细节均亲眼复核；notice-detail-{light,dark}.png 是 4× 最近邻放大，不冒充原生高 DPI。没有宣称通用字号/AAA 门禁全部通过；原约束与未验证运行时见 preflight 和仍待。

本轮已完成八项、跨配置工具条和集成传输两个组件，四批产品每两项提交推送。最终终验用时低于 45–75 分钟目标窗口；未为凑时长加入无关改动或重复测试。最终归档另含恢复验证、文档、日志与截图。
