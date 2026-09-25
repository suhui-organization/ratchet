<script setup lang="ts">
import { site } from '~/site'

const { locale, t, altHref, href } = useLocale()
const p = computed(() => t.value.pricing)

// 收银台：Paddle overlay。配置缺失时按钮自动退回"写邮件询价"，不会出现点了没反应的按钮。
const { configured, busy, error, buy } = usePaddleCheckout()

useHead(() => ({
  title: `${site.name} · ${p.value.title}`,
  htmlAttrs: { lang: locale.value === 'zh' ? 'zh-CN' : 'en' },
  meta: [{ name: 'description', content: p.value.lead }],
}))
</script>

<template>
  <div class="min-h-screen bg-surface">
    <header class="border-b border-line">
      <nav class="mx-auto flex max-w-5xl items-center gap-6 px-6 py-4 text-sm">
        <NuxtLink :to="href('/')" class="py-1 font-semibold tracking-tight text-ink">{{ site.name }}</NuxtLink>
        <div class="ml-auto flex items-center gap-5 text-muted">
          <NuxtLink :to="href('/')" class="py-1.5 hover:text-brand">{{ t.nav.home }}</NuxtLink>
          <a :href="altHref" class="py-1.5 whitespace-nowrap hover:text-brand">{{ t.switchLabel }}</a>
        </div>
      </nav>
    </header>

    <main class="mx-auto max-w-5xl px-6 py-16">
      <h1 class="text-[2.2rem] font-semibold tracking-[-0.03em] text-ink">{{ p.title }}</h1>
      <p class="mt-4 max-w-[60ch] text-[15px] leading-relaxed text-muted">{{ p.lead }}</p>

      <div class="mt-10 grid gap-6 md:grid-cols-2">
        <section class="rounded-xl border border-line p-6">
          <h2 class="text-[17px] font-semibold text-ink">{{ p.free.title }}</h2>
          <p class="mt-3 text-[2rem] font-semibold tracking-tight text-ink">{{ p.free.price }}</p>
          <p class="mt-3 text-[14px] leading-relaxed text-muted">{{ p.free.body }}</p>
          <ul class="mt-5 space-y-2 text-[14px] text-muted">
            <li v-for="b in p.free.bullets" :key="b">· {{ b }}</li>
          </ul>
          <a :href="site.repoUrl"
             class="mt-6 inline-block rounded-lg border border-line px-4 py-2 text-[14px] text-ink hover:border-brand">
            {{ p.free.cta }}
          </a>
        </section>

        <section class="rounded-xl border border-brand/40 bg-surface-2 p-6">
          <h2 class="text-[17px] font-semibold text-ink">{{ p.audit.title }}</h2>
          <p class="mt-3 text-[2rem] font-semibold tracking-tight text-ink">{{ site.auditPrice }}</p>
          <p class="mt-1 text-[13px] text-faint">{{ p.audit.per }}</p>
          <p class="mt-3 text-[14px] leading-relaxed text-muted">{{ p.audit.body }}</p>
          <ul class="mt-5 space-y-2 text-[14px] text-muted">
            <li v-for="b in p.audit.bullets" :key="b">· {{ b }}</li>
          </ul>
          <button v-if="configured" type="button" :disabled="busy"
                  class="mt-6 inline-block rounded-lg bg-brand-ink px-4 py-2 text-[14px] font-medium text-white hover:bg-ink disabled:opacity-60"
                  @click="buy">
            {{ busy ? p.opening : p.buy }}
          </button>
          <a v-else :href="site.contactUrl"
             class="mt-6 inline-block rounded-lg bg-brand-ink px-4 py-2 text-[14px] font-medium text-white hover:bg-ink">
            {{ p.requestQuote }}
          </a>
          <p v-if="error" class="mt-3 text-[13px] leading-relaxed text-fail">{{ error }}</p>
          <p class="mt-3 text-[12px] leading-relaxed text-faint">{{ p.paddleNote }}</p>
        </section>
      </div>

      <p v-if="!site.auditPriceIsPublished"
         class="mt-8 rounded-lg border border-line bg-surface-2 px-5 py-4 text-[13.5px] text-muted">
        <strong class="font-medium text-ink">{{ p.reviewNoteTitle }}</strong>
        {{ p.reviewNoteBody1 }}
        <a :href="site.contactUrl" class="py-1 text-brand-ink hover:underline">{{ site.contactEmail }}</a>
        {{ p.reviewNoteBody2 }}
      </p>

      <footer class="mt-14 flex flex-wrap gap-x-6 gap-y-2 border-t border-line pt-6 text-[13px] text-faint">
        <NuxtLink :to="href('/legal/terms')" class="py-1 hover:text-brand">{{ t.nav.terms }}</NuxtLink>
        <NuxtLink :to="href('/legal/privacy')" class="py-1 hover:text-brand">Privacy</NuxtLink>
        <NuxtLink :to="href('/legal/refund')" class="py-1 hover:text-brand">Refunds</NuxtLink>
        <a :href="site.contactUrl" class="py-1 hover:text-brand">{{ t.nav.contact }}</a>
      </footer>
    </main>
  </div>
</template>
