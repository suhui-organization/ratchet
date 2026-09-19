# 交付记录 0.13.0 —— 站点落到自己的 K8s 集群（华为 SWR + web01 出口）

> 日期：2026-09-19。上一版见 [DELIVERY-0.6.0.md](DELIVERY-0.6.0.md)。
> 这一版不是功能版，是**把站点从"托管在别人那里"变成"跑在自己的机器上"**。

## 1. 这一版做了什么

站点现在跑在 `192.168.66.8` 的 K8s 集群里（命名空间 `ratchet`），镜像走华为 SWR，
出口域名 `https://podcloud.dlszjr.com` 由 web01（192.168.66.10）的前置 nginx 反代。

```
本地构建 ──► 华为 SWR ──► 同步部署文件 ──► 集群 helm upgrade ──► 漂移检查
                                   │
                     web01 nginx:443（*.dlszjr.com 通配符证书）
                                   │
                        192.168.66.8:30090  →  ratchet-web ×2
```

| 项 | 值 |
|---|---|
| 命名空间 | `ratchet` |
| 工作负载 | `ratchet-web`（nginx 发静态文件，2 副本，软反亲和） |
| 镜像 | `swr.ap-southeast-3.myhuaweicloud.com/digital-finance/ratchet-web:0.12.0-af0372a` |
| 拉取密钥 | `ratchet-swr`（复用 FinHarness 同一个 SWR 组织的凭据） |
| 出口 | NodePort 30090，反代在 web01 的 `/data/service/nginx/conf/vhost/podcloud.dlszjr.com.conf` |
| 状态 | 2/2 Running（worker1 + k8s-worker2），`check-drift.sh` 退出码 0 |

一套命令（细节见 [../deploy/swr/README.md](../deploy/swr/README.md)）：

```bash
./deploy/swr/build-push.sh 0.12.0-<sha>     # 构建 + 推 SWR
./deploy/swr/sync-deploy.sh                 # 同步 deploy/ 到服务器
# 服务器上：
IMAGE_TAG=0.12.0-<sha> bash deploy/swr/deploy-remote.sh
bash deploy/swr/check-drift.sh              # 0 = release 与集群一致

# 出口域名（本机执行，带 nginx -t 失败整批回滚）
EDGE_PASSWORD='...' ./deploy/swr/set-edge-domain.sh
```

## 2. 为什么这么做

站点原先挂在 Vercel（GitHub 集成，push 即部署），现在收回自建集群，原因有三个：

1. **同一个域名要服务两种用途**：`podcloud.dlszjr.com` 是 Paddle 已认证的域名，
   换域名等于重走认证；把 Ratchet 放上去就不动认证。
2. **和 SecurityHarness 走同一条路**：同一台集群、同一套 SWR、同一个前置 nginx。
   运维手法可以互相照搬，"发版路径"从两套收敛成一套。
3. **站点是纯静态的**：没有后端、没有数据库、没有密钥。放进集群的边际成本接近零，
   而"能自己托管"这件事对一个讲供应链安全的工具是有意义的姿态。

## 3. 途中修掉的三个真问题

这三个都不是"顺手优化"，是上线过程中撞出来的，且都会在生产上表现出来：

| # | 问题 | 表现 | 修法 |
|---|---|---|---|
| 1 | `LC_ALL=C.UTF-8` 被当成"有语言偏好" | 产物默认英文的承诺在 CI/容器（默认就是 C.UTF-8）里失效，报告变成中文 | `_normalize` 认 `c.*` / `posix.*` 为"无信息"，继续往下找 |
| 2 | 站点 nginx 配置写成 `nginx.conf` | 按字母序排在镜像自带 `default.conf` 之后，**整套配置静默不生效**：`/healthz` 404、`server_tokens` 没关、缓存策略全丢 | 改名 `default.conf` 覆盖它 |
| 3 | `try_files` 首项是 `$uri` | `/verify` 命中目录后 nginx 发 301 到 `/verify/`，站内每个链接都多一跳 | 首项改 `$uri/index.html`，直接命中预渲染文件 |

第 1 条顺带解释了"为什么本机跑测试全红"：开发机 `LANG=zh_CN.UTF-8` 时，
那批断言"默认英文"的用例测的其实是宿主环境。现在 conftest 把语言环境按到中性，
需要特定语言的用例自己设——测试不再依赖跑它的人用什么 locale。

## 4. 实测

```console
$ make test
ok  github.com/suhui-organization/ratchet/internal/...   （Go 全部包）
34 passed                                                （Python，含 7 条语言解析用例）
8 passed                                                 （Web）

$ curl -sI https://podcloud.dlszjr.com/ | head
HTTP/1.1 200 OK
Server: nginx
Cache-Control: no-cache
X-Content-Type-Options: nosniff
X-Frame-Options: SAMEORIGIN
Content-Security-Policy: default-src 'self'; ...

$ for p in / /verify /pricing /legal/terms /legal/privacy /legal/refund /healthz; do
    curl -s -o /dev/null -w "%{http_code} $p\n" https://podcloud.dlszjr.com$p; done
200 /      200 /verify   200 /pricing   200 /legal/terms
200 /legal/privacy       200 /legal/refund          200 /healthz
404 /nope                                                （品牌化 404 页，不是回落首页）
```

证书是既有通配符 `*.dlszjr.com`（有效期到 2026-12-06），没有另外签发。

## 5. 回滚路径

| 要回滚什么 | 怎么做 |
|---|---|
| 单个镜像 | `helm -n ratchet rollback ratchet <revision> --wait` |
| 出口域名 | web01 上把 `podcloud.dlszjr.com.conf.bak-20260919-175417` 拷回原名，`nginx -t && nginx -s reload` |
| 整个站点 | `helm -n ratchet uninstall ratchet`（命名空间与 Secret 可一并删除） |

旧 Pod Cloud 的 Deployment 还在 `podcloud` 命名空间里（NodePort 30088），
只是域名不再指向它。要彻底下线：`kubectl delete ns podcloud`。

## 6. 已知缺口

| # | 事项 | 影响 |
|---|---|---|
| K1 | `ratchet.dlszjr.com` 还没加 DNS 记录 | 想要个干净域名就加一条 A → `59.46.235.173`，然后在 web01 加同名 vhost（`DOMAINS=ratchet.dlszjr.com` 跑一次脚本） |
| K2 | 站点版本号仍显示 0.12.0 | 这一版只动了站点与部署，没有走 `make release`；下次发版时统一 |
| K3 | 集群里没有 Ratchet 自身的 MCP 服务 | CLI/MCP 镜像已推到 SWR（`digital-finance/ratchet`），但按设计 Ratchet 跑在用户机器上，集群里只放站点 |
| K4 | 没有 CI 自动发版到 SWR | 目前是本地 `build-push.sh`；要自动化得把 SWR 密钥放进 GitHub Secrets |

## 7. 建议的下一步

工程侧这次是**补齐部署的短板**，站点已经从"别人的托管"变成"自己的资产"。
接下来真正的稀缺资源不是代码：

- 站点的存在意义是让第一个客户能**自己验证**结论——现在的下一个瓶颈是把它发出去；
- K1/K2 是十分钟的活，等有人真的用 `ratchet.dlszjr.com` 再说不迟。
