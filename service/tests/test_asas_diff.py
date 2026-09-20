"""ASAS-3.4 差异比对：每条规则都对应一个真实攻击面。"""

from ratchet_service import asas_diff


def att(**overrides) -> dict:
    base = {
        "assets": [{
            "name": "kubernetes", "version": "1.0.0", "pinned": True,
            "contentHash": "sha256:" + "a" * 64, "descHash": "sha256:" + "b" * 64,
            "reachableTools": 23,
        }],
        "verdicts": [{"agent": "codex", "asset": "kubernetes", "decision": "deny", "basis": "x"}],
        "unknown": [],
    }
    base.update(overrides)
    return base


def kinds(changes) -> set[str]:
    return {c["kind"] for c in changes}


def test_no_change_is_empty():
    assert asas_diff.diff(att(), att()) == []


def test_content_hash_change_is_critical():
    new = att(assets=[dict(att()["assets"][0], contentHash="sha256:" + "c" * 64)])
    changes = asas_diff.diff(att(), new)
    assert changes[0]["kind"] == "content_changed"
    assert changes[0]["severity"] == asas_diff.CRITICAL


def test_description_change_is_high():
    new = att(assets=[dict(att()["assets"][0], descHash="sha256:" + "d" * 64)])
    change = next(c for c in asas_diff.diff(att(), new) if c["kind"] == "description_changed")
    assert change["severity"] == asas_diff.HIGH


def test_pin_removed_is_high():
    new = att(assets=[dict(att()["assets"][0], pinned=False)])
    change = next(c for c in asas_diff.diff(att(), new) if c["kind"] == "pin_removed")
    assert change["severity"] == asas_diff.HIGH


def test_verdict_widening_is_high_and_tightening_is_info():
    widened = asas_diff.diff(att(), att(verdicts=[{"agent": "codex", "asset": "kubernetes", "decision": "allow", "basis": "x"}]))
    assert next(c for c in widened if c["kind"] == "verdict_changed")["severity"] == asas_diff.HIGH
    tightened = asas_diff.diff(att(verdicts=[{"agent": "codex", "asset": "kubernetes", "decision": "approve", "basis": "x"}]), att())
    assert next(c for c in tightened if c["kind"] == "verdict_changed")["severity"] == asas_diff.INFO


def test_new_asset_and_new_unknown_are_medium():
    new = att(assets=att()["assets"] + [{"name": "filesystem", "pinned": False}],
              unknown=[{"what": "filesystem: contentHash", "why": "x", "discoveredAt": "x"}])
    changes = asas_diff.diff(att(), new)
    assert next(c for c in changes if c["kind"] == "asset_added")["severity"] == asas_diff.MEDIUM
    assert next(c for c in changes if c["kind"] == "unknown_added")["severity"] == asas_diff.MEDIUM


def test_summary_flags_critical():
    new = att(assets=[dict(att()["assets"][0], contentHash="sha256:" + "c" * 64)])
    assert asas_diff.summarize(asas_diff.diff(att(), new))["hasCritical"] is True
