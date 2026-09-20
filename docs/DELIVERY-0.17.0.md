# 交付记录 0.17.0 —— P4：委派边界、遏制编排、证据上传

> 日期：2026-09-20。上一版见 [DELIVERY-0.16.0.md](DELIVERY-0.16.0.md)。
> 按 [DEVELOPMENT-PLAN.md](DEVELOPMENT-PLAN.md) 的 P4 切片执行：**规范 → 实现 → 测试 → 本地 k8s 部署 → 验收 → 落档**。
> 判据逐条见 §4；这一轮的三处实测缺陷见 §3.6。

## 1. 一句话

把"委派"从一种**自述**变成一种**要交出凭据链才算数的行为**，并让吊销这件事
沿委派链走下去；同时把证据上传补上，让控制面第一次能把 `hashMatch` 从"未评估"变成"通过"。

## 2. 规范先改（v0.2）

初稿漏掉了一半：**委派是跨主体的**。补进 [spec/ASAS-v0.1.md](../spec/ASAS-v0.1.md)：

| 新增 | 内容 |
|---|---|
| **ASAS-8.5 (MUST)** | 遏制 MUST 沿委派链向下级联 |
| **V8 `delegationNarrows`** | 跨凭据收窄：作用域 ⊆、deny 只增不减、子不得比父活得久；**父凭据不可得 = 未知委派链 = 越权** |
| **V9 `containmentCascades`** | 被遏制 agent 的下级不得仍持有继承来的权限 |
| §6.1 | 为什么 V8 需要"父"这一份输入——以及"我声称我收窄了"为什么不构成证据 |
| §7 | 测试向量 20 → 27 条 |

ASAS-A 格式**不变**（v0.1 schema 继续有效）。理由：需要新增的只有"父凭据"与
"遏制记录"两份**输入**，凭据本身的字段够用——能不破坏已发出的格式，就不破坏。

## 3. 实现

### 3.1 委派图只有一份实现

[graph.py](../service/src/ratchet_service/graph.py)：把一叠凭据摊成委派边，供**两个消费者**用——
V9 的验证、控制面的遏制与查询。两边各写一遍"父子关系"的代价不是重复代码，
而是出事那天**两套答案**。环会被标记出来而不是静默展开。

### 3.2 控制面在委派里不是"多一个功能"，而是**父凭据在手的那一方**

`/verify` 现在自己从台账取：证据文件、父凭据、遏制记录。规则一行没重写。
纯离线、只收到子凭据的验证方会得到"未知委派链"——**这是刻意的**，
它会逼着委派链一起交付（§6.1）。

### 3.3 遏制：先看连带范围，再动手

```
$ ratchet contain --agent orch-agent --reason "credential leak in CI runner"
吊销 orch-agent 会连累：
  orch-agent（本人）
  worker-1（第 1 层，来自 orch-agent）
  worker-2（第 1 层，来自 orch-agent）

这是预告，**没有执行**。确认要动手：
  ratchet contain --agent orch-agent --reason "credential leak in CI runner" --yes
```

不写 `--yes` 就只是预告。吊销是只会追加、不会撤回的动作，所以默认不给扳机。

### 3.4 触达查询：问题不是"谁被授权了"，而是**依据在哪**

```
$ ratchet reach --api http://… --subject secrets
谁曾能触达 secrets

  能（授权面）1 条：
    🚫 worker-1                 allow   delegated from orch-agent
       └ 依据：acme-2026-10-20-bb67ad27
       └ 委派链：orch-agent → worker-1
  确实用过（观测面）：无记录
  已遏制：worker-1
```

三个设计取舍：

* **授权面与观测面分开列**。两者不一致本身就是结论：能碰但从未用过（可回收的权限）、
  用过但不在授权面里（有人绕过配置）。
* **每条都带依据**（哪份凭据、哪条 basis、哪条委派链）。没有依据的结论在应急时等于没有结论。
* **答不了就说答不了**：台账里没有关于 X 的判定时，输出里会出现
  "台账里没有关于 X 的任何判定——这**不**等于没人能碰它"。空列表和"没有覆盖"是两件事，
  混了会直接误导事故处理。

### 3.5 证据上传：控制面终于能自己算哈希（T18）

```
POST /evidence  {"attestationId": "…", "files": {"policy.json": "<base64>"}}
```

传感器跑完采集会自动上传（只传 manifest 里列过的文件）。上传后控制面重新验证：

```console
$ curl -sS -X POST http://127.0.0.1:18081/verify --data-binary @sensor-att.json
 ok = True · 未评估： ['silenceIsAuditable']
   ✅ hashMatch          ✅ narrowingOnly       ✅ delegationNarrows
   ✅ containmentCascades ✅ noUndeclaredUnknown ✅ everyVerdictHasBasis
   ✅ allPinnedOrExempt  — silenceIsAuditable  ✅ withinValidityWindow
```

