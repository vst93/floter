# P13-hr：配置刷新后的非文本控件

起点 d3b1961b，工作树干净，pull --ff-only 已最新。禁用墨已在 P13-hl/hm 结案，本轮沿最新待办检查选择控件；不是重复执行过期任务书行号。

## 抢救 stash 清点

当前八条均非 stale-kill，SHA 与 P13-hq 表一致，reverse-check 均退出 1；此前 stale-kill 原件已归档并接续，不重复合入。保留原件，不以 reverse-check 失败推断可覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 早期 keycap WIP，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 早期 rows/slider WIP，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载布局与旧探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧 number-field WIP，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段 WIP，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 WIP，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏 WIP，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri 实现，reverse=1 | 保留 |

## 实施记录

规格八项均先跑真实事件；门槛、变异、图与每批 SHA 随完成增量记录。V/B/S 暂为 0/0/0，旧断言不改。

### 第一批：下拉与开关

1. 现象：下拉跨组后菜单消失，待选 apps 丢失，Enter 后仍是 files。改动：choose 返回原 trigger，插件将其绑定到已有字段焦点句柄。验证：真实 Click→Down→移组→Enter，深浅均选 apps、保留焦点；原实现报 `group move closed the active select menu` / `group move lost pending choice: files, want apps`。图上可见移组后的“应用”仍高亮，菜单位于新触发器下方。
2. 现象：开关跨组后焦点消失，Space 不再切换。改动：checkbox 返回原开关，绑定同一分组焦点路径。验证：点击→移组→Space 后 true 回 false，深浅均保焦点；原实现报 `group move lost toggle keyboard: value=true focus=false`。

相关回归 **146 PASS / 0 FAIL**，settingsui vet 净。M1 回退 pluginsheet.go 两条绑定，新增两函数六个 PASS 项变红，恢复后 MD5 OK、touch 后 **6 PASS**。日志和深浅图在 P13-hr/。旧 V/B/S 断言修改数 0/0/0。

第一批 `a1e2fe9c` 已推送。开盘完整 verbose 实测 **3947 PASS / 0 FAIL / 2850 测试函数**。

### 第二批：单选与复选

3. 现象：单选跨组后 Right 不再选下一项，files 原地不动。改动：原分段轨道绑定字段句柄，分组匹配改用 FocusWithin 识别内部选项。验证：Click 文件→移组→Right 得 apps，深浅均通过；旧实现报 `group move lost radio keyboard: files, want apps`。
4. 现象：复选跨组后 Space 不再取消刚勾选项，焦点丢失。改动：原复选行绑定同一字段句柄，用子树焦点保住组身份。验证：Click 文件→移组→Space 得空列表且焦点仍在文件；旧实现报 `group move lost checkbox keyboard: value=[files] focus=false`。

相关回归 **171 PASS / 0 FAIL**，settingsui/control vet 净。M2 将 FocusWithin 退为 Focused，两项事件实跑红；MD5 还原与 touch 后 **6 PASS**。没有新布局包装，也没有修改旧断言；V/B/S 0/0/0。

第二批 `0805b4b9` 已推送。

### 第三批：滑块与动作取消

5. 现象：滑块跨组后 End 不再响应，数值停在 0。改动：原滑块绑定字段焦点句柄。验证：点击轨道→Home=0→移组→End=100，深浅通过；旧实现报 `group move lost slider keyboard: 0, want 100`。
6. 现象：焦点位于已武装动作的取消按钮时，移组后 Space 无效。改动：原确认按钮行绑定字段句柄，识别其内部焦点。验证：Tab 到取消→移组→Space，仅 Unarm 1 次、Run 0 次；旧实现报 `group move lost cancellation: cancels=0 runs=0 armed="value"`。确认/取消机制未变。

相关回归 **206 PASS / 0 FAIL**，settingsui/control vet 净。M3 回退两条绑定，两项事件实跑红，MD5 还原与 touch 后 **6 PASS**。V/B/S 0/0/0。

第三批 `1bbdc9ed` 已推送。

### 第四批：已收起下拉的外部刷新

7. 现象：下拉用 Escape 收起后，外部把当前值换成未知值，字段焦点丢失，方向键无法重新选择。改动：记录菜单是否展开和触发器句柄，仅对仍可用、原先已收起且拥有焦点的控件交接焦点。验证：外部值先原样显示、配置不脏、菜单不自动打开；Down→End→Enter 可改为便条。旧实现报 `closed select refresh lost keyboard focus` / `refreshed select cannot recover with keyboard: 远程来源, want 便条`。
8. 现象：下拉已收起，schema 删除当前选项，也会使整个字段退出键盘操作。改动：同一交接路径覆盖选项撤回。验证：未显式选择前旧值保留、未写回，随后键盘可选便条；旧实现报 `refreshed select cannot recover with keyboard: 文件, want 便条`。已展开菜单、禁用控件、其他字段焦点继续由原回归守住。

相关回归 **212 PASS / 0 FAIL**，settingsui/control vet 净。M4 回退 selectvalue.go，两项实跑红，MD5 还原与 touch 后 **6 PASS**。V/B/S 0/0/0；测试迁到 selectrefreshfocus_test.go 后产品前红证据中的旧文件行号仍保留。
