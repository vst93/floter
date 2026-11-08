# P13-ju · 重连选择的当前归属与修复后的搜索恢复

完成八项，四批产品提交均已推送：13d13fa4 / 75450f68 / 22084c72 / 2ab9e867。最终 **5950 PASS / 0 FAIL / 3385函数**，基线5926 PASS / 3377函数。普通与最终verbose全量exit0，三平台vet/build净，gofmt空。以下保留逐批现象、验证和中断重跑记录。

开盘 HEAD 1a83f975，pull 已同步、工作树干净。Codex 单独执行，读取 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN。无外部模型与代理。指定 done.log 开盘不存在，收尾创建。

## Stash 清点

八条早期 WIP 的 SHA 与 P13-jt 语义审计一致，无新 stale-kill；逐条 reverse-check 退出1，保留原件，不重复接入已归档旧主题/Tauri工作。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/调试，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

基线正在运行。生成工具编辑覆盖保留待拍板；本轮转入已有修复和重连的结果恢复，逐项实测追加如下。

## 第一批：搜索恢复

1. 现象：真实点击重新连接后当前行可用，搜索仍为空。改动：Reconnect 完成调用已有 RefreshIntegrations，投影当前命令并清事实缓存。验证：深浅 Click→完成回调→搜索 root，命令准确1条且可见 Recovered command；改前均红 `reconnect-search completed but search stayed unavailable: commands=0`。
2. 现象：真实点击重新检查清除旧 broken 后搜索仍空。改动：Repair 同步采用完整刷新。验证：深浅当前行不再 broken，搜索命令1条且显示；改前均红 `repair-search completed but search stayed unavailable: commands=0`。

旧V/B/S断言0/0/0。before 为改前真实事件帧，after 在同一夹具下重新生成；定向结果见本批提交正文。

第一批 13d13fa4 已推送，相关8 PASS。开盘全量 **5926 PASS / 0 FAIL / 3377函数**，基线日志完整退出情况收尾核对。

## 第二批：选择目标的声明与包

3. 现象：打开运行时选择后磁盘声明已换 executableNames，旧选择仍重写当前记录并报成功。改动：返回后重读磁盘声明，与打开时比较，过期答案安静退出。验证：真实 Click→选择门控→磁盘更新→回答，深浅仓库字节不变、无旧通知、当前入口可重试。
4. 现象：同ID已指向另一包目录，旧选择仍绑定到新包。改动：同样复核 manifestPath。验证：两主题包路径及记录字节保持，原选择不产生成功通知且释放忙态。

夹具初次编译误引用不存在的 Managed 常量，已改为字段实际字符串值；编译失败不计变异。行为红串与完整验收收尾归档。

第二批 75450f68 已推送。原始红串 `obsolete manifest choice rewrote current repository` / `obsolete package choice rewrote current repository`，旧答案还导致“重新连接”入口消失。第三批把通知采样移到绘制前，避免已消费队列造成漏检；不改旧断言。

## 第三批：选择期间运行时刷新

5. 现象：选择窗口等待时工具已绑回当前程序，旧答案仍覆盖该路径。改动：复核当前 ExecutablePath。验证：深浅实际选择事件，仓库逐字节不变、当前绑定保留、无“已重新连接”假通知；改前红 `obsolete binding choice rewrote current repository`。
6. 现象：运行目录已更新，旧候选仍写入新运行时。改动：复核 RuntimeRoot 以及包/运行版本，选择归属随运行时变化退役。验证：深浅目录与仓库保留，原行解锁可再次重连；改前红 `obsolete root choice rewrote current repository`，并错误显示“检查工具 已重新连接”。

第三批 22084c72 已推送，相关20 PASS。

## 第四批：失效问题的结果退役

7. 现象：选择等待期间 ownership 已改为 managed，旧答案结束后报“检查工具 报告了问题”。改动：选择来源包含 RuntimeOwnership，旧问题安静结束。验证：深浅无仓库写入、done(nil)、无旧失败 toast；改前红 `obsolete ownership choice published a result`，实际错误 `extensions: only a system-owned integration can be reconnected: dev.floter.check`。
8. 现象：选择等待期间工具记录已移除，回答旧问题还在孤包列表上报失败。改动：可读取当前仓库且目标已不存在时，退役原选择；真正的仓库读取失败仍走原错误出口。验证：深浅空仓库字节保留、done(nil)、无过期失败；改前红 `obsolete removed choice published a result`，实际错误 `extensions: no such integration`。包文件保留，所以图中为既有“磁盘上有包，但没有记录”状态，并非空页面。

八项均有独立事件断言，跨集成行操作、运行时选择和启动器搜索。所有before来自对应批次改前状态，不声称来自同一HEAD。旧V/B/S断言修改0/0/0，未新增Skip。

第四批 2ab9e867 已推送，相关26 PASS。四批每两项通过定向test/vet后提交推送；完整门槛另记，不把定向数字冒称全量。

