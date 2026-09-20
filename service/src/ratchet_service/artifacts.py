"""制品哈希（ASAS-3.2）。

**为什么必须有这一步**：定版状态只回答"下次启动会不会变"，制品哈希回答"此刻它是什么"。
只有后者，下一次才能对比——**变化要能看出来，否则"供应链控制"是句空话**。

来源是 npm registry 的公开元数据：解析包引用 → 取 tarball → 算 sha256。

纪律：任何失败（离线、包不存在、超时）一律返回 ``None``，由调用方写进 ``unknown[]``。
**绝不"拿不到就当没问题"**——那正是这份标准要防的东西。

**出网必须有界。** 传感器跑在没有人在旁边的机器上（CI、客户内网）。本机实测：
kind 里的采集卡在 ``SYN_SENT``，每个请求各等 20 秒，一趟走完要好几分钟——而结论
还是 unknown。所以这里有两道闸：

* ``RATCHET_OFFLINE=1``：一步都不出网。**不是降级**——unknown 在规范里是一等公民，
  而且"传感器偷偷往 registry 发请求"本身就是客户内网里的一个噪声源。
* ``RATCHET_HTTP_BUDGET``（默认 60 秒）：整趟的总出网预算，用完就全部记 unknown，
  不会再让下一个包再等一轮。剩下的时间用来把凭据交出去。
"""

from __future__ import annotations

import hashlib
import json
import os
import time
import urllib.error
import urllib.parse
import urllib.request

# 这些"版本"等于没锁（与 internal/discover 的 pinned 判定保持一致）
_FLOATING = {"", "latest", "next", "beta", "canary", "alpha", "dev"}

# 单个请求的超时上限。
DEFAULT_TIMEOUT = 20
# 整趟出网的总预算（秒）。
DEFAULT_BUDGET = 60

_TRUE = {"1", "true", "yes", "on"}

# 预算的起点：第一次真的要用网时才开始计时，凭据生成之前的耗时不算在里面。
_budget_started: float | None = None
# 上一次为什么没取：None=试过了，'offline'/'budget'=压根没试。
_skip: str | None = None


def offline() -> bool:
    return os.environ.get("RATCHET_OFFLINE", "").strip().lower() in _TRUE


def _timeout(default: int = DEFAULT_TIMEOUT) -> int:
    try:
        value = int(os.environ.get("RATCHET_HTTP_TIMEOUT", "").strip() or default)
    except ValueError:
        value = default
    return value if value > 0 else default


def _budget(default: int = DEFAULT_BUDGET) -> float:
    try:
        value = float(os.environ.get("RATCHET_HTTP_BUDGET", "").strip() or default)
    except ValueError:
        value = default
    return value if value > 0 else float(default)


def reset_budget() -> None:
    """重新开始计时。一次交付跑一趟，跑第二趟不该继承上一趟的余量。"""
    global _budget_started, _skip
    _budget_started, _skip = None, None


def skip_reason() -> str | None:
    """上一次哈希调用是被哪道闸挡下的（``None`` = 真的试过了）。

    调用方要拿它写进 ``unknown[].why``：**"没试"和"试了没成"是两种事实**，
    凭据里不能都写成一句"取不到"。
    """
    return _skip


def _gated() -> bool:
    """是否应当放弃出网。挡下时顺手记下原因。"""
    global _budget_started, _skip
    if offline():
        _skip = "offline"
        return True
    if _budget_started is None:
        _budget_started = time.monotonic()
    if time.monotonic() - _budget_started > _budget():
        _skip = "budget"
        return True
    _skip = None
    return False


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


def hash_npm_package(ref: str, timeout: int | None = None) -> tuple[str, str] | None:
    """返回 ``(解析到的版本, "sha256:<hex>")``；任何一步失败都返回 None。

    哈希对象是 **tarball 本身**，不是解包后的内容树：前者是"这个制品"，
    而且 registry 侧任何人都能独立复算。

    出网前先过两道闸（离线 / 预算，见模块头）。挡下时**不重试、不等待**，
    立刻返回 None 并把原因留在 :func:`skip_reason` 里。
    """
    if _gated():
        return None
    timeout = timeout if timeout is not None else _timeout()
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
