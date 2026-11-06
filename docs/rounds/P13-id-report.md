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
