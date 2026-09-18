# 交付记录 0.5.0 —— 一条命令出交付包

> 日期：2026-09-18。上一版见 [DELIVERY-0.4.0.md](DELIVERY-0.4.0.md)。

## 1. 这一版补的是断点

之前 Go 出策略、Python 出清单，中间靠手工拼。现在 `ratchet-report build`
吃一份策略，直接产出完整的交付目录。

| # | 产物 | 位置 | 状态 |
|---|---|---|---|
| 1 | 报告渲染（四节：摘要/权限表/判定依据/覆盖边界 + 合规映射） | `service/src/ratchet_service/report.py` | ✅ 7 个测试 |
| 2 | `ratchet-report build --policy <策略.json>` | `service/.../cli.py` | ✅ |

## 2. 报告为什么是固定四节

它要同时满足两类读者，缺任何一节都会变成"看起来权威、其实什么也没说"：

| 节 | 给谁 | 缺了会怎样 |
|---|---|---|
| 1 摘要 | 客户/老板 | 看不出结论 |
| 2 权限表 | 客户 | 不知道收了什么 |
| 3 判定依据 | 审计方 | 无法判断这份报告可不可信 |
| 4 覆盖边界 | 审计方 | 被读成安全结论 |

第 4 节里有一条必须存在的话：**「未观测」不等于「没发生」**。
没有它，读者会把"没发现调用"当成"这个工具没用过"——那是两个完全不同的判断。

第 3 节会把"依据不足、需人工确认"的工具单列出来，并在摘要里提醒
**先核对这一批再用**。一份自动生成的策略如果不告诉你哪里它没把握，就不该被直接用。

## 3. 端到端（本机实测）

```console
$ for t in read_file read_file write_file shell; do
    printf '{"tool_name":"%s","tool_input":{"x":"ghp_NOPE"}}' "$t" | ratchet ingest
  done
$ ratchet observe --calls calls.jsonl --inventory inv.json --out used.json
$ ratchet policy draft --from used.json --only-observed --out policy.json

$ ratchet-report build --dir delivery --policy policy.json --note "端到端演示"
已生成交付目录：delivery
  产物 2 份 · 生成时间 2026-09-18T15:42:31+00:00
    policy.json  593 字节
    report.md  3339 字节

$ env -i PATH=/usr/bin:/bin HOME=/tmp python3 verify.py delivery
✅ Verification passed: 2 file(s) match manifest.json
```

**六步全部跑通，最后一行的执行环境里没有 Go、没有我们的包、没有环境变量。**
这就是"收货方不需要信任你"的字面实现。

产出目录：

```
delivery/
├── report.md        # 四节 + 合规映射
├── policy.json      # 可执行的策略
└── manifest.json    # 逐文件 sha256（不含自己，避免自引用）
```

## 4. 测试

```console
$ python3 -m pytest -q   →  21 passed
$ go test ./...          →  6 个包全绿
$ npm run test           →   8 passed
```

报告层的用例逐节断言：四节齐全、计数与策略一致、待确认被单列、
依据被带出、覆盖边界里必须有"不等于"、合规映射被内嵌，
以及**交付目录建完之后** `manifest.verify` 必须为真。

## 5. 现在还缺什么

| # | 事项 | 影响 |
|---|---|---|
| L1 | 其他 harness 的采集 | 只有 Codex；Claude Code / Cursor 还没接 |
| L2 | 分享包与网页验证（`bundle` 已实现，页面已实现，未串起来） | 收货方目前要收目录 + 一个 py 文件 |
| L3 | 30 天复验 | 续费钩子，未做 |
| L4 | 报告的中文之外语言 | 目前只有中文（合规映射是中文；面向海外要英文化） |

## 6. 建议的下一步

**L2 优先级最高**：把 `bundle` 与验证页串起来，交付形态就从
"一个目录 + 一个脚本"变成"一个 JSON + 一个链接"。
这直接决定了第一次交付时对方的体验——也是这件事能不能被
当作服务卖出去的门槛。
