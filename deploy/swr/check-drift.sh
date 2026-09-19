#!/usr/bin/env bash
# 检查「helm release 记录」与「集群实际运行」是否漂移（镜像 tag / 副本数）。
#
# 为什么需要：同一个字段被两个写者改过（helm upgrade 与 kubectl set image），
# 结果是 release 记着旧版本、集群跑着新版本，一旦有人 helm upgrade/rollback 就静默回退。
# 发布前跑一次，把这种漂移显性化。
#
# 用法（服务器上先 export KUBECONFIG=/etc/kubernetes/admin.conf）：
#   ./deploy/swr/check-drift.sh
#
# 退出码：0=无漂移  1=存在漂移  2=环境不可用
set -euo pipefail

NS="${NAMESPACE:-ratchet}"
RELEASE="${RELEASE:-ratchet}"

command -v helm >/dev/null 2>&1 || { echo "缺少 helm" >&2; exit 2; }
command -v kubectl >/dev/null 2>&1 || { echo "缺少 kubectl" >&2; exit 2; }
kubectl -n "$NS" get deploy >/dev/null 2>&1 || { echo "无法访问命名空间 $NS（检查 KUBECONFIG）" >&2; exit 2; }

live_file="$(mktemp)"
manifest_file="$(mktemp)"
trap 'rm -f "$live_file" "$manifest_file"' EXIT

kubectl -n "$NS" get deploy \
  -o custom-columns='NAME:.metadata.name,IMAGE:.spec.template.spec.containers[0].image,REPLICAS:.spec.replicas' \
  --no-headers > "$live_file"
helm -n "$NS" get manifest "$RELEASE" > "$manifest_file"

# 行级解析，不依赖 yaml 库：服务器上的 python 可能很老（FinHarness 那台是 3.7）
python3 - "$live_file" "$manifest_file" "$NS" <<'PY'
import re
import sys

live_path, manifest_path, ns = sys.argv[1], sys.argv[2], sys.argv[3]

live = {}
for line in open(live_path, encoding="utf-8"):
    parts = line.split()
    if len(parts) >= 3:
        live[parts[0]] = (parts[1], parts[2])

released = {}
kind = None
name = None
in_init = False
for raw in open(manifest_path, encoding="utf-8"):
    line = raw.rstrip("\n")
    if line.startswith("kind: "):
        kind = line.split(": ", 1)[1].strip()
        name = None
        in_init = False
        continue
    if kind != "Deployment":
        continue
    m = re.match(r"^  name: (.+)$", line)
    if m:
        name = m.group(1).strip()
        continue
    if not name:
        continue
    if re.match(r"^      initContainers:\s*$", line):
        in_init = True
        continue
    if re.match(r"^      containers:\s*$", line):
        in_init = False
        continue
    if in_init:
        continue
    m = re.match(r"^\s+image: \"?([^\"\s]+)\"?\s*$", line)
    if m and "image" not in released.setdefault(name, {}):
        released[name]["image"] = m.group(1)
        continue
    m = re.match(r"^  replicas: (\d+)\s*$", line)
    if m:
        released.setdefault(name, {})["replicas"] = m.group(1)

drift = []
for name in sorted(set(live) | set(released)):
    l = live.get(name)
    r = released.get(name, {})
    if l is None:
        drift.append("  %s: release 有、集群无" % name)
        continue
    if not r:
        drift.append("  %s: 集群有、release manifest 无（手工创建？）" % name)
        continue
    if l[0] != r.get("image"):
        drift.append("  %s 镜像:\n      release = %s\n      实际    = %s" % (name, r.get("image"), l[0]))
    if r.get("replicas") and l[1] != r["replicas"]:
        drift.append("  %s 副本数: release = %s 实际 = %s" % (name, r["replicas"], l[1]))

if drift:
    print("发现 helm release 与集群实际状态漂移（namespace=%s）:" % ns)
    print("\n".join(drift))
    print("\n修复方向：以 values 文件为准跑一次 helm upgrade（deploy/swr/deploy-remote.sh），")
    print("不要把 kubectl set image / scale 当作常规发布手段。")
    sys.exit(1)

print("无漂移：namespace=%s 下 %d 个 Deployment 与 helm release 记录一致" % (ns, len(live)))
PY
