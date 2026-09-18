"""清单与校验：一致、被改动、缺失、路径不合法。"""

import json
import subprocess
import sys
from pathlib import Path

from ratchet_service import manifest
from ratchet_service.verify import MANIFEST_NAME, verify_directory

REPO = Path(__file__).resolve().parents[2]
VERIFY_PY = REPO / "service" / "src" / "ratchet_service" / "verify.py"


def make_delivery(tmp: Path) -> Path:
    (tmp / "report.md").write_text("# 报告\n\n结论：权限已被收窄。\n", encoding="utf-8")
    (tmp / "policy.json").write_text(json.dumps({"servers": {}}), encoding="utf-8")
    manifest.write_manifest(tmp, note="unit test")
    return tmp


def test_unmodified_delivery_verifies(tmp_path):
    make_delivery(tmp_path)
    result = verify_directory(tmp_path)
    assert result["ok"] is True
    assert result["checked"] == 2          # report.md + policy.json
    assert result["missing"] == []
    assert result["mismatches"] == []


def test_manifest_does_not_list_itself(tmp_path):
    make_delivery(tmp_path)
    data = json.loads((tmp_path / MANIFEST_NAME).read_text(encoding="utf-8"))
    files = [a["file"] for a in data["artifacts"]]
    assert MANIFEST_NAME not in files, "清单不能自引用——那样哈希算不出来"


def test_tampering_is_detected_and_named(tmp_path):
    make_delivery(tmp_path)
    (tmp_path / "report.md").write_text("# 报告\n\n结论：一切正常。\n", encoding="utf-8")
    result = verify_directory(tmp_path)
    assert result["ok"] is False
    assert [m["file"] for m in result["mismatches"]] == ["report.md"]
    assert result["mismatches"][0]["expected"] != result["mismatches"][0]["actual"]


def test_missing_file_is_reported(tmp_path):
    make_delivery(tmp_path)
    (tmp_path / "policy.json").unlink()
    result = verify_directory(tmp_path)
    assert result["ok"] is False
    assert result["missing"] == ["policy.json"]


def test_empty_manifest_is_not_a_pass(tmp_path):
    """空清单不能算通过——否则"什么都没验"会变成"验证通过"。"""
    tmp_path.joinpath(MANIFEST_NAME).write_text(
        json.dumps({"format": manifest.FORMAT, "artifacts": []}), encoding="utf-8"
    )
    result = verify_directory(tmp_path)
    assert result["ok"] is False
    assert result["errorKey"] == "empty"
    assert result["checked"] == 0


def test_not_a_delivery_directory(tmp_path):
    result = verify_directory(tmp_path)
    assert result["ok"] is False
    assert result["errorKey"] == "no_manifest"


def test_standalone_verifier_runs_without_the_package(tmp_path):
    """收货方拿到的是**一个文件**：不装包、不设 PYTHONPATH 也要能跑。"""
    make_delivery(tmp_path)
    proc = subprocess.run(
        [sys.executable, str(VERIFY_PY), str(tmp_path), "--json"],
        capture_output=True, text=True, cwd="/tmp",
        env={"PATH": "/usr/bin:/bin", "HOME": "/tmp"},
    )
    assert proc.returncode == 0, proc.stderr
    assert json.loads(proc.stdout)["ok"] is True

    (tmp_path / "report.md").write_text("changed", encoding="utf-8")
    proc = subprocess.run(
        [sys.executable, str(VERIFY_PY), str(tmp_path)],
        capture_output=True, text=True, cwd="/tmp",
        env={"PATH": "/usr/bin:/bin", "HOME": "/tmp"},
    )
    assert proc.returncode == 1
    assert "MODIFIED" in proc.stdout
