<script setup lang="ts">
import { ref } from 'vue'
import { site } from '~/site'

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
const blocks = site.installUrl
  ? [
      { key: 'install', label: 'Install', lines: [`curl -fsSL ${site.installUrl}/install.sh | sh`] },
      { key: 'scan', label: 'Then scan your machine', lines: ['ratchet scan --home ~'] },
    ]
  : [
      { key: 'build', label: 'Build from source', lines: ['cd ratchet && make build', 'make dist   # 4 platforms, each with a sha256'] },
      { key: 'scan', label: 'Scan your machine', lines: ['./bin/ratchet scan --home ~'] },
    ]

const features = [
  {
    title: 'Reads what is really there',
    body: 'Agent harnesses keep MCP servers in a dozen different config formats. Ratchet parses them, executes nothing, and never records the values of the environment variables where your tokens live.',
  },
  {
    title: 'Compiles the policy',
    body: 'Capability is inferred from tool names and descriptions, then crossed with the calls that actually happened. Read, write, execute and destructive each land somewhere different — and every verdict carries the reason it was made.',
  },
  {
    title: 'Proves what was handed over',
    body: 'A delivery ships with a sha256 manifest. Whoever receives it recomputes every hash on their own machine, so the conclusion never depends on trusting the sender.',
  },
] as const

const limits = [
  'Not a sandbox — it does not contain anything.',
  'Does not block tool calls.',
  'A static scan reports configs it could not parse instead of guessing.',
  'Records tool names, never arguments.',
] as const
</script>

