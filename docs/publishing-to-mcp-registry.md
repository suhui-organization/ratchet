# 发布到官方 MCP Registry

官方 Registry 是**唯一会级联的目录**：发布一次自动同步到 Smithery、PulseMCP，
并进入 GitHub MCP Registry。

## 前提：镜像必须公开可拉取

Registry 要求"存在公开可取用的制品"。我们用 OCI（GHCR），
但 **GitHub 建的 package 默认私有**，即使仓库公开。

```bash
# 不带任何凭据验证（200 = 公开；401/403 = 还是私有，发布会被拒）
curl -s -o /dev/null -w '%{http_code}\n' \
  -H 'Accept: application/vnd.oci.image.index.v1+json' \
  https://ghcr.io/v2/suhui-organization/ratchet/manifests/latest
```

改成公开（两次点击，只能由账号持有人做）：

1. 打开 https://github.com/orgs/suhui-organization/packages/container/ratchet/settings
2. 页面底部 **Danger Zone → Change visibility → Public**

> 用 API 改需要 `admin:packages` 权限；现有仓库 token（`repo, workflow`）不够。

## 发布

```bash
mcp-publisher validate server.json    # 只校验
mcp-publisher publish                 # 发布（需 GitHub 归属验证）
```

`server.json` 已在仓库根目录，`name` 用 reverse-DNS：
`io.github.suhui-organization/ratchet`——前缀必须与 GitHub 组织一致。

## 版本对齐（别漏）

`server.json` 里 `version` 与 `packages[].version` 必须与**实际发布的镜像 tag** 一致。
现在都是 `0.10.0`。发新版的顺序：

1. 改 `cmd/ratchet/main.go` 的版本号
2. 打 tag 推上去 → CI 编四平台 + 推镜像
3. 改 `server.json` 两处版本号
4. `mcp-publisher publish`

> 第 3 步漏掉，Registry 上会长期挂着一个指向不存在镜像的版本，
> 而且**没有任何东西会报错**——第三方目录会一直展示错版本。
