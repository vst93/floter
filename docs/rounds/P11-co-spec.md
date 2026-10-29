# P11-co 规格：一帧只拷一份设置（三张表面同一把尺子）

## 意图（一句话）

把 `settings.Store.Snapshot()` 从「每行拷一次整份设置（含 extra map）」收成「每帧一份，
版本做键」，并把三张表面（启动器 / 设置页 / 终端页）收成同一个类型，而不是三份手写拷贝。

## 为什么现在做（实测证据，不是抄旧报告）

`Store.Snapshot()` 不是读字段，是**整份深拷贝**：`internal/settings/store.go:63-68`
`clone(st.current)`，而 `clone`（`:227-235`）在 `extra` 非空时**新建一个 map 并逐个搬键**。

本轮临时插桩（在 `Snapshot()` 里加计数，量完已 `cp` 还原、`md5sum` 逐字节核对
`b00e0fdd824d5aff09a6fedfd734f8f9`）实测**每帧调用次数**：

| 表面 | 每帧 `Store.Snapshot()` | 备注 |
|---|---|---|
| `internal/launcher`（720×420，带查询） | **0** | P11 早期已有版本 memo（`launcher.go:1115-1132`） |
| `internal/terminalui`（800×600，一个会话） | **5** | P11-bt 只 memo 了**调色板**，设置本体没 memo |
| `internal/settingsui` General（720×580） | **15** | 同上：P11-bs 只 memo 了调色板 |
| `internal/settingsui` Integrations（720×580，20 集成×5 命令） | **53** | 每张集成卡每行都拷一遍 |

分配剖面也指向同一处：`BenchmarkGeneralFrame` 的 `-alloc_objects` 里
`floter/internal/settings.clone` 占 **9.31%**（76 461 个对象 / 821 281），
在 `strings.Builder` 与 `ui.segmentedParts` 之后排第三。

同一个仓库里**已经有两个**版本 memo（`launcher.settings()` 与 `theme.Memo` 的
`load func() settings.Settings` 注释都写着「same version, same value」），
但它们是**两套手写形状**，而两张更热的表面一个都没接。这就是「一族同源问题」：
版本做键这件事有三份说法、一张表没接。

## 逐条落地项

### 1. 新增共享类型 `settings.Memo`（`internal/settings/snapshotmemo.go`，新文件）

- 位置 → 现状：`internal/settings/store.go:63-68` 的 `Snapshot()` 是全仓唯一的深拷贝入口；
  调用方各自决定「要不要缓存」。`launcher.go:206-208` 存三个字段
  （`settingsVersion`/`settingsCached`/`settingsValid`），`terminalui`/`settingsui` 什么都没有。
- 应改成：`internal/settings` 加一个 `Memo`，形状与 `theme.Memo`（`internal/theme/memo.go:29-63`）
  一致——`key`（store 指针 + version）、`val`、`valid`、`built`：

  ```go
  type Memo struct { store *Store; version uint64; val Settings; valid bool; built uint64 }
  func (m *Memo) Get(store *Store) Settings   // store 换身份 → Reset；版本相同 → 命中
  func (m *Memo) Builds() uint64
  func (m *Memo) Reset()
  ```

  **键是精确的**：`Store.Version()` 的注释（`store.go:55-59`）写着「same version, same value」，
  且 `Update` 的两条收尾路径（成功 `:104-106`、写失败回滚 `:88-89`）都 `st.version++`，
  所以一次写回（含被拒写回滚）一定换键；`nil` store 返回零值（照 `launcher.settings()` 的旧写法）。

### 2. `internal/launcher` 的私有 memo 换成共享类型（净改动 = 三字段换一字段）

- 位置 → 现状：`launcher.go:206-208` 三字段 + `:1115-1132` 手写比较。
- 应改成：字段换成 `settingsMemo settings.Memo`，`settings()` 变成
  `return a.settingsMemo.Get(a.Store)`（`a.Store == nil` 由 `Memo.Get` 处理）。
  **行为零改动**：版本命中语义、`nil` store 的零值、`settings()` 的 19 个读点全部不动。

