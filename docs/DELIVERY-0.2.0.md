# 交付记录 0.2.0 —— 真实扫描

> 日期：2026-09-18。上一版见 [DELIVERY-0.1.0.md](DELIVERY-0.1.0.md)。

## 1. 这一版加了什么

清单不再靠手写：`ratchet scan` 真的去读这台机器上的配置。

| # | 产物 | 位置 | 状态 |
|---|---|---|---|
| 1 | 配置发现（8 类 harness / 10 个探测点，JSON + TOML） | `internal/discover` | ✅ 测试通过 |
| 2 | 最小 MCP 客户端（只做 initialize + tools/list） | `internal/mcp` | ✅ 测试通过 |
| 3 | `ratchet scan [--introspect]` 命令 | `cmd/ratchet/main.go` | ✅ |

## 2. 两条设计纪律

**① 默认不执行任何东西。** `scan` 只读配置；`--introspect` 会连上 server 取工具名，
而那会**执行配置里写的命令**，所以必须显式开启。配置里可能有别人的 server，
不该因为"跑了一次扫描"就被启动。

**② 环境变量只记键名，不记值。** MCP 配置的 `env` 值通常就是密钥。
测试里专门放了一个 `ghp_...` 做诱饵，断言它不会出现在任何字段里。

另外两条与大原则一致的小决定：

- 用 `--out` 写清单时必须同时给 `--introspect`——静态扫描只知道 server、
  不知道工具名，**报错比给一份空清单诚实**；
- 连不上的 server 记进 `introspect_failures`，并明确写出"它们不在清单里，
  能力面是未知的，不要当成没有风险"。

## 3. 真机结果（本机 /home/walden）

```console
$ ratchet scan --home /home/walden
  harness   1（已解析 1）
  server    16
  未锁版本  12（同名包被替换时无法察觉）

  codex        Codex            16 个 server
      chrome-devtools    npx -y chrome-devtools-mcp@latest
      mcp-server-amap    npx -y @amap/amap-maps-mcp-server ⚠ 依赖未锁版本
      pod-codex          remote http://127.0.0.1:8788/mcp
```

16 个 server 里有 **12 个没锁版本**。这是这个工具第一次在真实环境里说出
一句用户自己不知道的话——也就是市场分析里"先让用户拿到一个具体数字"的落地。

## 4. 测试与真实数据各抓到一个问题

| 来源 | 问题 | 修法 |
|---|---|---|
| 测试 | 探测到但**不解析**的 harness 仍标记 `parsed=true` | 标为 false 并列出原因——否则报告会让人以为"这个 harness 没有 server" |
| 真机数据 | `@latest` 被判成"已锁版本" | `@latest` / `^1.2.0` / `~1.x` 一律算未锁，并加了专门用例 |

真机数据把统计从 8 修正到 12：**`@latest` 比不写版本号更危险，因为它看起来像被钉住了**。

## 5. 测试结果

```console
$ go test ./...
ok  github.com/suhui-organization/ratchet/internal/discover
ok  github.com/suhui-organization/ratchet/internal/mcp
ok  github.com/suhui-organization/ratchet/internal/policy
```

MCP 客户端用两个 Python 夹具做靶子：一个正常应答、并**故意往 stdout 打一行非 JSON**
（验证客户端会跳过噪音），一个永远不回话（验证超时后进程被杀、
且**返回错误而不是空列表**——"连不上"与"没有工具"是两件事）。

## 6. 没做完的

| # | 事项 | 说明 |
|---|---|---|
| L1 | 更多 harness 方言（Zed 等） | 现在只探测存在、不解析，并如实报告 |
| L2 | 远程 server（`url`）的 introspect | 目前只支持 stdio，远程会记为连接失败 |
| L3 | 用审计日志里的真实调用次数填 `calls` | 现在 `calls=0`，依据里写"未观测到调用" |
| L4 | 报告渲染 / 后端 API / 30 天复验 | 同 0.1.0，未动 |

## 7. 环境备注

Go 依赖需要走代理（本机直连 proxy.golang.org 被拒）：

```bash
export GOPROXY=https://goproxy.cn,direct
```
