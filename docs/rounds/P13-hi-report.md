# P13-hi · 配置 schema 刷新后的编辑状态

开盘 2ed2e7a8，工作树干净；备用 SSH 443 已同步。done.log 开盘不存在。规格先落盘，验收随每批追加。

## stash 清点

八条实际 reverse-check 均退出 1；SHA 与 P13-hh 结案表完全一致，无新 stale-kill。早期 WIP 原件继续保留，不把 reverse 失败当作必须覆盖现实现的理由。

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 422483b4 | 旧 keycap 与主题 | 保留原件 |
| stash@{1} / 8af94a07 | 旧行距与滑块 | 保留原件 |
| stash@{2} / 7e4aa710 | 旧卸载布局与探针 | 保留原件 |
| stash@{3} / 1e3aec94 | 旧数字框与透镜 | 保留原件 |
| stash@{4} / 363f5a42 | 旧确认带与快捷键 | 保留原件 |
| stash@{5} / 9b8ce23c | 旧快捷键反馈 | 保留原件 |
| stash@{6} / 90d314b2 | action bar 旧键盘合约 | 保留原件 |
| stash@{7} / 7a9f6e13 | 旧 Tauri 发布链 | 保留原件 |

## 增量验收

1. 数字改文字后仍显示半输入与数字错误 → 改型清掉旧 live 和错误 → 深浅数字输入 `1e` 后改文字，旧提示消失，重新输入“搜索”成功。原串：`text field inherited the number half-input or error`。
2. 文字改数字后撤销写回旧文字历史的 12 → 编辑器身份包含类型 → 深浅 Ctrl/Cmd+Z 不改已有值，重新输入 56 得到数字。原串：`number field inherited text undo and wrote an unrequested value: 12`。

第一批相关测试 **32 PASS / 0 FAIL**（包含新增两项深浅测试），V/B/S 0/0/0，旧断言零改。初稿 36 为抄录笔误，以 batch1-after 原始输出的 32 为准。全量基线仍在运行，未借用旧报告数字。

3. 数字上限放宽后红色错误行不消失 → 每帧对已有编辑按新边界校验 → 放宽 10→20 消错、收紧至 8 报错；12 与焦点均保留，输入 6 恢复。原串：`relaxed number bound left a stale error line`。
4. 文字长度放宽后仍显示超长 → 已有文本按当前长度约束校验 → 2→6 消错、6→3 报错；“工作索引”与焦点保留，输入“索引”恢复。原串：`relaxed text length left a stale error line`。

目视首批深浅图：旧文字字段仍带 `1e` 与数字错误红边；改型后回到中性槽面、下方错误行消失，动作带上移到正确间距。未改字号、行盒与材质。

第二批相关 **28 PASS / 0 FAIL**，提交 ebfcf611 已推送。开盘完整基线实测 **3764 PASS / 0 FAIL / 2773 函数**。

5. 插件文字字段暂时改成开关再恢复，旧未提交草稿仍被保留 → 离开 Text 类型即清草稿，类型进入控件身份 → 深浅变型周期退役 `/workspace/unsent`，恢复后可输入并提交 `/workspace/recovered`。原串：`retired text kind kept its unsent draft for the next text field`。
6. 插件数字字段改成开关再恢复仍保留 `1e` → 离开 Number 类型即清半输入，恢复读取宿主值 8 → 深浅恢复后可输入并提交 9。原串：`retired number kind kept its half-input for the next number field`。

第三批执行偏差：09ccca03 被提前推送时相关命令因 `conversion from Kind (int) to string yields a string of one rune, not a string of digits` 失败；不得把该提交记为过门槛。已改成 `strconv.Itoa(int(field.Kind))`，重新验证后单独补推修正，完整门槛仍在最终实现上执行。

修正后第三批相关 **38 PASS / 0 FAIL**，47eb4669 补推通过。

7. 多选字段整体移除再恢复，Shift 把中间“应用”也选中 → 在表单早退前退役不再存在的多选锚点 → 恢复后 Shift 只新增“书签”，随后新的范围选择可用。原串：`removed field resurrected a stale range: [文件 应用 书签]`。
8. 聚焦插件文字字段被移除，没有 blur 清理，恢复时画回旧路径 → 绘制 schema 前清理消失字段的编辑草稿，数字同路径一起覆盖 → 深浅恢复显示 `/workspace/restored`，重新输入并提交成功。原串：`removed plugin field kept a draft and repainted stale text on return`。

第四批相关 **111 PASS / 0 FAIL**，dbf06c65 已推送。八项跨集成配置和插件配置两个组件；产品四批另加一次转换修正。旧 V/B/S **0/0/0**，无删除旧测试、无放宽断言、无新增 Skip。

## 正式变异

三个注入点分别禁用：M1 改型清 live；M2 按新约束重校验；M3 schema 移除时清插件文本草稿。使用各自 pin 单独运行，每条退出 1。原始错误分别为：

- M1：`text field inherited the number half-input or error`。
- M2：`relaxed number bound left a stale error line`、`relaxed text length left a stale error line`。
- M3：`removed plugin field kept a draft and repainted stale text on return`。

两个源文件备份恢复，`md5sum -c` 全 OK；touch 后无缓存复跑 **24 PASS / 0 FAIL**。原始输出、源 MD5 与恢复检查在 `P13-hi-evidence/`。

## 图审与偷了什么

32 张原生 ui.Tester PNG（八项 × 深浅 × 前后），另有两张比较板。每一组逐像素差异非空，difference bbox 原件在 `image-differences.txt`。不是为了截图改静态样机。

