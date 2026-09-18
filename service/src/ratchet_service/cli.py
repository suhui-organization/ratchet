"""出具方用的命令行：建清单、验交付目录。"""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

import shutil

from . import compliance, manifest, report
from .verify import MESSAGES, render


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="ratchet-report", description="Ratchet 交付物工具")
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_build = sub.add_parser("build", help="生成交付目录：报告 + 策略 + sha256 清单")
    p_build.add_argument("--dir", required=True, help="交付目录")
    p_build.add_argument("--policy", default="", help="策略 JSON（给了就渲染报告并一起交付）")
    p_build.add_argument("--note", default="", help="写进清单的一句话说明")

    p_verify = sub.add_parser("verify", help="校验交付目录")
    p_verify.add_argument("dir")
    p_verify.add_argument("--lang", default="en-US", choices=sorted(MESSAGES))
    p_verify.add_argument("--json", action="store_true")

    p_compliance = sub.add_parser("compliance", help="打印合规条款映射（Markdown）")

    p_bundle = sub.add_parser("bundle", help="把交付目录打成一个可分享的 JSON（收货方拖进网页即可验证）")
    p_bundle.add_argument("--dir", required=True, help="交付目录")
    p_bundle.add_argument("--out", required=True, help="输出的 .json 路径")

    args = parser.parse_args(argv)

    if args.cmd == "build":
        d = Path(args.dir)
        d.mkdir(parents=True, exist_ok=True)
        m = build_delivery(d, policy_path=Path(args.policy) if args.policy else None, note=args.note)
        print(f"已生成交付目录：{d}")
        print(f"  产物 {len(m['artifacts'])} 份 · 生成时间 {m['generatedAt']}")
        for a in m["artifacts"]:
            print(f"    {a['file']}  {a['bytes']} 字节")
        print("  收货方验证：python3 verify.py <目录>（不需要装任何东西）")
        return 0

    if args.cmd == "verify":
        result = manifest.verify(Path(args.dir))
        print(json.dumps(result, ensure_ascii=False, indent=2) if args.json
              else render(result, MESSAGES[args.lang], False))
        return 0 if result["ok"] else 1

    if args.cmd == "compliance":
        print(compliance.render_markdown())
        return 0

    if args.cmd == "bundle":
        path = bundle(Path(args.dir), Path(args.out))
        print(f"已写出分享包 {path}")
        print("  收货方打开验证页，把这个文件拖进去即可——不需要账号、不需要装东西。")
        return 0

    return 2


def build_delivery(directory: Path, *, policy_path: Path | None = None, note: str = "") -> dict:
    """把交付目录做成一份完整的交付物：报告 + 策略 + 清单。

    报告由策略渲染而来（`report.render_report`），所以它天然包含判定依据与
    合规条款映射——这两节是"这份报告为什么可信"的全部依据，不能由使用者
    自己拼。最后统一建 sha256 清单，收货方据此独立校验。
    """
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)

    if policy_path is not None:
        policy = json.loads(Path(policy_path).read_text(encoding="utf-8"))
        target = directory / "policy.json"
        if Path(policy_path).resolve() != target.resolve():
            shutil.copyfile(policy_path, target)
        generated_at = policy.get("generatedAt", "")
        (directory / "report.md").write_text(
            report.render_report(policy, generated_at=generated_at, note=note), encoding="utf-8"
        )

    return manifest.write_manifest(directory, note=note)


def bundle(directory: Path, out: Path) -> Path:
    """把交付目录打成单个 JSON。

    为什么不做成"必须部署一个后端"：交付物要能在一个离线环境里被验证，
    多一个服务就多一个"对方要信任你"的环节。一个文件，拖进页面，就地算哈希。
    """
    data = manifest.build_manifest(directory)
    artifacts = [
        {"file": entry["file"], "content": (directory / entry["file"]).read_text(encoding="utf-8")}
        for entry in data["artifacts"]
    ]
    payload = {
        "format": "ratchet-bundle/v1",
        "generatedAt": data["generatedAt"],
        "manifest": data,
        "artifacts": artifacts,
    }
    out.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return out


if __name__ == "__main__":
    sys.exit(main())
