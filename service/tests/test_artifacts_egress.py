"""出网的两道闸：离线开关与总预算。

为什么值得单独测：传感器跑在**没人看着的机器上**。本机 kind 实测，采集卡在
SYN_SENT 上，每个请求各等 20 秒，一趟下来好几分钟、结论还是 unknown——
这种"看起来在干活"的失败比直接报错更贵。

这两条闸的作用是让"取不到"变成**一个立刻给出的、写清楚原因的事实**。
"""

import pytest

from ratchet_service import artifacts


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch):
    for name in ("RATCHET_OFFLINE", "RATCHET_HTTP_BUDGET", "RATCHET_HTTP_TIMEOUT"):
        monkeypatch.delenv(name, raising=False)
    artifacts.reset_budget()
    yield
    artifacts.reset_budget()


def _forbid_network(monkeypatch):
    """任何真的出网都会让用例炸掉——这比断言"返回 None"更强。"""
    def boom(*a, **kw):  # pragma: no cover - 只在闸门失效时才走到
        raise AssertionError("闸门没挡住，代码真的去访问网络了")

    monkeypatch.setattr(artifacts.urllib.request, "urlopen", boom)


def test_offline_never_touches_the_network(monkeypatch):
    _forbid_network(monkeypatch)
    monkeypatch.setenv("RATCHET_OFFLINE", "1")
    assert artifacts.hash_npm_package("@modelcontextprotocol/sdk@1.0.0") is None
    assert artifacts.skip_reason() == "offline"


def test_budget_exhaustion_stops_trying_instead_of_waiting(monkeypatch):
    import time

    _forbid_network(monkeypatch)
    monkeypatch.setenv("RATCHET_HTTP_BUDGET", "1")
    # 把预算起点拨到一分钟前：这不是"第一次调用"，而是"预算早就用完了"。
    artifacts._budget_started = time.monotonic() - 60
    assert artifacts.hash_npm_package("left-pad@1.0.1") is None
    assert artifacts.skip_reason() == "budget"


def test_budget_reset_starts_a_fresh_run(monkeypatch):
    import time
    import urllib.error

    monkeypatch.setenv("RATCHET_HTTP_BUDGET", "1")
    artifacts._budget_started = time.monotonic() - 60
    assert artifacts.hash_npm_package("left-pad@1.0.0") is None
    assert artifacts.skip_reason() == "budget"

    def unreachable(*a, **kw):
        # 重置之后应当**真的去试**，而这次失败属于"试了没成"，不是"没试"。
        raise urllib.error.URLError("network is unreachable")

    monkeypatch.setattr(artifacts.urllib.request, "urlopen", unreachable)
    artifacts.reset_budget()
    assert artifacts.hash_npm_package("left-pad@1.0.0") is None
    assert artifacts.skip_reason() is None


def test_timeout_is_configurable_and_sane(monkeypatch):
    monkeypatch.setenv("RATCHET_HTTP_TIMEOUT", "3")
    assert artifacts._timeout() == 3
    monkeypatch.setenv("RATCHET_HTTP_TIMEOUT", "0")
    assert artifacts._timeout() == artifacts.DEFAULT_TIMEOUT
    monkeypatch.setenv("RATCHET_HTTP_TIMEOUT", "不是数字")
    assert artifacts._timeout() == artifacts.DEFAULT_TIMEOUT


def test_unknown_reason_tells_apart_not_tried_from_tried(monkeypatch):
    """凭据里的 why 必须区分"我们的选择"和"环境不给"，否则审计没得看。"""
    from ratchet_service import asas_build

    monkeypatch.setenv("RATCHET_OFFLINE", "1")
    artifacts.reset_budget()
    artifacts.hash_npm_package("left-pad@1.0.0")
    offline_reason = asas_build._hash_unknown_reason("left-pad@1.0.0", hasher_supplied=True)
    assert "RATCHET_OFFLINE" in offline_reason

    artifacts.reset_budget()  # skip_reason 归零 = 试过了但没成
    tried_reason = asas_build._hash_unknown_reason("left-pad@1.0.0", hasher_supplied=True)
    assert "离线或包不存在" in tried_reason
    assert offline_reason != tried_reason


def test_offline_flag_on_the_cli_reaches_the_gate(monkeypatch, tmp_path):
    """CLI 的 --offline 必须真的把闸门落下，而不是只印一行提示。"""
    import json

    from ratchet_service import cli

    _forbid_network(monkeypatch)
    (tmp_path / "policy.json").write_text(json.dumps({"servers": {"demo": {"deny": ["x"]}}}), encoding="utf-8")
    code = cli.main(["asas", "--policy", str(tmp_path / "policy.json"), "--dir", str(tmp_path),
                     "--org", "demo", "--offline"])
    assert code == 0
    att = json.loads((tmp_path / "attestation.json").read_text(encoding="utf-8"))
    assert any("RATCHET_OFFLINE" in u["why"] or "离线" in u["why"] for u in att["unknown"]) or att["assets"]
