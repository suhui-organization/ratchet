<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { docBySlug } from '~/content/legal'
import { site } from '~/site'

const route = useRoute()
const doc = computed(() => docBySlug(String(route.params.doc || '')))
</script>

<template>
  <div class="min-h-screen bg-surface">
    <header class="border-b border-line">
      <nav class="mx-auto flex max-w-3xl items-center gap-6 px-6 py-4 text-sm">
        <NuxtLink to="/" class="font-semibold tracking-tight text-ink">{{ site.name }}</NuxtLink>
        <div class="ml-auto flex gap-5 text-muted">
          <NuxtLink to="/pricing" class="hover:text-brand">Pricing</NuxtLink>
          <NuxtLink to="/" class="hover:text-brand">Home</NuxtLink>
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
          <h2 class="text-[15px] font-semibold text-ink">Contact</h2>
          <p class="mt-2 text-[14px]">
            <a :href="site.contactUrl" class="text-brand hover:underline">{{ site.contactEmail }}</a>
          </p>
        </section>
      </template>
      <template v-else>
        <h1 class="text-[1.6rem] font-semibold text-ink">Not found</h1>
        <p class="mt-3 text-muted">That document does not exist.</p>
        <NuxtLink to="/" class="mt-6 inline-block text-brand hover:underline">← Back to Ratchet</NuxtLink>
      </template>
    </main>
  </div>
</template>
