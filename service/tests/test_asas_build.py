"""构建器：把 policy.json 翻译成 ASAS-A，并当场自查。"""

import json

from ratchet_service import asas, asas_build
from datetime import datetime, timezone

POLICY = {
    "version": "ratchet-policy/v1",
    "agent": "codex",
    "defaultDecision": "deny",
    "generatedAt": "2026-09-20T00:00:00Z",
    "generator": "ratchet 0.12.2",
    "servers": {
        "filesystem": {"allow": [], "approve": ["read_file"], "deny": []},
        "kubernetes": {"allow": [], "approve": [], "deny": ["kubectl_delete"]},
    },
    "rationale": {
        "filesystem/read_file": "no capability keyword; never observed",
        "kubernetes/kubectl_delete": 'name matches "delete"',
    },
    "needsReview": [],
}


def test_build_produces_conformant_shape():
    att = asas_build.build_attestation(POLICY, org="acme")
    assert att["asas"] == "0.1"
    assert {a["name"] for a in att["assets"]} == {"filesystem", "kubernetes"}
    assert all(v["basis"] for v in att["verdicts"]), "每条判定都必须带依据"


def test_build_declares_what_it_cannot_know():
    """拿不到的事实：产物哈希、定版状态、工具面——必须全部进 unknown[]，不许猜。"""
    att = asas_build.build_attestation(POLICY, org="acme")
    declared = " ".join(u["what"] for u in att["unknown"])
    assert "filesystem: contentHash" in declared
    assert "filesystem: pinned" in declared
    assert "filesystem: version" in declared
    assert "filesystem: reachableTools" in declared  # 没给 inventory
    assert all(a["contentHash"] is None and a["pinned"] is None for a in att["assets"])


def test_build_with_inventory_marks_tools_known():
    inventory = {"tools": [{"server": "filesystem", "tool": "read_file"}]}
    att = asas_build.build_attestation(POLICY, org="acme", inventory=inventory)
    fs = next(a for a in att["assets"] if a["name"] == "filesystem")
    assert fs["reachableTools"] == 1
    declared = " ".join(u["what"] for u in att["unknown"])
    assert "filesystem: reachableTools" not in declared


def test_server_facts_turn_unknown_into_known():
    """扫描侧给了事实，凭据就必须用上——否则等于浪费信息量。"""
    facts = [
        {"name": "filesystem", "ref": "filesystem-mcp@1.2.3", "pinned": True},
        {"name": "kubernetes", "ref": "mcp-server-kubernetes", "pinned": False},
    ]
    att = asas_build.build_attestation(POLICY, org="acme", inventory=facts)
    by_name = {a["name"]: a for a in att["assets"]}
    assert by_name["filesystem"]["pinned"] is True
    assert by_name["filesystem"]["version"] == "filesystem-mcp@1.2.3"
    assert by_name["kubernetes"]["pinned"] is False
    declared = " ".join(u["what"] for u in att["unknown"])
    assert "filesystem: pinned" not in declared and "filesystem: version" not in declared
    # 内容哈希仍然未知：本机扫描拿不到制品本体，这一步不能假装
    assert by_name["filesystem"]["contentHash"] is None
    assert "filesystem: contentHash" in declared


def test_written_exemption_makes_unpinned_compliant():
    """ASAS-3.1：未定版 + 书面豁免 + 到期日 = 合规；豁免本身留在凭据里供收货方查看。"""
    facts = [
        {"name": "kubernetes", "ref": "mcp-server-kubernetes", "pinned": False},
        {"name": "filesystem", "ref": "filesystem-mcp@1.2.3", "pinned": True},
    ]
    att = asas_build.build_attestation(
        POLICY, org="acme", inventory=facts,
        exemptions={"kubernetes": "2026-12-31T00:00:00Z"},
    )
    kubernetes = next(a for a in att["assets"] if a["name"] == "kubernetes")
    assert kubernetes["exemptUntil"] == "2026-12-31T00:00:00Z"
    report = asas.verify(att, now=datetime(2026, 9, 20, tzinfo=timezone.utc))
    pinned_rule = next(r for r in report.results if r.rule == "allPinnedOrExempt")
    assert pinned_rule.status == asas.PASS, pinned_rule.details


def test_expired_exemption_is_not_compliant():
    facts = [{"name": "kubernetes", "ref": "mcp-server-kubernetes", "pinned": False}]
    att = asas_build.build_attestation(
        POLICY, org="acme", inventory=facts,
        exemptions={"kubernetes": "2026-09-01T00:00:00Z"},  # 已经过期
    )
    report = asas.verify(att, now=datetime(2026, 9, 20, tzinfo=timezone.utc))
    pinned_rule = next(r for r in report.results if r.rule == "allPinnedOrExempt")
    assert pinned_rule.status == asas.FAIL, "过期的豁免必须失效，否则'豁免'就成了永久赦免"


def test_self_check_passes_on_generated_attestation():
    att, report = asas_build.build_and_verify(POLICY, org="acme")
    assert report.ok is True, [r.as_dict() for r in report.results]
    # 交付物里没有证据文件输入 → hashMatch 未评估；这是诚实的三态，不是通过
    assert "hashMatch" in report.not_evaluated


def test_generated_attestation_validates_against_schema():
    jsonschema = __import__("jsonschema")
    from pathlib import Path

    schema = json.loads(Path(__file__).resolve().parents[2].joinpath("spec/ASAS-A-v0.1.schema.json").read_text())
    att = asas_build.build_attestation(POLICY, org="acme")
    jsonschema.validate(att, schema)
