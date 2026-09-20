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


def test_index_page_is_html_not_a_404(api):
    """浏览器打开根路径要看到端点清单。

    真实反馈：之前没有 `/` 路由，打开就是 `{"error": "not found"}`——
    用户以为服务坏了。在这个位置，**看起来坏了就是坏了**。
    """
    import urllib.request

    with urllib.request.urlopen(api + "/") as resp:
        assert resp.status == 200
        assert resp.headers["Content-Type"].startswith("text/html")
        html = resp.read().decode()
    for expected in ("Ratchet 控制面", "/verify", "/silence", "/reach", "没有任何认证"):
        assert expected in html, f"首页缺了 {expected}"
    # 内网可用：页面**不加载**任何外部资源（一个 CDN 就等于一页白屏）。
    # 注意区分"外链"与"外部资源"：指向 GitHub 的文字链接没问题，
    # 会发请求的 script/link/字体/@import 才是致命的。
    assert "<script src" not in html and "<link rel=\"stylesheet\"" not in html
    assert "@import" not in html and "fetch('http" not in html


def test_index_shows_live_counts(api):
    import urllib.request

    call(api, "/attestations", attestation("a1"))
    html = urllib.request.urlopen(api + "/").read().decode()
    # 计数现在来自 /ledger（页面自己渲染），所以这里查的是数据源而不是页面里的字面量
    ledger = json.loads(urllib.request.urlopen(api + "/ledger").read())
    assert ledger["counts"]["credentials"] == 1
    assert "id=\"c-cred\"" in html


def test_ledger_gives_the_console_everything_in_one_call(api):
    """控制台的数据源：凭据 + 每份的九条规则 + 断流 + 遏制。

    为什么必须是后端算好：界面**不许重新实现验证规则**（项目第 1 条原则）。
    所以这里断言的就是"后端把判定算出来了"，而不是"界面能算"。
    """
    att = attestation("a1")
    ident = call(api, "/attestations", att)[1]["id"]
    status, ledger = call(api, "/ledger")
    assert status == 200
    assert ledger["counts"] == {"credentials": 1, "chains": 0, "breaks": 0, "containment": 0}
    entry = ledger["credentials"][0]
    assert entry["id"] == ident
    assert entry["agents"] == ["a1"]
    assert entry["assets"] == ["filesystem"]
    rules = {r["rule"] for r in entry["report"]["results"]}
    assert "silenceIsAuditable" in rules and "allPinnedOrExempt" in rules
    # 没上传证据/事件时，规则如实报"未评估"，不是通过
    assert entry["report"]["notEvaluated"] == ["hashMatch", "silenceIsAuditable"] or set(
        entry["report"]["notEvaluated"]
    ) >= {"hashMatch", "silenceIsAuditable"}


def test_ledger_reports_isolation_of_each_credential(api):
    """两份凭据互不干扰：各自的规则结果按各自的输入算（这是台账最容易写错的地方）。"""
    first = attestation("a1")
    ident = call(api, "/attestations", first)[1]["id"]
    policy_bytes = json.dumps({"agent": "a1", "servers": {}}, sort_keys=True).encode()
    call(api, "/evidence", {"attestationId": ident,
                            "files": {"policy.json": base64.b64encode(policy_bytes).decode()}})
    second = attestation("a2")
    another = call(api, "/attestations", second)[1]["id"]
    assert another.startswith("acme-") and another != ident

    ledger = call(api, "/ledger")[1]
    by_id = {c["id"]: c for c in ledger["credentials"]}
    assert "hashMatch" not in by_id[ident]["report"]["notEvaluated"], "传了证据的那份应当能重算哈希"
    assert "hashMatch" in by_id[another]["report"]["notEvaluated"], "没传证据的那份不该跟着变"


def test_console_copy_has_no_markdown_marks_or_hidden_entry_point(api):
    """控制台是 HTML，不是 Markdown。

    真实缺陷：正文里写了 `**交得出东西**`，浏览器把它**原样**显示成四个星号。
    这类错只有"真的打开看一眼"才会发现——所以把它变成断言。
    """
    import re
    import urllib.request

    html = urllib.request.urlopen(api + "/").read().decode()
    body = html.split("</style>", 1)[1].split("<script>", 1)[0]
    assert not re.search(r"\*\*[^*\n]{1,40}\*\*", body), "页面正文里残留 Markdown 加粗"
    assert "`" not in body, "页面正文里残留 Markdown 反引号"

    # 入口指引必须在页面里：陌生人打开要能看到"从哪开始"。
    assert 'id="onboard"' in html and "把这台机器接进来" in html
    assert "这不是防火墙" in html, "定位那句话必须在页面上"
    assert 'id="cmd-attest"' in html, "接入命令的容器要在（内容由 JS 按当前地址填）"


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