9 条规则 8 条已评估。剩下那一条是**真的没有输入**（事件流没接 hook），
不是我们没去查——这正是三态的意义。

### 3.6 这一轮修掉的三处（都是实测出来的）

| 症状 | 根因 | 现在 |
|---|---|---|
| 父与子两份凭据只留下一份，"谁能触达 X"少一半答案 | 台账主键是"组织 + 覆盖日期"，第二个主体覆盖第一个 | 主键加内容哈希前 8 位：同一份凭据重复上报仍幂等，不同主体各占一行 |
| 合法的跨文档委派被 V2 判成"未知委派链" | V2 写在"委派都在同一份凭据里"的假设下 | 分工写清：V2 只管文档内，跨凭据归 V8；拿不到父凭据才是越权 |
| 一份**默认生成**的子凭据直接违规 | 默认委派到期取自己的有效期，而子凭据比父晚几秒生成 | 默认取"自己有效期与父到期的较早者"——默认值不许生产出失败 |

## 4. P4 判据逐条对照

| 判据（DEVELOPMENT-PLAN §2） | 结果 | 证据 |
|---|---|---|
| 子 agent 放大权限被拒 | ✅ | 真机：`worker-1: 委派放大 ['secrets']（不在父 orch-agent 的作用域内）`，控制面 HTTP 422 |
| "哪些 agent 曾触达 X"能查到 | ✅ | `ratchet reach --subject secrets` 输出见 §3.4（带依据与委派链） |
| 遏制是编排（不是记一笔） | ✅ | 吊销 `orch-agent` → 级联 `worker-1`、`worker-2`，每条记录带 `via` |
| 控制面能重算哈希 | ✅ | §3.5：`hashMatch` 由"未评估"变 pass |
| 本地 k8s 部署验证 | ✅ | kind `ns=asas`：`asas-api` 0.1.0-p4b Running；传感器 Job 采集→出证→上报→传证据→复验全绿 |
| 测试全绿 | ✅ | Python 133 项 + Go 10 包（`make test`） |

### 4.1 验收里的红绿两条路径（真机粘贴）

```
$ python3 -m ratchet_service.cli asas --policy child-widened.json --dir child --org acme \
    --delegated-from orch-agent --parent parent/attestation.json
  ❌ delegationNarrows  worker-1: 委派放大 ['secrets']（不在父 orch-agent 的作用域内）

$ python3 -m ratchet_service.cli asas --policy child-narrow.json --dir child2 --org acme \
    --delegated-from orch-agent --parent parent/attestation.json
  ✅ hashMatch  ✅ narrowingOnly  ✅ delegationNarrows  ✅ withinValidityWindow
  — containmentCascades  没有提供遏制记录，无法检查级联

$ curl -X POST …/verify --data-binary @child2/attestation.json      # 证据已上传
HTTP 200  ok = True · 未评估：['silenceIsAuditable']
```

## 5. 一致性：控制面与本地仍是同一份规则

控制面多出来的能力只有**输入**（它手边有台账），判定逻辑仍然只有一份：

* V8 在本地（`--parent`）与控制面（从台账解析父）给出同一句话术，
  都是 `委派放大 ['secrets']（不在父 orch-agent 的作用域内）`；
* 遏制级联的名单只由 `containment.plan_containment` 算一次，
  预演（`--dry-run`）与执行走的是同一个函数。

## 6. 已知限制（不粉饰）

1. **`silenceIsAuditable` 永远需要事件流**：没接 hook 就是"未评估"。这是
   ASAS-4.4/5.3 的活，不是这一轮的。
2. **遏制是"记录"不是"执行"**：本文具会写下吊销事实并级联，但真正的权限回收
   发生在客户的身份系统里。凭据里 `boundaries.revocationMinutes` 是客户**声明**的
   时间窗，我们无法验证它——所以 V9 只能验证"链条走完了没有"。
3. **`reach` 只回答台账**：没被任何凭据覆盖的资产会得到"答不了"。
   把它读成"没人能碰"是使用者最容易犯的错，所以输出里专门写了这句话。
4. **委派父的解析按"同组织 + agent id"**：跨组织的委派会被判成未知委派链。
   这是有意的（跨组织不构成继承），但真实企业里的联合身份还需要一轮设计。

## 7. 下一轮

| # | 事项 | 判据 |
|---|---|---|
| T20 | 把 V8/V9 送给外部评审（评审稿已更新到 8 条） | 至少 1 位不相关的人能照着判，或指出判不了的地方 |
| T21 | 事件流接入（`silenceIsAuditable` 从"未评估"变可评） | hook 采集的事件带序号与前序哈希，断流能被检出 |
| T22 | 跨组织/联合身份的委派模型 | 两个组织之间的委派要么被安全支持，要么在规范里明确排除 |
