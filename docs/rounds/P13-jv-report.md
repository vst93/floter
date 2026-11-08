# P13-jv · 孤包接管确认的当前条件与退出恢复

八项完成，最终 **5974 PASS / 0 FAIL / 3393测试函数**（基线5950/3385，+24 PASS/+8函数）。普通与verbose全量均exit0，三平台vet/build净、gofmt空；race24 PASS，四组mutation实红并MD5还原、touch后各6 PASS。最新产品56e378ce；过程中的提前推送、合批与漏暂存已补修，详见下文，不能宣称流程完全合规。

开盘7ab8b039，pull同步，工作树干净。Codex独立执行，已读qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography与Apple DESIGN。指定done.log开盘不存在。产品和验收按批增量追加。

## Stash 清点

八条SHA与P13-ju一致，无新stale-kill。reverse-check均退出1；沿用已归档语义审核保留旧WIP，不恢复旧主题/Tauri发布链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留原件 |
| stash@{1} | 8af94a07 | 旧行距/slider，reverse=1 | 保留原件 |
| stash@{2} | 7e4aa710 | 旧卸载度量/调试，reverse=1 | 保留原件 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留原件 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留原件 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留原件 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留原件 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留原件 |

## 当前验证

全量基线运行中，八项选题见spec。为什么更轻/更有空间感：等待状态留在原行，过期回答不占浮层，当前目标重新成为唯一注意中心；不堆叠材质。

## 第一批：孤包行等待状态

1. 现象：接管正在等待时再次点击仍派发，深浅原始红串 `pending orphan permits repeat: adopts=2 deletes=0`。改动：孤包操作接入行锁，接管使用既有忙态胶囊。验证：真实Click后第二次派发0次，结束后可重试；既有行互斥回归不改断言。
2. 现象：同一行等待期间删除仍派发，红串 `pending orphan permits delete: adopts=1 deletes=1`。改动：删除入口遵循同一个行锁，宿主也拒绝重复/竞争调用。验证：真实Click期间删除0次，结束后删除1次；深浅PNG已生成。

旧V/B/S断言0/0/0，新Skip0。定向test/vet通过后立即提交推送，计数见提交正文。

第一批过程偏差：84825694在定向测试exit1后被错误继续提交推送，正文中的vet净不成立（test失败后vet未执行）。新增两项测试通过，但旧PendingOutletRepairOrphan/ToggleOrphan断言抓到“已安装→孤包”转换后的旧行忙态阻断新出口。补修将孤包门控限定为孤包自身操作，使用既有disabledActionButton熄灭冠线；新夹具改为真正的孤包等待态。旧B断言不改。后续提交命令由同一shell的test && vet && commit && push严格串联。

72ac1237补修已推送，相关57 PASS、settingsui/shell vet净。开盘完整基线5950 PASS/0 FAIL，3385测试函数。

## 第二批：权限回答的声明归属

3. 现象：权限窗口等待时manifest新增clipboard-read，旧回答仍登记并批准新权限。改动：仅PrepareAdopt产物在Commit复核当前manifest摘要。验证：实际接管Click→权限门控→磁盘修改→回答，仓库字节不变、无成功/失败旧通知、原行可重试。M2原始红串 `obsolete permissions adoption rewrote current repository`。
4. 现象：package.json转向新的current.json，旧回答仍接管旧声明。改动：复核当前解析路径、包名与版本。验证：同样真实事件中保留当前包和仓库，不派发过期结果，接管入口恢复。M2红串 `obsolete package adoption rewrote current repository`。

事件夹具最初点到推荐工具的同名接管按钮，失败 `orphan click dispatched to ""` 不算产品证据；移除夹具推荐区后才进行有效before/after。M2正式注入保护缺失后两场景深浅实红，MD5还原并touch后6 PASS；扩展审批回归和vet净。

M2第一次夹具运行时不可用，只复现过期失败通知，脚本因未观察到写盘红串拒收；将fixture runtime名称设为真实测试程序路径后重跑，不改产品语义或断言。

