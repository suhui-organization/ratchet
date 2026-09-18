# Content pack: "I scanned my own machine"

> 面向英文社区。纪律：只讲数字与边界，不讲产品、不加形容词。
> 数字来自 2026-09-18 本机实测。这一版**不带链接**——它是一篇发现，不是广告。

---

## X / Twitter thread

1/

I scanned my own machine for MCP servers today.

16 of them.

12 are launched through a package manager.

All 12 are unpinned. `npx -y some-mcp` — no version.

---

2/

So every one of those servers re-resolves to "latest" the next time it starts.

Whoever controls that npm package controls what runs on my laptop, with my permissions, the next time my agent calls it.

I would not notice.

---

3/

Three of the twelve are worse than unpinned — they're `@latest`:

`chrome-devtools-mcp@latest`
`@upstash/context7-mcp@latest`
`@playwright/mcp@latest`

`@latest` *looks* pinned. It reads like a version. It resolves to nothing.

---

4/

Also from the same static scan:

- 5 of the 16 servers carry env vars (that's where the tokens live)
- 2 are remote endpoints
- 4 are local binaries, not packages

Nothing was executed to gather any of this. It read config files.

---

5/

I wrote a small tool to print this because I wanted the number for myself.

The useful part isn't the scan — it's what comes after: which of these tools the agent actually *used*, and a privilege list you can hand to someone else.

Not public yet. Reply if you want the scan output for your own machine.

---

## Show HN

**Title**（选一个，越像事实越好）:

- `Show HN: I scanned my machine – 12 of 12 npx-launched MCP servers are unpinned`
- `Show HN: Static scan of my MCP configs – every package-managed server is unpinned`

**Body:**

I run 16 MCP servers on my laptop. I wanted to know which of them could change
under me, so I wrote a scanner that reads my agent configs and prints the answer.
It doesn't execute anything — it parses config files.

What it found:

- 12 of the 16 are launched via a package manager (`npx -y ...`)
- all 12 are pinned to nothing; three are explicitly `@latest`
- 5 carry env vars, 2 are remote endpoints, 4 are local binaries

The `@latest` cases bother me most. They read like a version. They're not one —
they re-resolve on every start, so a compromised or transferred package gets
picked up silently. The plain `npx -y foo` cases are the same thing with less
ceremony.

Two things I want to be clear about, because I think they matter more than the
finding:

1. This is a *static* scan. It only sees servers in configs it knows how to
   parse. On my machine that's one harness (Codex, TOML). A Zed config showed up
   and I deliberately did not parse it — I'd rather report "found but not
   parsed" than guess and give myself a false picture of my own exposure.
2. Pinning versions is not a security fix. It's a precondition for *noticing*.
   With `@latest` you can't tell whether a behavior change came from your config
   or from someone else's release.

The tool isn't public yet — I'm still working out what the output should be
beyond the scan. Happy to compare numbers if you count your own.

---

## Reddit — r/mcp

**Title:** `Static scan of my 16 MCP servers: all 12 package-managed ones are unpinned`

Did this today with a small scanner I wrote, and the result surprised me enough
to post.

16 servers, 1 harness (Codex). Of the 12 launched through `npx`/`uvx`, **zero**
have a version pin. Three are literally `@latest`.

I think we've collectively gotten casual about this because `npx -y` is the
default form in every README. But it means the package can change under you, and
your config looks identical before and after.

Three things I noticed while writing it:

- **`@latest` is the sneaky one.** It looks pinned. Plain `npx -y pkg` at least
  *looks* unpinned, so you might notice it.
- **env vars are where the secrets are.** 5 of my 16 carry them. If a server is
  swapped, it inherits them — that's the blast radius, not the tool list.
- **"Not parsed" is a real category.** One of my configs is a format the scanner
  doesn't understand. Reporting that as "no servers found" would have been a lie
  about my own machine.

Not launching anything, not selling anything. Curious whether other people's
numbers look similar — if you count your own `mcpServers` entries, what fraction
have a version?

**First comment（发出后立刻自己补）:**

For anyone who wants to check by hand: `grep -o '"command": *"[^"]*"' ~/.claude.json`
gets you half way. The thing to look for is whether any argument after the
package name contains an exact `x.y.z`.

---

## 怎么复现（自己核对，别信我的数字）

```bash
~/Workspaces/ratchet/bin/ratchet scan --home ~
```

没有这个二进制也能手查：

```bash
grep -c '"command"' ~/.claude.json                     # 你有几个 server
grep -o 'npx[^]]*' ~/.claude.json | grep -c '@[0-9]'   # 有几个锁了版本
```

---

## 发布纪律（贴之前再读一遍）

- 正文不放链接；链接放第一条回复。
- 不写"安全"这个词，只写数字和机制。
- 先说自己做不到什么（静态扫描的边界），再说发现。
- 有人回复才发第二条；0 回复的帖不要删，吃搜索长尾。
- 被问"这是不是你做的产品"→ 直接承认，并说清它现在还不公开。
