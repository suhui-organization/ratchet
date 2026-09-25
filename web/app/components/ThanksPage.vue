<script setup lang="ts">
import { site } from '~/site'

const { locale, t, altHref, href } = useLocale()
const th = computed(() => t.value.thanks)

useHead(() => ({
  title: `${th.value.title} · ${site.name}`,
  htmlAttrs: { lang: locale.value === 'zh' ? 'zh-CN' : 'en' },
}))
</script>

<template>
  <div class="min-h-screen bg-surface">
    <header class="border-b border-line">
      <nav class="mx-auto flex max-w-3xl items-center gap-6 px-6 py-4 text-sm">
        <NuxtLink :to="href('/')" class="py-1 font-semibold tracking-tight text-ink">{{ site.name }}</NuxtLink>
        <a :href="altHref" class="ml-auto py-1.5 whitespace-nowrap text-muted hover:text-brand">{{ t.switchLabel }}</a>
      </nav>
    </header>

    <main class="mx-auto max-w-3xl px-6 py-20">
      <h1 class="text-[2rem] font-semibold tracking-[-0.03em] text-ink">{{ th.title }}</h1>
      <p class="mt-4 max-w-[62ch] text-[15px] leading-relaxed text-muted">{{ th.lead }}</p>

      <h2 class="mt-12 text-[17px] font-semibold text-ink">{{ th.next }}</h2>
      <ol class="mt-5 space-y-4 text-[14.5px] leading-relaxed text-muted">
        <li>
          <span class="font-medium text-ink">{{ th.step1When }}</span>
          {{ th.step1Body(site.contactEmail).replace(/^[^-]*[-—]\s*/, '') }}
        </li>
      </ol>

      <h2 class="mt-12 text-[17px] font-semibold text-ink">{{ th.clientTitle }}</h2>
      <p class="mt-4 max-w-[62ch] text-[15px] leading-relaxed text-muted">{{ th.clientBody }}</p>
      <NuxtLink :to="href('/verify')"
                class="mt-5 inline-block rounded-lg bg-brand-ink px-5 py-2.5 text-[14px] font-medium text-white hover:bg-ink">
        {{ th.clientCta }}
      </NuxtLink>

      <p class="mt-12 border-t border-line pt-6 text-[13px] leading-relaxed text-faint">
        {{ th.refundNote1 }}
        <NuxtLink :to="href('/legal/refund')" class="py-1 text-brand-ink hover:underline">{{ th.refundNoteLink }}</NuxtLink>。
        {{ th.refundNote2 }}
        <a :href="site.contactUrl" class="py-1 text-brand-ink hover:underline">{{ site.contactEmail }}</a>
        {{ th.refundNote3 }}
      </p>
    </main>
  </div>
</template>
