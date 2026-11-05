# P13-ha · 运行表单与输出的状态闭环

2026-11-05 续接抢救稿，起点 `35751144`，分支 `mygo-rewrite`。工作树干净，pull --ff-only 已是最新。沿用原轮号完成八项，不把前次中断当作已验收。当前环境 done.log 不存在，收尾创建。

## stash 清点

开盘十条 reverse-check 全部失败。两条新增 orphan 的 tracked tree 与第三父 untracked tree 分别完全相同；属于独有、未提交的首批两项，已用 `git apply -3` 接续代码及文档，恢复第三父中的测试和 PNG，重新实测。原报告中未经本次复核的计数不沿用。

| 开盘 ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / 92315415 | P13-ha 首批两项独有工作 | 接续，门槛后单独提交；提交后核对并 drop |
| stash@{1} / ff170912 | 与上一条代码及第三父完全相同 | 不重复套用，核对覆盖后 drop |
| stash@{2} / 422483b4 | P7 keycap/旧主题 WIP | 沿用 P13-gq/gz 语义审核，保留原件 |
| stash@{3} / 8af94a07 | 行距/边缘旧刻度草稿 | 后续已有正式刻度，保留原件 |
| stash@{4} / 7e4aa710 | 卸载布局与 scratch 探针 | 保留原件 |
| stash@{5} / 1e3aec94 | 旧数字框/透镜草稿 | 保留原件 |
| stash@{6} / 363f5a42 | 旧确认带/快捷键草稿 | 保留原件 |
| stash@{7} / 9b8ce23c | P7 快捷键拒绝反馈草稿 | 保留原件 |
| stash@{8} / 90d314b2 | 旧 action bar，与现有键盘合约不同代 | 保留原件 |
| stash@{9} / 7a9f6e13 | 旧 Tauri release workflow | 保留原件，不恢复旧发布链 |

## 已完成（逐条，增量记录）

1. **运行按钮漏锁**：展开后复检、禁用、移除、卸载、开关写入中，或 stopped/broken/orphan 时，内层 Run 仍派发运行 → 同时检查已有 rowBusy 与运行可用状态，建造时禁用 → 两主题 × 八状态拒绝派发，解除后恰好增加一次。旧实现实跑：`unavailable repair form dispatched 1 runs`。
2. **文字参数漏锁**：文本、数字、路径参数在同行复检中仍获焦并写入 `123` → `!locked` 建材质，根 disabled、父行 Inert，保留草稿 → 两主题 × 三类型拒绝编辑，解除后恢复。旧实现实跑：`busy text input accepted editing: value="123" focused=true`。

上述红测为本次重新执行，日志 `/tmp/floter-P13-ha-rescue-before.log`。首批定向与 mutation 恢复后均 **65 PASS / 0 FAIL**；全量基线 **3546 PASS / 0 FAIL**，测试函数 **2705**。接续版本全量 settingsui 通过（313.312s）；其余包中 shell 的 `TestARescanAsksForAFreshDetectedList` 曾报 `the frame after a rescan started no pass`，单项重复 10 次也出现一次；它直接观察异步任务的瞬时 pending，未改其断言，shell 包整体复跑通过。三平台 vet/build 通过，gofmt 空。最终全量将覆盖全部八项。

正式 M1 把 runDisabled 恢复成只看 issues，16 场景红；正式 M2 将行锁读数替换为 false，6 场景红。原始串分别同上两项；每次恢复 `md5sum -c` 均 OK（settingsui.go `a4eec1370b22cb5622a559175292a0e5`），touch 后非缓存回归 65 PASS。V/B/S 台账 **0/0/0**，未改旧断言。

3. **布尔参数漏锁**：忙态点开关或标题仍把 false 改成 true，父行浅色 246→220、深色 15→49 → 两入口遵守 rowBusy，控件根禁用、父行 Inert → 深浅 × 两入口保持 false 和静息像素；解锁后一次点击变 true。原始红测：`busy boolean changed through switch` / `busy boolean changed through label`。
4. **分段参数漏锁**：忙态点击/方向键仍可把 fast 改为 careful，父行也抬升 → 整个分段根禁用，写值与父行都遵守行锁 → 两主题拒绝点击/方向键，解除后正常选择 careful。原始红测：`busy select changed to "careful"`。

第二批相关回归 **73 PASS / 0 FAIL**（`choice-after.log`）。所有旧断言零改动。首批提交 `c6d0d6d6` 已推送；两个相同 orphan 的产品补丁 reverse-check 通过，已创建 `refs/archive/floter/P13-ha/<完整 SHA>` 保存原始报告/图片/测试，再按开盘 ref 1、0 顺序 drop；活动清单回到八条早期 WIP。

## 设计与验证计划

读取为：面向桌面工具用户的原生参数表单与输出凹槽，轻便液态玻璃。拨盘 2/2/6，保留高度链、键盘、会话、写回与 i18n。三秒内能分清可操作项、错误所属表单和最新输出的开头。

为什么更轻、更有空间感：不可操作的面退回静息层，只有可操作控件承担受光反馈；输出沿用现有凹槽，不叠加玻璃或阴影。

已实际读取 qiaomu-design / user-preferences / craft-loop / preflight / chinese-typography，以及 Apple 供体的颜色、字阶、按钮章节。实际借用值、逐项 preflight、图片与正式 mutation 在后续批次补齐；未完成项不预记 PASS。
