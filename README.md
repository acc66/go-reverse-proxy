# Go Reverse Proxy

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
