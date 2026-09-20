# 开发流程与计划（ASAS 控制面）

> 这份文件的目的是消除"断断续续"。约束很简单：
> **每一轮必须走完同一条流水线，任何一环没闭环就不开下一轮。**

## 0. 唯一的流程（每轮都走这条，不允许跳步）

```
① Spec 更新      → ② 实现（一个可独立验收的切片） → ③ 测试（含一致性向量）
        → ④ 本地 k8s 部署 → ⑤ 验收（按判据逐条打勾） → ⑥ 落档（DELIVERY-*.md）→ 下一轮
```

**硬约束（违反即停下）：**

1. **一轮只动一个切片**——不允许"顺手把另一个也改了"。
2. **没有测试的代码不算实现**；校验类逻辑必须有对应的失败用例。
3. **没有部署的切片不算完成**——本地 k8s 跑不起来就不算做完。
4. **判定逻辑只有一份**：Go 采集/编译，Python 出证与验证；**同一规则不许在两个语言里各写一遍**。
5. **每轮结束时仓库必须可部署、测试全绿、`docs/DELIVERY-*.md` 有记录**。

## 1. 目标形态（一次性说清，后面不再改）

```
客户环境                        控制面（本地 k8s 优先）
┌──────────────────┐           ┌─────────────────────────────┐
│ Agent / MCP      │           │ asas-api    台账/策略/凭据   │
│      ↑ 只读观察   │  元数据    │ asas-ingest 接收传感器事件   │
│ Sensor(sidecar)  │ ────────▶ │ asas-verify 离线验证器(同一份)│
│  容器 / daemon   │           │ asas-web    台账与验收 UI     │
└──────────────────┘           │ postgres    （单租户，本地盘）│
                               └─────────────────────────────┘
```

**不做**（本阶段明确排除）：多租户计费、跨客户情报、拦截网关、Windows 传感器。

## 2. 阶段划分与验收判据

| 阶段 | 切片 | 交付物 | 验收判据（必须可执行） |
|---|---|---|---|
| **P0**（本轮） | 凭据闭环 | `deliver` 产出 ASAS-A + 自查 | `ratchet deliver` 结束时报"凭据自查通过"；产物通过 `asas.py` 全部可评估规则 |
| **P1** | 控制面骨架上本地 k8s | `asas-api` + `asas-verify` + postgres 三个 Deployment | `kubectl -n asas get deploy` 全 Ready；`POST /attestations` 存取一份凭据；`POST /verify` 返回六条规则结果 |
| **P2** ✅ | 传感器容器化 | `asas-sensor` sidecar，默认不执行 | 见 §8（digest 固定运行、只读挂载、默认不出网也不执行） |
| **P3** | CI 门禁 | GitHub Action + SARIF + 非 0 退出码 | 未定版新增会让 PR 变红；SARIF 能在 Security 标签页显示 |
| **P4** ✅ | 委派边界与遏制 | 跨凭据委派校验 + 吊销编排 + 证据上传 | 见 §9（放大权限被拒并点名资产；触达查询带依据；控制面能自己重算哈希） |

**每阶段的"完成"定义**：spec 更新 + 实现 + 测试全绿 + **本地 k8s 部署验证** + DELIVERY 记录。

## 3. P0 的任务分解（本轮执行）

| # | 任务 | 判据 |
|---|---|---|
| T1 | `asas_build` 从 `deliver` 产物生成 ASAS-A | ✅ 已完成（66 项测试） |
| T2 | `deliver` 调用 `asas-build` → `asas-verify`，不过则不算交付 | 命令行输出出现"凭据自查通过/未通过"两态之一 |
| T3 | 扫描侧补齐 `pinned` / `version` / 制品哈希（能拿到的那部分） | 凭据里这些字段不再是 `null`，`unknown` 相应变短 |
| T4 | `docs/DELIVERY-0.15.0.md` 记录本轮 | 有验收输出粘贴在文档里 |

## 4. P1 的任务分解（下一轮，本地 k8s）

| # | 任务 | 判据 |
|---|---|---|
| T5 | Helm chart `asas/`（api + verify + postgres） | `helm template` 渲染通过；`values-remote.yaml` 单一事实源 |
| T6 | 两个镜像进 SWR（沿用现有流水线） | `build-push` + `release.sh` 一条命令跑完，漂移检查 0 |
| T7 | `postgres` 用本地盘 PV（与 FinHarness 同款 storageClass） | 数据落盘后重建 Pod 仍在 |
| T8 | `POST /attestations` + `POST /verify` 两个端点 | `curl` 能存取与验证；**验证结果与 Python 验证器逐字一致** |
| T9 | 部署到本地 k8s 并留档 | `kubectl -n asas get all` 截图/输出进 DELIVERY |

## 5. 环境与命令（本地 k8s 优先）

```bash
# 本地集群（服务器上的 k8s，ns=asas）
export KUBECONFIG=/etc/kubernetes/admin.conf
helm upgrade --install asas deploy/k8s/helm/asas -n asas \
  -f deploy/k8s/helm/asas/values-local.yaml

# 镜像走现有 SWR 流水线（不新增任何新工具）
./deploy/swr/build-push.sh <tag>
./deploy/swr/release.sh <tag>
```

**部署目标顺序**：本地 k8s（`ns=asas`）→ 验证通过后 → 才考虑公网。

## 6. 风险与止损

