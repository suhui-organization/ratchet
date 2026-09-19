# 安全响应头：唯一定义处（deploy/docker/security-headers.inc）。
#
# 站点自托管字体与脚本，唯一的第三方是收银台 Paddle，所以 CSP 能收得很紧。
# 内联样式只留给 Nuxt 产物的 <style>（Tailwind 产物里确实有）。
#
# 用法：server 块与每个自己写了 add_header 的 location 都要 include 一次——
# nginx 的 add_header 不跨块叠加，漏 include 就等于把这一整组头丢掉了。
#
# 关于 Paddle 的那几个来源（少一个，收银台就静默打不开，且只在浏览器控制台报错）：
#   script-src  cdn.paddle.com        Paddle.js 本体
#   frame-src   *.paddle.com          收银台跑在 iframe 里
#   connect-src *.paddle.com          Paddle.js 与 iframe 之间的通信
#   form-action *.paddle.com          某些支付方式会提交表单
add_header X-Content-Type-Options "nosniff" always;
add_header X-Frame-Options "SAMEORIGIN" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
add_header Content-Security-Policy "default-src 'self'; img-src 'self' data: https://*.paddle.com; style-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'self' https://*.paddle.com; script-src 'self' https://cdn.paddle.com; frame-src https://*.paddle.com; form-action 'self' https://*.paddle.com" always;