第一轮四视角：艺术总监看到旧数字错误红线留在文字框上，破坏安静槽面（1）；工程师发现撤销写入跨类型（2）；任务使用者发现放宽约束仍被错误提示阻拦（3/4）；读者看到恢复字段显示旧路径或 `1e`（5/6/8）。第二轮相应修复后重新截图，保留内容与既有层级。

目视具体变化：浅色插件路径从 `/workspace/unsent` 换成当前 `/workspace/current`；深色数字恢复值从 `1e` 换成 `8`；多选中间“应用”的误勾消失；两类放宽约束后下方错误行和红边消失，动作带回到单行字段的正常下方。

从 Apple DESIGN 提取三条 DNA：交互色单一（#0071e3 → 现有 Accent）、近白/近黑中性基底（#f5f5f7/#1d1d1f → 既有 token）、胶囊形（980px → 既有 999px）；本轮遵循，不重定主题。`craft-loop.md` 菜单的应用逐项为：

| 菜单 | 本轮落点与数值 | 新改动或沿用 |
|---|---|---|
| #3 品牌 focus 环 | 原 2px 焦点标识只属于当前类型；约束刷新保住焦点 | 身份修复，环宽沿用 |
| #4 签名交互 | schema/键盘反馈 0ms，恢复后第一帧显示当前值 | 实际修复 |
| #6 编辑细节 | 无效错误提示由 1 行变 0 行，有效错误仍 1 行 | 实际修复 |
| #9 自定义滚动条 | 66px 多选视口保留，退役范围不重置保存值 | 几何沿用 |
| #12 材质细节 | 1px 受光边与三档面保持，过期错误红边回中性面 | 材质沿用 |

不把沿用的工艺项冒充五项新视觉改版；以任务要求的真实可见修复优先。

## preflight 逐条结论

| 检查 | 结论 |
|---|---|
| A1 色彩/发光 | 无新增渐变、纯黑或外发光；沿用主题 |
| A2 字体 | 按中文条款与项目合约沿用系统字体，例外于通用“非系统字体”建议 |
| A3 斜体 | 未增加斜体 |
| A4 Hero | 桌面表单，不适用 |
| A5 卡片 | 未增卡片或嵌套层 |
| A6 假数据 | 测试用本地搜索路径、真实边界数；未写产品假数据 |
| A7 文案 | 产品词条零改，无新增空词或感叹号 |
| A8 眉批 | 无新增编号眉批 |
| A9 H1 | 未增标题或改行高 |
| A10 网格 | 无新增 Bento/网格 |
| A11 竖线 | 无新增选中装饰色条 |
| B1 方向 | 稳定 2 / 动效 2 / 密度 6，轻便玻璃设置表单 |
| B2 主题 | 深浅各自独立，32 原生帧，无混色 |
| B3 记忆点 | 同一个安静槽面只显示当前值与当前错误 |
| B4 一致性 | 即时恢复，未加动效层 |
| C1 目的/频率 | 高频键盘与刷新均 0ms |
| C2 禁止动画 | 未加 transition/all、ease-in、scale(0) 或 fade |
| C3 时长/按压 | 无新动画；复用既有按压反馈 |
| C4 浮层起点 | 未改弹出菜单或位置 |
| C5 减弱动效 | 两套夹具均 ReduceMotion=true |
| C6 hover | 原生指针控件，无新增 hover，Web media 不适用 |
| C7 属性 | 无尺寸或位置动画 |
| C8 快速触发 | 复用现有控件状态机，无重播 keyframes |
| C9 手势 | 本轮无手势或拖拽变更 |
| C10 Motion | 无 Web/Framer Motion，不适用 |
| C11 抽查 | 逐帧真实输入与截图；没有新增时间曲线需慢放 |
| D2-1 字体 | 中文系统字体，零外部字体资源 |
| D2-2 字号/行高/字重 | **未全通过，沿用需拍板**：正文 10.8px、Caption 9.9px 小于 14px；既有 580/620 等字重不改，避免越权改高度链 |
| D2-3 混排 | 产品文案零改，未增加混排问题 |
| D2-4 标点/对比 | 无新句子、标点或色值；未新增全面 7:1 对比度审计，不冒称通过 |

## 门槛与仍待

Linux/Windows/macOS 的 vet 与 build 均通过，gofmt 输出为空，更新 §6.185 与待办后的文档契约检查通过。完整普通/verbose 门槛运行中，最终数字待实际输出补录。tracked 测试函数 **2781**（基线 2773，+8）。

字号/行盒、161-4/176-9 与终端持久化 S1b/S1c/S2 尾沿用需拍板。本轮未改这些语义。下一轮首选现有插件配置提交失败/宿主规范化组合：Enter 后未失焦时显示值与确认值的关系，以及数字边界动态变化后的显示/clamp；目前只是代码线索，不计已复现 bug。8 条早期 WIP 保留，无新 stale-kill。

执行边界：本轮是原生 ui.Tester 渲染与真实模拟输入，没有声称三平台真机人工操作通过；三平台证据是 vet/build。当前字号对 skill 的 D2 通用门槛仍有公开例外。未创建新的系统、页面、依赖或重构架构。

完整普通测试发现一条旧 B 断言红：`TestPluginNumberDraftDoesNotBleedAcrossPlugins`，原串 `editing B disturbed A's draft: map[plug.b\x00size:9]`。schema 清理原本误覆盖其他插件的数字草稿。已将清理限定为当前插件前缀；旧测试一个字不改，保留跨插件草稿的行为契约。本项是第 8 项的回归补修，不另计第九项。首次失败完整日志保留为 `tests-first-failure.txt`，补修后重新跑门槛。