## 变异实跑

| 组 | 注入缺陷 | 原始红串 | 还原 |
| --- | --- | --- | --- |
| M1 | 两处完整刷新退回库存专用刷新 | `reconnect-search completed but search stayed unavailable: commands=0`；`repair-search completed but search stayed unavailable: commands=0` | MD5 OK，touch后6 PASS |
| M2 | 去掉声明与包路径比较 | `obsolete manifest choice rewrote current repository`；`obsolete package choice rewrote current repository` | MD5 OK，touch后6 PASS |
| M3 | 去掉程序与运行目录比较 | `obsolete binding choice rewrote current repository`；`obsolete root choice rewrote current repository` | MD5 OK，touch后6 PASS |
| M4 | 去掉归属与目标移除保护 | `obsolete ownership choice published a result`；`obsolete removed choice published a result` | MD5 OK，touch后6 PASS |

四组缺陷测试均exit1，修复复验均exit0。原始red/restored日志随轮次入库。M1/M2首次恢复复验的进程曾短暂重叠，后续统一在已验证MD5的原文件上再跑本轮全部回归与race，避免把缓存或交错作为最终证据；产品文件最终逐字节还原。

## 偷了什么

- craft-loop 工艺密度菜单 **#4 签名交互**：选择窗口仍是1个；过期答案派发写入0次，结束后当前行可重试1次，新增动画延迟0ms。落在 reconnectIntegration 的原动作链。
- **#6 编辑细节**：当前声明/路径/目录的仓库字节变化0；过期成功/失败通知0条，搜索恢复后准确显示1条命令。落在现有状态行、命令行与通知的内容更新。
- **#12 材质细节**：沿用1px受光边与8px控件圆角，新加玻璃/阴影0层。此项为继承约束，不冒称新视觉制作。
- Apple DESIGN供体：单一交互强调色角色、8px圆角、浮层黑0.22柔影的克制原则；映射既有token，不照搬其网页blur/负中文字距。

为什么这轮更轻、更有空间感：过期提示及时退出浮层，当前状态留在原行，恢复的命令回到原搜索层；没有为异常再堆面板。用户要求修已有功能优先于skill的五种装饰工艺数量，本轮不添加装饰凑数。

## 两轮图像复核

改前事件帧的四视角问题：艺术总监——错误成功toast抢走当前不可用状态的注意力；工程师——设置已恢复而搜索仍空；使用者——选择等待时新绑定被覆盖；读者——已经移除的工具仍说“报告了问题”，不知该操作哪一行。对应修复为失效结果退役、完整刷新、来源复核、安静释放原行。

改后目视：浅色root对照中右上错误成功toast消失，当前“工具不可用”状态和“重新连接”出口同时可见；浅色搜索从空结果恢复为一条Recovered command；深色记录移除对照保留孤包状态并消掉旧错误toast。4×动作条检查原受光边与圆角无新增框层；这是最近邻局部放大，不是高DPI截图。

32张960×1600原始PNG、16张before/after局部对照、2张4×局部在本轮目录。截图为Linux ui.Tester软件自绘；选择窗口使用现有可注入回调安排真实Click后的回答，未捕获原生系统选择窗口，不声称Windows/macOS真机。长画布用于同时看到操作区，未改窗口高度链。

| 项 | 浅色 | 深色 |
| --- | --- | --- |
| 1 重连搜索 | [对照](P13-ju/reconnect-search-light-compare.png) | [对照](P13-ju/reconnect-search-dark-compare.png) |
| 2 检查搜索 | [对照](P13-ju/repair-search-light-compare.png) | [对照](P13-ju/repair-search-dark-compare.png) |
| 3 声明变化 | [对照](P13-ju/choice-manifest-light-compare.png) | [对照](P13-ju/choice-manifest-dark-compare.png) |
| 4 包路径变化 | [对照](P13-ju/choice-package-light-compare.png) | [对照](P13-ju/choice-package-dark-compare.png) |
| 5 程序换绑 | [对照](P13-ju/choice-binding-light-compare.png) | [对照](P13-ju/choice-binding-dark-compare.png) |
| 6 运行目录 | [对照](P13-ju/choice-root-light-compare.png) | [对照](P13-ju/choice-root-dark-compare.png) |
| 7 归属变化 | [对照](P13-ju/choice-ownership-light-compare.png) | [对照](P13-ju/choice-ownership-dark-compare.png) |
| 8 记录移除 | [对照](P13-ju/choice-removed-light-compare.png) | [对照](P13-ju/choice-removed-dark-compare.png) |

## Preflight逐项

