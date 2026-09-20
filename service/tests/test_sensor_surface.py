"""传感器脚本与 Helm chart 的表面测试。

这一组用例的存在理由都是**已经发生过的真实故障**，不是假想：

1. `sensor.sh` 里写了 `--org`，而 `ratchet deliver` 只有 `--client`
   → 容器里 exit code 2，采集全灭（本机 kind 实测）。脚本和 CLI 之间没有编译器，
   那就用测试把"脚本里用的 flag 必须真的存在"钉死。
2. `--introspect` 会执行配置里的命令。脚本必须**默认不执行**，只有在
   `ASAS_INTROSPECT=1` 时才走到那一步——这条线要靠测试守住，不能靠注释。
3. 传感器在集群里默认必须**不存在**（sensor.enabled=false 时不该渲染出 CronJob）。
4. 镜像要么用 digest 部署（生产），要么就别假装自己在钉版本。
"""

import re
import shutil
import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[2]
SENSOR_SH = REPO / "deploy/sensor/sensor.sh"
MAIN_GO = REPO / "cmd/ratchet/main.go"
CHART = REPO / "deploy/k8s/helm/asas"


def _set_dashdash_lines(path: Path) -> str:
    """只收集 `set -- ...` 拼接出来的参数（含续行）。

    整份脚本里还有 curl 的参数（`--data-binary` 之类），它们不属于 ratchet CLI，
    混进来只会让用例变成噪声。
    """
    lines = path.read_text(encoding="utf-8").split("\n")
    collected: list[str] = []
    i = 0
    while i < len(lines):
        if lines[i].lstrip().startswith("set --"):
            while i < len(lines):
                collected.append(lines[i])
                if not lines[i].rstrip().endswith("\\"):
                    break
                i += 1
        i += 1
    return "\n".join(collected)


def _deliver_flags() -> set[str]:
    """从 cmdDeliver 的源码里读出它真正注册了哪些 flag。"""
    src = MAIN_GO.read_text(encoding="utf-8")
    body = src.split("func cmdDeliver(")[1].split("\n}")[0]
    return {"--" + name for name in re.findall(r'fs\.(?:String|Bool|Int)\("([a-zA-Z0-9-]+)"', body)}


def test_sensor_script_only_uses_flags_deliver_actually_has():
    used = set(re.findall(r"--[a-z][a-z0-9-]*", _set_dashdash_lines(SENSOR_SH)))
    # --introspect / --allow-exec 属于 `scan`，由下面的用例单独管它的门。
    deliver_used = used - {"--introspect", "--allow-exec"}
    known = _deliver_flags()
    assert known, "没能从 main.go 里解析出 deliver 的 flag，测试本身失效了"
    unknown = sorted(deliver_used - known)
    assert not unknown, f"sensor.sh 用了 deliver 不认识的 flag（跑起来就是 exit 2）：{unknown}"


def test_sensor_never_executes_unless_explicitly_allowed():
    lines = SENSOR_SH.read_text(encoding="utf-8").split("\n")
    guard = next(
        (i for i, l in enumerate(lines) if "ASAS_INTROSPECT" in l and l.lstrip().startswith("if ")),
        None,
    )
    assert guard is not None, "找不到 ASAS_INTROSPECT 的开关分支"
    first = next((i for i, l in enumerate(lines) if "--introspect" in l and not l.lstrip().startswith("#")), None)
    assert first is not None, "脚本里应当保留 introspect 的能力（受开关控制）"
    assert first > guard, "--introspect 出现在开关之前，等于默认就会执行配置里的命令"


def test_sensor_does_not_leave_workdir_to_chance():
    """不给 --workdir 就会扫到随机 cwd，同一个 .mcp.json 会被算两遍。"""
    text = SENSOR_SH.read_text(encoding="utf-8")
    assert '--workdir "$ASAS_PROJECT"' in text
    assert re.search(r'ASAS_PROJECT="\$\{ASAS_PROJECT:-/tmp\}"', text)


def test_sensor_judges_by_http_status_not_by_pipeline_exit_code():
    """`curl ... | head` 的退出码是 head 的——控制面 422 时脚本照样"成功"。

    本机 kind 实测过这条：Pod 报 Completed，而凭据根本没通过 ASAS-V。
    """
    text = SENSOR_SH.read_text(encoding="utf-8")
    code = "\n".join(l for l in text.split("\n") if not l.lstrip().startswith("#"))
    assert not re.search(r"curl[^\n]*\|\s*head", code), "管道会吞掉 curl 的退出码"
    assert text.count("%{http_code}") >= 2, "上报与验证两步都要显式取状态码"
    # 不合格 = 非 0 退出：让调度器和值夜班的人看得见，而不是安静地"跑完了"。
    assert re.search(r"422\)[\s\S]{0,300}exit 1", text)


def test_sensor_uploads_only_the_evidence_the_manifest_lists():
    """上传证据是为了让控制面能自己重算哈希（V1 不再是"未评估"）。

    只传 manifest 里列过的文件：整个交付目录倒出去，等于把"凭据声明覆盖了什么"
    和"实际给了什么"变成两件事——那正是这份标准要防的。
    """
    text = SENSOR_SH.read_text(encoding="utf-8")
    assert "/evidence" in text
    assert "manifest" in text and 'item.get("file")' in text
    # 上传失败必须非 0 退出：静默丢证据 = 控制面永远算不了哈希，而没人知道为什么。
    assert re.search(r"except urllib\.error\.HTTPError[\s\S]{0,200}SystemExit\(1\)", text)


