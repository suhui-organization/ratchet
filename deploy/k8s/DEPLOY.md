# Ratchet 的 K8s 部署（chart 说明）

这套清单只为**站点**而生：Ratchet 本体是一个 CLI/MCP 工具，跑在**用户自己的机器**上
（这正是它的卖点：结论在收货方的机器上得出）。集群里只放对外的静态站点。

## 这是一个纯静态站点

| | 有 | 没有 |
|---|---|---|
| 工作负载 | 1 个 Deployment（nginx 发静态文件） | 没有数据库、没有后端、没有 CronJob |
| 存储 | 无 | 没有 PVC/PV（丢了 Pod 不丢数据） |
| 密钥 | 只有私仓拉取密钥 | 没有 JWT/数据库口令/API Key |
| 出口 | Service NodePort（前置 nginx 反代） | 不依赖 Ingress 控制器 |

所以这份 chart 故意很短：`templates/` 下只有 web 的 Deployment / Service / Ingress。
它存在的意义不是"能部署"，而是"发版路径只有一个入口、可回滚、可对账"。

## 文件

```
deploy/k8s/
├── DEPLOY.md                     ← 你在这里
└── helm/ratchet/
    ├── Chart.yaml
    ├── values.yaml               # 默认值（对外通用）
    ├── values-remote.yaml        # 线上收敛值（192.168.66.8），不含密钥
    └── templates/
        ├── _helpers.tpl
        ├── web-deployment.yaml
        ├── web-service.yaml
        └── ingress.yaml
```

## 常用命令

```bash
# 渲染检查（不碰集群）
helm template ratchet deploy/k8s/helm/ratchet -f deploy/k8s/helm/ratchet/values-remote.yaml

# 部署（正式路径见 deploy/swr/README.md）
KUBECONFIG=/etc/kubernetes/admin.conf \
  helm upgrade --install ratchet deploy/k8s/helm/ratchet \
  -n ratchet --create-namespace \
  -f deploy/k8s/helm/ratchet/values-remote.yaml \
  --set image.repositoryPrefix=swr.ap-southeast-3.myhuaweicloud.com/digital-finance \
  --set image.web.tag=<tag> \
  --set image.pullSecrets[0].name=ratchet-swr \
  --wait --timeout 10m
```

## 默认值里值得注意的三处

1. **`web.updateStrategy = maxUnavailable:0 / maxSurge:1`**：先起新 Pod 再退旧 Pod。
   站点没有"新旧数据不兼容"的窗口，所以不需要 FinHarness 那种 `maxSurge:0` 的硬反亲和权衡。
2. **软反亲和**：默认尽量把 2 个副本铺到不同节点，但节点不够时不会卡在 Pending
   （硬反亲和 + `maxSurge:0` 的组合在滚动时会死锁，FinHarness 为此专门写了注释）。
3. **`ingress.enabled = false`（线上）**：这台集群的对外入口是 web01 的宿主 nginx，
   它反代到 NodePort。Ingress 模板留着，是给"别的集群有 ingress-nginx 且没有前置 nginx"
   的场景用的——别在两个地方同时配同一个域名。

## 和 FinHarness 的差异（刻意为之）

| | FinHarness | Ratchet |
|---|---|---|
| 组件 | server + web + postgres(+fluvia/module-market) | 只有 web |
| values 里的密钥 | JWT/DB/SSO 走 `values-remote.secret.yaml` | 没有密钥，只有拉取密钥名 |
| 副本策略 | 3 副本硬反亲和（每节点一个） | 2 副本软反亲和 |
| 数据 | PVC + 备份 Job | 无状态 |

骨架相同（同一套 helm/单一事实源/漂移检查），因此运维手法可以互相照搬；
差异只在"站点没有状态"这一条上。
