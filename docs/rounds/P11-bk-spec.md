# P11-bk 规格 · 空查询页只给最近启动（F4-2，上一轮明写的「下轮第 1 条」）

## 意图

搜索框为空时，页面先铺**六个内置指令**（重启/关机/剪贴板/浏览器/计算器/终端）再铺最近启动；
用户说「未输入时不展示内置指令列表」，参考实现的空查询分支也只有最近启动。本轮把这条
与参考相反的行为改掉，并把依赖旧夹具的 pin 重写到「最近启动的若干应用」上。

为什么现在做：`.pending-bugs.md` 顶部 F4 第 2 条是唯一一条「现象明确、根因已定位、只差
夹具改造」的待办；P11-bj 已按参考改过产品行但**因为 21 条 pin 的夹具就是那六行**而回退
（`docs/rounds/P11-bj-report.md` §5），本轮就是补那一半。

## 证据（本轮实测，行号已核对）

- `internal/launcher/search.go:869-878`（`otherResults` 空查询分支）：
  `if strings.TrimSpace(a.Query) == "" { out = append(out, a.commands()...); … recents … }`。
- 参考 `main:src/hooks/useLauncherCatalog.ts:1138-1166` 的空查询分支只 `return recentRows`；
  六条 `SYSTEM_COMMANDS` 只进**有查询时**的打分（`:1201` 起）。
- `internal/launcher/search.go:127` 的 `commands()` 仍被 `Catalog()` 使用 → 命令不会丢，
  输入任意字符走 `matchedCatalog()` 就命中。

## 逐条落地项

1. **位置** `internal/launcher/search.go:864-883`（`otherResults`）。
   - 现状：空查询 `out = append(out, a.commands()...)` 后接 `section:recent` 与最近启动。
   - 改成：空查询**不追加** `a.commands()`；只剩 `calculator` 答案（有算式时）、
     `RecentSection` 标题与最近启动。有查询分支不动（`matchedCatalog()`）。
   - `Results()` 的注释（`:812-816`、`:834-837`）同步改掉「empty query shows the built-in
     commands」的说法 —— 文档不能留着旧答案。
2. **pin 夹具重写（B 类）**：把「空页面上那六行」当夹具的 pin 换成「最近启动的若干应用」。
   新增一个共享夹具助手（`internal/launcher/recentsfixture_test.go`）：
   `recentPage(a *App, n int) []apps.App` —— 造 n 个 `RecentApp 00…`，`SetApps` + `SetRecent`，
   返回应用表；行标题就是 `RecentApp 00` 这一族，别处可直接按名字找行。
   - 需要「六行」的 pin：改成六个最近启动（`RecentApp 00…05`），计数仍是 6（**只有行的来源变了**）。
   - 需要「够滚」的 pin：改成 `n=20`（原来 6 行 + 窗口高度已经能滚的，按各自断言取数）。
   - `SetRecent` 的顺序就是行序（`recentItems` 保序），所以行序断言不变。
3. **新增 pin**：`TestTheEmptyPageShowsOnlyTheRecents`（`internal/launcher/search_test.go` 同级）
   —— 空查询的 `Results()` 里**没有任何** `power-*/clipboard/browser/calculator/terminal`
   ID，只有 `section:recent` + 应用行；`TestBuiltInCommandsStayReachableByTyping`
   —— `Catalog()` 里六条仍在，`SetQuery("restart")` 后 `Results()[0].ID == "power-restart"`，
   `SetQuery("ter")` 能命中 `terminal`。

## 零改动红线

高度链、键盘模型（含 `Super+N` 编号与按住修饰键的常驻行）、会话管理、设置写回、i18n 文案、
命令接线：**一个字节不动**。`commands()` 本身、`Catalog()`、`powerItems()` 也不动。
本轮只改「空页的行集」与钉住它的夹具。

## 门槛命令 + PASS 基线（本轮实测起跑值）

```
gofmt -l cmd internal        # 空
go vet ./...                 # 净
GOOS=windows go vet ./...    # 净
GOOS=darwin  go vet ./...    # 净
go test -count=1 ./...       # 全绿
PASS 计数                    # 2448（起跑实测）
测试函数数                   # 1994（起跑实测）
三平台 build                  # 全过
```

## mutation 计划（≥2，说明每条让哪条 pin 变红）

- **M1** 把 `out = append(out, a.commands()...)` 加回空查询分支 →
  `TestTheEmptyPageShowsOnlyTheRecents` 变红（空页出现 `power-restart` 等）。
- **M2** 把 `commands()` 从 `Catalog()` 里删掉（假装「删行就完事」）→
  `TestCatalogIsAuthoredOrderAndLocalized` 与 `TestBuiltInCommandsStayReachableByTyping` 变红。
- **M3**（可选）把 `recentItems` 的 `limit` 置 0 之外的越界改动 → 行数 pin 红。
