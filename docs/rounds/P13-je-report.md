# P13-je：配置刷新后的动作有效性

开盘 353032fa，pull 已同步、工作树干净。基线中文 locale 5407 PASS / 0 FAIL，cmd/internal 3226 测试函数。done.log 不存在。

**最终交付九项**：第 1–6 项为配置目标刷新后的复制/导出/保存保护，第 7–8 项为第五批的配置/集成导出文件写入错误，第 9 项是目视发现的长路径撑走浮层关闭键。第四批工具条锁定已撤回，过程保留，不计完成。

## Stash 清点

八条与上轮同 SHA，均非新增 stale-kill，reverse-check 全部退出 1。沿用已核验的历史处置，不把旧主题与 Tauri 工作覆盖进当前 Go 实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse exit 1 | 保留 |
| stash@{1} | 8af94a07 | 旧间距，reverse exit 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载指标/临时测试，reverse exit 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse exit 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse exit 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验，reverse exit 1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse exit 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse exit 1 | 保留 |

规格见 P13-je-spec.md；以下按批次追加实测证据。

## 第一批

1. 复制旧入口：现象 → provider 撤回 configuration 或停用后，实际按钮事件静默返回 nil。改动 → 在宿主读取当前配置，失效时原 toast 显示拒绝并回调错误。验证 → 原代码 `stale copy silently succeeded after missing/disabled refresh` 实红；无剪贴板副作用，恢复后可再次复制。
2. 导出旧入口：现象 → 同样刷新后无选择器、无任何结果。改动 → 同一当前配置校验给可见拒绝，解锁原按钮。验证 → 原代码 `stale export silently succeeded after missing/disabled refresh` 实红；路径选择器零次，恢复后可导出。

空 schema 原本被未知字段校验拒绝，保留回归但不计修复。夹具首次编译误用 FindText、随后误点内置插件 Configure，均已修正；只有目标集成的真实按钮事件失败计入产品证据。

第一批定向 32 PASS / 0 FAIL，shell vet 净，e98caabc 已推送。

## 第二批

3. 复制 ownership：现象 → provider 改由 tool 持有后，旧复制仍写剪贴板。改动 → 沿既有 tool-owner 语义在宿主拒绝，提示打开工具配置。验证 → `stale copy reached output: picker=0 clipboard=1` 原事件实红；修后不输出、不改草稿，恢复 host 后可重试。
4. 导出 ownership：现象 → 相同刷新后仍打开选择器、落文件。改动 → 同一宿主 owner 校验拦在选择器前。验证 → `stale export reached output: picker=1 clipboard=0` 原事件实红；修后选择器零次，恢复 host 后导出一次。

第二批定向 38 PASS / 0 FAIL，shell vet 净，ee3e1107 已推送。

## 第三批

5. 保存配置撤回：现象 → 点击保存时 provider 已撤回配置，空指针使动作崩溃。改动 → 保存接同一个当前配置校验，错误回到原表单错误带。验证 → 旧事件 `stale save panicked after missing refresh: runtime error: invalid memory address or nil pointer dereference` 实红；拒绝后磁盘原配置逐字节未变。
6. 保存 ownership：现象 → provider 改成 tool 后，旧保存仍写入磁盘。改动 → 同一 host 条件阻止该写入。验证 → `stale save changed stored configuration: <nil>` 原事件实红；修后非 nil 错误可见、文件字节不变，不调整保存时机。

## 第四批

**本批产品锁定已撤回，不计最终九项。** 全量既有 B pin `TestOptionExitSelectDoesNotReclaimFocus/{light,dark}/busy` 报 `cannot Tab to Copy configuration`，证明原复制工具条是明确保留的键盘出口。恢复原产品行为，旧 B 断言零改；本轮新增的两条错误忙态假设改为保留出口的回归。下述记录仅保留过程证据，最终第 7/8 项由第五批替换。

7. 行忙态复制：现象 → 真实 Recheck 在飞时复制图标仍亮、可派发。改动 → 工具条读取既有 configLocked/rowBusy，与字段和底部动作同锁。验证 → 原事件 `busy row dispatched copy-row-busy 1 times` 实红；修后零次，修复失败返回后可再复制一次。
8. 行忙态导出：现象 → 同一在飞状态下可打开第二个导出动作。改动 → 导出与复制共享原行锁与 DimFace。验证 → `busy row dispatched export-row-busy 1 times` 实红；修后零次，行解锁后可导出一次，目标和草稿不变。

