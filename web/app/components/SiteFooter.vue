<script setup lang="ts">
import { site } from '~/site'

defineProps<{
  t: {
    footer: { license: string; year: string; note: string }
    nav: { home: string; pricing: string; contact: string; terms: string }
  }
  homeHref: string
  pricingHref: string
  termsHref: string
}>()
</script>

<template>
  <footer class="border-t border-line">
    <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-x-4 gap-y-2 px-6 py-8 text-[13px] text-faint">
      <span class="font-medium text-ink">{{ site.name }}</span>
      <span>{{ t.footer.license }}</span>
      <span>·</span>
      <span>{{ t.footer.year }}</span>
      <span>{{ t.footer.note }}</span>
      <div class="ml-auto flex flex-wrap gap-x-5 gap-y-2">
        <!-- py-1 把热区撑到 28px（WCAG 2.5.8 的 24px 下限）；行高决定视觉高度，所以看起来没变。 -->
        <NuxtLink :to="homeHref" class="py-1 hover:text-brand">{{ t.nav.home }}</NuxtLink>
        <NuxtLink :to="pricingHref" class="py-1 hover:text-brand">{{ t.nav.pricing }}</NuxtLink>
        <NuxtLink :to="termsHref" class="py-1 hover:text-brand">{{ t.nav.terms }}</NuxtLink>
        <a :href="site.contactUrl" class="py-1 hover:text-brand">{{ t.nav.contact }}</a>
      </div>
    </div>
  </footer>
</template>
