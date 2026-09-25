<script setup lang="ts">
import { site } from '~/site'

/**
 * 站点自己的错误页。
 *
 * 为什么不用 Nuxt 自带的：实测它有三处不合适——`<html>` 没有 lang（读屏会把中文
 * 当英文念）、正文颜色不满足对比度、返回链接的可点区域不到 24px。
 * 一个讲"证据"的产品，客户点到一个连 lang 都写错的页面会怎么看。
 *
 * 文案里那句"这台机器上没有任何东西被改动"是有意写的：404 出现在一个
 * 涉及本地权限的工具站点上时，读者第一反应是"我是不是把什么弄坏了"。
 */
const { locale, t, altHref, href } = useLocale()
const props = defineProps<{ error: { statusCode?: number; statusMessage?: string } }>()

useHead(() => ({
  title: `${props.error.statusCode || 500} · ${site.name}`,
  htmlAttrs: { lang: locale.value === 'zh' ? 'zh-CN' : 'en' },
}))
</script>

<template>
  <div class="flex min-h-screen flex-col bg-surface">
    <header class="border-b border-line">
      <nav class="mx-auto flex max-w-3xl items-center gap-6 px-6 py-4 text-sm">
        <NuxtLink :to="href('/')" class="py-1 font-semibold tracking-tight text-ink">{{ site.name }}</NuxtLink>
        <a :href="altHref" class="ml-auto py-1.5 whitespace-nowrap text-muted hover:text-brand">{{ t.switchLabel }}</a>
      </nav>
    </header>

    <main class="mx-auto w-full max-w-3xl px-6 py-20">
      <p class="tnum font-mono text-[13px] text-faint">{{ error.statusCode || 500 }}</p>
      <h1 class="mt-3 text-[2rem] font-semibold tracking-[-0.03em] text-ink">
        {{ t.error.title }}
      </h1>
      <p class="mt-4 max-w-[56ch] text-[15px] leading-relaxed text-muted">{{ t.error.body }}</p>
      <NuxtLink :to="href('/')"
                class="mt-8 inline-block rounded-lg bg-brand-ink px-5 py-2.5 text-[14px] font-medium text-white hover:bg-ink">
        {{ t.error.home }}
      </NuxtLink>
    </main>
  </div>
</template>
