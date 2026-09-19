#!/usr/bin/env bash
# 一条命令走完发布后半程：同步部署文件 → 服务器 helm upgrade → 漂移检查。
#
# 为什么需要：README 里那三步（sync / deploy-remote / check-drift）单独跑没问题，
# 但它们之间隔着一次 ssh，中间任何一次 VPN 抖动都会让"发到一半"变成常态。
# 把三步串成一个脚本，失败即停，且每一步都能单独重跑（幂等）。
#
# 用法（仓库根目录）：
#   ./deploy/swr/release.sh 0.12.0-54ba377
#
# 前置：镜像已经推到 SWR（build-push.sh 跑过），且 values-remote.yaml 里就是同一个 tag。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

TAG="${1:-}"
DEST_HOST="${DEST_HOST:-root@192.168.66.8}"
DEST_DIR="${DEST_DIR:-/opt/ratchet-deploy}"

if [[ -z "$TAG" ]]; then
  echo "用法: $0 <镜像 tag>（例如 0.12.0-54ba377）" >&2
  exit 1
fi

# 本地 values 必须已经指向这个 tag：不然发布了镜像、集群却跑旧版本，
# 而且两边都不会报错——这正是我们要防的那种漂移。
values_tag="$(sed -n 's/^ *tag: *"\{0,1\}\([^"]*\)"\{0,1\} *$/\1/p' deploy/k8s/helm/ratchet/values-remote.yaml | head -1)"
if [[ "$values_tag" != "$TAG" ]]; then
  echo "错误:values-remote.yaml 里是 ${values_tag}，与本次要发布的 ${TAG} 不一致。" >&2
  echo "      先改 values（单一事实源），再重跑本脚本。" >&2
  exit 1
fi

echo "==> 1/3 同步部署文件"
./deploy/swr/sync-deploy.sh

echo "==> 2/3 服务器上 helm upgrade（IMAGE_TAG=${TAG}）"
ssh -o BatchMode=yes -o ConnectTimeout=10 "$DEST_HOST" \
  "cd '${DEST_DIR}' && KUBECONFIG=/etc/kubernetes/admin.conf IMAGE_TAG='${TAG}' bash deploy/swr/deploy-remote.sh"

echo "==> 3/3 漂移检查"
ssh -o BatchMode=yes -o ConnectTimeout=10 "$DEST_HOST" \
  "cd '${DEST_DIR}' && KUBECONFIG=/etc/kubernetes/admin.conf bash deploy/swr/check-drift.sh"

cat <<EOF

发布完成：${TAG}

出口验收（在本机跑）：
  curl -s -o /dev/null -w '%{http_code}\n' https://podcloud.dlszjr.com/            # 200
  curl -sI https://podcloud.dlszjr.com/pricing | grep -i content-security-policy   # 应含 *.paddle.com
EOF
