"""ASAS-V 一致性测试向量（对应 spec/ASAS-v0.1.md §7 的 20 条）。"""

import hashlib
import json
from datetime import datetime, timedelta, timezone

import pytest

from ratchet_service import asas

NOW = datetime(2026, 9, 20, 12, 0, tzinfo=timezone.utc)
FUTURE = (NOW + timedelta(days=30)).isoformat()
PAST = (NOW - timedelta(days=1)).isoformat()


def sha(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()


def base_attestation() -> dict:
    """一份最小的合规凭据：一个 agent、两个资产、两条判定、无未知。"""
    return {
        "asas": "0.1",
        "subject": {"org": "acme", "period": {"from": PAST, "to": FUTURE}},
        "agents": [{"id": "a1", "owner": "alice", "runtime": {"kind": "codex", "version": "1"}, "identity": {"type": "service-account", "shared": False}}],
        "assets": [
            {"kind": "server", "name": "filesystem", "version": "1.2.3", "pinned": True, "contentHash": "sha256:" + "a" * 64, "descHash": "sha256:" + "b" * 64, "reachableTools": 14},
            {"kind": "tool", "name": "read_file", "version": "1.2.3", "pinned": True, "contentHash": "sha256:" + "c" * 64},
        ],
        "verdicts": [
            {"agent": "a1", "asset": "filesystem", "decision": "approve", "basis": 'name matched "filesystem"'},
            {"agent": "a1", "asset": "read_file", "decision": "allow", "basis": "observed 12 times"},
        ],
        "unknown": [],
        "observations": [{"tool": "read_file", "count": 12, "argKeys": ["path"]}],
        "boundaries": {"mode": "readonly", "revocationMinutes": 5},
        "manifest": {"evidence": [{"file": "policy.json", "sha256": sha("policy")}], "generatedAt": NOW.isoformat(), "generator": "ratchet 0.12.2"},
    }


def run(att: dict, *, files=None, events=None):
    return {r.rule: r for r in asas.verify(att, files=files, events=events, now=NOW).results}


# ── 基线：干净凭据六条规则全过（V1/V6 因为没有输入而"未评估"）──────────────
def test_clean_attestation_passes_evaluated_rules():
    results = run(base_attestation())
    for rule in ("narrowingOnly", "noUndeclaredUnknown", "everyVerdictHasBasis", "allPinnedOrExempt"):
        assert results[rule].status == asas.PASS, (rule, results[rule].details)


# ── V1 哈希一致（4 条）────────────────────────────────────────────────────────
def test_v1_pass():
    att = base_attestation()
    assert run(att, files={"policy.json": b"policy"})["hashMatch"].status == asas.PASS


def test_v1_content_tampered():
    att = base_attestation()
    assert run(att, files={"policy.json": b"policy-tampered"})["hashMatch"].status == asas.FAIL


def test_v1_missing_file():
    assert run(base_attestation(), files={})["hashMatch"].status == asas.FAIL


def test_v1_not_evaluated_without_files():
    assert run(base_attestation())["hashMatch"].status == asas.NOT_EVALUATED


# ── V2 委派只可收窄（4 条）────────────────────────────────────────────────────
def _with_child(scope, *, source="a1", expires=FUTURE):
    att = base_attestation()
    att["agents"].append({
        "id": "a2", "owner": "alice", "runtime": {"kind": "codex", "version": "1"},
        "identity": {"type": "service-account", "shared": False},
        "delegation": {"from": source, "scope": scope, "expires": expires},
    })
    return att


def test_v2_pass_subset():
    att = _with_child(["read_file"])
    assert run(att)["narrowingOnly"].status == asas.PASS


def test_v2_widened_scope_fails():
    att = _with_child(["read_file", "kubernetes"])
    result = run(att)["narrowingOnly"]
    assert result.status == asas.FAIL and "委派放大" in result.details[0]


def test_v2_expired_delegation_fails():
    att = _with_child(["read_file"], expires=PAST)
    assert run(att)["narrowingOnly"].status == asas.FAIL


def test_v2_unknown_delegation_chain_fails():
    att = _with_child(["read_file"], source="ghost")
    result = run(att)["narrowingOnly"]
    assert result.status == asas.FAIL and "未知委派链" in result.details[0]


# ── V3 未知必须声明（3 条）────────────────────────────────────────────────────
def test_v3_pass_when_declared():
    att = base_attestation()
    att["assets"][0]["descHash"] = None
    att["unknown"] = [{"what": "filesystem", "why": "initialize 时退出", "discoveredAt": NOW.isoformat()}]
    assert run(att)["noUndeclaredUnknown"].status == asas.PASS


def test_v3_undeclared_enumeration_failure_fails():
    att = base_attestation()
    att["assets"][0]["reachableTools"] = None
    result = run(att)["noUndeclaredUnknown"]
    assert result.status == asas.FAIL and "未在 unknown" in result.details[0]


def test_v3_unknown_entry_without_why_fails():
    att = base_attestation()
    att["unknown"] = [{"what": "x", "why": "", "discoveredAt": NOW.isoformat()}]
    assert run(att)["noUndeclaredUnknown"].status == asas.FAIL


# ── V4 每条判定都有依据（3 条）────────────────────────────────────────────────
@pytest.mark.parametrize("basis", ["", "n/a", "  "])
def test_v4_missing_basis_fails(basis):
    att = base_attestation()
    att["verdicts"][0]["basis"] = basis
    assert run(att)["everyVerdictHasBasis"].status == asas.FAIL


def test_v4_all_verdicts_have_basis_passes():
    assert run(base_attestation())["everyVerdictHasBasis"].status == asas.PASS


# ── V5 定版或有未过期豁免（3 条）──────────────────────────────────────────────
def test_v5_unpinned_without_exemption_fails():
    att = base_attestation()
    att["assets"][0]["pinned"] = False
    assert run(att)["allPinnedOrExempt"].status == asas.FAIL


def test_v5_expired_exemption_fails():
    att = base_attestation()
    att["assets"][0]["pinned"] = False
    att["assets"][0]["exemptUntil"] = PAST
    assert run(att)["allPinnedOrExempt"].status == asas.FAIL


def test_v5_valid_exemption_passes():
    att = base_attestation()
    att["assets"][0]["pinned"] = False
    att["assets"][0]["exemptUntil"] = FUTURE
    assert run(att)["allPinnedOrExempt"].status == asas.PASS


# ── V6 沉默可被审计（3 条）────────────────────────────────────────────────────
def _events(count=3):
    out, prev = [], ""
    for seq in range(count):
        body = json.dumps({"seq": seq, "prevHash": prev, "tool": f"t{seq}"}, sort_keys=True, separators=(",", ":"))
        digest = sha(body)
        out.append({"seq": seq, "prevHash": prev, "tool": f"t{seq}", "hash": digest})
        prev = digest
    return out


def test_v6_intact_stream_passes():
    assert run(base_attestation(), events=_events())["silenceIsAuditable"].status == asas.PASS


def test_v6_sequence_gap_fails():
    events = _events()
    events[2]["seq"] = 5
    assert run(base_attestation(), events=events)["silenceIsAuditable"].status == asas.FAIL


def test_v6_prev_hash_mismatch_fails():
    events = _events()
    events[1]["prevHash"] = "deadbeef"
    assert run(base_attestation(), events=events)["silenceIsAuditable"].status == asas.FAIL


def test_v6_not_evaluated_without_events():
    assert run(base_attestation())["silenceIsAuditable"].status == asas.NOT_EVALUATED


# ── 报告层（三态与总体判定）────────────────────────────────────────────────────
def test_report_ok_means_no_failures():
    report = asas.verify(base_attestation(), now=NOW)
    assert report.ok is True
    assert "hashMatch" in report.not_evaluated  # 没给文件就不算通过


def test_report_lists_evaluated_rules():
    report = asas.verify(base_attestation(), files={"policy.json": b"policy"}, events=_events(), now=NOW)
    assert report.ok is True
    assert report.not_evaluated == []


# ── V3 新增分支：哈希与定版状态未知时必须声明 ────────────────────────────────
def test_v3_missing_content_hash_must_be_declared():
    att = base_attestation()
    att["assets"][0]["contentHash"] = None
    assert run(att)["noUndeclaredUnknown"].status == asas.FAIL
    att["unknown"] = [{"what": "filesystem", "why": "拿不到制品", "discoveredAt": NOW.isoformat()}]
    assert run(att)["noUndeclaredUnknown"].status == asas.PASS


def test_v3_unknown_pinned_state_must_be_declared():
    att = base_attestation()
    att["assets"][0]["pinned"] = None
    assert run(att)["noUndeclaredUnknown"].status == asas.FAIL
