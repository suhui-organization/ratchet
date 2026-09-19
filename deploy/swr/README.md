# 华为 SWR + 服务器 K8s 部署

目标：把 Ratchet 站点发到 `192.168.66.8` 的 K8s 集群（命名空间 `ratchet`），
镜像走华为 SWR，出口域名由 web01 的前置 nginx 反代到 NodePort。

这套流程是照着 SecurityHarness（FinHarness）在**同一台集群**上跑了半年的路径复刻的，
连坑都一起复刻了：单一事实源、镜像 tag 只写在 values 里、不要用 `kubectl set image`、
SWR 不认 Docker 的 attestation manifest。改这里之前先读 §0。

## 前置

- 本机能 `ssh -o BatchMode=yes root@192.168.66.8 true`（192.168.66.8 不在本机网段，
  走 VPN 隧道；本机已配好 key）
- 本机 `docker login swr.ap-southeast-3.myhuaweicloud.com` 成功（华为云账号 + SWR 长期密钥）
- 服务器上有 `kubectl` + `helm`（实测 kubectl v1.28.2 / helm v3.12.0）
- 服务器上控制平面凭据在 `/etc/kubernetes/admin.conf`（**没有** `~/.kube/config`）

## 流水线总览

```
本地构建镜像 → 推 SWR → 同步部署文件到服务器 → 服务器上 helm upgrade → 漂移检查 → 出口域名
   build-push.sh          sync-deploy.sh        deploy-remote.sh     check-drift.sh  set-edge-domain.sh
```

## 0. 单一写者原则（必读）

**chart + `values-remote.yaml` 是唯一事实源；镜像 tag 只写在 values 里。**

在 FinHarness 上踩过的坑：镜像曾经用 `kubectl set image` 更新，而 helm release 里记的
还是 values 里的旧 tag，于是出现「release 说 A、集群跑 B」的漂移——一旦有人执行
`helm upgrade` / `helm rollback`，线上会被**静默回退**。所以：

1. 改 `deploy/k8s/helm/ratchet/values-remote.yaml` 里的 `image.web.tag` → 提交；
2. `./deploy/swr/build-push.sh <tag>`（也可以不传 tag，脚本按 `<版本>-<短 sha>` 自算）；
3. `./deploy/swr/release.sh <tag>` —— 一条命令跑完后三步：同步部署文件 → 服务器
   `helm upgrade` → 漂移检查（退出码 0 = 无漂移）。

> 第 3 步以前是三条手敲命令，中间隔着两次 ssh；VPN 抖一下就会出现"发到一半"。
> 现在串成一个脚本，且每一步都幂等，可以单独重跑。
> 想拆开单跑也可以，等价命令是：`sync-deploy.sh` → `deploy-remote.sh` → `check-drift.sh`。

> 对 helm 管理下的 Deployment **不要**再用 `kubectl set image` / `kubectl scale`。
> 紧急情况下不得已用了，事后必须把 values 改到一致并跑一次 helm upgrade 收敛。

## 1. 构建并推送镜像

```bash
cd <ratchet 仓库根目录>
export SWR_REGISTRY=swr.ap-southeast-3.myhuaweicloud.com   # 线上值；换 Region 才改
export SWR_NAMESPACE=digital-finance                       # SWR 组织（不是 ratchet）
docker login "$SWR_REGISTRY"

./deploy/swr/build-push.sh 0.12.0-abc1234
```

产物：

- `${SWR_REGISTRY}/${SWR_NAMESPACE}/ratchet-web:<tag>`  ← 站点镜像（k8s 跑的就是它）
- `${SWR_REGISTRY}/${SWR_NAMESPACE}/ratchet:<tag>`      ← CLI/MCP 镜像（`CLI_PUSH=0` 可关）

两个必须记住的约束（都是实测撞出来的）：

1. **SWR 不认 Docker 默认的多平台 attestation manifest**：必须
   `--provenance=false --sbom=false --platform linux/amd64`，脚本里已经写死；
2. **集群节点是 Core2 级 x86-64-v1**（无 POPCNT）：别换 UBI/RHEL 系基础镜像，
   glibc 编到 x86-64-v2 会直接 `Fatal glibc error`。站点用 `nginx:alpine`，这个组合
   在集群上已经跑了很久。

## 2. 同步部署文件到服务器

```bash
./deploy/swr/sync-deploy.sh --dry-run   # 只看差异
./deploy/swr/sync-deploy.sh             # 先预演，再同步
```

把 `deploy/` 同步到 `root@192.168.66.8:/opt/ratchet-deploy/deploy`，并且：

- 先预演再同步，把"这次会动哪些文件"打在台面上；
- 默认不删除服务器上多余的文件（要删用 `SYNC_DELETE=1`）；
- 末尾比对两边 `values-remote.yaml` 的镜像 tag，不一致直接算失败——
  这一步专门用来挡住"chart 悄悄停在旧版本、helm upgrade 用的其实是旧 values"。

