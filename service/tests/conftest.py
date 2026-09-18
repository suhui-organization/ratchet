import sys
from pathlib import Path

# 不要求先 pip install -e：把 src 挂进 sys.path，测试开箱即跑。
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))
