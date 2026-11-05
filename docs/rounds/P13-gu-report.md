# P13-gu 报告 · 插件配置浮层的通用 schema 边界 + 设置页下拉的空 ID

起点 `296f549a`（P13-gt），pull 已同步，开盘工作树为空。执行者 pi（provider `magpie` /
`group/dsv41`，thinking high）。规格 `docs/rounds/P13-gu-spec.md`，合约新增
**§6.163–§6.168**。

## 选题

临时主线（2026-11-05）：「不开新系统，把已有功能的问题修干净」。`docs/liquid-glass.md`
§6 无 `未开始` 行、用户实测无新条目、上轮「仍待」全是拍板项。于是从**通用队列**里挑了
一条同族线：**「一格全局状态被 N 个对象轮流用」**（P13-gs 的数字草稿是同一个形状），
顺着插件配置浮层这块「第三方输入」的面一路量下去，量出 7 条，再加设置页下拉的 1 条。
跨 **2 个组件**：插件配置浮层（`settingsui/pluginsheet.go`）与设置页下拉入口
（`settingsui/settingsui.go` 的 `choose`）。

## 八项（现象 → 改动 → 验证）

1. **换插件沿用上一张面的滚动位置** → `PluginSheet` 记住 `sheetPlugin`，插件 id 变了把
   `PluginScroll.Y` 归零 → 改前实测插件 B 开在 `Y=609.8`（页头被切在带子上沿），改后 `0`
   且同一插件的重绘仍保住用户滚到的位置（`TestThePluginSheetForgetsTheScrollWhenThePluginChanges`）。
2. **0.1 步长的滑块读数抹成整数** → `boundedText` 改用最短小数写法 → 改前
   `boundedText({Step:0.1}, 0.4) = "0"`，改后 `"0.4"`（真滑块帧：改前读 `0`、改后读 `0.4`）。
3. **读数取未量化的拖拽值** → 先 `quantize(v, Step)` 再打印，六位小数收口 → 改前
   `{Step:10, Unit:"items"}, 12.7 = "13 items"`，改后 `"10 items"`（= 会被保存的数）；
   整数值逐字节不变（`30 days` / `300`）。
4. **空标签的字段画空标签列** → `sheetField` 入口把副本的 `Label` 回落成 `Key` → 改前
   `Texts() = [plug.l  items  ]`（标签列是空的），改后 `[plug.l size items …]`。
5. **空标签的 Action 是零宽空药丸** → 同一条回落一起接住 → 改前 `Texts()` 里没有
   `clear_history`，改后胶囊上就是它。
6. **空标题仍占 25.2px 空表头** → 标题为空就不画那一行 → 改前首行墨 `Y=48.7`（有无标题同值），
   改后上移 **36.0px**（= 表头行 `Space(7)` 25.2 + 列 `Gap(Space(3))` 10.8；实测首行墨 `48.7 → 12.7`）。
7. **越界数字：框与值不同源** → 夹后的数写回草稿 → 改前 `pluginNumberDrafts["plug.n\x00days"]`
   停在 `"999"`、插件收到 100，改后同帧是 `"100"`；界内的半输入 `1e` 仍留在框里（§6.162 不倒退）。
8. **空 ID 的下拉选项派发零次** → `choose` 用 `picked bool` 取代 `chosen != ""` → 改前
   选「off」实测 `applied=[]`，改后 `[""]`（一次）。

## 台账

**V 0 / B 0 / S 0**：没有一条既有断言被改、没有一条容差被放宽、没有 `t.Skip`。
（163-2 记一条「故意不做」：把 `PluginScroll` 拆成每插件一格会让六个既有量尺改成读 map，
那是结构真变，换不来一个像素。）

## 门槛（实测）

- `gofmt -l cmd internal`：**空**。
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...`：**净**。
- `go test -count=1 ./...`：**exit 0**。
- `go test -count=1 -v ./... | grep -c -- "--- PASS"`：**3514**（基线 3490，+24；本轮新增 6 个测试函数）。
- `--- FAIL` 计数：**0**。测试函数总数 **2679**（基线 2673）。
- 三平台 `go build ./...`：**过**（`BUILD_OK`）。
- `internal/settingsui` 单包全绿（292s）。

## 变异（六条实跑，先红后绿）

先在**未改的产品代码**上跑本轮六个 pin（`git stash push -- pluginsheet.go settingsui.go`），
六条全部红，原始失败串：

| # | 注入缺陷 | 原始失败串（节录） |
|---|---|---|
| M1 | 去掉换插件归零 | `plugin B opened at plugin A's offset: Y=609.79987` |
| M2 | `boundedText` 退回 `int(Round)` | `fraction: boundedText(0.1, 0.4) = "0", want "0.4"` / `a 0.1-step slider read [plug.s ratio 0], want 0.4` |
| M3 | 去掉 `Label` 回落 | `a field with no label drew an empty label column: [plug.l  items  ]` |
| M4 | 表头无条件画 | `an untitled schema did not save the header row: titled=48.699997 untitled=48.699997` |
| M5 | 去掉夹后写回 | `the box still shows the number the plugin never got: draft="999"` |
| M6 | `picked` 退回字符串守卫 | `choosing the empty-ID option dispatched [], want one empty id` |

还原（`git stash pop`）后同一组命令全绿，`git diff --stat` 只剩本轮两文件。

## 出图

本轮的八项里只有 166-1 有像素位移（首行墨上移 36.0px），其余是状态/读数/接线；按纪律
「只有测试数字没有图 = 不算完成」的例外只适用于纯状态轮，这里给出 166-1 的两张整页
dump（`/tmp/floter-P13-gu-untitled-{light,dark}.png`，由
`TestDumpP13GuUntitled` 在 `FLOTER_P13GU_DUMP=1` 时导出）。图上看到的一处变化：无标题的
浮层顶上不再有一条 25.2px 的空白带，第一组标题直接坐在 scroller 的 2u 顶留白上。

## 仍待

1. 上轮与更早的拍板项全部沿用（`140-5`、138-3 / 138-4、§6.111 / 113-5 / 114-4 / 129-5 /
   151-4、字号账、36px 行盒、终端弧 S1b / S1c / S2 尾）。
2. **新发现 1 条需拍板**：`ui.SegmentedBase` 把 `*selected` 夹到 `[0, n-1]`（框架
   `ui/toggle.go:124-126`），所以「单选字段的值不在选项表里」时**无法**画出「一个都没选中」——
   控件必然把第 0 格画成选中。要改就得改框架控件或换一种表示（例如加一格「未设置」），
   本轮不动。
3. 下一格首选：同一族（「一格状态被 N 个对象轮流用」）在 `settingsui` 之外还剩
   `launcher` 的 `a.config`（换插件时由 `ConfigFor` 换整份，已核对无串味）与
   `shell` 的插件窗口（`detach.go` / `pinned.go` 的滚动状态是否共享，未量）。
