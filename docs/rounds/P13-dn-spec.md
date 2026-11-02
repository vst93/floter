# P13-dn 规格 · 复制按钮的确认语把整行推开 10px

## 意图一句话

一颗按钮从「静息语」换到「确认语」时，**盒子必须不动**：状态反馈改的是这一颗按钮说的话，
不是它邻居的位置。

## 为什么现在做（证据）

选题来自任务书「细节打磨 · 状态反馈」+ P13-dm「仍待」第 1 条剩下的那一半（工具表单里
还没量过盒子的两处）。先量两处：

- `paramempty` 的空槽（`parameditor.go:81-93`）：槽盒 191..702、内沿 200..693、
  1px 虚线 + 8u padding 逐边成立（左 `191+1+8=200`、右 `702-8-1=693`），
  句子墨宽 210 / 盒宽 493 —— **P13-ch 已经把它量透了，本轮无可改**（见报告「量过但不动」）。
- `toolContentActions` 的动作行（`settingsui.go:5407`）：`Copy script` 的墨宽 **62**、
  确认语 `Script copied` 的墨宽 **72**（深色帧逐像素相同），胶囊宽度由孩子撑开，
  于是**按下的一瞬间**右边那颗 `Export script` 的胶囊从 **x=305.0 → 315.0**（+10px）。
  程序模式同一族：`Copy command` / `Command copied`、`Copy execution plan` /
  `Execution plan copied`（i18n 成对，`internal/i18n/copy.go:761-767` 与 `:1593-1597`）。

一次「我按了复制」的反馈，动了这一行两颗按钮的布局：用户的手指没动，界面自己挪了 10px。

## 逐条落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `settingsui.go` `contentAction` | 先换 `label`（`label = done`）再建按钮，宽度随字长变 | 先算地板 `contentActionFloor(c, label, done)`，再换字；按钮 `.MinWidth(floor)` |
| `settingsui.go` 新增 `contentActionFloor` | 无 | 取两句里**更宽**的一句（`c.MeasureText(0, ui.Span{...,Size: body, Weight: 600})`，与画出来的是同一支画笔）+ 胶囊两侧 10u（`t.Space(2.5)`）+ 2px 顶格 |

不动的东西：胶囊的墨（`Control` / `ControlEdge` / `TextStrong`）、高度 30u、圆角、
`Grep` 与间距、1.6s 的确认窗口、`copiedContent` 的语义、i18n 文案、`CopyText` 接线。

## 行为契约零改动红线（逐条核对）

- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**一字未动**。
- 唯一被改的是**一颗按钮的盒宽地板**：它现在按「两句里更长的那句」预留，不再随状态变。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 3321（基线 3320）
GOOS=linux/windows/darwin go build ./...               # 过
```

SKIP **154**（+1 = 本轮 opt-in 的截图 dump）、测试函数 **2483**（+2）。

## mutation 计划（≥2 条）

- **M1**：`contentActionFloor` 只量静息语（`if w > widest && label == labels[0]`）
  → `TestTheCopyButtonsDoNotReflowTheRow` 变红（深/浅各一条，报错串给的是 305.0 → 315.0）。
- **M2**：地板去掉 2px 顶格（`+2` → `+1`）→ 同一条 pin 变红（314.0 → 315.0）。
- 每条都要 `md5sum -c` 逐字节还原 + `touch` 复跑一次（防缓存）。
