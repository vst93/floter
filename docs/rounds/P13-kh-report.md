# P13-kh · 本地工具与参数声明的键盘焦点净空

开盘 16255c24，工作区干净、pull 最新。任务书安装失败恢复已经 P13-jx 完成，沿最新 kg「仍待」推进。指定 done.log 不存在，收尾创建。当前 Codex 独立执行。

## stash 清点

本轮逐条 reverse-check=1；SHA 与指定交接账一致。八条均为早期独立 WIP，没有新增 stale-kill。沿用语义审计，不能把旧主题或旧 Tauri 链覆盖新实现；保留原件，没有静默丢弃。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap/主题 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/滑块 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42 | 旧字段/确认栏 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 Tauri 发布链 | 保留 |

## 增量记录

先测八个候选，复现成立才修改。旧 V/B/S 断言暂为 0/0/0。

### 第一批：添加与删除参数

1. 添加：现象→Tab 到 28px 加号时下环落在 y=521，画布之外；改动→原按钮外套既有 19px 显现净空；验证→深浅上下环可见，Enter 添加第六个参数并回原标签，随后滚轮可离开。
2. 删除：现象→删除键下环同样被裁；改动→同一净空只在焦点进入时显现；验证→深浅光栅通过，Enter 只移除当前记录并回到下一条标签，参数数量 5→4。

相关 44 PASS/0 FAIL，定向 vet 净。M1 撤回两条接线，深浅分别实红 `add bottom ring missing at [689 521]: blue-red=0`、`remove bottom ring missing at [680 521]: blue-red=0`；MD5 还原、touch 后 6 PASS。旧 V/B/S 0/0/0。

首轮像素夹具发现类型/高级标题与控件同名，Find 首个命中是标题，不能当控件；类型改读取当前值中心对应的30px框，不放宽阈值。初次高级探针不作正式证据。模式与选择器的首次Tab路径正常，不计修复，继续检查已存在的返回路径。

### 第二批：必填与类型

3. 必填：现象→14px复选格的焦点环下沿落到 y=521；改动→原复选控件包裹19px净空；验证→深浅上/下环均出现，Space只改第一条Required，其他记录不变，随后滚轮可离开。
4. 类型：现象→30px下拉的下环在视口外；改动→只在类型控件进入焦点时显现原环；验证→读取真实选中值的中心光栅，深浅通过，Down/End/Enter选择原列表末项，原条件字段恢复测试继续通过。

相关56 PASS/0 FAIL、vet净；旧V/B/S仍0/0/0。真实before失败分别为 `required bottom ring missing at [214 521]: blue-red=0`、`kind bottom ring missing at [552 521]: blue-red=0`。第一批已推送5f6caf89。

中文全量开盘实测6389 PASS/0 FAIL/3495函数，exit0；C locale误跑结果不算基线。

### 第三批：输出方式与提交

5. 输出方式：现象→终端/后台分段选择的焦点下沿在y=521被裁；改动→原分段控件外显现19px净空；验证→深浅闭环可见，Right切换仍写原routeBackground，滚轮不被锁住。
6. 保存/连接：现象→Tab到末端提交键时只剩三边环；改动→忙态与可用态共用原位置的净空；验证→深浅上下环完整，Enter只派发一次原保存回调，校验拒绝/程序选择恢复旧测试继续绿。

相关133 PASS/0 FAIL，vet净；before两项均 `bottom ring missing ... blue-red=0`，旧V/B/S0/0/0。第二批已推送0ffedc63。

### 第四批：高级披露与平台选择

7. 高级：现象→30px胶囊下方焦点环被淡边裁掉；改动→原披露按钮包裹19px净空；验证→深浅上下环像素通过，Enter仍打开原权限声明区，不改权限值。
8. 平台：现象→macOS/Linux/Windows复选框的原生环在Tab进入底部时缺下沿；改动→三个现有复选控件逐个显现净空，保留原Space、写回及第一个平台的校验返回锚点；验证→真实Tab到macOS，深浅方框外环可见，Space只切换目标平台，原平台拒绝恢复回归通过。

高级夹具第二次错误来自Tester.Find返回被视口裁过的字幕高度，不能用裁过的高度反推按钮。改用同字体、未裁剪的上方说明行高度还原居中布局，断言仍为固定像素阈值，没有扩容差。平台原生环围着16px方框，按框架8px字框间隔定位，未误测文字周围。

