#!/bin/sh
# ASAS 传感器：采集本机 agent 的元数据 → 出凭据 → 上报控制面。
#
# **它该跑在哪里**：跑在**有 agent 的那台机器上**（开发者的笔记本 / CI runner），
# 不是跑在集群里。原因很直接——集群里的 Pod 看不到你笔记本上的 harness 配置，
# 而"agent 到底能碰什么"的答案只存在于那台机器的配置文件里。
# chart 里的定时任务是给"受管机器本身就是集群节点"的场景用的。
#
# 只上报元数据：工具名、版本、制品哈希、判定与依据。**参数值永远不出这台机器**
# （凭据结构里没有放参数值的地方，由 schema 保证）。
#
# 环境变量：
#   ASAS_API        控制面地址，例如 http://asas-api.asas.svc:8080
#   ASAS_ORG        凭据主体（组织名），传给 deliver 的 --client
#   ASAS_AGENT      agent 名（**必须是稳定的**：跨次比对链摘要用它当键）
#   SCAN_HOME       要扫描的 HOME（容器里通常是挂进来的 /scan）
#   ASAS_PROJECT    项目级配置目录（可选；不给就扫一个空目录，见下）
#   ASAS_EXEMPT     书面豁免，形如 name=YYYY-MM-DD,...（可选）
#   ASAS_INTROSPECT 设为 1 才加 --introspect（**默认不加**，见下）
#   ASAS_OFFLINE    设为 1 就不出网取制品哈希（内网/无出网时必须开）
#   ASAS_HTTP_BUDGET 出网总预算秒数（默认 60；用完即停，剩下的如实记 unknown）
set -eu

ASAS_API="${ASAS_API:-http://asas-api.asas.svc:8080}"
ASAS_ORG="${ASAS_ORG:-unknown-org}"
SCAN_HOME="${SCAN_HOME:-/scan}"
OUT="${OUT:-/data/sensor}"
# deliver 的 --workdir 是"项目级配置"（./.mcp.json、./.vscode/mcp.json 这类）。
# 传感器没有"当前项目"这个概念，所以**不给**它一个随机的 cwd——否则同一个 .mcp.json
# 会既被 HOME 扫描到、又被 cwd 扫描到，凭据里凭空多出一份重复的 server。
# 没指定就指向一个空目录，让 HOME 扫描的结果就是全部。
ASAS_PROJECT="${ASAS_PROJECT:-/tmp}"
# 出网必须有界。传感器跑在没人看着的机器上：卡在 SYN_SENT 上等二十分钟，
# 结论还是 unknown，那不如一开始就说"没出网"。预算由 artifacts 强制执行。
ASAS_HTTP_BUDGET="${ASAS_HTTP_BUDGET:-60}"
export RATCHET_HTTP_BUDGET="$ASAS_HTTP_BUDGET"

echo "==> 采集 $SCAN_HOME（只读；不执行任何 server）"
mkdir -p "$OUT"

# 参数用 set -- 拼，而不是 `${VAR:+--flag "$VAR"}`——POSIX sh 里后者的引号是**字面量**，
# 会把一对引号当成值的一部分传进去（豁免名字里多一对引号，解析就找不到条目）。
set -- --out "$OUT" --home "$SCAN_HOME" --client "$ASAS_ORG" --workdir "$ASAS_PROJECT" \
  --report-cmd "python3 -m ratchet_service.cli"
# 调用记录就在被扫描的那棵家目录里（hook 写在 <HOME>/.ratchet/calls.jsonl）。
# 不指它的话，deliver 会去找容器自己的 HOME，事件流永远是空的——
# 而"沉默可被审计"这一条就永远停在"未评估"。
if [ -f "$SCAN_HOME/.ratchet/calls.jsonl" ]; then
  set -- "$@" --calls "$SCAN_HOME/.ratchet/calls.jsonl"
else
  echo "==> 没有调用记录（$SCAN_HOME/.ratchet/calls.jsonl）：事件流会是空的"
  echo "    hook 没接的话这很正常——凭据里会把这条规则如实记为'未评估'。"
fi
# agent 名必须是稳定的：默认取宿主节点名（chart 从 spec.nodeName 注入）。
# 用容器主机名的话每次跑都不一样，跨次比对就永远比不出东西来。
if [ -n "${ASAS_AGENT:-}" ]; then
  set -- "$@" --agent "$ASAS_AGENT"
fi
if [ -n "${ASAS_EXEMPT:-}" ]; then
  set -- "$@" --exempt "$ASAS_EXEMPT"
fi
# 离线开关用环境变量而不是 --offline 参数：出网取哈希发生在 Python 出证那一步
# （deliver 只是把 --report-cmd 拉起来），所以闸门开在进程环境里，不经过 deliver 的解析。
if [ "${ASAS_OFFLINE:-0}" = "1" ]; then
  export RATCHET_OFFLINE=1
fi

# 这里**故意默认不加 --introspect**：只读扫描永远不执行配置里的命令。
# 要连上 server 取工具名，必须显式设 ASAS_INTROSPECT=1——只有那一步才会执行
# 配置里写的命令（几乎总是 `npx -y <包>`），也正是本工具劝别人别做的事。
if [ "${ASAS_INTROSPECT:-0}" = "1" ]; then
  echo "==> ⚠️  已显式允许执行：会启动配置里的每个 MCP server"
  set -- "$@" --introspect --allow-exec
else
  echo "==> 跳过 introspect（默认）：工具名将记为 unknown，而不是猜一个"
