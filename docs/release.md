# 发版（mygo 原生版）

安装包由 [`.github/workflows/release-mygo.yml`](../.github/workflows/release-mygo.yml) 产出。
`go.yml` 只是每次推送的测试门，**不产出发行资产**。

## 触发

```sh
# 推荐：直接触发，版本号自己给（不带 -f version 时把 mygo.json 的 patch 加一）
gh workflow run release-mygo.yml --ref mygo-rewrite -f version=0.3.16

# 或者推 tag
git tag mygo-v0.3.16 && git push origin mygo-v0.3.16
```

带 `-preview` / `-rc` / `-beta` / `-alpha` 的版本号自动标成 prerelease。
版本号只在**这次构建的工作区**里改写（`mygo.json` 的 `version`、`CHANGELOG.md` 的
`## [Unreleased]` 就地改成 `## [<version>] - <日期>`），不提交回分支。

## 资产

| 文件 | 平台 |
|---|---|
| `floter-<v>-macos-apple-silicon.dmg` / `.app.zip` | macOS（Apple Silicon） |
| `floter-<v>-macos-intel.dmg` / `.app.zip` | macOS（Intel） |
| `floter-<v>-windows-x86_64-setup.exe` | Windows x86_64（NSIS） |
| `floter-<v>-windows-arm64-setup.exe` | Windows ARM64（NSIS） |
| `floter-<v>-windows-<arch>.exe` | Windows 免安装 |
| `floter-<v>-linux-x86_64.deb` / `.rpm` / `.pkg.tar.zst` | Debian/Ubuntu、Fedora/RHEL、Arch（x86_64） |
| `floter-<v>-linux-arm64.deb` / `.rpm` / `.pkg.tar.zst` | 同上（ARM64） |
| `install.sh` + `floter-<v>-linux-<arch>.tar.gz` | 任意 Linux，装到 `~/.local` |
| `update-<target>.json` + `*.delta` | 应用内更新清单与增量（需要签名密钥） |

分工：mygo 自己产出 dmg / `.app` / NSIS setup / deb / tar.gz + `install.sh`；rpm 用
[nfpm](https://goreleaser.com/nfpm/) 补，pacman 的 `pkg.tar.zst` 手工打（`.PKGINFO` +
`.MTREE`），两者与 deb 的路径形状一致：`/opt/floter/floter` + `/usr/bin/floter` 软链 +
hicolor 图标 + `.desktop`。

**归档不改名**：`update-<target>.json` 里的 `url` 和 `install.sh` 都按
`floter-<v>-<target>.tar.gz` 找文件，改名会让应用内更新 404、`install.sh` 找不到包。

## 应用内更新（可选，一次配置）

```sh
go tool mygo keygen                     # 生成 ~/.config/mygo/update-keys/mygo-update.{key,pub}
```

1. `mygo-update.pub` 的内容写进 `mygo.json` 的 `updates.publicKey`（仓库里的那份就是
   `~/.config/mygo/floter-update-keys/` 里的公钥）；
2. `mygo-update.key` 的内容作为仓库 secret `MYGO_UPDATER_PRIVATE_KEY` 存进 GitHub；
3. 私钥**不进仓库**：装了应用的机器只接受用它的签名，丢了就再也发不出能更新的版本。

没配 secret 时构建照常，只是不产出 `update-*.json`（应用内更新不可用）。发布 job 会校验
每个 `update-*.json` 指向的归档确实在 release 里，漏传就在发布时卡住。

## 本机复核打包链路

```sh
MYGO_UPDATER_PRIVATE_KEY="$(cat ~/.config/mygo/floter-update-keys/mygo-update.key)" \
  go tool mygo build -platform linux/amd64,linux/arm64
ls dist/linux-amd64        # floter / .deb / .tar.gz / install.sh / update-linux-amd64.json
```

rpm 与 pacman 两段只在 CI 的 Linux job 里跑（需要 `nfpm`、`bsdtar`、`zstd`）。
