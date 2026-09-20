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
    POST /verify               跑 ASAS-V（与 CLI 同一份实现）
    POST /contain              记录一次遏制动作（ASAS-8）
    GET  /contain              遏制记录
"""

from __future__ import annotations

import json
import os
import sqlite3
import threading
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from . import asas

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
  reason     TEXT,
  recorded_at TEXT NOT NULL
);
"""


def connect() -> sqlite3.Connection:
    directory = os.path.dirname(DB_PATH)
    if directory:
        os.makedirs(directory, exist_ok=True)
    conn = sqlite3.connect(DB_PATH)
    conn.executescript(SCHEMA)
    return conn


def _attestation_id(att: dict) -> str:
    subject = att.get("subject") or {}
    stamp = str(subject.get("period", {}).get("to") or "")[:10] or "unknown"
    org = "".join(ch for ch in str(subject.get("org") or "org") if ch.isalnum() or ch in "-_")[:24]
    return f"{org}-{stamp}"


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


def record_containment(agent: str, reason: str = "") -> dict:
    now = datetime.now(timezone.utc).isoformat()
    with _lock, connect() as conn:
        cur = conn.execute(
            "INSERT INTO containment (agent, reason, recorded_at) VALUES (?,?,?)", (agent, reason, now)
        )
        ident = cur.lastrowid
    return {"id": ident, "agent": agent, "reason": reason, "recordedAt": now}


def list_containment() -> list[dict]:
    with _lock, connect() as conn:
        rows = conn.execute(
            "SELECT id, agent, reason, recorded_at FROM containment ORDER BY id DESC"
        ).fetchall()
    return [{"id": r[0], "agent": r[1], "reason": r[2], "recordedAt": r[3]} for r in rows]


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
        elif self.path == "/attestations":
            self._send(200, {"attestations": list_attestations()})
        elif self.path.startswith("/attestations/"):
            att = get_attestation(self.path.rsplit("/", 1)[-1])
            self._send(200, att) if att else self._send(404, {"error": "not found"})
        elif self.path == "/contain":
            self._send(200, {"containment": list_containment()})
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
            # 与 CLI 完全一致：直接调同一份实现，不在服务端重写规则
            report = asas.verify(payload, files=None, events=None)
            code = 200 if report.ok else 422
            return self._send(code, report.as_dict())

        if self.path == "/contain":
            agent = str(payload.get("agent") or "")
            if not agent:
                return self._send(400, {"error": "agent is required"})
            return self._send(201, record_containment(agent, str(payload.get("reason") or "")))

        return self._send(404, {"error": "not found"})


def main() -> int:
    port = int(os.environ.get("ASAS_PORT", "8080"))
    server = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    print(f"asas control plane listening on :{port} (db={DB_PATH})", flush=True)
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
