# 回源超时与日志教程

## 1. 设置回源超时

在 .env 中添加（新版本 v1.2.0 起）：

```dotenv
UPSTREAM_CONNECT_TIMEOUT=10s
UPSTREAM_TLS_TIMEOUT=15s
UPSTREAM_HEADER_TIMEOUT=60s
ACCESS_LOG=true
```

| 配置 | 默认 | 作用 |
| --- | --- | --- |
| UPSTREAM_CONNECT_TIMEOUT | 30s | 新建源站连接的超时，包括连接阶段的 DNS 解析 |
| UPSTREAM_TLS_TIMEOUT | 10s | HTTPS 握手超时 |
| UPSTREAM_HEADER_TIMEOUT | 30s | 请求发送完成后等待源站响应头的超时 |
| ACCESS_LOG | false | 是否记录请求状态码、耗时、字节数和缓存结果 |

必须带单位，例如 10s、60s、2m；不接受零、负数或裸数字 60。
超时配置不限制整个响应正文下载时长，不是视频和 WebSocket 的总时长。
连接已复用时不需要重新连接或 TLS 握手。
加大等待时间不能修复断网，也不会让源站变快。前置 CDN 或其他网关若先超时，仍需检查那一层。

首次更新源码和镜像：

```bash
git pull
docker compose -f compose.https.yaml up -d --build
```

以后只修改 .env：

```bash
docker compose -f compose.https.yaml up -d
```

普通 HTTP Compose 去掉 -f compose.https.yaml。
docker run 在创建容器时添加：

```bash
-e UPSTREAM_CONNECT_TIMEOUT=10s \
-e UPSTREAM_TLS_TIMEOUT=15s \
-e UPSTREAM_HEADER_TIMEOUT=60s \
-e ACCESS_LOG=true
```

这些是 docker run 的参数片段，不能单独运行。已有容器需重新创建。
二进制不会自动读取 .env，在启动前通过 export 或 PowerShell 的 $env: 设置同名环境变量，并重启程序。

## 2. Docker Compose 查看日志

最近 100 行 Go 日志：

```bash
docker compose -f compose.https.yaml logs --tail 100 --timestamps proxy
```

实时跟踪：

```bash
docker compose -f compose.https.yaml logs -f --tail 100 --timestamps proxy
```

Ctrl+C 只退出日志查看，不停止容器。

证书申请、续期、Caddy 启动错误：

```bash
docker compose -f compose.https.yaml logs --tail 100 --timestamps caddy
```

查看最近 30 分钟并筛选回源错误（Linux）：

```bash
docker compose -f compose.https.yaml logs --since 30m --timestamps proxy 2>&1 | grep -Ei 'upstream request failed|timeout|invalid|must be|error'
```

导出两服务日志：

```bash
docker compose -f compose.https.yaml logs --since 1h --timestamps proxy caddy > proxy-diagnostics.log 2>&1
```

检查实际超时设置：

```bash
docker compose -f compose.https.yaml exec proxy printenv UPSTREAM_HEADER_TIMEOUT
```

配置无效导致容器启动失败时，exec 可能无法执行，直接看 logs。

## 3. docker run 容器日志

```bash
docker logs --tail 100 --timestamps seo-proxy
docker logs -f --tail 100 seo-proxy
docker logs --since 30m seo-proxy 2>&1 | grep -Ei 'upstream request failed|timeout|error'
```

两套 Compose 已设置 json-file 日志轮转：每个容器最多 3 个文件，每个约 10 MiB。
升级后用 up -d 重建应用日志配置。旧日志可能因重建或轮转消失，排障时及时导出。
docker run 可自行加 --log-driver json-file --log-opt max-size=10m --log-opt max-file=3。

## 4. 二进制查看和保存日志

默认写入标准错误，前台运行时直接显示在终端。
程序不会自动创建 error.log。

Linux/macOS 在已有环境变量配置下运行并保存日志：

```bash
./go-reverse-proxy 2>&1 | tee -a proxy.log
```

另一个终端实时看文件：

```bash
tail -f proxy.log
grep -Ei 'upstream request failed|timeout|error' proxy.log
```

PowerShell：

```powershell
.\go-reverse-proxy.exe 2>&1 | Tee-Object -FilePath proxy.log -Append
Get-Content .\proxy.log -Tail 100
Select-String -Path .\proxy.log -Pattern "upstream request failed|timeout|error"
```

文件重定向不提供自动日志轮转；长期运行请使用系统服务日志或自行配置轮转。
若已配置 systemd 服务，使用 journalctl -u 你的服务名 -n 100 --no-pager 和 journalctl -u 你的服务名 -f。
本项目不会自动创建 systemd 服务。

## 5. 日志含义

ACCESS_LOG=true 时，示例：

```text
access method=GET status=200 duration_ms=12 bytes=2048 cache=HIT
```

访问日志不记录 URL 路径、查询参数、Cookie、Authorization 或正文，避免泄露订阅 token 和登录数据。
duration_ms 是该请求处理耗时，不是单独的回源耗时；长连接在结束后才产生完整日志。
WebSocket 升级后的传输字节不计入普通响应字节统计。
错误日志始终开启，不受 ACCESS_LOG=false 影响。
回源错误日志包括方法、源站主机和底层错误原因。
Caddy 默认未启用逐请求访问日志；其 logs 主要用于运行和证书问题。

| 错误 | 排查方向 |
| --- | --- |
| dial tcp ... timeout | 线路、源站防火墙、源站端口；连接阶段 |
| no such host | DNS 或域名错误 |
| TLS handshake timeout | HTTPS 握手、丢包、源站 TLS |
| timeout awaiting response headers | 源站处理慢，或连接建立后网络迟滞 |
| connection refused | 目标端口没有服务或被主动拒绝 |
| x509 ... | 源站证书、域名或系统时间，不要通过关闭校验掩盖 |
| status=502 | Go 回源失败或上游本身返回 502，结合错误日志判断 |
| status=504 | 可能是源站返回；外层网关也可能在 Go 完成前就超时 |

Go 自身回源错误仍返回 502。错误之后不要盲目重试登录、支付等 POST。
静态缓存只能减少可缓存文件的重复回源，不能解决登录超时。

新版一键安装器默认使用 fango 服务与 /etc/fango.env，日志命令用 journalctl -u fango。本文早期手动安装示例的 go-reverse-proxy 名称仅适用于未迁移的旧部署。迁移、更新回源及卸载请见 [管理教程](manager.md)。

## 客户端 IP（v1.3.0 起）

访问和回源错误日志新增 ip=，默认记录 TCP 连接方的 IP。
同机原生 Caddy 部署可在 /etc/fango.env 配置：

```dotenv
ACCESS_LOG=true
TRUSTED_PROXIES=127.0.0.1/32,::1/128
```

修改后 systemctl restart fango。只有来自明确可信代理的连接才解析 X-Forwarded-For，按链条从右向左取首个不可信节点，防止直接访客伪造头部。
IP/CIDR 用逗号分隔；格式无效时拒绝启动。留空则不信任任何代理。
字段不会写入 Cookie、Authorization、URL 或请求正文。
ACCESS_LOG=false 时回源错误日志仍包含 IP。
Docker 场景需填写真实代理来源，不要盲目把整个公网或不受控网段设为可信。
公网 CDN 前置时还需在 Caddy 端配置 CDN 信任边界。
详见 [国内服务器离线安装及 IP 日志](china-offline-install.md)。
