# P13-fj 报告：空态的上下空气 —— 区域自己那 24u 就是它的呼吸

**意图**：通用队列「空态满态」的最后一格（P13-fi「仍待」第 1 条）。`emptyState`
（`internal/settingsui/empty.go:36`）是四个调用点共用的唯一空态实现：§6.9 钉了三对**部件之间**
的缝，§6.13 钉了出口的**种类**，而**区域自己的上下空气**从来没量过——它此前只是
`Padding(Space(6)) + Center()` 一句散文，加上「floor = 92u / 160u」这个读起来像高度的数。
本轮把它量到底：**三个调用点、两套外观，上下空气都是 21.6**，没有一处需要抬。

## 1. 先量（探针跑完即删，数字抄在这里）

真画法（产品自己的 `emptyState`，720 宽，en，深/浅两套**逐十分位相同**）：

| arm | 区域盒高 | 内容 + 2×24u | floor（u→px） | 上空气 | 下空气 |
|---|---|---|---|---|---|
| 会话页（160u + 板） | **154.6** | 154.6 | 144.0 | **21.6** | **21.6** |
| 自定义快捷键（92u + 一行字） | **147.8** | 147.8 | 82.8 | **21.6** | **21.6** |
| 集成页（92u，无 hint 无出口） | **84.6** | 84.6 | 82.8 | **21.6** | **21.6** |

三句话：

1. **下空气在三个调用点上都是 21.6** = `Space(6)`：空态的密度由这一块区域挑**一次**，
   不由调用点各挑一遍。
2. **上空气是同一个数**：`title.Y − box.Y = 45.0 = 24u + 22u + 4u`（内边距 + 标记盒 + `head`
   的 4u 缝）——标记盒顶正好落在内边距上，上面没有多出来的一层。
3. **floor 是地板，不是高度**：只有集成页（84.6 对 82.8）差 1.8px 就生效；会话页的 160u
   被内容反超 10.6、自定义快捷键的 92u 被反超 65。写下来是为了下次别再以为「区域高 = floor」。

顺带量到的一个读法陷阱：按**墨**量（`inkBands`）时板那一臂读成「上 33 / 下 54」，差 21——
那不是空气，是板自己的面（29.8）比它标签的墨（8 行）高出来的那一截。所以这一轮的 pin
按**盒**量，不按墨量。

## 2. 落地

| # | 位置 | 内容 |
|---|---|---|
| 1 | `internal/settingsui/emptyair_test.go` | `TestAnEmptyRegionBreathesWithOneNumberOnBothSides`：三 arm × 深浅，上/下空气都等于区域自己的 `Space(6)`（±0.6） |
| 2 | 同上 | `TestTheEmptyRegionGrowsPastItsFloor`：会话页 > 144、自定义 > 82.8（floor 是地板），且两个区域的空气相等（一个密度） |
| 3 | 同上 | opt-in dump `TestDumpP13FJEmptyAirPNGs`（`FLOTER_P13FJ_DIR`）：三 arm × 深浅 |
| 4 | `docs/liquid-glass.md` | 新增 **§6.131**（131-1…131-5 台账） |

产品代码**零像素改动**：`git diff --stat` 里只有 `internal/settingsui/emptyair_test.go`
与 `docs/`（spec / report / `liquid-glass.md`）。

## 3. 台账与门槛（实测）

**V 0 / B 0 / S 0**：没有一条既有 pin 变红，没有一条断言被改；产品零改动。

```
gofmt -l cmd internal                       # 空
go vet ./...                                # 净
GOOS=windows go vet ./...                   # 净
GOOS=darwin  go vet ./...                   # 净
go test -count=1 ./...                      # 全绿（0 FAIL）
--- PASS                                    # 3410（基线 3408，+2）
--- SKIP                                    # 192（基线 191，+1 = 本轮 opt-in dump）
测试函数（静态计数）                          # 2580（基线 2577，+3）
GOOS=linux/windows/darwin go build ./...     # 全过
```

### mutation（实跑，原始报错抄在下面）

**M1** — `empty.go:62` 的 `Padding(t.Space(6))` → `Space(5)`：

```
--- FAIL: TestAnEmptyRegionBreathesWithOneNumberOnBothSides (0.08s)
    emptyair_test.go:122: sessions (dark=false): the air under the last part is 18.0, want the region's own padding 21.6
    emptyair_test.go:126: sessions (dark=false): the air above the mark is 18.0, want the same padding 21.6 (nothing may sit above the mark)
    ... 三个 arm × 两套外观，共 12 处
```

**M2** — `empty.go:70` 的 `head` 加 `.Margin(t.Space(1), 0, 0, 0)`：

```
--- FAIL: TestAnEmptyRegionBreathesWithOneNumberOnBothSides (0.11s)
    emptyair_test.go:126: sessions (dark=false): the air above the mark is 25.2, want the same padding 21.6 (nothing may sit above the mark)
    ... 只有上空气红（6 处），下空气一处不动
```

