# P13-id · 选项局部刷新后的键盘出口

开盘 HEAD 7823b5dd，pull 最新、工作树干净；实际接 P13-ic 最新仍待。Codex 独立执行。done.log 不存在，结束时创建。

## Stash

无新 stale-kill，下面都是交接账本已审核的独立旧 WIP；SHA 与上轮相同，reverse-check 均为 1，不据此推断存在可合入新工作。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 复现

四条 select 均在深浅失败；多选初次无前项的夹具掩盖了 Tab 回整页头的问题，加入真实相邻控件后两条集成多选和插件 Checkbox/Radio 均红。Radio 必须以既有方向键进入第二项，Tab 只进当前项；未修改键盘模型来适配夹具。

旧 V/B/S 断言修改 0/0/0，新 Skip 0。开盘完整基线退出 0：4473 PASS / 0 FAIL / 2960 测试函数。

## 第一批

1. 现象：菜单打开时宿主把值刷新成未知 ID，旧菜单消失后方向键无响应。改动：失效后新触发器继承真实持焦入口，继续关闭、不自动改值。验证：Tab → Down → 外部刷新 → Down/End/Enter 选择便条；深浅通过。原始变异报错 `external refresh lost select keyboard exit`、`external cannot choose after refresh: 远程来源`。
2. 现象：当前选项被移除，旧值保留正确但键盘丢失。改动：同一失效路径保留触发器焦点。验证：移除文件后仍显示文件，用户明确 Down/End/Enter 才替换为便条；深浅通过。原串 `removed refresh lost select keyboard exit`、`removed cannot choose after refresh: 文件`。

M1 两条深浅共四个叶子实红，cp 还原后 md5sum -c 为 OK，并 touch/count=1 复验。图片目视：浅色 after 的既有蓝环清楚围住入口，深色 before 无环；字段、操作行与留白尺寸保持。

流程偏差：首批 c20ae5c7 在同族门槛退出 1 后被错误提交推送，提交正文的“回归与 vet 已通过”不实（当时 122 PASS / 6 FAIL，vet 未执行）。原因是忽略返回退出码。旧鼠标路径要求失效后 Enter 不重开，补偿一概恢复焦点破坏了该行为。后续限于真实键盘可见焦点，旧 B 断言零改；修正通过后另提交，不改写历史掩盖此事。

第一批修正 aebe747a 已推送；128 PASS / 0 FAIL，包 vet 退出 0。基线 4473 PASS。

## 第二批

3. 现象：下拉选项全部清空，恢复后原焦点丢失，方向键不能开菜单。改动：键盘持有的不可用入口保留身份，但不画菜单、不响应输入；恢复时清旧菜单并承接仍归属此处的焦点。验证：清空 → 恢复 → Down/End/Enter 正确选择；主动移到 Copy 后不抢回。原串 `empty refresh lost select keyboard exit`。
4. 现象：修复忙态锁住表单后解锁，焦点同样掉出下拉。改动：同一可用性恢复链保留原键盘归属。验证：repairBusy=true → false → 原入口可操作；忙时主动移焦后恢复不抢焦。原串 `unlock refresh lost select keyboard exit`。

第二批同族回归 134 PASS / 0 FAIL，包 vet 退出 0；补充主动移焦四叶子 5 PASS。M2 四叶子实红，MD5 OK、touch 后 select 新事件 17 PASS。旧 V/B/S 0/0/0。

## 第三批

5. 现象：多选当前焦点项被删除，Tab 跳回整个表单开头的前项。改动：用实际持焦选项的 Handle 检测退场，暂回既有列表框；该框不处理 Space，下一次 Tab 进入剩余选项。验证：前项 → 文件 → 删除文件 → Space 不写入 → Tab 应用 → Space 只勾应用。原串 `multi-removed Tab did not continue at remaining options`。
6. 现象：选项全部清空后再恢复，Tab 同样从整表重走。改动：中性列表框只在拥有焦点时保持临时遍历位置，恢复列表不挪焦、不写值。验证：清空 → Space 无写入 → 恢复 → Tab 应用 → 正确勾选。原串 `multi-empty Tab did not continue at remaining options`。

M3 撤掉退场交接后四叶子实红，MD5 OK、touch 后同族回归已完成；最终数字见 batch3.txt。无旧断言修改。
