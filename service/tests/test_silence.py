"""V6 可验证的沉默：用**仓库里的同一份向量**验链（ASAS-5.3 / 6.6）。

向量是 Go 造的（`internal/chain`），这里是 Python 读的。两边共用一份文件，
是因为"哈希口径漂移"这种错在单语言里永远测不出来：各自都自洽，合起来就是断链。

断流必检的三种：
  1. 序号断裂（少了中间几条）——删日志最省事的做法；
  2. 前序哈希不符（改了内容但没重算链）——最常见的手改；
  3. 自身哈希不符（重算了被改的那条，但没管后面的链脚）——稍微懂一点的做法。
"""

import json
from datetime import datetime, timedelta, timezone
from pathlib import Path

from ratchet_service import asas

VECTOR = Path(__file__).resolve().parents[2] / "spec/testvectors/silence/events.jsonl"
NOW = datetime(2026, 9, 20, 12, 0, tzinfo=timezone.utc)
PAST = (NOW - timedelta(days=1)).isoformat()
FUTURE = (NOW + timedelta(days=30)).isoformat()


def vector_events() -> list[dict]:
    return [json.loads(line) for line in VECTOR.read_text(encoding="utf-8").splitlines() if line.strip()]


def clean_attestation() -> dict:
    return {
        "asas": "0.1",
        "subject": {"org": "acme", "period": {"from": PAST, "to": FUTURE}},
        "agents": [{"id": "a1", "owner": "alice", "runtime": {"kind": "codex", "version": "1"},
                    "identity": {"type": "service-account", "shared": False}}],
        "assets": [],
        "verdicts": [{"agent": "a1", "asset": "filesystem", "decision": "allow", "basis": "observed"}],
        "unknown": [],
        "observations": [],
        "boundaries": {"mode": "readonly", "revocationMinutes": 5},
        "manifest": {"evidence": [], "generatedAt": NOW.isoformat(), "generator": "test"},
    }


def verdict(events) -> asas.RuleResult:
    report = asas.verify(clean_attestation(), events=events, containment=[], now=NOW)
    return {r.rule: r for r in report.results}["silenceIsAuditable"]


def test_the_committed_vector_passes_the_python_verifier():
    """Go 造的链，Python 必须逐条认——这就是"两边同一套口径"的可执行定义。"""
    events = vector_events()
    assert len(events) == 3
    assert events[0]["seq"] == 0 and events[0]["prevHash"] == ""
    assert verdict(events).status == asas.PASS


def test_a_full_run_with_events_has_nothing_unevaluated():
    report = asas.verify(clean_attestation(), events=vector_events(), containment=[], now=NOW)
    assert report.ok is True
    assert report.not_evaluated == ["hashMatch"], "证据文件之外不该还有未评估的了"


def test_deleting_a_line_is_caught_as_a_sequence_break():
    events = vector_events()
    del events[1]
    result = verdict(events)
    assert result.status == asas.FAIL
    assert any("序号断裂" in d for d in result.details)


def test_editing_a_tool_name_without_rechaining_is_caught():
    events = vector_events()
    events[0]["tool"] = "delete_file"
    result = verdict(events)
    assert result.status == asas.FAIL
    assert any("自身哈希不符" in d for d in result.details)


def test_rechaining_the_edited_line_but_not_the_followers_is_caught():
    """改完自己重算哈希、却没跟着修后面：前序哈希会露馅。"""
    events = vector_events()
    events[1]["tool"] = "delete_file"
    body = json.dumps({k: v for k, v in events[1].items() if k != "hash"},
                      sort_keys=True, separators=(",", ":"))
    import hashlib

    events[1]["hash"] = hashlib.sha256(body.encode()).hexdigest()
    result = verdict(events)
    assert result.status == asas.FAIL
    assert any("前序哈希不符" in d for d in result.details)


def test_truncating_the_tail_is_invisible_to_the_rule_but_not_to_the_head():
    """**这条要说清楚**：砍掉尾巴在"流本身"里看不出来——链自洽，规则判通过。

    能发现它的只有一个东西：此前被引用过的摘要。所以凭据里必须留下 head，
    下一次交付才有得比（控制面的跨次比对就是干这个的）。
    """
    events = vector_events()[:2]
    assert verdict(events).status == asas.PASS
    full_head = vector_events()[-1]["hash"]
    truncated_head = events[-1]["hash"]
    assert full_head != truncated_head, "两次的摘要必须不同，否则没法比"
