<script setup lang="ts">
import { site } from '~/site'

/**
 * 静态产物下的 404 页。
 *
 * 它是一条**普通路由**，会被 `nuxt generate` 预渲染成真实文件
 * （`/not-found/index.html` 与 `/zh/not-found/index.html`），再由 nginx 的
 * `error_page 404` 指过来，同时保留 404 状态码。
 *
 * 为什么绕这一圈：nginx 直接发 nitro 生成的 `404.html` 会得到一片空白
 * ——那份壳在客户端启动就抛异常（详见 ErrorPanel.vue 与 default.conf 注释）。
 * 静态站点里 404 的身体必须是构建期就写好的 HTML。
 *
 * 这个地址本身返回 200，所以加 noindex：它不是内容页，不该被收录，
 * 也不该出现在搜索结果的"Ratchet"里。
 */
const { locale } = useLocale()

useHead(() => ({
  title: `404 · ${site.name}`,
  htmlAttrs: { lang: locale.value === 'zh' ? 'zh-CN' : 'en' },
  meta: [{ name: 'robots', content: 'noindex' }],
}))
</script>

<template>
  <ErrorPanel :status-code="404" />
</template>
