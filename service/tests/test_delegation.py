"""V8：跨凭据的委派收窄（ASAS-2.4/2.5）。

为什么值得单独一组用例：V2 只能看**同一份凭据内部**的委派。真实世界里父子常是两个
主体、两份凭据，这时只看子凭据能证明的只是"它自己说它收窄了"。V8 把父凭据当作
独立输入，而"拿不到父凭据"这个分支（= 未知委派链 = 越权）是最容易被实现者放松的一条。
"""

from datetime import datetime, timedelta, timezone

from ratchet_service import asas

NOW = datetime(2026, 9, 20, 12, 0, tzinfo=timezone.utc)
LATER = (NOW + timedelta(days=60)).isoformat()
SOON = (NOW + timedelta(days=10)).isoformat()
PAST = (NOW - timedelta(days=1)).isoformat()
FUTURE = (NOW + timedelta(days=30)).isoformat()


def parent_attestation(*, grants=("filesystem", "postgres"), denies=("secrets",), to=FUTURE) -> dict:
    return {
        "asas": "0.1",
        "subject": {"org": "acme", "period": {"from": PAST, "to": to}},
        "agents": [{
            "id": "parent-agent", "owner": "alice",
            "runtime": {"kind": "codex", "version": "1"},
            "identity": {"type": "service-account", "shared": False},
        }],
        "assets": [],
        "verdicts": [
            *[{"agent": "parent-agent", "asset": a, "decision": "allow", "basis": "matched keyword"}
              for a in grants],
            *[{"agent": "parent-agent", "asset": a, "decision": "deny", "basis": "sensitive path"}
              for a in denies],
        ],
        "unknown": [],
        "observations": [],
        "boundaries": {"mode": "readonly", "revocationMinutes": 5},
        "manifest": {"evidence": [], "generatedAt": NOW.isoformat(), "generator": "test"},
    }


def child_attestation(*, from_agent="parent-agent", scope=("filesystem",), expires=SOON,
                      denies=("secrets",), to=FUTURE) -> dict:
    return {
        "asas": "0.1",
        "subject": {"org": "acme", "period": {"from": PAST, "to": to}},
        "agents": [{
            "id": "child-agent", "owner": "bob",
            "runtime": {"kind": "codex", "version": "1"},
            "identity": {"type": "service-account", "shared": False},
            "delegation": {"from": from_agent, "scope": list(scope), "expires": expires},
        }],
        "assets": [],
        "verdicts": [
            # 子凭据里的 deny 必须把父的 deny 带上（可见性，见 asas.py 的注释）
            *[{"agent": "child-agent", "asset": a, "decision": "deny", "basis": "inherited deny"}
              for a in denies],
            *[{"agent": "child-agent", "asset": a, "decision": "allow", "basis": "delegated"}
              for a in scope],
        ],
        "unknown": [],
        "observations": [],
        "boundaries": {"mode": "readonly", "revocationMinutes": 5},
        "manifest": {"evidence": [], "generatedAt": NOW.isoformat(), "generator": "test"},
    }


def rule(att, *, parents=None):
    report = asas.verify(att, parents=parents, containment=[], now=NOW)
    return {r.rule: r for r in report.results}["delegationNarrows"]


def test_no_delegation_is_a_pass_not_an_unknown():
    """没有声明委派 = 读过输入、里面没有委派。这和"没有输入"不是一回事。"""
    att = child_attestation()
    del att["agents"][0]["delegation"]
    assert rule(att).status == asas.PASS


def test_unknown_chain_is_an_escalation_not_an_omission():
    """拿不到父凭据 = ASAS-2.5 的未知委派链 = 越权。这一条不能放成"未评估"。"""
    result = rule(child_attestation(), parents=None)
    assert result.status == asas.FAIL
    assert "未知委派链" in result.details[0]


def test_narrowing_child_passes():
    assert rule(child_attestation(), parents={"parent-agent": parent_attestation()}).status == asas.PASS


def test_widening_is_rejected_with_the_asset_named():
    result = rule(
        child_attestation(scope=("filesystem", "secrets")),
        parents={"parent-agent": parent_attestation()},
    )
    assert result.status == asas.FAIL
    assert "secrets" in result.details[0]
    assert "委派放大" in result.details[0]


def test_child_may_not_outlive_parent():
    result = rule(child_attestation(expires=LATER), parents={"parent-agent": parent_attestation()})
    assert result.status == asas.FAIL
    assert "不得比父活得久" in result.details[0]


def test_expired_parent_takes_the_child_with_it():
    result = rule(
        child_attestation(expires=SOON),
        parents={"parent-agent": parent_attestation(to=PAST)},
    )
    assert result.status == asas.FAIL
    assert "已过期" in result.details[0]


def test_dropping_a_parent_denial_is_rejected():
    """丢掉父的一条拒绝 = 这条限制在子的凭据里彻底不可见。"""
    result = rule(
        child_attestation(denies=()),
        parents={"parent-agent": parent_attestation()},
    )
    assert result.status == asas.FAIL
    assert "deny 只增不减" in result.details[0]
    assert "secrets" in result.details[0]


def test_parent_inside_the_same_document_is_left_to_v2():
    """同一份凭据里的委派归 V2 管——两条规则不许对同一件事各判一次。"""
    att = child_attestation(from_agent="child-agent")
    assert rule(att, parents=None).status == asas.PASS  # V8 跳过
    results = {r.rule: r for r in asas.verify(att, containment=[], now=NOW).results}
    # V2 会看到"父=自己"这个退化情形，并给出它的判断
    assert results["narrowingOnly"].status in (asas.PASS, asas.FAIL)


# ── 默认值不该生产出失败 ──────────────────────────────────────────────────────
def test_default_expiry_never_outlives_the_parent():
    """父子两份凭据相隔几秒生成时，默认委派到期必须被父的有效期压住。

    实测踩过：只取自己的有效期，子就"比父多活 3 秒"，一份默认生成的凭据直接违规。
    """
    from ratchet_service import asas_build

    parent = parent_attestation(to=FUTURE)
    child_period_to = datetime.fromisoformat(FUTURE)
    delegation = asas_build._delegation_block(
        {"from": "parent-agent", "parentNotAfter": FUTURE},
        [{"agent": "child-agent", "asset": "filesystem", "decision": "allow", "basis": "delegated"}],
        "child-agent",
        child_period_to + timedelta(seconds=5),  # 子凭据晚几秒生成
    )
    assert datetime.fromisoformat(delegation["delegation"]["expires"]) <= child_period_to
    assert delegation["delegation"]["expires"] == FUTURE


def test_explicit_expiry_is_respected():
    from ratchet_service import asas_build

    block = asas_build._delegation_block(
        {"from": "parent-agent", "expires": SOON, "parentNotAfter": FUTURE},
        [{"agent": "c", "asset": "filesystem", "decision": "allow", "basis": "x"}],
        "c",
        datetime.fromisoformat(FUTURE),
    )
    assert block["delegation"]["expires"] == SOON
