# P13-hw：确认在列表刷新与忙态下的归属

开盘 ac1575a3，pull 已最新、工作树干净。15:04 UTC 开始。Codex 独立执行；不使用其他执行器。基线中文 locale 实测 **4135 PASS / 0 FAIL / 2896 函数**；最初继承 C.UTF-8 的运行不作为设计基线，保留原始输出。

## Stash 清点

本轮逐条导出 patch 并 reverse-check，以下 SHA 与上轮一致且均退出 1。已接续归档的 stale-kill 不在活动清单内；这八条为早期独立 WIP，沿用已审核处置，不把旧主题或 Tauri 链覆盖当前 Go 产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实施记录

仅已复现并修复的项目计入八项。旧 V/B/S 断言修改目前 0/0/0。

### 第一批：忙态不占用其他问题的到期唤醒

1. 现象：停用派发后等 4 秒，再打开配置丢弃，静止 3002ms 后丢弃仍开着。改动：忙态停用不参与未回答问题的 next 选择，操作仍由结果回调唤醒。验证：两主题真实 Click/Tab/Space + Advance（到期不强制 Frame），2 秒内保留、3 秒后退出，草稿 edited、保存 0 次；停用仍 busy 且迟到失败正常可读。
2. 现象：移除 busy 同样让后开丢弃丢掉唤醒。改动：移除忙态采用同族规则。验证：相同真实事件与深浅两场景，运行操作不取消、错误不丢失。

定向回归 **101 PASS / 0 FAIL**，包 vet 净。M1 回注旧分支，四场景实跑红：`later discard missed its deadline behind a busy confirmation`；恢复后 md5sum -c 通过，touch 后 **6 PASS**。旧 V/B/S 0/0/0。

第一批 53ee1228 已提交推送。

### 第二批：消失目标的停用/移除问题退场

3. 现象：已打开停用确认的集成从列表消失后，Escape 只关掉看不见的问题；同 ID 重新出现还带着旧问题。改动：View 在路由 Escape 前按当前安装列表退役不存在目标的停用问题、时钟和旧错误，不动 busy。验证：深浅 × 消失后 Escape/重新出现四场景，Close=1 或旧 Cancel 不复活，破坏性调用 0 次。
4. 现象：移除确认同样吞键并在刷新后复活。改动：按同一列表事实清理移除确认。验证：同上四场景，原移除入口仍存在，未派发操作。

相关回归 **111 PASS / 0 FAIL**，包 vet 净。M2 撤销清理入口后八场景红：`stale disable question revived after list refresh`、`invisible remove question swallowed Escape: close=0`（另外两种同族报错也在原日志）。MD5 恢复与 touch 后 **10 PASS**。旧 V/B/S 0/0/0。

第二批 fd376999 已提交推送。

### 第三批：消失目标的丢弃与卸载问题退场

5. 现象：配置丢弃随目标暂时消失而隐藏，却继续吞 Escape，目标回来时重新提问。改动：只 disarm 失效问题，保留 configOpen 与原 draft。验证：两主题 × Escape/返回四场景，问题退场、edited 草稿保留，不派发破坏操作。
6. 现象：组件卸载浮层失去目标后仍保留 open，目标回来时旧浮层盖住页面。改动：不存在目标的 uninstallOpen 退役，不替在飞卸载作完成或取消决定。验证：真实点击移除、Tab/Space 进入组件浮层，再让目标消失；四场景 Escape 正常关闭设置或返回时旧浮层不复活，卸载派发 0 次。

相关回归 **133 PASS / 0 FAIL**，包 vet 净。M3 回注旧的两项遗漏，八场景红：`stale discard question revived after list refresh`、`stale uninstall question revived after list refresh`、两类 `invisible … question swallowed Escape: close=0`。MD5 恢复与 touch 后 **10 PASS**。旧 V/B/S 0/0/0。

第一次目视：浅色 busy-disable 图中，过期的丢弃带已撤回，Token 的 edited 留在原字段，仍在执行的停用带单独留下。没有加材质或提高表面层数；待重排事件完成后再查深浅焦点局部。
