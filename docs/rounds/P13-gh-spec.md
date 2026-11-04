# P13-gh · 卡里那些规则线占几行（规格）

**意图一句话**：给「一行整值 / 两行各半」这件事一把不依赖颜色的尺子，把设置页整帧的
**整幅宽分隔线**量一遍，证明「半行」只属于控件内线，不属于页面规则线。

**为什么现在做**（纪律第 4 档，通用队列「像素细节」第二格）：§6 差距表无 `未开始` 行；
用户实测反馈无新条目；合约「下轮」注脚全落；禁用族五道腿（材质 P13-s/t/u、指针 P13-fi、
一道淡、键盘 P13-gf、遍历 P13-gg）已收口。残下的都是「需拍板」。通用队列里
「间距节奏 / 状态反馈 / 空态满态 / 文案密度 / 边界情况」都有过专轮，**「像素细节」只有
§6.140（发丝线网格）一格**，而「半行」这个机制此前只有一句口耳相传的经验
（`docs/liquid-glass.md` §6.140 的 140-2：线被摊成两行时每行只剩一半强度），
着落在一个探针盒上，**整页从没量过**。

## 逐条落地项

位置 → 现状 → 应改成：

1. `internal/settingsui/halfrow_test.go`（新）→ 无此文件 → 新增量尺
   `classifyRow` / `scanHalfRows`：对每一行，用 `lum(y-1) - lum(y)` 与 `lum(y) - lum(y+1)`
   的**符号**分类——异号且两侧都 ≥4 级 = **一行整值的线**（极值）；同号、各 ≥2 级、
   和 ≥8 级 = **两行各半的斜坡**。整帧按列统计，`crispCols[y]` / `smearedCols[y]`。
2. 同上 → 无判据 → 三条 pin：
   - **153-1** 整幅宽分隔线正好 **9** 条，深浅落同一批 y；
   - **153-2** 每条两侧**逐值相等**、台阶 ∈ [40,100]；
   - **153-3** 线本身与上下两行 `smeared` < 10% 列宽。
3. 产品代码 → **零改动**（本轮是量尺轮，量到的是「本来就是对的」）。

## 动手前读到的真实行号（不是照抄旧报告）

- `internal/settingsui/rows.go:1553` / `:1713` / `:1774`：三处
  `ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline)`——页面规则线的全部来源。
- `internal/settingsui/controlslot_test.go:156`：`renderInAt(t, page, width, dark)`。
- `internal/settingsui/extensionrowface_test.go:137`：`imagePixel`；
  `internal/settingsui/litpair_census_test.go:41`：`litSum`；
  `internal/settingsui/counterside_test.go:37`：`realPageGround`。
- `internal/settingsui/litpair_census_test.go:43-47`：28u 控件的 1px 内线**跨两行**，
  一行本轮的既有事实（不是新发现）。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**一个字不碰**。
本轮只新增测试文件一个，产品代码字节零改动（`git diff --stat` 只有 docs 与 test）。

## 门槛命令 + 本轮实测基线

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净
GOOS=windows go vet ./...             # 净
GOOS=darwin  go vet ./...             # 净
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3481（上轮基线 3480）
三平台 go build ./...                 # 过
```

## mutation 计划（≥2 条，每条说明让哪个 pin 变红）

| # | 注入 | 预期 |
| --- | --- | --- |
| M1 | 分隔线 `Background(tokens.Hairline)` 后加 `.Opacity(0.5)`（墨减半） | **153-2** 红：台阶 63 → 33 |
| M2 | 分隔线整体挪半行：`.MarginX(inset)` → `.Margin(0.5, inset)` | 预期 **153-2** 红（两侧不再相等）——**实测绿**，见报告 |
| M2' | 线长高一行：`.Height(1)` → `.Height(2)` | **153-1** 红：9 → 5 条（且 y 整体位移） |
