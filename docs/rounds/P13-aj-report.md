# P13-aj 报告：空态的间距属于成对的两件东西（墨上 12/12 → 8/12）

规格：`docs/rounds/P13-aj-spec.md`。合约：`docs/liquid-glass.md` **§4.3**（新）、**§6.9**（新）。

## 这轮为什么让它更有空间感

空间感不是「加阴影」，是**哪两件东西属于同一块**。空态区域一直向刻度问**一个数**——
`Gap(8u)`——用在它里面每两件东西之间，于是「标记 / 句子 / 理由」在墨上量出 **12px / 12px**：
一枚 22u 的标记悬在句子上方，读起来是三个平级的邻居（那正是用户说的「东西挤在一个平面上」的
微观版本）。把标记与它引入的那句话收成一块之后，墨上是 **8px / 12px**——眼睛先看到
「标记+句子」这一块，再看到它下面那一句为什么。

## 先量再改（动手前的数）

会话页空态，720×620，深浅两帧，内容列 `x=188..680 / y=120..580`，墨带按 luma 相对区域中位面
±12 级、每行 ≥3 像素检测（≤3 行的悬挂部合并）：

| 帧 | 标记 | 句子 | 理由 | 墨（句子/标记/理由） |
|---|---|---|---|---|
| dark | `165-182` | `195-207` | `220-227` | 222 / 139 / 94 |
| light | 逐像素同 dark | 同上 | 同上 | 219 / 139 / 102 |

- 标记→句子 **12px**，句子→理由 **12px** → 差值 **0**（这就是要改的那一条）。
- 三档墨本来就是对的（P13-ah 之后浅色受光边已经量过；这里量的是**文字**的三档）：
  句子 ≫ 标记 ≫ 理由，深浅两套都在。**所以这一轮不动墨，只动间隙。**

## 落地

| 文件 | 改动 |
|---|---|
| `internal/settingsui/empty.go` | `emptyState`：图标 + 标题合成 `head := ui.Column(c).Center().Gap(t.Space(1))`（4u）；理由、动作仍挂在区域的 8u 上。注释按 §4.3 写清「间隙属于成对的两件东西」与旧值→新值 |
| `internal/settingsui/emptyink_test.go`（新，+2 测试函数） | 墨带检测 `inkBands` / 墨级 `bandInk` + `TestTheEmptyRegionHangsItsMarkOffItsSentence`（三条账 × 深浅两套）+ `TestDumpEmptyRegionInk`（opt-in 出图） |
| `docs/liquid-glass.md` | **§4.3** 新规则 + 旧值→新值表；**§6.9** 新增 12-1..12-4，全部 `已落地` |

**改动后实测**（同一窗口、同一算法）：

| 帧 | 标记 | 句子 | 理由 | 标记→句子 | 句子→理由 | 墨 |
|---|---|---|---|---|---|---|
| dark | `167-184` | `193-205` | `218-225` | **8px** | 12px | 222 / 139 / 94 |
| light | 同 dark | 同上 | 同上 | **8px** | 12px | 219 / 139 / 102 |

**出图**（`FLOTER_P13AJ_DUMP=1`）：`/tmp/floter-P13-aj-sessions-empty-{dark,light}.png`。
与改动前的帧（`/tmp/floter-P14-ab-sessions-{dark,light}.png`，本轮测量阶段留下的同一场景）
逐像素比对：diff bbox `(360,165)-(531,228)`，深 **2086 px** / 浅 **2078 px** 变——
盒子只上移 4px，所以整条「标记以下」的墨都落在 diff 里，而**墨一个级数没变**（上表 222/139/94
与 219/139/102 前后一致），这就是「只动间隙」在像素上的样子。
**这张图上看到的一处变化**：标记与句子的墨带之间原本空 12 行，现在空 8 行；句子与理由之间
仍是 12 行——三者不再等距。

## V / B / S 台账

- **V 1**：`internal/settingsui/emptyink_test.go` 的新账钉的就是本轮重定的间距
  （旧 12/12 → 新 8/12），注释写 `P13-aj 重定基线`。
- **B 0**：没有一条行为/度量账需要改。`empty_test.go` 的两条盒子账（floor 160u、
  句子→理由 8u）**零改动全绿**——这轮恰好证明了盒子账与墨账是两条独立的账。
- **S 0**：`emptyState` 的签名、返回元素类型、四条调用点全部不变（内部多一层 `head` 列）。

## 门槛（实测）

```
gofmt -l cmd internal                                → 空
go vet ./...；GOOS=windows|darwin go vet ./...        → 三平台净
go test -count=1 ./...                               → 全绿（0 FAIL）
--- PASS 计数                                        → 3043 → 3044
--- SKIP 计数                                        → 90 → 91（+1 = 本轮 opt-in 出图）
测试函数总数                                          → 2245 → 2247
GOOS=linux|windows|darwin go build ./...             → 三平台过
```

