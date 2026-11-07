# P13-je：配置刷新后的动作有效性

开盘 353032fa，pull 已同步、工作树干净。基线中文 locale 5407 PASS / 0 FAIL，cmd/internal 3226 测试函数。done.log 不存在。

## Stash 清点

八条与上轮同 SHA，均非新增 stale-kill，reverse-check 全部退出 1。沿用已核验的历史处置，不把旧主题与 Tauri 工作覆盖进当前 Go 实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse exit 1 | 保留 |
| stash@{1} | 8af94a07 | 旧间距，reverse exit 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载指标/临时测试，reverse exit 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse exit 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse exit 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验，reverse exit 1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse exit 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse exit 1 | 保留 |

规格见 P13-je-spec.md；以下按批次追加实测证据。

## 第一批

1. 复制旧入口：现象 → provider 撤回 configuration 或停用后，实际按钮事件静默返回 nil。改动 → 在宿主读取当前配置，失效时原 toast 显示拒绝并回调错误。验证 → 原代码 `stale copy silently succeeded after missing/disabled refresh` 实红；无剪贴板副作用，恢复后可再次复制。
2. 导出旧入口：现象 → 同样刷新后无选择器、无任何结果。改动 → 同一当前配置校验给可见拒绝，解锁原按钮。验证 → 原代码 `stale export silently succeeded after missing/disabled refresh` 实红；路径选择器零次，恢复后可导出。

空 schema 原本被未知字段校验拒绝，保留回归但不计修复。夹具首次编译误用 FindText、随后误点内置插件 Configure，均已修正；只有目标集成的真实按钮事件失败计入产品证据。

第一批定向 32 PASS / 0 FAIL，shell vet 净，e98caabc 已推送。

## 第二批

3. 复制 ownership：现象 → provider 改由 tool 持有后，旧复制仍写剪贴板。改动 → 沿既有 tool-owner 语义在宿主拒绝，提示打开工具配置。验证 → `stale copy reached output: picker=0 clipboard=1` 原事件实红；修后不输出、不改草稿，恢复 host 后可重试。
4. 导出 ownership：现象 → 相同刷新后仍打开选择器、落文件。改动 → 同一宿主 owner 校验拦在选择器前。验证 → `stale export reached output: picker=1 clipboard=0` 原事件实红；修后选择器零次，恢复 host 后导出一次。
