# 一键安装与数字管理菜单（Linux/systemd）

适用于 Debian/Ubuntu 的 amd64、arm64 主机，使用原生 Go 程序和 Caddy，不依赖 Docker Hub。
服务器需要能够访问 Debian/Ubuntu 软件源；在线安装还需要访问 GitHub。脚本需要 root。
如果已运行 Docker 版，请先停止对应容器释放 80、443、8080，避免两套部署冲突。

## 在线安装

下载两个脚本到同一目录（建议先查看内容，再执行）：

```bash
mkdir -p /root/proxy-installer
cd /root/proxy-installer
curl -fL --retry 2 https://raw.githubusercontent.com/acc66/go-reverse-proxy/main/scripts/install.sh -o install.sh &&
curl -fL --retry 2 https://raw.githubusercontent.com/acc66/go-reverse-proxy/main/scripts/fango -o fango &&
bash install.sh   --domain wp.xn--8prw66b70hsx0c.com   --upstream https://api10.seox9.com/
```

请替换为你自己的域名和源站。域名 A 记录应指向本机，TCP 80/443 放行。
安装器下载最新稳定 Release，校验 SHA256，安装 systemd 服务并设置开机启动。
Caddy 自动申请和续期证书，真正签发成功需查看菜单 9 的日志。
没有改动本机 DNS、安全组或防火墙；这些需要按部署环境配置。
不需要 HTTPS 时添加 --no-https，程序默认仍只监听 127.0.0.1:8080。

新安装默认配置：

```dotenv
UPSTREAM=https://api10.seox9.com/
LISTEN_ADDR=127.0.0.1:8080
CACHE_BYPASS_PATHS=/auth,/password,/payment,/res,/v1,/sublink,/wordpress,/user
UPSTREAM_CONNECT_TIMEOUT=10s
UPSTREAM_TLS_TIMEOUT=15s
UPSTREAM_HEADER_TIMEOUT=60s
ACCESS_LOG=true
```

源站地址来自你传入的 --upstream，其余默认值如上。

## 已有安装

安装器保留已有缓存、监听地址和超时设置；显式指定 --upstream 时会备份配置并更新回源地址。
不传 --upstream 则保留原回源地址。也可以使用 fango set-upstream https://api10.seox9.com/ 单独更新并重启，启动检查失败自动恢复配置。
保留已有 systemd 单元；若它执行的程序不在 /opt/fango/fango，安装器会提示停止。
Caddyfile 若已出现指定域名则保留，不擅自替换网站配置。
否则创建 /etc/caddy/fango.d/site.caddy 并在 Caddyfile 添加 import；添加前备份主配置。
如已有复杂的 import、通配域名或其他反代，请先人工检查 Caddy 配置，脚本不能识别所有复杂配置。
不会删除证书数据。已有 LISTEN_ADDR 必须与 Caddy 的 127.0.0.1:8080 回源一致。

## 数字菜单

安装后执行：

```bash
fango
```

| 输入 | 操作 |
| --- | --- |
| 1 | 服务状态和安装版本 |
| 2 | 实时访问与错误日志 |
| 3 | 最近一小时的错误和 4xx/5xx 日志 |
| 4 | 重启 Go 代理 |
| 5 | 启动 Go 代理并启用开机启动 |
| 6 | 停止 Go 代理 |
| 7 | nano 编辑配置，应用失败恢复原配置 |
| 8 | 在线升级到最新稳定发行版 |
| 9 | Caddy、证书申请和续期日志 |
| 10 | 修改回源地址并应用 |
| 11 | 卸载 Go 代理 |
| 0 | 退出菜单 |

实时查看日志时 Ctrl+C 退出跟踪；不会停止代理。需要时重新运行 fango。
菜单 6 只停止 Go，不停止 Caddy；停止后网站可能返回 502。
配置文件位于 /etc/fango.env。
访问日志由 ACCESS_LOG 控制，错误日志始终开启。systemd 日志由系统 journald 策略管理，菜单不擅自改变全机日志保留策略。

已有服务器如需追加指定路径，用菜单 7 把该行设为：

```dotenv
CACHE_BYPASS_PATHS=/auth,/password,/payment,/res,/v1,/sublink,/wordpress,/user
```

## 国内网络：离线安装

在能访问 GitHub 的电脑下载：
1. scripts/install.sh 和 scripts/fango，放在同一目录；
2. Release 中对应的 linux-amd64 或 linux-arm64 tar.gz；
3. 同一个 Release 的 SHA256SUMS.txt。

