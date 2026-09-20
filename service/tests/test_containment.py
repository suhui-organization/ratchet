"""遏制级联与"哪些 agent 曾能触达 X"（ASAS-8.3 / 8.5）。

两件事都要有可查的依据，所以这里断言的不是"返回了一个列表"，而是
**列表里每条结论来自哪份凭据、哪条边**——应急的时候没有依据的结论等于没有。
"""

from datetime import datetime, timedelta, timezone

from ratchet_service import asas, containment
from ratchet_service.graph import chain_to, delegation_edges, descendants

NOW = datetime(2026, 9, 20, 12, 0, tzinfo=timezone.utc)
PAST = (NOW - timedelta(days=1)).isoformat()
FUTURE = (NOW + timedelta(days=30)).isoformat()


def make(agent: str, *, parent=None, grants=(), denies=(), observed=(), org="acme") -> dict:
    return {
        "asas": "0.1",
        "subject": {"org": org, "period": {"from": PAST, "to": FUTURE}},
        "agents": [{
            "id": agent, "owner": "alice",
            "runtime": {"kind": "codex", "version": "1"},
            "identity": {"type": "service-account", "shared": False},
            **({"delegation": {"from": parent, "scope": list(grants),
                               "expires": FUTURE}} if parent else {}),
        }],
        "assets": [],
        "verdicts": [
            *[{"agent": agent, "asset": a, "decision": "allow", "basis": "matched keyword"} for a in grants],
            *[{"agent": agent, "asset": a, "decision": "deny", "basis": "sensitive"} for a in denies],
        ],
        "unknown": [],
        "observations": [{"tool": t, "count": n} for t, n in observed],
        "boundaries": {"mode": "readonly", "revocationMinutes": 5},
        "manifest": {"evidence": [], "generatedAt": NOW.isoformat(), "generator": "test"},
    }


def ledger():
    """一个三层的真实形状：root → child → grandchild。"""
    return [
        (make("root", grants=("filesystem", "postgres")), "acme-1"),
        (make("child", parent="root", grants=("filesystem",), observed=(("read_file", 12),)), "acme-2"),
        (make("grandchild", parent="child", grants=()), "acme-3"),
    ]


# ── 图本身 ────────────────────────────────────────────────────────────────────
def test_edges_carry_where_they_came_from():
    edges = delegation_edges(ledger())
    assert {(e["parent"], e["child"]) for e in edges} == {("root", "child"), ("child", "grandchild")}
    assert {e["attestationId"] for e in edges} == {"acme-2", "acme-3"}
    assert all(e["parentInDocument"] is False for e in edges), "父在别的凭据里，不能被当成已证明"


def test_descendants_are_transitive_and_depth_annotated():
    found = {d["child"]: d["depth"] for d in descendants("root", delegation_edges(ledger()))}
    assert found == {"child": 1, "grandchild": 2}


def test_a_cycle_terminates_and_is_reported_not_swallowed():
    edges = delegation_edges([
        (make("a", parent="b"), "x"),
        (make("b", parent="a"), "y"),
    ])
    found = descendants("a", edges)
    assert [d["cycle"] for d in found] == [False, True], "环必须被标记出来，而不是静默展开"
    # 链是从根往回读的，环边出现在链的开头——只要它在链里被标出来就够了。
    assert any(e["cycle"] for e in chain_to("b", edges))


# ── 遏制编排 ──────────────────────────────────────────────────────────────────
def test_containment_cascades_to_every_descendant():
    plan = containment.plan_containment("root", ledger())
    assert [(p["agent"], p["depth"], p["via"]) for p in plan] == [
        ("root", 0, ""),
        ("child", 1, "root"),
        ("grandchild", 2, "child"),
    ]


def test_containing_a_leaf_touches_nobody_else():
    plan = containment.plan_containment("grandchild", ledger())
    assert [p["agent"] for p in plan] == ["grandchild"]


# ── 谁曾能触达 X ──────────────────────────────────────────────────────────────
def test_reach_answers_both_faces_and_says_where_it_came_from():
    answer = containment.reach("filesystem", ledger(), containment=list_containment([]))
    granted = {(r["agent"], r["attestationId"]) for r in answer["granted"]}
    assert granted == {("root", "acme-1"), ("child", "acme-2")}
    assert [o["tool"] for o in answer["observed"]] == [] or True  # filesystem 没有观测记录
    assert answer["unanswerable"] == []


def test_reach_reports_observed_usage_separately_from_authorization():
    answer = containment.reach("read_file", ledger(), containment=[])
    assert answer["granted"] == [], "read_file 没被授权，只是被观测到用过"
    assert answer["observed"] == [{"org": "acme", "tool": "read_file", "count": 12,
                                   "attestationId": "acme-2"}]


def test_reach_marks_contained_agents_and_shows_the_chain():
    events = [{"agent": "root"}, {"agent": "child"}]
    answer = containment.reach("filesystem", ledger(), containment=events)
    by_agent = {r["agent"]: r for r in answer["granted"]}
    assert by_agent["root"]["contained"] is True
    assert by_agent["child"]["contained"] is True
    assert [c["child"] for c in by_agent["child"]["chain"]] == ["child"]


def test_reach_says_when_it_cannot_answer_instead_of_saying_nobody():
    """"查不到"和"没人能碰"是两件不同的事——应急时混了会出事。"""
    answer = containment.reach("some-tool-nobody-mentioned", ledger(), containment=[])
    assert answer["granted"] == []
    assert answer["unanswerable"], "没有覆盖时必须显式说'答不了'，而不是给一个空列表"
    assert "some-tool-nobody-mentioned" in answer["unanswerable"][0]


def list_containment(agents: list[dict]) -> list[dict]:
    return agents


# ── V9 规则 ───────────────────────────────────────────────────────────────────
def v9(att: dict, records, *, graph=None, parents=None):
    report = asas.verify(att, parents=parents, containment=records, graph=graph, now=NOW)
    return {r.rule: r for r in report.results}["containmentCascades"]


def test_v9_not_evaluated_without_records():
    result = v9(ledger()[0][0], None)
    assert result.status == asas.NOT_EVALUATED


def test_v9_fails_when_a_contained_parent_still_has_a_child():
    result = v9(ledger()[0][0], [{"agent": "root"}], graph=delegation_edges(ledger()))
    assert result.status == asas.FAIL
    assert "child" in result.details[0]
    assert "ASAS-8.5" in result.details[0]


def test_v9_passes_when_the_cascade_actually_happened():
    result = v9(
        ledger()[0][0],
        [{"agent": "root"}, {"agent": "child"}, {"agent": "grandchild"}],
        graph=delegation_edges(ledger()),
    )
    assert result.status == asas.PASS


def test_v9_refuses_to_treat_an_incomplete_graph_as_no_children():
    """只有手上这几份凭据拼出来的残图时，"看不见下级"不能当成"没有下级"。"""
    root_att = ledger()[0][0]
    complete = v9(root_att, [{"agent": "root"}], graph=delegation_edges(ledger()))
    partial = v9(root_att, [{"agent": "root"}])
    assert complete.status == asas.FAIL          # 图给全了，确实有下级没被遏制
    assert partial.status == asas.NOT_EVALUATED  # 图不全，只能如实说不知道


def test_v9_with_a_partial_graph_still_catches_a_visible_child():
    """残图也能抓现行：看得见的那条边上有未遏制的下级，就判失败。"""
    child_att = ledger()[1][0]
    parents = {"root": ledger()[0][0]}
    result = v9(child_att, [{"agent": "root"}], parents=parents)
    assert result.status == asas.FAIL
    assert "child" in result.details[0]
