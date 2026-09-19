#!/usr/bin/env python3
# set-edge-domain.sh 的远端执行部分：上传 vhost、nginx -t、失败整批回滚、reload。
#
# 为什么拆成单独文件：脚本里塞一段 heredoc 的 Python，改一处就得整体重排；
# 单独一个文件能用 python3 -m py_compile 检查语法，也方便以后加域名。
#
# 只依赖 paramiko（web01 没开 key 登录）。所有输入走环境变量，见 set-edge-domain.sh。

import os
import posixpath
import sys
import time

try:
    import paramiko
except ImportError:
    sys.exit("错误:需要 paramiko（pip install paramiko）")

HOST = os.environ["EDGE_HOST"]
USER = os.environ["EDGE_USER"]
PASSWORD = os.environ["EDGE_PASSWORD"]
DOMAINS = os.environ["DOMAINS"].split()
WORKDIR = os.environ["WORKDIR"]
VHOST_DIR = os.environ["REMOTE_VHOST_DIR"]
NGINX = os.environ["REMOTE_NGINX"]
NGINX_ARGS = os.environ["REMOTE_NGINX_ARGS"]


def run(client, cmd, check=True):
    """跑一条远程命令；check=True 时非 0 退出码直接终止。"""
    _, out, err = client.exec_command(cmd)
    rc = out.channel.recv_exit_status()
    text = out.read().decode("utf-8", "replace") + err.read().decode("utf-8", "replace")
    if check and rc != 0:
        raise SystemExit("远程命令失败(rc=%d): %s\n%s" % (rc, cmd, text))
    return rc, text


def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=20)
    sftp = client.open_sftp()

    stamp = time.strftime("%Y%m%d-%H%M%S")
    backups = []

    try:
        for domain in DOMAINS:
            remote = posixpath.join(VHOST_DIR, "%s.conf" % domain)
            try:
                sftp.stat(remote)
            except IOError:
                backups.append((remote, None))
                print("==> %s 不存在，新建（无需备份）" % remote)
            else:
                backup = "%s.bak-%s" % (remote, stamp)
                run(client, "cp -p '%s' '%s'" % (remote, backup))
                backups.append((remote, backup))
                print("==> 已备份 %s -> %s" % (remote, backup))

            sftp.put(posixpath.join(WORKDIR, "%s.conf" % domain), remote)
            run(client, "chmod 644 '%s'" % remote)
            print("==> 已写入 %s" % remote)

        rc, text = run(client, "%s -t %s" % (NGINX, NGINX_ARGS), check=False)
        print(text.strip())
        if rc != 0:
            print("!! nginx -t 未通过，整批回滚", file=sys.stderr)
            for remote, backup in backups:
                if backup:
                    run(client, "cp -p '%s' '%s'" % (backup, remote))
                    print("==> 已回滚 %s" % remote, file=sys.stderr)
                else:
                    run(client, "rm -f '%s'" % remote)
                    print("==> 已删除新文件 %s" % remote, file=sys.stderr)
            run(client, "%s -t %s" % (NGINX, NGINX_ARGS), check=False)
            return 1

        run(client, "%s -s reload %s" % (NGINX, NGINX_ARGS))
        print("==> nginx 已 reload")
    finally:
        sftp.close()
        client.close()

    print("")
    print("==> 验收（在本机跑）")
    for domain in DOMAINS:
        print("  curl -s -o /dev/null -w '%%{http_code}\\n' https://%s/" % domain)
    print("  回滚：把上面打印的 .bak-<时间戳> 拷回原路径，再 nginx -t && nginx -s reload")
    return 0


if __name__ == "__main__":
    sys.exit(main())
