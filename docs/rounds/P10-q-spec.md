# P10-q 规格：发现排名的八项评分就是判据——逐项钉住，并补回缺失的 `~/bin` 输入

## 0. 意图（一句话）

`internal/inventory` 的 `Priority` 是决定设置页「Detected」那 12 行**谁上榜**的唯一函数，
本仓把它移植过来了却从没按参考的每一项核对过；本轮逐项量完，**补回唯一真缺的一项
（`~/bin`）**，并把八项加权的算术值、平局规则、clamp 全部钉成 pin。

为什么现在做（P10-o 报告遗留建议）：`internal/extensions/candidates.go` 的候选排名是上一轮
点名的「唯一还没按参考逐条核对过的排序面」。量完之后结论分两半：

- **Detected 的排名（`inventory.Priority`）** 已经移植，但没有任何测试钉住它的**算术**——
  只钉了「git > a52dec > gcc-12」这种序关系（`internal/inventory/inventory_test.go:78`），
  改一个系数（300→200）不会有任何测试变红。
- **候选选择器（`extensions.SystemRuntimeCandidates`）没有排名**，而这是**对的**：参考的
  `rank_candidates`（`main:src-tauri/src/extensions/inventory.rs:377`）只服务于
  `commands/extensions.rs:419` 的 discovery 建议列表，也就是本仓的 Detected 列表；
  选择器走的是 `resolver.rs:55` 的 `resolve_executable_names`，它按
  `(exact_match, version, stability, history)` 排序后**只把最高分那一档**交出来
  （`resolver.rs:101-108`）。本仓选择器已有的 PATH 先序 + recordedPath 短路
  （`candidates.go:100-127`）等价于「history 让 recorded 成为唯一最优」，
  剩下的差别（选择器行序＝PATH 先序 vs 参考按 path id 字典序）是**故意的**，
  本轮把它钉住而不是改掉。

## 1. 逐条落地项（动手前重读代码确认，行号为本轮实测）

### 1.1 补回 `~/bin`（真缺的一项）

- 位置：`internal/inventory/curated.go:153-169`（`isUserOwnedDirectory`）。
- 现状：owned 表是 `.local/bin`、`.cargo/bin`、`.nix-profile/bin`、`go/bin`、
  `.local/share/flatpak/exports/bin`。参考的表是 `~/.local/bin`、`~/.cargo/bin`、
  `~/.nix-profile/bin`、`~/bin`（`main:src-tauri/src/extensions/inventory.rs:410-418`）——
  **`~/bin` 漏了**，`go/bin` 与 flatpak exports 是本仓自己的补充。
- 实测后果（`HOME=/home/tar`，本轮实测）：`~/bin/a52dec` → **1000**（无加分），
  `~/.local/bin/a52dec` → **1040**。参考会给 `~/bin` 同样 +40。
  差 40 分足以让一个 `~/bin` 里的工具掉出 Detected 前 12 行。
- 应改成：表按参考的四项顺序排前，本仓两项补在后，并注明「前四项＝参考、后两项＝本仓补充」。
- 另一处**故意不同**（钉住，不改）：参考用裸 `starts_with`（`inventory.rs:412`），
  所以 `~/.local/bin-old/x` 也会加分；本仓用 `owned + Separator`（`curated.go:167`），
  只有真正的子目录才算。这是更严的判据，写进注释 + pin。

### 1.2 八项加权的算术值钉住

- 位置：`internal/inventory/curated.go:86-119`（`Priority`）。
- 现状：只有序关系测试（`inventory_test.go:78,102-123`）。
- 应改成：新文件 `internal/inventory/prioritycontract_test.go`，用本轮**实测**值逐项断言
  （表见 §3），每行注释写清这一项对应参考的哪个常量（`inventory.rs:324-329`、
  `:352-368`）。改系数、改长度阈值、改 clamp 都必须变红。

### 1.3 平局规则与 clamp 钉住

- `ranked()`（`internal/inventory/inventory.go:218-236`）：quality 降 → `Priority` 降 → name 升。
  参考 `rank_candidates`（`inventory.rs:377-382`）逐字相同。