## 3. 在服务器上部署

```bash
ssh root@192.168.66.8
cd /opt/ratchet-deploy
export KUBECONFIG=/etc/kubernetes/admin.conf

# 首次：建 SWR 拉取密钥。也可以直接从 finharness 命名空间复制一份（同一个 SWR 组织）：
export SWR_USERNAME=...   # 华为云账号
export SWR_PASSWORD=...   # SWR 长期密钥

IMAGE_TAG=0.12.0-abc1234 bash deploy/swr/deploy-remote.sh
bash deploy/swr/check-drift.sh
```

脚本做四件事：建/复用命名空间 → 建/复用 `ratchet-swr` 拉取密钥 →
`helm upgrade --install ratchet`（固定带 `-f values-remote.yaml`，只覆盖本次的 tag）→
等 rollout 并打印资源与自检命令。

## 4. 访问与出口域名

集群内部：`http://ratchet-web.ratchet.svc.cluster.local`
节点直连：`http://192.168.66.8:30090`（NodePort，见 values-remote.yaml）

公网出口**不在集群里**：`*.dlszjr.com` 的 TLS 由 web01（192.168.66.10）上的 nginx
终结（通配符证书 `*.dlszjr.com`），它按 `server_name` 反代到各服务的 NodePort：

```
podcloud.dlszjr.com ──> web01:nginx:443 ──> 192.168.66.8:30090 (ratchet-web)
                          （TLS: *.dlszjr.com 通配符证书）
finharness.dlszjr.com ──> 192.168.66.8:30080
market.dlszjr.com     ──> 192.168.66.8:30085
```

**只用一个域名：`podcloud.dlszjr.com`。** 这个域名是 Paddle 认证绑定的那个，
换域名等于重走一遍收单审核；再挂一个别名（比如 `ratchet.dlszjr.com`）只会让
"客户在哪个域名上付的钱、发票上写的是谁"多一个需要解释的地方，收益为零。
所以集群 NodePort、web01 vhost、站点里的 Paddle 配置都对准这一个域名。

改映射（本机执行，带自动回滚）：

```bash
EDGE_PASSWORD='<web01 root 密码>' ./deploy/swr/set-edge-domain.sh

# 只想看会写什么（不碰服务器）：
DRY_RUN=1 ./deploy/swr/set-edge-domain.sh

# 万不得已要换域名（会同时改 DNS 与 Paddle 认证，别轻易做）：
DOMAINS="new.dlszjr.com" EDGE_PASSWORD='...' ./deploy/swr/set-edge-domain.sh
```

脚本会先 `cp` 备份原 vhost 到 `<文件>.bak-<时间戳>`，写入后 `nginx -t`，
**不通过就整批回滚**，通过了才 reload。证书是通配符，换域名不用重新签，
但 DNS 的 A 记录要指到 `59.46.235.173`（web01 的公网出口）。

> 域名目前的状态：`podcloud.dlszjr.com` 原来指向旧 Pod Cloud（NodePort 30088），
> 现已改指 Ratchet；旧 Pod Cloud 的 Deployment 还在集群里，回滚就是把 vhost 的
> `proxy_pass` 换回 `192.168.66.8:30088`，再用 `.bak-*` 也行。

## 5. 验证

```bash
# 集群侧
kubectl -n ratchet get deploy,pods,svc
kubectl -n ratchet exec deploy/ratchet-web -- wget -qO- http://127.0.0.1/healthz   # → ok

# NodePort
curl -s -o /dev/null -w '%{http_code}\n' http://192.168.66.8:30090/verify          # → 200

# 出口域名
curl -s -o /dev/null -w '%{http_code}\n' https://podcloud.dlszjr.com/               # → 200
curl -sI https://podcloud.dlszjr.com/ | grep -iE 'x-content-type|content-security'  # 安全头在
```

## 6. 回滚

```bash
# 回上一个镜像 tag
helm -n ratchet history ratchet
helm -n ratchet rollback ratchet <revision> --wait

# 回出口域名（把备份拷回去）
ssh root@192.168.66.10 'cp -p /data/service/nginx/conf/vhost/podcloud.dlszjr.com.conf.bak-<时间戳> \
  /data/service/nginx/conf/vhost/podcloud.dlszjr.com.conf && \
  /data/service/nginx/sbin/nginx -t -p /data/service/nginx -c conf/nginx.conf && \
  /data/service/nginx/sbin/nginx -s reload -p /data/service/nginx -c conf/nginx.conf'
```

## 7. 安全提示

- SWR 密钥不要写进仓库；放环境变量或直接 `kubectl create secret docker-registry` 预建；
- web01 的 root 密码同样不要进仓库（`EDGE_PASSWORD` 只在命令行临时给）；
- 本机 `~/.docker/config.json` 里存着 SWR 长期密钥，别整份拷给别人。
