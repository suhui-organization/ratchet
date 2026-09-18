"""报告渲染：四节齐全、依据可追、边界写清。"""

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
    "rationale": {
        "filesystem/read_file": "名称命中「read」；观测到 120 次调用",
        "filesystem/write_file": "名称命中「write」；观测到 9 次调用",
        "filesystem/delete_file": "名称命中「delete」；观测到 1 次调用",
        "internal/acme_opaque": "名称与描述都未命中能力词表；未观测到调用（仅清单中存在）",
    },
    "needsReview": ["internal/acme_opaque"],
}


def test_report_has_all_four_sections():
    text = report.render_report(POLICY, generated_at="2026-09-18T00:00:00Z")
    for heading in ["## 1. 摘要", "## 2. 权限表", "## 3. 判定依据", "## 4. 覆盖边界"]:
        assert heading in text, f"缺少 {heading}"


def test_counts_match_policy():
    text = report.render_report(POLICY)
    assert "放行 **1**" in text
    assert "需人工审批 **2**" in text
    assert "拒绝 **1**" in text


def test_needs_review_is_called_out_separately():
    text = report.render_report(POLICY)
    assert "请先核对这一批再用" in text
    assert "待确认（依据不足，需人工判断）" in text


def test_rationale_is_included_for_traceability():
    text = report.render_report(POLICY)
    assert "观测到 120 次调用" in text
    assert "名称命中「delete」" in text


def test_coverage_boundary_is_explicit():
    text = report.render_report(POLICY)
    # 「未观测」不等于「没发生」——这条必须写在报告里，否则会被读成安全结论
    assert "未观测" in text and "不等于" in text


def test_compliance_mapping_is_embedded():
    text = report.render_report(POLICY)
    assert "EU AI Act" in text and "ISO/IEC 42001" in text
    assert "不是法律意见" in text


def test_report_and_policy_land_in_delivery_with_verifiable_manifest(tmp_path):
    from ratchet_service.cli import build_delivery

    policy_path = tmp_path / "policy.json"
    policy_path.write_text(json.dumps(POLICY, ensure_ascii=False), encoding="utf-8")
    out = tmp_path / "delivery"
    out.mkdir()

    build_delivery(out, policy_path=policy_path, note="unit test")

    assert (out / "report.md").is_file()
    assert (out / "policy.json").is_file()
    data = json.loads((out / manifest.MANIFEST_NAME).read_text(encoding="utf-8"))
    files = {a["file"] for a in data["artifacts"]}
    assert files == {"report.md", "policy.json"}
    assert manifest.verify(out)["ok"] is True
