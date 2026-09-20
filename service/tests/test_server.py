"""控制面的 HTTP 层测试。

这一层此前**没有任何测试**——而它恰恰是"客户能碰到"的那一层，也是唯一能让
`hashMatch` 从"未评估"变成"已评估"的地方（因为证据在它手里）。所以这里起真的
HTTP 服务、打真的请求，不用 mock：mock 掉的部分正是出过问题的那部分。

覆盖三件本轮新增的事：
  1. 证据上传后，控制面能**自己重算哈希**（T18）；
  2. 遏制的**级联**（吊销父 → 子一起，且记下 via）；
  3. `哪些 agent 曾能触达 X`——以及查不到时**如实说"答不了"**。
"""

import base64
import json
import threading
from datetime import datetime, timedelta, timezone
from http.server import ThreadingHTTPServer
from urllib import request

import pytest

from ratchet_service import server

NOW = datetime(2026, 9, 20, 12, 0, tzinfo=timezone.utc)
PAST = (NOW - timedelta(days=1)).isoformat()
FUTURE = (NOW + timedelta(days=30)).isoformat()


def attestation(agent: str, *, parent=None, grants=(), org="acme") -> dict:
    import hashlib

    policy_bytes = json.dumps({"agent": agent, "servers": {}}, sort_keys=True).encode()
    return {
        "asas": "0.1",
        "subject": {"org": org, "period": {"from": PAST, "to": FUTURE}},
        "agents": [{
            "id": agent, "owner": "alice",
            "runtime": {"kind": "codex", "version": "1"},
            "identity": {"type": "service-account", "shared": False},
            **({"delegation": {"from": parent, "scope": list(grants), "expires": FUTURE}} if parent else {}),
        }],
        "assets": [{"kind": "server", "name": "filesystem", "version": "1.2.3", "pinned": True,
                    "contentHash": "sha256:" + "a" * 64, "descHash": "sha256:" + "b" * 64,
                    "reachableTools": 3}],
        "verdicts": [{"agent": agent, "asset": "filesystem", "decision": "allow", "basis": "matched"}],
        "unknown": [],
        "observations": [],
        "boundaries": {"mode": "readonly", "revocationMinutes": 5},
        "manifest": {
            "evidence": [{"file": "policy.json", "sha256": hashlib.sha256(policy_bytes).hexdigest()}],
            "generatedAt": NOW.isoformat(),
            "generator": "test",
        },
    }


@pytest.fixture()
def api(tmp_path, monkeypatch):
    monkeypatch.setattr(server, "DB_PATH", str(tmp_path / "asas.db"))
    httpd = ThreadingHTTPServer(("127.0.0.1", 0), server.Handler)
    thread = threading.Thread(target=httpd.serve_forever, daemon=True)
    thread.start()
    base = f"http://127.0.0.1:{httpd.server_address[1]}"
    try:
        yield base
    finally:
        httpd.shutdown()
        httpd.server_close()


def call(base: str, path: str, payload=None, method: str | None = None):
    data = json.dumps(payload).encode() if payload is not None else None
    req = request.Request(
        base + path, data=data, method=method or ("POST" if data else "GET"),
        headers={"Content-Type": "application/json"},
    )
    try:
        with request.urlopen(req) as resp:
            return resp.status, json.loads(resp.read() or b"{}")
    except request.HTTPError as exc:
        return exc.code, json.loads(exc.read() or b"{}")


def test_healthz(api):
    assert call(api, "/healthz") == (200, {"status": "ok"})


def test_attestation_roundtrip(api):
    att = attestation("a1")
    status, stored = call(api, "/attestations", att)
    assert status == 201 and stored["id"].startswith("acme-")
    status, listing = call(api, "/attestations")
    assert [a["id"] for a in listing["attestations"]] == [stored["id"]]
    assert call(api, f"/attestations/{stored['id']}")[1]["agents"][0]["id"] == "a1"


def test_verify_without_evidence_says_not_evaluated(api):
    """证据没上传时，控制面**不能**假装验过哈希。"""
    att = attestation("a1")
    call(api, "/attestations", att)
    status, report = call(api, "/verify", att)
    assert status == 200
    rules = {r["rule"]: r for r in report["results"]}
    assert rules["hashMatch"]["status"] == "not_evaluated"
    assert "hashMatch" in report["notEvaluated"]


def test_evidence_upload_lets_the_control_plane_recompute_hashes(api):
    """T18 的判据：证据上传后，控制面自己能把哈希重算出来并比对。"""
    att = attestation("a1")
    call(api, "/attestations", att)
    ident = call(api, "/attestations", att)[1]["id"]
    policy_bytes = json.dumps({"agent": "a1", "servers": {}}, sort_keys=True).encode()
    status, stored = call(api, "/evidence", {
        "attestationId": ident,
        "files": {"policy.json": base64.b64encode(policy_bytes).decode()},
    })
    assert status == 201 and stored["stored"] == [{"file": "policy.json", "bytes": len(policy_bytes)}]

    status, report = call(api, "/verify", att)
    rules = {r["rule"]: r for r in report["results"]}
    assert status == 200 and report["ok"] is True
    assert rules["hashMatch"]["status"] == "pass"


