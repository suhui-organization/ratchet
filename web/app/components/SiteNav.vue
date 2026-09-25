<script setup lang="ts">
import { site } from '~/site'

/**
 * 站点头部。
 *
 * 窄屏上只留「品牌（回首页）+ 版本 + 语言切换」：390px 下四个锚点链接放不下，
 * 之前实测会顶到 398px、把整页撑出横向滚动。页面自己的内容里已经有那些入口，
 * 所以砍掉的是导航里重复的一层，不是唯一路径。
 */
const props = defineProps<{
  /** 当前页要显示的锚点链接（`{ href, label }`） */
  links?: { href: string; label: string }[]
  /** 另一语言下同一页的地址 */
  altHref: string
  /** 切换器上显示的文字（目标语言的名字） */
  altLabel: string
  /** 当前语言的首页地址 */
  homeHref: string
}>()

const links = computed(() => props.links ?? [])
</script>

<template>
  <header class="sticky top-0 z-20 border-b border-line bg-surface">
    <nav class="mx-auto flex max-w-6xl items-center gap-2.5 px-6 py-3.5 text-sm sm:gap-3">
      <!-- py-1 让品牌链接的可点区域到 28px：WCAG 2.5.8 要求目标至少 24×24。
           视觉上不变（行高决定高度），只是把热区撑开。 -->
      <NuxtLink :to="homeHref" class="shrink-0 py-1 font-semibold tracking-tight text-ink hover:text-brand">
        {{ site.name }}
      </NuxtLink>
      <span class="tnum shrink-0 rounded border border-line px-1.5 py-0.5 font-mono text-[11px] text-faint">
        {{ site.version }}
      </span>
      <div class="ml-auto flex items-center gap-5 text-muted sm:gap-6">
        <a v-for="l in links" :key="l.href" :href="l.href"
           class="hidden py-1.5 hover:text-brand sm:inline">{{ l.label }}</a>
        <a :href="altHref" class="py-1.5 whitespace-nowrap hover:text-brand">{{ altLabel }}</a>
      </div>
    </nav>
  </header>
</template>
