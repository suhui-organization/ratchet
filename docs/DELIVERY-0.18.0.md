# 交付记录 0.18.0 —— P5：事件流（可验证的沉默）

> 日期：2026-09-20。上一版见 [DELIVERY-0.17.0.md](DELIVERY-0.17.0.md)。
> 按 [DEVELOPMENT-PLAN.md](DEVELOPMENT-PLAN.md) 的 P5 切片执行：规范 → 实现 → 测试 → 本地 k8s → 验收 → 落档。

## 1. 一句话

**最后一条"未评估"的规则被评起来了**——而且评起来的过程暴露了一个比它更重要的事实：

> **V6 只能证明"这条链内部没被动过"，不能证明"它就是当时那条链"。**

砍尾巴、或者改完重新算链，得到的都是**自洽的链**，规则照样判通过。补上这一半的只有一样东西：
**此前被引用过的摘要**。所以这一轮真正交付的是两件配套的事：**造链** 与 **跨交付比对**。

## 2. 规范先改（v0.3）

原来的规范只说"事件带单调序号与前序哈希"，**没写哈希怎么算**。这一条不写死，
两个实现算出来的链必然不同，标准也就无法被独立实现。补进
[spec/ASAS-v0.1.md](../spec/ASAS-v0.1.md)：

| 新增 | 内容 |
|---|---|
| **§6.2 规范化哈希** | `seq` 从 0；首条 `prevHash` 为空串；`hash = sha256(canonical_json(event \ {hash}))`（键字典序、无多余空格、UTF-8、小写十六进制） |
| **§6.3 跨交付比对** | 单条链自洽是不够的：验证方 MUST 用**重算值**与上次的逐条哈希比对，并把不一致**记为一个事件** |
| 向量 | `spec/testvectors/silence/events.jsonl` —— **跨语言锚点**：Go 造、Python 验，两边各有一个"必须与向量逐字相同"的用例 |

§6.3 里点名的一个实现细节值得单说：**比"重算值"，不是比事件里自称的 `hash`**。
改掉内容却留着旧 `hash`，是最像正常日志的一种篡改；只比自称值会漏掉它（§3.4 有真机证据）。

## 3. 实现

### 3.1 造链在 Go，验链在 Python，**各只有一份**

| 职责 | 位置 | 为什么在这里 |
|---|---|---|
| 造链 | `internal/chain` | 引擎侧：hook 写下的调用记录在这边 |
| 验链 | `asas.rule_silence_auditable` | 收货方侧：验证方要用的是这份实现 |

两边共用 `spec/testvectors/silence/events.jsonl` 作为锚点。**哈希口径漂移**这种错，
在单语言里永远测不出来（各自都自洽），只有跨语言向量能钉住。

### 3.2 `deliver` 自动出事件流，并把它变成证据

```
  ✅ scan      1 harnesses, 1 servers, 0 unpinned
  ✅ observe   3 calls, 0 used, 1 never used, 3 outside the inventory
  ✅ chain     3 events · head 520d66d48cda…
  ✅ policy    1 tools: 0 allow, 1 approve, 0 deny
  ✅ report    /tmp/asas-chain-demo/delivery
  ✅ attest    …（已通过 ASAS-V 自查）
```

事件流写进 `delivery/events.jsonl`，它的哈希进了 manifest——**改事件会被 V1 抓**，
而链本身被 V6 检查。本地自查从"`— silenceIsAuditable（未评估）`"变成 `✅`。

### 3.3 控制面：断流是**事件**，不是一条日志

新增两个端点：

* `POST /events`：上传事件流；与**上次**的逐条哈希比对；
* `GET /silence`：每条链的当前摘要 + 全部断流记录。

断流**不阻止入库**：凭据照收，事实照记。沉默要被看见，不是被拒绝。

### 3.4 真机：把一条 deny 改成 allow，下一趟交付当场露馅

```console
# 事后改本地日志（最典型的一种掩盖）
$ grep -c allow /var/asas-scan/.ratchet/calls.jsonl
2                      # 原来是 1：那条 write_file 的 deny 被改成了 allow

# 下一趟采集
==> 上传事件流（控制面会与上次的摘要逐条比对）
   3 条事件 · 摘要 c3415e29b6a3…
   ❌ 检出断流：本地日志被改写：2 条与上次不一致（seq 1、seq 2…）
容器退出码 = 1（Job：Error）
```

