#!/usr/bin/env bash
# 把 release 产物打成一个 MCPB 包（MCP Bundle，原 DXT），用于 Smithery 的 stdio 发布。
#
# 为什么需要：Smithery 现在只收三种 release——hosted(JS)、external(公网 URL)、stdio(MCPB 包)。
# ratchet 是本地 stdio server，所以只能走第三种；而 MCPB 要求"把 server 和 manifest 打成一个 zip"。
#
# 规范依据（照抄自 modelcontextprotocol/mcpb 的 MANIFEST.md，别凭记忆改）：
#   manifest_version "0.3" · name · version · description · author{name} · server{...} 为必需字段
#   server.type = "binary"，entry_point 相对包根，mcp_config 用 ${__dirname} 定位
#   多平台差异写在 mcp_config.platform_overrides 里；Windows 上宿主会自动补 .exe
#
# 用法：
#   bash scripts/build-mcpb.sh            # 用当前版本号，从 GitHub Release 拉产物
#   VERSION=0.12.0 bash scripts/build-mcpb.sh
#   KEEP_STAGE=1 ...                      # 保留中间目录便于排查
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-$(sed -n 's/^const version = "\(.*\)"/\1/p' cmd/ratchet/main.go)}"
TAG="v${VERSION}"
RELEASE_BASE="${RELEASE_BASE:-https://github.com/suhui-organization/ratchet/releases/download/${TAG}}"
STAGE="${STAGE:-dist/mcpb-stage}"
OUT="${OUT:-dist/ratchet-${VERSION}.mcpb}"

# 只放我们真的构建并测过的平台：Windows 还没有产物，就不写进 manifest 假装支持。
PLATFORMS="darwin_arm64 darwin_amd64 linux_amd64 linux_arm64"

rm -rf "$STAGE"; mkdir -p "$STAGE/server"

echo "==> 拉取 ${TAG} 的产物"
for p in $PLATFORMS; do
  asset="ratchet_${VERSION}_${p}.tar.gz"
  echo "    $asset"
  curl -fsSL -o "$STAGE/$asset" "$RELEASE_BASE/$asset"
  tar -xzf "$STAGE/$asset" -C "$STAGE"
  mv "$STAGE/ratchet" "$STAGE/server/ratchet-${p}"
  chmod 0755 "$STAGE/server/ratchet-${p}"
  rm -f "$STAGE/$asset"
done

# 宿主按 name 直接执行；这里放一个薄启动器，按 uname 选对应架构的二进制。
# 为什么不用 platform_overrides 直接指到具体架构：那个字段只按平台（darwin/win32/linux）区分，
# 分不出 arm64 与 x64，所以用一个 shim 来选。
cat > "$STAGE/server/ratchet" <<'SHIM'
#!/bin/sh
# MCPB 入口：按当前机器选 ratchet-<os>_<arch>。
set -e
dir="$(cd "$(dirname "$0")" && pwd)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
esac
case "$os" in
  darwin) os=darwin ;;
  linux) os=linux ;;
  *) echo "ratchet: unsupported platform $os" >&2; exit 1 ;;
esac
exec "$dir/ratchet-${os}_${arch}" "$@"
SHIM
chmod 0755 "$STAGE/server/ratchet"

cat > "$STAGE/manifest.json" <<JSON
{
  "manifest_version": "0.3",
  "name": "ratchet",
  "version": "${VERSION}",
  "description": "Read-only inventory of the MCP servers an agent can reach - which are unpinned, which tools they expose - then a least-privilege policy with the reason behind every verdict.",
  "author": {
    "name": "Walden Wu",
    "url": "https://github.com/suhui-organization/ratchet"
  },
  "server": {
    "type": "binary",
    "entry_point": "server/ratchet",
    "mcp_config": {
      "command": "\${__dirname}/server/ratchet",
      "args": ["mcp"]
    }
  },
  "compatibility": {
    "platforms": ["darwin", "linux"]
  },
  "license": "Apache-2.0",
  "repository": {
    "type": "git",
    "url": "https://github.com/suhui-organization/ratchet"
  }
}
JSON

if command -v npx >/dev/null 2>&1; then
  # 注意：`mcpb validate` 吃的是**目录**，不是打好的 .mcpb；喂 zip 进去它会当成 manifest.json
  # 去解析，报 "Unexpected token 'P', \"PK...\"" —— 那是命令用错，不是包坏了（实测踩过）。
  echo "==> 用官方工具校验 manifest（针对目录）"
  npx -y @anthropic-ai/mcpb validate "$STAGE" || echo "    (校验未通过，见上面的输出)"
fi

echo "==> 打包 $OUT"
mkdir -p "$(dirname "$OUT")"
(cd "$STAGE" && zip -qr "$ROOT/$OUT" manifest.json server)
ls -lh "$OUT" | awk '{print "    " $9, $5}'

echo "==> 自检：启动器能在本机拉起正确架构的二进制"
_t="$(mktemp -d)"; unzip -q "$OUT" -d "$_t"
"$_t/server/ratchet" --version
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1"}}}' \
  | timeout 8 "$_t/server/ratchet" mcp | head -c 200; echo

if [[ "${KEEP_STAGE:-0}" != "1" ]]; then rm -rf "$STAGE"; fi

cat <<EOF

完成。下一步（发布到 Smithery）：
  curl -X PUT "https://api.smithery.ai/servers/iverson-wuwei/ratchet/releases" \\
    -H "Authorization: Bearer \$SMITHERY_API_KEY" \\
    -F 'payload={"type":"stdio"};type=application/json' \\
    -F 'bundle=@${OUT};type=application/octet-stream'
（payload 的字段以 docs/api-reference/servers/publish-a-server.md 的 DeployPayload 为准，
  发之前先 GET 一次该文档确认字段名，别照抄上面的示例。）
EOF