# ── 事件流：能让 V6 评起来，也能抓住"改写历史" ────────────────────────────────
def chain_events(count: int, *, tool: str = "read_file") -> list[dict]:
    """造一条自洽的链（哈希口径与 Go 侧一致：去掉 hash 后按键排序序列化）。"""
    import hashlib

    events: list[dict] = []
    prev = ""
    for seq in range(count):
        event = {"seq": seq, "prevHash": prev, "agent": "a1", "tool": tool, "decision": "allow"}
        body = json.dumps(event, sort_keys=True, separators=(",", ":"))
        event["hash"] = hashlib.sha256(body.encode()).hexdigest()
        prev = event["hash"]
        events.append(event)
    return events


def test_events_upload_makes_the_silence_rule_evaluable(api):
    """T21 的判据：上传事件流之后，V6 不再是"未评估"。"""
    att = attestation("a1")
    ident = call(api, "/attestations", att)[1]["id"]
    policy_bytes = json.dumps({"agent": "a1", "servers": {}}, sort_keys=True).encode()
    call(api, "/evidence", {"attestationId": ident,
                            "files": {"policy.json": base64.b64encode(policy_bytes).decode()}})

    status, stored = call(api, "/events", {"attestationId": ident, "events": chain_events(5)})
    assert status == 201 and stored["events"] == 5 and stored["break"] is None

    status, report = call(api, "/verify", att)
    rules = {r["rule"]: r for r in report["results"]}
    assert rules["silenceIsAuditable"]["status"] == "pass"
    assert report["notEvaluated"] == [], "证据与事件都齐了，不该还有未评估的规则"


def test_a_rewritten_log_is_caught_on_the_next_upload(api):
    """砍掉尾巴的链**自己完全自洽**——只有比对上一次的摘要才看得见。

    这是"可验证的沉默"里最难的一条：本地日志被改写，单看这一次的交付毫无异常。
    """
    att = attestation("a1")
    ident = call(api, "/attestations", att)[1]["id"]
    call(api, "/events", {"attestationId": ident, "events": chain_events(5)})

    # 下一次交付只交前 3 条（尾巴被砍）——链本身是自洽的
    short = chain_events(5)[:3]
    status, stored = call(api, "/events", {"attestationId": ident, "events": short})
    assert status == 201
    assert stored["break"] is not None, "断流必须被记成事件"
    assert "改写" in stored["break"]["reason"] or "变短" in stored["break"]["reason"]
    assert stored["break"]["expectedSeq"] == 4 and stored["break"]["actualSeq"] == 2

    state = call(api, "/silence")[1]
    assert len(state["breaks"]) == 1
    assert state["heads"][0]["lastSeq"] == 2, "摘要更新到这一趟看到的为止"


def test_editing_a_middle_event_is_caught_too(api):
    """改中间一条、把 hash 字段原样留着——这最像"正常日志"，只看链头看不见。

    控制面手上有上次的逐条哈希，所以它比"只比摘要"更强：能点到具体是第几条。
    """
    att = attestation("a1")
    ident = call(api, "/attestations", att)[1]["id"]
    good = chain_events(5)
    call(api, "/events", {"attestationId": ident, "events": good})

    tampered = json.loads(json.dumps(good))
    tampered[1]["tool"] = "delete_file"
    status, stored = call(api, "/events", {"attestationId": ident, "events": tampered})
    assert status == 201 and stored["break"] is not None
    assert "被改写" in stored["break"]["reason"]
    assert "seq 1" in stored["break"]["reason"]


def test_a_growing_chain_is_not_a_break(api):
    """正常情况：下一趟是上一趟的延长线。不能把正常增长报成事故。"""
    att = attestation("a1")
    ident = call(api, "/attestations", att)[1]["id"]
    first = chain_events(3)
    call(api, "/events", {"attestationId": ident, "events": first})
    status, stored = call(api, "/events", {"attestationId": ident, "events": chain_events(6)})
    assert status == 201 and stored["break"] is None
    assert call(api, "/silence")[1]["breaks"] == []


def test_events_are_required_to_be_a_non_empty_list(api):
    status, body = call(api, "/events", {"attestationId": "x", "events": []})
    assert status == 400 and "events" in body["error"]
