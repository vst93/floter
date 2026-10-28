# P11-ai 报告 — 指针不动就不算「进入」

## 1. 为什么做这一轮

上一轮（P11-ah）的「仍待」第 2 条就是我自己的实测数字：**指针静止停在列表上时，15 次 Down
只前进 8 行**，并把它记成「需拍板（参考侧 React `pointerenter` 在此不可判）」。本轮先回去
读参考的读法，发现它其实可判——然后把这条当交互 bug 修掉，而不是继续挂着。

用户的选题顺序（2026-10-28）：**界面质感与交互逻辑 + 用户报的 bug 优先**。F1/F2/F3 已由
P11-y/z/aa 收完，合约 §6 只剩 `⌘K`（需拍板），所以这一档里就是它。

## 2. 根因（修前实测）

`internal/launcher/launcher.go:2250` 的 R21 入口边只问一件事：**指针底下那一行变了没有**。

```go
if row.Hovered() {
    a.hoverFrameRow = i
    if i != a.hoveredRow && rowSelectable(item) && item.Run != nil {
        a.Selected = i          // 抢走键盘刚走到的选中
        a.pointerChose = true
    }
}
```

「行变了」有两种原因，代码只认一种：

1. **指针动了**（用户的入口）；
2. **内容动了**（`launcher.go:1528` 的 `a.List.ScrollIntoView(row)`——注释自己写着这是
   键盘的规则）——列表每跨过窗口末行就滚一次，静止指针底下的行随之换人，
   于是每一步都被「指针进入」读一次，选中被拉回指针底下。

参考侧为什么有答案：`LauncherResults.tsx:859-862` 的读法是 `onPointerEnter`——**一个指针
事件**。浏览器不会因为内容在静止光标下滑过就补发 `mouseenter`（窗口出现在光标下也不补），
所以「内容动了」在参考里根本不是入口。P11-ah 把它记成「不可判」是看错了层：不可判的是
React 的合成事件细节，可判的是**这条边属于指针还是属于内容**。

复现（修前，新增 pin 的真实报错）：

```
--- FAIL: TestTheKeyboardWalksTheListUnderARestingPointer
    pointeridle_test.go:60: fifteen arrow steps from row 3 landed on 6, want 18:
    a resting pointer took the choice back
```

20 行列表、鼠标停在 Walk 03、按 15 次 Down：**只走到 6**（前进 3 行）。P11-ah 记的 8 行
是它自己那组参数，本轮这组是 3。

## 3. 改法（一处契约 + 三个落点）

规则：**一个指针位置只算「用户给的」当且仅当指针自己动过**（坐标变化，或进出面板）；
**用户用它滚轮也是指针手势**（滚轮是瞄着同一处的手部动作，P11-ah 的 pin 断言滚过之后
`Selected == hoveredRow`，那条不许动）；**app 替键盘滚的那一滚不算**。

| 落点 | 位置 | 做了什么 |
|---|---|---|
| ① 每 pass 读一次指针与偏移 | `launcher.go:1199` 面板具名 + `trackPointer`（新方法，`launcher.go:2055`） | `panel.PointerPosition()`（`mygo` `ui/input.go:1265`，P11-af 在同一 API 上已有先例）给坐标与是否在面板内；`a.List.Visible()` 给首行。两者与上一 pass 比：`moved` / `scrolled`。**第一次读到指针不算动**——窗口开在光标下不该把选中交给它压着的那一行（这是参考的行为，也是旧读法唯一的「开机即选」来源）。 |
| ② 入口边加门 | `launcher.go:2250` | `if a.hoverFresh && i != a.hoveredRow && …` |
| ③ reveal 自报家门 | `launcher.go:1532` | `a.appScrolled = true`；`trackPointer` 用它区分「app 滚的」与「用户滚的」，等列表静下来再复位 |

状态字段（`launcher.go:296-315`）：`hoverFresh` / `pointerAt` / `pointerOver` / `pointerSeen` /
`listFirst` / `listFirstSeen` / `appScrolled`。

### 取舍

- **为什么不用「滚轮事件」直接判定**：`mygo` 不把滚轮事件交给 View；能看见的是偏移变了。
  偏移变有两种来源，我们只知道其中一种是我们自己发的，所以用「自己发的那次要被认领」。
- **`appScrolled` 复位条件**是「这一 pass 偏移没再变」，不是「下一 pass」：`ScrollIntoView`
  的落地可能在后续 pass，一次性的标志会把后续那一格误判成用户的滚轮。
  代价：键盘滚动紧接着的一格滚轮可能不把选中交给新行（顶多一格），方向是「不抢」。
