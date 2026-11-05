# P13-gu 规格 · 插件配置浮层的通用 schema 边界 + 设置页下拉的空 ID

起点 `296f549a`（P13-gt），工作树为空。执行者 pi。

## 意图

插件配置浮层是**第三方输入**的渲染器：`plugincfg.Schema` 由插件声明，空标签、空标题、
0.1 的步长、越界的旧值都合法。本轮把这些边界一次读进帧，并顺手收掉设置页下拉入口上
「ID 为空串的选项永远选不中」这条守卫。**不新增面、不新增材质、不改任何行为契约**
（高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线全部零改动）。

## 逐条（现象 → 应改成）

1. `PluginSheet` 的滚动位置跨插件串味：`a.PluginScroll` 是 App 上一格、`ui.Scroll` 的 key 是常量。
   → 插件 id 变了归零（同一插件的重绘不动）。
2. `boundedText` 把 `Step < 1` 的分数抹成整数（0.4 → "0"）。→ 最短小数写法。
3. `boundedText` 取未量化值（Step 10 拖到 12.7 写 "13"，存 10）。→ 先 `quantize` 再打印。
4. `sheetField` 空 `Label` 画空标签列。→ 回落到 `Key`（集成配置表单同规则）。
5. `sheetAction` 空 `Label` 是零宽空药丸。→ 同上。
6. 空 `Title` 仍画 25.2px 空表头。→ 不画。
7. `sheetNumberSlot` 越界时框里留原文、插件收夹后的值。→ 夹后的数写回草稿。
8. `choose` 的 `chosen != ""` 让空 ID 选项派发零次。→ `picked bool`。

## 门槛（实测）

- 基线：`go test -count=1 ./...` exit 0；PASS **3490**（P13-gq 报告值）/ 函数 **2673**。
- 本轮：PASS **3514** / 0 FAIL / 函数 **2679**；`gofmt -l cmd internal` 空；
  `go vet ./...` + `GOOS=windows` + `GOOS=darwin` 净；三平台 `go build ./...` 过。

## mutation 计划

- M1 把 `sheetPlugin` 的归零删掉 → 163-1 红（B 开在 A 的偏移上）。
- M2 `boundedText` 退回 `int(math.Round(v))` → 164-1/164-2 四条字面量 + 真滑块帧红。
- M3 去掉 `Label` 回落 → 165-1/165-2 红（`Texts()` 里没有 key）。
- M4 表头行改成无条件画 → 166-1 红（首行墨不动）。
- M5 去掉夹后写回 → 167-1 红（draft 停在 "999"）。
- M6 `picked` 退回 `chosen != ""` → 168-1 红（`applied=[]`）。
