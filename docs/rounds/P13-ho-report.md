# P13-ho 报告：配置草稿与字段生命周期

开盘 ba5e3220，pull 最新，树干净。中文 locale 实测基线 3868 PASS / 0 FAIL / 2824 测试函数。
done.log 在本机不存在，收尾建立。qiaomu-design 的 SKILL、偏好、craft-loop、preflight、
chinese-typography 与 Apple 供体已读。未调用外部模型或子代理。

## 抢救 stash 清点

八条 reverse-check 均退出 1，与 hn 的 SHA 相同；无 stale-kill 新稿。
沿用 hn 已有语义审核：独立早期 WIP 不直接覆盖现实现，不把检查失败说成独有可合入。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | P7 keycap 未完稿，非抢救 | 保留原件 |
| stash@{1} | 8af94a07 | 旧行距/滑块稿，非抢救 | 保留原件 |
| stash@{2} | 7e4aa710 | 旧卸载度量/探针，非抢救 | 保留原件 |
| stash@{3} | 1e3aec94 | 旧数字字段稿，非抢救 | 保留原件 |
| stash@{4} | 363f5a42 | 旧文本槽稿，非抢救 | 保留原件 |
| stash@{5} | 9b8ce23c | 旧快捷键形状稿，非抢救 | 保留原件 |
| stash@{6} | 90d314b2 | 旧动作栏行为稿，非抢救 | 保留原件 |
| stash@{7} | 7a9f6e13 | 旧 Tauri 发布稿，非抢救 | 保留原件 |

## 侦察纠正

“schema 放宽后数字保存值可能仍为 nil”未成立：数字 Changed 保存原始有限数，
只解析失败才写 nil。该线索撤回，不计修复。后续只凭真实事件红绿日志记账。

## 第一批（1–2）

1. 现象：文本改走再改回原路径，表单已显示 Saved，外部新路径仍被旧 live 遮住。
   改动：只在新宿主代际到来时，退役与 saved 同值且合法的编辑。验证：深浅真实输入、复制读到 external，焦点保留；真正本地编辑不覆盖。
2. 现象：数字改回 5.0 已与保存值 5 相同，外部 40 仍显示 5.0。
   改动：按有效数值识别撤回，合法同值才释放。验证：深浅复制得到 40，继续输入 12 遇外部刷新仍保留。

M1 回退 config.go 实跑：`reverted text masks host update: "/workspace/initial", want "/workspace/external"`；
`reverted number masks host update: "5.0", want "40"`。恢复 md5sum OK，touch 后相关回归 50 PASS / 0 FAIL，settingsui vet 通过。
V/B/S 0/0/0，旧测试零修改；新增 2 函数，截图 before/after 深浅均生成。

## 第二批（3–4）

3. 现象：数字输入 1e，schema 移除字段又重新加入，旧半输入和解析错误一起复活。
   改动：移除字段时退役 live 与其错误，保留已有 values 保存语义。验证：重加后显示空值，错误消失，可输入 8。
4. 现象：外部写入已与本地路径相同，状态仍显示 Unsaved changes，下一代路径仍被遮住。
   改动：仅当新宿主代际与合法编辑同值时确认 saved、释放 live。验证：Saved 恢复，下一代 latest 可见。

M2 原文：`removed number resurrected partial text: "1e"`、`removed number resurrected old parse error`、
`host-confirmed edit still says Unsaved changes`、`host-confirmed edit masks next generation: "/workspace/shared"`。
两项深浅实跑红，md5sum OK、touch 后相关 56 PASS / 0 FAIL，vet 通过。V/B/S 0/0/0。

## 第三批（5–6）

5. 现象：移除已滚动的多选字段再加回，新列表仍从旧偏移 206 开始，第一项藏在上方。
   改动：字段移除或变为非多选时退役其 ScrollState。验证：同一字段重加 Y=0，存储值不变。
6. 现象：已保存路径两端加空格后 Enter，又写一次同值，拒绝写入就显示 Save failed。
   改动：提交比较沿用宿主 TrimSpace 规范化，无变化时退役草稿。验证：0 次写入、框回到规范值；不同路径仍提交一次，拒绝后原草稿可重试。

M3 原文：`readded list starts at old offset 206`、`normalized same text wrote 1 times and failed=true`。
实跑回退两文件，深浅均红；MD5 还原 OK，touch 后相关回归与 vet 通过，PASS 数随提交记录。
V/B/S 0/0/0，已有断言未改。

