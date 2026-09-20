"""ASAS 控制面（单租户 v1）。

三条设计约束，全部来自前面定下的纪律：

1. **只用标准库**。控制面如果自己需要一堆依赖，部署本身就成了问题——
   与 CLI「收货方零安装」是同一条原则。
2. **判定逻辑不在这里重写**。`/verify` 直接调用 `asas.verify`：规则只有一份，
   服务端与 CLI 的结果必须逐字一致（P1 的验收判据）。
3. **存储先用 sqlite（落 PVC）**。计划里写的是 postgres；v1 用 sqlite 是有意的取舍：
   单租户、无并发写、表结构就是"一行一份凭据"，迁移路径清晰。

端点：
    GET  /healthz              探针
    POST /attestations         存一份凭据
    GET  /attestations         列表
    GET  /attestations/<id>    取全文
    POST /evidence             上传证据文件（base64），让 hashMatch 可被重算
    POST /verify               跑 ASAS-V（与 CLI 同一份实现）
    POST /contain              记录一次遏制动作，**级联到下级**（ASAS-8.5）
    GET  /contain              遏制记录
    GET  /reach?subject=<x>    谁曾能触达 X（ASAS-8.3）

`/verify` 与「本地 CLI」的差别只有一处，而那一处正是控制面存在的理由：**它手边有台账**。
证据文件、委派父凭据、遏制记录都能从库里自己取，所以同一份凭据在这里能被评得更全——
`hashMatch` 与 `containmentCascades` 不再落到"未评估"。
"""

from __future__ import annotations

import base64
import hashlib
import json
import os
import sqlite3
import threading
import urllib.parse
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from . import asas, containment

DB_PATH = os.environ.get("ASAS_DB", "/data/asas.db")

_lock = threading.Lock()

SCHEMA = """
CREATE TABLE IF NOT EXISTS attestations (
  id         TEXT PRIMARY KEY,
  org        TEXT,
  generated  TEXT,
  stored_at  TEXT,
  body       TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS containment (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  agent      TEXT NOT NULL,
  via        TEXT NOT NULL DEFAULT '',
  reason     TEXT,
  recorded_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS evidence (
  attestation_id TEXT NOT NULL,
  name           TEXT NOT NULL,
  body           BLOB NOT NULL,
  stored_at      TEXT NOT NULL,
  PRIMARY KEY (attestation_id, name)
);
"""

# sqlite 的 ALTER 没有 IF NOT EXISTS：老库（0.16 之前建的）缺 via 列时补上，
# 否则升级后第一条遏制记录就会报 "no such column"。
SCHEMA_UPGRADES = [
    "ALTER TABLE containment ADD COLUMN via TEXT NOT NULL DEFAULT ''",
]


def _migrate(conn: sqlite3.Connection) -> None:
    for statement in SCHEMA_UPGRADES:
        try:
            conn.execute(statement)
        except sqlite3.OperationalError:
            pass  # 已经加过了


def connect() -> sqlite3.Connection:
    directory = os.path.dirname(DB_PATH)
    if directory:
        os.makedirs(directory, exist_ok=True)
    conn = sqlite3.connect(DB_PATH)
    conn.executescript(SCHEMA)
    _migrate(conn)
    return conn


def _attestation_id(att: dict) -> str:
    """台账主键：组织 + 覆盖日期 + **内容哈希前 8 位**。

    为什么不能只用"组织-日期"：那样同一个组织的**第二个主体**会把第一个覆盖掉——
    本机实测踩过（父与子两份凭据互相顶掉，`哪些 agent 曾能触达 X` 只剩一个）。
    加内容哈希之后：同一份凭据重复上报仍然是幂等的 upsert，不同主体各占一行。
    """
    subject = att.get("subject") or {}
    stamp = str(subject.get("period", {}).get("to") or "")[:10] or "unknown"
    org = "".join(ch for ch in str(subject.get("org") or "org") if ch.isalnum() or ch in "-_")[:24]
    body = json.dumps(att, sort_keys=True, ensure_ascii=False).encode()
    return f"{org}-{stamp}-{hashlib.sha256(body).hexdigest()[:8]}"