fi

ratchet deliver "$@"

ATT="$OUT/delivery/attestation.json"
echo "==> 上报 $ATT → $ASAS_API"
# 状态码必须显式取出来判断。**不能靠 `curl | head` 的退出码**：管道的退出码是 head 的，
# 控制面返回 422 时脚本照样"成功"结束——本机实测踩过，Pod 报 Completed，凭据其实没过。
put_code=$(curl -sS -o "$OUT/attest-response.json" -w '%{http_code}' \
  -X POST "$ASAS_API/attestations" -H 'Content-Type: application/json' \
  --data-binary @"$ATT" || echo 000)
if [ "$put_code" != "201" ]; then
  echo "❌ 控制面没有收下凭据（HTTP $put_code）："
  cat "$OUT/attest-response.json" 2>/dev/null || true
  exit 1
fi
echo "   已入账：$(cat "$OUT/attest-response.json")"

# 证据上传：控制面只有拿到**原始字节**才能自己重算哈希（V1 才不是"未评估"）。
# 这一步只传凭据 manifest 里列过的文件，不把整个目录倒出去。
echo "==> 上传证据文件（让控制面能自己重算哈希）"
python3 - "$OUT" "$ASAS_API" <<'PY'
import base64, json, os, sys, urllib.error, urllib.request

out, api = sys.argv[1], sys.argv[2]
delivery = os.path.join(out, "delivery")
att = json.load(open(os.path.join(delivery, "attestation.json"), encoding="utf-8"))
ident = json.load(open(os.path.join(out, "attest-response.json"), encoding="utf-8"))["id"]

files = {}
for item in (att.get("manifest") or {}).get("evidence") or []:
    name = item.get("file") or ""
    path = os.path.join(delivery, name)
    if name and os.path.isfile(path):
        files[name] = base64.b64encode(open(path, "rb").read()).decode()

if not files:
    # 没有证据可传不是失败：凭据里已经声明了它覆盖不到什么（ASAS-6.3）。
    print("   没有可上传的证据文件（凭据里的 hashMatch 会保持未评估）")
    raise SystemExit(0)

body = json.dumps({"attestationId": ident, "files": files}).encode()
req = urllib.request.Request(api + "/evidence", data=body,
                             headers={"Content-Type": "application/json"}, method="POST")
try:
    with urllib.request.urlopen(req, timeout=30) as resp:
        print("   " + resp.read().decode())
except urllib.error.HTTPError as exc:
    print(f"❌ 证据上传失败（HTTP {exc.code}）：{exc.read().decode()[:200]}")
    raise SystemExit(1)
PY

# 事件流：让控制面能判"这段时间的日志有没有被动过"（ASAS-5.3/6.6）。
# 上传时控制面会拿它跟上次的摘要逐条比——改写过的日志在这里露馅。
if [ -f "$OUT/delivery/events.jsonl" ]; then
  echo "==> 上传事件流（控制面会与上次的摘要逐条比对）"
  python3 - "$OUT" "$ASAS_API" <<'PY'
import json, os, sys, urllib.error, urllib.request

out, api = sys.argv[1], sys.argv[2]
path = os.path.join(out, "delivery", "events.jsonl")
events = [json.loads(line) for line in open(path, encoding="utf-8") if line.strip()]
ident = json.load(open(os.path.join(out, "attest-response.json"), encoding="utf-8"))["id"]
body = json.dumps({"attestationId": ident, "events": events}).encode()
req = urllib.request.Request(api + "/events", data=body,
                             headers={"Content-Type": "application/json"}, method="POST")
try:
    with urllib.request.urlopen(req, timeout=60) as resp:
        result = json.loads(resp.read())
except urllib.error.HTTPError as exc:
    print(f"❌ 事件流上传失败（HTTP {exc.code}）：{exc.read().decode()[:200]}")
    raise SystemExit(1)
print(f"   {result['events']} 条事件 · 摘要 {result['head'][:12]}…")
if result.get("break"):
    # 断流是**事件**，不是警告：它必须以非 0 退出，让人看见。
    print(f"   ❌ 检出断流：{result['break']['reason']}")
    raise SystemExit(1)
PY
fi

# 顺带把验证也跑一遍：控制面与本地必须一致，不一致就该立刻知道。
echo "==> 控制面验证（与本地自查是同一份规则）"
verify_code=$(curl -sS -o "$OUT/verify.json" -w '%{http_code}' \
  -X POST "$ASAS_API/verify" -H 'Content-Type: application/json' \
  --data-binary @"$ATT" || echo 000)
case "$verify_code" in
  200)
    echo "   ✅ 通过：控制面与本地自查一致"
    ;;
  422)
    # 凭据已经入账（上面那一步），但**它没通过自己的规则**。
    # 这种情况必须以非 0 退出：让调度器、让看板、让值夜班的人看见。
    # 悄悄结束才是真的坏——本工具的主张就是"不过自查不算交付"。
    echo "   ❌ 不通过：凭据没过 ASAS-V，交付不算完成（原因见下）"
    cat "$OUT/verify.json"
    exit 1
    ;;
  000|"")
    echo "   ❌ 连不上控制面 $ASAS_API——凭据仍在 $ATT，未入账"
    exit 1
    ;;
  *)
    echo "   ❌ 控制面返回 HTTP $verify_code"
    cat "$OUT/verify.json" 2>/dev/null || true
    exit 1
    ;;
esac
