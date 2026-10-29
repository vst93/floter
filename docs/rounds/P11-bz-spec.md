# P11-bz 规格：第二把尺子——字盒 / 行盒的高度阶梯

选题顺序：用户实测反馈 F1–F4 已收完 → `docs/raycast-design.md` §6 全 `已落地`（只剩拍板项）
→ P11-by「仍待」第 1 条整张是拍板项、第 2 条是**上一轮实测产物**：

> 「字号暴涨 / 行距失控」这类回归**不会**被字压字尺子看见（重排 + 裁剪后的盒子），
> 要单独一条**字盒高度 / 行距**的臂。P11-bx M4 的旧归因已作废，别按它再找「整宽横幅」。

## 1. 意图

**给「一帧画出了哪几档高度」补一条尺子**：字号被乘大、行高从 24 掉到 20、某个控件换了数、
某一块字被顶出视口不再画——这些都不产生「两段字压在一起」，但都会让**高度的集合**变化。
本轮把这条规则写进 `internal/frameaudit`（第一条尺子所在的包），并在两个表面上钉住阶梯。

为什么现在做：P11-bx 的 M4（拒绝句 `FontSize(t.FontSize)` → `× 6`）在 `frameaudit.Audit`
下**全绿**，P11-by §2 纠正了旧归因（不是整宽横幅，是会重排的列），并把这条列进仍待。这是
上一轮自己列的第 2 条，不需要拍板。

## 2. 逐条落地项

| 位置 | 现状 | 应改成 |
| --- | --- | --- |
| `internal/frameaudit/` | 只有 `Audit`/`Violations`（字压字 + 出窗） | 新增 `heights.go`：`Heights`（叶子 run 的**去重**高度，升序，四舍到 0.1u）、`Ladder`（不在阶梯上就报）、`Missing`（该出现的档位没画就报）、`AuditHeights`（三者的接线）、`MinHeight = 10`（裁剪碎片的默认下限） |
| `internal/launcher/` | 只有 `textframe_test.go`（字压字） | 新增 `textheight_test.go`：三页 × 2 语言 × 2 外观，钉住六档 `15/18/22/24/25/28` 与每页的 `must` 子集 |
| `internal/settingsui/` | 只有 `textframe_test.go`（字压字） | 新增 `textheight_test.go`：五页 × 2 语言 × 2 外观 × 2 尺寸进同一张阶梯 `15/17/18/19/27/30/30.6/34`（下限 10u 挡裁剪碎片），外加 General 页的 `must` 覆盖臂 |
| `internal/frameaudit/heights_test.go` | 无 | 4 条用手工高度问规则本身：阶梯外必报 / 半格容差边界 / 缺档必报 / 容器盒不入表 / 重复高度只算一档 |

**产品代码零改动**（`git diff --stat` 只有新文件与 docs）：这是一条新守卫，不是一处修复。

## 3. 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一个字都不动**。本轮新增的全是
`_test.go` 与 `internal/frameaudit/heights.go`（只被测试引用）。

## 4. 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 基线 2601（P11-by），只许涨
GOOS=linux/windows/darwin go build ./...               # 过
```

## 5. mutation 计划（≥2 条，实跑变红并还原）

- **M1（阶梯外的高度）**：设置页拒绝句 `FontSize(t.FontSize)` → `× 6`（P11-bx M4 的原样
  复现）。预期 `TestTheSettingsPagesStayOnTheHeightLadder/*general*` 变红：那一行的高度离开
  阶梯。这是本轮存在的理由——同一条变异在 `frameaudit.Audit` 下仍是绿的。
- **M2（行高被改）**：启动器结果行的行高（`22` 那一档）改一个数，预期
  `TestTheLauncherHeightsStayOnTheLadder/*match*` 红。
- **M3（某一档没画）**：让启动器空页不再画最近启动的行（`must` 里的 `22` 缺席），预期
  `Missing` 红——这条守的是「字被顶出去 / 整块没画」。
- **M4（弱变异，预期绿）**：字号 ±5%，四舍五入后仍落在同一档 ⇒ 绿。写清这是本尺子的
  分辨力边界，别当缺陷。