```
$ curl -sS $API/silence
 链摘要：
   kind-demo/kind-demo: 到 seq 2，头 c3415e29b6a3…（2026-09-20T05:20:05Z）
 断流记录 1 条：
   #1 kind-demo/kind-demo：本地日志被改写：2 条与上次不一致（seq 1、seq 2…）
      上次到 seq 2（9901d5ace526…）→ 这次到 seq 2（c3415e29b6a3…）
      来自凭据 kind-demo-2026-10-20-1c384482，检出时间 2026-09-20T05:20:05Z
```

**同一趟的凭据自己仍然"通过"**：

```
$ curl -X POST $API/verify --data-binary @att5b.json
 ok = True · silenceIsAuditable = pass
```

这不是 bug，正是 §6.3 说的那件事：**流本身自洽，改写只有跨次比对才看得见**。
把两句话分开写，是为了不让读者以为"凭据通过"等于"日志没被动过"。

### 3.5 全部九条规则，第一次全部评出来

传感器跑完一趟之后，控制面重新验证同一份凭据：

```
$ curl -X POST $API/verify --data-binary @att5.json
 ok = True · 未评估：（无）
   ✅ hashMatch            ✅ narrowingOnly        ✅ delegationNarrows
   ✅ containmentCascades  ✅ noUndeclaredUnknown  ✅ everyVerdictHasBasis
   ✅ allPinnedOrExempt    ✅ silenceIsAuditable   ✅ withinValidityWindow
```

这是从 P1 开始一直挂在文档里的那条"未评估"消失的一刻：**九条规则，一条不剩，全部有输入**。

## 4. P5 判据逐条对照

| 判据 | 结果 | 证据 |
|---|---|---|
| 哈希口径可被独立实现 | ✅ | spec §6.2 + 跨语言向量（Go/Python 各有一个用例） |
| 删行、改行、砍尾巴都能被指出 | ✅ | `test_silence.py` 3 条 + `test_server.py` 3 条，含真机 §3.4 |
| 跨交付改写记为一个事件 | ✅ | `silence_breaks` 表 + `GET /silence`；传感器检出即非 0 退出 |
| 规则不再停留"未评估" | ✅ | §3.5：9/9 已评估 |
| 本地 k8s 部署验证 | ✅ | kind `ns=asas`：0.1.0-p5；传感器 Job 走完 采集→造链→出证→上报→传证据→传事件→复验 |
| 测试全绿 | ✅ | Python 146 项 + Go 11 包（`make test`） |

## 5. 已知限制（不粉饰）

1. **日志轮转会产生合法断流**：归档后重新开链，序号从 0 再来一次，当前会**记为一个事件**。
   规范里还没有"轮转声明"这个字段——所以这类断裂被记录但**不判失败**。
   这是明确的缺口，不是我们没注意。
2. **事件流是全量上传**：`events.jsonl` 是完整历史链，忙的机器上会越来越大。
   分段上传 + 锚点续接（带 `startSeq` / `anchorHash`）还没做。
3. **断流目前不影响合规判定**：`/verify` 只跑九条规则，断流记录在 `/silence`。
   要让"改写过的日志"直接影响凭据结论，需要先把第 1 条的轮转语义定下来，
   否则会把正常运维动作判成违规。
4. **hook 没接就没有事件流**：这时规则如实报"未评估"，而不是假装通过。
   本机 kind 的夹具是我为验收放进节点的，不是真实机器上的持续采集。

## 6. 下一轮

| # | 事项 | 判据 |
|---|---|---|
| T26 | 日志轮转声明（`rotation` 语义）→ 让断流能真正影响合规判定 | 合法轮转不被误判；改写仍然失败 |
| T27 | 分段上传 + 锚点续接 | 只传增量，控制面仍能比对前缀 |
| T28 | 把 v0.3 的口径送去外部评审（评审稿同步加两条） | 至少 1 位不相关的人能照着实现出同样的哈希 |
