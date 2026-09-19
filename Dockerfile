# Ratchet 容器镜像 —— 供 MCP 客户端与官方 Registry 收录使用。
#
# 默认入口就是 `ratchet mcp`：一条 `docker run -i --rm <image>` 就是一个 stdio MCP server。
# 这正是官方 Registry 要求"存在公开可取用的制品"时想要的那种形态。

# ── 构建 ────────────────────────────────────────────────────────────────────
FROM golang:1.27-alpine AS build
WORKDIR /src
# 先拷清单再拷源码：依赖没变时，这一层命中缓存
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 静态链接 + 去符号：目标镜像里不需要任何运行时
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/ratchet ./cmd/ratchet

# ── 运行 ────────────────────────────────────────────────────────────────────
FROM alpine:3.20
# 不以 root 运行：容器里跑的是一个会读配置文件的工具，没有理由给它 root
RUN adduser -D -u 10001 ratchet
COPY --from=build /out/ratchet /usr/local/bin/ratchet

# MCP Registry 要求的标注：没有它，publish 会被拒
# （错误原文：OCI image is missing required annotation. Add this to your Dockerfile:
#   LABEL io.modelcontextprotocol.server.name="…"）
LABEL io.modelcontextprotocol.server.name="io.github.iversonwuwei/ratchet"
LABEL org.opencontainers.image.source="https://github.com/suhui-organization/ratchet"
LABEL org.opencontainers.image.description="Read-only tools to inspect your agent setup: list MCP servers, flag unpinned ones, test a policy."
LABEL org.opencontainers.image.licenses="Apache-2.0"

USER ratchet
WORKDIR /home/ratchet
ENV HOME=/home/ratchet

# 默认跑 MCP server（stdio）。要跑别的子命令，覆盖 CMD 即可：
#   docker run --rm <image> scan --home /host
ENTRYPOINT ["ratchet"]
CMD ["mcp"]
