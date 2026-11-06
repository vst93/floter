# P13-hw：确认在列表刷新与忙态下的归属

开盘 ac1575a3，pull 已最新、工作树干净。15:04 UTC 开始。Codex 独立执行；不使用其他执行器。基线中文 locale 实测 **4135 PASS / 0 FAIL / 2896 函数**；最初继承 C.UTF-8 的运行不作为设计基线，保留原始输出。

## Stash 清点

本轮逐条导出 patch 并 reverse-check，以下 SHA 与上轮一致且均退出 1。已接续归档的 stale-kill 不在活动清单内；这八条为早期独立 WIP，沿用已审核处置，不把旧主题或 Tauri 链覆盖当前 Go 产品。

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

仅已复现并修复的项目计入八项。旧 V/B/S 断言修改目前 0/0/0。

### 第一批：忙态不占用其他问题的到期唤醒

1. 现象：停用派发后等 4 秒，再打开配置丢弃，静止 3002ms 后丢弃仍开着。改动：忙态停用不参与未回答问题的 next 选择，操作仍由结果回调唤醒。验证：两主题真实 Click/Tab/Space + Advance（到期不强制 Frame），2 秒内保留、3 秒后退出，草稿 edited、保存 0 次；停用仍 busy 且迟到失败正常可读。
2. 现象：移除 busy 同样让后开丢弃丢掉唤醒。改动：移除忙态采用同族规则。验证：相同真实事件与深浅两场景，运行操作不取消、错误不丢失。

定向回归 **101 PASS / 0 FAIL**，包 vet 净。M1 回注旧分支，四场景实跑红：`later discard missed its deadline behind a busy confirmation`；恢复后 md5sum -c 通过，touch 后 **6 PASS**。旧 V/B/S 0/0/0。

第一批 53ee1228 已提交推送。

### 第二批：消失目标的停用/移除问题退场

3. 现象：已打开停用确认的集成从列表消失后，Escape 只关掉看不见的问题；同 ID 重新出现还带着旧问题。改动：View 在路由 Escape 前按当前安装列表退役不存在目标的停用问题、时钟和旧错误，不动 busy。验证：深浅 × 消失后 Escape/重新出现四场景，Close=1 或旧 Cancel 不复活，破坏性调用 0 次。
4. 现象：移除确认同样吞键并在刷新后复活。改动：按同一列表事实清理移除确认。验证：同上四场景，原移除入口仍存在，未派发操作。

相关回归 **111 PASS / 0 FAIL**，包 vet 净。M2 撤销清理入口后八场景红：`stale disable question revived after list refresh`、`invisible remove question swallowed Escape: close=0`（另外两种同族报错也在原日志）。MD5 恢复与 touch 后 **10 PASS**。旧 V/B/S 0/0/0。

第二批 fd376999 已提交推送。

### 第三批：消失目标的丢弃与卸载问题退场

5. 现象：配置丢弃随目标暂时消失而隐藏，却继续吞 Escape，目标回来时重新提问。改动：只 disarm 失效问题，保留 configOpen 与原 draft。验证：两主题 × Escape/返回四场景，问题退场、edited 草稿保留，不派发破坏操作。
6. 现象：组件卸载浮层失去目标后仍保留 open，目标回来时旧浮层盖住页面。改动：不存在目标的 uninstallOpen 退役，不替在飞卸载作完成或取消决定。验证：真实点击移除、Tab/Space 进入组件浮层，再让目标消失；四场景 Escape 正常关闭设置或返回时旧浮层不复活，卸载派发 0 次。

相关回归 **133 PASS / 0 FAIL**，包 vet 净。M3 回注旧的两项遗漏，八场景红：`stale discard question revived after list refresh`、`stale uninstall question revived after list refresh`、两类 `invisible … question swallowed Escape: close=0`。MD5 恢复与 touch 后 **10 PASS**。旧 V/B/S 0/0/0。

第一次目视：浅色 busy-disable 图中，过期的丢弃带已撤回，Token 的 edited 留在原字段，仍在执行的停用带单独留下。没有加材质或提高表面层数；待重排事件完成后再查深浅焦点局部。

第三批 80ff83fd 已提交推送。

### 第四批：列表重排时答案身份跟随集成

