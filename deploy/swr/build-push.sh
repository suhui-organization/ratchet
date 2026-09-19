#!/usr/bin/env bash
# 构建 Ratchet 镜像并推送到华为 SWR。
#
# 用法（在仓库根目录执行）：
#   docker login swr.ap-southeast-3.myhuaweicloud.com   # 华为云账号 + SWR 长期有效密钥
#   ./deploy/swr/build-push.sh [IMAGE_TAG]
#
# 默认值就是线上那套（registry / 组织名），不 export 也能跑。这两个值以前在
# FinHarness 上被抄反过一次（registry 写 cn-east-3、组织写产品名），照抄注释就把镜像
# 推到了一个集群拉不到的地址，所以这里的默认值按"线上真实值"钉住。
#
# 注意 tag 的默认值是 `<版本>-<当前 HEAD 短 sha>`：如果你刚改完 values 里的 tag
# 又提交了那次改动，默认值算出来的是**新**commit，和 values 里写的对不上。
# 正式发版请显式传 tag（`build-push.sh 0.12.0-fa700c3`），让两边是同一个字符串。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

SWR_REGISTRY="${SWR_REGISTRY:-swr.ap-southeast-3.myhuaweicloud.com}"
SWR_NAMESPACE="${SWR_NAMESPACE:-digital-finance}"
VERSION="$(sed -n 's/^const version = "\(.*\)"/\1/p' cmd/ratchet/main.go)"
IMAGE_TAG="${1:-${VERSION}-$(git rev-parse --short HEAD)}"
# 站点镜像必须推（k8s 里跑的就是它）；CLI/MCP 镜像默认也推，集群内起 MCP 服务时要用。
CLI_PUSH="${CLI_PUSH:-1}"
PLATFORM="${PLATFORM:-linux/amd64}"

prefix="${SWR_REGISTRY}/${SWR_NAMESPACE}"

if ! docker info >/dev/null 2>&1; then
  echo "错误:连不上 docker daemon" >&2
  exit 1
fi

# SWR 不认 Docker 默认生成的多平台 attestation manifest（会报 manifest 无效），
# 所以固定单平台 + 关掉 provenance/sbom。集群是 x86-64-v1，只发 amd64。
echo "==> 构建站点镜像 ${prefix}/ratchet-web:${IMAGE_TAG}"
docker build --provenance=false --sbom=false --platform "$PLATFORM" \
  -f deploy/docker/web.Dockerfile -t "${prefix}/ratchet-web:${IMAGE_TAG}" .

if [[ "$CLI_PUSH" == "1" ]]; then
  echo "==> 构建 CLI/MCP 镜像 ${prefix}/ratchet:${IMAGE_TAG}"
  docker build --provenance=false --sbom=false --platform "$PLATFORM" \
    -f Dockerfile -t "${prefix}/ratchet:${IMAGE_TAG}" .
fi

echo "==> 推送"
docker push "${prefix}/ratchet-web:${IMAGE_TAG}"
[[ "$CLI_PUSH" == "1" ]] && docker push "${prefix}/ratchet:${IMAGE_TAG}"

cat <<EOF

推送完成。接下来：
  1) 把 deploy/k8s/helm/ratchet/values-remote.yaml 里 image.web.tag 改成 ${IMAGE_TAG}（提交）
  2) ./deploy/swr/sync-deploy.sh
  3) 服务器上：IMAGE_TAG=${IMAGE_TAG} bash deploy/swr/deploy-remote.sh
  4) bash deploy/swr/check-drift.sh    # 退出码 0 = 集群与 release 记录一致

镜像：
  ${prefix}/ratchet-web:${IMAGE_TAG}
$([[ "$CLI_PUSH" == "1" ]] && echo "  ${prefix}/ratchet:${IMAGE_TAG}")
EOF
