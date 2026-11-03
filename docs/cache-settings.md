# 自定义不缓存设置教程

通过 CACHE_BYPASS_PATHS 配置，无需编辑 Go 源码。
以下命令在 Linux 服务器运行，进入此前下载的 go-reverse-proxy 目录。
请按实际部署方式选择 HTTPS Compose、HTTP Compose 或 docker run，不要同时启动多套服务占用同一端口。

## 一、HTTPS Compose 用户

### 1. 更新到支持自定义设置的版本

```bash
cd go-reverse-proxy
git pull
nano .env
```

保留已有 DOMAIN、UPSTREAM，在 .env 中添加或修改这一行；不要重复定义同一个变量：

```dotenv
CACHE_BYPASS_PATHS=/auth,/password,/payment,/res,/v1,/sublink,/private
```

nano 保存退出：Ctrl+O → 回车 → Ctrl+X。

首次升级代码后构建并应用：

```bash
docker compose -f compose.https.yaml up -d --build
```

### 2. 以后修改设置

```bash
nano .env
docker compose -f compose.https.yaml up -d
```

仅修改环境变量无需重新构建。Compose 会重建配置有变的代理容器，可能短暂中断请求，并清空其内存缓存。
不要只执行 restart，它不会重新读取 .env。

## 二、普通 HTTP Compose 用户

编辑同一份 .env。首次升级：

```bash
git pull
docker compose up -d --build
```

以后只修改 .env：

```bash
docker compose up -d
```

默认 compose.yaml 对外端口是 8080，访问 http://服务器IP:8080。

## 三、docker run 用户

docker run 不会自动读取项目的 .env。创建容器时使用 -e；已有容器需重新创建才能更改环境变量。

以下示例适用于之前名为 seo-proxy、对外监听 80 的 HTTP 容器。重建会短暂停机。
先构建成功，再删除旧容器：

```bash
cd go-reverse-proxy
git pull &&
docker build -t go-reverse-proxy:local . &&
docker rm -f seo-proxy &&
docker run -d \
  --name seo-proxy \
  --restart unless-stopped \
  -p 80:8080 \
  -e UPSTREAM=https://api10.seox9.com/ \
  -e "CACHE_BYPASS_PATHS=/auth,/password,/payment,/res,/v1,/sublink,/private" \
  --read-only \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  go-reverse-proxy:local
```

首次安装没有旧容器时，跳过 docker rm -f seo-proxy 那一行。
这套 docker run 示例不提供 HTTPS；需要自动证书时使用 [HTTPS 教程](https-deployment.md)。
保留你自己的回源地址和端口，不要直接覆盖自定义部署参数。

## 四、怎么填写规则

| 配置 | 效果 |
| --- | --- |
| /private | 不缓存 /private 和 /private/ 下全部子路径 |
| /private/* | 与 /private 相同 |
| /images/avatar.png | 不缓存该路径及其子路径 |
| /auth,/payment,/res | 添加多个排除路径 |
| / | 全部绕过缓存 |
| 留空或删除变量 | 仅移除自定义规则，保留内置保护 |

- 使用英文逗号分隔，不写域名、查询参数或片段。
- /private 不会误匹配 /private-other。
- 大小写不敏感；两端空格和尾部斜杠会被规范化。
- 不支持正则表达式或任意通配符，只兼容末尾 /*。
- 匹配的是回源 URL 路径。如果 UPSTREAM=https://example.com/base，而浏览器请求 /private/a.js，应配置 /base/private。
- 格式错误（例如填完整网址）会导致启动报错，请查看日志修正。

### 全部关闭缓存

```dotenv
CACHE_BYPASS_PATHS=/
```

按上面的部署方式应用配置即可。

### 恢复默认规则

```dotenv
CACHE_BYPASS_PATHS=
```

这不会强制缓存任何资源。内置 /api、/admin、/user、/wp-admin、/wp-login.php 仍绕过缓存。
带 Cookie、Authorization、条件请求、Range 等请求仍绕过，非 GET 不缓存。
源站 private/no-store/no-cache、Set-Cookie 等限制仍然有效。

## 五、确认设置生效

以 HTTPS Compose 为例，查看容器实际收到的配置：

```bash
docker compose -f compose.https.yaml exec proxy printenv CACHE_BYPASS_PATHS
docker compose -f compose.https.yaml logs --tail 50 proxy
```

使用真实存在的文件路径验证。例如配置排除 /private 后：

```bash
curl -sS -D - -o /dev/null https://你的域名/private/app.js
```

预期响应头：

```text
X-Proxy-Cache: BYPASS
```

不要用 curl -I 验证 GET 缓存，因为 HEAD 本来就不缓存。
BYPASS 也可能由 Cookie 等其他条件触发，因此同时检查容器中的配置值。
若域名前还有 CDN，它可能直接返回自己的缓存；需清理该资源的 CDN 缓存或直接验证本代理。

| 响应头 | 意义 |
| --- | --- |
| HIT | 本次命中 Go 内存缓存 |
| MISS | 候选静态请求回源，不保证响应最终存入缓存 |
| BYPASS | 本次绕过 Go 缓存 |

想验证正常缓存，选一个未排除的公开静态资源，连续 GET 两次。
源站必须返回 public 和有效 max-age/s-maxage，文件不超过 2 MiB，且没有其他绕过条件；仅 Vary: Accept-Encoding 可缓存，其他 Vary 响应不缓存。

## 六、常见问题

- 修改 .env 没生效：执行对应 Compose 的 up -d，不是 restart；检查是否位于正确项目目录。
- 日志提示 invalid CACHE_BYPASS_PATHS：检查是否填了网址、问号、非法通配符或中文逗号。
- 登录接口仍然 504：不缓存设置不能修复回源超时，需检查网络和源站处理时间。
- 静态文件一直 MISS：检查 Cache-Control、Set-Cookie、Vary、大小及容量限制。
- 设置只控制本 Go 代理，不会自动清理或禁止浏览器、CDN 中的缓存。敏感响应仍应由源站返回合适的 Cache-Control。