最终九项跨配置表单动作、集成导出与错误浮层三个既有组件；设置页忙态工具条保持原行为。未新增系统/依赖/设置项，旧 V/B/S 断言修改 0/0/0，新 Skip 0。

## 第五批（最终第 7/8 项）

7. 配置导出写入失败：现象 → 选择器返回目录时，真实 os.WriteFile 返回 is a directory，界面却提示集成文件格式错误。改动 → 宿主 toast 展示原路径/系统原因。验证 → 旧事件 `config-write hides actual filesystem failure`、`config-write wrongly reports an invalid import document` 实红；修后错误可读，回调保留原错，换有效路径可重试写出文件。
8. 集成导出写入失败：现象 → 同样真实文件系统失败被说成导入文档格式错误。改动 → 集成传输出口显示原写入原因。验证 → 旧事件 `integrations-write hides actual filesystem failure` 实红；修后错误可读，忙态释放、有效路径重试写出文件。

最终八项跨配置工具条/保存表单与集成整体导出两个已有组件；行忙态工具条保持开盘行为，第四批两项不计修复。没有新增系统或依赖。

## 第六批（目视追加）

9. 长路径错误浮层：现象 → 最新写入错误截图中，长文件名把关闭键撑到窗口外；旧测试实报 `long path overlaps dismiss or viewport: body={414.09998 65.799995 305.90002 51} dismiss={1085.3 80.5 0 0}`，关闭事件报 outside the window。改动 → 原 toast 文本列 `MinWidth(0)`，按剩余宽度换行，保留全文。验证 → 深浅全文存在，文本不跨关闭键、关闭键在720px窗口内，实际点击能移除通知；短 toast、动作与原层级回归通过。此项不改固定高度链，使用原多行错误浮层的自适应高度。

第五批 **51 PASS**、第六批 **27 PASS**，各自 shell vet 净；552451e5 / ce2ee6c6 已推送。恢复旧键盘出口的 ffcd6850 **40 PASS**。后续全量覆盖最终九项；长路径加入前启动的普通全量另保留，不混成最终版本的普通全量。

第三批定向 **45 PASS / 0 FAIL**、第四批 **84 PASS / 0 FAIL**，各自包 vet 净；f9e52bef / 5cb111f3 已推送。每批完成两项就提交，没有把产品改动攒到轮末。

## Mutation

- M1 回注开盘 sync.go 与 shell.go，六项目标刷新事件实红。原始串包括 `stale copy silently succeeded after missing refresh`、`stale export reached output: picker=1 clipboard=0`、`stale save panicked after missing refresh: runtime error: invalid memory address or nil pointer dereference`、`stale save changed stored configuration: <nil>`。空 schema 的原保护仍绿。还原两个文件并 MD5 核对 OK，touch 后 **32 PASS**。
- M2 历史忙态假设：曾回注开盘 settingsui/config.go 并实红，MD5 核对、touch 后 6 PASS；后被既有 B pin 证明该假设不合法，**作废，不计最终 mutation**。日志仅保留过程。
- M3 最终文件错误映射：回注 ffcd6850 的 sync.go，两类导出深浅实红，原串 `config-write hides actual filesystem failure` / `integrations-write hides actual filesystem failure`；MD5 还原 OK、touch 后 **6 PASS**。最终 M1 也在 552451e5 上重新实跑，32 PASS 还原。

全部 -count=1；原始红/还原日志与 md5 文件入轮次目录。未改旧断言、未跳过失败。

## 偷了什么

实际读取 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography，以及 Apple DESIGN 的色彩/控件段。用户已确定方向，采用打磨路径。

- craft-loop 菜单 **#4 签名交互**：拒绝进入原反馈出口，新增动效 **0ms**；失效动作副作用 **0 次**，恢复后传输 **1 次**。没有为后台条件变更再开确认窗。
- **#12 材质细节**：新拒绝接原错误带的 **1px** 边、**8px** 圆角，沿用原 toast 浮面，没有新增玻璃层。供体 Apple 控件 8px 与仓库已有 RadiusSM 一致；工具条材质最终没有改动。
- **#6 编辑细节**：错误仅在原 toast/表单带 **1 处**出现，静息常驻说明新增 **0 行**；旧字段与草稿保留，不堆双重状态说明。
- **#3 品牌 focus 环**：复用现有单一 accent 与既有焦点出口，新增色值 **0**、新键盘模式 **0**。Apple 的单一交互色只作为角色参照，未复制其品牌蓝。

