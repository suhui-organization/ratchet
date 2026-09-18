"""交付目录的 sha256 清单。

清单是"收货方能自己验"的前提：没有它，对方只能相信出具方的说法。
注意 `manifest.json` **不把自己**列进去——那是自引用，算不出来。
"""
from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path

from .verify import MANIFEST_NAME, sha256_file, verify_directory

FORMAT = "ratchet-manifest/v1"


def build_manifest(directory: Path, *, note: str = "") -> dict:
    """为目录里除 manifest 自身以外的所有普通文件建立清单（按路径排序，保证确定性）。"""
    directory = Path(directory)
    if not directory.is_dir():
        raise NotADirectoryError(f"{directory} 不是目录")

    entries = []
    for path in sorted(directory.rglob("*")):
        if not path.is_file():
            continue
        rel = path.relative_to(directory).as_posix()
        if rel == MANIFEST_NAME:
            continue
        entries.append(
            {
                "file": rel,
                "sha256": sha256_file(path),
                "bytes": path.stat().st_size,
            }
        )

    manifest = {
        "format": FORMAT,
        "generatedAt": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "artifacts": entries,
    }
    if note:
        manifest["note"] = note
    return manifest


def write_manifest(directory: Path, *, note: str = "") -> dict:
    """建立并落盘 manifest.json，返回清单内容。"""
    directory = Path(directory)
    manifest = build_manifest(directory, note=note)
    (directory / MANIFEST_NAME).write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    return manifest


def verify(directory: Path) -> dict:
    """与独立验证器同一份实现——两边结论必须一致，所以共用而不是各写一遍。"""
    return verify_directory(Path(directory))