def test_tampered_evidence_is_caught_by_the_control_plane(api):
    """上传的字节和 manifest 里的哈希对不上 → 控制面必须判 fail（不是"上传成功"就算数）。"""
    att = attestation("a1")
    ident = call(api, "/attestations", att)[1]["id"]
    call(api, "/evidence", {
        "attestationId": ident,
        "files": {"policy.json": base64.b64encode(b"tampered").decode()},
    })
    status, report = call(api, "/verify", att)
    rules = {r["rule"]: r for r in report["results"]}
    assert status == 422 and rules["hashMatch"]["status"] == "fail"
    assert "policy.json" in rules["hashMatch"]["details"][0]


def test_evidence_rejects_non_base64(api):
    status, body = call(api, "/evidence", {"attestationId": "x", "files": {"p": "不是 base64!!"}})
    assert status == 400 and "base64" in body["error"]


def test_containment_cascades_and_records_where_it_came_from(api):
    call(api, "/attestations", attestation("root", grants=("filesystem",)))
    child = attestation("child", parent="root", grants=("filesystem",))
    call(api, "/attestations", child)

    status, result = call(api, "/contain", {"agent": "root", "reason": "credential leak"})
    assert status == 201
    assert result["contained"] == ["root", "child"]
    assert result["cascaded"] == [{"agent": "child", "via": "root", "depth": 1}]

    _, state = call(api, "/contain")
    assert {r["agent"] for r in state["containment"]} == {"root", "child"}
    assert {e["child"] for e in state["graph"]} == {"child"}, "遏制导出要带完整的委派图"


def test_dry_run_shows_the_blast_radius_without_pulling_the_trigger(api):
    """先看再执行：预演要给出完整名单，但**一条记录也不能落库**。"""
    call(api, "/attestations", attestation("root", grants=("filesystem",)))
    call(api, "/attestations", attestation("child", parent="root", grants=("filesystem",)))
    status, plan = call(api, "/contain", {"agent": "root", "dryRun": True})
    assert status == 201 and plan["dryRun"] is True
    assert plan["contained"] == ["root", "child"]
    assert call(api, "/contain")[1]["containment"] == [], "预演不能留下任何记录"


def test_after_containment_the_child_credential_fails_v9(api):
    """遏制之后再去验子凭据：V9 必须报出来——这正是"吊销"这件事的可验证形式。"""
    call(api, "/attestations", attestation("root", grants=("filesystem",)))
    child = attestation("child", parent="root", grants=("filesystem",))
    call(api, "/attestations", child)
    call(api, "/contain", {"agent": "root"})  # 会级联到 child
    status, report = call(api, "/verify", child)
    rules = {r["rule"]: r for r in report["results"]}
    assert rules["containmentCascades"]["status"] == "pass"
    # 父凭据在台账里，所以 V8 也能评：child 的作用域没超出 root
    assert rules["delegationNarrows"]["status"] == "pass"


def test_containment_without_cascade_is_caught(api, monkeypatch):
    """绕过级联（直接往库里塞一条只有父的记录）必须被 V9 抓住。"""
    call(api, "/attestations", attestation("root", grants=("filesystem",)))
    child = attestation("child", parent="root", grants=("filesystem",))
    call(api, "/attestations", child)
    manual = server.record_containment("child")  # 先吊销子，父还活着：这不是违规
    assert manual["cascaded"] == []
    # 直接插一条"父被吊销"的记录，绕过级联逻辑
    import sqlite3

    with server._lock, server.connect() as conn:
        conn.execute(
            "INSERT INTO containment (agent, via, reason, recorded_at) VALUES (?,?,?,?)",
            ("root", "", "manual", NOW.isoformat()),
        )
        conn.execute("DELETE FROM containment WHERE agent = 'child'")
    status, report = call(api, "/verify", child)
    rules = {r["rule"]: r for r in report["results"]}
    assert status == 422
    assert rules["containmentCascades"]["status"] == "fail"


def test_reach_answers_who_could_touch_a_subject(api):
    call(api, "/attestations", attestation("root", grants=("filesystem",)))
    call(api, "/attestations", attestation("child", parent="root", grants=("filesystem",)))
    status, answer = call(api, "/reach?subject=filesystem")
    assert status == 200
    assert sorted(r["agent"] for r in answer["granted"]) == ["child", "root"]
    assert answer["unanswerable"] == []


def test_reach_says_i_cannot_answer_instead_of_nobody(api):
    status, answer = call(api, "/reach?subject=nothing-mentioned-this")
    assert status == 200
    assert answer["granted"] == []
    assert answer["unanswerable"], "没有凭据覆盖时必须显式说'答不了'"


def test_reach_requires_a_subject(api):
    status, body = call(api, "/reach")
    assert status == 400 and "subject" in body["error"]
