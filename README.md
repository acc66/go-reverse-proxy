# Go Reverse Proxy

首次部署请阅读 [完整安装教程](docs/installation.md)：Docker、Linux/Windows/macOS 二进制、HTTPS 自动续期、自定义缓存及更新排障。


基于 Go 标准库的单上游反向代理，支持 HTTP/HTTPS 上游、WebSocket 升级、流式响应、路径和查询参数转发、优雅退出。无第三方 Go 依赖。

## Docker
复制 .env.example 为 .env，修改 UPSTREAM 为目标网站地址，然后运行：

```sh
docker compose up -d --build
```

访问 http://localhost:8080。停止：docker compose down。

## 本地运行
需要 Go 1.23 或更新版本。PowerShell：

```powershell
$env:UPSTREAM="https://example.com"
go run .
```

Linux/macOS：UPSTREAM=https://example.com go run .

LISTEN_ADDR 可选，默认 :8080。UPSTREAM 必填，只支持 http/https，不允许 URL 内嵌凭据。目标可以带路径前缀。

## 测试
go test ./...
go vet ./...

GitHub Actions 会执行 race 测试、静态检查及 Docker 构建。

## 行为和边界
代理把 Host 设置为上游域名，并重建 X-Forwarded-*，不信任客户端伪造的转发头。上游不可达返回 502。
本服务监听 HTTP；对公网提供 HTTPS 时请在前面配置 TLS 网关。
不会改写 HTML 内绝对链接、Location 或 Cookie Domain。依赖固定域名、登录回调的网站通常需要额外配置，不能保证任意网站直接兼容。
无内置身份认证，请按实际需求限制访问。环境配置中的敏感值不要提交到仓库。

## 静态资源内存缓存
自动启用，进程重启清空。缓存条目预算 64 MiB（计入内容、键与响应头，进程总内存会更高），单文件最多 2 MiB，采用最早写入淘汰；并发首次请求可能分别回源。
只缓存 GET 的公开 CSS、JS、常见图片和字体。源站必须明确返回 Cache-Control: public, max-age=3600（或其他正数时长），代理缓存上限 1 小时，并扣除已有 Age。
没有明确 public 和有效时长则不缓存。尊重 private/no-store/no-cache，拒绝缓存带 Set-Cookie、非 Accept-Encoding 的 Vary、Content-Range、声明的 trailers 的响应以及非 200 状态。
API、后台、用户路径，带 Cookie、Authorization、Range、条件请求头、Cache-Control 或 Pragma 的请求均绕过缓存；HEAD/POST 等不缓存。
查询字符串和 Accept-Encoding 分别计入缓存键。不缓存 HTML、SVG 或私有文件。
响应头 X-Proxy-Cache: HIT 表示命中，MISS 表示候选静态请求回源（不一定已存储），BYPASS 表示绕过。
静态文件更新请使用版本化文件名；需要立即清空缓存可 docker restart seo-proxy。
更新部署：git pull，docker build -t go-reverse-proxy:local .，然后删除旧容器并使用原 docker run 参数重新创建。
缓存只改善重复静态请求，不解决登录接口的回源超时。公开缓存资源应由源站明确标记，不能只凭文件后缀判断公开性。

## HTTPS 与自动续期
参阅 [HTTPS 部署教程](docs/https-deployment.md)，包括域名解析、从原容器迁移、Caddy 自动续期、缓存策略和故障排查。
仓库提供 compose.https.yaml、Caddyfile 与 .env.https.example。HTTP 和 HTTPS 两套 Compose 配置应独立使用。

## 自定义不缓存路径

完整操作步骤见 [自定义不缓存设置教程](docs/cache-settings.md)，包含 Compose、docker run、关闭缓存和验证方法。

无需修改 Go 代码。在 .env 中设置：

```dotenv
CACHE_BYPASS_PATHS=/auth,/password,/payment,/res,/v1,/sublink,/private
```

规则用英文逗号分隔，只填写路径，不填写域名、查询参数。
/private 匹配该路径及 /private/ 下所有子路径，不匹配 /private-other。
也接受 /private/*、尾部斜杠和两侧空格。大小写不敏感。
匹配的是转发到源站的规范化 URL 路径；UPSTREAM 若包含 /base 前缀，应填写 /base/private。
设置 CACHE_BYPASS_PATHS=/ 可关闭全部缓存。留空仍保留内置排除规则。
自定义配置追加到内置 /api、/admin、/user、/wp-admin、/wp-login.php 排除规则，不能用来解除这些规则。
Cookie、Authorization、非 GET 请求和源站 private/no-store/no-cache 等限制仍然有效。
配置格式错误会在启动时报错，避免悄悄忽略排除规则。

首次升级程序：

```bash
git pull
docker compose -f compose.https.yaml up -d --build
```

之后只修改 .env 时，执行下面命令即可，无需重新构建：

```bash
docker compose -f compose.https.yaml up -d
```

普通 HTTP Compose 用 docker compose up -d。
不要仅使用 docker restart：它不会读取新的 .env。应用配置会重建代理容器并清空其内存缓存。
docker run 用户在创建容器时添加 -e "CACHE_BYPASS_PATHS=/auth,/payment,/res"，修改时需重建容器。
响应头 X-Proxy-Cache: BYPASS 可用于确认请求已绕过本代理缓存。

## 预编译下载
在 [GitHub Releases](https://github.com/acc66/go-reverse-proxy/releases) 下载 Linux、Windows、macOS 的 amd64/arm64 版本，见 [二进制安装教程](docs/binary-install.md)。

## 回源超时与日志
支持 UPSTREAM_CONNECT_TIMEOUT（默认 30s）、UPSTREAM_TLS_TIMEOUT（10s）、UPSTREAM_HEADER_TIMEOUT（30s）以及 ACCESS_LOG（默认 false）。见 [配置与错误日志教程](docs/timeouts-and-logs.md)。Compose 已配置每容器约 3 × 10 MiB 日志轮转。

## 一键安装与管理菜单
[一键安装及数字菜单教程](docs/manager.md)：Debian/Ubuntu 原生安装、开机启动、输入数字查看日志/重启/改配置/在线升级，支持离线包校验和失败回滚。安装后运行 fango。

默认后台进程与 systemd 服务为 fango。支持 fango set-upstream https://api10.seox9.com/ 更新已有回源，fango uninstall 卸载；详见管理教程。
## 在线多域名
支持 fango domain-add、domain-list、domain-remove：多个域名共用回源、独立回源及在线切换，详见 [多域名教程](docs/domains.md)。
