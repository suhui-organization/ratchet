"""CLI 卫生：这条测试是为了消灭一类反复犯的错。

`cli.py` 末尾的 ``if __name__ == "__main__": sys.exit(main())`` 会**立刻**执行 main()，
所以任何写在它后面的顶层 def 在调用时都还不存在 → NameError。
这个错在同一个文件上犯过三次（_parse_exemptions、_asas_diff …）。

约定写成测试，就不靠人记了。
"""

from pathlib import Path


def test_no_top_level_def_after_main_guard():
    source = Path(__file__).resolve().parents[1] / "src/ratchet_service/cli.py"
    lines = source.read_text(encoding="utf-8").split("\n")
    guard = next((i for i, l in enumerate(lines) if l.startswith("if __name__")), None)
    assert guard is not None, "cli.py 必须保留 __main__ guard"
    after = [l for l in lines[guard:] if l.startswith(("def ", "class "))]
    assert not after, f"这些顶层定义在 __main__ guard 之后，运行时会 NameError：{after}"
