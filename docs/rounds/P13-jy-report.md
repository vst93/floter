# P13-jy · 本地工具连接恢复

2026-11-08开盘，首个时钟04:04:15 UTC。HEAD6325a179，工作区干净，pull已最新。P13-jx已做完registry失败恢复，按其仍待继续本地工具连接。指定done.log不存在。规格见同目录P13-jy-spec.md。

## stash清点

本轮逐条实际reverse-check均1，完整SHA与交接相同。八条均是早期独立WIP，沿用交接审计保留，不用旧主题/旧Tauri链覆盖当前Go实现；无新增抢救稿、无drop。

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

## 增量记录

### 第四批：选择成功与提交后的键盘出口

7. **选择成功后不能立即修订路径**：现象→新路径已写入但焦点仍在选择按钮；改动→成功选择也返回程序输入；验证→键盘选择后路径为/usr/bin/forecast，直接全选键入得到/usr/bin/retry；新旧表单归属继续由已有回调测试守住。
8. **提交收起表单后键盘悬空**：现象→原提交按钮退场后没有可操作的焦点；改动→仅本次连接提交把焦点交给同表单收起后的添加入口，token变化则退役；验证→普通提交和等待提交两种、深浅两套，Enter直接打开下一份空草稿，完成等待不抢焦点。

定向59 PASS/0 FAIL/exit0、vet净。M4撤掉成功选择返焦点和提交返焦点，实红`picked picker did not return program focus`、`picked picker lost immediate keyboard edit: "/usr/bin/forecast"`、`submitted form lost keyboard exit: calls=1 open=false addFocus=false`；MD5还原并touch后59 PASS。旧V/B/S断言0/0/0、新增Skip0。

### 第三批：选择器错误与取消出口

5. **选择程序失败无反馈**：现象→原生选择器返回错误却被吞掉；改动→向已有宿主警告出口发LocalToolFailed，保留原路径，旧表单退出后不再发过期警告；验证→真实按钮回调返回os.ErrPermission，警告恰1条，完整宿主ui.Tester可见错误浮层；新编辑器替换后的错误不警告、不覆盖。
6. **取消选择后键盘留在按钮**：现象→取消后直接输入无法修订程序路径；改动→空选择返回程序字段；验证→Tab/Enter打开选择器、取消后直接全选输入，字段变为/usr/bin/retry，取消无警告。

定向51 PASS/0 FAIL/exit0，vet净。M3实红`program picker error warnings=[]`、`cancel picker did not return program focus`、`cancel picker lost immediate keyboard edit: "/usr/bin/weather"`及`program picker failure never reached host notice`；MD5还原、touch后51 PASS。旧V/B/S0/0/0、新增Skip0。第二批08f2e09f已推送。

中文基线完整完成：6029 PASS/0 FAIL/exit0、3413函数。第一批0ff958a2已推送。

### 第二批：提交失败与等待互斥

3. **提交失败**：现象→审核后声明文件消失，提交报错且草稿丢失；改动→Commit失败也走本表单恢复出口；验证→完整设置页选择程序、输入名称/命令、声明网络权限、Enter提交、审核中移除声明，失败后重新提交的请求严格相等。
4. **等待重复连接**：现象→第一次还在准备/审核，第二份完整草稿也能派发；改动→本地表单宿主入口互斥，折叠时在添加入口旁显示已有连接忙态，打开时提交按钮忙态；验证→等待中的完整第二份草稿不派发，完成后解锁；添加入口仍允许编辑新草稿，已检测工具的独立连接路径保持原样。

41 PASS/0 FAIL/exit0、定向vet净。M2撤回忙态和三处恢复，实红`busy connection dispatched 2 requests`与`commit recovery=0 settled=1, want one each`（准备/取消也再次实红）；两文件MD5还原、touch后41 PASS。前三个异常现已由完整设置页真实输入→磁盘/权限回调→重试覆盖，深浅出图。夹具最初把“程序”类型按钮当成同名输入，已改用实际选择程序入口；同名连接按钮改为从本表单用Tab到提交，不改产品来适配夹具。旧V/B/S断言0/0/0、新增Skip0。

首次基线LANG被LC_ALL=C.UTF-8覆盖，出现已记录的字体pin失败，已终止并用中文LC_ALL重跑；不修改旧断言。后续只记录完整退出的实测结果。

### 第一批：准备失败与权限取消

1. **准备失败**：现象→宿主拒绝不存在的程序后仅通知，表单已清空；改动→同步提交捕获本表单的恢复出口，失败在UI线程恢复原草稿；验证→真实磁盘失败调用恢复1次，ui.Tester程序/脚本两种表单逐字段保留（含参数、声明、平台、隐藏模式草稿和原始空白），再次提交请求严格相等。
2. **权限取消**：现象→拒绝权限后进入Commit并报“连接失败”，表单不可重试；改动→拒绝直接返回并恢复草稿；验证→真实PrepareCustom和拒绝回调，恢复1次、通知0条、仓库条目0；新输入、主动清空、折叠与另一个编辑器四种状态不被旧结果替换。

定向与既有回调契约共35 PASS/0 FAIL/exit0，定向vet净。M1撤回两条宿主恢复及表单出口，实红`failed connection lost complete draft: open=false name="" program="" script="" params=[]`、`prepare recovery=0 settled=1, want one each`、`cancel recovery=0 settled=1, want one each`；两文件MD5逐字节恢复，touch后35 PASS/exit0。旧V/B/S断言修改0/0/0、新增Skip0。夹具初次窗口1100px不足以点击1319.7px处提交，后改用既有1500px测试画布；另一初次把参数渲染前未初始化的key用于相等比较，已在首帧后取样。均不是产品mutation证据，未放宽产品断言。
