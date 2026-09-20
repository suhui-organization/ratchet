# 本机 k8s 人工验证清单

> 这份文件是给**人**照着做一遍的，不是给 CI 跑的。
> 每一步都给「做什么 / 应该看到什么 / 看到别的说明什么」。
> 全程只用到两个东西：仓库里的 CLI，和本机 kind 集群里的控制面。

## 0. 现在跑着什么

| 东西 | 在哪 | 地址 |
|---|---|---|
| 控制面 `asas-api` | 本机 kind 集群 `ns=asas` | **http://127.0.0.1:30090** |
| 数据库 | 该 namespace 的 PVC（`asas-data`） | 重启 Pod 不丢 |
| 传感器 CronJob | 同 namespace，每天 09:00 | 默认关在 chart 里，本地这份是开着的 |
| CLI | 仓库 `bin/ratchet`（或 GitHub Release） | `ratchet version` → 0.13.0 |

地址为什么是 `127.0.0.1:30090` 而不是集群 IP：kind 的节点是 docker 容器，
**宿主机打不到它的 IP**（实测），所以用一个常驻的转发容器接出来。重建它：

```bash
make local-gateway          # 幂等；关终端、重启机器都还在
```

整套部署也是**一条命令**（幂等，改了再跑一次即可）：

```bash
make deploy-local           # helm upgrade --install，含传感器与豁免清单
```

## 准备（一次）

```bash
cd /home/walden/Workspaces/ratchet
export PATH="$PWD/bin:$PATH"          # 没有 bin/ratchet 就先 make build
export PYTHONPATH="$PWD/service/src"  # Python 侧（出证/验证）在这里
export ASAS_API=http://127.0.0.1:30090
# 本机到 npm registry 不通，出网取制品哈希会白等（默认预算 60 秒）。传感器默认也是离线。
export RATCHET_OFFLINE=1
curl -sS $ASAS_API/healthz && echo    # 应打印 {"status": "ok"}
```

## 1. 控制面活着吗

```bash
kubectl -n asas get deploy,pod,svc,cronjob
curl -sS $ASAS_API/healthz
```

**应该看到**：`asas-api` 1/1 Running；Service 是 NodePort `8080:30090`；`{"status":"ok"}`。

## 2. 采一台机器（本机真实数据），出一条凭据

```bash
ratchet deliver --out /tmp/verify --home "$HOME" --client acme \
  --report-cmd "python3 -m ratchet_service.cli"
```

**应该看到**：5 步全 ✅（`scan / observe(可能⚠️) / chain(可能没有) / policy / report / attest`），
最后一行是 `attest …（已通过 ASAS-V 自查）`。

凭据里的 agent 名 = 这台机器的**短主机名**（没给 `--agent` 时）。想换成别的：
`--agent worker-1`。**名字不能留空**：空名或字面量 `agent` 对每台机器都一样，
而"这台机器是谁"是凭据的第一条规定（ASAS-1.1）。

| 看到 | 说明什么 |
|---|---|
| `observe ⚠️ no call records yet` | 这台机器没接 hook。事件流因此是空的，V6 会记「未评估」——**不是通过** |
| `attest ⚠️ 未通过自查` | 有未定版组件且没写书面豁免。凭据本身仍然产出，但不算交付 |

## 3. 看凭据里到底写了什么

```bash
python3 -m json.tool /tmp/verify/delivery/attestation.json | head -60
```

**重点看四处**（这是这份格式的全部要点）：

* `unknown[]`：拿不到的事实有没有**逐条声明**（不是省略）；
* `verdicts[].basis`：每条 allow/approve/deny 有没有依据；
* `assets[].pinned` + `exemptUntil`：未定版的是不是都有书面豁免和到期日；
* `manifest.evidence[].sha256`：证据文件的哈希（改文件就会露馅）。

## 4. 交到控制面：让它自己验一遍（这一步是 server 存在的理由）

下面这段做三件事：先**只交凭据**验一遍，再**把证据传上去**验第二遍。
注意两遍的差别——同一个凭据、同一套规则，只是输入齐了。

```bash
python3 - <<'PY'
import base64, json, os, urllib.request

api, d = os.environ["ASAS_API"], "/tmp/verify/delivery"

def post(path, payload):
    req = urllib.request.Request(api + path, data=json.dumps(payload).encode(),
                                 headers={"Content-Type": "application/json"}, method="POST")
    try:
        return urllib.request.urlopen(req).read().decode()
    except urllib.error.HTTPError as exc:      # 422 = 凭据没通过，不是接口坏了
        return exc.read().decode()

att = json.load(open(f"{d}/attestation.json"))
ident = json.loads(post("/attestations", att))["id"]
print("台账 id =", ident)

def show(title, raw):
    report = json.loads(raw)
    print(f"\n{title}  ok={report['ok']}  未评估={report['notEvaluated'] or '（无）'}")
    for r in report["results"]:
        mark = {"pass": "✅", "fail": "❌", "not_evaluated": "—"}[r["status"]]
        print(f"  {mark} {r['rule']:<22}{r['details'][0] if r['details'] else ''}")

show("第一次（只有凭据）", post("/verify", att))

files = {i["file"]: base64.b64encode(open(f"{d}/{i['file']}", "rb").read()).decode()
         for i in att["manifest"]["evidence"] if os.path.isfile(f"{d}/{i['file']}")}
print("\n上传证据：", post("/evidence", {"attestationId": ident, "files": files}))
show("第二次（证据齐了）", post("/verify", att))
PY
```

