#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/.."
source scripts/proxyctl
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
APP_DIR="$work/app"
mkdir -p "$work/source"
printf '\177ELFtest-new' > "$work/source/go-reverse-proxy"
echo v1.2.0 > "$work/source/VERSION"
export WORK="$work"
python3 - <<'PY'
import os,tarfile,hashlib,pathlib,io
p=pathlib.Path(os.environ["WORK"])
with tarfile.open(p/"asset.tar.gz","w:gz") as tf:
    tf.add(p/"source/go-reverse-proxy",arcname="./go-reverse-proxy")
digest=hashlib.sha256((p/"asset.tar.gz").read_bytes()).hexdigest()
(p/"SHA256SUMS.txt").write_text(digest+"  asset.tar.gz\n")
with tarfile.open(p/"bad.tar.gz","w:gz") as tf:
    info=tarfile.TarInfo("./go-reverse-proxy")
    info.type=tarfile.SYMTYPE
    info.linkname="/etc/passwd"
    tf.addfile(info)
digest=hashlib.sha256((p/"bad.tar.gz").read_bytes()).hexdigest()
(p/"bad-sums").write_text(digest+"  bad.tar.gz\n")
PY
verify_extract "$work/asset.tar.gz" "$work/SHA256SUMS.txt" asset.tar.gz "$work/output"
cmp "$work/output" "$work/source/go-reverse-proxy"
if verify_extract "$work/asset.tar.gz" "$work/bad-sums" asset.tar.gz "$work/no"; then
 echo "Bad checksum accepted"; exit 1
fi
if verify_extract "$work/bad.tar.gz" "$work/bad-sums" bad.tar.gz "$work/no"; then
 echo "Symlink accepted"; exit 1
fi
# Test failed start restores old executable; successful restart writes version.
mkdir -p "$APP_DIR"
echo original > "$APP_DIR/go-reverse-proxy"
systemctl() { return 1; }
sleep() { :; }
if deploy_binary "$work/source" true; then echo "Failed service accepted"; exit 1; fi
[[ $(cat "$APP_DIR/go-reverse-proxy") == original ]]
systemctl() { return 0; }
deploy_binary "$work/source" true
cmp "$APP_DIR/go-reverse-proxy" "$work/source/go-reverse-proxy"
[[ $(cat "$APP_DIR/VERSION") == v1.2.0 ]]
# Failed download leaves installed binary unchanged.
fetch() { return 1; }
if download_binary "$work/source" latest amd64; then echo "Failed download accepted"; exit 1; fi
cmp "$APP_DIR/go-reverse-proxy" "$work/source/go-reverse-proxy"
[[ $DEFAULT_BYPASS == /auth,/password,/payment,/res,/v1,/sublink,/wordpress,/user ]]
need_root() { :; }
printf 'wrong\n1\n4\n0\n' | menu > "$work/menu-output"
grep -q '请输入 0 到 9' "$work/menu-output"
echo "Manager tests passed: checksum, safe extraction, rollback, upgrade, menu, default bypass."
