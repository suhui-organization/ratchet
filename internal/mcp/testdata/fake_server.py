#!/usr/bin/env python3
"""最小的 stdio MCP server，只用来给测试当靶子。

刻意做三件事：往 stdout 打一行非 JSON（验证客户端会跳过）、
按协议回应 initialize 与 tools/list、返回两个工具。
"""
import json
import sys

print("this line is not json — a noisy server should not break the client", flush=True)

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        msg = json.loads(line)
    except json.JSONDecodeError:
        continue
    method = msg.get("method")
    if method == "initialize":
        print(json.dumps({
            "jsonrpc": "2.0", "id": msg.get("id"),
            "result": {
                "protocolVersion": "2024-11-05",
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "fake", "version": "0"},
            },
        }), flush=True)
    elif method == "tools/list":
        print(json.dumps({
            "jsonrpc": "2.0", "id": msg.get("id"),
            "result": {"tools": [
                {"name": "read_file", "description": "Read a file from disk"},
                {"name": "delete_file", "description": "Delete a file"},
            ]},
        }), flush=True)
