#!/bin/sh
# 给本机 kind 集群开一个**固定地址**。
#
# 为什么需要这东西（都是实测，不是推测）：
#   kind 的节点是个 docker 容器，而**宿主机打不到它的 IP**：
#     curl http://172.18.0.2:30090/healthz → 空响应
#   这个集群又不是我们建的（里面还跑着 finharness / fluvia / traefik），
#   没法为了端口映射重建它。
#
#   但**同 docker 网络里的容器能到**：
#     docker run --rm --network kind alpine wget -qO- http://172.18.0.2:30090/healthz → {"status":"ok"}
#   所以这里放一个常驻的 TCP 转发容器，把 127.0.0.1:30090 接到节点的 NodePort。
#
#   比 `kubectl port-forward` 好在哪：后者是前台进程，关终端就断（这台机器上
#   还撞到过端口被别的服务占用，curl 打到别人身上）。这个容器 --restart unless-stopped，
#   关终端、重启机器都还在。
#
# 用法：bash scripts/local-gateway.sh [宿主端口] [节点IP]
set -eu

HOST_PORT="${1:-30090}"
NODE_IP="${2:-$(docker inspect desktop-control-plane --format '{{(index .NetworkSettings.Networks "kind").IPAddress}}')}"
NAME=asas-local-gateway

echo "节点 $NODE_IP:$HOST_PORT → 宿主机 127.0.0.1:$HOST_PORT"

# 先确认节点那边真的应了，再动手；否则转发出去也是个不通的地址。
if ! docker run --rm --network kind alpine:latest \
      sh -c "wget -qO- --timeout=5 http://$NODE_IP:$HOST_PORT/healthz" >/dev/null 2>&1; then
  echo "❌ 节点 $NODE_IP:$HOST_PORT 没有应答。检查："
  echo "   1) helm 里 api.serviceType 是不是 NodePort、nodePort 是不是 $HOST_PORT"
  echo "   2) kubectl -n asas get svc asas-api"
  exit 1
fi

docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" --restart unless-stopped \
  --network kind -p "127.0.0.1:$HOST_PORT:$HOST_PORT" \
  alpine/socat "tcp-listen:$HOST_PORT,fork,reuseaddr" "tcp:$NODE_IP:$HOST_PORT" >/dev/null

sleep 2
echo -n "控制面地址： http://127.0.0.1:$HOST_PORT"
if curl -fsS --max-time 10 "http://127.0.0.1:$HOST_PORT/healthz" >/dev/null; then
  echo "   ✅ 通"
else
  echo "   ❌ 不通，看 docker logs $NAME"
  exit 1
fi
