# Ratchet

> **从 agent 的真实行为编译最小权限策略，并给出收货方能自己验证的证据。**

```bash
curl -fsSL https://github.com/suhui-organization/ratchet/releases/latest/download/install.sh | sh
```

macOS / Linux 上也可以用 Homebrew（formula 钉住每个平台的 sha256）：

```bash
brew install suhui-organization/tap/ratchet
```

**Claude Desktop 用户**可以直接装本地 server（一键安装包，不需要命令行）：

下载 [ratchet-0.12.0.mcpb](https://github.com/suhui-organization/ratchet/releases/latest/download/ratchet-0.12.0.mcpb)
并打开它，Claude Desktop 会完成安装。包内是四个平台的原生二进制 + 一份 `manifest.json`
（MCPB 规范），**不需要 Node 或 Python 运行时**。

Linux / macOS（amd64 · arm64）。装完先跑这一条——**只读配置，不执行任何东西**：

```bash
ratchet scan --home ~
```

它会在十秒内告诉你：这台机器上有几个 MCP server，其中几个**没锁版本**。
（我自己的机器：16 个里 12 个没锁。）

棘轮只能往一个方向转：权限可以收紧，放宽必须显式。

## 这是什么

一个跑在**你自己机器上**的工具：它读 agent 用到的工具清单，
编译出一份最小权限策略，再把这次审计的产物打成一份
**收货方不需要信任你、也不需要装任何东西就能验证**的交付目录。

## 这不是什么

- 不是沙箱、不是容器运行时；
- 不是 prompt 内容过滤器；
- 不是企业控制台（不做 dashboard、不做多租户、不做 SaaS 依赖）；
- 不要求你把日志传到哪里去。

## 为什么值得存在

市场上的产品要么**让你自己手写策略**（Cedar / CEL / 自然语言），
要么**要你相信它的日志**。Ratchet 做的是这两件之间缺失的一步：
**策略由观测编译，证据由收货方自己算哈希。**

## 一个例子

```bash
# 1) 编译：从工具清单得到一份保守的最小权限策略
ratchet policy draft --from examples/inventory.json --out policy.json

# 2) 出交付物：报告 + 逐文件 sha256 清单
ratchet-report build --dir ./delivery --policy policy.json

# 3) 收货方验证（不需要装 ratchet）
python3 verify.py ./delivery
```

## 仓库结构

| 目录 | 语言 | 是什么 |
|---|---|---|
| `cmd/` `internal/` | **Go** | 机器上的引擎：工具清单 → 能力判定 → 最小权限策略。编译成单个静态二进制，目标机器不需要任何运行时 |
| `service/` | **Python** | 交付物侧：合规映射、sha256 清单、以及**只用标准库**的独立验证器 |
| `web/` | **Nuxt 4 + Vue 3 + Tailwind** | 公开页面：落地页 + 收货方在浏览器里自己算哈希的验证页 |
| `docs/` | — | 设计与计划（`DESIGN.md` / `PLAN.md` / `DECISIONS.md`） |

## 状态

**0.12.0 — developer preview，链路已经整条打通**（本机实测：16 个 MCP server、
12 个未锁版本、164 个工具、策略 allow 43 / approve 106 / deny 15）：

| 命令 | 做什么 |
|---|---|
| `ratchet scan [--introspect --allow-exec] [--share page.html]` | 读 harness 配置列出 MCP server（只读，不执行）；加 `--introspect --allow-exec` 才连上去列工具（连上就会执行配置里的命令，所以要显式开） |
| `ratchet ingest` / `observe` | 采集真实调用（hook 出错静默退出），并对照清单看出哪些工具从没被用过 |
| `ratchet policy draft` / `check` | 编译三态策略（每条带判定依据），并试跑一次假设调用 |
| `ratchet deliver` / `report build` | 一条命令出交付目录：策略 + 报告 + sha256 清单 |
| `ratchet mcp` | 以 stdio MCP server 形态跑（`ratchet_scan` / `ratchet_policy` / `ratchet_check`） |
| `ratchet reach --api <控制面> --subject <资产>` | 问台账"谁曾能触达 X"（ASAS-8.3）：授权面与观测面分开列，每条带依据；**台账没覆盖就直说"答不了"** |
| `ratchet contain --api <控制面> --agent <id> [--yes]` | 吊销一个 agent，默认**级联到它的下级**（ASAS-8.5）；**默认只预告不动手**，看清单再加 `--yes` |
| `ratchet chain --calls ~/.ratchet/calls.jsonl --out events.jsonl` | 把调用记录变成**带哈希链的事件流**（ASAS-5.3/6.6）：删行、改行、砍尾巴都会露馅。`deliver` 会自动做这一步并写进交付目录 |
| `ratchet feedback` | 把去标识化结果整理成 issue 正文供你复制；**没有遥测，不会自动发** |

拿到它的方式：`curl -fsSL https://github.com/suhui-organization/ratchet/releases/latest/download/install.sh | sh`、
`brew install suhui-organization/tap/ratchet`、容器 `ghcr.io/suhui-organization/ratchet`，
或官方 MCP Registry 里的 `io.github.iversonwuwei/ratchet`。

**清楚的边界**：能力判定是名称/描述的启发式，会**双向出错**（仓里有两条已知误判的测试）；
它不是沙箱、运行时不拦调用；判定不了的会单独列出来请人确认，而不是猜一个结论。
详见 [docs/PLAN.md](docs/PLAN.md) 与 [docs/DELIVERY-0.14.0.md](docs/DELIVERY-0.14.0.md)。

## 许可证

Apache-2.0，见 [LICENSE](LICENSE)。

## 开发

```bash
make test          # 三端全部测试
make go-test
make py-test
make web-test
```
