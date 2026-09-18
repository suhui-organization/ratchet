"""合规映射：条款齐、每条都写边界、语言正确。"""

from ratchet_service import compliance


def has_cjk(text: str) -> bool:
    return any("\u4e00" <= ch <= "\u9fff" for ch in text)


def test_covers_the_key_articles_in_english_by_default():
    text = compliance.render_markdown()
    assert not has_cjk(text), "英文映射里出现了中文"
    for expected in ["Art. 11", "Art. 12", "Art. 14", "Art. 15", "Art. 17", "ISO/IEC 42001"]:
        assert expected in text, f"映射缺少 {expected}"


def test_every_row_states_what_it_cannot_prove():
    for row in compliance.rows():
        assert len(row) == 4, row
        assert len(row[3]) > 20, f"「不能证明什么」写得太短，容易读成全覆盖：{row[0]}"


def test_disclaimer_is_explicit_in_both_languages():
    en = compliance.render_markdown()
    assert "not legal advice" in en
    assert "does not mean the provision is met" in en
    zh = compliance.render_markdown("zh-CN")
    assert "不是法律意见" in zh
    assert "不代表该要求已经满足" in zh


def test_same_rows_in_both_languages():
    """中英两份必须是同一批条款——不能各写一版，否则给出的承诺会不一致。"""
    assert len(compliance.rows()) == len(compliance.rows("zh-CN"))
    for en, zh in zip(compliance.rows(), compliance.rows("zh-CN")):
        assert en[0] != zh[0], "同一行的中英标题不该相同"
        # 条款标识（Art. N / ISO）在两种语言里必须一致，否则对不上审计方的编号
        for marker in ("Art.", "ISO/IEC"):
            if marker in en[0]:
                assert marker in zh[0], f"{en[0]} 与 {zh[0]} 的条款编号对不上"
