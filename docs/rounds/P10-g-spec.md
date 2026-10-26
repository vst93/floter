# P10-g 规格：register 深链只认裸命令名——参考在路由层就拒绝路径

## 0. 意图（一句话）

`floter://register` 携带的值按参考的 `validate_command` 在**路由层**校验：只收一个裸命令名，
带 `/`、`\`、`..`、前导 `-`、字符集外或超过 64 字符的值一律走既有的拒绝句（`settings.deepLinkRejected`），
于是 `registerFromLink` 不再有「绝对路径原样收下」这条与参考相反的分支。

## 1. 为什么现在做

- 任务书「现在该做什么」交棒条目：`registerFromLink` 的绝对路径分支
  （`internal/shell/system.go:252-260`）只有 `filepath.IsAbs` 判断，没有「是不是程序」判断；
  `floter://register?cmd=C:\notes.txt` 会被直接连成集成。
- 参考的答案已确认，**不是**「宽松判据」，而是**路由层拒绝**：
  - `main:src-tauri/src/deep_link.rs:364-390` `validate_command`：trim 后非空、≤ `MAX_COMMAND_CHARS`
    （`:116` = 64）、不含 `/` 或 `\`、不是 `..`、不以 `-` 开头、只允许 `[A-Za-z0-9._+-]`。
  - `main:src-tauri/src/deep_link.rs:1465-1487` 的测试把 `%2Fusr%2Frg`（`/usr/rg`）、`..%2Frg`
    逐个断言为 `parse_url(...).is_err()`。
  - 拒绝不是静默的：`Reject::is_silent` 只对 `UnknownAction` / `UnsupportedScheme` 为真
    （`deep_link.rs:225-228`），`CommandNotBasename` 会弹一句 toast；本仓已有的那一句就是
    `i18n.Settings.DeepLinkRejected`（`internal/shell/notices.go:486-495`），**不需要新文案**。
  - 参考的 register 解析本身也只在 inventory 候选里找名字（`deep_link.rs:1136-1180` →
    `resolver.rs:63-77`），绝对路径不在候选里，因此「绝对路径不在 PATH 里」的答案就是「不存在」，
    而它连这一步都走不到——路由先拒。
- 本仓还有第二条入口把路径送进同一个分支：`floter register /usr/bin/rg` 经
  `LinkFromArguments`（`internal/shell/linkargs.go:88-92`）拼成 `floter://register?cmd=%2Fusr%2Fbin%2Frg`，
  同样落到 `HandleURL`。路由是唯一闸口，所以只改路由一处即可覆盖两条拼写。

## 2. 逐条落地项

### 2.1 新增纯函数 `bareCommandName`（新文件 `internal/shell/linkname.go`）