- clamp：`Priority` 末行 `if score < 0 { score = 0 }`（`curated.go:116-118`）。
  参考 `score.max(0)`（`inventory.rs:369`）。可达路径：一个 300 字符的 variant 名
  → `1000 − 1160 − 200 < 0` → 0。

### 1.4 选择器行序（故意的分歧）钉住

- 位置：`internal/extensions/candidates.go:74-99`（`lookToolsIn`）。
- 现状：`TestLookToolsListsEveryMatchInSearchOrder`（`candidates_test.go:28`）钉了
  「同一名字跨目录＝PATH 先序」，但**没钉**「目录主序」：`names=["a","b"]`、
  `dir1` 只有 `b`、`dir2` 只有 `a` 时，输出是 `dir1/b, dir2/a`（整组 probe 按 PATH 先序），
  而不是 `a` 先。
- 应改成：新增一条 pin 钉住目录主序，注释写清「参考没有这个概念——它把
  ambiguity 列表按 path id 字典序排（`resolver.rs:95-98`）；本仓按 PATH 先序，
  因为那是操作系统回答「这个命令是谁」的顺序」。

## 2. 行为契约零改动红线

- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**本轮零改动**。
- 产品代码只改 `curated.go` 的 owned 表一项 + 注释；`Priority` 的系数、`ranked()` 的
  比较链、`lookToolsIn` 的循环嵌套**都不动**。
- 不许放宽任何既有断言；`TestDiscoveryRanksRecognisableToolsFirst` 等既有 pin 必须零改动全绿。

## 3. 门槛命令与 PASS 基线（本轮实测，HEAD `cbdccba`）

```
gofmt -l cmd internal                                  # 期望空
go vet ./...                                           # 期望净
go test -count=1 ./...                                 # 期望全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2130，只许涨
GOOS=linux/windows/darwin go build ./...               # 三条都过
```

基线（`cbdccba` 实测）：`--- PASS` **2130** / `--- SKIP` **15** / `--- FAIL` **0** /
测试函数 **1732** / 包 **36**。

本轮实测的 `Priority` 值（`HOME=/home/tar`，用来写 pin）：

| 名字 | 路径 | Priority |
|---|---|---|
| `git` | `/usr/bin/git` | 1300 |
| `a52dec` | `/usr/bin/a52dec` | 1000 |
| `gcc-12` | `/usr/bin/gcc-12` | 800 |
| `xml2-config` | `/usr/bin/xml2-config` | 796 |
| `x` | `/usr/bin/x` | 900 |
| `my-very-long-tool-name` | `/usr/bin/…` | 952 |
| `a52dec` | `/usr/share/applications/a52dec.desktop`（desktop 源） | 1200 |
| `a52dec` | `/usr/bin/a52dec`（有 description） | 1060 |
| `a52dec` | `~/.local/bin/a52dec` | 1040 |
| `a52dec` | `~/bin/a52dec` | **1000 → 本轮修成 1040** |
| `a52dec` | `~/go/bin/a52dec` | 1040 |

## 4. mutation 计划（每条都实跑，看到真红再还原）

1. `curated.go` 把 owned 表里的 `filepath.Join(home, "bin")` 删掉 →
   `TestPriorityUserOwnedDirectories` 的 `~/bin` 行变红（这是本轮修的那一项）。
2. `curated.go` 的 `curatedBonus` 300 → 200 → `TestPriorityArithmetic` 的 `git` 行
   （1300 → 1200）变红。
3. `inventory.go` 的 `return left.Name < right.Name` → `>` → 平局 pin 变红。
4. `curated.go` 的 clamp `if score < 0 { score = 0 }` 删掉 → 长 variant 名那一行变红（负数）。
5. `candidates.go` 把 `for _, dir` / `for _, name` 两层循环对调 → 目录主序 pin 变红。

每条：注入 → `go test -count=1 ./internal/...` 看到红 → 还原 → `md5sum -c` 逐字节核对 →
`touch` 后复跑一次（避免测试缓存）。
