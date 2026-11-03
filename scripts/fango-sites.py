#!/usr/bin/env python3
"""Manage exact-domain Caddy routes and optional isolated fango services."""
import argparse
import contextlib
import hashlib
import json
import os
import pathlib
import re
import socket
import subprocess
import sys
import time
from urllib.parse import urlsplit

def valid_domain(value):
    value = value.lower().rstrip(".")
    labels = value.split(".")
    if len(value) > 253 or len(labels) < 2 or any(
        not re.fullmatch(r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?", label)
        for label in labels
    ):
        raise ValueError("域名格式错误，请填写 ASCII/Punycode 域名，不带协议、端口或路径")
    return value

def valid_upstream(value):
    u = urlsplit(value)
    if not re.fullmatch(r"https?://[A-Za-z0-9./:_?=%@+\-]+", value) or not u.hostname or u.username or u.password:
        raise ValueError("回源必须是无凭据、无空白的 http(s) 地址")
    return value

class Sites:
    def __init__(self, root="/", run=None):
        self.root = pathlib.Path(root)
        self.run = run or self.command
        self.main = self.path("etc/caddy/Caddyfile")
        self.state = self.path("etc/fango/sites.json")
        self.base_env = self.path("etc/fango.env")

    def path(self, name):
        return self.root / name

    @staticmethod
    def command(args):
        return subprocess.check_output(args, text=True, stderr=subprocess.PIPE)

    def load(self):
        return json.loads(self.state.read_text()) if self.state.exists() else {}

    @staticmethod
    def write(path, text, mode=0o600):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)
        path.chmod(mode)

    def files(self, domain):
        key = hashlib.sha256(domain.encode()).hexdigest()[:20]
        unit = "fango-site-" + key
        return (
            self.path("etc/caddy/fango.d/" + domain + ".caddy"),
            self.path("etc/fango/" + unit + ".env"),
            self.path("etc/systemd/system/" + unit + ".service"),
            unit,
        )

    @contextlib.contextmanager
    def transaction(self, paths, unit, active):
        snapshots = {}
        for p in paths:
            if p.is_symlink():
                raise ValueError("拒绝修改符号链接配置：" + str(p))
            snapshots[p] = (p.read_bytes(), p.stat().st_mode & 0o777) if p.exists() else None
        try:
            yield
        except Exception:
            if not active:
                # Stop a new service before removing its unit file.
                try:
                    self.run(["systemctl", "disable", "--now", unit])
                except Exception:
                    pass
            for p, old in snapshots.items():
                if old is None:
                    p.unlink(missing_ok=True)
                else:
                    p.parent.mkdir(parents=True, exist_ok=True)
                    p.write_bytes(old[0])
                    p.chmod(old[1])
            for cmd in [
                ["systemctl", "daemon-reload"],
                ["systemctl", "restart" if active else "disable", *( [] if active else ["--now"] ), unit],
                ["systemctl", "reload", "caddy"],
            ]:
                try:
                    self.run(cmd)
                except Exception:
                    pass
            raise

    def main_port(self):
        for line in self.base_env.read_text().splitlines():
            if line.startswith("LISTEN_ADDR="):
                match = re.fullmatch(r"(?:127\.0\.0\.1)?:([0-9]+)", line.split("=", 1)[1])
                if not match:
                    raise ValueError("主实例 LISTEN_ADDR 需为 127.0.0.1:端口 或 :端口")
                port = int(match[1])
                if 1 <= port <= 65535:
                    return port
                raise ValueError("主实例端口无效")
        return 8080

    def free_port(self, state):
        used = {item["port"] for item in state.values()} | {self.main_port()}
        for port in range(18080, 19000):
            if port in used:
                continue
            with socket.socket() as s:
                try:
                    s.bind(("127.0.0.1", port))
                    return port
                except OSError:
                    pass
        raise ValueError("没有可用端口")

    def hosts(self):
        config = json.loads(self.run(["caddy", "adapt", "--config", str(self.main), "--adapter", "caddyfile"]))
        found = set()
        def walk(node):
            if isinstance(node, dict):
                for k, v in node.items():
                    if k == "host" and isinstance(v, list):
                        found.update(v)
                    walk(v)
            elif isinstance(node, list):
                for v in node:
                    walk(v)
        walk(config)
        return found

    def validate_reload(self):
        self.run(["caddy", "validate", "--config", str(self.main), "--adapter", "caddyfile"])
        self.run(["systemctl", "reload", "caddy"])

    def add(self, domain, upstream=None):
        domain = valid_domain(domain)
        if upstream:
            valid_upstream(upstream)
        state = self.load()
        old = state.get(domain)
        route, env, unit_file, unit = self.files(domain)
        if not old:
            hosts = self.hosts()
            if domain in hosts or any(h.startswith("*.") and domain.endswith(h[1:]) for h in hosts):
                raise ValueError("域名已在原 Caddy 配置中存在；保留原配置，不重复添加")
            if route.exists() or env.exists() or unit_file.exists():
                raise ValueError("发现未登记的同名配置，请先人工检查")
        elif route.read_text() != old["route"]:
            raise ValueError("域名文件已被手动修改，请先人工检查")
        port = (old["port"] if old and old["upstream"] else self.free_port(state)) if upstream else self.main_port()
        text = "# Managed by fango\n" + domain + " {\n    reverse_proxy 127.0.0.1:" + str(port) + "\n}\n"
        if old and old["upstream"] == upstream and old["route"] == text:
            return "域名已存在且配置相同，无需重复添加"
        active = bool(old and old["upstream"])
        with self.transaction([self.main, self.state, route, env, unit_file], unit, active):
            main_text = self.main.read_text()
            directive = "import /etc/caddy/fango.d/*.caddy"
            if directive not in main_text.splitlines():
                self.write(self.main, main_text.rstrip() + "\n\n" + directive + "\n", 0o644)
            self.write(route, text, 0o644)
            if upstream:
                lines = [line for line in self.base_env.read_text().splitlines()
                         if not line.startswith(("UPSTREAM=", "LISTEN_ADDR="))]
                lines += ["UPSTREAM=" + upstream, "LISTEN_ADDR=127.0.0.1:" + str(port)]
                self.write(env, "\n".join(lines) + "\n")
                self.write(unit_file, "[Unit]\nDescription=Fango site " + domain +
                    "\nAfter=network-online.target\nWants=network-online.target\n[Service]\nDynamicUser=yes\nEnvironmentFile=" +
                    str(env) + "\nExecStart=/opt/fango/fango\nRestart=on-failure\nRestartSec=3\nNoNewPrivileges=yes\nProtectSystem=strict\nProtectHome=yes\nPrivateTmp=yes\n[Install]\nWantedBy=multi-user.target\n", 0o644)
                self.run(["systemctl", "daemon-reload"])
                self.run(["systemctl", "restart", unit])
                time.sleep(0.5)
                self.run(["systemctl", "is-active", "--quiet", unit])
            self.validate_reload()
            if upstream:
                self.run(["systemctl", "enable", unit])
            elif active:
                self.run(["systemctl", "disable", "--now", unit])
                env.unlink(missing_ok=True)
                unit_file.unlink(missing_ok=True)
                self.run(["systemctl", "daemon-reload"])
            state[domain] = {"upstream": upstream, "port": port, "unit": unit if upstream else None, "route": text}
            self.write(self.state, json.dumps(state, indent=2) + "\n")
        return "已添加/更新 " + domain + "；证书签发状态请查看 Caddy 日志"

    def remove(self, domain):
        domain = valid_domain(domain)
        state = self.load()
        old = state.get(domain)
        if not old:
            raise ValueError("不是本工具管理的域名，未删除任何配置")
        route, env, unit_file, unit = self.files(domain)
        if route.read_text() != old["route"]:
            raise ValueError("域名配置已手动修改，拒绝删除")
        with self.transaction([self.state, route, env, unit_file], unit, bool(old["upstream"])):
            route.unlink()
            self.validate_reload()
            if old["upstream"]:
                self.run(["systemctl", "disable", "--now", unit])
                env.unlink(missing_ok=True)
                unit_file.unlink(missing_ok=True)
                self.run(["systemctl", "daemon-reload"])
            del state[domain]
            self.write(self.state, json.dumps(state, indent=2) + "\n")
        return "已移除 " + domain + "；证书数据保留"

def main():
    if os.geteuid() != 0:
        raise SystemExit("请使用 sudo fango")
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["domain-add", "domain-remove", "domain-list"])
    parser.add_argument("domain", nargs="?")
    parser.add_argument("--upstream")
    args = parser.parse_args()
    import fcntl
    with open("/run/lock/fango-update.lock", "w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        sites = Sites()
        if args.action == "domain-list":
            for domain, info in sites.load().items():
                print(domain, "->", info["upstream"] or "主实例（跟随 fango set-upstream）",
                      "端口", info["port"], "服务", info["unit"] or "fango")
        elif not args.domain:
            parser.error("请指定域名")
        elif args.action == "domain-add":
            print(sites.add(args.domain, args.upstream))
        else:
            print(sites.remove(args.domain))
if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print("操作失败：" + str(error), file=sys.stderr)
        if isinstance(error, subprocess.CalledProcessError) and error.stderr:
            print(error.stderr, file=sys.stderr)
        sys.exit(1)
