# 终端、切换与浏览器发现

已完成：终端移除装饰边框、高光与外圈留白；可见窗口切换页面不重复 Show/Focus 或整页入场；发现扫描后台执行、30 秒缓存、打开配置主动刷新并保留旧结果；自动模式搜索全部配置。

路径覆盖：Windows/macOS/Linux Edge 稳定版及可用预览渠道；Chrome、Brave、Chromium、Vivaldi、Opera，macOS Arc/Safari，Firefox 原生与 profiles.ini；Linux XDG、常见 Flatpak 和 Snap。修复自定义单配置目录重复拼接。

第一轮针对性回归已通过（browser、plugincfg、shell）：平台路径、Edge 多配置、单配置目录、扫描后新增配置、配置读取不等待扫描、切换不重播入场。

追加完成：终端返回只同步变化的 font/theme/copy 设置，保留现有缩放，避免向所有会话重复发送重绘。新增回归与竞态检查全部通过（browser/shell/terminalui/plugincfg）。深浅终端 720×580 已目视，外圈玻璃、描边、高光与内侧留白均移除。

初次统一门槛：gofmt、linux/windows/darwin vet/build 通过；全量测试失败。两条新增代码契约错误已修复，复用项目 pathid/textlines 后针对性复验通过。

原始 HEAD `934b2c94` 通过 git archive 在独立目录、相同中文 locale 执行全量测试：102 个顶层失败。与初次修改版比较，仅新增上述两条已修正契约错误；原始 HEAD 另有一次 integrationfacts 异步时序失败。现有视觉/字号断言与当前 HEAD 实现已不一致，包含旧界面指标和像素采样越界，未借本轮更改断言来掩盖。

最终版本普通全量测试完成：101 个顶层失败，新增失败名 0，全部在原始 HEAD 复现。最终竞态检查四包通过；gofmt、linux/windows/darwin vet 和 build 全部通过。统一入口完成，普通/verbose 全量均因既有失败返回非零，不能称为全绿。verbose 记录 3481 PASS，测试函数总数 3734，failed_gates=2。两次全量的失败名均无超出基线的新增项。

本轮只更新明确改变的预期：终端外圈留白为零、可见切页不 Show/Focus、增加浏览器路径及环境缺失回退。其余旧视觉断言保持原样。验证结论是新增回归与三平台构建通过、已有视觉测试仍失败；不存在全量通过的声明。

证据：`P14-browser-surface/validation.txt`、`race-results.txt`、`baseline-test.log.gz`、`final-test.log.gz`、`final-verbose.log.gz`，以及深浅终端截图。Windows/macOS 在本机仅交叉构建与路径夹具验证，不等同原生实机验证。
