# 交付记录 0.16.0 —— P2：传感器容器化（含三处实测缺陷修复）

> 日期：2026-09-20。上一版见 [DELIVERY-0.15.0.md](DELIVERY-0.15.0.md)。
> 本轮按 [DEVELOPMENT-PLAN.md](DEVELOPMENT-PLAN.md) 走完整条流水线：
> **Spec → 实现 → 测试 → 本地 k8s 部署 → 验收 → 落档**。P2 的判据逐条打勾（§4）。

## 1. 这一轮做了什么

把"传感器"从一份脚本变成**能在客户集群里跑、跑完能验收、跑不成会喊**的东西。
P2 的任务与判据见 [DEVELOPMENT-PLAN.md §8](DEVELOPMENT-PLAN.md)。

| # | 任务 | 状态 | 产物 |
|---|---|---|---|
| T10 | 采集 → 出证 → 上报 一条命令 | ✅ | `deploy/sensor/sensor.sh`、`deploy/docker/sensor.Dockerfile` |
| T11 | 集群内 CronJob（默认关） | ✅ | `deploy/k8s/helm/asas/templates/sensor-cronjob.yaml` |
| T12 | 默认不执行配置里的命令 | ✅ | `ASAS_INTROSPECT` 开关 + 镜像里根本没有 node/npx（§3.4） |
| T13 | 镜像以固定 digest 运行 | ✅ | SWR digest `sha256:bac7ff55…` |
| T14 | 出网有界（离线 / 预算） | ✅ | `RATCHET_OFFLINE`、`RATCHET_HTTP_BUDGET` |
| T15 | 落档 | ✅ | 就是这份文件 |

顺带修掉三个**实测出来的**缺陷（都不是假想）：脚本 flag 写错导致采集全灭、
管道吞掉退出码导致失败被报成成功、`--set` 把豁免列表截断。三个都补了用例。

## 2. 验收：一条流水线走完（真机粘贴）

```
# 构建（provenance 必须关，见 §3.5）
$ make images IMAGE_TAG=0.1.0-p2
$ docker push swr.ap-southeast-3.myhuaweicloud.com/digital-finance/asas-sensor:0.1.0-p2
0.1.0-p2: digest: sha256:bac7ff554ff1e72bff04d8af230bf490f896791e30655229e2a39aeac1759b38

$ kind load docker-image swr…/asas-sensor:0.1.0-p2 --name desktop
$ kubectl -n asas create secret docker-registry swr-creds --docker-server=swr… (本机 docker 凭据)
$ helm upgrade --install asas deploy/k8s/helm/asas -n asas \
    -f deploy/k8s/helm/asas/values-local.yaml -f /tmp/asas-sensor-local.yaml \
    --set sensor.image.digest=sha256:bac7ff55… \
    --set 'image.pullSecrets[0].name=swr-creds' --wait
DESCRIPTION: Upgrade complete
```

**绿路径**（豁免写全之后，调度器自己触发的那一趟）：

```
$ kubectl -n asas logs asas-sensor-29831315-tvtgw --tail=20
==> 采集 /scan（只读；不执行任何 server）
==> 跳过 introspect（默认）：工具名将记为 unknown，而不是猜一个
已生成交付目录：/data/sensor/delivery
ASAS-A 凭据：/data/sensor/delivery/attestation.json
  已评估规则 6 条 · 未评估 1 条
  ✅ hashMatch            ✅ narrowingOnly        ✅ noUndeclaredUnknown
  ✅ everyVerdictHasBasis ✅ allPinnedOrExempt    — silenceIsAuditable（无事件流）
  ✅ withinValidityWindow
  unknown 声明 40 条（拿不到的事实，不填假值）
  离线模式：未出网取制品哈希（如需要，去掉 --offline 或设 RATCHET_OFFLINE=0）
ratchet 0.12.2 — 一次交付
  ✅ scan      1 harnesses, 16 servers, 12 unpinned
  ⚠️ observe   no call records yet — run your agents with the hook attached
  ✅ policy    16 tools: 0 allow, 16 approve, 0 deny
  ✅ report    /data/sensor/delivery
  ✅ attest    /data/sensor/delivery/attestation.json（已通过 ASAS-V 自查）
==> 上报 /data/sensor/delivery/attestation.json → http://asas-api:8080
   已入账：{"id": "kind-demo-2026-10-20", "storedAt": "2026-09-20T04:35:01.877750+00:00"}
==> 控制面验证（与本地自查是同一份规则）
   ✅ 通过：控制面与本地自查一致
```

