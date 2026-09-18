"""合规映射：条款要齐、每条都要写边界、且不能读成"已合规"。"""

from ratchet_service import compliance


def test_covers_the_key_articles():
    text = compliance.render_markdown()
    for expected in ["Art. 11", "Art. 12", "Art. 14", "Art. 15", "Art. 17", "ISO/IEC 42001"]:
        assert expected in text, f"映射缺少 {expected}"


def test_every_row_states_what_it_cannot_prove():
    for row in compliance.MAPPING:
        assert len(row) == 4, row
        cannot = row[3]
        assert len(cannot) > 10, f"「不能证明什么」写得太短，容易读成全覆盖：{row[0]}"


def test_disclaimer_is_present_and_explicit():
    text = compliance.render_markdown()
    assert "不是法律意见" in text
    assert "不代表该要求已经满足" in text