两条都 `git checkout` 还原，`md5sum -c` 逐字节核对（`internal/settingsui/empty.go: 成功`），
`touch internal/settingsui/emptyair_test.go` 后复跑。

## 4. 出图

`FLOTER_P13FJ_DIR=/tmp/p13fj go test ./internal/settingsui -run TestDumpP13FJEmptyAirPNGs` →
`/tmp/p13fj/floter-P13-fj-emptyair-{light,dark}.png`（720×560，三个 arm 竖排）。
**这张图上看到的一处变化**：这一轮零像素改动，所以图是**证据**而不是 before/after——
三块区域的上下留白在图上读成同一个厚度（21.6），而三块的高度差（154.6 / 147.8 / 84.6）
全部来自内容，没有一块是被 floor 撑出来的。

## 5. 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

| 偷的项 | 落成哪个具体数值/动作 |
|---|---|
| 第 **12** 项 材质细节（内边框高光 / 玻璃折射边） | 受光边是**内**线，所以「空气」也要按**盒**读、不按墨读：板的面 29.8 对标签墨 8 行，墨上读成 21px 的假空气；这条差别写进了报告与 §6.131 的 131-4 |
| 第 **6** 项 编辑细节（编号 / hairline / 大写字距标签） | 三个调用点写成一张**编号表**（arm × 盒高 / 内容 / floor / 上空气 / 下空气），每行一个实测数，深浅两套逐值相同 |
| 第 **5** 项 多层染色阴影（拒绝单层灰） | 空态的「层次」只有一条：`Space(6)` 这一档。三对部件缝（8 / 12 / 23）已由 §6.9 钉住，本轮证明**外框那一层也是同一个刻度**，于是整块只有一条密度 |
| 第 **4** 项 签名交互（一个足矣） | 一个决定：**密度只由区域挑一次**（21.6 在三个调用点上逐值相同）——不是每个调用点各调一次内边距 |

只取 4 项而不是凑满 5 项：这一轮是**量尺轮**（零像素改动），凑第 5 项就得为凑数往
空态里加东西，而 §0 的减法禁令优先于密度菜单（craft-loop §四：门禁 > 凑满）。

## 6. preflight 逐条（A / B / C / D2）

- **A 反套路**：本轮不产生像素。无渐变、无发光、无纯黑、无斜体（`noitalic_test.go` 仍绿）、
  无三等分卡片、无「赋能/革命性」类文案（本轮**零文案**）。
- **B 设计读取一致**：读取为「桌面命令面板的设置页，面向每天用键盘的人，用轻便液态玻璃的
  语言」；记忆点未变。本轮的判据只有一条：**一块区域只有一个密度**——上/下空气都是 21.6。
- **C 动效抽查**：不动效。`ReduceMotion` 在探针里是开的；没有 `transition: all` 可违反。
- **D2 中文页**：本轮**不新增一个汉字**（i18n 是红线）。唯一的 D2 缺口仍是既有账：
  `tokens.Caption` 9.9px / 正文 10.8px 对「正文 ≥ 14px」——**需拍板**（§6 已记），本轮不动。
- **A4.1（Carbon 决策门）**「空 / 满 / 错 三种状态不混淆」：空态的三个 arm（有出口的板 /
  有出口的一行字 / 只有结论）在这一轮被同一把尺量过，三种状态的**密度相同、内容不同**——
  这正是「状态只改内容，不改骨架」。
- **P-1（禁斜体）/ P-5（可读性优先）**：本轮零墨改动，两条不受影响。

## 7. 仍待

1. **空态族收口**：`emptyState` 的间距（§6.9）、出口（§6.13）、满态（§6.16）、空气（本轮
   §6.131）四格都已落地，**这一族清空**。剩下的两条空态账都要**新增文案**，属 i18n 红线：
   集成页空态只有一行结论没有出路、会话页空态说错对象（`需拍板`）。
2. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2。
3. **需拍板（沿用）**：启动器两行结果行 36px 盒装 27 行墨 / `ui/tooltip.go:133` 禁用元素不弹提示 /
   §6.111 / §6.113 的 113-5 / §6.114 的 114-4 / 浮层页头字号上阶梯（`104-2`）/ 便条行 rank 1.28 /
   终端会话抽屉失败只加字不换墨 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 / 路径截断保头弃尾 /
   会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
4. **下一格（首选）**：通用队列只剩**边界情况**与**像素细节**两类没有专轮。第一格建议
   `internal/settingsui/rows.go` 的 `cardRow` 控制槽 `Shrink(0)`（多选箱宽度是同族）——
   它既是边界情况也是像素细节，且**不需要拍板**（只动槽的收缩，不动文案与交互）。