- 位置：`internal/shell/linkname.go`（新）。
- 现状：无。`HandleURL` 的 register 分支（`system.go:208-221`）只判 `name == ""`。
- 应改成：`func bareCommandName(raw string) (string, bool)`，逐条照参考：
  trim → 非空 → `len ≤ 64` → 不含 `/` `\` → 不是 `..` → 不以 `-` 开头 →
  每个字符属于 `[A-Za-z0-9._+-]`。返回 trim 后的值。
- 参考里 `value.split('.').any(|s| s == "..")` 这一句是**死代码**（切分后的片段不可能含有分隔符
  `.`），只有 `value == ".."` 那一半会命中；移植行为而不是移植死代码，注释写明。

### 2.2 路由层接上判据（`internal/shell/system.go:208-221`）

- 现状：算出 `name`，只判空，随后 `a.Open(SurfaceSettings)` + `a.registerFromLink(name)`。
- 应改成：`name, ok := bareCommandName(...)`；不合法时在**打开设置页之前**
  `a.refuseLink()` 并 return——拒绝不能顺带把用户带到集成页（参考的 `focus_main` 只把面板
  提到前面，不切页）。

### 2.3 `registerFromLink` 只剩一个答案（`internal/shell/system.go:242-274`）

- 现状：`path := name; if !filepath.IsAbs(path) { resolved, ok := extensions.LookTool(name); … }`
  ——绝对路径原样收下，且完全不判它是不是程序。
- 应改成：删掉 `filepath.IsAbs` 分支，名字一律走名字探针
  `lookTool(name)`（`var lookTool = extensions.LookTool`，与 `internal/extensions` 里
  `lookTool` / `findInterpreter` 同一套「测试可换实现」的接缝）。找不到 → 既有的
  `RegisterNotFound` 句子。
- 这样「路径」这一族在深链上不存在了，`filepath.IsAbs` 那条与参考相反的分支随之消失。

### 2.4 测试

- 新 `internal/shell/linkname_test.go`：
  - 拒：`/usr/bin/rg`、`C:\Windows\notepad.exe`、`..\rg`、`../rg`、`-rg`、`rg;rm -rf /`、
    `rg --exec=ls`、65 字符、`rg$(x)`。
  - 收：`rg`、`my-tool`、`python3.13`、`fd-find`、64 字符整、前后空白的 `  rg  `（返回 trim 值）。
- 新用例（同文件或 `register_test.go`）：
  - `HandleURL("floter://register?cmd=%2Fusr%2Fbin%2Frg")` → 表面不切到设置、`registerTarget` 为空、
    弹出 `DeepLinkRejected`（与 `floter://register` 无名字同一句）。
  - 路径拼写 `floter://register//usr/bin/rg` 同样被拒（`parsed.Path` 剥一个 `/` 后是绝对路径）。
  - CLI 拼写 `LinkFromArguments([]string{"register", "/usr/bin/rg"})` → `HandleURL` 同样被拒。
- 改写（S）：`register_test.go` 的两个用例原本直接传绝对路径给 `registerFromLink`。
  路径分支删除后它们改为：stub `lookTool` 返回 fixture 路径 + 传名字，断言不变
  （mark 落到 fixture 上、第二次是 `RegisterAlreadyConnected`、mark 会自己清）。
  `appkeys_test.go:252` 传的是裸名，不受影响。

## 3. 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**不动**。
- 复用既有文案 `settings.deepLinkRejected` / `settings.extensions.registerNotFound` /
  `registerAlreadyConnected`，不新增 i18n key。
- Linux/macOS/Windows 三平台走同一个纯函数，不按平台分叉。

## 4. 门槛（实测基线，本轮开工时）

```
gofmt -l cmd internal                 空
go vet ./...                          净
go test -count=1 ./...                36 包全绿
--- PASS                              2087
--- SKIP                              15
--- FAIL                              0
测试函数                               1697
GOOS=linux / windows / darwin build    全过
```

PASS 只许涨；测试函数数不许减。

## 5. mutation 计划（≥2，每条说清会让哪个 pin 变红）

1. `bareCommandName` 里删掉 `strings.ContainsAny(value, "/\\")` 判断 →
   `TestARegisterLinkRefusesAPath`（新）与 `TestBareCommandNameRefusesWhatIsNotOne` 同时红。
2. `HandleURL` 的 register 分支把校验挪到 `a.Open(SurfaceSettings)` 之后 →
   `TestARegisterLinkRefusesAPath` 的「表面没切到设置」那条红（拒绝句仍在，所以只有这一条抓得到）。
3. `registerFromLink` 恢复 `if !filepath.IsAbs(name) { … }` 的旧形状（即路径原样收下）→
   新用例里 `registerFromLink("/usr/bin/rg")` 的「找不到」断言红（这条是给路由之外的
   第二道闸门用的，防止有人再从内部调用绕过）。

## 6. 台账预判

- V = 0（纯行为/接线，无像素）。
- B = 0（i18n 文案、键盘、高度链、会话、设置写回、命令接线零改动）。
- S = 2（`register_test.go` 两个用例从「传路径」改成「传名字 + stub 探针」；
  另加新文件，不算 S）。