7. 现象：停用答案拿到键盘后，列表调换顺序或前插一个集成，答案失焦，Space 派发 0 次。改动：列表行按集成 ID 建立稳定节点身份。验证：深浅 × 重排/插入四场景，答案保持 Focused，Space 只对 first 派发停用 1 次，移除 0 次。
8. 现象：移除答案在相同刷新下也失焦。改动：相同节点归属覆盖移除答案。验证：四场景保留焦点，仅向 first 派发移除 1 次、停用 0 次；不是只看按钮文案。

相关回归 **143 PASS / 0 FAIL**，包 vet 净。M4 撤销行 key 后八场景红：`refresh lost the original answer focus`、`refreshed disable answer lost its owner: disabled=[] removed=[]`、`refreshed removal answer lost its owner: removed=[] disabled=[]`。MD5 恢复与 touch 后 **10 PASS**。旧 V/B/S 0/0/0。

第二次目视：浅色重排图里，Workspace Tools 已移到第二行，蓝色焦点环仍圈住它自己的移除答案；深色忙态移除对照中，原先滞留的丢弃带撤走，表单草稿和仍忙的移除带保留。

第四批 62b165d3 已提交推送。产品固定后普通/verbose 全量各一次，三平台 vet/build 已全过、gofmt 空。完整测试结果待日志结束再录。

完整门槛首次失败：两个旧的设置快照成本 B 测试给出重复 ID `ext`，行 key 直接用 ID 触发 `ui: two elements under one parent have the key "ext"` 崩溃。旧断言与夹具不动；产品改用 ID + 同 ID 出现序号，合法唯一 ID 仍稳定、重复/空 ID 也可画。第四批已先推送，是本轮过程偏差；本次修正通过旧 B 与新事件后单独提交，再重跑完整门槛，不把首次失败隐去。

重复 ID 修正后相关 **151 PASS / 0 FAIL**（包含原两条 B 与六个 surface 场景），settingsui/shell vet 净。最终 key 再做 M4：撤 key 实跑八场景红，MD5 逐字节还原并 touch 后 **10 PASS**。未修改任何旧测试；此项是第 7/8 项实现兼容性的补修，不另外凑第九项。

## 偷了什么

真实读取 qiaomu-design 的 SKILL、偏好账本、craft-loop、preflight、chinese-typography，并读 Apple DESIGN 的色彩/控件章节。既定方向由用户指定，Codex 直接实现；没有四方向预览或外部模型。

- craft-loop 工艺密度菜单 **#3 品牌 focus 环**：采用 Apple 单一交互蓝角色与 **2px** focus 描边，沿用本仓主题值；对象重排后的焦点交接 **0ms**，不额外播放动画。
- **#4 签名交互**：确认沿用 **3000ms**，静止页面在 3002ms 验收退场；取消/刷新派发 **0 次**，刷新后确认只派发原对象 **1 次**。真实 UI 事件替代仅写内部布尔量的测试。
- **#6 编辑细节**：无效问题及时消失，保留 edited 草稿；多余常驻提示 **0 条**、额外占高 **0px**。
- **#12 材质细节**：保留 Apple 控件圆角刻度对应的既有 **8px** 圆角、**1px** 受光边、E1 面与 E2 浮层；新增透明面和阴影 **0 层**。失效浮层不复活，已有空间层级才有含义。

用户要求修已有功能优先，不为菜单凑够五项添加装饰。为什么这轮更轻、更有空间感：过期与失效的提示不再占住表单或浮在页面上；留下的答案和焦点稳定属于当前目标。

## 两轮图像复核

第一轮逐项真实事件捕获 before/after；四视角问题：艺术总监看见重复警告带争夺注意力，工程师看见重排后答案节点换身份，任务用户遇到 Escape 吞键与 Space 取消，读者遇到旧浮层遮住工具行。第二轮集中核对四处：到期只收起丢弃、消失目标不复活、重排保留确认焦点、edited 草稿仍可见。小字号仍是旧拍板项，没有冒充解决。

八项的深浅同帧图全部有可测像素变化；最小为重排移除的 **996 像素**，变动集中在答案焦点环。4× 最近邻局部清楚显示 before 焦点误回 Cancel，after 保留在 Remove package；放大图仅供检查，不冒充高 DPI 真机渲染。