**应该看到**：第一遍 `hashMatch` 是 `—`（控制面手里还没有证据文件，这是**正确的三态**，
不是失败）；第二遍变成 `✅`。

`ok=False` 也是**预期的**：这台机器有 12 个未定版 MCP server 且没写书面豁免，
`allPinnedOrExempt` 判失败并点名第一个。这说明规则真的在工作。

### 4b. 写书面豁免之后（看它变成全绿）

未定版不是不能接受，但**必须书面写明到期日**（开放式豁免不合规）。豁免清单直接从凭据自己
的 `assets` 里生成，省得手抄：

```bash
EX=$(python3 -c "
import json
att = json.load(open('/tmp/verify/delivery/attestation.json'))
print(','.join(f\"{a['name']}=2026-12-31\" for a in att['assets'] if a.get('pinned') is False))")
ratchet deliver --out /tmp/verify-ex --home "$HOME" --client acme --exempt "$EX" \
  --report-cmd "python3 -m ratchet_service.cli" | sed -n '/已评估规则/,/unknown 声明/p'
```

**应该看到**：`allPinnedOrExempt` 变 ✅，`ok` 变 True（只剩两条"没有输入"的未评估：
遏制记录与事件流）。凭据里每个 server 会多一个 `exemptUntil` 字段——豁免是写进凭据的，
收货方看得见，到期即失效。

## 5. 问一句：谁能碰到某个资产（ASAS-8.3）

先从凭据里挑一个资产名：

```bash
python3 -c "import json;print([a['name'] for a in json.load(open('/tmp/verify/delivery/attestation.json'))['assets']][0])"
ratchet reach --api $ASAS_API --subject <上面那个名字>
```

**应该看到**：授权面 / 观测面 / 拒绝分列，每条带 `依据：<凭据 id>`。
如果台账里没有覆盖这个资产，会明确写「**不等于没人能碰它**」。

## 6. 传感器：一条命令走完 采集→出证→上报→传证据→传事件→复验

```bash
kubectl -n asas create job --from=cronjob/asas-sensor sensor-manual
kubectl -n asas logs -f job/sensor-manual
```

**应该看到**：`scan → observe → chain → policy → report → attest` 之后，
`已入账：{"id": …}` → `上传证据文件` → `上传事件流` → `✅ 通过：控制面与本地自查一致`。

如果日志里出现 `❌ 检出断流`，说明被扫描那棵家目录里的日志被改过——
这是**设计如此**（见 `spec/ASAS-v0.1.md` §6.3）。

## 7. 遏制：先看会连累谁，再决定动不动手

```bash
ratchet contain --api $ASAS_API --agent <某个 agent id>
```

**应该看到**：只打印预告**不做任何事**，并给出确认命令。
加 `--yes` 才真的吊销；如果这个 agent 有下级，会一起列出来（ASAS-8.5）。

```bash
curl -sS $ASAS_API/contain | python3 -m json.tool   # 遏制记录 + 完整委派图
```

## 8. 篡改演示（最能说明"沉默可被审计"这一步）

前提：被扫描的家目录里有调用记录（`<home>/.ratchet/calls.jsonl`）。

```bash
# 1) 先跑一趟，记住日志里那行摘要
kubectl -n asas create job --from=cronjob/asas-sensor sensor-before
kubectl -n asas logs job/sensor-before | grep 摘要

# 2) 改一条（把 deny 改成 allow 最直观）
docker exec desktop-control-plane sh -c \
  "sed -i 's/\"decision\":\"deny\"/\"decision\":\"allow\"/' /var/asas-scan/.ratchet/calls.jsonl"

# 3) 再跑一趟
kubectl -n asas create job --from=cronjob/asas-sensor sensor-after
kubectl -n asas logs job/sensor-after | tail -6
```

**应该看到**：`❌ 检出断流：本地日志被改写：N 条与上次不一致（seq …）`，
Pod 状态 `Error`（非 0 退出），`curl -sS $ASAS_API/silence` 里有一条带
「上次的摘要 → 这次的摘要」的记录。

**注意**：同一趟的凭据自己仍然 `pass`——链是自洽的。改写的发现靠的是**跨次比对**，
这就是为什么需要一个持续存在的台账（规范 §6.3）。

## 9. 清理

```bash
kubectl -n asas delete job sensor-manual sensor-before sensor-after
helm uninstall asas -n asas            # 连 PVC 一起删（台账数据也会没）
docker rm -f asas-local-gateway        # 关掉固定地址的转发
```

**只想重置台账、保留部署**：把数据一起删掉再装回来（PVC 归 Helm 管，删了要重新 apply）：

```bash
kubectl -n asas scale deploy/asas-api --replicas=0
kubectl -n asas delete pvc asas-data
make deploy-local
```

## 这张清单**没有**覆盖的（别误以为验过了）

* 多租户与计费：没有。现在是一套台账、一个组织。
* 认证：**控制面没有任何认证**。这份清单里的地址只能在本机用，不要暴露出去。
* 公网/生产可用性：单副本 + sqlite，没有备份、没有 HA、没有 TLS。
* 日志轮转：归档后重新开链会产生一条"断流"记录（规范尚未定义轮转声明）。
