# P13-jx · 安装表单失败恢复

2026-11-08开盘（首个明确时钟记录03:36:09 UTC）。HEAD 762606e1，pull已最新，工作区干净；done.log在给定路径不存在。按上轮“仍待”继续registry表单与本地选择出口。全量基线实测5998 PASS / 0 FAIL、3403函数、exit0。

## stash清点

以下均本轮实际reverse-check=1，SHA与P13-jw相同。它们是早期独立WIP，不是待接续stale-kill；沿用交接语义审计，不能用旧主题覆盖现有设计。没有新增独有抢救稿、没有drop。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap/主题 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/滑块 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42 | 旧字段/确认栏 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏 | 保留 |
| stash@{7} | 7a9f6e13 | 旧Tauri发布链 | 保留 |

## 增量实施记录

候选与mutation计划见spec。每两项完成即记录、提交、推送；旧测试断言不动。

### 第一批：失败与取消恢复

1. **下载/解析失败**：现象→HTTP 503后再次点击派发空包名/空版本；改动→失败回到UI线程恢复本次输入，编辑revision保护后续草稿；验证→真实HTTP + 点击、Tab输入版本、完成后再次点击，断言恢复`review-current / ^1.0.0`，深浅两套。
2. **权限取消**：现象→拒绝审核后输入被丢弃；改动→取消清理下载目录后恢复本次输入，仍不发失败通知；验证→本地registry包与权限回调、取消后重新点击，输入完整、通知数0，深浅两套。

M1实跑：撤回两条宿主恢复接线，两个场景深浅均红，原始串`download lost retry draft: "" / ""`、`cancel lost retry draft: "" / ""`。MD5还原OK，touch复跑6 PASS，旧提交即清空B测试1 PASS，定向vet无输出。第一次夹具误把无Label的版本占位符当Click目标，未触发业务；已改为真实Tab进入版本，初次输出保留但不计mutation证据。旧V/B/S修改0/0/0。

### 第二批：registry等待与空态

3. **等待期间仍可重复提交**：现象→本地/推荐已有锁而registry仍亮着可按；改动→同一安装入口复用packageInstallBusy，显示原忙态环、禁用重复点击，宿主同步拒绝重复派发；验证→等待期间真实点击派发0次、完成后重试1次，原取消宿主回归仍通过。
4. **空包名仍是Tab停靠点**：现象→空白包名的安装按钮看起来可用、能拿焦点，却无动作；改动→空白包名走disabledActionButton，输入有效包名即解锁；验证→Tab跳过、点击派发0次、填入后派发1次，深浅均验。

M2将表单还原到上一批产品，实红`busy registry permits repeat: 1`与`blank install remains a keyboard stop`；MD5还原OK、touch后6 PASS。宿主恢复与旧取消回归9 PASS，相关vet净。第一批已推送e5c74f50；V/B/S旧断言修改仍0/0/0。

### 第三批：提交失败与registry键盘

5. **审核后提交失败**：现象→准备成功、审核批准，但manifest在落盘前消失，结果报错且两字段仍丢失；改动→提交阶段所有错误也走本次草稿恢复；验证→在真实权限回调移除下载目录manifest，看到`extensions: no manifest`，完成后重新点击派发原名/版本，只有一个失败通知，深浅均验。
6. **registry等待丢键盘**：现象→Enter提交后焦点留在禁用按钮，继续输入无效；改动→只在原安装按钮持有焦点时，把键盘交回已有包名字段；验证→Enter提交、等待中直接输入`next-package`成功，完成不抢回焦点，焦点已在关闭按钮时也不抢。深浅均验。

M3a撤回提交失败恢复，深浅均报`commit lost retry draft: "" / ""`，还原后3 PASS；M3b撤回焦点接线，深浅均报`waiting registry lost keyboard focus`，还原后3 PASS。两次都MD5核对、touch复跑。新草稿保护另有name/version/输入后清空三条真实编辑事件，共4 PASS；定向vet净。键盘夹具起初漏接Close回调，补上后负向不抢焦点也通过，旧B测试未动。

### 第四批：本地入口退出与错误反馈

7. **本地审核取消后误重试**：现象→Enter提交、等待并取消审核后，再按Enter会再次安装（实测close=0、installs=2）；改动→原按钮持有键盘时，等待态把焦点交给现有关闭按钮，完成不抢回；验证→实际Tab/Enter、等待/取消/Enter链，close=1、installs=1，深浅两套。单纯文件选择器空结果取消原本已能保留入口焦点，不把该正常路径算bug。
8. **选择器错误静默**：现象→系统选择器失败没有任何结果，用户无法区分取消和失败；改动→复用既有LocalToolFailed宿主warning，不新增文案或浮层；验证→失败一次warning、取消不新增、重新选择成功派发一次，真实shell接线在深浅帧均看见提示。

