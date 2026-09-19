"""Ratchet 交付物侧。

分两层：

- `verify.py` —— **单文件、只用标准库**。它会被单独发给收货方，
  所以它绝不 import 本包里的任何东西。
- 其余模块（manifest / compliance / cli）—— 出具方用，可以自由依赖。
"""

__version__ = "0.8.0"
