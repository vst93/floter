# P13-ix：录制期间的自定义绑定编辑与草稿退出

开盘 f20d1c6c，pull 最新、工作区干净；Codex 独立执行。读取 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography。done.log 开盘不存在。遵循仓库最新 P13-iw 仍待，不重复旧轮次。

## Stash 清点

八条均与最新交接 SHA 相同，reverse-check 全部退出 1；是已审核的早期独立 WIP，无新的 stale-kill。保留原件，不重放旧主题与 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 早期 keycap，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 早期 rows/slider，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse 1 | 保留 |

## 增量验收

全量基线运行中。八项先事件复现；每批相关测试与 vet 通过后提交。最终数字、截图、mutation 均以实测补录。

基线完成：**5177 PASS / 0 FAIL**，退出 0。git 跟踪文件口径 **3152 个 func Test**（上轮报告记 3154，本轮不照抄）；最终使用同一口径加未跟踪新增文件。原有测试文件零改。

### 第一批（1–2）

1. 现象：唤起录制仍在等键，鼠标改另一条自定义动作后，系统已有两个可派发绑定。改动：显式提交仍按原时机验证和保存，若真实录制状态仍活动，交回前释放临时取得的系统键；完成捕获不重复释放。验证：真实录制→打开动作菜单→选 Browser，确认新动作已存、live 为空；Escape 后恢复最新绑定。原串 `action activated system keys during recording: [Ctrl+Shift+A Ctrl+Shift+B]`。
2. 现象：录制 B 时删除 A，B 在录制完成前被系统重新注册。改动：同一显式编辑出口保持录制暂停。验证：真实删除后列表仅 B、系统零绑定，Escape 后只恢复 B 与唤起键，A 不复活。原串 `remove activated system keys during recording: [Ctrl+Shift+B]`。

两条在深浅主题原代码均红，原始日志 batch1-before.log。当前 V/B/S 修改 **0/0/0**，新增 Skip **0**。

第一批相关 **157 PASS / 0 FAIL**，包 vet 退出 0。M1 撤暂停守卫后 **6 FAIL**（包含父测试），MD5 OK、touch 后 **6 PASS**。完整全量最终执行；本批验证通过后提交推送。

第一批 **8fd66cbb** 已推送。

### 第二批（3–4）

3. 现象：有完整未提交草稿时，切到已保存 B 录制，再删除草稿，B 的录制也被取消，下一键无法改 B。改动：草稿关闭只结束属于草稿的录制/重复提示，保留其他行归属。验证：真实 Add→录 Z→录 B→点击草稿 trash，之后录 R 仍写回 B 的原动作。
4. 现象：草稿已有 Z，再点其录制器后直接 Bind，草稿消失但不可见的录制状态继续暂停全部系统键。改动：成功绑定复用草稿关闭出口，结束其录制，焦点仍归新保存行。验证：真实重录→Bind 后 Z、B、唤起三键全部可用，暂停为 false，原录制入口仍可继续。

第二批相关 **163 PASS / 0 FAIL**、包 vet 净。原串 `canceling draft stopped saved binding recorder` 与 `bound draft left an invisible recorder suspending all keys`。M2 恢复两处旧状态处理后 **6 FAIL**，MD5 OK、touch 后 **6 PASS**。旧 V/B/S **0/0/0**。

第二批 **a602fce1** 已推送。

### 第三批（7–8）

7. 现象：录好 Z 后系统占用 Z，Bind 被宿主拒绝，但草稿消失且 occupied 提示没有承载行。改动：宿主仍只保存接受的绑定，UI 保留被拒的草稿及既有拒绝说明。验证：真实 Add/录键/Bind，occupied 和 Bind 仍可见，重新录 R 后成功保存且旧提示退场。原串 `draft-occupied discarded draft or hid refusal`。
8. 现象：草稿录 Z 后配置刷新增加同键已有动作，Bind 被 normalize 去重，原动作虽保住但新草稿静默丢失。改动：提交前重新检查当前绑定集合，复用录键时的本地冲突提示，保留草稿，不提交无效列表。验证：真实录键→外部增加 Z→Bind，原动作和写入次数不变，冲突可见；换 R 后可绑定。原串 `draft-duplicate discarded draft or hid refusal`。

第三批相关 **169 PASS / 0 FAIL**、包 vet 净。M3 撤去两条草稿保护后 **6 FAIL**，MD5 OK、touch 后 **6 PASS**。旧 V/B/S **0/0/0**，未改现有 normalize/保存接受列表的行为。