### 3. `internal/terminalui` 接上（每帧 5 → 1）

- 位置 → 现状：`terminalui.go:682`、`:795`、`:822`、`:878`、`:1062`、`:1411`、
  `drawer.go:66`、`:129` 全部直接 `a.Store.Snapshot()`。
- 应改成：`App` 加 `settingsMemo settings.Memo` 与 `func (a *App) settings() settings.Settings`，
  上述**绘制期**读点一律改读 `a.settings()`。
  **不动**的两类：`terminalui.go:237`（粘贴回调，事件期，不在帧里）保留 `Store.Snapshot()`；
  `:390`/`:395`/`:406`/`:675` 是标题/文案函数（有的被事件期调用），也保留。
  → 帧内 5 次降到 1 次。

### 4. `internal/settingsui` 接上（General 15 → 1，Integrations 53 → 1）

- 位置 → 现状：`settingsui.go` 17 处 `a.Store.Snapshot()`，其中 `:1746`、`:4018`、
  `:4606` 是 `s := ...` 的整份读（每行一次），`:1057`/`:1090`/`:2356`/`:2980`/`:3182`/
  `:4480`/`:5195`/`:5239` 只要 `Language` 却拷了整份；`pluginsheet.go:476`、
  `textinput.go:51`、`uninstall.go:169` 同源。
- 应改成：`App` 加 `settingsMemo` + `func (a *App) settings() settings.Settings`；
  **绘制期**读点改读 `a.settings()`；`copy()`（`:5238`）与 `language()`（`pluginsheet.go:476`）
  走 `a.settings().Language`。事件期（`recordShortcut` `:3716`、`binding` `:3604`）保留
  `Store.Snapshot()`——它们不在帧里，也不该被帧缓存串味。

### 5. pin（新 `internal/settings/snapshotmemo_test.go` + 三张表面各一条）

- `TestAMemoServesOneSnapshotPerVersion`（`internal/settings`）：同版本 200 次 `Get` 只
  拷 1 次（用 store 的计数钩子或 `Builds()` 读）；`Update` 之后一次 `Get` 换新值；
  写失败回滚后一次 `Get` 读到回滚后的值；`nil` store 返回零值；换 store 身份（两个都
  version 0）必须重建。
- `TestAFrameCopiesTheSettingsOnce`（三张表面各一条，读同一个计数）：每帧
  `Store.Snapshot()` 恰好 1 次（启动器既有 0/1 语义见下）；Integrations 20×5 的夹具
  也必须还是 1（抓「每行拷一次」）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一个字节不动**。
本轮只改「设置值从哪里读」，不改任何值、任何像素、任何布局。V=0 / B=0 / S=1
（launcher 三字段换一字段；两个 App 加一个字段与一个方法）。

## 门槛命令 + PASS 基线（本轮实测，干净 worktree）

```
gofmt -l cmd internal                                   # 空
go vet ./...                                            # 净
GOOS=windows go vet ./...                               # 净
GOOS=darwin  go vet ./...                               # 净
go test -count=1 ./...                                  # 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 2776（本轮实测）
GOOS=linux|windows|darwin go build ./...                # 过
```

基线：**PASS 2776 / FAIL 0 / SKIP 56 / 函数 2060**（本轮在干净 worktree 上量的）。
目标：PASS 只许涨，函数只许涨。

## mutation 计划（≥2 条，说明每条让哪个 pin 变红）

1. **M1**：`Memo.Get` 的命中判断去掉 `m.version == version`（只比 store 指针）→
   `TestAMemoServesOneSnapshotPerVersion` 的「Update 之后换新值」一臂红（服务了旧值）。
2. **M2**：`settingsui` 的 `general()` 里 `s := a.settings()` 改回 `a.Store.Snapshot()` →
   `TestAFrameCopiesTheSettingsOnce`（设置页）计数 > 1 红。
3. **M3（负对照）**：`Memo.Reset` 只清 `valid` 不清 `built` → 计数 pin 仍绿（本轮不量
   `Builds` 的绝对值），证明 pin 读的是它要守的那一件事。
