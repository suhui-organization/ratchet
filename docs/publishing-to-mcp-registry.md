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
`io.github.iversonwuwei/ratchet`。

> **为什么是个人命名空间而不是组织**：Registry 只允许发布到"你已证明归属"的命名空间。
> 用 `io.github.suhui-organization/*` 会返回 403：
> `You have permission to publish: io.github.iversonwuwei/*`。
> 要改成组织命名空间，需要先在 GitHub 设置里**把组织成员身份设为公开**
> （个人资料 → Organizations → 把该组织可见性改成 public），之后才能发布到组织前缀。
> 命名空间与仓库归属无关——`repository.url` 仍然指向组织仓库。

## 版本对齐（别漏）

`server.json` 的顶层 `version` 必须与**实际发布的镜像 tag** 一致。
OCI 包**不能有 `version` 字段**——版本要写进 `identifier`，形如
`ghcr.io/owner/image:tag`。

**注意 tag 带 `v`**：CI 用 git tag 名做镜像 tag，所以实际是 `v0.10.0` 而不是 `0.10.0`。
写错会得到 `OCI image '…' does not exist in the registry`。

现在 `identifier` 是 `ghcr.io/suhui-organization/ratchet:v0.10.0`。发新版的顺序：

1. 改 `cmd/ratchet/main.go` 的版本号
2. 打 tag（`vX.Y.Z`）推上去 → CI 编四平台 + 推镜像（tag 为 `vX.Y.Z` 与 `latest`）
3. 改 `server.json` 的顶层 `version` 与 `identifier` 里的 tag（**两处都要带 `v`**）
4. `mcp-publisher publish`

> 第 3 步漏掉，Registry 上会长期挂着一个指向不存在镜像的版本，
> 而且**没有任何东西会报错**——第三方目录会一直展示错版本。
