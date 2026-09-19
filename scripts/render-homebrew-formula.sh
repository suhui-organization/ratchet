#!/usr/bin/env bash
# ============================================================================
# 生成 Homebrew formula（钉住每个平台 tarball 的 sha256）。
#
# 为什么必须动态生成：sha256 只有发布之后才算得出来，而手抄四个平台的
# 哈希是必然出错的活（抄错的表现是"brew install 下载校验失败"）。
#
#   bash scripts/render-homebrew-formula.sh                 # 用当前版本
#   bash scripts/render-homebrew-formula.sh --version 0.12.0 --out Formula/ratchet.rb
# ============================================================================
set -uo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_DIR"

VERSION="$(sed -n 's/^const version = "\(.*\)"/\1/p' cmd/ratchet/main.go)"
OUT="dist/ratchet.rb"
while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    *) echo "未知参数：$1" >&2; exit 2 ;;
  esac
done

BASE="https://github.com/suhui-organization/ratchet/releases/download/v${VERSION}"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

echo "版本 $VERSION"
echo "从 $BASE 取四个平台并算 sha256…"
declare -A SHA
for p in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  asset="ratchet_${VERSION}_${p}.tar.gz"
  if ! curl -fsSL --retry 3 --max-time 180 "$BASE/$asset" -o "$TMP/$asset"; then
    echo "  ❌ 取不到 $asset —— 这个版本还没发布？" >&2
    exit 1
  fi
  SHA[$p]="$(sha256sum "$TMP/$asset" | awk '{print $1}')"
  echo "  ✅ $p  ${SHA[$p]:0:16}…"
done

mkdir -p "$(dirname "$OUT")"
cat > "$OUT" <<RUBY
# 由 scripts/render-homebrew-formula.sh 生成 —— 不要手改。
#
# 重新生成（发新版之后）：
#   bash scripts/render-homebrew-formula.sh --out /path/to/homebrew-tap/Formula/ratchet.rb
class Ratchet < Formula
  desc "Compile least-privilege policy from what your AI agents actually reached"
  homepage "https://github.com/suhui-organization/ratchet"
  version "${VERSION}"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "${BASE}/ratchet_${VERSION}_darwin_arm64.tar.gz"
      sha256 "${SHA[darwin_arm64]}"
    else
      url "${BASE}/ratchet_${VERSION}_darwin_amd64.tar.gz"
      sha256 "${SHA[darwin_amd64]}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "${BASE}/ratchet_${VERSION}_linux_arm64.tar.gz"
      sha256 "${SHA[linux_arm64]}"
    else
      url "${BASE}/ratchet_${VERSION}_linux_amd64.tar.gz"
      sha256 "${SHA[linux_amd64]}"
    end
  end

  def install
    bin.install "ratchet"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/ratchet version")
  end
end
RUBY

echo
echo "已写出 $OUT"
echo "装它：brew install suhui-organization/tap/ratchet"
