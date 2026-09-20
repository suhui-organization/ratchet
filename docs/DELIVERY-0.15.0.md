# 交付记录 0.15.0 —— P0：凭据闭环

> 日期：2026-09-20。上一版见 [DELIVERY-0.14.0.md](DELIVERY-0.14.0.md)。
> 本轮按 [DEVELOPMENT-PLAN.md](DEVELOPMENT-PLAN.md) 的 P0 切片执行：**交付产物对齐 ASAS-A，并自查**。

## 1. 完成的任务

| # | 任务 | 状态 | 证据 |
|---|---|---|---|
| T1 | `asas_build` 从 policy 生成 ASAS-A | ✅ | `service/src/ratchet_service/asas_build.py`；66 项测试 |
| T2 | `deliver` 调用构建器并自查，不过不算交付 | ✅ | 见下 §2 的真机输出 |
| T3 | 扫描侧补齐 `pinned`/`version`/制品哈希 | ⏳ | 见 §4 |
| T4 | 本轮落档 | ✅ | 就是这份文件 |

## 2. 验收输出（真机粘贴）

```console
$ /tmp/ratchet-t2 deliver --out /tmp/t2-demo --report-cmd "python3 -m ratchet_service.cli"
  ✅ scan      1 harnesses, 16 servers, 12 unpinned
  ⚠️ observe   no call records yet — run your agents with the hook attached
  ✅ policy    16 tools: 0 allow, 16 approve, 0 deny
  ✅ report    /tmp/t2-demo/delivery
  ✅ attest    /tmp/t2-demo/delivery/attestation.json（已通过 ASAS-V 自查）
```

凭据自查的六条规则（三态，不把"没检查"说成"通过"）：

```console
$ python3 -m ratchet_service.cli asas --policy …/policy.json --dir …/delivery --org demo-org
  已评估规则 5 条 · 未评估 1 条
  ✅ hashMatch            ✅ narrowingOnly        ✅ noUndeclaredUnknown
  ✅ everyVerdictHasBasis ✅ allPinnedOrExempt    — silenceIsAuditable（无事件流）
  unknown 声明 64 条（拿不到的事实，不填假值）
```

## 3. 这一轮真正的设计产出

写参考实现时发现**规范初稿有一处会逼人撒谎**：schema 原先把 `contentHash` / `version` /
`pinned` / `reachableTools` 定为必填，而本机扫描**本来就拿不到制品哈希**——一份"必须填"的
schema 只会生产出一批看起来干净、实际是假值的凭据。

已修正为 `值 | null`，并形成通用规则（写进规范）：

> **未知可表达三元组**：任何事实字段都允许 `null`；允许 `null` 就必须在 `unknown[]` 声明。
> **不能表达的未知，一定会变成假值。**

这不是放松要求，而是收紧：初稿可以用假值蒙过去，现在验证器的 V3 会直接判不合规。

## 4. 未完成（下一轮）

| # | 事项 | 归属 |
|---|---|---|
| T3 | 扫描侧补齐 `pinned` / `version` / 制品哈希，让 `unknown` 从 64 条降下来 | P0 收尾 |
| T5–T9 | 控制面三个 Deployment 部署到**本地 k8s**（`ns=asas`），`POST /verify` 与 Python 验证器逐字一致 | **P1** |
