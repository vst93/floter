# P10-g 报告：register 深链只认裸命令名——参考在路由层就拒绝路径

规格 `docs/rounds/P10-g-spec.md`。方向：任务书「现在该做什么」交棒条目。

## 1. 参考的答案是「路由层拒绝」，不是「宽松判据」

交棒条目问的是「绝对路径不在 PATH 里时参考给什么答案」。读完参考，问题比它说的更早一层：
参考**根本没有路径分支**，它在路由层就把带路径的 `cmd` 拒掉。

- `main:src-tauri/src/deep_link.rs:364-390` `validate_command`：trim 后非空、≤ `MAX_COMMAND_CHARS`
  （`:116` = 64）、不含 `/` `\`、不是 `..`、不以 `-` 开头、只允许 `[A-Za-z0-9._+-]`。
- `deep_link.rs:1465-1487` 的测试把 `%2Fusr%2Frg`（`/usr/rg`）、`..%2Frg`、`rg%3Brm%20-rf%20%2F`、
  `rg%20--exec%3Dls`、`-rg`、`rg%24(x)` 逐个断言为 `is_err()`。
- 拒绝**不静默**：`Reject::is_silent` 只对 `UnknownAction` / `UnsupportedScheme` 为真
  （`deep_link.rs:225-228`），`CommandNotBasename` 会弹一句 toast。
- 参考的 register 解析本身只在 inventory 候选里找名字（`deep_link.rs:1136-1180` →
  `resolver.rs:63-77`），绝对路径不在候选里；它连这一步都走不到，路由先拒。
- 本仓的**文档早就这么写了**：`docs/extensions/README.zh-CN.md:99-102` 明写
  「`cmd` 只能是单个命令名（不允许路径、`..`、shell 元字符，长度 ≤ 64）」——是代码没做到。
  所以本轮**不需要新 i18n 文案**（复用 `settings.deepLinkRejected`），也不需要拍板。

## 2. 落地

1. **新 `internal/shell/linkname.go`**：`bareCommandName(raw) (string, bool)`，`maxCommandNameChars = 64`。
   逐条照参考；返回 trim 后的值。
   - 参考的 `value.split('.').any(|s| s == "..")` 那一半是**死代码**（按 `.` 切分出的片段不可能
     含 `.`），只有 `value == ".."` 会命中，因此只移植后者，注释写明。
   - 参考把「一个路径分量」拆成 `contains('/') || contains('\\')` **加**字符集两处判断；本仓的
     拒绝句不带原因串（参考带 `CommandNotBasename` / `CommandInsecure` 两个原因），字符集已经
     拒掉两个分隔符，所以**只留字符集一处**（同一条判据不留两个答案）。这一点是被 mutation
     抓出来的：删掉 `ContainsAny` 那条检查，测试**全绿**——它本来就是不可观测的冗余。
2. **路由层接上判据**（`internal/shell/system.go:208-226`）：`name, ok := bareCommandName(...)`，
   不合法时在 `a.Open(SurfaceSettings)` **之前** `a.refuseLink()` 并 return。拒绝不能顺带把用户
   带到集成页（参考的 `focus_main` 只把面板提到前面）。
3. **`registerFromLink` 只剩一条路**（`internal/shell/system.go:247-283`）：删掉
   `if !filepath.IsAbs(path)` 那条与参考相反的分支，名字一律走 `lookTool(name)`
   （`var lookTool = extensions.LookTool`，与 `internal/extensions` 的 `lookTool` / `findInterpreter`
   同一套测试接缝）。活路径修掉的是：`floter://register?cmd=C:\notes.txt` 今天会把一个文本文件
   直接连成集成。
4. **两条入口一次覆盖**：终端拼写 `floter register /usr/bin/rg` 经
   `LinkFromArguments`（`linkargs.go:88-92`）拼成 `floter://register?cmd=%2Fusr%2Fbin%2Frg`，
   落到同一个 `HandleURL`，因此同一条判据同时管住 URL 与命令行（有 pin）。

## 3. 台账

