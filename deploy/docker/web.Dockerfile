# Ratchet 站点镜像：Nuxt 静态导出 → nginx。
#
# 站点是纯静态的（落地页 + 定价/法务页 + 验证页，哈希在浏览器里算），
# 所以运行期只跑 nginx：没有 Node 运行时、没有后端进程、没有任何环境变量，
# 也就没有可以在运行时被打歪的东西。这不是省事，见 docs/deploy-site.md。
#
# 构建上下文 = 仓库根目录：
#   docker build -f deploy/docker/web.Dockerfile -t ratchet-web:<tag> .
#
# 集群节点是 Core2 级的 x86-64-v1，别换成 UBI/RHEL 系基础镜像（glibc 编到 x86-64-v2，
# 起来就是 Fatal glibc error）；nginx:alpine 在这套集群上已经跑了几十个 Pod。

FROM node:22-alpine AS build

WORKDIR /app
# 先拷清单再拷源码：依赖没变时这一层命中缓存，发版只重跑 nuxt generate
COPY web/package.json web/package-lock.json ./
# npm 源可换：这台机器在国内，实测取 registry.npmjs.org 上一个 73KB 的包要 17s，
# 一次 npm ci 会在某个 tarball 上稳定超时（errno ETIMEDOUT @parcel/watcher-wasm），
# 加满重试也没用——不是抖动，是慢。换国内镜像后同一个包 0.18s。
# 默认不写死：在别处构建仍然走官方源；从这台机器发版时传
#   --build-arg NPM_REGISTRY=https://registry.npmmirror.com
# build-push.sh 已把这个变量透传（见 NPM_REGISTRY）。
ARG NPM_REGISTRY=""
RUN npm ci --no-audit --no-fund ${NPM_REGISTRY:+--registry=$NPM_REGISTRY} \
      --fetch-retries=6 \
      --fetch-retry-mintimeout=15000 \
      --fetch-retry-maxtimeout=180000 \
      --fetch-timeout=600000
COPY web/ ./
RUN npm run generate

# CSP 的脚本哈希必须在 generate **之后**算：Nuxt 往产物里写的内联 <script>
# （运行时配置 + 路由 payload + importmap）带 buildId，每个版本都不一样。
# 不生成这一步、或者是拿旧产物生成的，表现是"页面看起来完全正常但整站不水合"
# ——收银台按钮点了没反应。详见 render-security-headers.mjs 顶部。
COPY deploy/docker/security-headers.inc.in deploy/docker/render-security-headers.mjs /app/deploy/docker/
RUN node /app/deploy/docker/render-security-headers.mjs \
      /app/.output/public \
      /app/deploy/docker/security-headers.inc.in \
      /app/security-headers.inc

FROM nginx:1.27-alpine

# 必须覆盖镜像自带的 default.conf（名字也得叫这个，否则按字母序排在它后面就不生效）；
# 头部片段用 .inc 而不是 .conf：conf.d/*.conf 会被主配置自动加载一遍，
# 那相当于提前把 add_header 挂到 http 层，行为会变得不好预测。
COPY deploy/docker/default.conf /etc/nginx/conf.d/default.conf
# 响应头用的是构建阶段按本次产物渲染出来的版本（脚本哈希在里面），不是仓库里那份模板。
COPY --from=build /app/security-headers.inc /etc/nginx/conf.d/security-headers.inc
COPY --from=build /app/.output/public /usr/share/nginx/html

EXPOSE 80
