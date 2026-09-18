"""分享包：一个 JSON 文件，收货方拖进页面就能验。"""

import json

from ratchet_service import manifest
from ratchet_service.cli import bundle
from ratchet_service.verify import verify_directory


def test_bundle_carries_manifest_and_content(tmp_path):
    src = tmp_path / "delivery"
    src.mkdir()
    (src / "report.md").write_text("# 报告\n", encoding="utf-8")
    (src / "policy.json").write_text('{"servers": {}}', encoding="utf-8")
    manifest.write_manifest(src)

    out = bundle(src, tmp_path / "share.json")
    payload = json.loads(out.read_text(encoding="utf-8"))

    assert payload["format"] == "ratchet-bundle/v1"
    assert {a["file"] for a in payload["artifacts"]} == {"report.md", "policy.json"}
    # 清单与产物必须都在包里：缺任何一个，收货方就没法在本地算哈希
    assert len(payload["manifest"]["artifacts"]) == 2


def test_bundle_content_matches_manifest_hashes(tmp_path):
    """包里带的内容必须与清单里的哈希对得上，否则网页上永远验不过。"""
    import hashlib

    src = tmp_path / "delivery"
    src.mkdir()
    (src / "report.md").write_text("# 报告\n", encoding="utf-8")
    (src / "policy.json").write_text("{}", encoding="utf-8")
    manifest.write_manifest(src)

    payload = json.loads(bundle(src, tmp_path / "share.json").read_text(encoding="utf-8"))
    for artifact, entry in zip(payload["artifacts"], payload["manifest"]["artifacts"]):
        assert artifact["file"] == entry["file"]
        actual = hashlib.sha256(artifact["content"].encode("utf-8")).hexdigest()
        assert actual == entry["sha256"]


def test_bundle_does_not_include_manifest_itself(tmp_path):
    src = tmp_path / "delivery"
    src.mkdir()
    (src / "report.md").write_text("# 报告\n", encoding="utf-8")
    manifest.write_manifest(src)
    payload = json.loads(bundle(src, tmp_path / "share.json").read_text(encoding="utf-8"))
    assert "manifest.json" not in [a["file"] for a in payload["artifacts"]]


def test_directory_verify_still_works_after_bundling(tmp_path):
    src = tmp_path / "delivery"
    src.mkdir()
    (src / "report.md").write_text("# 报告\n", encoding="utf-8")
    manifest.write_manifest(src)
    bundle(src, tmp_path / "share.json")
    assert verify_directory(src)["ok"] is True