过程偏差补录：仅修改runtime名称仍不足，ResolveRuntime使用StubToolLookup而非StubToolCandidates。第二批提交因M2验收拒收未执行；随后97444804错误合入中间四项，且漏暂存orphans.go的声明，形成短暂不完整提交。立即以单独修正提交补齐并重跑M2/M3；不将该过程说成四批严格每两项完成。最终产品证据以修正夹具后的日志为准。

## 第三批：孤包身份刷新

5. 现象：权限确认期间另一来源登记了同ID，旧接管仍能重写其记录。改动：仅接管Commit再次核对当前仓库中确实无该ID。验证：当前disabled记录逐字节保留，结束后显示已安装行，无旧接管通知。
6. 现象：权限确认期间包目录消失，旧回答报安装失败且孤包卡仍在。改动：提交前复核目录存在，ErrNoOrphan作为失效问题退出并刷新列表。验证：深浅目标行消失、仓库不改、旧失败通知0条。

真实Click→确认门控→磁盘变化→答案返回；第三批红串、MD5还原、touch复验见M3原始日志。普通安装重复ID规则未变。

8bfc5c98补齐后M2/M3均实红并逐字节还原，touch后各6 PASS；真实写盘红串分别为 `obsolete permissions/package/record adoption rewrote current repository`；移除为 `obsolete removed adoption published a result` 与 `removed package remains on screen`。

## 第四批：取消与失败出口

7. 现象：权限窗口选取消后显示“安装失败”。改动：不再以拒绝答案调用Commit，刷新当前事实后释放行。验证：取消仓库不改、通知0条、可以再次接管，重复宿主派发也只打开1个确认。
8. 现象：删除孤包遇到仓库读取失败只有日志，页面无失败反馈。改动：复用已有IntegrationRemoveFailed通知。验证：真实删除Click遇到损坏仓库，恰好1条失败结果、包保留；修好仓库后同一入口重试成功、孤包卡消失。

第四批以M4注入缺陷取得深浅before，修复后重新渲染；实测与提交记录收尾追加。

第四批复验还抓到失败后Refresh损坏仓库把原孤包行清掉，重试按钮消失；第8项一并修正为通知后返回，保留已知原行，defer释放忙态。第一次M4还原日志exit1保留说明，不计通过。

第四批56e378ce已推送；M1/M4最终均实红、MD5还原、touch后各6 PASS，settingsui/shell vet净。

## 变异与证据边界

| 组 | 注入 | 原始红串 | 还原 |
| --- | --- | --- | --- |
| M1 | 关闭孤包两按钮的忙态分支 | `pending orphan permits repeat: adopts=2 deletes=0`；`pending orphan permits delete: adopts=1 deletes=1` | MD5 OK、touch后6 PASS |
| M2 | 跳过接管条件复核 | `obsolete permissions adoption rewrote current repository`；`obsolete package adoption rewrote current repository` | MD5 OK、touch后6 PASS |
| M3 | 跳过孤包身份复核 | `obsolete record adoption rewrote current repository`；`obsolete removed adoption published a result` | MD5 OK、touch后6 PASS |
| M4 | 取消仍提交；删除失败不通知 | `obsolete cancel adoption published a result`；`orphan deletion failure lost result` | MD5 OK、touch后6 PASS |

深浅before为同一事件夹具在各组缺陷注入时的实际ui.Tester帧，after为逐字节还原修复后重新运行；不是声称32张来自同一旧HEAD。四组完整red/restored日志、32原图和16对照在P13-jv目录。平台范围：Linux自绘与真实临时磁盘变化；原生系统权限对话框由确定性确认门控替代，未做Windows/macOS真机操作。保护用户等待期间的条件变化，不承诺跨进程原子事务。

## 偷了什么

