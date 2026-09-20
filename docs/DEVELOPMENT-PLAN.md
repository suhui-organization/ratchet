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
| **P2** | 传感器容器化 | `asas-sensor` sidecar，默认不执行 | `--introspect` 需显式 `--allow-exec`；镜像以固定 digest 运行；上报仅元数据 |
| **P3** | CI 门禁 | GitHub Action + SARIF + 非 0 退出码 | 未定版新增会让 PR 变红；SARIF 能在 Security 标签页显示 |
| **P4** | 委派边界与遏制 | 委派链校验 + 吊销编排 | 子 agent 放大权限被拒；"哪些 agent 曾触达 X"能查到 |

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
| `deliver` 接线（T2） | ⏳ 本轮之后立刻做 |
| 本地 k8s 控制面（P1） | ⏳ 下一轮 |
