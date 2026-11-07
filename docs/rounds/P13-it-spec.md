# P13-it：设置表单的草稿、刷新与失败恢复

起点 `1be93d7a`，2026-11-07 07:39 UTC 开盘；实际最新轮是 P13-is。pull 已最新、工作树空；无 AGENTS.md。仅 Codex 独立执行。

意图：让已有表单显示与存储一致，过期菜单及时退场，失败后保留可继续操作的输入。现有方向是面向桌面用户的轻便液态玻璃设置页；视觉冒险度 3（保留布局）、动效 2（即时键盘反馈）、密度 7（保留原行密度）。用户三秒内应看见当前值，并能修改、明确提交、失败重试。

## 落地项（逐批复现后补充，未证实不计数）

1. `internal/settingsui/settingsui.go:5363`：首次自定义值打开时跳过种子初始化；真实 Tab/Enter 把 45 秒误写为 0 → 首帧同时初始化展开状态和数字草稿。
2. `internal/settingsui/settingsui.go:5389`：菜单只观察 custom 这个显示槽；宿主改为 53 秒后旧 Off 高亮仍提交 0 → 观察原始时长，关闭过期菜单、保留 47 秒未提交草稿与原焦点。

3. `settingsui.go:5419`：预设选择先收起草稿，再写入；保存失败后 30 秒草稿被 45 秒旧值覆盖 → 只有存储接受预设才关闭原输入。
4. `settingsui.go:5427`：自定义 Enter 提交在失败前规范化并关闭输入 → 写入被接受后再更新草稿/展开状态，拒绝时原文本可重试。

5. `settingsui.go:5263`：ParseUint 把超 u32、负数、小数、指数回落为默认 10 秒；参考 `backup/main-pre-mygo:src/surface-residency.ts:106` 为有限数值先截断到 0..86400 再舍入，保留 never 哨兵 → 按既有数值合约解析。
6. `settingsui.go:5367`：输入关闭时宿主写入 53 秒，选择 Custom 同值不产生 Changed，用户无法编辑 → 闭合态收到 custom 时播种并显示输入，活动草稿优先级不变。

参数 Required 和验证即时反馈的真实事件探针原本为绿，不计修复；不改待拍板的草稿优先级。

7. `internal/settingsui/custom.go:501`：新绑定在磁盘失败后仍关闭清空 → 由既有 Store 写入结果决定是否结束草稿，沿用 save alert。
8. `custom.go:541/552`：命令 Enter 先清编辑缓存再保存 → 仅接受后清缓存，原文本/焦点可重试。

同族扩查：命令改型、删除和重新录制也在写回前改本地状态，逐条事件验证；系统占用/冲突的拒绝历史不改变。

## 红线

高度链、键盘模型、会话管理、设置写回时机、i18n 文案、命令接线均保持；不改 180ms/Flush、3000ms，不新增页面、系统、依赖。允许修复状态映射与错误恢复，旧 V/B/S 断言零改。

## 门槛与 mutation

中文 locale `LANG=zh_CN.UTF-8 LC_ALL=zh_CN.UTF-8`；基线实测 **5050 PASS / 0 FAIL / 3110 函数**，全量退出 0。
每批定向事件与包 vet 通过后 commit/push；收尾一次 gofmt -l cmd internal（空）、三平台 vet、普通/verbose 全量、三平台 build。测试函数不可减少。

M1 回注初始草稿与菜单源值遗漏，应使 `TestResidencyRefreshInitialCustom` / `OpenDraftMenu` 红。
M2 回注失败恢复遗漏，应使提交/切换失败事件红。至少两组实跑：红 → 恢复 → md5sum -c → touch → 复验。

## 工艺来源

真实读取 qiaomu-design、user-preferences、craft-loop、preflight、chinese-typography；Apple DESIGN 的单蓝交互角色、2px focus、8px 控件角映射到本仓既有 token，不换主题。
craft-loop #3：0ms 焦点交接与原 2px 环；#4：刷新 0 次写入、明确提交 1 次；#6：失效状态额外占高 0px；#12：保持 E1/E2、新透明面 0 层。用户已指定方向，不制作四方案或新增装饰凑数量。

为什么更轻、更有空间感：只让仍有效的菜单占据浮层，让输入与焦点指向当前可操作的面。