| 项 | 浅色对照 | 深色对照 |
| --- | --- | --- |
| 停用忙态与丢弃到期 | [图](P13-hw/busy-disable-deadline-light-comparison.png) | [图](P13-hw/busy-disable-deadline-dark-comparison.png) |
| 移除忙态与丢弃到期 | [图](P13-hw/busy-remove-deadline-light-comparison.png) | [图](P13-hw/busy-remove-deadline-dark-comparison.png) |
| 停用目标消失/返回 | [图](P13-hw/disable-return-light-comparison.png) | [图](P13-hw/disable-return-dark-comparison.png) |
| 移除目标消失/返回 | [图](P13-hw/remove-return-light-comparison.png) | [图](P13-hw/remove-return-dark-comparison.png) |
| 丢弃目标消失/返回 | [图](P13-hw/discard-return-light-comparison.png) | [图](P13-hw/discard-return-dark-comparison.png) |
| 卸载目标消失/返回 | [图](P13-hw/uninstall-return-light-comparison.png) | [图](P13-hw/uninstall-return-dark-comparison.png) |
| 停用目标重排 | [图](P13-hw/disable-reorder-light-comparison.png) | [图](P13-hw/disable-reorder-dark-comparison.png) |
| 移除目标重排 | [图](P13-hw/remove-reorder-light-comparison.png) | [图](P13-hw/remove-reorder-dark-comparison.png) |

[总览](P13-hw/overview.png)、[浅色 4×](P13-hw/focus-detail-light.png)、[深色 4×](P13-hw/focus-detail-dark.png)、[像素差](P13-hw/image-differences.txt)。原始 PNG 是 720×900 的 Linux ui.Tester 软件自绘图；没有 Windows/macOS 真机交互证据。

## Preflight A / B / C / D2

| 项 | 结论 |
| --- | --- |
| A1 渐变/纯黑/霓虹/渐变字 | 无新增，沿用近黑主题 |
| A2 非系统字体 | 本仓中文系统字体规则优先；不为网页模板规范换装 |
| A3 斜体 | 无新增 |
| A4 Hero | 不适用，原生设置面 |
| A5 三等分/卡片海 | 无新增卡片或包装 |
| A6 假数据 | 无产品假指标；Workspace/Build Tools 是事件夹具 |
| A7 空词/感叹号 | 产品文案零改 |
| A8 眉批 | 无新增 |
| A9 H1 | 既有页头完整可见 |
| A10 Bento | 不适用 |
| A11 左侧竖线装饰 | 无新增 |
| B1 读取/拨盘 | 3/2/7：原功能密度、克制、即时反馈 |
| B2 主题锁定 | 每个 Tester 指定深/浅，未混主题 |
| B3 记忆点 | 问题与焦点始终跟随自己的工具 |
| B4 动效气质 | 键盘恢复 0ms，适合高频操作 |
| C1 目的/频率 | 未为高频键盘增加动画 |
| C2 all/ease-in/scale(0)/fade | 无新增 |
| C3 时长/按压 | 沿用既有 120ms 颜色过渡和 0.97 按压 |
| C4 origin | 内联确认仍紧随原工具；modal 保留中心 |
| C5 reduced-motion | 全部新增事件用 ReduceMotion=true；静止到期路径实测 |
| C6 hover 查询 | Go 自绘，无 CSS 媒体查询；原指针路由不变 |
| C7 布局动画 | 无新增；到期/删除是既有布局收回 |
| C8 重播 keyframes | 无新增 |
| C9 手势 | 未涉及 |
| C10 Framer | 不适用 Go 自绘 |
| C11 慢放 | 无新增动效，未声称慢放真机验收 |
| D2-1 系统字体 | 沿用，无 webfont |
| D2-2 字号/行高/字重 | 通用 ≥14px/1.5 未满足；旧 Caption 9.9/正文 10.8 与行盒仍待拍板 |
| D2-3 混排空格 | i18n 零改，不引入新的中文混排句 |
| D2-4 标点/暗色对比 | 墨与原标点不改，不宣称旧次要墨全部过 7:1 |

可复现缺陷落入本仓 §6.198 与事件 pin；不把一次性修复改成全局 skill 的审美偏好。用户指定已有方向优先于四方案/浏览器 viewport/工艺数量等泛化要求。