## 第四批（7–8）

7. 现象：已接受 25，上限收紧到 10 后再放回 100，框仍画 10，宿主却一直存 25。
   改动：确认值仅为显示而收界时不继续占用编辑草稿。验证：10 → 25，整个 schema 刷新 0 次写入；1e 半输入不丢。
8. 现象：有限数字 1e308 在数字槽画成 309 个字符，只能看到零。
   改动：绝对值 ≥1e21 的数字框沿用滑块的科学计数法，1e+308 共 6 字符。
   验证：真实框复制读到 1e+308，宿主值仍为 1e308；深浅截图核对。

M4 原文：`relaxed bounds keep obsolete clamped draft: "10"`、
`extreme number spills 309 characters into narrow slot`。回退 pluginsheet.go 实跑深浅全红，
md5sum 还原 OK，touch 后相关回归 112 PASS / 0 FAIL，vet 通过。
第三批相关回归为 106 PASS；第四批 V/B/S 0/0/0，旧断言未改。

## 当前验收状态

八项均完成，四批每批两项；32 张原始 before/after PNG 与四张对照板已入轮次目录。
普通、verbose 全量与三平台 vet/build 已完成：3893 PASS / 0 FAIL，详见最终门槛。

最终代码的普通全量已通过（settingsui 实跑 294.599s），三平台 vet/build 再跑均退出 0；
gofmt 与三平台 vet 输出均 0 字节。verbose 最终计数 3893 PASS / 0 FAIL，退出 0。
中途那份失效旧版本 verbose 已主动终止（exit 143，部分计数 1603 不作验收），保留终止事实。
本轮新增测试文件 251 行，旧测试文件没有删除/改动，函数 2824 → 2832；无新增 Skip。

## 偷了什么

读取为现有配置表单，面向配置工具的用户，沿用安静紧凑的轻便液态玻璃；冒险 2 / 动效 2 / 密度 7。
用户要立刻辨认当前值、是否修改、是否能保存。为什么更轻/更有空间感：无效错误句退场，读数不再被窄槽截断，已有层次不被额外状态噪声遮住。

- 从 `craft-loop.md` 工艺密度菜单第 6 项“编辑细节”取内容收口：309 字符 → 6 字符，阈值 ≥1e21；移除字段带回的旧校验句 1 → 0；无变化文本引发的失败句 1 → 0。
- 第 3 项“focus 环”：保留 1px 语义焦点边与原字段身份，文本/数字外部刷新均断言焦点仍在。属于保留验收，不计新修复。
- 第 12 项“材质细节”：保留 8u 槽圆角与 1px 边，新增透明层 0；这是保留，不称为新设计。
- Apple 供体的单一交互色、0.48 弱化、400/600 字重角色映射已有 accent、框架 0.5 与字重 token；没有复制网页 blur 或扩大字号。
- 不凑五种装饰：当前用户明确要求修已有功能，工艺密度让位于真实状态和阅读空间。独立设计记忆没有改写；缺陷规范与 pin 留在仓库合约 §6.190。

## 截图与两次复核

ui.Tester 原图 720×320（集成）/720×220（插件），32 张深浅 before/after，
4 张对照板和 2 张数字局部放大，共 38 PNG。路径 `P13-ho/`；原件同名在 `/tmp/floter-P13-ho-*.png`。

第一次复核查看浅色第一板与深色第二板，第二次逐图看另一套外观及 4× 数字局部：
浅色可直接看到 5.0 → 40、旧红色解析提示消失、红色 Unsaved changes → 灰色 Saved；
深色列表从来源 09/10/11 回到 00/01/02；路径无变化时不再多出“无法保存该修改”；
数字槽从一截 00000000 变成完整 1e+308，末位 8 未被槽边切掉。

四视角复核：艺术总监关注无意义错误带抢焦点（第 3/6 项已去掉）；工程师关注旧状态遮新值（1/2/4/7）；
任务所有者关注显示 Saved 的含义（第 4 项匹配真实宿主值）；读者关注极大值完整性（第 8 项）及原有小字号。
本轮是缺陷前帧→修复后帧的打磨；截图复核未再发现需要新增样式的缺陷，没有另改装饰冒充第二轮成果。
小字号仍属原需拍板项，不宣称通过全部网页审美门槛；没有声称做过 Windows/macOS 真机 GUI 验收。

