# 安全响应头：唯一定义处（deploy/docker/security-headers.inc）。
#
# 站点自托管字体与脚本，不加载任何第三方资源，所以 CSP 可以收到 default-src 'self'。
# 内联样式只留给 Nuxt 产物的 <style>（Tailwind 的产物里确实有）。
#
# 用法：server 块与每个自己写了 add_header 的 location 都要 include 一次——
# nginx 的 add_header 不跨块叠加，漏 include 就等于把这一整组头丢掉了。
add_header X-Content-Type-Options "nosniff" always;
add_header X-Frame-Options "SAMEORIGIN" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
add_header Content-Security-Policy "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'self'; script-src 'self'" always;
