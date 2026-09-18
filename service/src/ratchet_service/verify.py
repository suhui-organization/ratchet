#!/usr/bin/env python3
"""独立验证一份 Ratchet 交付目录——只依赖 Python 标准库。

这个文件是**给收货方**的：客户、外部审计、法务。他们大概率没有 Go 工具链，
也不该为了验一份报告去装软件。所以：

    python3 verify.py <交付目录>

就完事了。它只读文件、不出网、不执行目录里的任何东西。

纪律（不要为了"方便"破坏它）：
  1. 不 import 本仓库的其它模块——它必须能被单独复制出去；
  2. 不引入第三方依赖；
  3. 结论与 `ratchet-report verify` 必须一致。

退出码：0 = 全部一致；1 = 有缺失/被改动/目录不合法。
"""
from __future__ import annotations

import argparse
import hashlib
import json
import sys
from pathlib import Path

MANIFEST_NAME = "manifest.json"

MESSAGES = {
    "en-US": {
        "title": "Ratchet — independent verification",
        "no_manifest": "Not a Ratchet delivery: {path} does not exist",
        "bad_manifest": "manifest.json is not valid JSON: {reason}",
        "empty": "The manifest lists no artifacts — nothing to verify",
        "bad_artifacts": "manifest.json field `artifacts` must be an array",
        "checked": "checked",
        "missing": "MISSING",
        "mismatch": "MODIFIED",
        "pass": "✅ Verification passed: {n} file(s) match manifest.json — "
                "this delivery has not changed since it was generated.",
        "fail": "❌ Verification FAILED: {missing} missing, {mismatch} modified "
                "(out of {checked} checked).",
        "generated": "Generated at: {ts}",
        "hint": "The manifest records each artifact's sha256 at generation time; "
                "this script recomputed them on your machine.",
    },
    "zh-CN": {
        "title": "Ratchet — 独立校验",
        "no_manifest": "这不是一份 Ratchet 交付目录：找不到 {path}",
        "bad_manifest": "manifest.json 不是合法 JSON：{reason}",
        "empty": "清单里没有列出任何产物——没有可校验的内容",
        "bad_artifacts": "manifest.json 的 artifacts 必须是数组",
        "checked": "已校验",
        "missing": "缺失",
        "mismatch": "被改动",
        "pass": "✅ 校验通过：{n} 份产物与清单完全一致——这份交付物自生成以来未被改动。",
        "fail": "❌ 校验失败：{missing} 份缺失、{mismatch} 份被改动（共校验 {checked} 份）。",
        "generated": "生成时间：{ts}",
        "hint": "清单记录的是生成时每份产物的 sha256；本脚本在你的机器上重新计算并比对。",
    },
}


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify_directory(directory: Path) -> dict:
    """返回结构化结果：ok / checked / missing / mismatches（+ 可选 error/generatedAt）。"""
    manifest_path = directory / MANIFEST_NAME
    result: dict = {
        "dir": str(directory),
        "ok": False,
        "checked": 0,
        "mismatches": [],
        "missing": [],
    }
    if not manifest_path.is_file():
        result["errorKey"] = "no_manifest"
        result["errorDetail"] = str(manifest_path)
        return result
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        result["errorKey"] = "bad_manifest"
        result["errorDetail"] = str(exc)
        return result

    artifacts = manifest.get("artifacts")
    if not isinstance(artifacts, list):
        result["errorKey"] = "bad_artifacts"
        return result
    if not artifacts:
        # 空清单不能算"通过"：那会让"什么都没验"变成"验证通过"，
        # 而这一页存在的全部意义就是别让这种事发生。
        result["errorKey"] = "empty"
        return result
    if manifest.get("generatedAt"):
        result["generatedAt"] = manifest["generatedAt"]

    for artifact in artifacts:
        name = artifact.get("file") if isinstance(artifact, dict) else None
        expected = artifact.get("sha256") if isinstance(artifact, dict) else None
        if not isinstance(name, str) or not isinstance(expected, str):
            result["missing"].append(str(name))
            continue
        target = directory / name
        if not target.is_file():
            result["missing"].append(name)
            continue
        actual = sha256_file(target)
        result["checked"] += 1
        if actual != expected:
            result["mismatches"].append(
                {"file": name, "expected": expected, "actual": actual}
            )

    result["ok"] = not result["missing"] and not result["mismatches"]
    return result


def render(result: dict, text: dict, as_json: bool) -> str:
    if as_json:
        return json.dumps(result, ensure_ascii=False, indent=2)
    lines = [text["title"], "=" * len(text["title"]), result["dir"]]
    if result.get("generatedAt"):
        lines.append(text["generated"].format(ts=result["generatedAt"]))
    lines.append("")
    if result.get("errorKey"):
        template = text[result["errorKey"]]
        detail = result.get("errorDetail", "")
        rendered = template.format(path=detail, reason=detail) if detail else template
        lines.append(f"❌ {rendered}")
        return "\n".join(lines)

    for item in result["mismatches"]:
        lines.append(f"  {text['mismatch']:<9} {item['file']}")
        lines.append(f"            expected {item['expected']}")
        lines.append(f"            actual   {item['actual']}")
    for name in result["missing"]:
        lines.append(f"  {text['missing']:<9} {name}")
    if result["checked"]:
        lines.append(f"  ({text['checked']}: {result['checked']})")
    lines.append("")
    if result["ok"]:
        lines.append(text["pass"].format(n=result["checked"]))
    else:
        lines.append(
            text["fail"].format(
                missing=len(result["missing"]),
                mismatch=len(result["mismatches"]),
                checked=result["checked"],
            )
        )
    lines.append("")
    lines.append(text["hint"])
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Verify a Ratchet delivery directory (standard library only)."
    )
    parser.add_argument("directory", help="交付目录（含 manifest.json）")
    parser.add_argument(
        "--lang", default="en-US", choices=sorted(MESSAGES),
        help="输出语言（默认 en-US：这份工具是发给收货方的）",
    )
    parser.add_argument("--json", action="store_true", help="输出机器可读结果")
    args = parser.parse_args(argv)

    directory = Path(args.directory).expanduser()
    result = verify_directory(directory)
    print(render(result, MESSAGES[args.lang], args.json))
    return 0 if result["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