| 检查 | 结论 |
| --- | --- |
| A 渐变/纯黑/霓虹/渐变字 | 未新增，沿用已有近黑/浅灰token |
| A 字体换新 | 不适用；中文系统字体优先且本轮禁止扩大字号选题 |
| A 斜体 | 无新增，界面无斜体改动 |
| A Hero居中 | 无Hero，功能设置与搜索 |
| A 三等分/卡片滥用 | 沿用既有行，新增容器0 |
| A 假名/假精确数据 | 测试夹具名称只在测试，不注入产品 |
| A 空洞文案/感叹号 | i18n零改动，删除过期结果而非追加说明 |
| A 眉批 | 无新增 |
| A H1行数 | 既有“集成”标题1行 |
| A Bento空洞 | 无Bento |
| A 左侧装饰竖线 | 无新增 |
| A2 DNA/工艺/两轮 | 继承三个具体DNA；before/after实帧；五种装饰数量服从用户修复主线 |
| A2.1 执行者 | 当前Codex，未调用K3 |
| A2.2 开放创意 | 不适用；单一方向修bug，无随机风格与媒体资产 |
| A3 可读性 | 按钮不换行；历史小字号/次要墨对比限制保留，未冒称全量对比审计 |
| A4 功能契约 | entry→选择→当前条件检查→结果→重试均有事件断言 |
| A4.1 状态/组件 | 沿用原选择器、原行锁与通知；失效、错误、可用状态分开；URL规则不适用原生Go |
| A4.2/4.3/5/7/8 | 无图表、阅读器、四方向、封面任务，不适用 |
| A6 身份边界 | 无署名/二维码/外部浮条 |
| B 设计读取 | 沿用密度7与低动效；没有重绘偏离方向 |
| B 主题 | 深浅独立渲染，无混主题 |
| B 记忆点 | 当前状态在原行，真实完成才在浮层反馈 |
| B 动效气质 | 既有120ms颜色过渡，新增动作延迟0ms |
| C 目的/频率 | 条件更新即时；无新增高频动画 |
| C all/ease-in/scale(0)/fade | 本轮无新增动效，原行入场例外按仓库合约保留 |
| C 时长/按压 | 120ms既有过渡；原按压面保留，4×图已看 |
| C origin | 既有原生选择窗口与通知布局未改 |
| C reduced-motion | 所有新增深浅Tester使用ReduceMotion=true |
| C hover媒体查询 | 非Web，无CSS媒体查询；指针行为未改 |
| C 属性范围 | 无几何动画或宽高动画新增 |
| C 可中断/手势/Framer | 未涉及，框架原实现不改 |
| C 慢放/真机 | 无新增动画；不冒称原生系统弹窗或多平台真机验证 |
| D2 系统中文字体 | 沿用系统字体，无Webfont |
| D2 ≥14px/1.5行高/字重 | 历史Caption9.9/正文10.8与字重580未达通用模板，沿用用户需拍板边界，不重定数值 |
| D2 中西文/数字空格 | 本轮i18n零改动，未新增混排产品文案 |
| D2 标点/暗色7:1 | 沿用全角中文通知；暗色所有次要墨7:1未实测，不声称通过 |

第0步/自进化：用户规则与可复现事件沉淀于本轮spec/report、§6.248及pending；不把项目例外改写成全局skill偏好。以上历史限制明确保留，不以“全门禁通过”概括。

## 门槛与仍待

三平台vet/build均exit0，七份gofmt/vet/build日志均0字节。普通全量exit0；最终单独verbose全量 **5950 PASS / 0 FAIL，exit0**，比基线5926增加24。race为24 PASS / 0 FAIL、exit0；M1/M2在最终MD5还原且touch后的统一补验12 PASS、exit0。8个新测试函数，3377→3385。所有16张对照均已目视（root浅色检查原始两帧）。

汇总门槛的外层执行进程在约5分钟处exit143：普通全量已写下exit0，随后verbose被中断（只有304 PASS的截断日志），因此不计作完成。本轮保留final-interrupted.log，单独重跑verbose全量，不重复已通过的vet/build和普通全量。文档契约追加检查exit0。

调用预算偏差：验收恢复与收尾超过40次调用，本轮未在超额后添加产品选题，只完成已启动门槛的等待、实测记录与归档交付。后续把两个全量测试分开运行，避免汇总命令超过执行时限。

1. 下一轮首选：孤包接管的权限确认期间，磁盘声明或当前记录刷新后旧回答是否仍适用。入口 shell.go 的 AdoptIntegration 与 extensions/orphans.go:29 PrepareAdopt；仅代码线索，先事件复现。若碰到重复ID覆盖/重装拍板边界，记录并另选现有状态问题。
2. 生成工具编辑/转换的整包覆盖仍待拍板，本轮未实现；配置模板失败重试、176-9/161-4保存期间草稿、字号/行盒、accent禁用面、未知多选/Radio与终端S1b/S1c/S2尾沿用。
3. 本轮不引入跨进程事务锁；保护的是用户等待选择窗口期间发生的目标变化。读取检查与后续已有写回之间的跨进程原子性未承诺，也未改仓库架构。
4. 八条早期WIP原件保留，未发现新stale-kill；上轮79ad8d537b归档不重复接入。
