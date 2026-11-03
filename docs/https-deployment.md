# HTTPS 反代部署与证书自动续期

适用于已有 Docker Engine、Docker Compose 插件和 Git 的 Linux 服务器。
示例：用户访问 https://aaaa.com，回源 https://seo.webshost.club/。
aaaa.com 是示例，请替换为你拥有且能管理 DNS 的真实域名。

访问路径：浏览器 → Caddy（HTTPS）→ Go（内网 HTTP、静态缓存）→ 源站（HTTPS）。

## 1. DNS 和端口

- 将访问域名的 A 记录解析到代理服务器公网 IPv4。
- 仅在 IPv6 可用并可入站访问时配置 AAAA，否则删除错误记录。
- 云安全组、防火墙、NAT 必须允许公网 TCP 80、443 到代理服务器。
- UDP 443 用于 HTTP/3，可选；证书申请不依赖 UDP。
- 确保 80、443 没被其他 Nginx、Apache 或容器占用。
- 回源域名应指向源站，不能再指向本代理，避免请求循环。
- 初次部署可先使用仅 DNS 解析模式，避免前置 CDN 干扰验证。

## 2. 下载与配置

首次安装：

```bash
git clone https://github.com/acc66/go-reverse-proxy.git
cd go-reverse-proxy
cp .env.https.example .env
nano .env
```

已有仓库：进入目录执行 git pull。保留已有 .env，直接编辑并补上 DOMAIN，不要覆盖自定义配置。

.env 示例：

```dotenv
DOMAIN=aaaa.com
UPSTREAM=https://seo.webshost.club/
```

DOMAIN 只填写域名，不加 https:// 或路径。UPSTREAM 需要完整 http(s) 地址。
.env 不应提交到 Git。

仓库已提供 Caddyfile 和 compose.https.yaml，无需手动复制 YAML。
Caddyfile 用 DOMAIN 配置自动 HTTPS，并转发到 Docker 内网 proxy:8080。
Go 不向宿主机发布 8080 端口。

## 3. 停止旧服务并启动 HTTPS

如果旧服务由之前的 docker run 创建：

```bash
docker stop seo-proxy
```

如果旧服务由默认 compose.yaml 创建，在其项目目录执行：

```bash
docker compose down
```

以上按实际部署方式二选一；首次安装直接跳过。
停止旧服务到新服务就绪期间会短暂停机。

检查配置并启动（显式指定独立 HTTPS 配置，不要与默认配置合并）：

```bash
docker compose -f compose.https.yaml config --quiet
docker compose -f compose.https.yaml up -d --build
docker compose -f compose.https.yaml logs --tail 100 caddy
```

访问 https://你的域名。HTTP 自动跳转 HTTPS。
证书签发要求 DNS、公网验证端口和出站网络正常；CAA 若有限制，还需允许所选 CA。

## 4. 自动续期与维护

Caddy 自动申请、管理和续期证书，无需 cron 或手动上传证书。
保持服务运行、域名有效、验证路径可用，并保留 caddy_data 卷。

不要使用 docker compose down -v，不要删除证书数据卷。
普通容器重启、重建不会删除命名卷。迁移服务器时要妥善迁移包含私钥的数据卷。
项目目录名影响默认卷名前缀；不要随意改变项目名后误以为证书数据仍被挂载。

查看状态：

```bash
docker compose -f compose.https.yaml ps
docker compose -f compose.https.yaml logs --tail 100 caddy
docker compose -f compose.https.yaml logs --tail 100 proxy
```

更新源码并重新构建：

```bash
git pull
docker compose -f compose.https.yaml up -d --build
```

更新 Caddy 镜像：

```bash
docker compose -f compose.https.yaml pull caddy
docker compose -f compose.https.yaml up -d
```

清空 Go 内存缓存：

```bash
docker compose -f compose.https.yaml restart proxy
```

## 5. 静态资源和 API 缓存

当前 Go 实现只缓存明确公开的静态文件，不缓存 API。
源站应对确认公开的静态文件返回 Cache-Control: public, max-age=3600。
不要全站添加此响应头，也不要仅凭扩展名判断文件是否公开。

| 内容 | 策略 |
| --- | --- |
| 公开 CSS、JS、字体、普通图片 | 可以缓存 |
| /api/v1/passport/* | 登录、注册、密码找回，不缓存 |
| /api/v1/user/* | 订阅、订单、流量、工单等，不缓存 |
| /user/*、/admin/*、/auth/*、/password/* | 不缓存 |
| /payment/*、支付回调 | 不缓存 |
| /sublink、/v1/link、订阅内容 | 不缓存 |
| /res/captcha/* | 验证码，即使是图片也不能缓存 |
| /api/v1/guest/comm/config | 公开配置可单独评估短缓存，当前仍回源 |

GET 不等于可缓存：某些业务通过 GET 签到或生成邀请。
套餐和公告接口如果需要登录，也不要直接作为公共缓存。

以上是部署策略，不代表所有路径都单独写进 Go 黑名单。
Go 还要求静态扩展名、明确公开缓存头及无凭据等条件；不要在源站误标动态响应为 public。
单文件上限 2 MiB、条目预算约 64 MiB、缓存最长 1 小时；重启清空。

页面 assets_url 若指向其他域名，浏览器直接访问那个域名，不经过本代理。
带 Cookie 的静态请求也会绕过缓存。当前版本拒绝缓存任何带 Vary 的响应（包括 Vary: Accept-Encoding），因此部分资源不会命中。

连续执行两次以下 GET（替换成真实静态文件地址）：

```bash
curl -sS -D - -o /dev/null https://你的域名/实际静态文件.js
```

X-Proxy-Cache: HIT 表示命中；MISS 表示候选静态请求回源，不保证已存储；BYPASS 表示绕过。
不要用 curl -I 验证缓存命中，因为它发送 HEAD，当前只缓存 GET。

## 6. 登录跳转与限制

代理不会改写绝对链接、Location、Cookie Domain 或页面中的 API 域名。
源站若将公开地址固定为原域名，需调整应用配置并检查登录、订阅和支付回调。

当前 Go 重建 X-Forwarded-*，不会信任 Caddy 传来的原始转发头。
因此源站看到的转发协议可能为 http，客户端地址可能是 Caddy 的内网地址。
HTTPS 传输仍正常，但依赖原始访客 IP 或外部协议判断的应用需另行配置可信代理支持；
不要直接让应用无条件信任公网传来的转发头。

静态缓存不会解决登录接口 504。先区分 Caddy、Go 和源站哪一层超时。

## 参考

- https://caddyserver.com/docs/automatic-https
- https://hub.docker.com/_/caddy

本教程配置尚未在本机 Docker 环境实际运行验证。

## 自定义不缓存路径

在 .env 中增加或修改：

```dotenv
CACHE_BYPASS_PATHS=/auth,/password,/payment,/res,/v1,/sublink,/private
```

路径用英文逗号分隔，不写域名。/private 覆盖自身和子路径，也可以写 /private/*。
规则会追加到内置排除列表；设置 / 可关闭全部缓存。匹配源站路径，UPSTREAM 的路径前缀也应包含在规则内。
完整规则说明见 [README](../README.md#自定义不缓存路径)。

首次获取此功能需 git pull 并执行 docker compose -f compose.https.yaml up -d --build。
之后修改 .env，执行 docker compose -f compose.https.yaml up -d 即可；仅 restart 不会载入新环境变量。

逐步操作、docker run 用法与验证步骤见 [自定义不缓存设置教程](cache-settings.md)。
