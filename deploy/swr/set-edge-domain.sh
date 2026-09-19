#!/usr/bin/env bash
# 把某个 *.dlszjr.com 域名指到 Ratchet 站点（改的是 web01 上的前置 nginx）。
#
# 为什么要脚本化：域名出口在 web01（192.168.66.10），k8s 里只暴露 NodePort。
# "域名指向谁"散落在别人机器的 /data/service/nginx/conf/vhost/ 里，不写进版本控制
# 就会变成只有当时那个人知道的秘密。这里把它变成可重放、可回滚的一步。
#
# 安全阀：写完先 nginx -t；不通过就整批回滚到备份，绝不把 web01 上别人的站点带下线。
#
# 用法（本机执行；web01 只开密码登录，所以走 paramiko）：
#   EDGE_PASSWORD='<web01 root 密码>' ./deploy/swr/set-edge-domain.sh
#
# 可选环境变量：
#   EDGE_HOST   默认 192.168.66.10
#   EDGE_USER   默认 root
#   DOMAINS     默认 "podcloud.dlszjr.com"（空格分隔可写多个）
#   UPSTREAM    默认 192.168.66.8:30090（= values-remote.yaml 里的 nodePort）
#   DRY_RUN=1   只打印将要写入的内容，不碰服务器
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEMPLATE="${REPO_ROOT}/deploy/swr/edge-ratchet.conf.template"

EDGE_HOST="${EDGE_HOST:-192.168.66.10}"
EDGE_USER="${EDGE_USER:-root}"
DOMAINS="${DOMAINS:-podcloud.dlszjr.com}"
UPSTREAM="${UPSTREAM:-192.168.66.8:30090}"
REMOTE_VHOST_DIR="${REMOTE_VHOST_DIR:-/data/service/nginx/conf/vhost}"
REMOTE_NGINX="${REMOTE_NGINX:-/data/service/nginx/sbin/nginx}"
REMOTE_NGINX_ARGS="${REMOTE_NGINX_ARGS:--p /data/service/nginx -c conf/nginx.conf}"

[[ -f "$TEMPLATE" ]] || { echo "错误:找不到模板 $TEMPLATE" >&2; exit 1; }

if [[ "${DRY_RUN:-0}" == "1" ]]; then
  for d in $DOMAINS; do
    echo "==> 将写入 ${EDGE_HOST}:${REMOTE_VHOST_DIR}/${d}.conf（upstream=${UPSTREAM}）"
    sed -e "s|__DOMAIN__|${d}|g" -e "s|__UPSTREAM__|${UPSTREAM}|g" "$TEMPLATE"
    echo
  done
  exit 0
fi

if [[ -z "${EDGE_PASSWORD:-}" ]]; then
  echo "错误:需要 EDGE_PASSWORD（web01 的 root 密码）。" >&2
  echo "" >&2
  echo "  EDGE_PASSWORD='...' ./deploy/swr/set-edge-domain.sh" >&2
  echo "" >&2
  echo "想免掉这个变量，可以给自己发一把公钥上去（一次性）：" >&2
  echo "  ssh-copy-id -i ~/.ssh/id_ed25519.pub root@192.168.66.10" >&2
  echo "不做也能用——本脚本用 paramiko 走密码认证。" >&2
  exit 1
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
for d in $DOMAINS; do
  sed -e "s|__DOMAIN__|${d}|g" -e "s|__UPSTREAM__|${UPSTREAM}|g" "$TEMPLATE" \
    > "${workdir}/${d}.conf"
done

EDGE_PASSWORD="$EDGE_PASSWORD" EDGE_HOST="$EDGE_HOST" EDGE_USER="$EDGE_USER" \
DOMAINS="$DOMAINS" WORKDIR="$workdir" REMOTE_VHOST_DIR="$REMOTE_VHOST_DIR" \
REMOTE_NGINX="$REMOTE_NGINX" REMOTE_NGINX_ARGS="$REMOTE_NGINX_ARGS" \
python3 "$(dirname "${BASH_SOURCE[0]}")/edge_apply.py"
