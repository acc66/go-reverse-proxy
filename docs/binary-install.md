# 二进制安装（无需 Go 或 Docker）

在 GitHub Releases 下载对应系统与 CPU 的文件：
https://github.com/acc66/go-reverse-proxy/releases

- Linux x86_64：linux-amd64
- Linux aarch64：linux-arm64
- Windows x64：windows-amd64
- Windows ARM64：windows-arm64
- Intel Mac：darwin-amd64
- Apple Silicon Mac：darwin-arm64

与发行版 SHA256SUMS.txt 对照校验，解压到任意目录。
Linux/macOS：sha256sum 文件名（macOS 可用 shasum -a 256）。
Windows PowerShell：Get-FileHash 文件名 -Algorithm SHA256。

## Linux / macOS

解压 tar.gz 后进入目录：

```bash
chmod +x go-reverse-proxy
export UPSTREAM=https://seo.webshost.club/
export LISTEN_ADDR=:8080
export CACHE_BYPASS_PATHS=/auth,/password,/payment,/res,/v1,/sublink
./go-reverse-proxy
```

## Windows PowerShell

解压 zip 后进入目录：

```powershell
$env:UPSTREAM="https://seo.webshost.club/"
$env:LISTEN_ADDR=":8080"
$env:CACHE_BYPASS_PATHS="/auth,/password,/payment,/res,/v1,/sublink"
.\go-reverse-proxy.exe
```

访问 http://localhost:8080，Ctrl+C 停止。
直接运行二进制不会自动加载 .env；请通过系统或启动脚本设置环境变量。
修改环境变量后重新启动程序。设置 CACHE_BYPASS_PATHS=/ 可以全部绕过缓存。
程序本身提供 HTTP；需要 HTTPS 和自动续期可在前面部署 Caddy，详见仓库 HTTPS 教程。
跨平台编译成功不代表所有 CPU 和系统都已实际运行测试，以 Release 说明及 CI 结果为准。

## 自行编译全部平台

安装 Go、PowerShell 和 tar，在仓库根目录执行：

```powershell
./scripts/build-release.ps1 -Version custom
```

输出到 dist/custom；同版本目录已存在时会拒绝覆盖。
