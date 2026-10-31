# P13-au 规格：设置页滚动体两端的边，穿的是哪一张面？

**意图一句话**：§6.16（P13-as）留了一句「仍未量」——设置页滚动体上下两道 `glass.ScrollEdge`
的 `Background` 都是 `t.Background`，而它们脚下是面板的 `Sunken` 面，**理论上**静止帧顶部那
16px 会有一道 ~15 级的淡痕。这一轮就是去量那道痕：**先量再改**，量完再决定改不改。

**证据（P13-as 仍待 2 的原话）**：「浅色 229 / 深色 39 对 `Background` 247 / 14」——
两组值差 15~25 级，若成立，边就在淡向另一张面。

## 逐条

| # | 位置 | 现状 | 本轮结论 |
|---|---|---|---|
| 1 | `internal/settingsui/settingsui.go` `body`（`ScrollEdge{Background: t.Background}` 两处，`settingsui.go:1504` / `:1516`） | 边穿窗口地，脚下是 `Sunken` 面板 | **不改**。实测那道「痕」在浅色是 **1 级**（247 → 246）、深色 **1 级**（14 → 15），远在 §1 的 8 级判据之下 |
| 2 | 那个「229 / 39」是哪来的 | P13-at 的 `runOutputEdgeFace` 里 `Sunken` 压在**扩展卡** `Control` 上 | `Sunken` 是「窗口地 + 一层**近白/近黑**的薄罩」（浅 `rgba(243,244,247,0.169)`、深 `rgba(17,18,20,0.169)`），压在窗口地上只挪 1 级；229/39 是它压在卡上的合成值，不是压在窗口地上的值 |
| 3 | 谁来守这个前提 | 无 | 新增 pin `internal/theme/sunkennear_test.go`：`theme.FaceOver(bg, Sunken)` 与 `bg` 每个通道相差 **≤2 级**，深浅两套。哪天 `Sunken` 变厚，这条先红，红在「边底下那张面已经不是边穿的那张」 |

**零像素改动**：本轮不改任何生产代码（`git status` 只有 docs + 一条测试）。

## 顺手量到的一件事（记进 §6.18，下一轮）

同族最后一块面——配置表单的多选列表箱（`config.go` 的 `configMultiselectBox`，66px 视口 /
56px 滚动区）——**两端都是平刀切**：实测（浅色，900×1400，Integrations 页，停在开头）视图
最后四行的墨展开 **111**，与最内一行（111）一样满（**100%**），框外一行立刻归 0；深色 86/86。
这一轮试着给它补 §6.17 那两道边，**卡在一个布局事实上**，见报告「仍待」。

## 门槛（P13-au 实测基线）

```
gofmt -l cmd internal                     # 必须空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 3076 PASS / 0 FAIL / 100 SKIP（基线）
go test -count=1 -v ./... | grep -c -- "--- PASS"
三平台 go build                            # 过
```

## mutation 计划

- **M1**：把 `Sunken` 换成**黑**薄罩 `ui.RGBA(0,0,0,0.17)` → 浅色 42 级、深色 3 级，pin 红。
- **M2**：换成**白**薄罩 `ui.RGBA(255,255,255,0.17)` → 深色 41 级，pin 红（浅色白压白仍绿）。

（抬 alpha 不算变异：薄罩的颜色就是地面色的邻居，α 0.5 也只挪 2 级——这正是本轮结论。）