工艺密度采用四项；用户禁止无关扩张优先，不为凑五项增加装饰。**为什么更轻、更有空间感**：失效动作及时结束，只在已有表单错误带或浮层给出一处准确反馈；没有新增常驻说明和卡片层，原有 E1/E2 层级保持。

## 深浅图与目视复核

第一组旧实帧的四视角问题：艺术总监看到错误只能靠旧泛化 toast 区分；工程师看到保存一直停在 Saving；任务所有者点击后没有可见结果；读者在 tool owner 改变后却看到了复制成功。第二组修复帧逐一比对：保存出现原错误带与恢复按钮，传输拒绝给出下一步，文件错误出现路径与原因。曾认为亮工具条不合适，但行为契约要求其作为出口，最终保留；没有以视觉理由覆盖它。

亲自检查深浅两套：深色保存旧帧左侧卡在 Saving，新帧右侧显示原 draft 和错误带；浅色导出撤回出现原浮层拒绝，owner 改变不再显示复制成功。主布局保持原有密度，错误带使用原布局插入规则，不修改高度链常量。忙态灰墨图只属于撤回尝试，不能作为最终产品效果。

最终 36 原帧 + 18 深浅前后对照入目录，每对差异非零；完整读数见 image-differences.txt。路径含测试临时目录，因此不把差异像素数当固定视觉 pin。代表图 `/tmp/floter-P13-je-config-{dark,light}.png`。旧忙态图是撤回尝试的过程附件；原生倍率文件保留，review 图仅缩小用于总览。追加长路径浅色实帧中，全文折成多行，关闭叉重新落回浮层右侧。

| 项 | 深色对照 | 浅色对照 |
| --- | --- | --- |
| 1 复制撤回 | [图](P13-je/copy-missing-dark-comparison.png) | [图](P13-je/copy-missing-light-comparison.png) |
| 2 导出撤回 | [图](P13-je/export-missing-dark-comparison.png) | [图](P13-je/export-missing-light-comparison.png) |
| 3 复制归属 | [图](P13-je/copy-tool-dark-comparison.png) | [图](P13-je/copy-tool-light-comparison.png) |
| 4 导出归属 | [图](P13-je/export-tool-dark-comparison.png) | [图](P13-je/export-tool-light-comparison.png) |
| 5 保存撤回 | [图](P13-je/save-missing-dark-comparison.png) | [图](P13-je/save-missing-light-comparison.png) |
| 6 保存归属 | [图](P13-je/save-tool-dark-comparison.png) | [图](P13-je/save-tool-light-comparison.png) |
| 7 配置写入错误 | [图](P13-je/config-write-dark-comparison.png) | [图](P13-je/config-write-light-comparison.png) |
| 8 集成写入错误 | [图](P13-je/integrations-write-dark-comparison.png) | [图](P13-je/integrations-write-light-comparison.png) |
| 9 长路径关闭 | [图](P13-je/long-path-dark-comparison.png) | [图](P13-je/long-path-light-comparison.png) |

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，原 token 不改 |
| A2 非系统字体 | 不采用；中文系统字体与仓库契约优先 |
| A3 斜体 | 无新增 |
| A4 Hero | 原生设置无 Hero，不适用 |
| A5 三等分/卡片海 | 无新增容器 |
| A6 假数据 | 测试用真实临时 provider、配置文件与按钮事件，不进入产品指标 |
| A7 空泛文案 | 无口号；新宿主错误给刷新/打开工具配置的下一步，i18n 字典未改 |
| A8 编号眉批 | 产品无新增 |
| A9 标题行数 | 原页头完整，未改 |
| A10 Bento | 不适用 |
| A11 左侧装饰线 | 无新增 |
| B1 读取/拨盘 | 2/1/6：保守、即时、原密度 |
| B2 主题 | 深浅独立实帧，无主题混用 |
| B3 记忆点 | 当前配置决定动作能否执行，拒绝与恢复可见 |
| B4 动静气质 | 原静态状态切换，不添动画 |
| C1 高频操作 | 新增动效 0ms |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 原按钮反馈不改 |
| C4 origin | 原 toast/行内带；无新弹层 |
| C5 reduced-motion | 测试帧 ReduceMotion=true |
| C6 hover 查询 | 原生指针路由；CSS media 不适用 |
| C7 布局动画 | 无新增 |
| C8 keyframes | 无新增 |
| C9 手势 | 不涉及拖拽/滑动 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新动效；未宣称真机慢放 |
| D2-1 字体 | 系统字体不改，无 webfont |
| D2-2 字号/行盒 | 旧 Caption 9.9/正文 10.8 不满足通用 ≥14px/1.5，沿用需拍板 |
| D2-3 中西空格 | 原字典未改，宿主技术错误保持英文原文 |
| D2-4 标点/对比 | 原 token 不改；不宣称所有安静墨达到 7:1 |

