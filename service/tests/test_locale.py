"""语言解析：显式参数 > RATCHET_LANG > LC_ALL/LANG > 默认 en-US。

这组用例盯的是一个会被环境悄悄带偏的地方：产物语言若受调用者 shell 的
LC_ALL/LANG 影响，同一份代码在不同机器上打出来的报告语言就不一样。
"""

import importlib

import pytest

import ratchet_service.i18n as i18n


@pytest.fixture
def clean_env(monkeypatch):
    for key in ("RATCHET_LANG", "LC_ALL", "LANG"):
        monkeypatch.delenv(key, raising=False)
    return monkeypatch


def test_defaults_to_english(clean_env):
    assert i18n.resolve_locale() == "en-US"


def test_explicit_beats_everything(clean_env):
    clean_env.setenv("RATCHET_LANG", "zh-CN")
    clean_env.setenv("LANG", "zh_CN.UTF-8")
    assert i18n.resolve_locale("en-US") == "en-US"


def test_ratchet_lang_beats_shell(clean_env):
    clean_env.setenv("RATCHET_LANG", "en-US")
    clean_env.setenv("LANG", "zh_CN.UTF-8")
    assert i18n.resolve_locale() == "en-US"


@pytest.mark.parametrize("value", ["C", "c", "POSIX", "C.UTF-8", "c.utf8", "posix.UTF-8"])
def test_c_locale_is_not_a_language(clean_env, value):
    """LC_ALL=C.UTF-8 是 CI/Docker 的默认值，必须继续往下找，而不是当英文/中文。"""
    clean_env.setenv("LC_ALL", value)
    clean_env.setenv("LANG", "zh_CN.UTF-8")
    assert i18n.resolve_locale() == "zh-CN"


def test_c_locale_with_nothing_else_falls_back_to_english(clean_env):
    clean_env.setenv("LC_ALL", "C.UTF-8")
    assert i18n.resolve_locale() == "en-US"


def test_shell_language_is_respected(clean_env):
    clean_env.setenv("LANG", "zh_CN.UTF-8")
    assert i18n.resolve_locale() == "zh-CN"


def test_module_import_is_side_effect_free():
    """回头看一眼：resolve_locale 是函数，不是在 import 时就把语言定死。"""
    importlib.reload(i18n)
    assert callable(i18n.resolve_locale)
