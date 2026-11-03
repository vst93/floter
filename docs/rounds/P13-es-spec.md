# P13-es · 捕获输出页的下沿：一页的呼吸只长在上头

## 意图（一句话）

捕获输出页（`internal/launcher/output.go` 的 `outputBody`）的**上沿**有一格属于页面的呼吸
（`header` = `Space(4)` = 14.4，在裁剪之外），**下沿一格都没有**——滚到底时最后一行的墨离窗口
下沿只剩 **8px**。这一轮把下沿补成同一格呼吸。

为什么现在做：`P13-er`「仍待」第 1 条（合约 113-4）问的就是这一页的下内边距
（`Padding(0, t.Space(2), 0, t.Space(2))` 垂直为 0），标着「本轮未量」。量完的结论是
**列表分支不用改、正文分支才是那块没有带的面**。

## 先量（720×439 帧，深浅同值；`FLOTER_P13ES_PROBE` 探针，未入库）

| # | 量的是什么 | 读数 |
|---|---|---|
| es-1 | 列表分支滚到底：最后一行的墨 → 窗口下沿 | **25px** = 6.2（行内边距）+ 19.8（页脚带 `MinHeight(Space(5.5))`） |
| es-2 | 列表分支的页脚带在 `footerMore` 时有没有画字 | 没有（`footerWord` 返回空），**但带还在** —— 所以下沿的空气与状态无关 |
| es-3 | 正文分支滚到底：最后一行的墨 → 窗口下沿 | **8px** = 3.6（`Padding` 底 `Space(1)`）+ 4.4（行尾） |
| es-4 | 正文分支的上沿：裁剪上沿 → 第一行墨 | 3.6（行首），而**裁剪上沿之上**还有 `header` = 14.4 的页面呼吸 |
| es-5 | 正文分支的 `MaxY` | **685** |
| es-6 | 静止帧（`Y=0`） | 第一行墨仍在 y=104 |

结论：**呼吸只长在头上**。同一个页面，上沿 14.4（页面）+ 3.6（行首），下沿 0（页面）+ 3.6（内容）。

## 落地

| # | 位置 | 现状 → 落成 |
|---|---|---|
| es-7 | `output.go:769` 正文分支的 `ui.Scroll` 内边距 | `Padding(0, Space(3), Space(1), Space(3))` → 底边 **`header`**（`Space(4)` = 14.4）；左右 `Space(3)`、顶部 0 不动 |
| es-8 | 规矩（写进 §6.114） | **有软边 → 软边 + 一格 `Space(2)`**（§6.113 的设置体 / 浮层）；**无软边 → 与页头同一格呼吸**（卡片上的滚动体没有软边，§6.48） |

改后：末尾的墨离下沿 **19px**，`MaxY` 685 → **695.8**，静止帧**逐字节相同**（呼吸在滚动内容的末尾）。

## 行为契约零改动红线

- 高度链（`pluginTextRows` / 窗口高度）：**不动**——改的是滚动内容的内边距，不是窗口的定价。
- 键盘模型、会话管理、设置写回、i18n 文案、命令接线：**不动**。
- 可选中的墨、`outputSpans`、`foldAlpha`、编号族：**不动**。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3357（基线 3356）
GOOS=linux/windows/darwin go build ./...  # 过
```

SKIP 175 → **176**（新增一条 opt-in 出图），测试函数 2527 → **2528**。

## mutation 计划（实跑）

- **M1**：底内边距退回 `Space(1)` → `TestTheOutputTextKeepsThePagesBreathAtItsEnd` 红：
  `the last line ends 8 px above the window's rim, under the 14 px this page spends above its clip`。
- **M2**：把那格呼吸搬到滚动内容的**顶部**（`Padding(header, …, Space(1), …)`）→ 同一条 pin
  仍红（8px），`TestTheOutputTextNeverScrollsIntoTheHeaderBand` **未红**（记在报告里，不改它的判据）。
- **M3（对照）**：`Space(6)` = 21.6 → 26px，pin **仍绿**——它守的是下限，不是某一个数。