第四批最终31 PASS/0 FAIL、vet净，b838c37c已推送；第三批fcc40113已推送。至此八项四批完成。

## 为什么更轻、偷了什么

焦点墨离开裁剪和16px淡边后，当前可操作面与背景分得更清楚；继续使用原来一层凹面和一圈焦点，不叠卡片或厚玻璃。

- 从craft-loop工艺菜单第3项取品牌focus环：保持原生2px宽、1px间隙，新增显现净空固定19px；8个入口族沿用深#8FB7FF/浅#064AA6主题蓝。
- 第4项取滚动显现：仅进入焦点时一次显现，新增动画0ms；8项均有随后滚轮能离开原位置的断言。
- 第12项取材质细节：复用原受光边和凹槽；新增组件透明度0层、阴影0层，不把修复变成重新装修。
- Linear供体的#0f1011、0.05–0.08发丝线、交互专用单一accent分别转译既有#0E0E10、ControlEdge深0.07/浅0.06与蓝色。用户现有方向和不新增系统优先，不凑5种装饰。

## 两轮图证与复核

同一ui.Tester事件、中文字体环境、720×520窗口、ReduceMotion=true，深浅各自独立渲染。before来自实际旧接线或撤回净空，after来自恢复后的代码。PNG在/tmp/floter-P13-kh-*.png，归档在本轮目录。

第一轮四视角：艺术总监看到蓝环与底部淡边混在一起；工程师发现同名标题/裁过的文字框不能代表真实按钮；任务所有者要Enter/Space仍执行原目标；读者看到最底部“必填”被淡边冲淡。第二轮显现原环、按真实控件定位、逐项测试事件出口，并复核深浅底沿局部。

| 项 | 图上可见变化 | 浅色 | 深色 |
| --- | --- | --- | --- |
| 添加 | 加号的下方蓝环重新闭合 | [图](P13-kh/add-light-comparison.png) | [图](P13-kh/add-dark-comparison.png) |
| 删除 | 叉号方格从贴底变为完整可见 | [图](P13-kh/remove-light-comparison.png) | [图](P13-kh/remove-dark-comparison.png) |
| 必填 | 方格与“必填”退出底部淡边 | [图](P13-kh/required-light-comparison.png) | [图](P13-kh/required-dark-comparison.png) |
| 类型 | 文本类型胶囊下环可见 | [图](P13-kh/kind-light-comparison.png) | [图](P13-kh/kind-dark-comparison.png) |
| 输出 | “在终端中”选项的下环完整 | [图](P13-kh/output-light-comparison.png) | [图](P13-kh/output-dark-comparison.png) |
| 提交 | 保存胶囊闭环与底面分开 | [图](P13-kh/save-light-comparison.png) | [图](P13-kh/save-dark-comparison.png) |
| 高级 | 右端高级胶囊不再只有三边环 | [图](P13-kh/advanced-light-comparison.png) | [图](P13-kh/advanced-dark-comparison.png) |
| 平台 | macOS方框环四角可见 | [图](P13-kh/platforms-light-comparison.png) | [图](P13-kh/platforms-dark-comparison.png) |

