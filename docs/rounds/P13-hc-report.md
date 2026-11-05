# P13-hc · 设置编辑中的输入与反馈归属

起点 cb345a63，2026-11-05；pull --ff-only 已同步、工作树空。唯一执行者 Codex，无外部模型和子代理。done.log 开盘不存在，收尾创建。

## stash 清点

以下均已实际 reverse-check，全部失败。按 P13-gq/ha/hb 入库的语义审核，属于早期独立 WIP，不能把旧主题和旧 Tauri 发布链覆盖当前实现；保留原件。任务书所指 stale-kill 已由前轮归档处理，当前没有新 orphan。

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

## 增量验收

当前中文 locale 全量基线 **3608 PASS / 0 FAIL**，旧测试函数实测 **2719**。原始日志 `/tmp/floter-P13-hc-baseline.log`。

1. **类型切换后命令输入丢失**：选中 Command 后切换脚本模式，继续输入仍是 weather → Command 行使用稳定语义身份 → 深浅真实输入均变成 forecast。原始红串 `mode switch lost command edit: "weather"`。
2. **类型切换后版本输入丢失**：选中 Version 后切换模式，输入仍是 1.2.3 → Version 行保持自己的身份 → 输入恰为 2.0.0。原始红串 `mode switch lost version edit: "1.2.3"`。

第一批相关回归 **33 PASS / 0 FAIL**，internal/ci 通过。V/B/S 0/0/0，旧断言零改。深浅 Command 前后 PNG 已实际导出。校验提示插入和 Advanced 展开两候选实测通过，未计入修复。

3. **撤销带回上一工具名称**：第二个工具点 Undo 得到 First → 表单每次 beginEdit/endEdit 分配独立编辑身份 → 第二工具旧历史为空、自己的编辑仍能撤销回 Second。原始红串 `new tool inherited text undo: "First"`。
4. **界面翻译变化后参数输入丢失**：英文参数 Label 全选后切换中文，输入仍是 Target → 六个参数字段使用不依赖翻译的语义 key → 深浅两套均接受 Destination，类型切换与删除参数的旧回归保持。

第二批相关回归 **37 PASS / 0 FAIL**；旧断言零改，V/B/S 0/0/0。首批 a70e682f 已推送。参数语言切换深浅前后图已导出。

5. **删前一条后录制改写错误绑定**：原第二条录制中删除第一条，Ctrl+Shift+R 被写到 clipboard 那条 → 删除/转为命令草稿时同步录制行索引 → 键只写到原 say original 绑定，clipboard 保持 Ctrl+Shift+C。原始红串 `recording moved to another binding: [{Key:Ctrl+Shift+B Action:say original} {Key:Ctrl+Shift+R Action:plugin:clipboard}]`。
6. **删除正在录制的行仍继续录制**：原第一条被删，后续按键改写存活的第二条 → 删除录制对象立即取消录制 → 录制提示消失，余下两条键值与草稿都保持。原始红串 `removed binding left recorder armed`、`removed recorder rewrote surviving binding`。

第三批相关快捷键回归 **23 PASS / 0 FAIL**。删除和「转为命令草稿」两条移除路径共用修复，键盘格式/持久化回调保持。V/B/S 0/0/0。

7. **冲突提示跑到下一条绑定**：第二条提示沿用旧下标，删除前一条后警告出现在 clipboard 行 → 同步提示索引，删除提示对象清空提示 → 深浅两套只在原命令行显示冲突，删除该行后提示消失。原始红串 `conflict moved to another binding: row=1`、`removed binding left conflict behind`。
8. **删除前一绑定后命令编辑丢失**：选中 say original 后删前一行，输入和 Enter 没有更新命令 → 保存行按绑定 key、草稿行按固定身份构建，cardRow 内部主体也保持身份 → 深浅实际输入提交 say edited，clipboard 不变。原始红串 `remaining command lost edit: [{Key:Ctrl+Shift+B Action:say original} {Key:Ctrl+Shift+C Action:plugin:clipboard}]`。第一版仅给外行加 key 仍红，定位为首行取消分隔线导致内部主体次序改变；修主体身份后通过，分隔线像素与尺寸不变。

