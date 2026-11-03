# 完整安装教程

目标：通过 https://你的域名 访问 https://seo.webshost.club/，缓存公开静态文件，API 和个人数据回源，并自动续期 HTTPS 证书。
示例 aaaa.com 必须替换为你拥有的域名。

## 选择安装方式

- 需要 HTTPS、自动续期和后台自启动：优先用下面的 Docker Compose 方案。
- 不想安装 Docker：下载预编译 Go 程序，按二进制方案运行；HTTPS 另由 Caddy 提供。
- 两种方式不要同时占用同一监听端口。

## A. Docker Compose 完整部署

### A1. 准备服务器

推荐 Linux 服务器。准备公网 IP、域名，以及服务器 SSH 登录权限。
安装 Git、Docker Engine 和 Docker Compose 插件。不同发行版安装步骤以官方文档为准：

- https://docs.docker.com/engine/install/
- https://docs.docker.com/compose/install/linux/

检查：

```bash
git --version
docker version
docker compose version
```

docker version 应能连接服务端；出现 permission denied 时使用有 Docker 权限的账号或 sudo。
仓库构建镜像不要求服务器预装 Go。

### A2. 设置 DNS 和网络

1. 为访问域名配置 A 记录，值为代理服务器公网 IPv4。
2. 无可用 IPv6 时不要配置 AAAA。
3. 云安全组和服务器防火墙放行 TCP 80、443；HTTP/3 可额外放行 UDP 443。
4. 确保其他服务未占用 80、443。不要直接停止不相关业务，应先安排端口迁移。
5. 源站域名 seo.webshost.club 继续解析到源站，不能解析回本代理。
6. 服务器需能出站访问源站、镜像仓库和证书签发服务。

### A3. 下载项目

```bash
git clone https://github.com/acc66/go-reverse-proxy.git
cd go-reverse-proxy
cp .env.https.example .env
nano .env
```

若已经下载：进入原目录 git pull，再编辑原 .env，不要覆盖已有配置。

.env 内容示例：

```dotenv
DOMAIN=aaaa.com
UPSTREAM=https://seo.webshost.club/
CACHE_BYPASS_PATHS=/auth,/password,/payment,/res,/v1,/sublink
```

DOMAIN 不带协议或路径；UPSTREAM 带 https://。
nano 保存：Ctrl+O，回车，Ctrl+X。

### A4. 从旧部署迁移

首次安装跳过此步。
之前用 docker run 启动了 seo-proxy，则先 docker stop seo-proxy。
之前用默认 compose.yaml 启动，则在原项目目录 docker compose down。
只停止你自己的旧代理，迁移期间会短暂停机。

### A5. 检查并启动

```bash
docker compose -f compose.https.yaml config --quiet
docker compose -f compose.https.yaml up -d --build
docker compose -f compose.https.yaml ps
docker compose -f compose.https.yaml logs --tail 100 caddy
```

首次需要下载镜像和编译。成功后访问 https://你的域名。
Caddy 对外监听 80、443，Go 只在 Docker 内网监听 8080。
HTTP 自动跳转 HTTPS；不需要单独设置域名证书文件。

### A6. 证书自动续期

Caddy 自动申请和续期。无需 cron，也无需手动下载证书。
保持域名解析、验证端口和网络可用；CAA 记录若有限制，需允许所选签发机构。
证书和私钥保存在 caddy_data 命名卷中。

- 不要执行 docker compose down -v。
- 不要删除 caddy_data 卷。
- 不要随意更换 Compose 项目名称，否则可能使用新的空卷。
- 迁移服务器时安全迁移数据卷，不要把私钥提交到仓库。

证书签发失败先检查 Caddy 日志、DNS、80/443 入站、CAA 和系统时间，避免反复删除证书重新申请。

### A7. 修改不缓存路径

编辑 .env 的 CACHE_BYPASS_PATHS，用英文逗号分隔。
例如 /private 匹配自身和所有子路径；设置 / 表示关闭全部缓存。

```bash
nano .env
docker compose -f compose.https.yaml up -d
```

仅改配置无需重新编译。不要仅 restart，它不会读取新环境变量。
内置 /api、/admin、/user 等排除规则仍然保留。
完整语法见 [不缓存设置教程](cache-settings.md)。

### A8. 查看和验证

```bash
docker compose -f compose.https.yaml logs --tail 100 proxy
docker compose -f compose.https.yaml exec proxy printenv CACHE_BYPASS_PATHS
curl -sS -D - -o /dev/null https://你的域名/实际静态文件.js
```

连续 GET 相同公开静态文件，第二次有机会返回 X-Proxy-Cache: HIT。
源站必须返回 public 和有效缓存时长；Cookie、Vary、Set-Cookie 等会阻止缓存。
MISS 不保证已存储，BYPASS 表示绕过。不要用 HEAD 检查 GET 缓存。

### A9. 更新、停止和清缓存

更新程序：

