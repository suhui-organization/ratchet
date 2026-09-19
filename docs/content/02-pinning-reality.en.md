# Content pack: "12 of 16 are unpinned — and 4 of them don't even say `@latest`"

> 面向英文社区。纪律：只讲数字与边界，不讲产品、不加形容词。
> 数字来自 2026-09-19 本机实测（Codex harness）。canonical 指向 `https://podcloud.dlszjr.com`。

---

## X / Twitter thread

1/

Last week I counted the MCP servers on my machine: 16, of which 12 are launched through a package manager.

All 12 are unpinned. Today I split them by *how* they're unpinned, because the split is the interesting part.

---

2/

4 of the 12 say `@latest` outright:

`chrome-devtools-mcp@latest`
`@upstash/context7-mcp@latest`
`@playwright/mcp@latest`
`superpowers-mcp@latest`

`@latest` at least reads like a version string. It resolves to nothing.

---

3/

The other 8 have no version string at all:

`npx -y firecrawl-mcp`
`npx -y gitee-mcp-server`
`npx -y mcp-server-kubernetes`
`npx -y @edjl/docker-mcp`

No `@`, no tag, nothing. And these are the ones people trust *more*, because they look like a name rather than a moving target.

---

4/

Both forms mean the same thing: the next time the agent starts that server, it re-resolves from the registry.

Whoever can publish that npm name decides what runs on my machine, with my permissions, on the next launch.

---

5/

One more number from the same scan: 4 of the 16 servers couldn't be enumerated at all — one exited during initialization, three are remote and timed out.

I have no idea what tools those four expose. "Unknown" is not "safe", and nothing in my setup was telling me the difference.

---

## Show HN

**Show HN: I counted how my machine's MCP servers resolve their versions**

I wrote a read-only scanner for my own setup. It parses the harness config files, executes nothing, and reports what the agent can actually reach.

Headline numbers on this machine: 16 MCP servers, 12 launched through a package manager, 12 unpinned — 4 with an explicit `@latest`, 8 with no version string at all. Four more servers couldn't be enumerated, so their tool surface is unknown to me.

This is not a CVE and nothing here is exotic; it's the default state of following an install instruction, which usually says `npx -y <name>`.

What I'm not claiming: an unpinned package is not the same as a compromised package. The point is narrower — you can't tell the difference after the fact, because the artifact you approved isn't the artifact that runs next time.

Run it on your own machine and post the two numbers (servers, unpinned). I'd like to know whether 12/16 is normal or whether my setup is unusually sloppy.

---

## Reddit — r/mcp

Title: **Counted my own setup: 16 MCP servers, 12 unpinned. Split by how they're unpinned.**

I went through my harness configs and counted. 16 MCP servers configured, 12 of them started via `npx`.

Of those 12:

- 4 are explicitly `pkg@latest` (`chrome-devtools-mcp@latest`, `@playwright/mcp@latest`, `@upstash/context7-mcp@latest`, `superpowers-mcp@latest`)
- 8 have no version specifier at all (`npx -y firecrawl-mcp`, `npx -y mcp-server-kubernetes`, `npx -y gitee-mcp-server`, `npx -y @edjl/docker-mcp`, …)

The second group surprised me more than the first. `@latest` is at least honest about being a moving reference; a bare package name reads like a stable dependency.

Separately: 4 of my 16 servers couldn't be enumerated (1 exits on initialize, 3 are remote and time out), so I genuinely don't know what they expose.

Not claiming this is a breach — claiming I can't tell an unpinned name from a swapped one after the fact.

What does your config look like? Two numbers: how many servers, how many pinned.

---

## 怎么复现（自己核对，别信我的数字）

```bash
# 只读：解析 config，不执行任何 server
ratchet scan --home ~
```

输出里就是那三行：`harness` / `server` / `未锁版本`。
要拿到工具数（会真的启动 server 去列工具，这一步是显式请求才发生的）：

```bash
ratchet scan --home ~ --introspect --out inventory.json
```

## 发布纪律（贴之前再读一遍）

1. 不写"这不安全"。写"我分不清"——前者是结论，后者是事实。
2. 不放安装命令。这一篇是发现，不是广告；链接只放 canonical 与复现命令。
3. 有人回"`npx -y foo` 本来就会缓存"——先承认，再指出缓存不等于锁定（`npx` 的缓存是按 semver 解析后的版本，不是按你写下的名字）。
4. 被问是不是你做的工具：**直接承认**。
