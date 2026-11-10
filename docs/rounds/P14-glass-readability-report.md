# P14 · 透明材质与文字可读性报告

最终范围已落地：主界面 72–100%（72 / 86 / 100），终端 66–100%（66 / 83 / 100）。滑块直接设置实际背景不透明度，去掉重复的有效值读数。高对比度模式为 86–100%（86 / 93 / 100）。方向键可以逐 1% 调整，拖近推荐点自动吸附，也可点击推荐点。

| Before | After | Why |
| --- | --- | --- |
| 低值时固定灰字被桌面吞没 | 字色按背景不透明度和控件状态适配 | 黑白底均检查，避免深浅主题互相污染 |
| 终端与应用背景混色 | 框和画布采用同一终端底色 | 允许 66% 下限；工具栏与抽屉单独加强底色 |
| 100% 仍是 98%，继续开玻璃效果 | 完全不透明，关闭原生模糊和控件玻璃 | 减少不可见背景的工作；降低数值/切换页面时恢复 |
| 透明度变动时文字仍过渡 | 字色与底色同帧更新 | 拖动时避免短暂失配 |

测量（最差黑/白背景、含 macOS frosted 补偿）：主界面 70% 最差 4.47:1，71% 开始通过；选择 72% 作为带 1 个百分点余量的下限（最低对比度 4.51:1）。终端 64% 即使纯黑/白文字也只能达到 4.30:1，65% 为 4.44:1，66% 可达 4.56:1。终端默认文字/光标做最小颜色调整，ANSI/真彩色输出由程序自行控制，选区有独立底色。测量见 [日志](P14-glass-readability/logs/opacity-study.log)。

已通过：主题/透明度模型全包测试；终端所有内置配色默认文字、光标、选区、抽屉与工具栏；100% 内部像素完全不透明；原生效果随主/终端页面开关；已有终端会话更新颜色而不重置字号。
最终验证：

| 检查 | 结果与证据 |
| --- | --- |
| 主题、透明度模型、设置全包 | 通过：[核心测试](P14-glass-readability/logs/verified-core.log) |
| 100% 像素、材质恢复、主/终端独立、实时主题、字色、滑块点击/吸附/键盘 | 14 项专项通过：[UI 测试](P14-glass-readability/logs/verified-ui.log)、[主/终端普通与高对比度四组操作](P14-glass-readability/logs/final-slider.log) |
| 推荐按钮设计约束 | 通过：[约束测试](P14-glass-readability/logs/final-contract.log) |
| gofmt / diff check | 通过 |
| darwin / linux / windows vet 与 build | 均通过：`P14-glass-readability/logs/final-{vet,build}-*.log` |
| mygo v0.8.3 vet | 通过：[日志](P14-glass-readability/logs/final-mygo-vet.log) |
| 修改前/最终截图 | 36 张修改前、48 张最终图；检查了极端背景下文字、按钮和 100% 背景。[最终对照](P14-glass-readability/final/comparison.png)；`after/` 是中间方案，最终以 `final/` 为准 |

全量测试记录 2037 项通过 / 103 项失败（顶层测试口径）。与上轮 2022 / 107 的原始日志相比，新增的 2 项是推荐按钮缺少细边的设计约束，已补上并专项复验通过；其余 101 项在上轮失败集合内。[比较台账](P14-glass-readability/logs/full-comparison.json)。launcher/settingsui 的既有像素索引 panic 仍会提前结束对应包，因此不宣称全套通过，也不把未执行项算作通过。

删除了已经下线的“有效不透明度”读数及其专用测试夹具。补充运行通用滑块间距测试时，`TestASlidersAxisWordsHugTheirTrack` 仍失败（该项被全量测试的早期 panic 挡住）：旧通用滑块的轴词离轨道 14px，而提示离轨道 13px。[补充日志](P14-glass-readability/logs/verified-slider.log)。实际不透明度滑块使用三枚推荐按钮，其范围、操作与截图已单独检查。

三条反向验证均捕获故障：[结果](P14-glass-readability/logs/final-mutation-results.json)。移除下限报 `readability floor = 0, want 0.72`；恢复固定墨色报 `1.32:1`；保留 100% 玻璃效果报 `100% keeps a background effect`。使用临时 overlay，未修改产品文件。

设置格式保持不变，旧低值按对应下限显示/渲染，未知键保留，主/终端默认均为 86%。图片为无头生产帧的黑白底合成，不模拟系统折射；模型中的 frosted veil 是估计，不宣称能精确测量每个系统版本的光学效果。100% 已去掉背景模糊开销，但没有整机 GPU/功耗基准，不能声称总开销绝对最低。
