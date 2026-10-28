# P11-u 规格：「哪个版本更新」只有一个答案

## 意图

把「两个版本谁更新」的判据收成一份实现（`internal/semver`），`internal/extensions` 与
`internal/versioning` 都转发；顺手修掉 `versioning` 里预发布号按**字典序**比较的错答案。

## 为什么现在做（证据）

- 同一个问题有两份实现：
  - `internal/extensions/semver.go:72` `func (v Semver) Compare`，预发布号走
    `comparePrerelease`（`:95`，注释自称 "as the specification says"：数字标识符按数值比）。
  - `internal/versioning/versioning.go:64` `func (v Version) Less`，预发布号走
    `return v.Pre < other.Pre`（`:76`）——**字典序**。
- 两份在 `1.2.3-preview.10` vs `1.2.3-preview.9` 上给出相反答案：
  `extensions` 说 `preview.9` 更旧（对），`versioning` 说 `preview.10` 更旧（错）。
- 两份都没有 pin：`internal/extensions` 没有 `semver*_test.go`，`internal/versioning` 的
  8 条测试全走 `Next`，`Less` 的预发布分支没被直接断言过（`grep -n "Pre:" versioning_test.go`
  无命中）。
- 这个包存在的理由本身就是「同一个版本号不能有两种含义」（`versioning.go:1-20` 的包注释：
  同版本不同字节 → 更新器永远不推送）。判据自己有两份实现，是同一族的第三个形状。

## 逐条落地项

1. **新增 `internal/semver`**（约 90 行）
   - `type Version struct { Major, Minor, Patch int; Prerelease string }`
   - `func Compare(a, b Version) int`：数值核心 → 预发布（空 > 非空；否则交给 `ComparePrerelease`）。
   - `func ComparePrerelease(a, b string) int`：语义化版本 2.0.0 第 11 条——按 `.` 切；
     两侧都是数字则按数值比，数字标识符低于字母数字标识符，否则按字节序；前缀相同则短的更旧。
   - 实现从 `internal/extensions/semver.go` 的 `Compare`/`comparePrerelease` **搬过来**（不是重写），
     行为以它为准（它是两份里对的那份）。

2. **`internal/extensions/semver.go`**：`(Semver) Compare` 改成一行转发
   `semver.Compare(...)`；删掉 `comparePrerelease`（约 40 行）。`ParseSemver` 的**容错语法
  不动**（`ProtocolVersion = "1.0"` 依赖它接受两段式，`compatibility.go:31`）。

3. **`internal/versioning/versioning.go`**：`(Version) Less` 改成
   `semver.Compare(v.semver(), other.semver()) < 0`；删掉 4 分支 switch 与
   `v.Pre < other.Pre`。`Parse` 的**严格语法不动**（三段式、无前导零——那是发布标签的语法，
   与扩展清单的容错语法是两个问题，本轮只收「排序」这一件事）。

4. **pin**
   - `internal/semver/semver_test.go`：排序表（数值预发布、数字<字母、前缀短的更旧、
     发布 > 预发布、核心优先于预发布）。
   - `internal/versioning/versioning_test.go` 新增一条：`preview.9 < preview.10`（**S**：
     旧答案 `preview.10 < preview.9` → 新答案），并断言发布 > 自己的预发布。
   - `internal/ci/semver_contract_test.go`：
     - `TestVersionOrderingHasOneAnswer`：`internal/` 里除 `internal/semver` 外不得出现
       预发布号的字典序比较（`.Pre <` / `.Prerelease <` / 自定义 `comparePrerelease`）；
     - `TestTheTwoVersionPackagesOrderTheSameWay`：同一张表同时喂
       `extensions.ParseSemver`+`Compare` 与 `versioning.Parse`+`Less`，两者符号必须一致。

## 红线（零改动）

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：不碰。
- `versioning.Parse` 接受的拼写集合、`Next` 的规则（floor / patch+1 / 跳过已占用核心）、
  `extensions.Satisfies` 的运算符语义：**断言零改动**，`Next` 的 8 条测试与
  `registry_test.go`/`reprobe_test.go` 的 `Satisfies` 断言必须原样全绿（B）。

## 门槛（实测基线，改动前）

- `gofmt -l cmd internal`：空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：0 FAIL · PASS **2315** · SKIP **15** · 测试函数 **1874**
- 三平台 `go build ./...`：过

## mutation 计划

1. `internal/semver.ComparePrerelease` 里把「数字标识符低于字母数字标识符」的 `-1` 改成 `1`
   → `internal/semver` 排序表 + ci 的「两包同答案」表同时变红。
2. `internal/versioning.Version.Less` 改回 `v.Pre < other.Pre`（字典序）
   → 新增的 `preview.9 < preview.10` 与 ci 的一致性表变红。
3. 在 `internal/extensions` 里加一行 `.Prerelease <` → 源码扫描 pin 变红。
