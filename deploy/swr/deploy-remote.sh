#!/usr/bin/env bash
# 在服务器（或任何持有目标集群 kubeconfig 的机器）上执行 Ratchet 站点的 Helm 部署。
#
# 单一写者原则：chart + values-remote.yaml 是唯一事实源，镜像 tag 只写在 values 里。
# 本脚本固定带 -f values 文件，--set 只用于"本次要发布的 tag"与拉取密钥名；
# **不要对 helm 管理下的 Deployment 用 kubectl set image**（会造成 release 与实际漂移）。
#
# 必填：
#   IMAGE_TAG        本次要发布的 ratchet-web 镜像 tag
# 可选：
#   SWR_REGISTRY / SWR_NAMESPACE   默认与线上一致
#   SWR_USERNAME / SWR_PASSWORD    首次创建 imagePullSecret 时需要；已有同名 Secret 自动复用
#   NAMESPACE（默认 ratchet） RELEASE（默认 ratchet）
#   VALUES_FILE（默认 deploy/k8s/helm/ratchet/values-remote.yaml）
#   SWR_SECRET（默认 ratchet-swr） KUBECONFIG
set -euo pipefail

# 服务器上没有 ~/.kube/config，控制平面的 admin.conf 是那把钥匙
if [[ -z "${KUBECONFIG:-}" && -r /etc/kubernetes/admin.conf ]]; then
  export KUBECONFIG=/etc/kubernetes/admin.conf
fi

SWR_REGISTRY="${SWR_REGISTRY:-swr.ap-southeast-3.myhuaweicloud.com}"
SWR_NAMESPACE="${SWR_NAMESPACE:-digital-finance}"
IMAGE_TAG="${IMAGE_TAG:-}"
NAMESPACE="${NAMESPACE:-ratchet}"
RELEASE="${RELEASE:-ratchet}"
SWR_SECRET="${SWR_SECRET:-ratchet-swr}"
VALUES_FILE="${VALUES_FILE:-deploy/k8s/helm/ratchet/values-remote.yaml}"

if [[ -z "$IMAGE_TAG" ]]; then
  echo "错误:请 export IMAGE_TAG=<这次要发的 ratchet-web 镜像 tag>" >&2
  exit 1
fi
if [[ ! -f "$VALUES_FILE" ]]; then
  echo "错误:找不到 values 文件 ${VALUES_FILE}（这是单一事实源，不能省）" >&2
  exit 1
fi

kubectl version --client >/dev/null
kubectl get nodes >/dev/null

kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

if kubectl -n "$NAMESPACE" get secret "$SWR_SECRET" >/dev/null 2>&1; then
  echo "==> 复用已有拉取密钥 ${SWR_SECRET}"
else
  if [[ -z "${SWR_USERNAME:-}" || -z "${SWR_PASSWORD:-}" ]]; then
    cat >&2 <<EOF
错误:命名空间 ${NAMESPACE} 里没有 ${SWR_SECRET}，也没有给 SWR_USERNAME/SWR_PASSWORD。

两种做法（任选其一）：
  A. 直接建密钥：
       kubectl -n ${NAMESPACE} create secret docker-registry ${SWR_SECRET} \\
         --docker-server=${SWR_REGISTRY} \\
         --docker-username=<华为云账号> --docker-password=<SWR 长期密钥>
  B. 从已配好的命名空间复制（同一台集群、同一个 SWR 组织）：
       kubectl -n finharness get secret finharness-swr -o json \\
         | python3 -c 'import json,sys; d=json.load(sys.stdin); d["metadata"].pop("resourceVersion",None); d["metadata"]= {"name":"${SWR_SECRET}","namespace":"${NAMESPACE}"}; print(json.dumps(d))' \\
         | kubectl apply -f -
EOF
    exit 1
  fi
  kubectl -n "$NAMESPACE" create secret docker-registry "$SWR_SECRET" \
    --docker-server="$SWR_REGISTRY" \
    --docker-username="$SWR_USERNAME" \
    --docker-password="$SWR_PASSWORD" >/dev/null
  echo "==> 已创建拉取密钥 ${SWR_SECRET}"
fi

prefix="${SWR_REGISTRY}/${SWR_NAMESPACE}"

echo "==> helm upgrade --install ${RELEASE} (${prefix}/ratchet-web:${IMAGE_TAG})"
helm upgrade --install "$RELEASE" deploy/k8s/helm/ratchet \
  --namespace "$NAMESPACE" --create-namespace \
  -f "$VALUES_FILE" \
  --set "image.repositoryPrefix=${prefix}" \
  --set "image.web.tag=${IMAGE_TAG}" \
  --set "image.pullSecrets[0].name=${SWR_SECRET}"

kubectl -n "$NAMESPACE" rollout status "deploy/${RELEASE}-web" --timeout=300s
kubectl -n "$NAMESPACE" get deploy,pods,svc -o wide

node_port="$(kubectl -n "$NAMESPACE" get svc "${RELEASE}-web" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || true)"
if [[ -n "$node_port" ]]; then
  cat <<EOF

自检：
  curl -s -o /dev/null -w '%{http_code}\n' http://192.168.66.8:${node_port}/healthz   # 期望 200
  curl -s -o /dev/null -w '%{http_code}\n' http://192.168.66.8:${node_port}/verify   # 期望 200
出口域名由 web01 的 nginx 反代到这个 NodePort，见 deploy/swr/README.md。
EOF
fi
