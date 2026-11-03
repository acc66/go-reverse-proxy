# 国内服务器：另一台服务器下载，再上传离线安装

适用于目标服务器访问 GitHub 慢或超时，但另一台服务器可以下载的情况。
目标机使用 Debian/Ubuntu 和 systemd，无需 Docker，不依赖 Docker Hub。
下面以 Linux amd64、v1.3.0 为例。目标机 uname -m 为 aarch64 时，将 amd64 换成 arm64。

## 1. 在下载服务器准备文件

下面命令均在能访问 GitHub 的服务器执行：

```bash
mkdir -p /root/fango-package
cd /root/fango-package
curl -fL --retry 3 --connect-timeout 20 --max-time 1800   https://github.com/acc66/go-reverse-proxy/releases/download/v1.3.0/go-reverse-proxy-v1.3.0-linux-amd64.tar.gz   -o go-reverse-proxy-v1.3.0-linux-amd64.tar.gz &&
curl -fL --retry 3   https://github.com/acc66/go-reverse-proxy/releases/download/v1.3.0/SHA256SUMS.txt   -o SHA256SUMS.txt
```

下载三个管理/安装文件：

```bash
for file in install.sh fango fango-sites.py; do
  curl -fL --retry 3     "https://raw.githubusercontent.com/acc66/go-reverse-proxy/main/scripts/$file"     -o "$file" || break
done
ls -lh install.sh fango fango-sites.py SHA256SUMS.txt go-reverse-proxy-v1.3.0-linux-amd64.tar.gz
```

校验程序包（必须得到 OK）：

```bash
grep '  go-reverse-proxy-v1.3.0-linux-amd64.tar.gz$' SHA256SUMS.txt | sha256sum -c -
```

如果大文件中途超时，可使用相同 URL 和输出文件加 curl -C - 继续下载；继续前不要覆盖已有部分文件。
安装器仍会再次验证 SHA256，损坏文件不会被安装。

## 2. 上传到国内服务器

在下载服务器执行，把占位 IP 换成目标公网 IP：

```bash
scp -r /root/fango-package root@国内服务器IP:/root/
```

SSH 使用非 22 端口（例如 2222）：

```bash
scp -P 2222 -r /root/fango-package root@国内服务器IP:/root/
```

使用私钥：

```bash
scp -i /安全路径/ssh私钥 -P 22 -r /root/fango-package root@国内服务器IP:/root/
```

首次连接核对目标服务器指纹。不要在聊天或教程中填写真实密码、私钥、AccessKey。
若只能用普通 SSH 用户，先上传到该用户家目录，再用 sudo 安装。

## 3. 在国内服务器离线安装

以下命令改为在国内目标服务器执行：

```bash
cd /root/fango-package
bash install.sh   --domain wp.xn--8prw66b70hsx0c.com   --upstream https://api10.seox9.com/   --version v1.3.0   --archive /root/fango-package/go-reverse-proxy-v1.3.0-linux-amd64.tar.gz   --checksums /root/fango-package/SHA256SUMS.txt
```

域名替换成自己的，提前把 A/AAAA 记录指向目标机，并放行 TCP 80/443。
三个脚本齐全时，安装器不会从 GitHub 下载程序或管理组件。
仍然需要连接系统 apt 软件源安装依赖，以及连接证书签发服务申请 HTTPS 证书。

已有配置会保留，显式 --upstream 会更新原回源；已有缓存和超时不会被覆盖。
旧 go-reverse-proxy 服务会迁移为 fango。配置改用 /etc/fango.env。

## 4. 查看状态、日志与 IP

```bash
fango
systemctl is-active fango
journalctl -u fango -n 100 --no-pager
journalctl -u caddy -n 100 --no-pager
```

v1.3.0 起，访问日志和回源错误日志包含 ip= 字段。
同机原生 Caddy → fango（127.0.0.1:8080）部署，在 /etc/fango.env 中配置：

```dotenv
ACCESS_LOG=true
TRUSTED_PROXIES=127.0.0.1/32,::1/128
```

新安装默认写入；旧配置不会自动覆盖，需要用菜单 7 补上，然后应用。
不要重复添加同名变量。

```bash
systemctl restart fango
journalctl -u fango -f
```

示例（文档示例 IP）：

```text
access ip=198.51.100.9 method=GET status=200 duration_ms=208 bytes=821 cache=BYPASS
upstream request failed ip=198.51.100.9 method=GET upstream=api10.seox9.com error=...
```

只有实际连接来自 TRUSTED_PROXIES 指定地址时，才从 X-Forwarded-For 链右侧向左识别访客 IP。
否则记录直接连接者的地址，忽略伪造的转发头。空配置不信任任何转发头。
不要配置 0.0.0.0/0 或 ::/0 来信任所有来源。
如果 Caddy 前面还有 CDN，可能只能看到 CDN 地址；需在 Caddy 端单独正确配置该 CDN 的可信来源。
Docker 部署里 Caddy 的来源可能是 Docker 内网地址，不能照搬 loopback 配置；仅信任真实代理地址或隔离的受控网段。
当前 TRUSTED_PROXIES 用于日志 IP 识别，不改变向源站传递的 X-Forwarded-* 行为。
独立域名实例使用 /etc/fango/fango-site-*.env；旧实例需分别补充该变量并重启对应服务。

## 5. 后续离线升级

重复下载并上传对应版本归档和 SHA256SUMS.txt，然后在国内服务器执行：

```bash
fango offline-upgrade v1.3.0   /root/fango-package/go-reverse-proxy-v1.3.0-linux-amd64.tar.gz   /root/fango-package/SHA256SUMS.txt
```

升级程序不会自动改已有环境配置。安装器、菜单和多域名组件更新请重新上传三个脚本并运行安装器。
脚本已实现校验、失败回滚，细节见 [管理教程](manager.md)。