第四批回归 **57 PASS / 0 FAIL**；V/B/S 0/0/0。前三批 a70e682f、8f3dd208、f32742e9 均已推送。八项均有原实现红测与修后断言，跨主表单、参数板、快捷键录制与编辑组件。

## 正式 mutation

- M1 撤掉 Command/Version 的语义身份：`mode switch lost command edit: "weather"`（深浅）、`mode switch lost version edit: "1.2.3"`；退出 1。
- M2 不迁移录制索引：`recording moved to another binding: [{Key:Ctrl+Shift+B Action:say original} {Key:Ctrl+Shift+R Action:plugin:clipboard}]`；退出 1。
- M3 撤掉 cardRow 主体身份：`remaining command lost edit: [{Key:Ctrl+Shift+B Action:say original} {Key:Ctrl+Shift+C Action:plugin:clipboard}]`（深浅）；退出 1。

每条均恢复后执行 `md5sum -c /tmp/floter-P13-hc-product.md5`，settingsui.go / custom.go / parameditor.go / rows.go 四文件全部 OK，touch 后 `-count=1` 复跑通过。最终新增场景 **16 PASS / 0 FAIL**。原始日志 `/tmp/floter-P13-hc-M1.log`、`M2.log`、`M3.log`、`restored.log`。

还原指纹：settingsui.go `4a5f4b9f7b29d8829fdb48cc6f8b66e3`；custom.go `6b78f49cdcab8a801390f384f5b9fe94`；parameditor.go `01ba18080c504b6e6107c9102b568905`；rows.go `d85c12dea448953512bf7a665df7716e`。

## 偷了什么

实际读取 qiaomu-design 的 SKILL、user-preferences、craft-loop、preflight、chinese-typography，以及 Linear 供体。用户明确指定唯一方向，直接打磨；当前 Codex 独立实现。

- craft-loop **#3 品牌 focus 环**：把焦点归属落在 **1 个当前控件**，主表单两行与参数六格使用语义身份；字段保留现有 **1px** 主题焦点边，不另造色。
- **#4 签名交互**：录制器始终对应 **1 条绑定**；删除自身后剩余录制器 **0 个**，删除前项后原对象继续收键。高频按键即时响应，新增动画 **0ms**。
- **#6 编辑细节**：错误提示只跟随所属行，其他行错误数 **0**；保留 **1px 分隔线 / 44px 行地板**。首行是否有分隔线不再改变输入身份，几何常量修改数 **0**。
- **#9 滚动条**与 **#12 材质细节**保留现有实现，没有把保留项算成新修复；本轮只有前三项直接改动，不为凑五项工艺添加装饰。

Linear 的三档中性墨、代码 monospace、1px 细边映射到项目现有 TextStrong/TextSecondary/TextTertiary、等宽字体与 ControlEdge；没有换字体、引入紫色或 Web blur。

## 深浅图与两轮看图

ui.Tester 原图四场景 × 深浅 × 前后，共 16 张；Command 720×1500、参数板 720×700、快捷键两场景 720×440。对比图直接裁原图并上下排列，未生成替代画面。

| 场景 | 深色 | 浅色 |
|---|---|---|
| 类型切换后的命令输入 | [前后](P13-hc/command-mode-dark-compare.png) | [前后](P13-hc/command-mode-light-compare.png) |
| 参数语言切换 | [前后](P13-hc/parameter-locale-dark-compare.png) | [前后](P13-hc/parameter-locale-light-compare.png) |
| 快捷键冲突归属 | [前后](P13-hc/shortcut-conflict-dark-compare.png) | [前后](P13-hc/shortcut-conflict-light-compare.png) |
| 快捷键命令编辑 | [前后](P13-hc/shortcut-command-dark-compare.png) | [前后](P13-hc/shortcut-command-light-compare.png) |

第一轮四视角：艺术——焦点边在模式切换后消失，画面不再说明键盘在哪里；工程——整行加 key 后仍红，定位到首行分隔线；任务——录制一次键却改了别的绑定；读者——冲突句贴在无关行。第二轮修语义身份、主体身份、录制与提示迁移，并重新出图。

自行复核可见变化：深色命令框从 weather 且无焦点边变为 forecast 且带焦点边；浅色参数从 Target 变为 Destination；冲突红字从 Clipboard history 下方移到 Ctrl+Shift+B 原命令行；快捷键输入框出现 say edited。材质和布局尺度保持。

