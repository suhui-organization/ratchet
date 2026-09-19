#!/usr/bin/env bash
# 发布流水线第 3 步：把本地 deploy/ 同步到服务器的部署目录。
#
#   本地打包 → SWR 上传 → 【同步部署文件】 → 服务器上 helm upgrade → 漂移检查
#
# 为什么单独成脚本：这一步以前是手敲 scp/rsync，结果服务器上的 chart 悄悄停在
# 某个旧版本（FinHarness 实测停在 3 天前），后面 helm upgrade 用的其实是旧 values。
# 同步本身没难度，难点是"让差异可见"，所以这里先预演再同步，并把两边的文件数打出来。
#
# 用法（仓库根目录）：
#   ./deploy/swr/sync-deploy.sh            # 先预演，再同步
#   ./deploy/swr/sync-deploy.sh --dry-run  # 只看差异
#
# 可选环境变量：
#   DEST_HOST    默认 root@192.168.66.8
#   DEST_DIR     默认 /opt/ratchet-deploy/deploy
#   SYNC_DELETE=1  删除服务器上本地已不存在的文件（默认关闭）
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SRC="${REPO_ROOT}/deploy/"
DEST_HOST="${DEST_HOST:-root@192.168.66.8}"
DEST_DIR="${DEST_DIR:-/opt/ratchet-deploy/deploy}"

DRY_RUN=0
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=1

SSH_OPTS=(-o BatchMode=yes -o ConnectTimeout=8 -o StrictHostKeyChecking=no)
ssh_run() { ssh "${SSH_OPTS[@]}" "$DEST_HOST" "$@"; }

if ! ssh_run true 2>/dev/null; then
  cat >&2 <<EOF
错误:无法免密 ssh 到 ${DEST_HOST}

这台机器本机已配好 key（BatchMode 直连可用）。换机器的话自查：
  ssh -o BatchMode=yes ${DEST_HOST} true
  ip -brief addr | grep tun        # 192.168.66.8 不在本机网段，需要走 VPN 隧道
EOF
  exit 1
fi

ssh_run "mkdir -p '${DEST_DIR}'"

rsync_opts=(-az --human-readable --itemize-changes
  --exclude '__pycache__/' --exclude '*.pyc' --exclude '.DS_Store')
[[ "${SYNC_DELETE:-0}" == "1" ]] && rsync_opts+=(--delete)

echo "==> ${SRC}  →  ${DEST_HOST}:${DEST_DIR}/"
echo "==> 预演（不写入）"
rsync "${rsync_opts[@]}" --dry-run "$SRC" "${DEST_HOST}:${DEST_DIR}/" || true

if [[ "$DRY_RUN" == "1" ]]; then
  echo "==> --dry-run，结束"
  exit 0
fi

echo "==> 同步"
rsync "${rsync_opts[@]}" "$SRC" "${DEST_HOST}:${DEST_DIR}/"

# 同步后核对：写进 values 的 tag 必须和本地一致，否则后面 helm upgrade 发的是旧版本
echo "==> 两边 values-remote.yaml 的镜像 tag"
local_tag="$(sed -n 's/^ *tag: *"\{0,1\}\([^"]*\)"\{0,1\} *$/\1/p' \
  "${SRC}k8s/helm/ratchet/values-remote.yaml" | head -1)"
remote_tag="$(ssh_run "sed -n 's/^ *tag: *\"\\{0,1\\}\\([^\"]*\\)\"\\{0,1\\} *\$/\\1/p' '${DEST_DIR}/k8s/helm/ratchet/values-remote.yaml' | head -1")"
echo "  本地 = ${local_tag}"
echo "  服务器 = ${remote_tag}"
if [[ "$local_tag" != "$remote_tag" ]]; then
  echo "错误:两边 tag 不一致，同步没生效" >&2
  exit 1
fi

echo "==> 完成。下一步在服务器上：KUBECONFIG=/etc/kubernetes/admin.conf IMAGE_TAG=${local_tag} bash ${DEST_DIR}/swr/deploy-remote.sh"