改完 `touch internal/settingsui/empty.go internal/settingsui/emptyink_test.go` 后复跑
（避免测试缓存复用）：`TestTheEmptyRegionWearsTheReferenceSettingsEmptyMetrics` /
`TestTheSessionsEmptyStateIsTheReferenceEmptyRegion` / `TestTheEmptyRegionHangsItsMarkOffItsSentence`
三条 **PASS**。

## mutation（3 条实跑，全部变红并逐字节还原）

1. **M1** `head` 的 `Gap(Space(1))` → `Space(2)`（恢复均匀 8u）：
   `dark: the mark hangs 12px off its sentence and the why 12px further down — the mark reads as a third peer instead of belonging to the sentence (§4.3)`
2. **M2** 标记墨 `TextSecondary` → `TextTertiary`：
   `dark: mark ink 95 vs why ink 94 — the why must stay the quietest of the three (§4.7)`
3. **M3** 理由墨 `TextTertiary` → `TextSecondary`：
   `dark: mark ink 139 vs why ink 137 — the why must stay the quietest of the three (§4.7)`

每条跑完 `cp` 还原 + `md5sum -c /tmp/empty.md5` → `internal/settingsui/empty.go: 成功`。

## 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

本轮按任务书只走「已有方向，只要打磨」那条道（`craft-loop.md` + `preflight.md`），
没有 Phase 2 的四方向预览、没有 HTML 预览页（方向是用户给定的一条）。

- **第 6 项「编辑细节 → hairline / 大写字距标签」的间距一侧**：菜单里「每一个尺寸/
  每一个间隙都要有存在的理由」。落成 **4u（`Space(1)`）= 刻度上紧邻 8u 的那一档**，
  而不是自造的 6u/7u：区域里只有「同属一句」的那一对能取更紧的一档。
- **第 12 项「材质细节」的反向用法**：菜单要求「内边框高光 / 玻璃折射边」这类**加**的细节，
  本轮做的是**减**——把「实心玻璃」从标记上拿掉（标记曾有一个隐形的 22u 实心盒子，
  它的留白让 8u 的盒子间隙读成 12px 的墨间隙）。宁可换掉均分的 8u，也不给标记加看不见的底盘。
- **第 5 项「多层染色阴影」**：**故意不用**。空态是 E0 区域（§0 硬禁令 3：底层不画受光边、
  不给阴影），加阴影就违反「材质只在需要分层的地方出现」。

## preflight（逐条结论）

- **A 反 AI 套路**：本轮不加渐变、不加 glow、不加白边高光、不加卡片容器；空态仍是「一句话 +
  一句为什么」，没有新增装饰元素。**过**。
- **A2 进攻三步**：DNA 注入 = 上面的「偷了什么」（3 项）；工艺密度菜单只取 2 项**并说明为什么
  不取第 5 项**——这一族共 4 个部件的微调，密度按「每屏 5 项」量不适用（本轮改的对象是一个
  160u 的空态区域，不是一屏），如实记。两轮制：第一轮量 + 改，第二轮（出图复核 + 三个视角）
  见下。**按事实记：密度项不足 5，原因是改动面只有一个区域**；没有静默跳过。
- **B 设计读取一致性**：仍是「近黑/近白底 + 面差 + 受光边」，本轮没有引入任何新色相、
  新半径、新字号；只用刻度上的 4u。**过**。
- **C 动效抽查**：空态无动效，本轮也没有加。`§5` 的 `theme.ColorStep` 不受影响。**过（不适用）**。
- **D2 中文页**：中文空态文案未改（i18n 零改动）；`head` 列只影响垂直间距，中文两行的
  「理由」行高不变（`LineHeight(1.5)` 未动）。**过**。
- **四视角评审各挑一处**：①艺术总监——空态的标记仍是灰的（`TextSecondary`），这不是模板感，
  是合约的「图标次文字色」，不改；②工程师——盒子账（8u）与墨账（8px）现在差 4u 是**故意的**，
  已写进 §4.3 免得下轮有人「修齐」；③甲方——三秒看完：标记 → 一句「没有会话」 → 一句
  「怎么开一个」，顺序与领属都对；④读者——理由的墨 94/102 级与 `TextTertiary` 的 4.5:1 阶梯账
  一致（`theme.go:962` 的那条既有 pin），不需要抬。

## 仍待

1. **需拍板**：`emptyState` 的**失败臂**（`sessionsEmptyFaceFor(failed=true)` → 图标 `alert`）
   现在与「还没有东西」穿同一套墨；错误态要不要借 `t.Warning` 说话（§5 只允许 accent 落三处，
   warning 没有这条限制，但会改一页的色相预算）。当前实现根本走不到这一臂（会话列表是进程内同步），
   所以没有顺手改。
2. 空态的**动作**（`emptyState` 的 `action` 槽）仍是 `pageButton` 的 E1 安静胶囊，与「空态是
   一句话」的语气是否相称，需拍板（P13-ai 仍待 1 是同一类问题的页级版本）。
3. 通用队列下一个候选：`updateRow` 的只读臂（「当前版本: vX」+「已是最新」，控制槽是空的）
   与可编辑行**长得一样**——要先把「只读行的墨」定出来（本轮量墨的方法可以直接复用）。