**红路径**（同一台机器，不写豁免就跑）：

```
  ❌ allPinnedOrExempt  mcp-server-amap: 未定版且没有豁免期限
  ⚠️ attest    ASAS-A 凭据未通过自查，交付不算完成（见上面的规则输出）
==> 上报 … 已入账：{"id": "kind-demo-2026-10-20", …}
==> 控制面验证（与本地自查是同一份规则）
   ❌ 不通过：凭据没过 ASAS-V，交付不算完成（原因见下）
{"ok": false, … "allPinnedOrExempt", "status": "fail", "details": ["mcp-server-amap: 未定版且没有豁免期限", … 共 11 条]}
容器退出码 = 1（Job 状态 Error，失败历史保留 3 条）
```

两条路径都符合预期：**凭据入账了**（失败也是证据，不能悄悄丢掉），而失败以非 0 退出
让调度器与看板看得见。本机 16 个 server 里 12 个未定版是真实状态，凭据里每个都带
`pinned: false` 与 `exemptUntil`（豁免写进凭据本身，到期即失效）。

## 3. 这一轮真正的设计产出

### 3.1 凭据里区分"没试"和"试了没成"

`unknown[].why` 现在会写明是哪一道闸挡下的：

```json
{"what": "mcp-server-docker: contentHash",
 "why": "已设为离线（RATCHET_OFFLINE=1），未出网取 mcp-server-docker 的制品；据此声明 unknown"}
```

理由：审计的人要能分开"这份凭据的能力边界"和"这次运气不好"。两者混成一句
"取不到"，等于把我们的选择藏进环境噪声里。

### 3.2 无人值守的采集，"等"不出结论

本机实测：kind 里的采集卡在 `SYN_SENT`，每个请求各等 20 秒。一趟下来好几分钟，
**而结论还是 unknown**。所以出网变成有界动作：

* `RATCHET_OFFLINE=1` —— 一步都不出网（chart 默认开）。企业网络通常不给传感器放行
  registry；"传感器偷偷往外发请求"本身就是客户内网里的一个噪声源。
* `RATCHET_HTTP_BUDGET=60` —— 整趟出网的总预算，用完即停，剩下的照记 unknown。

**离线不是降级**：unknown 在规范（ASAS-A schema）里是一等公民，凭据会写明为什么没取。

### 3.3 失败必须响，不能"安静地跑完"

原来的脚本用 `curl … | head -c 300` 收尾。管道的退出码是 `head` 的——控制面返回 422 时
脚本照样 exit 0，Pod 报 **Completed**。现在显式取 HTTP 状态码并分情况收尾：
200 报通过、422 打出全部失败规则并非 0 退出、连不上控制面说明凭据留在本地未入账。

### 3.4 "默认不执行"要有结构性保证，不只是开关

除了 `--introspect` 要配 `--allow-exec`、容器里多一道 `ASAS_INTROSPECT=1`，还有一条更硬的：

```
$ docker run --rm --entrypoint sh asas-sensor:0.1.0-p2 -c 'command -v node npx npm'
node/npx/npm: 都不存在
$ id
uid=10001(sensor) gid=10001(sensor) groups=10001(sensor)
```

**镜像里根本没有 node/npx**。MCP server 几乎清一色是 `npx -y <包>`，所以"采集会执行
配置里的命令"在这套镜像里不是"我们保证不会"，而是**做不到**。工具面因此记为
`reachableTools: null` 并逐条进 `unknown`（16 个 server 各一条），而不是猜一个数。

### 3.5 华为 SWR 不接受带 attestation 的镜像索引

```
$ docker push swr.ap-southeast-3.myhuaweicloud.com/digital-finance/asas-sensor:0.1.0-p2
error from registry: Invalid image, fail to parse 'manifest.json'
```

`docker build` 默认给镜像加 provenance/SBOM，产物是 OCI index，SWR 解析不了。
已固化成 Makefile 目标（`make images` 带 `--provenance=false --sbom=false`，
`make push-images` 打完顺手打印两个 digest）。digest 才是部署时该钉的东西：
同一个 tag 可以被推成另一个镜像，digest 不会。

### 3.6 两个"配好了"的假象