通用 skill 的五项密度/字号/全局记忆要求未照搬：本仓用户合约优先；缺陷规则落 §6.232，不向全局偏好写产品规则。Linux ui.Tester 与真实 POSIX provider 已验；原生桌面与 Windows/macOS 运行时未验。

## 仍待

1. 下一轮首选集成导入读文件和导出读取仓库时的失败出口：sync.go 的 os.ReadFile / extensions.ExportSync 错误仍显示 IntegrationsTransferBad。只是代码线索，先用实际失效路径/仓库事件复现，不重复本轮配置目标条件与文件写入错误。
2. 字号/行盒、accent 禁用填充、多草稿覆盖、未知多选/Radio、极小步长、176-9/161-4 保存期草稿、终端 S1b/S1c/S2 尾继续需拍板。确认窗口、保存时机和会话语义未动。
3. 8 条早期 WIP 继续保留；无新活动抢救 stash。系统剪贴板与原生选择器的运行时验证未覆盖。

## 过程记录

开盘读取过宽导致输出截断，若干猜测文件名不存在；后来按已定位符号收窄。底层工具调用计入读取、轮询、mutation、图像与提交后超过 40 次预算；八项完成后，目视验收发现长路径关闭入口越界，补修为第九项，其后仅执行必要门槛和归档。不声称流程全部合规。

## 最终门槛与交付

最终产品为 **ce2ee6c6**，下表均覆盖其全部九项修改。统一 `LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4`，普通与 verbose 全量分别运行，退出码均为 0。

| 检查 | 最终实测 | 证据 |
| --- | --- | --- |
| 全量 verbose | **5454 PASS / 0 FAIL**，基线 5407，增加 47 | full-verbose.txt / baseline.txt |
| 测试函数 | **3237**，基线 3226，增加 11 | cmd/internal 的 func Test 计数 |
| 普通全量 | 退出 0 | full-test.txt |
| gofmt | 输出为空 | gofmt.txt |
| Linux / Windows / Darwin vet | 全部退出 0、输出为空 | vet-{linux,windows,darwin}.txt |
| Linux / Windows / Darwin build | 全部退出 0、输出为空 | build-{linux,windows,darwin}.txt |
| 最终相关回归与截图 | **41 PASS / 0 FAIL** | final-related.txt |
| 目标刷新/原忙态出口 race | 退出 0 | race.txt |
| 文件写入错误 race | 退出 0 | write-race.txt |
| 文档契约检查 | 退出 0 | docs-ci.txt |
| 最终产品源码 MD5 | 四个文件全部 OK | final-source.md5 |
| 有效 mutation | **2 组**实红、MD5 还原、touch 后 **32/6 PASS** | M1-* / M3-*；M2 作废 |

基线全量从 **15:02:13 UTC** 开始，最终门槛于 **15:59:26 UTC** 完成，间隔 **57 分 13 秒**；此区间不含更早的开盘读取和随后文档归档时间。测试函数未减少，旧 V/B/S 断言修改 **0/0/0**，新增 Skip **0**。36 张最终原帧与 18 张深浅对照图已归档；8 条 stash 均保留。

有效产品分批提交并推送：**e98caabc / ee3e1107 / f9e52bef / 552451e5 / ce2ee6c6**；忙态尝试 **5cb111f3** 由 **ffcd6850** 撤回。最终没有改变原键盘出口、确认时限、保存时机或终端会话语义。跨平台结论限于 vet/build，原生 Windows/macOS 交互未实测。