第二次代码审阅补上第 4 项组合：旧输入 12 超过上限 10；宿主确认 12 的同时上限变 20，
live 退役后旧错误仍留。实跑新子场景红：`host-acknowledged valid value retains obsolete range error`。
确认/撤回的合法编辑退役时一并清理自己的校验句。M5 回退实跑红、MD5 恢复 OK，touch 后本轮 25 PASS；
相关回归 113 PASS / 0 FAIL、vet 通过。这是第 4 项修正，不重复算第九项。
初次完整门槛运行中发现此问题，修正后重跑完整门槛，不用旧构建结果冒充最终结果。

## Preflight 逐项结论

A：A1 无新增渐变、纯黑或 glow；A2 沿用系统字体（中文规则与原生应用契约优先，不按网页规则换字体）；
A3 无斜体；A4 无 Hero；A5 无三栏卡片与新嵌套容器；A6 使用明确的配置夹具，没有假用户/营销数据；
A7 未新增营销文案与感叹号；A8 无产品编号眉批（对照板标记只在证据）；A9 无 H1；
A10 非 Bento；A11 无装饰左竖线。A2 工艺密度不足五种新动作已说明用户范围冲突；不伪报全 PASS。

B：B1 与冒险 2/动效 2/密度 7 相符；B2 每帧单一深/浅主题；B3 记忆点是紧凑字段中当前值清楚、保存状态可信；B4 即时输入反馈与安静材质一致。

C：C1 高频输入即时反馈，未加动画；C2 无新增 all/ease-in/scale(0)；C3 沿用原 120ms 色阶与按压；
C4 未新建弹层；C5 测试启用 ReduceMotion；C6 原生鼠标事件，不适用 CSS hover media；
C7 无新增几何动画；C8 未新增 keyframes；C9 未改手势；C10 无 Framer Motion；C11 本轮无新动效，未做慢放设备验收。

D2：D2-1 系统中文字体，无外部字体；D2-2 **未通过**，原有 Body 10.8px/Caption 9.9px、部分字重 620/650 与 ≥14px/400–600 不符，沿用需拍板；
D2-3 新数字为 ASCII，单位原有空格保留，未新增中文句子；D2-4 标点无新问题，暗色 token 沿用，未证明全页正文 7:1，保留限制。

## 仍待

1. 下轮优先复现同一配置表单的“已有本地编辑、外部更新、再恢复默认/回到外部值”组合；本轮未改冲突编辑归属，不把猜测算 bug。
2. 插件字段跨分组移动的焦点与草稿需真实事件验证，尚未确认为缺陷；原地增加/删除组标题已有旧测试覆盖。
3. 字号/行盒、accent 禁用填充、保存期间草稿归属、未知 Radio、多选宿主过滤、小于 1e-6 存储精度、终端 S1b/S1c/S2 尾继续沿用需拍板。
4. 八条早期非 stale-kill WIP 保留，已清点；无新增待抢救内容。

## 最终门槛与提交

- 基线 3868 PASS / 0 FAIL / 2824 函数；最终 **3893 PASS / 0 FAIL / 2832 函数**，增加 25 PASS 与 8 函数。
- 普通与 verbose 均 `-count=1` 实跑退出 0，统一 `LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4`。
- gofmt 输出 0 字节；Linux/Windows/Darwin vet 与 build 全部退出 0，vet 输出均 0 字节。
- 五组 mutation 覆盖八项及第 4 项修正，全部实跑红，逐字节 MD5 恢复、touch 后复跑。旧测试与 V/B/S 均零改动，无新增 Skip。
- 日志 `P13-ho/full-test.txt`、`targeted-test.txt`、`gates.txt` 与 `mutation1–5.{txt,md5}`；完整 verbose 在 `/tmp/floter-P13-ho/verbose-final.txt`。
- 四批八项 `8b6d4b0e`、`c714405b`、`6777c0cb`、`f5af4c5b`，补验修正 `0969f822`，均已推送 `origin/mygo-rewrite`；最终文档随收尾提交。

过程偏差：本轮工具编排调用超过 40 次；超预算后仅继续必要验收与归档，没有新开选题。
重复的旧版本 verbose 因补验修正而作废并终止；最终数字只来自最后一版完整成功运行。
