"""出具方用的命令行：建清单、验交付目录。"""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

import shutil

from . import compliance, manifest, report
from .i18n import resolve_locale
from .verify import MESSAGES, render


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="ratchet-report", description="Ratchet 交付物工具")
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_build = sub.add_parser("build", help="生成交付目录：报告 + 策略 + sha256 清单")
    p_build.add_argument("--dir", required=True, help="交付目录")
    p_build.add_argument("--policy", default="", help="策略 JSON（给了就渲染报告并一起交付）")
    p_build.add_argument("--bundle", default="", help="同时产出分享包 JSON（收货方拖进验证页即可）")
    p_build.add_argument("--lang", default="", help="产物语言：en-US（默认）/ zh-CN；也可用 RATCHET_LANG")
    p_build.add_argument("--note", default="", help="写进清单的一句话说明")

    p_verify = sub.add_parser("verify", help="校验交付目录")
    p_verify.add_argument("dir")
    p_verify.add_argument("--lang", default="en-US", choices=sorted(MESSAGES))
    p_verify.add_argument("--json", action="store_true")

    p_compliance = sub.add_parser("compliance", help="打印合规条款映射（Markdown）")
    p_compliance.add_argument("--lang", default="", help="en-US（默认）/ zh-CN")

    p_bundle = sub.add_parser("bundle", help="把交付目录打成一个可分享的 JSON（收货方拖进网页即可验证）")
    p_bundle.add_argument("--dir", required=True, help="交付目录")
    p_bundle.add_argument("--out", required=True, help="输出的 .json 路径")

    # ASAS-A 凭据：产出并**当场自查**。自查不过就返回非 0——自己产的凭据先过自己的校验，
    # 不过就不算交付（见 docs/DEVELOPMENT-PLAN.md 的 P0/T2）。
    p_asas = sub.add_parser("asas", help="产出 ASAS-A 凭据并立即验证")
    p_asas.add_argument("--policy", required=True, help="策略 JSON（policy draft 的产物）")
    p_asas.add_argument("--dir", required=True, help="交付目录（凭据写到 <dir>/attestation.json）")
    p_asas.add_argument("--org", required=True, help="凭据主体：组织名")
    p_asas.add_argument("--environment", default="", help="环境名（可选）")
    p_asas.add_argument("--owner", default="unassigned", help="agent 的具名责任人")
    p_asas.add_argument("--inventory", default="", help="scan --introspect 的产物（给了才知道工具面）")
    p_asas.add_argument("--json", action="store_true", help="以 JSON 打印验证报告")
    p_asas.add_argument("--exempt", action="append", default=[],
                        help="未定版组件的书面豁免：name=YYYY-MM-DD（可重复，ASAS-3.1）")

    args = parser.parse_args(argv)

    if args.cmd == "build":
        d = Path(args.dir)
        d.mkdir(parents=True, exist_ok=True)
        m = build_delivery(d, policy_path=Path(args.policy) if args.policy else None,
                           note=args.note, lang=args.lang)
        print(f"已生成交付目录：{d}")
        print(f"  产物 {len(m['artifacts'])} 份 · 生成时间 {m['generatedAt']}")
        for a in m["artifacts"]:
            print(f"    {a['file']}  {a['bytes']} 字节")
        print("  收货方验证：python3 verify.py <目录>（不需要装任何东西）")
        if args.bundle:
            path = bundle(d, Path(args.bundle))
            print(f"  分享包：{path}")
            print("    收货方把它拖进验证页即可——不需要账号、不需要装东西、不需要服务端。")
        return 0

    if args.cmd == "verify":
        result = manifest.verify(Path(args.dir))
        print(json.dumps(result, ensure_ascii=False, indent=2) if args.json
              else render(result, MESSAGES[args.lang], False))
        return 0 if result["ok"] else 1

    if args.cmd == "compliance":
        print(compliance.render_markdown(args.lang))
        return 0

    if args.cmd == "bundle":
        path = bundle(Path(args.dir), Path(args.out))
        print(f"已写出分享包 {path}")
        print("  收货方打开验证页，把这个文件拖进去即可——不需要账号、不需要装东西。")
        return 0

    if args.cmd == "asas":
        return _asas(args)

    return 2


def _asas(args) -> int:
    """产出 ASAS-A 凭据并自查。返回非 0 = 凭据不通过验证，不该交给收货方。"""
    from . import asas, asas_build

    directory = Path(args.dir)
    directory.mkdir(parents=True, exist_ok=True)
    policy = json.loads(Path(args.policy).read_text(encoding="utf-8"))
    inventory = json.loads(Path(args.inventory).read_text(encoding="utf-8")) if args.inventory else None

    # 证据只取"凭据所描述的那些文件"，显式列举而不是遍历目录——
    # 否则 attestation.json 会把自己算进自己的证据里，形成自指。
    evidence = [
        (name, (directory / name).read_bytes())
        for name in ("policy.json", "report.md")
        if (directory / name).is_file()
    ]

    attestation, report = asas_build.build_and_verify(
        policy,
        org=args.org,
        environment=args.environment or None,
        owner=args.owner,
        inventory=inventory,
        evidence=evidence,
        exemptions=_parse_exemptions(args.exempt),
    )
    out = asas_build.write(attestation, directory / "attestation.json")

    if args.json:
        print(json.dumps(report.as_dict(), ensure_ascii=False, indent=2))
    else:
        print(f"ASAS-A 凭据：{out}")
        print(f"  已评估规则 {len(report.evaluated)} 条 · 未评估 {len(report.not_evaluated)} 条")
        for result in report.results:
            mark = {"pass": "✅", "fail": "❌", "not_evaluated": "—"}[result.status]
            print(f"  {mark} {result.rule}" + (f"  {result.details[0]}" if result.details else ""))
        print(f"  unknown 声明 {len(attestation['unknown'])} 条（拿不到的事实，不填假值）")
    return 0 if report.ok else 1


def build_delivery(
    directory: Path,
    *,
    policy_path: Path | None = None,
    note: str = "",
    lang: str = "",
) -> dict:
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
        code = _locale_code(resolve_locale(lang))
        (directory / "report.md").write_text(
            report.render_report(policy, generated_at=generated_at, note=note, locale=code),
            encoding="utf-8",
        )

    return manifest.write_manifest(directory, note=note)


def _locale_code(locale: str) -> str:
    """i18n 的 resolve_locale 返回 'en-US'/'zh-CN'，两者同名，这里只是留个转换点。"""
    return locale


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


def _parse_exemptions(items) -> dict:
    """把 ``name=YYYY-MM-DD`` 解析成 ``{name: date}``。格式不对就直接报错——
    豁免是"明知不合规而接受"的东西，不接受含糊输入。"""
    out = {}
    for item in items or []:
        name, _, when = str(item).partition("=")
        name, when = name.strip(), when.strip()
        if not name or not when:
            raise SystemExit(f"错误：--exempt 需要 name=YYYY-MM-DD 形式，收到 {item!r}")
        out[name] = when
    return out


if __name__ == "__main__":
    sys.exit(main())
