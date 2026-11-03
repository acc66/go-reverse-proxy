#!/usr/bin/env bash
# Debian/Ubuntu native installer; no Docker required.
set -Eeuo pipefail
HERE=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
[[ -f "$HERE/fango" ]] || { echo "请把 install.sh 和 fango 放在同一目录。"; exit 1; }
source "$HERE/fango"
need_root
[[ $(uname -s) == Linux && -d /run/systemd/system ]] || { echo "需要使用 systemd 的 Linux 主机"; exit 1; }
DOMAIN= UPSTREAM= VERSION=latest ARCHIVE= SUMS= HTTPS=true
while [[ $# -gt 0 ]]; do
 case "$1" in
  --domain|--upstream|--version|--archive|--checksums)
   [[ $# -ge 2 ]] || { echo "$1 缺少参数"; exit 1; }
   case "$1" in
    --domain) DOMAIN=$2;; --upstream) UPSTREAM=$2;; --version) VERSION=$2;;
    --archive) ARCHIVE=$2;; --checksums) SUMS=$2;;
   esac
   shift 2 ;;
  --no-https) HTTPS=false; shift ;;
  *) echo "参数：--domain 域名 --upstream https://源站 --version v1.2.0 --archive 归档 --checksums 校验文件 --no-https"; exit 1 ;;
 esac
done
# Adopt legacy settings without deleting the original file.
LEGACY_ENV=/etc/go-reverse-proxy.env
if [[ ! -f "$ENV_FILE" && -f "$LEGACY_ENV" ]]; then
 cp -p "$LEGACY_ENV" "$ENV_FILE"
 echo "已从旧配置迁移到 $ENV_FILE；原文件保留。"
fi
if [[ ! -f "$ENV_FILE" && -z $UPSTREAM ]]; then
 read -r -p "请输入回源网址，例如 https://api10.seox9.com/: " UPSTREAM
fi
if [[ $HTTPS == true ]]; then
 [[ -n $DOMAIN ]] || read -r -p "请输入访问域名，例如 wp.example.com: " DOMAIN
 [[ $DOMAIN =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ && $DOMAIN == *.* && $DOMAIN != *..* ]] || { echo "域名无效，不要带协议、空格或路径。"; exit 1; }
fi
if [[ -n $UPSTREAM ]]; then
 [[ $UPSTREAM =~ ^https?://[A-Za-z0-9./:_?=%@+-]+$ ]] || { echo "回源网址无效，请填写简单 http(s) 地址。"; exit 1; }
fi
if [[ -n $ARCHIVE || -n $SUMS ]]; then
 [[ -f $ARCHIVE && -f $SUMS && $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "离线安装需同时提供 --archive、--checksums 和明确的 --version。"; exit 1; }
fi
command -v apt-get >/dev/null || { echo "此安装器支持 Debian/Ubuntu。"; exit 1; }
exec 9>/run/lock/fango-update.lock
flock -n 9 || { echo "另一个安装或升级正在运行。"; exit 1; }
apt-get update
apt-get install -y ca-certificates curl python3 nano
if [[ $HTTPS == true ]]; then apt-get install -y caddy; fi
work=$(mktemp -d)
legacy_restart=false
cleanup() {
 local result=$?
 if [[ $legacy_restart == true ]]; then
  systemctl stop fango || true
  systemctl start go-reverse-proxy || true
 fi
 rm -rf -- "$work"
 return "$result"
}
trap cleanup EXIT
arch=$(architecture)
if [[ -n $ARCHIVE ]]; then
 verify_extract "$ARCHIVE" "$SUMS" "go-reverse-proxy-$VERSION-linux-$arch.tar.gz" "$work/go-reverse-proxy"
 printf '%s\n' "$VERSION" > "$work/VERSION"
else
 download_binary "$work" "$VERSION" "$arch"
fi
if [[ ! -f "$ENV_FILE" ]]; then
 umask 077
 cat > "$ENV_FILE" <<EOF
UPSTREAM=$UPSTREAM
LISTEN_ADDR=127.0.0.1:8080
CACHE_BYPASS_PATHS=$DEFAULT_BYPASS
UPSTREAM_CONNECT_TIMEOUT=10s
UPSTREAM_TLS_TIMEOUT=15s
UPSTREAM_HEADER_TIMEOUT=60s
ACCESS_LOG=true
EOF
 umask 022
else
 echo "保留已有配置；若指定 --upstream，将在安装成功后仅更新回源地址。"
fi
if [[ ! -f /etc/systemd/system/fango.service ]]; then
 cat > /etc/systemd/system/fango.service <<'EOF'
[Unit]
Description=Fango Reverse Proxy
Wants=network-online.target
After=network-online.target
[Service]
DynamicUser=yes
EnvironmentFile=/etc/fango.env
ExecStart=/opt/fango/fango
Restart=on-failure
RestartSec=3
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
[Install]
WantedBy=multi-user.target
EOF
fi
systemctl daemon-reload
actual_exec=$(systemctl show "$SERVICE" --property=ExecStart --value)
[[ $actual_exec == *"/opt/fango/fango"* ]] || { echo "已有服务使用不同程序路径，请先检查 systemctl cat $SERVICE"; exit 1; }
if systemctl is-active --quiet go-reverse-proxy; then
 systemctl stop go-reverse-proxy
 legacy_restart=true
fi
deploy_binary "$work" true
systemctl enable "$SERVICE"
if [[ -n $UPSTREAM ]]; then apply_upstream "$UPSTREAM"; fi
# New process is healthy; keep legacy files for manual recovery but disable its autostart.
legacy_restart=false
if [[ $(systemctl show go-reverse-proxy --property=LoadState --value) != not-found ]]; then
 systemctl disable go-reverse-proxy
fi
install -m 755 "$HERE/fango" /usr/local/bin/fango
if [[ ! -e /usr/local/bin/proxyctl ]] || grep -q '^SERVICE=go-reverse-proxy$' /usr/local/bin/proxyctl; then
 ln -sfn /usr/local/bin/fango /usr/local/bin/proxyctl
fi
if [[ $HTTPS == true ]]; then
 mkdir -p /etc/caddy/proxyctl.d
 touch /etc/caddy/Caddyfile
 if grep -Fq "$DOMAIN" /etc/caddy/Caddyfile; then
  echo "Caddyfile 已包含 $DOMAIN，保留现有站点配置。"
 else
  if [[ ! -f /etc/caddy/proxyctl.d/site.caddy ]]; then
   printf '%s {\n    reverse_proxy 127.0.0.1:8080\n}\n' "$DOMAIN" > /etc/caddy/proxyctl.d/site.caddy
  else
   grep -Fq "$DOMAIN {" /etc/caddy/proxyctl.d/site.caddy || { echo "已有不同域名配置，请检查 /etc/caddy/proxyctl.d/site.caddy"; exit 1; }
  fi
  if ! grep -Fxq 'import /etc/caddy/proxyctl.d/*.caddy' /etc/caddy/Caddyfile; then
   cp -p /etc/caddy/Caddyfile "/etc/caddy/Caddyfile.before-proxyctl.$(date +%s)"
   printf '\nimport /etc/caddy/proxyctl.d/*.caddy\n' >> /etc/caddy/Caddyfile
  fi
 fi
 caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
 systemctl enable --now caddy
 systemctl reload caddy
 echo "访问 https://$DOMAIN；请确保 DNS 指向本机、TCP 80/443 放行。"
 echo "证书实际签发结果请看菜单 9；不以配置加载成功代替签发成功。"
fi
echo "进程安装完成。输入 fango 打开菜单。已有日志和证书数据未清除。"