M4撤回本地两条接线，实红`folder waiting lost keyboard focus`、`cancelled folder exit: close=0 installs=2`、`picker failure warning=[]`；MD5还原后touch复跑6 PASS。宿主警告3 PASS，相关vet净。八项均完成，四批产品提交；后续只补完整门槛、图像复核和交接。

## 为什么更轻、偷了什么

安装不可执行时只熄灭原胶囊的受光边，出错才出现既有结果浮层；用户仍能在原字段继续编辑，不添加卡片、常驻提示或第二层玻璃。等待、可编辑内容与结果各有明确层级。

- 从`craft-loop.md`工艺密度菜单第3项（品牌focus环）取“焦点只标出可用出口”：复用现有accent深`#8FB7FF`/浅`#064AA6`，registry交回1个包名框、本地交回1个关闭按钮，禁用安装按钮的焦点停靠数降到0。
- 第4项（签名交互）：只复用1种忙态环，沿用`actionSpinnerTurn=360`、`actionSpinnerPerTurn=900ms`；ReduceMotion帧不转动，不再添加动画。此900ms是进度环周期，非UI进入/退出时长。
- 第12项（材质细节）：registry等待/空态进入原DimFace，额外受光边数从1降到0；只保留框架1次0.5禁用透明度，组件新增阴影数0。
- 供体`design-systems/linear.app/DESIGN.md`的近黑面`#0f1011`、0.05–0.08发丝边、只用于交互的单一accent，分别转译为本仓既有`#0E0E10`、ControlEdge深0.07/浅0.06、原蓝色焦点墨。没有复制其紫色或换成Inter。恢复已有工艺不凑5个新装饰，用户“不新增系统、已有方向打磨”优先。

## 两轮看图与逐项图证

第一轮真实旧接线/缺陷注入，第二轮同一事件、相同字体环境、深浅各自固定后复渲染。32原始PNG、16对照、16差异局部与6张4×局部、2总览全部归档。

四视角记录：艺术总监看到空按钮仍有焦点亮环，落地禁用面；工程师看到异步结果丢草稿，补三条失败出口；任务所有者看到取消后的Enter会误重试，落地既有关闭出口；读者看到选择器失败完全无反馈，接回已有宿主warning。第二轮再次检查这四处，没扩张成重设计。

| 项 | 第二轮肉眼复核变化 | 浅色 | 深色 |
| --- | --- | --- | --- |
| 1 下载失败 | 两个占位符恢复为review-current与^1.0.0 | [对照](P13-jx/registry-download-light-comparison.png) | [对照](P13-jx/registry-download-dark-comparison.png) |
| 2 审核取消 | 输入内容恢复，右上没有误报toast | [对照](P13-jx/registry-cancel-light-comparison.png) | [对照](P13-jx/registry-cancel-dark-comparison.png) |
| 3 等待锁 | 胶囊前出现忙态环，字与受光边退一档 | [对照](P13-jx/registry-busy-light-comparison.png) | [对照](P13-jx/registry-busy-dark-comparison.png) |
| 4 空包名 | 安装环消失，Tab焦点落到后续有效控件 | [对照](P13-jx/registry-blank-light-comparison.png) | [对照](P13-jx/registry-blank-dark-comparison.png) |
| 5 提交失败 | 保留原名/版本且安装重新可用 | [对照](P13-jx/registry-commit-light-comparison.png) | [对照](P13-jx/registry-commit-dark-comparison.png) |
| 6 registry键盘 | 焦点从禁用胶囊移回包名框，插入光标可见 | [对照](P13-jx/registry-keyboard-light-comparison.png) | [对照](P13-jx/registry-keyboard-dark-comparison.png) |
| 7 本地键盘 | 取消后焦点在右上关闭，选择按钮不再持环 | [对照](P13-jx/folder-keyboard-light-comparison.png) | [对照](P13-jx/folder-keyboard-dark-comparison.png) |
| 8 选择器错误 | 右上出现既有“无法连接此工具”warning | [对照](P13-jx/folder-error-light-comparison.png) | [对照](P13-jx/folder-error-dark-comparison.png) |