- craft-loop 工艺密度菜单#4「签名交互」：等待时重复派发0次，宿主重复派发只保留1个确认；结束后原按钮可再派发1次，新增等待延迟0ms。复用busyActionButton的原行指示。
- #6「编辑细节」：取消与失效回答的结果文字0条；真正删除失败恰好1条现有通知。单一浮层承载结果，原行保留上下文，不新增常驻状态栏。
- #12「材质细节」：禁用按钮走已有DimFace，冠线关闭，框架只乘1次0.5；接管与删除共用30u胶囊，不叠第二层透明度。
- Apple DESIGN借用的是单一交互色、1px边界与5–8px容器圆角层级的克制原则；项目既有15u胶囊与theme token优先，不照搬纯黑、blur或网页字体。#5的两层浮影沿用原通知，不增加新的影面。
- 用户限定修已有功能优先，未为凑工艺密度五项增加纹理、插图或新动效。

## Preflight逐项

| 项 | 结论 |
| --- | --- |
| 0 偏好与缺陷记录 | 已读账本，真实缺陷写spec/report/合约；项目边界不升级为全局skill偏好 |
| A 紫蓝渐变/纯黑/霓虹/渐变字 | 无新增，保留近黑与浅灰token |
| A 字体每次换新 | 不适用；中文系统字体规则与用户既有方向优先 |
| A 斜体 | 无新增 |
| A 居中Hero | 功能管理页，无Hero |
| A 三等分/卡片滥用 | 新容器0，复用原行 |
| A 假名/精确假数据 | 夹具只在测试；不注入产品 |
| A 空洞文案/感叹号 | i18n零改动，复用既有失败文案 |
| A 眉批 | 新增0 |
| A H1行数 | 既有“集成”标题1行 |
| A Bento空洞 | 无Bento |
| A 左侧装饰线 | 新增0 |
| A2 工艺/DNA/两轮 | 真实状态前后帧；第二轮修退役记录忙态和删除失败后的原行丢失 |
| A2.1 执行者 | 当前Codex，无K3或子代理 |
| A2.2 开放创意 | 单一方向修bug，不适用随机风格或媒体生成 |
| A3 主CTA/纹理 | 原按钮文案不换行，新纹理0；历史次要墨对比限制保留 |
| A4 核心任务 | 接管→等待→结果/取消→重试均有真实Click断言 |
| A4.1 状态/组件/URL | 原按钮及原通知，禁用有结束出口；原生Go无URL状态规则 |
| A4.2/4.3 | 无图表或阅读器，不适用 |
| A5 四方向 | 用户指定唯一方向并禁止HTML预览，不适用 |
| A6 身份边界 | 无署名、二维码或外部浮条 |
| A7/A8 封面 | 无媒体封面，不适用 |
| B 方向/拨盘 | 视觉冒险3、动效2、密度7，保留原布局 |
| B 主题 | 深浅分别渲染，不混主题 |
| B 记忆点 | 原行说明正在处理谁，结果只在必要时出现 |
| B 动效气质 | 忙态用既有轻量指示，新增延迟0ms |
| C 目的/频率 | 等待指示只在操作在飞；键盘模型不变 |
| C all/ease-in/scale(0)/纯fade | 本轮无新增动效实现 |
| C 时长/按压 | 复用原120ms颜色过渡与按压面；未新增几何动画 |
| C origin | 原生确认窗口与既有通知布局不变 |
| C reduced-motion | 所有新增Tester使用ReduceMotion=true |
| C hover媒体查询 | 非Web；原生指针能力由框架处理 |
| C 动画属性 | 无新增宽高/边距动画 |
| C 快速触发 | 同行互斥，重复操作0次，无重播keyframes |
| C 手势/拖拽 | 未涉及 |
| C Framer高负载 | 未使用 |
| C 慢放/真机 | 没有新动画；不冒称系统弹窗与三平台真机验证 |
| D2 系统字体 | 沿用系统中文字体，不加Webfont |
| D2 ≥14px/1.5/字重 | 历史Caption9.9/正文10.8与580字重未满足通用模板，按用户拍板边界保留 |
| D2 中西文/数字空格 | i18n零改动，无新增混排文案 |
| D2 全角/7:1 | 现有中文通知；所有次要墨7:1未实测，不声明全部通过 |

