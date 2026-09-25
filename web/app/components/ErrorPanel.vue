<script setup lang="ts">
import { site } from '~/site'

/**
 * 错误页正文。抽成组件是因为它有两个入口，两处必须长得一模一样：
 *
 * 1. `app/error.vue`——Nuxt 的错误边界，开发环境和 SSR 下出错时走这里；
 * 2. `app/pages/[[lang]]/not-found.vue`——一个**真实存在的预渲染页面**，
 *    静态产物下由 nginx 的 `error_page 404` 指过来。
 *
 * 为什么必须有第二个入口（而不是让 nginx 发 nitro 生成的 `404.html`）：
 * 那份 `404.html` 是 SPA 兜底壳，客户端启动就抛
 * `Cannot read properties of undefined (reading 'app')`，页面**全白**——
 * 实测过，不是猜的。静态站点没有 Node 运行时来兜底，所以 404 必须是一个
 * 构建期就渲染好的文件，不能依赖浏览器补出来。
 *
 * 文案里那句"这台机器上没有任何东西被改动"是有意写的：404 出现在一个
 * 涉及本地权限的工具站点上时，读者第一反应是"我是不是把什么弄坏了"。
 */
defineProps<{ statusCode: number }>()

const { t, altHref, href } = useLocale()
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
      <p class="tnum font-mono text-[13px] text-faint">{{ statusCode }}</p>
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
