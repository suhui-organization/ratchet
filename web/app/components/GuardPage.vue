<script setup lang="ts">
import { ref } from 'vue'
import { site } from '~/site'

const { locale, t, altHref, href } = useLocale()
const g = computed(() => t.value.guard)

const copied = ref('')
async function copy(text: string, key: string) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = key
    setTimeout(() => (copied.value = ''), 1400)
  } catch { copied.value = '' }
}

const links = computed(() => [
  { href: '#rules', label: t.value.nav.rules },
  { href: '#agents', label: t.value.nav.agents },
  { href: '#risks', label: t.value.nav.limits },
])

const blocks = computed(() => [
  { key: 'dry', label: g.value.install.dry, lines: [g.value.commands.check] },
  { key: 'install', label: g.value.install.then, lines: [g.value.commands.install, g.value.commands.write] },
])

/**
 * 卖点拆成"一句主张 + 三条支撑"。
 *
 * 为什么不排成 2×2 等大卡片：craft floor 把「同样大小的卡片，每张一个标题加一段话」
 * 列为页面结构的默认套路。四条主张本来就有轻重，排成等大就把这个信息抹掉了。
 */
const leadPoint = computed(() => g.value.points[0])
const restPoints = computed(() => g.value.points.slice(1))

useHead(() => ({
  title: `${site.name} · ${g.value.title}`,
  // lang 属性跟着语言走：读屏与拼写检查都靠它，写错了中文页面会被当英文朗读。
  htmlAttrs: { lang: locale.value === 'zh' ? 'zh-CN' : 'en' },
  meta: [{ name: 'description', content: g.value.lead }],
}))
</script>