| 风险 | 触发信号 | 动作 |
|---|---|---|
| 控制面越做越大 | P1 超过两轮仍没部署成功 | 砍掉 web UI，只留 api + verify |
| 传感器装不上 | 试点客户安全评审不通过 | 回退到"CLI 按需 + 凭据"，放弃持续采集 |
| 规范没人认 | 3 次评审都拿不到"可以当验收标准" | 降级为产品格式，不再自称标准 |
| 需求不存在 | 60 天 0 付费 | 按 GTM 止损线，拆成开源组件 |

## 7. 当前进度快照

| 切片 | 状态 |
|---|---|
| ASAS v0.1 规范 + schema | ✅ |
| ASAS-V 验证器 + 20 条向量 | ✅ |
| ASAS-A 构建器 | ✅ |
| `deliver` 接线（T2） | ✅ |
| 本地 k8s 控制面（P1） | ✅ 部署在 kind `ns=asas`，`/verify` 与本地逐字一致 |
| 传感器容器化（P2） | ✅ 见 §8 |
| CI 门禁（P3，提前做掉） | ✅ `.github/workflows/asas-gate.yml` |
| 委派边界与遏制（P4） | ✅ 见 §9 |
| **待定**：这件事还差一步才算"能卖" | `spec/` 的两条新规则需要外部评审；评审稿见 [ASAS-SPEC-for-review.md](ASAS-SPEC-for-review.md) |

## 8. P2 的任务分解与结论（已完成）

| # | 任务 | 状态 | 判据 / 证据 |
|---|---|---|---|
| T10 | 传感器脚本 + 镜像（采集 → 出证 → 上报） | ✅ | `deploy/sensor/sensor.sh`、`deploy/docker/sensor.Dockerfile`；镜像 `asas-sensor:0.1.0-p2` |
| T11 | 集群内定时任务（默认关） | ✅ | `templates/sensor-cronjob.yaml`；`sensor.enabled=false` 时渲染结果里没有 CronJob（有用例） |
| T12 | 默认不执行配置里的命令 | ✅ | 需要 `--allow-exec`；`ASAS_INTROSPECT=1` 才加 `--introspect`（有用例锁住顺序） |
| T13 | 镜像以固定 digest 运行 | ✅ | SWR 侧 digest `sha256:bac7ff55…` 写进 `sensor.image.digest`，Pod 起来并跑通 |
| T14 | 出网有界（离线 / 预算） | ✅ | `RATCHET_OFFLINE=1`、`RATCHET_HTTP_BUDGET=60`；两条闸各有用例 |
| T15 | 落档 | ✅ | [DELIVERY-0.16.0.md](DELIVERY-0.16.0.md) |

**P2 的验收输出**（真机粘贴见 DELIVERY-0.16.0.md）：一次集群内采集把凭据存进控制面
（`{"id":"kind-demo-2026-10-20"}`），控制面 `/verify` 与本地 `asas.verify` 输出逐字一致。

**这一轮学到的三件事**（都写成了用例，不靠人记）：

1. 脚本里的 flag 必须真存在——`--org` 那次让容器 exit 2，采集全灭。
2. **管道会吞掉退出码**：`curl … | head` 的退出码是 `head` 的，控制面返回 422 时
   Pod 照样报 Completed。判断必须看 HTTP 状态码。
3. `--set` 用逗号分隔赋值：`sensor.exempt="a=1,b=2"` 会被截断成一条豁免 + 一堆垃圾顶层键。
   现在渲染期直接报错（`templates/values-guard.yaml`）。

## 9. P4 的任务分解与结论（已完成）

| # | 任务 | 状态 | 判据 / 证据 |
|---|---|---|---|
| T16 | 委派边界：子不得放大父的权限 | ✅ | V8 `delegationNarrows`：作用域 ⊆、deny 只增不减、子不得比父活得久；**父凭据不可得 = 未知委派链 = 越权**。真机：`worker-1: 委派放大 ['secrets']` → 控制面 422 |
| T17 | 遏制编排：吊销 + 溯源 | ✅ | `/contain` 级联（记 `via`）、`--dry-run` 预演、`ratchet contain`、`ratchet reach --subject X`（授权面 / 观测面 / 拒绝 / 遏制 / 委派链 / 依据） |
| T18 | 证据上传，让控制面能重算哈希 | ✅ | `POST /evidence`；传感器自动上传；真机：`hashMatch` 由"未评估"变 **pass**，9 条规则 8 条已评估（只剩事件流） |
| T19 | 落档 | ✅ | [DELIVERY-0.17.0.md](DELIVERY-0.17.0.md) |

**这一轮学到的三件事**（都写成了用例）：

1. **台账主键只用"组织+日期"会互相覆盖**——父与子两份凭据只留下一份，
   "谁能触达 X"当场少一半答案。现在主键含内容哈希：同一份凭据重复上报仍然幂等，
   不同主体各占一行。
2. **V2 曾把跨文档委派判成"未知委派链"**：那会把合法架构判违规，还让 V8 永远轮不到。
   分工写清楚了——V2 只管文档内，V8 管跨凭据，拿不到父凭据才是越权。
3. **默认值不许生产出失败**：父子两份凭据相隔几秒生成时，默认委派到期会比父晚几秒，
   于是一份默认生成的凭据直接违规。现在默认取"自己有效期与父到期的较早者"。
