#!/bin/sh
# Ratchet 安装脚本 —— 一条命令装完。
#
#   curl -fsSL <你的地址>/install.sh | sh
#
# 它做什么：
#   1. 认出系统与架构（linux/darwin × amd64/arm64）
#   2. 下载对应二进制包，**校验 sha256，不一致就退出**（不装可疑的东西）
#   3. 解到 ~/.local/bin（可用 RATCHET_BIN_DIR 覆盖）
#
# 环境变量：
#   RATCHET_BASE_URL   产物所在地址（默认见下）
#   RATCHET_VERSION    装哪个版本（默认 0.6.0）
#   RATCHET_BIN_DIR    装到哪（默认 ~/.local/bin）
#   RATCHET_SHA256     钉住校验值。给了就用它，不从网上取——
#                      从同一个地址取哈希，只能防传输损坏，防不了地址被换。
set -eu

# 打包时由 `make release RELEASE_URL=...` 把这个占位符换成真实地址
# （用占位符而不是正则替换：地址里的 :// 与 $ 在 sed 里都要转义，容易写错）
BASE_URL="${RATCHET_BASE_URL:-__RATCHET_BASE_URL__}"
VERSION="${RATCHET_VERSION:-0.6.0}"
BIN_DIR="${RATCHET_BIN_DIR:-$HOME/.local/bin}"

say()  { printf '  %s\n' "$*"; }
die()  { printf 'ratchet install: %s\n' "$*" >&2; exit 1; }

# ── 1. 平台 ────────────────────────────────────────────────────────────────
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux|darwin) ;;
  *) die "暂不支持的系统：$os（目前只有 linux 与 darwin）" ;;
esac

machine=$(uname -m)
case "$machine" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "暂不支持的架构：$machine" ;;
esac

asset="ratchet_${VERSION}_${os}_${arch}.tar.gz"
say "平台 $os/$arch，准备安装 ratchet $VERSION"

# ── 2. 下载 ────────────────────────────────────────────────────────────────
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

fetch() {
  if command -v curl >/dev/null 2>&1; then curl -fsSL --max-time 120 "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then wget -q -T 120 -O "$2" "$1"
  else die "需要 curl 或 wget"
  fi
}

say "下载 $BASE_URL/$asset"
fetch "$BASE_URL/$asset" "$tmp/$asset" || die "下载失败：$BASE_URL/$asset（确认这个地址能访问）"

# ── 3. 校验（fail-closed）──────────────────────────────────────────────────
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
else
  die "找不到 sha256sum 或 shasum——没有校验工具就不装"
fi

if [ -n "${RATCHET_SHA256:-}" ]; then
  expected="$RATCHET_SHA256"
  source_desc="你提供的 RATCHET_SHA256"
else
  fetch "$BASE_URL/$asset.sha256" "$tmp/$asset.sha256" \
    || die "取不到 $asset.sha256 —— 没有校验值就不装"
  expected=$(awk '{print $1}' "$tmp/$asset.sha256")
  source_desc="$BASE_URL/$asset.sha256"
fi

[ -n "$expected" ] || die "校验值为空，拒绝安装"
if [ "$actual" != "$expected" ]; then
  printf '  期望 %s\n  实际 %s\n' "$expected" "$actual" >&2
  die "sha256 不一致：这个包不是发布时的那一个。已中止，什么都没装。"
fi
say "sha256 校验通过（来源：$source_desc）"

# ── 4. 安装 ────────────────────────────────────────────────────────────────
tar -xzf "$tmp/$asset" -C "$tmp" || die "解包失败"
[ -f "$tmp/ratchet" ] || die "包里没有 ratchet 可执行文件"

mkdir -p "$BIN_DIR"
install -m 0755 "$tmp/ratchet" "$BIN_DIR/ratchet" 2>/dev/null \
  || { cp "$tmp/ratchet" "$BIN_DIR/ratchet" && chmod 0755 "$BIN_DIR/ratchet"; }
say "已安装到 $BIN_DIR/ratchet"

# ── 5. 收尾 ────────────────────────────────────────────────────────────────
"$BIN_DIR/ratchet" version || true

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo
    say "$BIN_DIR 不在 PATH 里。把它加进去："
    say "  echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.profile && . ~/.profile"
    ;;
esac

echo
say "下一步："
say "  ratchet scan --home ~          # 只读扫描，不执行任何东西"
