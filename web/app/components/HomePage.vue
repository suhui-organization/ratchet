<script setup lang="ts">
import { ref } from 'vue'
import { site } from '~/site'

const { locale, t, altHref, href } = useLocale()

const copied = ref('')
async function copy(text: string, key: string) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = key
    setTimeout(() => (copied.value = ''), 1400)
  } catch { copied.value = '' }
}

/**
 * 安装块。有托管地址时给"一条命令"，没有时退回源码构建——
 * 不给一个点了 404 的安装命令。
 */
const blocks = computed(() =>
  site.installUrl
    ? [
        { key: 'install', label: t.value.home.install.install, lines: [`curl -fsSL ${site.installUrl}/install.sh | sh`] },
        { key: 'scan', label: t.value.home.install.scan, lines: ['ratchet scan --home ~'] },
      ]
    : [
        { key: 'build', label: t.value.home.install.build, lines: ['cd ratchet && make build', t.value.home.install.dist] },
        { key: 'scan', label: t.value.home.install.scan, lines: [t.value.home.install.scanLocal] },
      ],
)

const links = computed(() => [
  { href: '#how', label: t.value.nav.how },
  { href: href('/guard'), label: t.value.nav.blocking },
  { href: '#limits', label: t.value.nav.limits },
])

useHead(() => ({
  title: `${site.name} · ${t.value.home.tagline.title}`,
  htmlAttrs: { lang: locale.value === 'zh' ? 'zh-CN' : 'en' },
  meta: [{ name: 'description', content: t.value.home.tagline.lead }],
}))
</script>

