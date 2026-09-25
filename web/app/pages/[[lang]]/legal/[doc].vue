<script setup lang="ts">
import { docBySlug } from '~/content/legal'
import { site } from '~/site'

/**
 * 法律条款页。
 *
 * **刻意只有英文。** 条款是要承担法律责任的文本，翻译它需要法律意见，
 * 不是文案工作；在没有律师过一遍之前，给出一份"看起来也是正式条款"的中文版
 * 比不给更危险。中文页脚因此指向同一份英文条款。
 */
const { locale, t, altHref, href } = useLocale()
const route = useRoute()
const doc = computed(() => docBySlug(String(route.params.doc || '')))

useHead(() => ({
  title: doc.value ? `${doc.value.title} · ${site.name}` : `Not found · ${site.name}`,
  htmlAttrs: { lang: locale.value === 'zh' ? 'zh-CN' : 'en' },
}))
</script>

<template>
  <div class="min-h-screen bg-surface">
    <header class="border-b border-line">
      <nav class="mx-auto flex max-w-3xl items-center gap-6 px-6 py-4 text-sm">
        <NuxtLink :to="href('/')" class="font-semibold tracking-tight text-ink">{{ site.name }}</NuxtLink>
        <div class="ml-auto flex items-center gap-5 text-muted">
          <NuxtLink :to="href('/pricing')" class="py-1.5 hover:text-brand">{{ t.nav.pricing }}</NuxtLink>
          <a :href="altHref" class="py-1.5 whitespace-nowrap hover:text-brand">{{ t.switchLabel }}</a>
        </div>
      </nav>
    </header>

    <main class="mx-auto max-w-3xl px-6 py-14">
      <template v-if="doc">
        <h1 class="text-[2rem] font-semibold tracking-[-0.03em] text-ink">{{ doc.title }}</h1>
        <p class="mt-2 text-[13px] text-faint">Last updated {{ doc.updated }}</p>
        <p class="mt-6 text-[15px] leading-relaxed text-muted">{{ doc.intro }}</p>

        <section v-for="s in doc.sections" :key="s.heading" class="mt-10">
          <h2 class="text-[17px] font-semibold text-ink">{{ s.heading }}</h2>
          <p v-for="(para, i) in s.body" :key="i" class="mt-3 text-[14.5px] leading-relaxed text-muted">
            {{ para }}
          </p>
        </section>

        <section class="mt-12 border-t border-line pt-6">
          <h2 class="text-[15px] font-semibold text-ink">{{ t.nav.contact }}</h2>
          <p class="mt-2 text-[14px]">
            <a :href="site.contactUrl" class="text-brand-ink hover:underline">{{ site.contactEmail }}</a>
          </p>
        </section>
      </template>
      <template v-else>
        <h1 class="text-[1.6rem] font-semibold text-ink">Not found</h1>
        <p class="mt-3 text-muted">That document does not exist.</p>
        <NuxtLink :to="href('/')" class="mt-6 inline-block text-brand-ink hover:underline">← {{ site.name }}</NuxtLink>
      </template>
    </main>
  </div>
</template>
