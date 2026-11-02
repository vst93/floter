# P13-dy 规格：启动器的行高阶梯在出货尺上是六档，其中两档只差 0.2

**意图一句话**：`TestTheLauncherHeightsStayOnTheLadder` 量了六年的那一帧是**框架库存主题**
的帧（`Spacing 4.0` / `FontSize 14`），它的六档 `15 / 18 / 22 / 24 / 25 / 28` 里有三档是
库存尺的字号，app 一个都不画；本轮把它量到出货尺上，并把 `±0.5` 的容差带换成精确相等。

**为什么现在做**：`P13-dx`「仍待」第 1 条 = `liquid-glass.md` §6.86 表里「阶梯（V）1 条」
（`rungs [15 18 22 24 25 28]` 实画 `[15 17 19 25 25.2]`），是 `dg-l` 的最后一格。
`P13-dq` 已经把两把尺子分开命名（`render` 库存 / `renderShipped` 出货），
`P13-dr`…`P13-dx` 已把同一族的 20 条逐面交割，只剩这一格。

## 证据（本轮实测，先量再改）

`renderShipped`（出货尺，`u = 0.9`）+ `frameaudit.Heights`，en/zh × 深/浅 四组合逐字节相同：

| 臂 | 实画高度 | 每一档是谁 |
|---|---|---|
| 空查询页 | `[17 25 25.2]` | `Type to search` / `Search` / `Settings` |
| 命中页 | `[15 17 19 25 25.2]` | `Ctrl`+`1`+`↩`+`K` / `git` 的副标题 + `PATH` + `Run in shell` / `git` 标题 / `Search` / `Settings`+`Clear` |
| 无结果页 | `[18 25 25.2]` | `No results` / `Search` / `Settings`+`Clear` |

库存尺（`render`，pin 今天量的那一帧）是 `[15 18 22 25 28]` —— `18 / 22 / 28` 三档与
出货尺一个都对不上，而 `±0.5` 的容差带在库存尺下永远绿。

`18.0` 的出处不是字号碰巧：`launcher.go:885` `quietStatement` 画在 `tokens.Emphasis`
（11.7），它的行盒就是 18；同一页 body 的一行是 17。**两档差 1px，两个音。**

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/launcher/textheight_test.go:47` | `render(t, a)` + `tt.SetDark(dark)`（库存尺） | `renderShipped(t, a, dark)`（出货尺） |
| 2 | 同文件 `rungs` | `{15, 18, 22, 24, 25, 28}`（库存尺刻度） | `{15, 17, 18, 19, 25, 25.2}`，每一档带名字（具名常量 + 注释） |
| 3 | 三条臂的 `must` | `{22,25,28}` / 全部 / `{18,25,28}` | `{17,25,25.2}` / `{15,17,19,25,25.2}` / `{18,25,25.2}`（实测值） |
| 4 | 同文件四条 `AuditHeights` 的 `tol` | `0.5` | **`0`**（`Heights` 已按十分位取整，精确相等即可；容差带是「看不见 0.4px」的那条假绿） |
| 5 | 新增 `TestTheLauncherIconBoxIsSevenUnitsTall` | — | 钉住只差 0.2 的那一对：图标键 `= 7 × Spacing = 25.2`、字段行 `= 25`、二者差 **2 个十分位**；且真帧上两档都画出来了 |
| 6 | 新增 opt-in 出图 `dumpLadderFrame`（`FLOTER_P13DY_DUMP=1`） | — | `/tmp/floter-P13-dy-ladder-{light,dark}-{shipped,stock}.png`，四张 |

## 行为契约零改动红线

产品代码**零改动**（只改 `internal/launcher/textheight_test.go`）。高度链、键盘模型、
会话管理、设置写回、i18n、命令接线一个字节不动 —— 本轮改的是**尺子**，不是画面。

## 门槛命令 + PASS 基线（实测）

```
gofmt -l cmd internal                                   # 空
go vet ./...                                            # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 基线 3325
GOOS=linux|windows|darwin go build ./...                # 过
```

测试函数 2492 → **2493**（+1），PASS 3325 → **3326**（实测；阶梯那条 4 个 subtest 不变，
新增函数 1 个 + 它内部无 subtest）。

## mutation 计划（≥2 条，实跑）

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | `renderShipped` 退回 `render`（库存尺） | `height 18.0 is not on the ladder` + `no run of height 19.0` |
| M2 | `rungIconBox 25.2 → 25`（把只差 0.2 的那一对塌成一档） | `height 25.2 is not on the ladder` |
| M3 | 产品：`height.go:62` `rowTitleStep 2 → 0`（行标题退回副标题那档） | `no run of height 19.0 was painted` |
