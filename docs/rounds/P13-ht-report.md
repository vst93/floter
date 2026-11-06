# P13-ht：集成动作退出与刷新后的恢复

开盘 `a1674426`，pull 最新，工作树干净。基线实测 **4007 PASS / 0 FAIL / 2870 测试函数**。done.log 开盘不存在。

## Stash 清点

活动八条 SHA 与 P13-hs 完全相同，逐条 reverse-check=1，属于早期独立 WIP；抢救 stale-kill 已有归档/接续记录，不重复试合，不覆盖当前 Go/主题实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实施记录

候选与真实行号见规格；下面随批次记录实测，未完成项不冒充修复。

### 第一批：参数随清单同步

1. 现象：表单已开后新增格式/数量参数，默认 `json`/`12` 没有显示。改动：按当前参数定义补齐缺少的默认答案，保留存量手改值。验证：深浅真实帧，旧实现报 `new parameters did not receive defaults without replacing edits: map[limit: target:my edit]`；修复后 target 仍为 my edit，format=json、limit=12。
2. 现象：删除参数后运行仍携带旧 key，重新加入也复活旧答案。改动：按当前完整 ID 集清除退役答案（含无默认值参数，不能用 defaults 的 key 集代替 schema）。验证：真实点击 Run 只派发当前 target，重加采用 fresh；旧实现报 `removed parameter reached command: map[obsolete:old target:current]` 与 `re-added parameter resurrected old answer: "old"`。

V/B/S 0/0/0，旧测试零改。M1 用 HEAD 产品回注两缺陷，红/绿原始日志与 MD5 校验落轮次目录；深浅新增参数前后图已导出。

第一批 `364ef2b8` 已推送；run 相关回归 69 PASS，M1 缺陷注入退出 1、恢复后 6 PASS，settingsui vet 净。

### 第二批：编辑与已提交答案分开

3. 现象：文字参数输入 24 后改成数字，Cmd/Ctrl+Z 将答案撤销回旧的 8。改动：参数编辑器身份包含 kind，改型时退役旧编辑历史，值仍保留 24。验证：两主题真实键盘事件；旧实现 `new number editor resurrected text undo: "8"`，新实现撤销不再引入旧答案；原参数重排保焦回归仍通过。
4. 现象：运行回调拿到表单 map，后续输入 second 把已提交 first 改掉；回调修改 map 又污染当前输入。改动：提交时拷贝字符串答案 map。验证：真实 Run 后继续编辑，命令快照仍 first、表单仍 second，派发恰好 1 次；原红串 `later edit rewrote submitted command: map[target:second]` / `command map mutated live form: "host-owned"`。

V/B/S 0/0/0。M2 回退本批产品变动，实际红；恢复后 MD5 校验、touch 复跑，原始日志随本批入库。

第二批 `dfa8ef77` 已推送；相关回归 73 PASS，M2 恢复后 4 PASS。

### 第三批：配置折叠确认的键盘出口

5. 现象：取消确认后焦点落空，下一次 Space 无法再打开确认。改动：确认退场当帧将焦点交回同一配置入口。验证：深浅真实取消后 Configure 有焦点，Space 再次打开确认，edited 草稿保留、保存次数 0。原红串 `config-cancel lost focus instead of returning to Configure`。
6. 现象：每帧强制聚焦取消，Tab 到不了丢弃；即便鼠标点丢弃，折叠后入口也无焦点。改动：取消只在确认初次出现时 AutoFocus，丢弃后沿同一出口归还入口。验证：Tab→Space 丢弃、Space 重开得到 stored，保存次数 0；原红串 `Tab could not reach Discard`。不是新增键盘模型，而是让既有可聚焦按钮可达。

M3 HEAD 回注两缺陷实跑红，MD5 恢复与 touch 后配置族回归。V/B/S 0/0/0。

候选修正：卸载“取消后重开旧错”探针绕过真实入口；真实 removalConfirmation:6957 已清错误，排除该候选、不改产品、不计修复。第 7 项改查真实 runForm 取消焦点（已复现），第 8 项继续卸载失败重试。

第三批 `546de9d0` 已推送；配置族 42 PASS，settingsui vet 净。

### 第四批：运行退出与卸载重试

7. 现象：参数表单取消后没有键盘落点，Space 不能重新展开。改动：原动作行绘制时，若同帧由表单取消而收起，将焦点交回 Run 入口。验证：真实集成页取消派发 0 次，随后 Space 重开，edited 参数仍在；原红串 `cancelled run form lost disclosure focus` / `keyboard reopen lost parameter draft`。
8. 现象：卸载局部 Handle 每帧新建，FocusWithin 永远读不到已提交身份，Tab 八次仍困在取消；失败插入错误带还会改变动作行 ID。改动：查询当前 panel Element 的 FocusWithin，保留延迟 focus 请求；给动作行稳定 key。验证：从真实 opener 打开仍先聚焦 Cancel，Tab 可达卸载，失败句插入后焦点仍在卸载，同步与延迟失败均可 Space 重试；错误原文保留、两次确认恰好派发两次。原红串 `Tab did not reach uninstall`，逐帧探针八次 `cancel=true`。

V/B/S 0/0/0，旧行为断言零改。八项跨运行参数、配置确认、卸载对话框三个组件；未新增页面/系统/依赖。

运行取消还需固定 `integration.actions` 行身份：仅同帧 Focus 会在下一帧因上方表单消失而失效，真实键盘 pin 抓到后补齐。M4 首次还原校验发现卸载注释从 Binding 改成 Querying 没进入备份；按 MD5 恢复该注释后再次逐字节核对和 touch 复跑，不隐瞒中间失败。
