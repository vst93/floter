# P16 · 搜索响应与紧凑设置导航

已完成本轮修改。设置保持单一滚动面，没有固定大页头；集成页不再重复标题和说明。

| 范围 | 结果 | 验证 |
| --- | --- | --- |
| V：小窗布局 | 30 DIP 页内导航随正文滚动；概览行约 48 DIP；滚动条变细并向内留边 | 720×580 浅/深主题，`after/` 共 18 张截图 |
| V：搜索标签 | “最近启动”等分组使用半粗次级文字、对齐结果文字列、尾部细线 | `recent-*` / `super-*` 截图 |
| B：Super | 按查询、目录版本和最近应用缓存结果；两种修饰键状态复用组合列表；稳定页脚身份，无重复淡入；徽标切换即时结束过渡 | 缓存失效、连续切换、单一页脚、高度和既有 ActionBar 回归 |
| B：集成管理 | 我的工具（支持搜索）→ 单项详情；添加工具按自动发现/本地工具/安装包区分；导入导出独立入口 | 搜索、键盘导航、草稿、返回位置、外部高亮、卸载后返回、运行/诊断/安装回归 |
| S：格式与构建 | gofmt、diff check、darwin/linux/windows vet/build 通过 | `logs/` |

现有控件回归的夹具已改为进入相应详情或操作入口，业务断言保留。特别处理了单项移除后的自动返回：不会提前提交别名草稿；显式返回才提交待写入别名。Escape 依次取消确认、返回概览、关闭设置。

## 性能

| 基准 | 修改前分配 | 修改后分配 |
| --- | ---: | ---: |
| 20 个工具的集成页 | 253 KB / 629 次 | 33 KB / 150 次 |
| 500 应用的搜索静态帧 | 189 KB | 6.8 KB |
| 1000 应用连续 Super 切换 | 约 2 MB | 约 59 KB |

对应日志：`bench-before.log`、`bench-after.log`、`modifier-before.log`、`modifier-after.log`。集成页完整帧中位数约 2.57 ms → 1.00 ms。Super 的无头完整帧主要耗在框架的软件光栅绘制，耗时受并发验证影响较大；这里不以它推断原生 GPU 帧率。CPU 采样保存在 `logs/modifier.cpu`。普通路径仅缓存匹配结果，文件路径状态继续按既有 250 ms TTL 更新。

## 验证结果与限制

- `verified-ui.jsonl`：73 项（含子测试）通过，0 失败，覆盖本轮 UI 和搜索回归。
- `verified-regressions-final.jsonl`：36 项通过，0 失败，复核全量测试里本轮新增失败涉及的 14 个顶层测试。
- `shell-fixes.log`：诊断恢复、重扫、状态刷新回归通过；`nav-retained-scroll.log`、`transfer-navigation.log`、`shell-last-fixes.log` 均通过。
- 修改前全量：3595 项通过 / 313 项失败（101 个顶层失败）。本轮全量首遍收尾：3597 / 311（103 个顶层失败）；其中本轮引入的 14 个顶层失败均已修复并单独重跑通过。其余 89 个顶层失败与修改前重叠。没有把局部重跑冒充全套绿色。
- 全量仍在既有 `launcher.TestANoteRowsPlateDoesNotWearTheLitEdge` 与 `settingsui.TestTheRowFacePaintsItsPaneAndKeyline` 的像素越界处提前终止对应包。其后未运行的测试不计为通过。
- 未使用原生桌面窗口做 GPU 帧时测量；截图来自框架无头渲染。配置文件格式未变。

## Mutation

- 用修改前搜索实现覆盖：`TestModifierReusesSearchAndKeepsOneTerminalAction` 失败，原始报错 `initial query built 0 times, want 1`，见 `search-mutation.log`。
- 用旧全量详情渲染覆盖：导航测试失败，原始报错 `overview built command forms`，见 `navigation-mutation.log`。
- 修改前主要源文件压缩留档于 `baseline/`。全量原始日志与对照清单在 `logs/`。