| 假象 | 真相 | 现在 |
|---|---|---|
| `sensor.sh` 传了 `--org` | `deliver` 没有这个 flag（只有 `--client`），容器 exit 2 | 用例锁住"脚本用的 flag 必须真存在" |
| `api.persistence.enabled` 是个开关 | 模板从不读它，关掉照样渲染 PVC 并挂载 | 模板真的读它；关掉走 emptyDir（有用例） |
| `--set sensor.exempt="a=1,b=2"` 配了俩豁免 | Helm 用逗号分隔赋值：只生效第一项，其余变成垃圾顶层键 | `templates/values-guard.yaml` 渲染期直接 fail 并说明怎么改 |
| `values-remote.yaml` 里写了 sensor 镜像 tag | 写在了 `image.sensor` 下，而模板读 `sensor.image`：渲染/部署都成功，用的是占位 tag | 引用写对位置；用例改成"values 里的 tag 与 digest 必须真的进模板" |

## 4. P2 判据逐条对照

| 判据（DEVELOPMENT-PLAN §2） | 结果 | 证据 |
|---|---|---|
| `--introspect` 需显式 `--allow-exec` | ✅ | `cmdScan` 拒绝路径 exit 2；脚本侧 `ASAS_INTROSPECT=1` 才加（用例） |
| 镜像以固定 digest 运行 | ✅ | 渲染出 `asas-sensor@sha256:bac7ff55…`；Pod 成功拉取并跑完 |
| 上报仅元数据 | ✅ | 凭据结构里没有放参数值的位置；schema 保证（见 [ASAS-A-v0.1.schema.json](../spec/ASAS-A-v0.1.schema.json)） |
| 本地 k8s 部署验证 | ✅ | kind `ns=asas`：CronJob 按 `*/5` 自己触发并完成 |
| 控制面与本地一致 | ✅ | 见 §5 |
| 测试全绿 | ✅ | Python 93 项 + Go 9 包（`make test`） |

## 5. 一致性验收（P1 判据，用传感器产的凭据复验）

```console
$ curl -sS http://127.0.0.1:18080/attestations
{"attestations": [{"id": "kind-demo-2026-10-20", "org": "kind-demo", …}]}

$ curl -sS http://127.0.0.1:18080/attestations/kind-demo-2026-10-20 -o /tmp/att-from-cp.json
$ curl -sS -X POST http://127.0.0.1:18080/verify -H 'Content-Type: application/json' \
    --data-binary @/tmp/att-from-cp.json -o /tmp/verify-cp.json -w '%{http_code}\n'
200

$ PYTHONPATH=service/src python3 -c "… asas.verify(att, files=None, events=None) …"   # 本地同一份实现
本地验证 ok = True | 已评估 5 条 · 未评估 2 条
$ diff -u /tmp/verify-local.json /tmp/verify-cp-norm.json
✅ 逐字一致（本地 asas.verify 与 /verify 输出完全相同）
```

台账落在 PVC 上（不是容器层）：

```console
$ kubectl -n asas exec deploy/asas-api -- sh -c 'ls -l /data; df -h /data | tail -1'
-rw-r--r-- 1 asas asas 45056 Sep 20 04:35 asas.db
/dev/sda2  492G  142G  325G  31% /data
```

挂载面（宿主机家目录只读）：

```console
$ kubectl -n asas get pod <sensor-pod> -o jsonpath='{range .spec.containers[0].volumeMounts[*]}{.name}{" -> "}{.mountPath}{" readonly="}{.readOnly}{"\n"}{end}'
host-scan -> /scan readonly=true
data      -> /data readonly=
tmp       -> /tmp  readonly=
```

## 6. 已知限制（不粉饰）

1. **kind 节点看不到宿主机家目录**：kind 的节点是容器，hostPath 只能指向节点容器里
   存在的路径。本机验收是把真实 harness 配置（`.codex/config.toml`、`.codex/hooks.json`）
   放进节点容器再挂载。**生产集群的节点就是宿主机**，这一条不成立；
   但"受管机器的家目录必须挂得进来"是部署时的硬前提，已写进 chart 注释。
2. **`hashMatch` 在控制面侧是"未评估"**：控制面只收到凭据、没有证据文件，无法重算哈希。
   要让它可评估，需要把证据文件一起上传——P4 的范围。