```bash
git pull
docker compose -f compose.https.yaml up -d --build
```

更新 Caddy：

```bash
docker compose -f compose.https.yaml pull caddy
docker compose -f compose.https.yaml up -d
```

清空 Go 内存缓存：

```bash
docker compose -f compose.https.yaml restart proxy
```

停止并保留证书数据：

```bash
docker compose -f compose.https.yaml down
```

再次启动：docker compose -f compose.https.yaml up -d。
Docker 服务需随系统启动；容器设置了 unless-stopped，手动停止的容器不会因此自动恢复。

## B. Linux / Windows / macOS 二进制部署

### B1. 下载对应平台

进入 https://github.com/acc66/go-reverse-proxy/releases ，下载对应归档和 SHA256SUMS.txt：

| 设备 | 文件标识 |
| --- | --- |
| Linux x86_64 | linux-amd64.tar.gz |
| Linux aarch64 | linux-arm64.tar.gz |
| Windows x64 | windows-amd64.zip |
| Windows ARM64 | windows-arm64.zip |
| Intel Mac | darwin-amd64.tar.gz |
| Apple Silicon Mac | darwin-arm64.tar.gz |

Linux/macOS 用 uname -m 确认架构；Windows 在系统信息中查看系统类型。
校验文件 SHA256 与 SHA256SUMS.txt 对应条目一致，再解压。
Windows：Get-FileHash 文件名 -Algorithm SHA256。
Linux：sha256sum 文件名。macOS：shasum -a 256 文件名。

### B2. Linux/macOS 运行

进入解压目录：

```bash
chmod +x go-reverse-proxy
export UPSTREAM=https://seo.webshost.club/
export LISTEN_ADDR=127.0.0.1:8080
export CACHE_BYPASS_PATHS=/auth,/password,/payment,/res,/v1,/sublink
./go-reverse-proxy
```

### B3. Windows PowerShell 运行

进入解压目录：

```powershell
$env:UPSTREAM="https://seo.webshost.club/"
$env:LISTEN_ADDR="127.0.0.1:8080"
$env:CACHE_BYPASS_PATHS="/auth,/password,/payment,/res,/v1,/sublink"
.\go-reverse-proxy.exe
```

本机打开 http://127.0.0.1:8080。
这里仅监听回环地址，便于在同机 Caddy 后运行；需要直接从其他机器访问 HTTP 时改为 :8080 并配置防火墙。
程序不读取 .env。环境变量必须由终端、服务管理器或启动脚本提供。
Ctrl+C 停止，关闭终端也可能终止服务。

### B4. 二进制方式启用 HTTPS 和自动续期

安装与你的系统对应的 Caddy：https://caddyserver.com/docs/install 。
DNS 和公网 80/443 条件与 Docker 方案相同。

在 Caddy 的配置文件中添加：

```caddyfile
aaaa.com {
    reverse_proxy 127.0.0.1:8080
}
```

此处与 Docker 方案不同：同机原生 Caddy 使用 127.0.0.1:8080，不使用 proxy:8080。

原生前台验证（另一个终端，Go 保持运行）：

```bash
caddy validate --config Caddyfile --adapter caddyfile
caddy run --config Caddyfile --adapter caddyfile
```

Windows 可使用 .\caddy.exe 替代 caddy。绑定 80/443 需要相应权限。
长期运行请按 Caddy 官方服务教程安装系统服务，并给 Go 配置服务管理器：
https://caddyserver.com/docs/running#manual-installation
使用发行版自带 Caddy 服务时，编辑其服务配置路径（常见 /etc/caddy/Caddyfile），不要重复启动第二个 Caddy 实例。

自动续期由 Caddy 负责，与 Go 无关。保持 Caddy 数据目录持久、可写并由同一服务账号运行。
不要在不同用户账号间随意切换 Caddy，否则默认数据目录可能改变。
前台命令仅用于验证，不代表已经配置开机启动。

### B5. 更新二进制

下载并校验新版本，停止旧 Go 进程，替换程序文件，再以原环境变量重启。
Caddy 配置和证书数据保留。Go 重启后内存缓存会清空。

## C. 网站兼容性与排障

- 证书成功但登录跳回源站：检查应用公开地址、Location 和 Cookie Domain；代理不会自动改写它们。
- 静态资源不经过代理：检查网站 assets_url 是否指向其他域名。
- 502/504：分别查看 Caddy、Go、源站日志，缓存不能解决登录接口超时。
- Go 当前重建 X-Forwarded-*：源站看到的访客 IP 可能是 Caddy 内网 IP，协议可能是 http。依赖原始访客信息的应用需另行增加可信代理支持。
- API、订阅、订单、验证码不应共享缓存；不要全站强制加 public 响应头。
- 交叉编译不代表每种 CPU 都实际运行测试；请查 Release 的验证范围。

更多说明：[HTTPS](https-deployment.md) · [缓存设置](cache-settings.md) · [二进制快速安装](binary-install.md)
