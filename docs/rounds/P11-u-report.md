# P11-u 报告：「哪个版本更新」只有一个答案

规格 `docs/rounds/P11-u-spec.md`。方向按用户定的「三平台一致性 → 基础功能 → 像素细节」，
落在**基础功能里的发布/安装判据**：同一个问题（两个版本号谁更新）有两份实现，两份在预发布号
上给出相反答案，而两份都没有一条直接断言。

## 1. 做了什么

新增 `internal/semver`（`semver.go`，约 120 行）：

- `Version{Major, Minor, Patch int; Prerelease string}`
- `Compare(a, b Version) int`：数值核心 → 预发布（空 > 非空，否则交 `ComparePrerelease`）。
- `ComparePrerelease(a, b string) int`：语义化版本 2.0.0 第 11 条——按 `.` 切；两侧都是数字
  按数值比；数字标识符低于字母数字标识符；否则按字节序；前缀相同则短的更旧。

实现是**搬**过来的，不是重写：`internal/extensions/semver.go` 的 `Compare` +
`comparePrerelease`（那份是对的）整段移入，`extensions` 现在一行转发；`versioning.Version.Less`
的 4 分支 switch 与 `v.Pre < other.Pre` 删除，同样转发。

只收**排序**，不动**语法**：`versioning.Parse` 仍是严格三段式（发布标签的语法），
`extensions.ParseSemver` 仍接受 `1.0` 两段式（`ProtocolVersion = "1.0"` 依赖它，
`compatibility.go:31`）。两份语法服务两种文档，一份排序服务所有版本号。

## 2. 修掉的那条

`internal/versioning/versioning.go` 旧 `Less` 的最后一行是 `return v.Pre < other.Pre` —— 预发布
后缀按**字典序**比，于是 `0.3.26-preview.10 < 0.3.26-preview.9`。同一对版本，
`extensions.Semver.Compare` 给的是相反（正确）答案。

影响面：`Next` 只用数值核心做 floor，所以这条错答案**从未**影响过发布号；但
`versioning` 包注释自己写着它存在的理由就是「同一个版本号不能有两种含义」（同版本不同字节 →
更新器永远不推送）。判据自己有两份实现，是同一族的第三个形状。改后两份对同一张表同答案。

## 3. V/B/S 台账

- **V = 0**：不碰墨与面。
- **B = 0**：既有断言零改动全绿。`internal/versioning` 的 8 条 `Next` 测试、
  `internal/extensions` 的 `Satisfies`/`registry`/`reprobe` 断言全部原样通过。
- **S = 2**：
  1. 结构：两份排序实现 → `internal/semver`（旧结构 → 新结构）。
  2. 答案：`0.3.26-preview.10 < 0.3.26-preview.9` → `preview.9 < preview.10`
     （`versioning_test.go` 新函数注释 `P11-u 重定基线：旧值 preview.10 < preview.9 → 新值 …`；
     旧代码没有这一行的 pin）。

新增测试函数 **5** 条：`internal/semver/semver_test.go` 2 条（规范表 17 对 + 全序）、
`internal/versioning/versioning_test.go` 1 条（预发布按数值）、
`internal/ci/semver_contract_test.go` 2 条（源码扫描 `TestVersionOrderingHasOneAnswer`、
两包同答案 `TestTheTwoVersionPackagesOrderTheSameWay`）。

## 4. 门槛实测

- `gofmt -l cmd internal`：**空**
- `go vet ./...`：净 · `GOOS=windows go vet ./...`：净 · `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：**0 FAIL**
- PASS **2320**（基线实测 2315）· SKIP **15**（不变）· 测试函数 **1879**（基线 1874）
- `GOOS=linux/windows/darwin go build ./...`：三平台过
- 非视觉轮，不出图。

## 5. 变异（实跑，逐字节还原）

1. `internal/semver.ComparePrerelease` 的「数字 < 字母数字」由 `-1` 改成 `1`：
   ```
   --- FAIL: TestCompareOrdersVersionsTheWayTheSpecificationSays
       semver_test.go:46: Compare(1.0.0-1, 1.0.0-alpha) = 1, want -1
   --- FAIL: TestCompareIsATotalOrder
       semver_test.go:71: Compare({…Prerelease:alpha}, {…Prerelease:1}) = 1 but the reverse is 1
   ```
2. `versioning.Version.Less` 改回 `v.Pre < other.Pre`（字典序）：
   ```
   --- FAIL: TestPreviewsAreOrderedByNumberNotByText
       versioning_test.go:135: 0.3.26-preview.9 is older than 0.3.26-preview.10, but Less says otherwise
   --- FAIL: TestTheTwoVersionPackagesOrderTheSameWay
       semver_contract_test.go:112: 0.3.26-preview.1 vs 0.3.26: versioning says 1, extensions says -1
   ```
   同一次注入还让源码扫描 pin 变红（`--- FAIL: TestVersionOrderingHasOneAnswer`），
   即「第 3 条 mutation：手写字典序比较」也实跑验证过。
3. 还原：`md5sum -c /tmp/mut.md5` 两文件 **成功**（`2fac2c3f…` / `3c886a03…`），
   `touch` 后复跑 `./internal/semver ./internal/versioning ./internal/ci` 三包 **ok**（避开测试缓存）。

## 6. 仍待

见 `.pending-bugs.md` 顶部「仍待（P11-u）」。
