# P13-cw 规格：集成卡的健康块——它挂在卡框上，而卡上每一行都缩进 8px

选题：通用队列 `空间感`（P13-cv「仍待」第 1 条的后续）。上一轮量的是**行内标点**；
这一轮量的是**块的列**——同一张集成卡上，四个块的标签各自站在哪条竖线上。

## 意图

一句话：**把健康块从卡框上收回卡的行列里，并把两条暖色重棱挪回它们介绍文字的那一侧。**

为什么现在做：这是同一张卡上唯一还挂在卡边框上的块。用户说的「所有东西挤在同一个 z 平面上、
靠 1px 描边分家」，在这块上有一个更具体的形式——**内容贴着卡自己的框走**，于是块与卡之间
没有空气，块读起来是卡的一部分，而不是 Health 那一行的细节。

## 实测（Integrations 页，720×1700，深浅两套逐像素同坐标）

| 事实 | 浅色 | 深色 |
|---|---|---|
| 卡面横向范围 | `192..700`（框线 1px 在 `191` / `701`） | 同 |
| 行标签 `Health` / `Approval record` / `Diagnostics` 左边界 | **x=200** | 同 |
| 健康块 `Checks` / `Failures` / `Checked …` 左边界 | **x=192** | 同 |
| 探针时长 `12ms` 右边界 | **x=701**（卡框线） | 同 |
| 失败卡横向范围 | `192..700`（占满卡的整个内容盒） | 同 |
| 失败卡的 2px 警示棱 | **`699..700`（尾边）** | `699..700` |
| 失败卡的 1px 警示 keyline | `192`（前导边，1px） | 同 |
| `Checked 2026-10-12 17:30` 左边界 | x=192 | 同 |

两处偏差，同一个根：

1. **整块挂在卡框上**：`healthDetail` 的列没有任何水平内缩，所以它的标签、探针、失败卡、
   时间戳都从卡面第一像素开始（192），比卡上每一个行标签（200）左 8px；右端同理，探针的
   时长一路贴到卡框（701）。`cardRow` 给「属于这一行的内容」留的列是 `rowInset`
   （集成卡 = `Space(2)` = 8，`settingsui.go:2649` 的 `cardGeometry`），`healthDetail`
   是 `a.row(copy.IntegrationsHealth, …)` 之后**平级**画出来的，从来没进过那一列
   ——这正是 P11-bf 给其它块修过的同一个 bug，`healthDetail` 是漏网的那一个。
2. **2px 暖棱画在尾边**：`healthFailureCard` 与 `approvalStaleBanner` 都写
   `InsetShadow(-2, 0, 0, 0, t.Warning)`。本仓框架里**负偏移把未覆盖的一条留在对侧**
   （`mygo/ui/shadow_test.go` 的 `InsetShadow(0, 6, 0, 0)` 案例：正 y 只剩顶部；`paint.go:1254`
   的 `hole.X = pad.X + sh.x*s + spread`），所以 `-2` 画的是**右边** 2px。参考写的是
   `border-left: 2px solid var(--text-warning)`
   （`floter-main:src/styles/extensions.css:2200` 失败卡、`:2171-2179` 陈旧横幅），
   也就是**前导边**。今天那条更重的线站在它不介绍的那一侧。

## 逐条落地项

位置均按本轮实测行号（动手前已核）。

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/settingsui.go` `healthDetail` 的 `block`（`ui.Column(c).FillWidth().Gap(t.Space(2))`） | 无水平内缩，块左边界 = 卡面第一像素（192） | `.Padding(0, a.rowInset(t))`：块进入卡的行列（200 / 右端 693），纵向节奏**一个像素都不动**（P13-cv 刚定完的线距保留） |
| 2 | `internal/settingsui/healthfaces.go` `healthFailureCard` 的 `card.InsetShadow(-2, 0, 0, 0, t.Warning)` | 2px 暖棱在 `699..700`（尾边） | `InsetShadow(2, 0, 0, 0, t.Warning)`：2px 在前导边（`192..193`），与参考的 `border-left` 同侧 |
| 3 | `internal/settingsui/healthfaces.go` `approvalStaleBanner` 的 `banner.InsetShadow(-2, 0, 0, 0, t.Warning)` | 同上（同一族、同一张卡） | 同上 |

**不动**：纵向间距（`Gap(t.Space(2))` 组间 8px、组内 `Space(1)` 4px、探针行 `Padding(Space(1),0)`）、
墨、字阶、字重、行盒、失败卡的 `Padding(Space(1.5))` 与圆角、i18n 文案、键盘模型。

## 行为契约零改动红线

只改墨与面/列：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线全部零改动。
本轮没有交互面，也没有新文案。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

PASS 基线 **3294** / FAIL 0 / SKIP 140 / 测试函数 **2446**（P13-cv 收尾实测值）。

## mutation 计划（≥2 条）

1. **M1**：把 `Padding(0, a.rowInset(t))` 改成 `Padding(0, 0)`（恢复旧列）→ 列 pin 必红
   （断言 `Checks.X == Health.X` 与块右端内缩）。
2. **M2**：`InsetShadow(2, …)` 改回 `-2`（棱回尾边）→ 棱 pin 必红（尾边 2px / 前导边 1px）。
3. **M3**（可选，过冲臂）：`Padding(0, a.rowInset(t)*2)` → 列 pin 红（不是越宽越好）。

## 视觉产出

`ui.Tester` 导出 `/tmp/floter-P13-cw-healthblock-{light,dark}-{before,after}.png`，报告写
「这张图上看到的一处变化」+ 差分像素数/bbox。
