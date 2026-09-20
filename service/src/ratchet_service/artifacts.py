"""制品哈希（ASAS-3.2）。

**为什么必须有这一步**：定版状态只回答"下次启动会不会变"，制品哈希回答"此刻它是什么"。
只有后者，下一次才能对比——**变化要能看出来，否则"供应链控制"是句空话**。

来源是 npm registry 的公开元数据：解析包引用 → 取 tarball → 算 sha256。

纪律：任何失败（离线、包不存在、超时）一律返回 ``None``，由调用方写进 ``unknown[]``。
**绝不"拿不到就当没问题"**——那正是这份标准要防的东西。
"""

from __future__ import annotations

import hashlib
import json
import urllib.error
import urllib.parse
import urllib.request

# 这些"版本"等于没锁（与 internal/discover 的 pinned 判定保持一致）
_FLOATING = {"", "latest", "next", "beta", "canary", "alpha", "dev"}


def parse_npm_ref(ref: str) -> tuple[str, str | None]:
    """``@scope/pkg@1.2.3`` / ``pkg@latest`` / ``pkg`` → (包名, 版本或 None)。

    scoped 包名以 @ 开头，所以版本分隔符必须取**最后一个** @，且不能是首字符。
    """
    ref = (ref or "").strip()
    if not ref:
        return "", None
    idx = ref.rfind("@")
    if idx <= 0:
        return ref, None
    name, version = ref[:idx], ref[idx + 1:]
    if version.lower() in _FLOATING:
        return name, None
    return name, version


def _get(url: str, timeout: int, *, binary: bool):
    try:
        with urllib.request.urlopen(url, timeout=timeout) as resp:  # noqa: S310（固定 https）
            raw = resp.read()
    except (urllib.error.URLError, TimeoutError, OSError):
        return None
    if binary:
        return raw
    try:
        return json.loads(raw.decode())
    except ValueError:
        return None


def hash_npm_package(ref: str, timeout: int = 20) -> tuple[str, str] | None:
    """返回 ``(解析到的版本, "sha256:<hex>")``；任何一步失败都返回 None。

    哈希对象是 **tarball 本身**，不是解包后的内容树：前者是"这个制品"，
    而且 registry 侧任何人都能独立复算。
    """
    name, version = parse_npm_ref(ref)
    if not name:
        return None
    base = "https://registry.npmjs.org/" + urllib.parse.quote(name, safe="@")
    if version:
        meta = _get(f"{base}/{version}", timeout, binary=False)
        if not isinstance(meta, dict):
            return None
        resolved = str(meta.get("version") or version)
        tarball = str((meta.get("dist") or {}).get("tarball") or "")
    else:
        # 不带版本查询拿到的是**完整 packument**：顶层没有 dist.tarball，
        # 要从 dist-tags.latest 找到具体版本，再进 versions[<ver>].dist.tarball。
        # （实测踩过：按顶层读会永远返回 None，看起来像"离线"，其实是解析错了。）
        packument = _get(base, timeout, binary=False)
        if not isinstance(packument, dict):
            return None
        resolved = str((packument.get("dist-tags") or {}).get("latest") or "")
        entry = (packument.get("versions") or {}).get(resolved) or {}
        tarball = str((entry.get("dist") or {}).get("tarball") or "")
    if not tarball:
        return None
    data = _get(tarball, timeout, binary=True)
    if not data:
        return None
    return resolved, "sha256:" + hashlib.sha256(data).hexdigest()
