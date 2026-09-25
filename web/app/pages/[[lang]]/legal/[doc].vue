<script setup lang="ts">
import { docBySlug } from '~/content/legal'
import { site } from '~/site'

const { locale, t, altHref, href } = useLocale()
const route = useRoute()
// 按当前语言取文档；取不到就是 null（页面渲染 Not found）。
const doc = computed(() => docBySlug(String(route.params.doc || ''), locale.value))

useHead(() => ({
  title: doc.value ? `${doc.value.title} · ${site.name}` : `Not found · ${site.name}`,
  htmlAttrs: { lang: locale.value === 'zh' ? 'zh-CN' : 'en' },
}))
</script>

<template>
  <div class="min-h-screen bg-surface">
    <header class="border-b border-line">
      <nav class="mx-auto flex max-w-3xl items-center gap-6 px-6 py-4 text-sm">
        <NuxtLink :to="href('/')" class="py-1 font-semibold tracking-tight text-ink">{{ site.name }}</NuxtLink>
        <div class="ml-auto flex items-center gap-5 text-muted">
          <NuxtLink :to="href('/pricing')" class="py-1.5 hover:text-brand">{{ t.nav.pricing }}</NuxtLink>
          <a :href="altHref" class="py-1.5 whitespace-nowrap hover:text-brand">{{ t.switchLabel }}</a>
        </div>
      </nav>
    </header>

    <main class="mx-auto max-w-3xl px-6 py-14">
      <template v-if="doc">
        <h1 class="text-[2rem] font-semibold tracking-[-0.03em] text-ink">{{ doc.title }}</h1>
        <p class="mt-2 text-[13px] text-faint">{{ t.legal.updated }} {{ doc.updated }}</p>
        <!-- 译本声明放在正文之前。放在页脚等于没写：读者已经按译文理解完了。 -->
        <p v-if="doc.governingNote"
           class="mt-5 rounded-lg border border-line bg-surface-2 px-4 py-3 text-[13.5px] leading-relaxed text-muted">
          {{ doc.governingNote }}
        </p>
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
            <a :href="site.contactUrl" class="py-1 text-brand-ink hover:underline">{{ site.contactEmail }}</a>
          </p>
        </section>
      </template>
      <template v-else>
        <h1 class="text-[1.6rem] font-semibold text-ink">{{ t.legal.notFound }}</h1>
        <p class="mt-3 text-muted">{{ t.legal.notFoundBody }}</p>
        <NuxtLink :to="href('/')" class="mt-6 inline-block text-brand-ink hover:underline">← {{ site.name }}</NuxtLink>
      </template>
    </main>
  </div>
</template>
