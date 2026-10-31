# P13-bj 规格：分页脚注的两个状态此前一档墨（Δ = 0）

## 意图

捕获输出页列表底部那一行「分页脚注」有两个会画字的状态：**「正在加载…」（在飞）** 与
**「没有更多结果」（到底了）**。HEAD 上这两个状态**逐像素同墨、同字号、同字重**
（实测对地面的峰值差都是 **118 / 118**，浅深两套外观**一个字节不差**）——一件
「正在发生的事」和一件「到此为止的边界」用同一个声音说话。

本轮把它们拆成两档：**在飞**是这一行唯一的活物（环 + 词），抬到活跃档并戴 500 的字重；
**到底了**是列表的地板，留在静息档不动。这是通用队列「状态反馈」这一格的收尾，
也是 §6.26（P13-bg：结局 / 中点 / 图例三档）与 §6.28（P13-bi：结局带）的同一条阶梯
在**分页状态**上的第三个表面。

## 证据（HEAD `f3b81805`，实测行号）

- `internal/launcher/output.go:291-313`（`outputFooter`）：一个 `switch` 把两个状态各自
  赋给同一个 `text` 变量，然后**同一句** `ui.Text(c, text).FontSize(t.FontSize - 1).TextColor(t.TextMuted)`
  画出去（`:310`）。两种状态在代码里就已经被折叠成一个「词」。
- `internal/launcher/output.go:372`（`footerSpinner`）：环的**活动那一象限**是
  `t.TextMuted`，与词**同墨**——所以环和词确实是一个物体，本轮抬档要一起抬，否则
  环和它的标签会分家。
- 实测（`ui.Tester`，`InputWindowWidth` × `frameHeight(maxResultSlots)`，自评分页插件，
  **装上 app 主题**（`c.SetTheme(a.tokens(c).Theme)`，与 `shell.View` 的生产路径一致），
  读的是画出来的像素，不是 token）：

| 状态 | 词 | 浅：地面 → 峰值 → 差 | 深：地面 → 峰值 → 差 |
|---|---|---|---|
| `footerLoading` | `Loading more…` | 246 → 128 → **118** | 15 → 133 → **118** |
| `footerEnd` | `No more results` | 246 → 128 → **118** | 15 → 133 → **118** |
| 环（活动象限） | — | 246 → 128 → **118** | 15 → 133 → **118** |

差不是「小」，是 **0**：换状态时这一行**没有任何一档墨变化**，只有字变了。

## 逐条落地项

### 1. 两个状态各自的墨与字重（`internal/launcher/output.go` `outputFooter`）

- 位置：`outputFooter`（`:291-313`），`switch state` 与 `:310` 的那一句。
- 现状：一个 `text` 变量 + 一句写死的 `t.TextMuted`。
- 应改成：两个状态各自带自己的墨与字重，**出处只有一个**（`footerInk(state, tokens, t)`）：

  | 状态 | 墨 | 字重 | 理由 |
  |---|---|---|---|
  | `footerLoading` | `tokens.TextSecondary`（浅 `#6B6B70` / 深 `#B0B0B8`） | **500** | 这一行唯一的活物，配一枚正在转的环；它是一个**标签**，不是一句话 |
  | `footerEnd` | `t.TextMuted`（**不动**，浅 `#808087` / 深 `#85858D`） | 默认 | 列表的地板：到此为止，退到静息档 |

- 字阶不动：两个状态仍然是 `t.FontSize - 1`。**改字阶 = 改行盒 = 改高度链**，
  这条线在 §6.24 / §6.25 / §6.26 / §6.28 已经撞过五次红（`tokens.Caption` 11px 与
  preflight D2 的张力仍是「需拍板」项）。

### 2. 环的活动象限跟词同一档墨（`internal/launcher/output.go` `footerSpinner`）

- 位置：`:372`。
- 现状：`p.StrokePath(&lead, footerSpinnerStroke, t.TextMuted)`。
- 应改成：`tokens.TextSecondary`（与 `footerLoading` 的词同墨）。环的**轨道**仍是
  `tokens.Hairline`（那是不动的底，不动）；只有转的那一象限跟着抬。
- 为什么要一起抬：环 + 词是**一个物体**（词是环的标签）。只抬词会让一行里出现
  两个互不相干的声音；只抬环会让标签比它标注的东西还轻。

### 3. 到底了之后环不在了，墨也不该回来（状态切换的连续读法）

- 不需要代码：`footerEnd` 的臂本来就不画环（`:305-307` 的 `if state == footerLoading`）。
- 本轮确认这一点，并在 pin 里量「同一帧里加载词的墨 **严格高于** 到底词的墨」，
  防止以后有人把两档又合并回去。

## 行为契约零改动红线（逐条核对）

| 契约 | 本轮 |
|---|---|
| 高度链（启动器窗口高度 = `Geometry.Height()`） | **不动**：字阶、`MinHeight(5.5u)`、`Padding(0, 3u)`、`Gap(2u)` 一个字节不改 |
| `outputFooterState`（三态的判定） | **不动**：`more` / `loading` / `end` / `none` 的判据一个字节不改 |
| 键盘模型 / 会话管理 / 设置写回 / i18n 文案 | **不动** |
| 分页接线（`loadMoreOutputPage` / `LoadOutputPage` / 滚动触发） | **不动** |
| 环的动效（0.7s 一转、`ReduceMotion` 静止、60fps、`footerSpin` 的起点） | **不动**：只换颜色，不动几何、时长、帧率 |
| 视觉轮只改墨与面 | **是**：改 3 个 token 引用 + 1 个字重，几何零变化 |

## 门槛命令 + PASS 基线（本轮实测，HEAD `f3b81805`）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 3165（基线）
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

- 基线：PASS **3165** / FAIL **0** / SKIP **110** / 测试函数 **2333**。
- 本轮目标：PASS 只许涨；测试函数只许涨。

## mutation 计划（≥2 条，说明每条会让哪个 pin 变红）

| # | 注入 | 预期红在哪 |
|---|---|---|
| M1 | `footerLoading` 的墨退回 `t.TextMuted` | `TestThePagingFooterSeparatesTheLiveStateFromTheEnd`：`the live word and the end word are one ink again (gap 0, want ≥ 12)` |
| M2 | `footerEnd` 也抬到 `tokens.TextSecondary` | 同一条：两个方向都量过（gap 0） |
| M3 | 环的活动象限留在 `t.TextMuted` | `TestThePagingFooterSpinnerSharesTheLiveWordInk`：环与词不是同一档 |
| M4 | `footerLoading` 的字重去掉 | 字重 pin：加载词与到底词的掩码覆盖差塌到 0 |

## 出图

`FLOTER_P13BJ_DUMP=1` 的 opt-in 截图测试导出
`/tmp/floter-P13-bj-footer-<light|dark>-<loading|end>-<before|after>.png`，
本轮至少给 loading/end × 深/浅 四张（before 用 `git stash` 前后的同一帧对比，或在
报告里用像素差 bbox 说明）。
