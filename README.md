# Ratchet

> **从 agent 的真实行为编译最小权限策略，并给出收货方能自己验证的证据。**

```bash
curl -fsSL https://github.com/suhui-organization/ratchet/releases/latest/download/install.sh | sh
```

macOS / Linux 上也可以用 Homebrew（formula 钉住每个平台的 sha256）：

```bash
brew install suhui-organization/tap/ratchet
```

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

**0.1.0 — 走的通的最小骨架**：策略编译与验证链路已实现且有测试，
真实环境扫描（读 harness 配置、MCP introspection）尚未实现，输入是清单文件。
详见 [docs/PLAN.md](docs/PLAN.md)。

## 开发

```bash
make test          # 三端全部测试
make go-test
make py-test
make web-test
```
