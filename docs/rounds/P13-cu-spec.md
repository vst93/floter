# P13-cu 规格 · 健康页的「可重试」小块：它的面是父面的同一桶漆叠两次

## 意图一句话

量清楚健康页失败卡里那枚 `--retryable` 小块的**面档**，把「这块面落在阶梯上的哪一格」
变成一条钉住的规矩——量下来的答案是「它有意留在阶梯之下」，所以本轮**不改像素、只立规矩**。

为什么现在做（选题来源）：任务书「下一格从通用队列里挑，先量再改」；通用队列里
`状态反馈` 的禁用/只读面一族已收官（P13-ct 是最后一条），`空态满态` 已被 §6.13/§6.16/
§6.23/§6.47/§6.50/§6.51 逐个表面清完。这一轮落在**空间感（elevation）**这一轴上，挑的是
`internal/settingsui/healthfaces.go` 的 `healthFailureCard`——**全仓最后一个暖面，一个像素
pin 都没有**（`facecensus_test.go` 只把它列进 allowlist，从没量过它的像素）。

## 证据（先量，实测）

平铺 harness（照 `disabledfade_dump_test.go` 的写法）：`realPageGround(t, dark)` 取真设置页
地面 → 同一块地面上画 `a.healthFailureCard(c, a.copy(), HealthProbe{ID:"probe.git", ExitCode:1,
Retryable:true})` → 采样三点（地面 / 卡内空白 / 小块内空白）。1 亮度级 = 3（rgb 和）。

| 采样 | 浅色 | 深色 |
|---|---|---|
| 地面 | `{247 247 248}` | `{14 14 16}` |
| 卡面 | `{243 230 222}` = −15.7 级 | `{41 32 29}` = +19.3 级 |
| 小块面 | `{240 215 199}` = **−13.7 级**（对卡） | `{65 48 41}` = **+17.3 级**（对卡） |
| `RaisedWarm` 叠两次（预测） | −13.7 | +17.3 |
| `Control`（预测） | −16.3 | +22.7 |
| `IconSurfaceHover`（预测） | −22.7 | +31.0 |
| `ControlHover`（预测） | −24.7 | +31.0 |

**三条事实**：

1. 小块的面 = **同一桶漆叠两次**（0.169 / 0.197），不是任何一个命名档；实测与预测逐级吻合。
2. 它的面差是卡自己抬升的 **87% / 90%**——一个 4px 高的词，面差几乎等于一整张卡。
3. 阶梯上所有**命名**档（`Control` / `IconSurfaceHover` / `ControlHover`）都比现状**更响**
   （16…31 级）——也就是「给它一个名字」这条路会把 4px 的字做得比它的父卡还重。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/healthchip_test.go`（新增 `TestTheRetryableChipPaintsTheCardPlaneTwice`） | 无 pin | ① 小块的面 == `panelOver(RaisedWarm, 卡面)`（容差 2）② 与 `Control` / `IconSurfaceHover` 两个候选人**判成不同** ③ 小块的面差 ≤ 卡的面差（嵌在卡里的东西不许比卡响）④ 面差数值 == 预测值（±2）⑤ 深浅各一遍 |
| 2 | 同上（新增 `TestTheHealthStatusChipFaces`） | `healthStatusFace` **零测试** | healthy → `Control`/`ControlEdge`/`TextStrong`；degraded+unhealthy → `RaisedWarm`/`ControlEdge`/`Warning`；unknown → 无面无环 + `TextMuted`；深浅各一遍 |
| 3 | `docs/liquid-glass.md` §6.64 | 无 | 记下这四步测量与「命名档全部越线」的结论，状态写 `先量，不改` |
| 4 | 产品代码 | — | **一个字节都不改**（`healthfaces.go` 与 HEAD 逐字节相同） |

## 行为契约零改动红线

- 高度链 / 行盒 / `Padding` / `Gap` / `Radius` / `Border`：不动。
- 键盘模型、命中框、`Disabled` 位：不动。
- i18n 文案、命令接线、会话管理、设置写回：不动。
- 产品文件零改动（`git status` 只有新增测试文件与本轮文档）。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 3290 → 3292
```

基线（HEAD `08bacb0b` 实测）：PASS **3290** / FAIL **0** / SKIP **139** / 测试函数 **2441**。
（任务书里写的 2439 是旧值，本轮实测 HEAD 上就是 2441。）

## mutation 计划（每条会让哪个 pin 变红）

1. **M1 · 小块的面换成 `Control`** → `TestTheRetryableChipPaintsTheCardPlaneTwice` 四红
   （深浅各两：面不等、读到 `Control`、面差越线、数值不等）。
2. **M2 · degraded/unhealthy 的面换成 `Control`** → `TestTheHealthStatusChipFaces` 四红
   （浅/深的 degraded 与 unhealthy 各一）。
