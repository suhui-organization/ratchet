import sys
from pathlib import Path

import pytest

# 不要求先 pip install -e：把 src 挂进 sys.path，测试开箱即跑。
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))


@pytest.fixture(autouse=True)
def _neutral_locale(monkeypatch):
    """把语言环境按到中性，让"默认英文"这类断言真的在测默认值。

    产物语言会读 RATCHET_LANG / LC_ALL / LANG（见 i18n.resolve_locale）。
    开发机常见的 LANG=zh_CN.UTF-8 会让整批"默认英文"用例变红——那不是代码坏了，
    是用例把宿主环境当成了被测对象。需要特定语言的用例自己设这两个变量。
    """
    monkeypatch.delenv("RATCHET_LANG", raising=False)
    monkeypatch.delenv("LANG", raising=False)
    monkeypatch.setenv("LC_ALL", "C")