<template>
  <div class="min-h-screen bg-surface">
    <SiteNav :links="links" :alt-href="altHref" :alt-label="t.switchLabel" :home-href="href('/')" />

    <!-- Hero：左对齐。这是信任优先的页面，不是表演性的落地页。
         标题上方不放大字标签——craft floor 把这种 eyebrow 列为硬禁：
         标题本身要能承重，加一个标签只是在替它打气。 -->
    <section class="mx-auto max-w-4xl px-6 pt-20 pb-14">
      <h1 class="rise max-w-[15ch] text-balance text-[2.4rem] leading-[1.1] font-semibold tracking-[-0.035em] text-ink sm:text-[3rem]">
        {{ g.title }}
      </h1>
      <p class="rise rise-2 mt-6 max-w-[54ch] text-[17px] leading-relaxed text-muted">{{ g.lead }}</p>
      <div class="rise rise-2 mt-8 flex flex-wrap items-center gap-x-7 gap-y-3 text-[15px]">
        <a href="#risks" class="rounded-lg border border-line px-5 py-2.5 font-medium text-ink hover:border-brand">
          {{ g.ctaReadLimits }}
        </a>
        <a href="#install" class="py-1 text-brand-ink hover:underline">{{ g.ctaInstall }}</a>
        <NuxtLink :to="href('/verify')" class="py-1 text-brand-ink hover:underline">{{ t.guard.install.verify }}</NuxtLink>
      </div>
    </section>

    <!-- 卖点：一条主张 + 三条支撑（不是等大卡片阵） -->
    <section class="border-y border-line bg-surface-2">
      <div class="mx-auto grid max-w-6xl gap-x-16 gap-y-10 px-6 py-18 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
        <div>
          <h2 class="max-w-[18ch] text-[1.5rem] leading-snug font-semibold tracking-[-0.02em] text-ink">
            {{ leadPoint?.title }}
          </h2>
          <p class="mt-4 max-w-[44ch] text-[15px] leading-relaxed text-muted">{{ leadPoint?.body }}</p>
        </div>
        <dl class="divide-y divide-line border-t border-line">
          <div v-for="p in restPoints" :key="p.title"
               class="grid gap-x-6 gap-y-1.5 py-5 sm:grid-cols-[8.5rem_minmax(0,1fr)]">
            <dt class="text-[14px] font-medium text-ink">{{ p.title }}</dt>
            <dd class="max-w-[54ch] text-[14px] leading-relaxed text-muted">{{ p.body }}</dd>
          </div>
        </dl>
      </div>
    </section>

    <!-- 判定顺序：这是全页最该被读懂的部分 -->
    <section id="rules" class="mx-auto max-w-6xl px-6 py-18">
      <h2 class="text-[1.35rem] font-semibold text-ink">{{ g.rulesTitle }}</h2>
      <p class="mt-4 max-w-[62ch] text-[14.5px] leading-relaxed text-muted">{{ g.rulesLead }}</p>

      <!-- 窄屏堆叠，宽屏成列。一份 DOM，不是"表 + 一份移动版列表"——后者会让读屏听两遍。 -->
      <div class="mt-10">
        <div class="hidden border-b border-line pb-2.5 sm:grid sm:grid-cols-[1.5rem_11rem_minmax(0,1fr)_7.5rem] sm:gap-x-6">
          <span class="text-[12px] font-medium tracking-wide text-faint uppercase">{{ g.rulesHead.n }}</span>
          <span class="text-[12px] font-medium tracking-wide text-faint uppercase">{{ g.rulesHead.name }}</span>
          <span class="text-[12px] font-medium tracking-wide text-faint uppercase">{{ g.rulesHead.when }}</span>
          <span class="text-[12px] font-medium tracking-wide text-faint uppercase">{{ g.rulesHead.result }}</span>
        </div>
        <ul class="divide-y divide-line border-t border-line sm:border-t-0">
          <li v-for="r in g.rules" :key="r.name"
              class="grid gap-x-6 gap-y-1.5 py-4 sm:grid-cols-[1.5rem_11rem_minmax(0,1fr)_7.5rem]">
            <span class="tnum font-mono text-[12.5px] text-faint">{{ r.n }}</span>
            <span class="font-mono text-[12.5px] text-ink">{{ r.name }}</span>
            <span class="text-[14px] leading-relaxed text-muted">
              {{ r.when }}
              <span class="mt-1 block text-[13px] text-faint">{{ r.why }}</span>
            </span>
            <span class="text-[14px] text-ink">{{ r.then }}</span>
          </li>
        </ul>
      </div>
    </section>

    <!-- 两类目标：为什么不能合成一张表 -->
    <section class="border-y border-line bg-surface-2">
      <div class="mx-auto max-w-6xl px-6 py-18">
        <h2 class="text-[1.35rem] font-semibold text-ink">{{ g.targets.title }}</h2>
        <p class="mt-4 max-w-[64ch] text-[14.5px] leading-relaxed text-muted">{{ g.targets.body }}</p>

        <div class="mt-10 grid gap-6 sm:grid-cols-2">
          <div v-for="c in g.targets.classes" :key="c.name" class="rounded-xl border border-line bg-surface p-5">
            <h3 class="font-mono text-[13px] text-ink">{{ c.name }}</h3>
            <p class="mt-3 text-[14px] leading-relaxed text-muted">{{ c.contents }}</p>
            <p class="mt-3 border-t border-line pt-3 text-[13.5px] text-ink">{{ c.when }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- 覆盖范围 -->
    <section id="agents" class="mx-auto max-w-6xl px-6 py-18">
      <h2 class="text-[1.35rem] font-semibold text-ink">{{ g.agents.title }}</h2>
      <p class="mt-4 max-w-[62ch] text-[14.5px] leading-relaxed text-muted">{{ g.agents.body }}</p>

      <!-- 同样是一份 DOM 两套排布。12 行 × 3 列在 390px 上必然要么横滚要么挤成一条线。 -->
      <ul class="mt-10 grid gap-x-10 sm:grid-cols-2">
        <li v-for="a in g.agents.rows" :key="a.agent" class="border-t border-line py-3.5">
          <p class="flex flex-wrap items-baseline gap-x-3">
            <span class="text-[14.5px] text-ink">{{ a.agent }}</span>
            <span class="font-mono text-[12px] text-faint">{{ a.event }}</span>
          </p>
          <p class="mt-1 font-mono text-[12.5px] break-all text-muted">{{ a.config }}</p>
        </li>
      </ul>
      <p class="mt-6 max-w-[64ch] text-[13.5px] leading-relaxed text-faint">{{ g.agents.note }}</p>
    </section>

    <!-- 风险点：主体内容，不是免责声明 -->
    <section id="risks" class="border-y border-line bg-surface-2">
      <div class="mx-auto max-w-6xl px-6 py-18">
        <h2 class="text-[1.35rem] font-semibold text-ink">{{ g.risks.title }}</h2>
        <p class="mt-4 max-w-[62ch] text-[14.5px] leading-relaxed text-muted">{{ g.risks.lead }}</p>

        <ul class="mt-10 grid gap-x-12 gap-y-8 sm:grid-cols-2">
          <li v-for="r in g.risks.items" :key="r.title" class="border-t border-line pt-4">
            <h3 class="text-[14.5px] font-semibold text-ink">{{ r.title }}</h3>
            <p class="mt-2 max-w-[46ch] text-[14px] leading-relaxed text-muted">{{ r.body }}</p>
          </li>
        </ul>
      </div>
    </section>

    <!-- 建议：有先后顺序，所以单列推进 -->
    <section class="mx-auto max-w-6xl px-6 py-18">
      <h2 class="text-[1.35rem] font-semibold text-ink">{{ g.advice.title }}</h2>
      <ol class="mt-10">
        <li v-for="a in g.advice.items" :key="a.when"
            class="grid gap-x-8 gap-y-2 border-t border-line py-5 sm:grid-cols-[9rem_minmax(0,1fr)]">
          <p class="text-[14px] font-medium text-ink">{{ a.when }}</p>
          <p class="max-w-[58ch] text-[14.5px] leading-relaxed text-muted">{{ a.body }}</p>
        </li>
      </ol>
    </section>

    <!-- 入口 -->
    <section id="install" class="border-t border-line bg-surface-2">
      <div class="mx-auto max-w-4xl px-6 py-18">
        <h2 class="text-[1.35rem] font-semibold text-ink">{{ g.installTitle }}</h2>
        <p class="mt-4 max-w-[54ch] text-[14.5px] leading-relaxed text-muted">{{ g.installLead }}</p>

        <div class="mt-8 grid gap-5 sm:grid-cols-2">
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

        <p class="mt-4 text-[13px] text-faint">
          {{ g.install.listPrefix }}
          <code class="rounded border border-line bg-surface px-2 py-0.5 font-mono text-[12px] text-ink">{{ g.commands.all }}</code>
        </p>

        <div class="mt-10 flex flex-wrap items-center gap-x-7 gap-y-3 text-[15px]">
          <NuxtLink :to="href('/verify')"
                    class="rounded-lg border border-line px-5 py-2.5 font-medium text-ink hover:border-brand">
            {{ g.install.verify }}
          </NuxtLink>
          <a :href="site.contactUrl" class="py-1 text-brand-ink hover:underline">{{ g.install.ask }}</a>
        </div>
      </div>
    </section>

    <SiteFooter :t="t" :home-href="href('/')" :pricing-href="href('/pricing')" :terms-href="href('/legal/terms')" />
  </div>
</template>
