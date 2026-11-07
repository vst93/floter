# P13-iv：默认快捷键恢复与拒绝后的键盘出口

开盘 `0726b2c4`；pull 最新，工作区干净。Codex 独立执行，无外部模型。done.log 开盘不存在。基线 **5129 PASS / 0 FAIL / 3134 测试函数**，基线进程退出 0。以下随每批增量记录，只填写实测。

## Stash 清点

当前八条与已审核早期 WIP SHA 一致；reverse-check 均退出 1。没有活动 stale-kill 独有稿，不重复把旧设计合入当前 Go 版本。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 早期 keycap；reverse 不通过 | 保留 |
| stash@{1} | 8af94a07 | 早期 rows/slider | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri | 保留，不覆盖发布链 |

## 实施与验证

旧 V/B/S 修改 0/0/0，未新增 Skip。

### 第一批（1–2）

1. 现象：恢复默认被系统拒绝后，Store 已变成 Ctrl+Space，而实际仍是 Ctrl+Alt+J。改动：先取得默认键，再一次性确认设置；拒绝不写，磁盘拒写释放试注册键，原绑定保留。验证：宿主真实 Actions 回调，检查系统键集合、原显示、原 app 键与写入次数，再解除占用重试。
2. 现象：恢复默认的 OS 拒绝只有日志，页面无提示。改动：恢复按钮读取已有 ShortcutRefused 回调，复用顶部与行内拒绝提示。验证：深浅真实 Tab/Enter，原代码两主题均报 `system-refused reset has no visible recovery feedback`，修后提示可见。

M1 已回注旧恢复流程实跑红，原串 `system-refused reset persisted unregistered defaults`。MD5 还原通过，touch 后定向回归与包 vet 见 batch1.txt；计数在提交时补录。
