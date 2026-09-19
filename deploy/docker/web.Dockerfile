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
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run generate

FROM nginx:1.27-alpine

# 必须覆盖镜像自带的 default.conf（名字也得叫这个，否则按字母序排在它后面就不生效）；
# 头部片段用 .inc 而不是 .conf：conf.d/*.conf 会被主配置自动加载一遍，
# 那相当于提前把 add_header 挂到 http 层，行为会变得不好预测。
COPY deploy/docker/default.conf /etc/nginx/conf.d/default.conf
COPY deploy/docker/security-headers.inc /etc/nginx/conf.d/security-headers.inc
COPY --from=build /app/.output/public /usr/share/nginx/html

EXPOSE 80
