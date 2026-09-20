# Content pack: "16 servers became 164 tools. 15 of them I would never approve."

> 面向英文社区。纪律：只讲数字与边界，不讲产品、不加形容词。
> 数字来自 2026-09-19 本机实测（Codex harness）。canonical 指向 `https://podcloud.dlszjr.com`。

---

## X / Twitter thread

1/

I connected to the MCP servers on my machine and asked them what they can do.

164 tools, from 12 of the 16 servers. (The other 4 refused to talk — one exited, three timed out.)

164 tools that my agent can call with my permissions.

---

2/

Per server, the distribution is not what I expected:

chrome-devtools 29
firecrawl 25
playwright 25
kubernetes 23
gitee 20
filesystem 14
docker 10
memory 9

The Kubernetes and Docker servers are what make this real. A browser automation tool is one thing; `kubectl` verbs are another.

---

3/

So I compiled a policy out of that inventory. Every tool gets one of three verdicts, and each verdict carries its reason:

allow 43 · approve 106 · deny 15

24 more came back as "can't judge the capability from the name or description" — those stay on approve and get flagged for a human.

---

4/

The 15 denials are the obvious ones, which is exactly why they're useful to look at:

kubectl_delete
uninstall_helm_chart
docker_remove
delete_entities / delete_observations / delete_relations
firecrawl_monitor_delete
node_repl js / js_reset
playwright browser_drop

---

5/

And now the part that makes this worth posting: the classifier is a keyword matcher, so it is wrong in both directions.

It denied `resolve-library-id` because its description matched the word "format". It denied `sequentialthinking` because the description said "clear".

That's why every verdict prints its reason instead of a score.

---

## Show HN

**Show HN: What 164 MCP tools look like when you write them into a policy**

I enumerated the MCP servers on my machine (12 of 16 answered), got 164 tools back, and compiled them into a three-state policy: allow 43, approve 106, deny 15, with 24 flagged as "capability can't be inferred — check by hand".

The interesting output isn't the split. It's that each verdict carries the literal reason it was made, so the wrong ones are auditable. Two examples of wrong: `resolve-library-id` was denied because the description contained "format"; `sequentialthinking` was denied because it contained "clear". A keyword matcher with a hidden score would have been worse than useless here — I'd have had no way to find those two.

What this does not do: it isn't a sandbox, it doesn't block calls, and it does not know what a tool *does* — only what its name and description claim. If a harmless-looking tool shell-executes, this won't catch it.

Two numbers from your own machine would be a useful comparison: how many servers answer, and how many tools come back.

---

## Reddit — r/mcp

Title: **12 of my 16 servers answered. 164 tools, 15 denied by name — and 2 of those denials were wrong.**

Enumerated my MCP setup and compiled the resulting inventory into a least-privilege style policy, mostly to see whether tool-name heuristics hold up.

Numbers: 164 tools from 12 servers. Verdicts: allow 43, approve 106, deny 15, plus 24 "can't tell" that get flagged rather than decided.

The deny list is unsurprising (`kubectl_delete`, `uninstall_helm_chart`, `docker_remove`, three `delete_*` in the memory server, `node_repl js/js_reset`, `playwright browser_drop`). The false positives are the useful part: `resolve-library-id` got denied for matching "format"; `sequentialthinking` got denied for matching "clear".

I kept both in because a policy that hides its reasons can't be corrected. Every verdict in the output prints the phrase it matched.

Caveat I'd repeat to anyone doing this: name/description matching tells you nothing about implementation. It ranks what to review; it doesn't review it.

Curious whether other people's servers answer the enumeration — I lost 4 of 16 (1 exits on initialize, 3 remote timeouts), and I'd rather know if that's typical.

---

## 怎么复现（自己核对，别信我的数字）

```bash
# 1) 列工具：会真的启动 server（本地 server 走 stdio，远程的暂不支持）
ratchet scan --home ~ --introspect --allow-exec --out inventory.json

# 2) 编译策略：三态 + 每条判定写出依据
ratchet policy draft --from inventory.json --out policy.json
```

`policy.json` 里 `rationale` 是逐条依据，`needsReview` 是"判不出来、请人看"的那一档。

## 发布纪律（贴之前再读一遍）

1. 主动说清它**不是**什么（不是沙箱、不拦调用、只看名字与描述）——这一条是整篇可信度的来源。
2. 误判那两条必须留着。删掉它们，这篇就从"实测"变成"宣传"。
3. 不放安装命令；canonical 指自己站点。
4. 有人问"这跟 Snyk 有什么区别"：扫描告诉你暴露面，这个告诉你**该允许什么**——从实际调用里编译。