def store(att: dict) -> dict:
    now = datetime.now(timezone.utc).isoformat()
    ident = _attestation_id(att)
    with _lock, connect() as conn:
        conn.execute(
            "INSERT INTO attestations (id, org, generated, stored_at, body) VALUES (?,?,?,?,?) "
            "ON CONFLICT(id) DO UPDATE SET body=excluded.body, stored_at=excluded.stored_at",
            (ident, (att.get("subject") or {}).get("org") or "", att.get("manifest", {}).get("generatedAt") or "",
             now, json.dumps(att, ensure_ascii=False)),
        )
    return {"id": ident, "storedAt": now}


def list_attestations() -> list[dict]:
    with _lock, connect() as conn:
        rows = conn.execute(
            "SELECT id, org, generated, stored_at FROM attestations ORDER BY stored_at DESC"
        ).fetchall()
    return [{"id": r[0], "org": r[1], "generatedAt": r[2], "storedAt": r[3]} for r in rows]


def get_attestation(ident: str) -> dict | None:
    with _lock, connect() as conn:
        row = conn.execute("SELECT body FROM attestations WHERE id = ?", (ident,)).fetchone()
    return json.loads(row[0]) if row else None


def stored_pairs() -> list[tuple[dict, str]]:
    """台账里的 (凭据, id) 列表。遏制级联与触达查询都从这一份快照出发。"""
    with _lock, connect() as conn:
        rows = conn.execute("SELECT id, body FROM attestations").fetchall()
    return [(json.loads(body), ident) for ident, body in rows]


def record_containment(agent: str, reason: str = "", *, dry_run: bool = False) -> dict:
    """记一次遏制，**并沿委派链级联**（ASAS-8.5）。

    为什么要级联：吊销一个 agent 却留着它的下级，等于把最容易漏掉的那部分留在原地。
    级联的依据是台账里能看到的委派边；哪一条边让它被吊销的（`via`）也一起记下来——
    事后复盘时"为什么它也被吊销了"必须能回答。
    """
    now = datetime.now(timezone.utc).isoformat()
    plan = containment.plan_containment(agent, stored_pairs())
    if not dry_run:
        with _lock, connect() as conn:
            for item in plan:
                conn.execute(
                    "INSERT INTO containment (agent, via, reason, recorded_at) VALUES (?,?,?,?)",
                    (item["agent"], item["via"], reason, now),
                )
    return {
        "agent": agent,
        "reason": reason,
        "recordedAt": now,
        "dryRun": dry_run,
        "contained": [item["agent"] for item in plan],
        "cascaded": [
            {"agent": item["agent"], "via": item["via"], "depth": item["depth"]}
            for item in plan
            if item["depth"] > 0
        ],
    }


def list_containment() -> list[dict]:
    with _lock, connect() as conn:
        rows = conn.execute(
            "SELECT id, agent, via, reason, recorded_at FROM containment ORDER BY id DESC"
        ).fetchall()
    return [
        {"id": r[0], "agent": r[1], "via": r[2], "reason": r[3], "recordedAt": r[4]}
        for r in rows
    ]


def containment_state() -> dict:
    """遏制记录 + **完整的委派图**。两样一起给，因为 V9 需要它们一起才成立：
    只看记录没法知道谁是谁的下级，只看图没法知道谁被吊销了。
    """
    pairs = stored_pairs()
    return {"containment": list_containment(), "graph": containment.graph_of(pairs)}


def store_evidence(attestation_id: str, files: dict[str, str]) -> dict:
    """存证据文件（base64 传入）。**存的是字节，不是解析后的结构**——重算哈希要原样字节。"""
    now = datetime.now(timezone.utc).isoformat()
    saved: list[dict] = []
    with _lock, connect() as conn:
        for name, payload in files.items():
            try:
                body = base64.b64decode(payload, validate=True)
            except (ValueError, TypeError) as exc:
                raise ValueError(f"{name}: 不是合法的 base64（{exc}）") from exc
            conn.execute(
                "INSERT INTO evidence (attestation_id, name, body, stored_at) VALUES (?,?,?,?) "
                "ON CONFLICT(attestation_id, name) DO UPDATE SET body=excluded.body, stored_at=excluded.stored_at",
                (attestation_id, name, body, now),
            )
            saved.append({"file": name, "bytes": len(body)})
    return {"attestationId": attestation_id, "stored": saved, "storedAt": now}


def load_evidence(attestation_id: str) -> dict[str, bytes] | None:
    """取回证据文件。一条都没有时返回 None——**None 表示"没有"，不是"空文件集"**，
    两者在验证器里分别对应 not_evaluated 与 pass。"""
    with _lock, connect() as conn:
        rows = conn.execute(
            "SELECT name, body FROM evidence WHERE attestation_id = ?", (attestation_id,)
        ).fetchall()
    return {name: body for name, body in rows} if rows else None