## 门槛与仍待

完整门槛已完成：普通全量exit0、verbose **5974 PASS / 0 FAIL、exit0**，测试函数3393（+8）。race24 PASS/0 FAIL、exit0；三平台vet/build均exit0，七份gofmt/vet/build日志均0字节。文档合约测试已实际运行并通过（首个过滤器未匹配测试，随后改为真实测试名重跑，最终日志有PASS，不计空跑）。旧V/B/S断言修改0/0/0；新增Skip0。提交节奏有上述偏差，不宣称完全符合每1–2项门槛。

最终四份MD5再核对全部OK。补截图的200ms只改测试采样时间，M2/M3/M4在此修改后均复验各6 PASS；产品文件与全量编译时相同。所有门槛日志与状态文件在 `P13-jv/logs/`，before/after原始帧在 `P13-jv/`。

下一轮首选：现有普通本地/推荐包安装的权限确认期间声明变化是否有同源问题。仅代码线索，先真实事件复现；普通安装重复ID覆盖、重装事务继续需拍板，本轮保护只限接管。字号、保存草稿归属与终端S1b/S1c/S2尾沿用，不开新系统。

视觉复核：深浅等待图中接管前出现spinner、同行删除冠线熄灭；权限/换入口场景从错误的已安装卡回到可重试孤包卡；新记录保留停用状态；目录消失回到空态。通知在右上角，最初overview只裁行区漏掉通知，后已改为“右上通知+原行”的组合裁切；原始完整PNG始终保留。取消前后的旧失败与删除失败新增通知分别核对。4×局部只检查已有按钮边界，不据此宣称字号通过通用14px模板。

| 逐项对照 | 浅色 | 深色 |
| --- | --- | --- |
| 1 接管等待 | [图](P13-jv/repeat-light-compare.png) | [图](P13-jv/repeat-dark-compare.png) |
| 2 删除互斥 | [图](P13-jv/delete-light-compare.png) | [图](P13-jv/delete-dark-compare.png) |
| 3 权限变化 | [图](P13-jv/permissions-light-compare.png) | [图](P13-jv/permissions-dark-compare.png) |
| 4 包入口变化 | [图](P13-jv/package-light-compare.png) | [图](P13-jv/package-dark-compare.png) |
| 5 新登记记录 | [图](P13-jv/record-light-compare.png) | [图](P13-jv/record-dark-compare.png) |
| 6 目录消失 | [图](P13-jv/removed-light-compare.png) | [图](P13-jv/removed-dark-compare.png) |
| 7 取消 | [图](P13-jv/cancel-light-compare.png) | [图](P13-jv/cancel-dark-compare.png) |
| 8 删除失败 | [图](P13-jv/delete-failure-light-compare.png) | [图](P13-jv/delete-failure-dark-compare.png) |

艺术总监视角：原行不能又亮又不可点，改用已有DimFace；工程师视角：旧记录忙态不能阻断新孤包键盘出口，旧B回归已守住；任务所有者视角：删除失败应留在原行重试，实际事件已过；读者视角：错误与取消不能同一个红通知，深浅对照已逐张确认。第一轮渲染、批评与第二轮修正均沿原方向，没有新增装饰。

调用预算偏差：超过40次后只完成既有八项的门槛、图像和归档，没有添加新产品选题。截图补加200ms Tester虚拟推进用于捕获通知稳定帧，不添加产品延迟。补截图时变异进程与全量进程有重叠；进程清单证明两个全量shell.test及race shell.test均在变异启动前已运行（3:11/3:09/2:09对0:52），加载的是修复后的测试二进制；最终以完整退出状态及补截图定向复验共同验收，不把交错当作额外变异证据。