def test_sensor_finds_call_records_inside_the_scanned_home():
    """调用记录在**被扫描的家目录**里，不是容器自己的 HOME。

    指错地方的话事件流恒为空，"沉默可被审计"就永远停在未评估——
    而且看起来一切正常，这才是最坏的情况。
    """
    text = SENSOR_SH.read_text(encoding="utf-8")
    assert '--calls "$SCAN_HOME/.ratchet/calls.jsonl"' in text
    assert "[ -f \"$SCAN_HOME/.ratchet/calls.jsonl\" ]" in text
    # 没有记录时要说清是"没接 hook"，而不是静默跳过
    assert "没有调用记录" in text


def test_sensor_treats_a_break_as_a_failure_not_a_warning():
    text = SENSOR_SH.read_text(encoding="utf-8")
    assert "/events" in text
    assert re.search(r'result\.get\("break"\)[\s\S]{0,200}SystemExit\(1\)', text)


requires_helm = pytest.mark.skipif(shutil.which("helm") is None, reason="本机没有 helm")


def _render(*setargs: str) -> str:
    cmd = ["helm", "template", "asas", str(CHART), "-n", "asas", *setargs]
    out = subprocess.run(cmd, capture_output=True, text=True, check=False)
    assert out.returncode == 0, out.stderr
    return out.stdout


@requires_helm
def test_sensor_is_absent_by_default():
    rendered = _render()
    assert "kind: CronJob" not in rendered, "默认值不该在客户集群里凭空起一个定时采集"
    assert "asas-sensor" not in rendered


@requires_helm
def test_enabling_sensor_renders_a_readonly_pinned_cronjob():
    digest = "sha256:" + "ab" * 32
    rendered = _render("--set", "sensor.enabled=true", "--set", f"sensor.image.digest={digest}")
    assert "kind: CronJob" in rendered
    assert f"asas-sensor@{digest}" in rendered, "给了 digest 就必须用 repo@digest，而不是 tag"
    assert "asas-sensor:0.1.0" not in rendered
    # 宿主机家目录只读挂载：采集只需要读，写权限是不必要的风险面。
    assert re.search(r"mountPath: /scan[\s\S]{0,80}readOnly: true", rendered)
    assert "path: /home" in rendered
    # 默认不能执行配置里的命令。
    assert "ASAS_INTROSPECT" not in rendered


@requires_helm
def test_allow_exec_is_the_only_way_in():
    rendered = _render("--set", "sensor.enabled=true", "--set", "sensor.allowExec=true")
    assert "name: ASAS_INTROSPECT" in rendered
    assert re.search(r'name: ASAS_INTROSPECT\s+value: "1"', rendered)


@requires_helm
def test_persistence_can_actually_be_turned_off():
    """values 里有 api.persistence.enabled 这个开关，此前模板从不读它——开关是假的。"""
    rendered = _render("--set", "api.persistence.enabled=false")
    assert "PersistentVolumeClaim" not in rendered
    assert "emptyDir: {}" in rendered


@requires_helm
def test_comma_spliced_values_fail_loudly_instead_of_half_applying():
    """`--set sensor.exempt="a=1,b=2"` 会被 Helm 拆成两个赋值。

    实测结果：渲染成功、部署成功、豁免少了 11 条。这种"看起来配好了"的失败
    必须变成渲染期的报错——值守卫就是干这个的。
    """
    out = subprocess.run(
        ["helm", "template", "asas", str(CHART), "-n", "asas",
         "--set", "sensor.enabled=true", "--set", "sensor.exempt=a=1,b=2"],
        capture_output=True, text=True, check=False,
    )
    assert out.returncode != 0, "被截断的值不该安静地渲染过去"
    assert "未知的顶层值" in out.stderr


@requires_helm
def test_image_refs_in_values_files_actually_reach_the_templates():
    """values 里写了的镜像引用，必须真的出现在渲染结果里。

    实测踩过：把远程的 sensor tag 写在 `image.sensor` 下，而模板读的是
    `sensor.image`——渲染成功、部署成功、用的是一个占位 tag。
    """
    remote = (CHART / "values-remote.yaml").read_text(encoding="utf-8")
    digests = re.findall(r'digest:\s*"([^"]+)"', remote)
    tags = re.findall(r'tag:\s*"([^"]+)"', remote)
    assert digests and tags, "values-remote.yaml 里应当同时有 tag 与 digest"

    # 有 digest 时用 repo@digest：tag 不该出现（同一个 tag 可以被推成另一个镜像）。
    pinned = _render("-f", str(CHART / "values-remote.yaml"), "--set", "sensor.enabled=true")
    for ref in digests:
        assert ref in pinned, f"values 里的 digest 没进模板：{ref}"

    # 去掉 digest 才能验证 tag 那条路径也被真的读过（这才是踩过的那个坑）。
    by_tag = _render("-f", str(CHART / "values-remote.yaml"),
                     "--set", "sensor.enabled=true", "--set", "sensor.image.digest=")
    for ref in tags:
        assert ref in by_tag, f"values 里的 tag 没进模板：{ref}"