**V = 0**（无像素）。
**B = 0**（i18n 文案、键盘、高度链、会话、设置写回、命令接线零改动；复用
`settings.deepLinkRejected` / `registerNotFound` / `registerAlreadyConnected`，无新 key）。
**S = 2 处用例重写 + 1 处分支删除**：
- `register_test.go` 的 `TestTheRegisterLinkMarksTheProgramItNamed` / `TestTheRegisterMarkIsAMoment`：
  旧「直接把绝对路径交给 `registerFromLink`」→ 新「传名字 + stub 探针给出 fixture 路径」，
  断言本身不变（mark 落到 fixture、第二次是 already-connected、mark 会自己清）。
- `system.go` 的绝对路径分支删除（旧：路径原样收下 → 新：只有名字探针一条路）。

## 4. 门槛（实测）

```
gofmt -l cmd internal                 空
go vet ./...                          净
go test -count=1 ./...                36 包全绿
--- PASS                              2091   （基线 2087，+4 = 本轮 4 个新函数）
--- SKIP                              15     （= 基线）
--- FAIL                              0
测试函数                               1701   （基线 1697，+4）
GOOS=linux / windows / darwin build    全过
```

## 5. 变异（4 条实跑，全部变红并逐字节还原）

1. `bareCommandName` 删掉 `if value == ".."` →
   ```
   --- FAIL: TestBareCommandNameRefusesWhatIsNotOne (0.00s)
       linkname_test.go:39: bareCommandName("..") = "..", true; a register link names a bare command, not this
   ```
2. 路由层的校验挪到 `a.Open(SurfaceSettings)` 之后 →
   ```
   --- FAIL: TestARegisterLinkRefusesAPath (0.00s)
       linkname_test.go:98: floter://register?cmd=%2Fusr%2Fbin%2Frg switched surface to 1
       linkname_test.go:101: floter://register?cmd=%2Fusr%2Fbin%2Frg landed on page 3, want the page it started on
       linkname_test.go:120: the register command line switched surface to 1
   ```
3. `registerFromLink` 恢复旧的 `if !filepath.IsAbs(path)` 形状 →
   ```
   --- FAIL: TestTheRegisterLinkDoesNotTakeAPath (0.00s)
       register_test.go:213: a path was marked as a register target: ".../003/notes.txt"
       register_test.go:217: a path reads "", want "No tool named “.../003/notes.txt” was found on this device. ..."
   ```
4. `bareCommandName` 删掉字符集循环（先试 `ContainsAny` 那条，发现它本来就不可观测——见 §2.1）→
   ```
   --- FAIL: TestBareCommandNameRefusesWhatIsNotOne (0.00s)
       linkname_test.go:39: bareCommandName("/usr/bin/rg") = "/usr/bin/rg", true; ...
       linkname_test.go:39: bareCommandName("rg;rm -rf /") = "rg;rm -rf /", true; ...
   --- FAIL: TestARegisterLinkRefusesAPath (0.00s)
       linkname_test.go:107: floter://register?cmd=%2Fusr%2Fbin%2Frg reads "No tool named “/usr/bin/rg” was found ...", want "That link could not be opened"
   ```
   每条都：注入 → 看到红 → `cp` 还原 → `md5sum -c` 两份文件「成功」→ `touch` 后复跑绿。

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 37 轮，本仓无可改代码。
2. 需拍板清单不变：P9-g 表四条 + **#692 日语 locale** + **#642 `MYGO_LDFLAGS`** +
   `apps.ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. **下一轮第一项**：参考的 register 路由还会拒**未知参数**（`deep_link.rs:491`
   `Reject::UnknownParameter`，测试 `:1515-1528` 用 `cmd=rg&manifest=/a/tool.json`），
   本仓的 register 分支只读 `cmd`/`tool`/`program`，其余查询参数一律**静默忽略**——
   同一个链接在参考里是一句拒绝，在本仓是「照常连接」。先读参考的 `parse_url` 参数表，
   再决定是否把「未知参数」判据接到同一个 `refuseLink`（同样不需要新文案）。
4. `registerFromLink` 的第二道闸门（`lookTool` 找不到 → not-found）现在有 pin
   （`TestTheRegisterLinkDoesNotTakeAPath`），但探针本身仍是「目录 > 名字 > 梯子」，
   大小写拼写限制见 P10-e/P10-f 的「仍待」。