- **不动滚轮语义**：`TestTheWheelScrollsTheListUnderThePointer` 零改动仍绿。

## 4. 台账（V / B / S）

- **V = 0**：没有像素/尺寸断言被改。
- **B = 0**：既有 pin 一条没改，全绿（含 `TestThePointerTakesTheSelectionOnEntry`、
  `TestARestingPointerDoesNotFightTheKeyboard`、`TestThePointerSkipsRowsTheKeyboardCannotActOn`、
  `TestTheWheelScrollsTheListUnderThePointer`、`TestTheKeyboardStillScrollsItsRowIntoView`）。
- **S = 0**：结构未变（面板只是从匿名表达式变成具名变量，`Children` 里多一句 `trackPointer`）。

新增 4 个测试函数（3 条 pin + 1 条 opt-in dump），函数数 1927 → **1931**
（dump 也算函数，默认 `t.Skip`，所以 SKIP 25 → 26）：

- `TestTheKeyboardWalksTheListUnderARestingPointer`（修前红：15 步 → 6，want 18）
- `TestThePointersWordCountsAgainOnceItMovesAfterAScroll`（同一场景的另一半：指针一动，
  它压着的那一行重新拥有选中——防止把「门」写成「关掉指针」）
- `TestATypedQueryKeepsTheChoiceOffTheIdlePointersRow`（查询重排时静止指针不接管选中）
- `TestDumpP11AIIdlePointerPNGs`（opt-in，`FLOTER_PNG=1`）

## 5. 门槛（本轮实测）

```
gofmt -l cmd internal                      空
go vet ./...                               净
GOOS=windows go vet ./...                  净
GOOS=darwin  go vet ./...                  净
go test -count=1 ./...                     全绿
go test -count=1 -v ./... | grep -c "--- PASS"   2363   （基线 2360，+3）
--- FAIL 计数                               0
--- SKIP 计数                               26
GOOS=linux/windows/darwin go build ./...   过
```

## 6. mutation（实跑，注入 → 红 → 还原 → `md5sum -c` → `touch` 复跑）

1. **拆门**：`a.hoverFresh &&` 删掉 →
   ```
   --- FAIL: TestTheKeyboardWalksTheListUnderARestingPointer
       pointeridle_test.go:64: fifteen arrow steps from row 3 landed on 6, want 18:
       a resting pointer took the choice back
   --- FAIL: TestThePointersWordCountsAgainOnceItMovesAfterAScroll
       pointeridle_test.go:85: the keyboard did not walk the list: selected=6 want 18
   ```
2. **reveal 不自报家门**（`a.appScrolled = true` 删掉，门保留）→ 同样的两条红，
   同样停在 6：app 自己的那一滚被读成「用户的」，交接照样发生。
   这说明两个落点各有一半责任，缺一不可。

还原后 `md5sum -c /tmp/p11ai-orig.md5` → `internal/launcher/launcher.go: 成功`，
`touch internal/launcher/*.go` 再跑 `go test -count=1 ./internal/launcher/` → `ok`。

## 7. 图上看到什么（实测）

`/tmp/floter-P11-ai-idle-{entered,walked}-{light,dark}.png`（12 行列表、鼠标停在 Walk 03、
再按 8 次 Down；`FLOTER_PNG=1` 生成）。dump 自己把量到的事实打进日志：

```
entered  chosen="Walk 03"  under-the-pointer="Walk 03"
walked   chosen="Walk 11"  under-the-pointer="Walk 05"
```

两张图叠起来看：**整个列表内容在 `walked` 帧比 `entered` 帧高两行**（同一串标题整体上移），
而指针一次没动——它压着的那一行从 Walk 03 变成了 Walk 05，但选中已经走到 Walk 11。
修前这两张图会是同一行（指针底下那行），也就是键盘白走。

## 8. 仍待（P11-ai）

1. **`⌘K` 动作面**仍是合约里唯一未落地的条目（§4.6）：页脚画着键帽但 chord 没绑定。
   要新增表面 + 新 chord → **需拍板**（原样有效）。
2. **拍板清单原样有效**（P11-aa 默认浅色安静档 3.04:1、P11-z 搜索框 caret、
   P11-y 终端滚动缓冲、P11-ae `CanvasAlpha`、P11-af 浮条拖拽手柄、P9-g 表那一批）。
3. 侦察结论（别当 TODO 重领）：launcher 列表的 Home/End/PageUp/PageDown **故意**没绑；
   `scroll_test.go` 的触控板 pin 保留；`internal/spawn`/剪贴板族/`calculator` 写回已收口。
