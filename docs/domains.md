# 在线添加多个反代域名

更新安装器后运行 fango，菜单 12 添加/更新、13 查看、14 删除。
也可以直接使用以下命令，无需重装 Go 或手工编辑 Caddy。

## 多个域名共用当前回源

```bash
fango domain-add a.example.com
fango domain-add b.example.com
```

两个域名转发到主 fango 实例，跟随 /etc/fango.env 的 UPSTREAM。
随后 fango set-upstream https://api10.seox9.com/ 会同时改变这些域名的回源。
重复添加同一个域名且参数相同不会创建重复站点或重启代理。

## 不同域名使用不同回源

```bash
fango domain-add api.example.com --upstream https://api10.seox9.com/
fango domain-add other.example.com --upstream https://other-origin.example.com/
```

每个独立回源域名创建独立 fango-site-* systemd 服务，自动分配 127.0.0.1:18080–18999 中空闲端口。
配置保存在 /etc/fango/fango-site-*.env。创建时复制主实例的缓存、超时和日志设置，后续主配置变化不自动同步这些独立配置。
各进程缓存互相隔离，每个进程有自己的内存开销，域名较多时注意服务器内存。
无需开放这些内部端口；公网仍只需 80/443。
菜单升级二进制时，会尝试重启正在运行的独立域名实例以使用新程序。

## 同一个域名改回源

```bash
fango domain-add api.example.com --upstream https://new-origin.example.com/
```

覆盖该托管域名的回源，复用原独立端口，不影响其他域名。
要改为跟随主实例：

```bash
fango domain-add api.example.com
```

同一个域名在同一配置中只有一个目标；目前不提供同域名按路径分流或多个源站负载均衡。

## 查看与移除

```bash
fango domain-list
fango domain-remove api.example.com
```

移除域名会重载 Caddy，并停止、移除该域名专属实例；不删除证书数据。
只管理通过这些命令创建的站点。已有安装器站点或手动站点不会自动导入。
若域名已在原 Caddy 配置中出现，命令会拒绝重复添加；编辑原站点，或备份后移除原定义，再使用 domain-add。
遇到通配站点冲突会拒绝操作，不自动覆盖。
卸载主程序前，需先逐个移除托管域名，防止留下无法运行的独立服务。

## DNS、证书和失败恢复

所有新增域名需先解析到服务器。域名参数不带协议/端口/路径；中文域名使用 Punycode。
Caddy 负责自动申请和续期证书：

```bash
journalctl -u caddy -n 100 --no-pager
```

添加成功表示配置通过验证并加载，不代表公网 DNS 或证书签发已成功。
新建/更新过程会检查配置及服务状态；Caddy 验证或重载失败会恢复原文件并尝试恢复原服务。
同机有复杂动态 Caddy 配置时，请先备份并人工检查。
管理文件位于 /etc/caddy/fango.d/*.caddy，登记文件为 /etc/fango/sites.json。
不要手动修改已登记的域名配置；检测到修改时会拒绝覆盖。
网站的绝对链接、Cookie Domain 和登录回调不会自动改写。

## 更新管理工具

在原安装目录下载最新的 install.sh、fango、fango-sites.py 三个文件后重新运行安装器。
已有部署也可仅更新两个管理文件（不修改回源和证书）：

```bash
mkdir -p /root/fango-update
cd /root/fango-update
curl -fL --retry 2 https://raw.githubusercontent.com/acc66/go-reverse-proxy/main/scripts/fango -o fango &&
curl -fL --retry 2 https://raw.githubusercontent.com/acc66/go-reverse-proxy/main/scripts/fango-sites.py -o fango-sites.py &&
install -m 644 fango-sites.py /opt/fango/fango-sites.py &&
install -m 755 fango /usr/local/bin/fango
```

以上要求已安装 fango，且 /opt/fango、/etc/fango.env 已存在。
国内网络无法下载时，在电脑下载两个文件后 SFTP 上传再执行 install 命令。
离线首次安装需同时上传三个管理/安装文件，以及 Release 归档和校验文件。
