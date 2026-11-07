# P13-jd：传输反馈与结果来源

起点 1a5d808e。开盘工作区干净、pull 已同步；实际基线 5381 PASS / 0 FAIL，3218 测试函数。done.log 开盘不存在。

## Stash 清点

下列八条与 P13-jc 和抢救 backlog 的早期 WIP 完全同 SHA；反向检查均退出 1。不是新增 stale-kill；历史语义核对为已替代的旧主题/字段/Tauri 工作，逐条保留，不强行覆盖当前产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse exit 1 | 保留 |
| stash@{1} | 8af94a07 | 旧间距，reverse exit 1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载指标/临时测试，reverse exit 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse exit 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse exit 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验，reverse exit 1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse exit 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse exit 1 | 保留 |

正常宿主每次只调用一次完成，未复现旧回调误清新锁；不为此引入代数或改关闭表单语义。实际选择传输结果与错误出口，详见规格。

## 第一批

1. 配置复制完成：现象 → 已报成功但剪贴板未写入，且绕过已有写入接口；旧代码探针报 `configuration copy bypassed the configured clipboard backend`、`configuration copy announced success before clipboard delivery`。改动 → 走原 writeClipboard，完成后才发成功与 done。验证 → 阻塞注入后端期间无成功、无完成；释放后成功，文本含 REDACTED、不含测试密钥。
2. 配置复制校验：现象 → 缺必填 endpoint 的草稿被误报为“不是集成导出文件”。改动 → toast 显示原校验器错误，不改字典。验证 → 深浅两帧均可读校验原因，失败不写剪贴板，回调错误等于校验器原错误。

第一批首次相关实际 5 PASS / 1 FAIL，shell vet 退出 0。原复制测试直连系统剪贴板，无桌面事件循环，完成移到写入后旧固定等待不再适用；给共享 provider 夹具接上已有 WriteClipboard 注入，所有旧行为断言保留。提交脚本未向 commit 传播测试退出码，02e87cbf 被提前提交推送，提交消息的“6 PASS”是错误记录；此处明确纠正，不冒充首次绿。后续所有提交使用 set -e 门禁。其余探针保留为非 Test helper，随各批启用，未将未修项算完成。

首批夹具修正后实际 6 PASS / 0 FAIL、shell vet 净，单独提交推送。

## 第二批

3. 配置导出校验：现象 → 缺必填 endpoint 的草稿误报导入文件格式。改动 → 导出校验失败展示实际字段错误。验证 → 两套外观中字段原因可读，回调携带原错误，文件选择器调用 0 次。
4. 配置导出选择器：现象 → 系统选择器不可用只留日志，用户看不到结果。改动 → 原错误进入宿主 toast；空路径且无错误仍为取消。验证 → 错误深浅可见、done 原样返回错误；再次执行空路径取消通知 0 条。

第二批累计相关 13 PASS / 0 FAIL，shell vet 净；旧断言 V/B/S 0/0/0，i18n 字典零改。

## 第三批

5. 集成导出选择器：现象 → 对话框系统错误被当成取消，无反馈。改动 → err 非空显示原错误；正常空路径单独返回。验证 → 深浅实帧可读系统原因，忙态清空，下一次取消不报错；原单任务 guard 和文件输出回归通过。
6. 集成导入选择器：现象 → 入口失败后直接消失，用户不知道为何没有导入。改动 → 同样区分错误和取消，仍走唯一宿主 toast。验证 → 深浅可见错误，忙态释放后可再次打开，正常取消 0 条新通知，导入原回归通过。

第三批累计相关 21 PASS / 0 FAIL，shell vet 净；旧 V/B/S 0/0/0。