<template>
  <div class="min-h-screen bg-surface">
    <header class="sticky top-0 z-20 border-b border-line bg-surface">
      <nav class="mx-auto flex max-w-6xl items-center gap-3 px-6 py-4 text-sm">
        <span class="font-semibold tracking-tight text-ink">{{ site.name }}</span>
        <span class="tnum rounded border border-line px-1.5 py-0.5 font-mono text-[11px] text-faint">
          {{ site.version }}
        </span>
        <span class="hidden text-[12px] text-faint sm:inline">developer preview</span>
        <div class="ml-auto flex items-center gap-6 text-muted">
          <a href="#how" class="hover:text-brand">How it works</a>
          <a href="#limits" class="hover:text-brand">Limits</a>
          <NuxtLink to="/verify" class="hover:text-brand">Verify</NuxtLink>
        </div>
      </nav>
    </header>

    <!-- Hero：居中大标题 + 导语 + 入口，与参照页同一节奏 -->
    <section class="mx-auto max-w-4xl px-6 pt-24 pb-16 text-center">
      <h1 class="rise text-[2.75rem] leading-[1.08] font-semibold tracking-[-0.035em] text-ink sm:text-[3.6rem]">
        Privileges are compiled,<br />not hand-written.
      </h1>
      <p class="rise rise-2 mx-auto mt-6 max-w-[52ch] text-[17px] leading-relaxed text-muted">
        Ratchet reads what your AI agents can actually reach, compiles that into a
        least-privilege policy, and hands over evidence the recipient verifies on their own machine.
      </p>
      <div class="rise rise-2 mt-8 flex flex-wrap items-center justify-center gap-x-7 gap-y-3 text-[15px]">
        <a :href="site.contactUrl"
           class="rounded-lg bg-brand px-5 py-2.5 font-medium text-white hover:bg-brand-ink">
          Request access
        </a>
        <a href="#start" class="text-brand hover:text-brand-ink hover:underline">Quick start</a>
        <NuxtLink to="/verify" class="text-brand hover:text-brand-ink hover:underline">Verify a delivery</NuxtLink>
      </div>
    </section>

    <!-- Quick start：两块深色代码块并排，带复制 -->
    <section class="mx-auto max-w-4xl px-6 pb-20">
      <div class="grid gap-5 sm:grid-cols-2">
        <div v-for="b in blocks" :key="b.key" class="overflow-hidden rounded-xl bg-code">
          <div class="flex items-center justify-between border-b border-white/10 px-4 py-2.5">
            <span class="text-[12px] text-white/60">{{ b.label }}</span>
            <button class="font-mono text-[11px] text-white/50 hover:text-white"
                    @click="copy(b.lines.join('\n'), b.key)">
              {{ copied === b.key ? 'copied' : 'Copy' }}
            </button>
          </div>
          <pre class="overflow-x-auto px-4 py-4 font-mono text-[12.5px] leading-[2]"><code><span
            v-for="(l, i) in b.lines" :key="i" class="block text-white/85"><span class="text-white/35">$ </span>{{ l }}</span></code></pre>
        </div>
      </div>
      <p class="mt-4 text-center text-[13px] text-faint">
        {{ site.sourceNote }}
      </p>
    </section>

    <!-- 主张 + 真数据 -->
    <section id="how" class="border-y border-line bg-surface-2">
      <div class="mx-auto grid max-w-6xl items-center gap-12 px-6 py-20 lg:grid-cols-2">
        <div>
          <h2 class="text-[1.9rem] leading-tight font-semibold tracking-[-0.025em] text-ink">
            A compiler, not another scanner.
          </h2>
          <p class="mt-5 max-w-[46ch] text-[15px] leading-relaxed text-muted">
            Scanners tell you what an agent could reach. Ratchet tells you what it should be
            allowed to do — by reading the surface and the calls that actually happened.
          </p>
        </div>
        <div>
          <div class="flex flex-wrap gap-1.5" aria-hidden="true">
            <span v-for="i in 16" :key="i" class="h-4 w-4 rounded-[3px]"
                  :class="i <= 12 ? 'bg-brand' : 'bg-line'"></span>
          </div>
          <p class="mt-5 max-w-[52ch] text-[14px] leading-relaxed text-muted">
            On one real developer machine: <span class="tnum font-mono text-ink">12 / 16</span> MCP servers
            are launched from a package manager, and none of them pin a version. Three say
            <span class="font-mono text-ink">@latest</span> — which reads like a pin and resolves like nothing.
          </p>
        </div>
      </div>
    </section>

    <!-- 三个能力块 -->
    <section class="mx-auto max-w-6xl px-6 py-20">
      <div class="grid gap-10 md:grid-cols-3">
        <article v-for="f in features" :key="f.title">
          <h3 class="text-[17px] font-semibold text-ink">{{ f.title }}</h3>
          <p class="mt-3 text-[14.5px] leading-relaxed text-muted">{{ f.body }}</p>
        </article>
      </div>
    </section>

    <!-- 设计主张 -->
    <section class="border-y border-line bg-surface-2">
      <div class="mx-auto max-w-6xl px-6 py-20">
        <h2 class="text-[1.9rem] leading-tight font-semibold tracking-[-0.025em] text-ink">
          Design approach: least privilege that comes from evidence.
        </h2>
        <div class="mt-10 grid gap-10 md:grid-cols-2">
          <div>
            <h3 class="text-[17px] font-semibold text-ink">Nothing is executed to learn the surface</h3>
            <p class="mt-3 text-[14.5px] leading-relaxed text-muted">
              Discovery reads config files. Connecting to a server to list its tools is a separate,
              explicitly requested step, because that one starts processes.
            </p>
          </div>
          <div>
            <h3 class="text-[17px] font-semibold text-ink">The recipient is the one who checks</h3>
            <p class="mt-3 text-[14.5px] leading-relaxed text-muted">
              Hashes are recomputed on the reader's machine — in a browser, or with one
              standard-library script. Nothing about the verdict depends on trusting the sender.
            </p>
          </div>
        </div>
      </div>
    </section>

    <!-- 边界 -->
    <section id="limits" class="mx-auto max-w-6xl px-6 py-16">
      <h2 class="text-[1.35rem] font-semibold text-ink">What it does not do</h2>
      <ul class="mt-6 grid gap-x-10 gap-y-3 text-[14.5px] text-muted sm:grid-cols-2">
        <li v-for="l in limits" :key="l" class="flex gap-3">
          <span class="mt-[0.6em] h-px w-3.5 shrink-0 bg-line"></span>{{ l }}
        </li>
      </ul>
    </section>

    <!-- 再给一次入口 -->
    <section id="start" class="border-t border-line bg-surface-2">
      <div class="mx-auto max-w-4xl px-6 py-20 text-center">
        <h2 class="text-[1.9rem] font-semibold tracking-[-0.025em] text-ink">Get started</h2>
        <p class="mx-auto mt-4 max-w-[48ch] text-[15px] leading-relaxed text-muted">
          {{ site.installUrl ? "One command. It downloads the binary for your platform and verifies its sha256 before installing." : "No public build yet — it is going to a small number of people first. The source is not public either; ask and I will send a build with its sha256." }}
        </p>
        <div class="mt-8 flex flex-wrap items-center justify-center gap-x-7 gap-y-3 text-[15px]">
          <a v-if="site.downloadUrl" :href="site.downloadUrl"
             class="rounded-lg bg-brand px-5 py-2.5 font-medium text-white hover:bg-brand-ink">
            Download {{ site.version }}
          </a>
          <a v-else :href="site.contactUrl"
             class="rounded-lg bg-brand px-5 py-2.5 font-medium text-white hover:bg-brand-ink">
            Request access
          </a>
          <a href="#how" class="text-brand hover:text-brand-ink hover:underline">How it works</a>
        </div>

        <div class="mt-10 grid gap-5 text-left sm:grid-cols-2">
          <div v-for="b in blocks" :key="'x-' + b.key" class="overflow-hidden rounded-xl bg-code">
            <div class="flex items-center justify-between border-b border-white/10 px-4 py-2.5">
              <span class="text-[12px] text-white/60">{{ b.label }}</span>
              <button class="font-mono text-[11px] text-white/50 hover:text-white"
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

    <footer class="border-t border-line">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-x-4 gap-y-2 px-6 py-8 text-[13px] text-faint">
        <span class="font-medium text-ink">{{ site.name }}</span>
        <span>Apache-2.0</span>
        <span>·</span>
        <span>© 2026</span>
        <span class="text-faint">developer preview — output formats will change</span>
        <NuxtLink to="/verify" class="ml-auto hover:text-brand">Verify a delivery</NuxtLink>
      </div>
    </footer>
  </div>
</template>
