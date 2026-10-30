# P13-g 规格 · §6.6 弹层：toast 是第五个浮层，它一直没进过 `ElevThrow` 那一档

## 1. 意图

`docs/liquid-glass.md` §6.6 是视觉队列里**最后一个** `未开始` 的表面。这一轮把它做完：
让 toast 真正按 E2 说话——两层影（环境 + 接触）+ 受光冠线，且**影跟着框的透明度缩放**。
做完之后 §6 表里没有 `未开始` 行，这一支（表面覆盖）收口，转回细节打磨通用队列。

## 2. 为什么现在做（证据）

`internal/shell/notices.go:381-391`（动手前的真实行号）：

```go
if collapsed {
    p.Root.Shadow(0, t.Space(1.5), t.Space(4.5), 0, tokens.ShadowAmbient)
} else {
    p.Root.Shadow(0, t.Space(10), t.Space(26), 0, tokens.ShadowAmbient).
        Shadow(0, t.Space(0.25), t.Space(0.5), 0, tokens.ShadowContact).
        InsetShadow(0, 1, 0, 0, tokens.EdgeHighlight)
}
```

三个问题，每个都能指出**别的浮层已经修过、只有它没修**的那一处：

1. **不随框的透明度缩放**。P11-an 把这条算术落成 `theme.ElevThrow` / `ElevContact`
   （`internal/theme/elevation.go:44-62`，`--elev-shadow-scale = 0.45 + 0.9·(1−opacity)`），
   四个浮层（启动器警告行、设置页卸载对话框、终端页退出便条与失败提示）都改过去了；
   **toast 是第五个，漏了**。框拉到 10%（scale 1.26）时别处重了 2.8 倍，toast 纹丝不动。
2. **墨比参考重一倍**。`ShadowAmbient` 现在是 0.62（P12-a 从 0.55 抬上来，那个值是为
   **窗口**投影定的，`internal/shell/shell.go:4392` 那两笔才是它的主人）；`--elev-3` 的
   宽笔是 `0.30 × scale`。toast 直接借了窗口那一档，于是它读成一圈黑边，不是一层薄影。
   §2 判据 3 要的是「浮起一张纸」，不是「贴一块铅」。
3. **启动器那条 20u 的带子只有一层影、没有冠线**。§2 判据 3：E2 必须两层影；
   `docs/liquid-glass.md` §3 表：E2 的顶边是 `EdgeHighlight`。`collapsed` 臂两样都没有
   （它走的是参考的 `--elev-3-compact`，那条派生是**为 20px 带子**把宽影收窄、并丢掉冠线，
   但没说过要丢掉接触影）。

`internal/ci/elevation_contract_test.go:28` 的扫描只盯 `../launcher ../settingsui
../terminalui` 三个包——**`../shell` 从来不在名单里**，所以这一处能一直活着。

## 3. 逐条落地

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| G1 | `internal/theme/elevation.go` | 只有 `ElevThrow`（10u/26u + 1u/2u，硬编码几何） | 新增 `ElevThrowCompact`：同一对 alpha × `ElevScale`，几何是 `--elev-3-compact` 的 6u/18u + 1u/2u 接触边 |
| G2 | `internal/shell/notices.go` 满高臂 | `tokens.ShadowAmbient`(0.62) + `tokens.ShadowContact`(0.44) 固定值 | `theme.ElevThrow(p.Root, t, stored.MainOpacity)`（0.30/0.20 × scale） |
| G3 | 同处 `collapsed` 臂 | 一笔 `ShadowAmbient`(0.62)，无接触影、无冠线 | `theme.ElevThrowCompact(...)` + `InsetShadow(0,1,0,0, tokens.EdgeHighlight)` |
| G4 | `internal/shell/notices_test.go` | 只有一条「有影」的 pin（满高、只判非零） | 新增：影**随框透明度反向缩放**（100% vs 10%，同一个像素、同一次盘点的前后帧差）；新增：两条臂的冠线都是 E2 的 `EdgeHighlight`（不暗于面色、且比面色亮） |

`ElevThrowCompact` 而不是给 `ElevThrow` 加几何参数：`--elev-3` 与 `--elev-3-compact`
在参考里就是**两条 token**（`base.css:208-211` / `:222-223`），本仓的 token 层也只给
「整档」；给一个函数加参数会让四个既有调用点全部要为「我不需要这个参数」付一次改动。

## 4. 行为契约零改动红线

只动墨与面：toast 的**几何**（宽 `min(90u, 宽-8u)` / 折叠 75u、`Padding(2u,3u)`、
`RadiusMD`、折叠 `Height(5u)`）、**锚点**（满高 Top 16u Right 4u；折叠 Bottom/Left 1.5u）、
**入场弹簧**（`--dur-4` 220ms、`--spring`）、**排序**（折叠只留最新一条）、
**点击/dismiss/action 接线**、i18n 文案、高度链一概不动。`p.Root` 的
`Opacity/Scale/Translate` 一行不改。

## 5. 门槛命令与 PASS 基线（本机实测，动手前）

```
gofmt -l cmd internal                  → 空
go vet ./...                           → 净
GOOS=windows/darwin go vet ./...       → 净
go test -count=1 ./...                 → 全绿
PASS 计数                              → 2891
SKIP 计数                              → 64
测试函数数                             → 2140
```

## 6. mutation 计划

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | `notices.go` 满高臂换回 `tokens.ShadowAmbient` 固定值 | `TestTheToastThrowFollowsTheFrameTransparency` |
| M2 | `ElevScale` 改成常数 1 | 同上（差值为零/反号） |
| M3 | 折叠臂删掉 `InsetShadow(... EdgeHighlight)` | `TestBothToastArmsWearTheFloatingRim/collapsed` |
| M4 | 折叠臂换回单笔 `ShadowAmbient` | `TestBothToastArmsWearTheFloatingRim/collapsed`（接触影）或影随框缩放的折叠子用例 |
