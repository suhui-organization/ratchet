"""报告渲染：四节齐全、依据可追、边界写清、语言正确。"""

import json

from ratchet_service import manifest, report

POLICY = {
    "version": "1",
    "agent": "codex",
    "defaultDecision": "deny",
    "generatedAt": "2026-09-18T00:00:00Z",
    "servers": {
        "filesystem": {"allow": ["read_file"], "approve": ["write_file"], "deny": ["delete_file"]},
        "internal": {"allow": [], "approve": ["acme_opaque"], "deny": []},
    },
    # 依据由 Go 引擎生成；英文产物要求它本身也是英文（Go 侧默认 en-US）
    "rationale": {
        "filesystem/read_file": 'name matches "read"; observed 120 call(s)',
        "filesystem/write_file": 'name matches "write"; observed 9 call(s)',
        "filesystem/delete_file": 'name matches "delete"; observed 1 call(s)',
        "internal/acme_opaque": "no capability keyword in the tool name or description; never observed (present in the inventory only)",
    },
    "needsReview": ["internal/acme_opaque"],
}


def has_cjk(text: str) -> bool:
    return any("\u4e00" <= ch <= "\u9fff" for ch in text)


def test_default_is_english_and_has_no_chinese():
    """默认产物是英文。这是这一版的核心要求：站点是英文，给客户的东西不能是中文。"""
    text = report.render_report(POLICY, generated_at="2026-09-18T00:00:00Z")
    assert not has_cjk(text), "英文产物里出现了中文"
    for heading in ["## 1. Summary", "## 2. Policy table",
                    "## 3. Why each verdict was made", "## 4. Coverage boundary"]:
        assert heading in text, f"缺少 {heading}"


def test_counts_and_review_are_english():
    text = report.render_report(POLICY)
    assert "**1** allowed" in text
    assert "**2** require approval" in text
    assert "**1** denied" in text
    assert "review that batch before you rely on this policy" in text


def test_coverage_boundary_is_explicit():
    text = report.render_report(POLICY)
    # 「未观测」不等于「没发生」——这句必须在，否则会被读成安全结论
    assert '"not observed" does not mean "did not happen"' in text


def test_chinese_still_available():
    """中文没有被删掉，只是不再是默认。"""
    text = report.render_report(POLICY, locale="zh-CN")
    assert has_cjk(text)
    for heading in ["## 1. 摘要", "## 2. 权限表", "## 3. 判定依据", "## 4. 覆盖边界"]:
        assert heading in text


def test_compliance_mapping_is_embedded_and_follows_locale():
    en = report.render_report(POLICY)
    assert "EU AI Act" in en and "ISO/IEC 42001" in en
    assert "not legal advice" in en
    zh = report.render_report(POLICY, locale="zh-CN")
    assert "不是法律意见" in zh


def test_report_and_policy_land_in_delivery_with_verifiable_manifest(tmp_path):
    from ratchet_service.cli import build_delivery

    policy_path = tmp_path / "policy.json"
    policy_path.write_text(json.dumps(POLICY, ensure_ascii=False), encoding="utf-8")
    out = tmp_path / "delivery"
    out.mkdir()

    build_delivery(out, policy_path=policy_path, note="unit test")

    assert (out / "report.md").is_file()
    assert (out / "policy.json").is_file()
    assert not has_cjk((out / "report.md").read_text(encoding="utf-8"))
    data = json.loads((out / manifest.MANIFEST_NAME).read_text(encoding="utf-8"))
    assert {a["file"] for a in data["artifacts"]} == {"report.md", "policy.json"}
    assert manifest.verify(out)["ok"] is True