<template>
  <div class="min-h-screen bg-surface">
    <SiteNav :links="links" :alt-href="altHref" :alt-label="t.switchLabel" :home-href="href('/')" />

    <!-- Hero -->
    <section class="mx-auto max-w-4xl px-6 pt-20 pb-16 text-center">
      <h1 class="rise text-balance text-[2.4rem] leading-[1.08] font-semibold tracking-[-0.035em] text-ink sm:text-[3.4rem]">
        {{ t.home.tagline.title }}
      </h1>
      <p class="rise rise-2 mx-auto mt-6 max-w-[52ch] text-[17px] leading-relaxed text-muted">
        {{ t.home.tagline.lead }}
      </p>
      <div class="rise rise-2 mt-8 flex flex-wrap items-center justify-center gap-x-7 gap-y-3 text-[15px]">
        <a :href="site.contactUrl"
           class="rounded-lg bg-brand-ink px-5 py-2.5 font-medium text-white hover:bg-ink">
          {{ t.home.cta.request }}
        </a>
        <a href="#start" class="py-1 text-brand-ink hover:underline">{{ t.home.cta.quickStart }}</a>
        <NuxtLink :to="href('/verify')" class="py-1 text-brand-ink hover:underline">{{ t.home.cta.verify }}</NuxtLink>
      </div>
    </section>

    <!-- Quick start -->
    <section class="mx-auto max-w-4xl px-6 pb-20">
      <div class="grid gap-5 sm:grid-cols-2">
        <div v-for="b in blocks" :key="b.key" class="overflow-hidden rounded-xl bg-code">
          <div class="flex items-center justify-between border-b border-white/10 px-4 py-2.5">
            <span class="text-[12px] text-white/60">{{ b.label }}</span>
            <button class="-my-1 py-1 font-mono text-[11px] text-white/50 hover:text-white"
                    @click="copy(b.lines.join('\n'), b.key)">
              {{ copied === b.key ? 'copied' : 'Copy' }}
            </button>
          </div>
          <pre class="overflow-x-auto px-4 py-4 font-mono text-[12.5px] leading-[2]"><code><span
            v-for="(l, i) in b.lines" :key="i" class="block text-white/85"><span class="text-white/35">$ </span>{{ l }}</span></code></pre>
        </div>
      </div>
      <p class="mt-4 text-center text-[13px] text-faint">{{ t.home.install.sourceNote }}</p>
    </section>

    <!-- 主张 + 真数据 -->
    <section id="how" class="border-y border-line bg-surface-2">
      <div class="mx-auto grid max-w-6xl items-center gap-12 px-6 py-20 lg:grid-cols-2">
        <div>
          <h2 class="text-[1.9rem] leading-tight font-semibold tracking-[-0.025em] text-ink">
            {{ t.home.how.title }}
          </h2>
          <p class="mt-5 max-w-[46ch] text-[15px] leading-relaxed text-muted">{{ t.home.how.lead }}</p>
        </div>
        <div>
          <div class="flex flex-wrap gap-1.5" aria-hidden="true">
            <span v-for="i in 16" :key="i" class="h-4 w-4 rounded-[3px]"
                  :class="i <= 12 ? 'bg-brand' : 'bg-line'"></span>
          </div>
          <p class="mt-5 max-w-[52ch] text-[14px] leading-relaxed text-muted">
            {{ t.home.evidence.lead1 }}
            <span class="tnum font-mono text-ink">{{ t.home.evidence.strong }}</span>
            {{ t.home.evidence.lead2 }}
            <span class="font-mono text-ink">{{ t.home.evidence.code }}</span>
            {{ t.home.evidence.lead3 }}
          </p>
        </div>
      </div>
    </section>

    <!-- 三个能力块 -->
    <section class="mx-auto max-w-6xl px-6 py-20">
      <div class="grid gap-10 md:grid-cols-3">
        <article v-for="f in t.home.features" :key="f.title">
          <h3 class="text-[17px] font-semibold text-ink">{{ f.title }}</h3>
          <p class="mt-3 text-[14.5px] leading-relaxed text-muted">{{ f.body }}</p>
        </article>
      </div>
    </section>

    <!-- 设计主张 -->
    <section class="border-y border-line bg-surface-2">
      <div class="mx-auto max-w-6xl px-6 py-20">
        <h2 class="text-[1.6rem] leading-tight font-semibold tracking-[-0.025em] text-ink sm:text-[1.9rem]">
          {{ t.home.design.title }}
        </h2>
        <div class="mt-10 grid gap-10 md:grid-cols-2">
          <div v-for="d in t.home.design.items" :key="d.title">
            <h3 class="text-[17px] font-semibold text-ink">{{ d.title }}</h3>
            <p class="mt-3 text-[14.5px] leading-relaxed text-muted">{{ d.body }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- 边界 -->
    <section id="limits" class="mx-auto max-w-6xl px-6 py-16">
      <h2 class="text-[1.35rem] font-semibold text-ink">{{ t.home.limitsTitle }}</h2>
      <ul class="mt-6 grid gap-x-10 gap-y-3 text-[14.5px] text-muted sm:grid-cols-2">
        <li v-for="l in t.home.limits" :key="l" class="flex gap-3">
          <span class="mt-[0.6em] h-px w-3.5 shrink-0 bg-line"></span>{{ l }}
        </li>
      </ul>
      <p class="mt-6 text-[14.5px]">
        <NuxtLink :to="href('/guard')" class="py-1 text-brand-ink hover:underline">
          {{ t.home.limitsLink }}
        </NuxtLink>
      </p>
    </section>

    <!-- 再给一次入口 -->
    <section id="start" class="border-t border-line bg-surface-2">
      <div class="mx-auto max-w-4xl px-6 py-20 text-center">
        <h2 class="text-[1.9rem] font-semibold tracking-[-0.025em] text-ink">{{ t.home.cta.getStarted }}</h2>
        <p class="mx-auto mt-4 max-w-[48ch] text-[15px] leading-relaxed text-muted">
          {{ site.installUrl ? t.home.install.getStartedLead : t.home.install.noBuildLead }}
        </p>
        <div class="mt-8 flex flex-wrap items-center justify-center gap-x-7 gap-y-3 text-[15px]">
          <a v-if="site.downloadUrl" :href="site.downloadUrl"
             class="rounded-lg bg-brand-ink px-5 py-2.5 font-medium text-white hover:bg-ink">
            {{ t.home.install.download }} {{ site.version }}
          </a>
          <a v-else :href="site.contactUrl"
             class="rounded-lg bg-brand-ink px-5 py-2.5 font-medium text-white hover:bg-ink">
            {{ t.home.cta.request }}
          </a>
          <NuxtLink :to="href('/guard')" class="py-1 text-brand-ink hover:underline">{{ t.nav.blocking }}</NuxtLink>
        </div>

        <!-- 资产不存在时整段不渲染。理由与 site.desktopBundleUrl 的注释同：
             宁可少一个入口，也不给一条点了就断的链接。 -->
        <p v-if="site.desktopBundleUrl" class="mt-5 text-[13px] leading-relaxed text-faint">
          {{ t.home.install.onDesktop }}
          <a :href="site.desktopBundleUrl" class="py-1 text-brand-ink hover:underline">
            {{ t.home.cta.desktop }} ({{ site.version }}, .mcpb)
          </a>
          {{ t.home.install.desktopSuffix }}
        </p>

        <div class="mt-10 grid gap-5 text-left sm:grid-cols-2">
          <div v-for="b in blocks" :key="'x-' + b.key" class="overflow-hidden rounded-xl bg-code">
            <div class="flex items-center justify-between border-b border-white/10 px-4 py-2.5">
              <span class="text-[12px] text-white/60">{{ b.label }}</span>
              <button class="-my-1 py-1 font-mono text-[11px] text-white/50 hover:text-white"
                      @click="copy(b.lines.join('\n'), 'x-' + b.key)">
                {{ copied === 'x-' + b.key ? 'copied' : 'Copy' }}
              </button>
            </div>
            <pre class="overflow-x-auto px-4 py-4 font-mono text-[12.5px] leading-[2]"><code><span
              v-for="(l, i) in b.lines" :key="i" class="block text-white/85"><span class="text-white/35">$ </span>{{ l }}</span></code></pre>
          </div>
        </div>
      </div>
    </section>

    <SiteFooter :t="t" :home-href="href('/')" :pricing-href="href('/pricing')" :terms-href="href('/legal/terms')" />
  </div>
</template>