def resolve_parents(att: dict, pairs: list[tuple[dict, str]] | None = None) -> dict[str, dict]:
    """给一份凭据找它的委派父凭据（同组织、agent 名相同）。

    这就是"控制面手里有台账"的具体含意：子凭据只声明 `delegation.from = X`，
    真正的 X 的权限范围要去库里找。
    """
    pairs = pairs if pairs is not None else stored_pairs()
    org = str((att.get("subject") or {}).get("org") or "")
    wanted = {
        str((a.get("delegation") or {}).get("from") or "")
        for a in att.get("agents") or []
        if a.get("delegation")
    } - {""}
    resolved: dict[str, dict] = {}
    for parent_att, _ident in pairs:
        if str((parent_att.get("subject") or {}).get("org") or "") != org:
            continue
        for agent in parent_att.get("agents") or []:
            if str(agent.get("id") or "") in wanted:
                resolved[str(agent["id"])] = parent_att
    return resolved


class Handler(BaseHTTPRequestHandler):
    server_version = "asas/0.1"

    def log_message(self, fmt, *args):  # 保持默认日志格式，只是显式声明
        super().log_message(fmt, *args)

    def _send(self, code: int, payload) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _body(self) -> dict:
        length = int(self.headers.get("Content-Length") or 0)
        if not length:
            return {}
        return json.loads(self.rfile.read(length).decode())

    def do_GET(self):  # noqa: N802 (stdlib 约定)
        if self.path == "/healthz":
            self._send(200, {"status": "ok"})
        elif self.path.startswith("/reach"):
            query = urllib.parse.urlparse(self.path).query
            subject = (urllib.parse.parse_qs(query).get("subject") or [""])[0]
            if not subject:
                return self._send(400, {"error": "subject is required, e.g. /reach?subject=filesystem"})
            pairs = stored_pairs()
            return self._send(200, containment.reach(subject, pairs, list_containment()))
        elif self.path == "/attestations":
            self._send(200, {"attestations": list_attestations()})
        elif self.path.startswith("/attestations/"):
            att = get_attestation(self.path.rsplit("/", 1)[-1])
            self._send(200, att) if att else self._send(404, {"error": "not found"})
        elif self.path == "/contain":
            self._send(200, containment_state())
        else:
            self._send(404, {"error": "not found"})

    def do_POST(self):  # noqa: N802
        try:
            payload = self._body()
        except ValueError as exc:
            return self._send(400, {"error": f"invalid json: {exc}"})

        if self.path == "/attestations":
            if not isinstance(payload, dict) or "asas" not in payload:
                return self._send(400, {"error": "body must be an ASAS-A attestation"})
            return self._send(201, store(payload))

        if self.path == "/verify":
            # 与 CLI **同一份实现**——差别只在输入：控制面从台账里自己取证据、父凭据、
            # 遏制记录。规则不在服务端重写一行。
            ident = _attestation_id(payload)
            pairs = stored_pairs()
            report = asas.verify(
                payload,
                files=load_evidence(ident),
                events=None,
                parents=resolve_parents(payload, pairs),
                containment=list_containment(),
                graph=containment.graph_of(pairs),
            )
            code = 200 if report.ok else 422
            return self._send(code, report.as_dict())

        if self.path == "/evidence":
            ident = str(payload.get("attestationId") or "")
            files = payload.get("files")
            if not ident or not isinstance(files, dict) or not files:
                return self._send(400, {"error": "attestationId and a non-empty files map are required"})
            try:
                return self._send(201, store_evidence(ident, files))
            except ValueError as exc:
                return self._send(400, {"error": str(exc)})

        if self.path == "/contain":
            agent = str(payload.get("agent") or "")
            if not agent:
                return self._send(400, {"error": "agent is required"})
            # dryRun：先把要吊销的名单拿给人看一眼再动手。高危动作不该只有一把扳机。
            return self._send(
                201,
                record_containment(
                    agent, str(payload.get("reason") or ""), dry_run=bool(payload.get("dryRun"))
                ),
            )

        return self._send(404, {"error": "not found"})


def main() -> int:
    port = int(os.environ.get("ASAS_PORT", "8080"))
    server = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    print(f"asas control plane listening on :{port} (db={DB_PATH})", flush=True)
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