3. **`silenceIsAuditable` 同样未评估**：没有事件流（hook 未接）。这是本机真实状态
   （`no call records yet`），凭据里如实记为"未评估"，不是"通过"。
4. **工具面 16 条 unknown**：`--introspect` 默认关，所以拿不到工具名。关闭是刻意的
   （见 §3.4），代价就是凭据少一块信息——**这是取舍，不是遗漏**。
5. **本机 12 个未定版 server 用了书面豁免**：豁免只是把"未定版"变成"已知并接受的未定版"，
   到期（2026-12-31）后凭据会重新变成不通过。

## 7. 下一轮（P4）

| # | 事项 | 判据 |
|---|---|---|
| T16 | 委派链校验：子 agent 不得放大父 agent 的权限 | 构造一个放大权限的委派，被拒绝并给出依据 |
| T17 | 遏制编排：`/contain` 记录 + "哪些 agent 曾触达 X" 可查 | 一条 CLI 命令能查出触达链路 |
| T18 | 证据上传（让 `hashMatch` 在控制面可评估） | 控制面能重算哈希并与凭据里的值比对 |

## 8. 复现这一次验收

验收用的叠加值（**只是本机的路径与豁免，不是产品默认值**）。注意豁免写成 values
文件而不是 `--set`——逗号在 `--set` 里是赋值分隔符（§3.6）：

```yaml
# /tmp/asas-sensor-local.yaml
sensor:
  enabled: true
  schedule: "0 9 * * *"
  org: kind-demo
  hostScanPath: /var/asas-scan      # kind 节点里能看到的那棵树（见 §6.1）
  scanHome: /scan
  image:
    tag: "0.1.0-1ca26f9"
    digest: "sha256:bac7ff554ff1e72bff04d8af230bf490f896791e30655229e2a39aeac1759b38"
  podSecurityContext:               # 宿主家目录属于 uid 1000，容器得用同一个身份读
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
  exempt: "chrome-devtools=2026-12-31,mcp-server-amap=2026-12-31,mcp-server-context7=2026-12-31,mcp-server-docker=2026-12-31,mcp-server-firecrawl=2026-12-31,mcp-server-gitee=2026-12-31,mcp-server-kubernetes=2026-12-31,mcp-server-memory=2026-12-31,mcp-server-playwright=2026-12-31,mcp-server-st=2026-12-31,superpowers=2026-12-31,xapi=2026-12-31"
```

```bash
# 1) 真实 harness 配置进节点（kind 的节点是容器，宿主机家目录不在里面）
docker exec desktop-control-plane mkdir -p /var/asas-scan
docker cp <真实家目录的 .codex/.claude 等配置>/. desktop-control-plane:/var/asas-scan/
docker exec desktop-control-plane chown -R 1000:1000 /var/asas-scan

# 2) 拉取凭据（SWR 是私有的，401 就是缺这一步；本机 docker 已登录）
python3 - <<'PY' | kubectl apply -f -
import base64, json, os
cfg = json.load(open(os.path.expanduser('~/.docker/config.json')))
srv = 'swr.ap-southeast-3.myhuaweicloud.com'
blob = base64.b64encode(json.dumps({"auths": {srv: {"auth": cfg["auths"][srv]["auth"]}}}).encode()).decode()
print(f"apiVersion: v1\nkind: Secret\nmetadata:\n  name: swr-creds\n  namespace: asas\ntype: kubernetes.io/dockerconfigjson\ndata:\n  .dockerconfigjson: {blob}")
PY

# 3) 部署 + 立刻跑一趟（不等定时）
helm upgrade --install asas deploy/k8s/helm/asas -n asas --create-namespace \
  -f deploy/k8s/helm/asas/values-local.yaml -f /tmp/asas-sensor-local.yaml \
  --set sensor.image.digest=sha256:bac7ff55… --set 'image.pullSecrets[0].name=swr-creds' --wait
kubectl -n asas create job --from=cronjob/asas-sensor sensor-accept
kubectl -n asas logs job/sensor-accept -f
```

**当前本机集群的状态**（留在那儿就能继续验）：`ns=asas` 里 `asas-api` 1/1 Running、
PVC Bound、`asas-sensor` CronJob 每天 09:00（Asia/Shanghai）跑一趟。
不想让它跑：`helm upgrade asas deploy/k8s/helm/asas -n asas -f deploy/k8s/helm/asas/values-local.yaml --set sensor.enabled=false`。
