# P13-iy：快捷键表单的编辑与退出归属

开盘 a74fcfb5，pull 最新、工作区干净。实际上一轮 P13-ix，未按旧任务书重复 P13-hw。Codex 独立执行。读取 qiaomu-design 指定参考与 Apple DESIGN。开盘 done.log 不存在。

## Stash 清点

8 条 SHA 与上一轮相同，reverse-check 均退出 1。与 stash-backlog 和 P13-ix 表比对，是早期独立 WIP，没有新的 stale-kill；保留原件，不把旧主题/Tauri 链重放到 HEAD。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse 1 | 保留 |

## 增量记录

基线完成：**5206 PASS / 0 FAIL，3165 测试函数**。先执行候选事件探针；失败、通过与待拍板分别记录，不把探针通过算修复。

### 第一批：字面命令

1. 现象：保存命令行输入 `command` 并按 Enter，绑定消失，出现空命令草稿。改动：命令编辑提交直接更新命令值，菜单的 mode sentinel 只属于菜单。验证：深浅主题真实输入/Enter，`TestCustomCommandLiteralDoesNotBecomeMode` 旧代码均红，原串 `literal command became a mode switch: entries=[] draft=true`；修复后必须保持原键、保存字面命令、不出现新草稿。

其他探针：命令 Bind 焦点、模式往返文本、成功换键焦点、提交 trim/空值恢复、重复录键重试、草稿重建 Undo、动作选择焦点均原代码通过，均不计修复。原始日志 probe1/probe2。

需拍板：真实 Add→录 Z→另一保存 B 的动作菜单选择命令，已有 Z 被 B 覆写。现有设计仅有一个草稿槽；保留多个草稿、禁止转换或确认覆盖会改变交互，暂不修改，已实证且纳入收尾队列。

第一批相关 **128 PASS / 0 FAIL**、包 vet 净；M1 恢复旧命令提交分支后 **3 FAIL**，MD5 OK、touch 后 **3 PASS**。V/B/S **0/0/0**。转换覆盖的探针只留日志，不把“证明旧缺陷存在”的断言放入长期产品测试。

第一批 **45f3c096** 已推送。

### 第二批：同一行的命令与键帽

2. 现象：命令输入 `say unfinished` 尚未 Enter，改录 Z 后输入框回到 `say original`。改动：换键时命令草稿随原行迁到新键，仍只在 Enter 提交。验证：真实输入→录键→读编辑器→Enter；原串 `changing key discarded command draft: "say original"`，深浅均红。
3. 现象：720px 完整设置页的长组合键越出键帽，叠到垃圾桶与动作菜单。改动：键帽文字单行可收缩并省略，命中框与全量 accessible 文本保持。验证：真实长键文本右边界必须在删除控件之前；原图显示覆盖，原串 `key text overlaps remove: key={X:210.40001 Y:670.89996 W:164.00002 H:17} trash={X:300.4 Y:666.7999 W:25.200012 H:25.200012}`。

第二批相关 **11 PASS / 0 FAIL**、包 vet 净。M2 同时恢复两处旧分支后 **5 FAIL**，两文件 MD5 OK、touch 后 **12 PASS**（含 recorder 保护测试）。V/B/S **0/0/0**。

第二批 **60f34de0** 已推送。

### 第三批：大字号下两个编辑行的可触达性

4. 现象：large 档已保存命令行选择器的左沿盖住垃圾桶。改动：快捷键行前列留 30u 最小宽度、后列与命令输入横向收缩。验证：深浅中文完整 720px 页断言删除按钮右沿不超过 picker 左沿。
5. 现象：large 档命令草稿的选择器盖在录制键上，点击键帽会落进菜单。改动：同一行宽分配保护草稿键帽/删除组，命令框让出横向空间。验证：真实 Add/录键/转命令后点击原键帽中心，必须启动草稿录制；深浅均原代码失败。初始仅检查窗口边界的探针全部通过，目视发现内部遮挡后补严格邻控件与点击检查；不以初始通过冒充无问题。