[浅色总览](P13-jx/overview-light.png)、[深色总览](P13-jx/overview-dark.png)、[逐图像素差](P13-jx/image-differences.txt)。输入恢复每套1503像素变化；忙态3182/3164，空态2246，键盘焦点1155。截图为Linux ui.Tester，未冒充Windows/macOS原生窗口；系统文件/权限选择器由注入回调驱动，没有给原生窗口截图。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹 | 无新增；深浅沿用本仓主题 |
| A2 字体 | 中文系统字体优先，不采用网页换字体通则 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置表单 |
| A5 卡片 | 无新增卡片/三等分布局 |
| A6 假数据 | 产品无新数据；测试使用隔离HTTP与磁盘包 |
| A7 文案 | i18n零改，复用原失败语句；“无法连接此工具”仍偏通用，未另加选择器专用文案 |
| A8 眉批 | 无新增 |
| A9 标题换行 | 真实帧标题完整；无营销H1 |
| A10 Bento | 不适用 |
| A11 竖线 | 无新增装饰 |
| B1 设计读取 | 3/2/7，原生紧凑功能面，与既有方向一致 |
| B2 明暗 | 各Tester固定深/浅，各图未混主题 |
| B3 记忆点 | 安装失败仍留在可继续操作的原上下文 |
| B4 动效气质 | 等待只复用原环，其他恢复即时生效 |
| C1 目的频率 | 只有在飞动作显示环，键盘恢复0ms |
| C2 禁用动画形式 | 无新增all/ease-in/scale(0)/纯fade |
| C3 时长/按压 | 原按压反馈；900ms进度环周期沿用，不是长过渡 |
| C4 origin | 环在原按钮、错误在既有toast栈 |
| C5 reduced-motion | 新帧统一ReduceMotion=true，沿用motion.Value静态降级 |
| C6 hover查询 | Go自绘，不适用CSS媒体查询；原指针路由不变 |
| C7 布局动画 | 无新增 |
| C8 重播关键帧 | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用Go自绘 |
| C11 慢放 | 没有新增运动轨迹；只验确定静态帧，不声称真机慢放 |
| D2-1 字体 | 中文系统字体，未引入webfont |
| D2-2 行高/字号/字重 | 既有Caption9.9/正文10.8不满足通用≥14px/1.5；字号行盒仍需拍板，未擅改 |
| D2-3 混排 | i18n零改；技术包名/版本维持ASCII |
| D2-4 标点/对比 | 复用既有全角文案，无新增斜体；不声称所有次要墨满足7:1 |

自发现缺陷与证据已写入项目§6.251和事件测试，不把本轮局部修复发布成skill全局审美偏好。用户当前明确方向、禁止新增系统、原生自绘条件优先于网页四方向、浏览器尺寸与5项装饰要求。

## 仍待

1. 下一轮首选：同一集成页现有“连接本地工具”表单的准备失败与失败后重试、选择程序与取消的草稿归属；先事件复现，复用现有出口，不改生成包覆盖/保存事务。此次实际检查的是包目录安装，不能把它冒充连接程序表单也已修完。
2. 字号/行盒、accent禁用填充、未知多选值、176-9/161-4保存期草稿、生成工具覆盖、重复包ID覆盖和重装事务、终端S1b/S1c/S2尾继续需拍板。
3. 旧ImageCopy/Rescan时序线索沿用；当前全量结果按最终日志更新，不放宽旧B断言。

## 最终门槛

产品四批均已推送：e5c74f50、f686cbf5、e84ce12d、430dda66。三平台vet/build均exit0，gofmt输出空。
本轮定向race **31 PASS / 0 FAIL / exit0**，覆盖两个包的十个新测试函数；测试函数总数实测3413（开盘3403，+10）。
旧测试文件修改0、删除测试函数0、新增Skip0；V/B/S旧断言修改0/0/0。四组mutation拆成五次执行，源码MD5与touch复验均通过。
2026-11-08 03:59:51 UTC复核最终 **6029 PASS / 0 FAIL**（基线5998，+31），普通与verbose全量均exit0。
最终两份产品源码MD5核对OK；八条stash完整SHA未变。首个记录时钟到完整门槛约24分钟，短于45–75分钟目标；八项均已复现、分批提交并完成门槛。

原始证据：[基线](P13-jx/baseline.txt)、[普通全量](P13-jx/full-test.txt)、[verbose全量](P13-jx/full-verbose.txt)、[race](P13-jx/race.txt)、[测试文件增量](P13-jx/test-delta.txt)、[最终源码MD5](P13-jx/final-source.md5)。三个平台vet/build各有独立txt与exit，五次mutation各有red/restored/MD5。四批提交正文分别记录当批实测，最终报告补齐全量数字。
