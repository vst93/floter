# P11-ab 规格：搜索行补上前导放大镜——字段与结果行共用同一条文字列

## 意图（一句话）

启动器搜索行左边那个 28u 的图标列**一直空着**，查询文字从 16u 起；把 Raycast 的
放大镜放回这一列，让**查询文字落到结果行标题的 54u 列上**——`docs/raycast-design.md`
§4.2 里记着的唯一一处「启动器自己没做」的质感缺口。

## 为什么现在做（证据）

选题顺序（任务书）：用户实测反馈 → 合约未落地条目 → 上一轮「仍待」→ 界面质感与交互。
用户实测三条（F1 终端异常字符 / F2 红线扎眼 / F3 终端玻璃边）已由 P11-y、P11-z 收官，
P11-aa 又补了高对比度下的「字」；**合约里未落地且不需要拍板的只剩这一条**：

- `docs/raycast-design.md` §4.2 末段：「**未落地项**：Raycast 的放大镜在文本列**左侧**的
  独立槽里。……所以这一版**不画前导放大镜**，缺口记在这里（见 §6 末尾）。」
- §6 末尾「启动器里自己没做的两处」：§4.2 的搜索行前导放大镜、§4.6 的 `⌘K` 动作面。
  后者要动键盘模型（合约明写「键盘模型不动」），属拍板项；**前者只是墨与几何的补完**，
  本轮做它。
- 现状代码：`internal/launcher/launcher.go:1266-1273` 是那段「不画前导标记」的注释，
  紧跟着 `:1279` 的 scope mark 分支。搜索行在 `:1245`（`launcher.fieldRow`），
  水平 padding `Space(4)`（16u，`:1247`），字段从 16u 起。

旧理由（「插标记会把整条文本列推右 22u」）只在**把标记塞进字段左侧的 padding 里**时
成立。本轮的落法不占 padding：放大镜**占用结果行那 28u 的图标列 + 10u gap**，
于是字段从 16u 挪到 **16 + 28 + 10 = 54u**——正好是结果行标题的列
（`internal/launcher/launcher.go:2164` 的 `Space(7)` 图标板，pin
`TestTheFieldAndTheRowsShareTheReferenceTextColumn` 的 54）。那条 pin 的名字
（「字段与结果行共用同一条文字列」）第一次变成字面事实，而不是「16 与 54 各自正确」。

## 逐条落地项（行号本轮实测）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/launcher/launcher.go:1266-1278`（field row 里 scope mark 之前） | 只有一段解释「不画前导标记」的注释，没有元素 | 注释改写为「标记占用结果行的图标列」；无 scope 时画一个 **28u 方框（`Space(7)`）+ 右 10u（`Space(2.5)`）**，框内居中一枚 **16u（`Space(4)`）`Glyph("search")`**，墨 `t.TextMuted`（与占位符同档，无 accent） |
| 2 | 同上，scope mark 分支 `:1279` | scope mark 与字段之间只有它自己的右 10u margin | 保持原样：**一个模式拥有字段时，scope mark 取代放大镜**（Raycast 亦然：槽里只有一样东西）。scope mark 的自然宽度照旧把字段推右——它是「谁拥有这个字段」的字，不裁 |
| 3 | `internal/launcher/actionbarfooter_test.go:246` 的 pin | 断言 caret 在 `x=16` | 断言 caret 在 `x=54`（= 结果行标题列），并新增断言：**放大镜的墨落在 16..44 这一段里**（`V` 重定基线，见台账） |
| 4 | `docs/raycast-design.md` §4.2 / §6 末尾 | 「不画前导放大镜，缺口记在这里」 | 改成已落地：放大镜占图标列、查询落在 54u、scope mark 取代它；§6 末尾只剩 `⌘K` 一条 |

## 行为契约零改动红线

- **高度链**：`fieldRowUnits = 14`（56u）、`rowUnits`、`actionBarUnits`、窗口定高
  一个数都不动（本片不碰 `height.go`）。
- **水平度量**：结果行的 16u 缩进 / 28u 图标列 / 10u gap 不动；字段行的 16u padding 不动；
  查询列 16 → 54 是本片唯一移动的数，且**移到结果行已有的列上**（不是新开一列）。
- **键盘模型**：`fieldKeys`、`pendingCaret`、`HandleInput`、选中/提交逻辑零改动。
- **会话管理 / 设置写回 / i18n 文案 / 命令接线**：零改动（放大镜没有文案，
  `Glyph("search")` 已存在，`internal/launcher/glyphs.go:392`）。
- **accent 预算**：放大镜走 `TextMuted`，不碰 `t.Accent`（§5）。

## 门槛命令 + PASS 基线（实测）

```
gofmt -l cmd internal            # 必须空
go vet ./...                     # 必须净
GOOS=windows go vet ./...        # 必须净
GOOS=darwin  go vet ./...        # 必须净
go test -count=1 ./...           # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

- 起点（HEAD `8df7600`，本轮开工前实测）：PASS **2349** / FAIL 0 / SKIP 18 /
  测试函数 **1909**（`grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l`）。
- 本轮预期：PASS 2349 + 新增 1 条 pin（+ 1 条 opt-in dump）= **2351**，函数 1909 → 1911。
  一条 opt-in dump 新增 SKIP（18 → 19）。

## mutation 计划（≥2 条，逐条说明会让哪个 pin 变红）

| # | 注入的缺陷 | 期望变红的 pin | 期望报错串（大意） |
|---|---|---|---|
| M1 | 删掉放大镜的 10u 右 margin（`Space(2.5)` → 0） | `TestTheFieldAndTheRowsShareTheReferenceTextColumn` | `the field's caret starts at x=44, want the rows' title column 54` |
| M2 | 把放大镜的墨改成 `t.Accent` | `TestTheSearchRowLeadsWithANeutralMark` | `the search row's leading mark carries accent ink` |
| M3 | 删掉整个放大镜槽 | 同上 + caret pin | `the search row draws no leading mark` / caret `x=16` |

还原一律 `cp` 备份回来（**不许 `git checkout`**，P11-z 的教训），`md5sum -c` 逐字节核对，
`touch` 后复跑一次避免测试缓存。

## 视觉证据

`FLOTER_PNG=1 go test -run TestDumpP11ABSearchRowPNGs ./internal/launcher/`
→ `/tmp/floter-P11-ab-searchrow-<dark|light>.png`（深/浅各一张，卡合成到中性桌面）。
报告写「这张图上看到的一处变化」= 像素读数（放大镜的墨柱落在 x=22..38、
查询 caret 从 16 挪到 54、54 处与结果行标题同一列）。
