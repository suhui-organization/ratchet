"""出具方用的命令行：建清单、验交付目录。"""
from __future__ import annotations

import argparse
import json
import os
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
    p_asas.add_argument("--offline", action="store_true",
                        help="不出网取制品哈希（内网/无出网时用）；相关字段如实记为 unknown")
    p_asas.add_argument("--http-budget", type=float, default=0,
                        help="整趟出网的秒数预算（默认 60）；用完即停，剩下的记为 unknown")
    p_asas.add_argument("--delegated-from", default="",
                        help="父 agent id：本凭据的 agent 是替它干活的（ASAS-2.4/2.5）")
    p_asas.add_argument("--delegation-expires", default="",
                        help="委派到期时间（默认取本凭据有效期；不得晚于父凭据）")
    p_asas.add_argument("--parent", action="append", default=[],
                        help="父凭据文件（可重复）。不给而声明了委派 = 未知委派链 = 越权")
    p_asas.add_argument("--containment", default="",
                        help="遏制记录 JSON（控制面 GET /contain 的产物），用于 V9 级联检查")
    p_asas.add_argument("--exempt", action="append", default=[],
                        help="未定版组件的书面豁免：name=YYYY-MM-DD（可重复，ASAS-3.1）")

    # ASAS-3.4：两次凭据的差异。"今天有什么变了"才是有人会看的东西。
    p_diff = sub.add_parser("asas-diff", help="比较两份 ASAS-A 凭据（ASAS-3.4）")
    p_diff.add_argument("old")
    p_diff.add_argument("new")
    p_diff.add_argument("--json", action="store_true")
    p_diff.add_argument("--fail-on", default="high", choices=["critical", "high", "medium", "info"],
                        help="达到该严重度就返回非 0（默认 high），供每日巡检/CI 使用")

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

    if args.cmd == "asas-diff":
        return _asas_diff(args)

    return 2


def _asas(args) -> int:
    """产出 ASAS-A 凭据并自查。返回非 0 = 凭据不通过验证，不该交给收货方。"""
    from . import asas, asas_build
    from . import artifacts

    directory = Path(args.dir)
    directory.mkdir(parents=True, exist_ok=True)
    # 出网的两道闸在 artifacts 里，CLI 只负责把它们翻译成参数：
    # 传感器（无人值守）必须能"一步都不出网"地把凭据交出来。
    if getattr(args, "offline", False):
        os.environ["RATCHET_OFFLINE"] = "1"
    if getattr(args, "http_budget", 0):
        os.environ["RATCHET_HTTP_BUDGET"] = str(args.http_budget)
    artifacts.reset_budget()
    policy = json.loads(Path(args.policy).read_text(encoding="utf-8"))
    inventory = json.loads(Path(args.inventory).read_text(encoding="utf-8")) if args.inventory else None

    # 证据只取"凭据所描述的那些文件"，显式列举而不是遍历目录——
    # 否则 attestation.json 会把自己算进自己的证据里，形成自指。
    evidence = [
        (name, (directory / name).read_bytes())
        for name in ("policy.json", "report.md")
        if (directory / name).is_file()
    ]

    # 遏制记录与委派图是一对输入：只看记录不知道谁是谁的下级，
    # 只看图不知道谁被吊销了（见 asas.rule_containment_cascades）。
    state = _load_containment(args.containment)
    parents = _load_parents(args.parent)
    attestation, report = asas_build.build_and_verify(
        policy,
        org=args.org,
        environment=args.environment or None,
        owner=args.owner,
        inventory=inventory,
        evidence=evidence,
        exemptions=_parse_exemptions(args.exempt),
        artifact_hasher=artifacts.hash_npm_package,
        delegation=_delegation_arg(args, parents),
        parents=parents,
        containment=state[0] if state else None,
        graph=state[1] if state else None,
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
        if artifacts.offline():
            print("  离线模式：未出网取制品哈希（如需要，去掉 --offline 或设 RATCHET_OFFLINE=0）")
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


def _delegation_arg(args, parents: dict[str, dict] | None) -> dict | None:
    """把 --delegated-from / --parent 拼成构建参数。

    父凭据在手时，把它的到期时间一并交给构建器，让默认委派到期取"两者的较早者"：
    否则父子两份凭据相隔几秒生成，子就比父多活几秒，默认产物直接违规（实测踩过）。
    """
    if not args.delegated_from:
        return None
    parent = (parents or {}).get(args.delegated_from) or {}
    bound = str((parent.get("subject") or {}).get("period", {}).get("to") or "")
    return {
        "from": args.delegated_from,
        "expires": args.delegation_expires,
        "parentNotAfter": bound,
    }


def _load_parents(paths) -> dict[str, dict] | None:
    """读父凭据文件 → ``{agent id: 凭据}``。

    一个文件里可能有好几个 agent，全部收进来：验证器按 `delegation.from` 去找。
    没给就是 None（= "没有这份输入"），与"给了一份不含父的凭据"是两件事。
    """
    if not paths:
        return None
    parents: dict[str, dict] = {}
    for path in paths:
        att = json.loads(Path(path).read_text(encoding="utf-8"))
        for agent in att.get("agents") or []:
            if agent.get("id"):
                parents[str(agent["id"])] = att
    return parents


def _load_containment(path: str) -> tuple[list[dict], list[dict] | None] | None:
    """读遏制记录（控制面 GET /contain 的产物）→ ``(记录, 委派图或 None)``。

    服务端返回 ``{"containment": [...], "graph": [...]}``。带 `graph` 的导出是真图，
    V9 因此能判"确实没有下级"；只有记录而没有图时图会退化成残图，
    V9 会如实报未评估（看不见 ≠ 没有）。裸数组也认，免得人手抄一份就报错。
    """
    if not path:
        return None
    payload = json.loads(Path(path).read_text(encoding="utf-8"))
    graph = None
    if isinstance(payload, dict):
        graph = payload.get("graph")
        payload = payload.get("containment") or []
    if not isinstance(payload, list):
        raise SystemExit("错误：--containment 需要 {\"containment\": [...]} 或 [...]")
    return payload, graph


def _asas_diff(args) -> int:
    from . import asas_diff

    old = json.loads(Path(args.old).read_text(encoding="utf-8"))
    new = json.loads(Path(args.new).read_text(encoding="utf-8"))
    changes = asas_diff.diff(old, new)
    summary = asas_diff.summarize(changes)

    if args.json:
        print(json.dumps({"summary": summary, "changes": changes}, ensure_ascii=False, indent=2))
    elif not changes:
        print("无变化——两次凭据一致（这本该是好消息）")
    else:
        print(f"发现 {summary['total']} 处变化：{summary['bySeverity']}")
        for c in changes:
            print(f"  [{c['severity']:8s}] {c['kind']:20s} {c['subject']}")
            print(f"             {c['why']}")

    threshold = {"critical": 0, "high": 1, "medium": 2, "info": 3}[args.fail_on]
    worst = min(({"critical": 0, "high": 1, "medium": 2, "info": 3}[c["severity"]] for c in changes),
                default=99)
    return 1 if worst <= threshold else 0
if __name__ == "__main__":
    sys.exit(main())