## Preflight逐项

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹 | 沿用原主题，无新增 |
| A2 字体 | 中文系统字体优先；用户既有设计优先于网页换字体建议 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生功能表单 |
| A5 三卡/万能卡片 | 无新增容器分组，仅无绘制净空包裹 |
| A6 假数据 | 产品无新增数据；天气工具/field参数是隔离测试夹具 |
| A7 文案 | i18n零改，未加营销词 |
| A8 眉批 | 无新增 |
| A9 H1 | 既有页头不改，无换行大标题 |
| A10 Bento | 不适用 |
| A11 装饰竖线 | 无新增 |
| B1 读取一致 | 稳定3/即时2/紧凑7；不改尺寸与任务流 |
| B2 主题 | 深浅分别渲染，不混色 |
| B3 记忆点 | 处于视口末端的键盘目标也有完整原生环 |
| B4 气质 | 只即时显现，无新增表演动效 |
| C1 目的/频率 | 键盘高频动作即时0ms |
| C2 transition/ease/scale | 无新增，原按钮反馈保留 |
| C3 时长/active | 新动画0ms；原按压反馈不改 |
| C4 弹层origin | 未改弹层 |
| C5 reduced-motion | 所有图证与事件在ReduceMotion=true通过 |
| C6 hover | 未改hover或指针判定 |
| C7 属性 | 无新增布局动画，净空静态正负抵消 |
| C8 快速重播 | 无新增keyframes |
| C9 手势 | 无新手势，八项后续滚轮可自由离开 |
| C10 Framer | 不适用Go自绘 |
| C11 慢放 | 无新时间曲线；真实事件后逐帧截图 |
| D2-1 字体 | 原中文系统字体，无新webfont |
| D2-2 字号/行高/字重 | 通用≥14px、行高≥1.5与本仓10.8/9.9及580等冲突；沿用已列需拍板，按用户红线不改 |
| D2-3 混排 | 产品文案零改，原i18n继续使用 |
| D2-4 标点/对比 | 标点不改；默认全部次级文字≥7:1未承诺，不冒充通过。原高对比主题断言随全量执行 |

未将仓库特有的自绘边界写成全局skill偏好；合约§6.261与事件/像素pin是本轮可追溯规则。没有HTML预览、K3、子代理、新依赖或终端语义改动。

## 仍待

1. 下轮先测已有参数文本/选项编辑区的键盘焦点在上下视口边缘的完整度；只在有真实像素失败时修复，同族一次清。不重复本轮八个入口。
2. 模式初次Tab正常；探索的返回夹具未准确取得原目标，不把它记为已确认产品bug。原P13-kc模式返回B测试继续作为依据。
3. 原字号/行盒、accent禁用、未知多选、保存期间草稿、输出首尾键/选区与终端S1b/S1c/S2尾仍需拍板；未越权。
4. Linux自绘ui.Tester图证；Windows/macOS仅编译，未做原生交互或真实窗口截图。

## Mutation实测

- M1：撤回parameditor.go添加/删除两条净空接线。两项×深浅实红，`add bottom ring missing at [689 521]: blue-red=0`、`remove bottom ring missing at [680 521]: blue-red=0`；原文件MD5还原OK、touch后6 PASS。
- M2首次：19→0，参数添加、删除、必填、类型四项×深浅实红；输出/保存等受外层布局影响未全部红。脚本按预期拦住，不把未全红的这次当作全部入口变异通过；trap已恢复产品文件。
- M2正式：Go overlay实际替换settingsui.go为第三批前代码，撤回输出、提交、高级、平台四组接线。分别实红 `output bottom ring missing at [585 521]: blue-red=0`、`save bottom ring missing at [445 521]: blue-red=0`、`advanced bottom ring missing at [667 521]: blue-red=0`、`platforms bottom ring missing at [498 521]: blue-red=0`，深浅共8失败子场景。overlay文件MD5还原OK、touch后12 PASS、exit0；实际工作树未被overlay修改。

全轮八项都有实测红/绿，没有新增Skip、删除测试或放宽阈值。旧V/B/S断言修改0/0/0。

## 高度链独立核对

用Go overlay在同一夹具分别运行开盘16255c24与最终产品：深/浅两套、静息位置0/600/MaxY，共6场景。逐场比较原始PNG和所有可发现文字的矩形、Body.Y/MaxY；12次cmp全部逐字节相同。说明新增包裹只改变进入焦点时的显现，不改变静息像素与滚动范围。探针只通过overlay加载，不混入正式测试函数统计。

## 完整门槛（增量）

gofmt空；Linux/Windows/Darwin vet与build六项exit0；文档CI exit0；普通全量go test -count=1 ./... exit0。抢救稿第三父树恢复的完整verbose日志为6413 PASS / 0 FAIL，full-verbose.exit与tests.exit均0。函数3495→3503（+8），旧测试删除0、新Skip0。

## 2026-11-08 收尾抢救

a45eb7b83667171164a4f9f22932efdd4b4adc79含独有的收尾文档与未跟踪图证/门槛日志；git apply -3恢复四份文档，第三父树恢复证据。补录最终实测值，文档CI再次exit0后单独提交。原对象归档至refs/archive/floter/P13-kh/完整SHA再从活动stash移除。产品八项已在此前四批提交，不计为下一轮新修复。