第三批相关 **161 PASS / 0 FAIL**、包 vet 净。M3 撤行宽保护，原串 `saved picker overlaps remove: trash={X:301.8 Y:760.7 W:30.799988 H:30.799988} picker={X:301.99997 Y:760.7 W:154 H:35.200012}` 与 `draft picker covers recorder hit target`。V/B/S **0/0/0**；只改横向分配，行盒与高度链保持。

第三批 **87b6037f** 已推送。M3 **5 FAIL**，MD5 OK、touch 后 **11 PASS**。

### 第四批：动作与拒绝说明的身份

6. 现象：已不可识别的 plugin/action 存值被显示成“命令行”，且再选命令行没有反应。改动：未知动作显示原 ID，保留未知选择值；自由文本命令才映射命令槽。验证：两种前缀、深浅主题均要求原值可见，并能真实选命令转入草稿；原串 `unknown action "plugin:removed-tool" disguised as command mode` / `unknown action cannot switch to real command mode`。
7. 现象：B 录成 A 被本地拒绝后，绑定另一个已完成的 Z 草稿会抹掉 B 的冲突说明。改动：Bind 只清草稿自己的 local rejection；成功提交仍沿原时机。验证：真实 Add Z→B 录 A→Bind，Z 已保存，B 的拒绝说明仍在；原串 `binding unrelated draft erased saved refusal`，深浅均红。

第五批的焦点问题已把“缺少 setter”探针换成真实磁盘失败：合法 settings 文件替换为同名目录，转换触发 Store 写失败，原绑定与现有草稿均保留，但焦点错误跳到草稿命令。原串 `failed conversion stole focus into unrelated command draft`，深浅均红。不改变提交时机或失败写回。

第四批相关 **24 PASS / 0 FAIL**、包 vet 净。M4 回注未知值伪装与无条件清拒绝，实跑 **8 FAIL**，MD5 OK、touch 后 **8 PASS**。V/B/S **0/0/0**。

### 第五批：转换失败的焦点出口

8. 现象：页面已有命令草稿，另一保存行尝试改命令但磁盘拒绝写入，键盘跳入无关草稿。改动：只在保存成功、目标草稿真正建立后设置焦点交接；失败保持原选择器。验证：真实文件写失败、原绑定和草稿不变、焦点仍在原动作菜单；原串见上，深浅均实红。此处没有解决需要拍板的多草稿覆盖，只修失败出口。

第四批 **d7bbe5e8** 已推送。第五批相关 **171 PASS / 0 FAIL**、包 vet 净。M5 回注失败仍交焦点的旧分支，**3 FAIL**，MD5 OK、touch 后 **3 PASS**。最后布局用例改为真实菜单操作进入 command，另补长动作真实存值；原断言不弱化。

## 偷了什么

- craft-loop **#3 品牌 focus 环**：Apple DESIGN 的 **2px** focus 与单蓝交互色沿用；转换失败不把环移给无关输入框，交接 **0ms**。
- **#4 签名交互**：录制者仍最多 **1** 个；改键保留 **1** 份原行命令草稿，未 Enter 写命令次数 **0**。
- **#6 编辑细节**：大字号前列至少 **30u**（large 为 **132px**），删除与 picker 之间不重叠；拒绝说明只留在自己的行，未新增常驻说明。
- **#12 材质细节**：沿用 E1 **1px** 受光边、常规控件 **8px** 圆角；新玻璃层 **0**、新阴影 **0**。优先两轮截图与功能完整度，依 craft-loop 自身优先级不为了第五项加装饰。

为什么更轻、更有空间感：为可操作控件留出真实横向空间，文字不再穿过相邻面；状态和焦点停在所属行，沿用底/面/浮层级，无额外卡片或提示层。
