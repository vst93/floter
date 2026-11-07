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