为什么本轮更轻、更有空间感：焦点环和错误句各守自己的位置，减少用户为找回输入而反复点击与重新扫描；不增加玻璃层、装饰和常驻提示。

## preflight A / B / C / D2

| 条目 | 结论 |
|---|---|
| A1 渐变/纯黑/霓虹 | 未新增；沿用近黑/近白与中性面 |
| A2 更换非系统字体 | 不采用，现有视觉合约与中文系统字体优先 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置表单 |
| A5 卡片模板 | 无新增分组卡 |
| A6 假名/数字 | 产品无新增假数据，测试夹具为具体键与命令 |
| A7 空词/感叹号 | 产品 i18n 文案零改动 |
| A8 编号眉批 | 无新增 |
| A9 H1 | 不适用，无营销标题 |
| A10 网格空洞 | 参数网格几何不变，真实中文截图可读 |
| A11 装饰侧线 | 无新增 |
| B1 读取/拨盘 | 2/2/6，与紧凑工具设置一致 |
| B2 主题 | 深浅独立渲染，无混色 |
| B3 记忆点 | 输入和反馈只属于当前对象 |
| B4 动效气质 | 即时编辑、沿用短控件反馈 |
| C1 目的/频率 | 高频输入不加动画 |
| C2 all/ease-in/scale(0) | 无新增 |
| C3 时长/按压 | 沿用既有 120ms 状态反馈与按压面 |
| C4 origin | 未新增浮层；既有下拉保留 |
| C5 reduced-motion | 测试使用 ReduceMotion，框架降级不变 |
| C6 hover 媒体条件 | 原生指针系统，不适用 Web 媒体查询 |
| C7 属性 | 无新增布局动画 |
| C8 快速触发 | 保留现有 transition，不重播动画 |
| C9 手势 | 未新增手势 |
| C10 Framer Motion | 不适用 |
| C11 真机/逐帧 | ui.Tester 验证输入序列和软件渲染，未宣称真机 GPU/桌面手测 |
| D2-1 字体栈 | 中文系统字体与代码等宽栈保持，无字体下载 |
| D2-2 行高/字号/字重 | 全局 Body 10.8px、Caption 9.9px 及既有字重不满足 skill ≥14px/400–600，沿用用户拍板项；本轮不改变高度链，不宣称全部通过 |
| D2-3 混排空格 | 产品文案不改；命令与快捷键按用户原文显示 |
| D2-4 标点/对比 | 既有中文全角文案保持；安静墨非全部 7:1，沿用已有限制 |

## 仍待

- 八项已完成，四批代码 a70e682f / 8f3dd208 / f32742e9 / e6fd4d46 已推送。
- 下一轮先查配置表单在 schema 增减、翻译变化后的输入与错误归属；属于待复现候选，不冒充已确认缺陷。
- 保存期间草稿归属、字号/行盒、终端 S1b/S1c/S2 尾沿用拍板项；本轮不改其契约。
- 八条早期独立 WIP 保留原件，stash 表已逐条记录。

## 最终门槛

中文 locale、GOMAXPROCS=4：普通 `go test -count=1 -timeout=20m ./...` 与 verbose 全量均退出 0；最终 **3624 PASS / 0 FAIL**（基线 **3608**，增加 **16**）。`^func Test` 同口径 **2719 → 2727**。没有新增 Skip、删除测试或改旧断言；V/B/S **0/0/0**。

`gofmt -l cmd internal` 输出 **0 字节**；Linux / Windows / macOS 的 `go vet ./...` 和 `go build ./...` 全部退出 0。新合约落盘后 internal/ci 再验通过。三条正式变异均红，四文件逐字节恢复；touch 后新增 16 个场景全绿。

完整日志：`/tmp/floter-P13-hc-baseline.log`、`final-test.log`、`final-verbose.log`、`vetbuild.log`、`doc-ci.log`。16 张原图、8 张前后比较和 2 张 4× 细节图随最终提交入库；4× 复核焦点边连续、字形完整，尺寸变化只来自错误句归回所属行。软件渲染与键盘测试已验证，未做操作系统桌面手测。
