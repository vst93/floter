# P13-iu：设置写入失败后的实际行为与错误出口

进行中；本报告随每批验证增量更新，只记录实测。

## 开盘与 stash

pull 为最新、工作区空；真实进度 P13-it。P13-hw 的后继问题已有多轮修复，不重复选题。done.log 缺失，最终重建。

| 开盘 ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | aa5e214d | reverse=1；P13-it 独有收尾文档 | apply -3 干净恢复，补验后单独提交；提交后删除此 stash |
| stash@{1} | 422483b4 | reverse=1；早期 keycap WIP | 按既有交接保留，不覆盖新设计 |
| stash@{2} | 8af94a07 | reverse=1；早期行/滑块 WIP | 保留 |
| stash@{3} | 7e4aa710 | reverse=1；旧卸载框 WIP | 保留 |
| stash@{4} | 1e3aec94 | reverse=1；旧数字字段 WIP | 保留 |
| stash@{5} | 363f5a42 | reverse=1；旧插件配置 WIP | 保留 |
| stash@{6} | 9b8ce23c | reverse=1；旧键形状校验 WIP | 保留 |
| stash@{7} | 90d314b2 | reverse=1；旧启动器 WIP | 保留 |
| stash@{8} | 7a9f6e13 | reverse=1；旧 Tauri 主线 WIP | 保留，不重合旧发布链 |

## 验证进度

开盘三平台 vet/build 退出 0，gofmt 空；全量基线运行中。原始证据暂存 `/tmp/floter-P13-iu/`，收尾入仓。

基线完成：5096 PASS / 1 FAIL / 3121 函数；失败为 P13-it 留下的旧 B `TestWindowBehaviourControls`。抢救提交 `f68577d3` 修复自动展开与活动草稿的区别，相关 38 PASS / 0 FAIL、包 vet 净，旧断言未动。stash `aa5e214d` 已提交后删除，其他八条继续保留。

## 第一批（1–2）

1. 现象：新增、删除、重新录制自定义键写失败后，系统注册仍对应未保存列表。改动：失败后从 Store 确认值恢复系统键集合。验证：真实文件路径改为目录拒写，注册钩子记录键集合；三条路径都回到原键，恢复文件后重试成功。
2. 现象：同一键修改命令失败，按原键执行 `say uncommitted`。改动：回滚时重新绑定原动作闭包。验证：调用实际注册回调只得到 `say original`，再次保存可成功。

M1 删除注册回滚，两个 pin 实红；原串 `failed add left uncommitted system keys` / `failed command update dispatched uncommitted action: [say uncommitted]`；MD5 OK 后 touch 复验。旧 V/B/S 0/0/0。定向与变异计数见 batch1/M1 原始日志。

第一批 `bf6a719e` 已推送；8 PASS，M1 5 FAIL → MD5 OK → touch → 5 PASS。

## 第二批（3–4）

3. 现象：唤起键保存失败已显示新键，系统也留下新键。改动：只有存储接受后才释放原键；失败释放试注册的新键。验证：真实磁盘失败，旧键/显示不变、新键不留；恢复后重试旧键释放、新键注册。
4. 现象：内置键录制保存失败，录制框退出却没有保存反馈。改动：观察同步宿主回调本次写入结果，接上既有保存错误面。验证：深浅真实 Tab/Enter/录制键，出现既有提示；无错误冲突标记，恢复文件可重录成功。

相关宿主 9 PASS / UI 12 PASS，包 vet 净。M2 漏释放失败的新唤起键，原串 `failed summon save changed system registration: [Ctrl+Alt+J Ctrl+Alt+K Ctrl+Shift+B]`；1 FAIL，MD5 OK + touch 后 1 PASS。首次变异恢复遇备份尚未生成，未提交；重新顺序备份、注入、还原和复验完成后才提交。旧 V/B/S 0/0/0。

第二批 `3b27a03e` 已推送。

## 第三批（5–6）

5. 现象：恢复快捷键默认值写失败，旧键还在却没有提示。改动：现有恢复入口观察本次 Store 写入错误。验证：真实按钮点击 + 磁盘拒写，旧键保留、提示出现；恢复路径后再点可写成功。
6. 现象：内置插件开关写失败静默跳回，用户不知道是否生效。改动：接上同一既有保存提示面，仍从 Store 读回确认状态。验证：深浅 Tab/Space，旧开关值保留、提示可见，文件恢复后第二次操作成功。

M3 反转“本次确有写入”条件，录制/恢复/插件三组事件 9 FAIL，MD5 OK + touch 后 9 PASS；原串 `reset write failure has no visible recovery feedback`、`plugin write failure has no visible recovery feedback`。旧 V/B/S 0/0/0，定向计数见 batch3。

第三批 `571ae9ad` 已推送，17 PASS。

## 第四批（7–8）

7. 现象：集成命令开关保存失败静默跳回。改动：同步宿主写入接入已有错误面，原开关/运行状态由确认值驱动。验证：深浅真实 Tab/Space 拒写显示提示，旧值不变，恢复磁盘后可重试。
8. 现象：别名写失败后无变化的 Commit 返回 nil；多别名中后一条成功还会清 Store.LastError，前一条失败被吞掉。改动：逐次回调完成就记住该次失败。验证：真实输入，179ms 不写、180ms 单次写；首条失败/次条成功仍显示错误，分别持有正确的已存值，恢复后 Flush 可重试。

M4 回退逐次别名错误观测，5 FAIL；原串 `alias-single lost the failed alias write behind a later commit` / `alias-mixed lost the failed alias write behind a later commit`。MD5 OK + touch 后 5 PASS。额外保护：无新写入与 OS 拒绝均不会重放旧磁盘错误。旧 V/B/S 0/0/0；相关计数见 batch4。

第四批 `63961993` 已推送，59 PASS。

## 抢救回归补强（9）

9. 现象：P13-it 预存 B 红，自动显示的 45 秒遮住宿主后来写入的 120 秒；用户提交预设关闭输入后再经历同样刷新也会重现。改动：自动展开与用户活动草稿分别跟踪，提交预设关闭输入时释放草稿归属。验证：原 B 不改，新增真实选择/输入/Enter → 宿主 45 → 120 事件；修前 `committed preset left stale draft ownership on a later host refresh`，深浅均红。活动未提交草稿优先级、菜单模型与写入时机保持。