通过 SFTP 上传到服务器，例如 /root/proxy-installer。
以 v1.2.0 amd64 为例：

```bash
cd /root/proxy-installer
bash install.sh   --domain wp.xn--8prw66b70hsx0c.com   --upstream https://api10.seox9.com/   --version v1.2.0   --archive ./go-reverse-proxy-v1.2.0-linux-amd64.tar.gz   --checksums ./SHA256SUMS.txt
```

离线方式不从 GitHub下载程序，但仍需要 apt 软件源安装依赖；HTTPS 仍需连通证书服务。
不要改归档文件名或校验文件中的对应名称。

## 在线与离线升级

菜单 8 或：

```bash
fango upgrade
```

指定版本（也可用于明确降级）：

```bash
fango upgrade v1.2.0
```

离线升级：

```bash
fango offline-upgrade v1.2.0   /root/proxy-installer/go-reverse-proxy-v1.2.0-linux-amd64.tar.gz   /root/proxy-installer/SHA256SUMS.txt
```

下载或校验失败时不替换当前程序。
替换时保留上一份程序，原子替换后重启；若服务启动检查失败，则恢复旧程序并尝试重启。
检查仅确认进程处于 active，不保证外部 DNS、证书、源站和业务登录正常。
升级会短暂中断请求并清空内存缓存；没有自动重试业务请求。
安装与升级使用文件锁，防止两次操作同时替换程序。
升级只更新 Go 二进制，不自动升级管理脚本或 Caddy，也不覆盖配置。
管理脚本更新可重新下载两个脚本，检查后重新运行安装器。
Caddy 通过系统包管理维护，证书由 Caddy 自动续期，不需要通过菜单升级证书。

## 排障

```bash
journalctl -u fango -n 100 --no-pager
journalctl -u caddy -n 100 --no-pager
systemctl cat fango
curl -sS -D - -o /dev/null --max-time 90 http://127.0.0.1:8080/
```

GitHub 连接重置时使用离线安装/升级，不要不断重装 Docker。
更多：[回源超时与日志](timeouts-and-logs.md) · [缓存规则](cache-settings.md) · [完整安装](installation.md)

## 从旧 go-reverse-proxy 迁移到 fango

默认 systemd 服务名为 fango，后台程序路径为 /opt/fango/fango，管理命令为 fango。
发行归档仍叫 go-reverse-proxy，安装器会将其中的程序安装为 fango，不需要手动改归档名。

如果 /etc/fango.env 不存在且检测到 /etc/go-reverse-proxy.env，会复制旧配置。
安装器先停止旧服务，再启动 fango，成功后禁用旧服务开机启动。
迁移过程中失败会尝试停止 fango 并恢复原来正在运行的旧服务。
旧服务文件、旧程序和旧配置保留作恢复用途，但不要手动同时启动新旧服务。
旧的受管理 proxyctl 命令会转换为 fango 的兼容入口。
Caddy 的 127.0.0.1:8080 回源无需改变。

显式更新已有回源：

```bash
fango set-upstream https://api10.seox9.com/
```

查看新服务：

```bash
systemctl status fango --no-pager
journalctl -u fango -n 100 --no-pager
grep '^UPSTREAM=' /etc/fango.env
```

迁移后编辑 /etc/fango.env，旧 /etc/go-reverse-proxy.env 不再是当前服务配置。
配置中写 https://，不需要写成 https\://。

## 卸载

菜单输入 11，或执行：

```bash
fango uninstall
```

命令会说明范围，输入 UNINSTALL 确认。无需交互时：

```bash
fango uninstall --yes
```

默认停止并禁用 fango，删除对应 systemd 单元、已知程序文件和管理命令。
保留 /etc/fango.env、配置备份、历史系统日志、Caddy、站点配置及证书，避免误删其他网站。
仅删除本工具明确管理的文件，不递归删除含有其他文件的程序目录。
旧迁移备份文件和已禁用的 go-reverse-proxy 服务不自动清理。

如需同时删除当前 fango 配置：

```bash
fango uninstall --yes --purge-config
```

--purge-config 仅额外删除 /etc/fango.env，不删除备份或证书。
卸载后保留的 Caddy 站点可能返回 502。如果不再使用该域名，请单独移除对应站点，验证并重载 Caddy。
不自动停止或卸载 Caddy，因为同机可能还有其他网站。