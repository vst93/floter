# P13-gm 规格 · E2 的四个圆角：直边上那四行墨，走到 45° 上只剩一行

> 轮次 P13-gm｜合约 §6.158｜上游 P13-gl（§6.157，E2 的左右竖边）｜
> 总目标：细节打磨 + 轻便液态玻璃 + 空间感

## 1. 意图

`docs/liquid-glass.md` §6.155 / §6.156 / §6.157 把 E2（浮层）的四条**直边**拆完了：
顶边四行（影 / 描边 / 冠 / 面）、下沿三行、两侧三行。四条边交汇的地方——`RadiusMD` 10u 的
四个圆角——**从来没进过帧**：那四行墨在弧上落在哪个像素、还读不读得出「面 / 受光边 / 影」
的阶梯，全无读数。P13-gl 的「仍待」④ 点名要量它。

为什么现在做：这是 E2 这把尺子的最后一格；量完才能回答「圆角要不要补一笔」，也才能判断
用户说的「空间感不够」是否真的发生在角落（如果角落把三层墨压成一层，那就是「读起来平」的
一个具体来源）。

**这轮为什么让它更轻、更有空间感**：它给出「角落**不需要**补墨」的实测依据——10u 弧上
1px 环的覆盖率只有约半个像素，所以结构上就读不出第二行；补一笔只会得到更粗的边框
（§1 禁令 3 的同构错误）。少一笔，盒子更薄，层级由影与受光边的**分布**（下多上少）承担。

## 2. 现状（动手前核过的位置）

- `internal/shell/notices.go:369`：`Padding(t.Space(2), t.Space(3)).Radius(tokens.RadiusMD)`
  → 圆角半径 10u，盒 = 720×520 帧里的 `(382,58)–(705,95)`（40 列前的左内边、30 行下的底边）。
- 同一行还挂着 `Background(tokens.Float).Border(1, tokens.InputStroke)`；
  `:412` / `:415` 各画一行 `InsetShadow(0, 1, 0, 0, tokens.EdgeHighlight)`（冠线只在顶边）；
  `theme.ElevThrow` 两层影（环境 + 接触，朝下偏移）。
- 四条直边已有 pin：`internal/shell/floatedge_test.go`（§6.155）、`floatbottom_test.go`
  （§6.156）、`floatvertical_test.go`（§6.157）。圆角没有。

## 3. 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/shell/floatcorner_test.go`（新） | 四角无读数 | 四角 × 深浅的 45° 对角线：ground / shadow / arc / inside / face / clearance / arcStep 字面量 pin |
| 2 | 同上 | 「角落的墨是哪来的」没有说法 | arc 必须落在**同帧直边那两行墨之间**（顶边 border→crown、下沿 border→face），且在中点 ±12 内 |
| 3 | 同上 | 没人守着「角落只有一行墨」 | `clearance == arcStep + 1`——弧上恰好一个着墨像素，面紧接着开始 |
| 4 | 同上 | 角落内侧会不会长出第二条亮线 | `|inside − face| ≤ 2`（§6.157 的规则绕到角落） |
| 5 | 同上 | 四个角是不是同一个半径 | 四角的 `arcStep` 相等（3），跨外观也相等 |
| 6 | 同上 | 角落的影是否服从「光从上面来」 | 同侧：底角 take > 顶角 take；浅色 ≥ 1.5×；深色 take ≤ 16（与 §6.156 的 8 级同一档） |
| 7 | `docs/liquid-glass.md` | 无 §6.158 | 新增 §6.158（含 158-1..158-6 台账）与「圆角故意不做」一行 |

**行为契约零改动红线**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线一律不动。
本轮预期**产品代码零像素改动**（只加 pin）；若量出结构性错误，先写进 `.pending-bugs.md`
「需用户拍板」，不擅自改。

## 4. 门槛（当前实测基线）

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净
GOOS=windows go vet ./...             # 净
GOOS=darwin  go vet ./...             # 净
go test -count=1 ./...                # 全绿
go test -v ./... | grep -c -- "--- PASS"   # 基线 3485（P13-gl）
测试函数数                                # 基线 2650（P13-gl）
GOOS=linux/windows/darwin go build ./...   # 三平台过
```

新 pin 会 +2 个函数（pin + dump），dump 默认 skip → PASS 计数 +1。

## 5. mutation 计划（≥2 条，逐字节还原 + `touch` 复跑）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | `notices.go` 冠线 `InsetShadow(0, 1, …)` → `(1, 2, …)`（把冠线在弧上挪一格） | 158-1（arc 值变）、158-3（arcStep / clearance 关系） |
| M2 | `notices.go` `Radius(tokens.RadiusMD)` → `Radius(tokens.RadiusSM)`（10u → 6u） | 158-1（arcStep 变小）、158-4（四角同半径仍成立但字面量全变） |

M1/M2 都用 `md5sum` 存原值，改回后 `md5sum -c` 逐字节核对，再 `touch` 测试文件复跑。
