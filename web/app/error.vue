<script setup lang="ts">
import { site } from '~/site'

/**
 * Nuxt 错误边界。正文在 ErrorPanel 里，与静态产物下的 404 页共用一份。
 *
 * 这里只负责两件事：把状态码传下去，以及补上 `<html lang>`——Nuxt 自带的错误页
 * 缺 lang（读屏会把中文当英文念）、正文对比度不达标、返回链接的可点区域不到 24px。
 * 一个讲"证据"的产品，客户点到一个连 lang 都写错的页面会怎么看。
 *
 * 生产环境（静态产物）下真正兜住 404 的不是这个文件，而是 /not-found 页面
 * ——见 ErrorPanel.vue 里的说明。
 */
const { locale } = useLocale()
const props = defineProps<{ error: { statusCode?: number; statusMessage?: string } }>()

useHead(() => ({
  title: `${props.error.statusCode || 500} · ${site.name}`,
  htmlAttrs: { lang: locale.value === 'zh' ? 'zh-CN' : 'en' },
}))
</script>

<template>
  <ErrorPanel :status-code="error.statusCode || 500" />
</template>
