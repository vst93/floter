# P11-ah 规格 · 滚轮滚不动结果列表：指针悬停的选中把平滑滚动掐断

## 意图

鼠标滚轮在启动器结果列表上**滚不到位**——一格滚轮（120u，约三行）只让列表走一行，而且
是瞬间跳，没有滑行。根因是「指针进入某行就选中它」这条参考规则（R21）与「把选中的行滚进
视野」这条键盘规则共用了同一个 `a.Selected != a.chosenRow` 判据：滚轮把内容从指针底下
挪走，指针底下换了一行 → 选中跟着变 → 启动器对**指针选中的行**也发一次
`List.ScrollIntoView`。框架把 `ScrollIntoView` 读成「应用接管了偏移」
（`mygo@v0.7.0/ui/wheel.go:88-96` 的 `s.list.scrolled`），于是**在途的滚轮滑行被丢弃**，
一格滚轮只走掉被丢弃前的那一小段。

为什么现在做：用户 2026-10-28 的实测反馈把选题顺序定为「界面质感与交互逻辑 + 用户报的 bug
优先」；这一条是**每个用鼠标滚轮的人每帧都会碰到**的交互 bug（不是像素细节，也不是内部一致性）。

## 证据（本轮实测，`ui.Tester` headless）

31 行结果、窗口十行，指针停在列表中部，连续四格滚轮各 120u（合计 480u ≈ 12 行）：

- 现状：`a.List.Visible()` = `[0 9]` → `[4 15]`，**只走了 4 行**；且 `selected` 落在 14
  （指针所指的行不在 14，漂移来自每格之后的 reveal 请求）。
- 逐帧日志（`internal/launcher` 内的临时探针）显示每格只推进一到两行便停住，符合
  「第一帧动了一点，第二帧 glide 被 `scrolled` 掐掉」。

## 落地项

1. **`internal/launcher/launcher.go` 的 App 增加 `pointerChose bool`**
   （放在 `hoveredRow`/`hoverFrameRow` 之后，带注释说明它回答什么：这一帧的
   `Selected` 变化是不是**指针**给的）。
2. **三处指针直接写 `Selected` 的地方各置一次 `a.pointerChose = true`**：
   - `launcher.go:2160` 行进入（`row.Hovered()` 的 entry 边，R21）；
   - `launcher.go:2394` 收藏星点击（「toggle acts on the row the star belongs to,
     which the pointer has just made the choice」）；
   - `launcher.go:2444` 行点击（`row.Clicked()` 之后 `activate`）。
3. **reveal 段（`launcher.go:1465-1483`）先读后清 `pointerChose`**，指针给的选择不滚：

   ```go
   byPointer := a.pointerChose
   a.pointerChose = false
   if a.Selected != a.chosenRow {
       a.chosenRow = a.Selected
       if !byPointer && (!a.heldRowShown || a.Selected != len(results)-1) { … }
   }
   ```

   `a.chosenRow` 仍然推进（否则同一行会被每帧重判一次）；键盘步进、查询变更后的默认选中、
   历史删除后的邻居接管等**非指针**路径不受影响，照旧滚进视野。
4. **不改**：R21 的「指针进入即选中」（选中仍跟着指针走）、键帽/行几何/高度链/字段键位、
   滚轮灵敏度（框架的 `wheelAnimation` 150ms）。

## 为什么这是根因修，不是加过滤

被去掉的是**一次没有意义的滚进视野**：指针选中的行就在指针底下，`near` 语义下它本来
就不会移动内容（`mygo@v0.7.0/ui/list.go:1383-1400`），唯一的副作用是把
`ListState.scrolled` 置真、让框架丢弃在途滑行。所以修的是「谁该发这个请求」，不是把
症状盖掉。

## 行为契约零改动红线

R21（指针进入接管选中）/ R67（⌘ 持行的行不滚进视野）/ 键盘上下步进滚进视野 / 查询变更回到
顶部 / 高度链 / 会话与设置写回 / i18n 文案——全部不动。

## 门槛与基线

- `gofmt -l cmd internal` 空；三份 `go vet`（host / windows / darwin）净。
- `go test -count=1 ./...` 全绿；PASS 计数基线以本轮开工实测为准（见报告）。
- 三平台 `go build ./...` 过。

## 新增 pin

1. `TestTheWheelScrollsTheListUnderThePointer`（`internal/launcher/wheelscroll_test.go`）：
   31 行结果、指针停在列表中段，连续四格 120u 后 `Visible()` 的首行必须前进
   **≥ 10 行**（480u / 40u ≈ 12 行；现状 4 行），并且回滚四格回到起点。同时断言
   指针底下那一行就是 `a.Selected`（滚轮不该让选中离开指针）。
2. `TestTheKeyboardStillScrollsTheChoiceIntoView`（同文件）：指针停在列表上段不动，
   连按 Down 走过视野底部，列表必须跟着滚（`Visible()` 覆盖到 `a.Selected`）——
   这条守 `pointerChose` 的清理，不清就会把键盘的 reveal 一起吞掉。
3. `TestDumpP11AHWheelPNGs`（opt-in，`FLOTER_P11AH_DUMP=1`）：滚轮前后两帧出图。

## mutation 计划（≥2 条，实跑）

- **M1**：reveal 段忽略 `byPointer`（回到旧行为）→ pin 1 红（四格只走 4 行）。
- **M2**：删掉 `a.pointerChose = false` 那一行（标志永不复位）→ pin 2 红（键盘 reveal 被吞）。
