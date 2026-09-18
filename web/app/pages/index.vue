<script setup lang="ts">
import { ref } from 'vue'
import { site } from '~/site'

const copied = ref('')

async function copy(text: string, key: string) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = key
    setTimeout(() => (copied.value = ''), 1500)
  } catch {
    copied.value = ''
  }
}

const quickStart = [
  { key: 'build', label: 'Build from source', lines: ['cd ratchet && make build', './bin/ratchet version'] },
  { key: 'scan', label: 'Scan your machine (reads configs, runs nothing)', lines: ['./bin/ratchet scan --home ~'] },
  { key: 'policy', label: 'Compile a least-privilege policy', lines: ['./bin/ratchet policy draft --from inventory.json --out policy.json'] },
]

const principles = [
  {
    title: 'Privileges are compiled',
    body: 'Static scans tell you what an agent could reach. Hand-written policies ask you to guess which part is actually needed. Ratchet reads both the tool surface and the calls that really happened, then compiles the policy — and every verdict carries the reason it was made.',
  },
  {
    title: 'The recipient verifies, not you',
    body: 'A delivery ships with a sha256 manifest. Whoever you hand it to recomputes every hash on their own machine, in a browser or with one standard-library script. The conclusion comes from their device, so they never have to trust yours.',
  },
  {
    title: 'Nothing has to leave the machine',
    body: 'Scanning reads config files and executes nothing. Collection is a hook that fails silently rather than blocking your agent. Tool arguments are never recorded — they routinely contain file contents and tokens, and compiling least privilege does not need them.',
  },
]
</script>

<template>
  <div class="min-h-screen bg-slate-950 text-slate-200">
    <header class="sticky top-0 z-20 border-b border-white/10 bg-slate-950/80 backdrop-blur">
      <nav class="mx-auto flex max-w-6xl items-center gap-4 px-6 py-4 text-sm">
        <span class="font-semibold tracking-tight text-white">{{ site.name }}</span>
        <span class="rounded border border-white/15 px-2 py-0.5 text-xs text-slate-400">{{ site.version }}</span>
        <div class="ml-auto flex items-center gap-5 text-slate-400">
          <a href="#start" class="hover:text-white">Quick start</a>
          <a href="#approach" class="hover:text-white">Design</a>
          <a href="#limits" class="hover:text-white">Limits</a>
          <NuxtLink to="/verify" class="hover:text-white">Verify</NuxtLink>
        </div>
      </nav>
    </header>

    <section class="mx-auto max-w-6xl px-6 pt-20 pb-16">
      <p class="text-sm font-medium text-sky-400">{{ site.tagline.eyebrow }}</p>
      <h1 class="mt-3 max-w-3xl text-4xl font-semibold tracking-tight text-white sm:text-5xl">
        {{ site.tagline.title }}
      </h1>
      <p class="mt-6 max-w-2xl text-lg leading-relaxed text-slate-400">{{ site.tagline.lead }}</p>

      <div class="mt-8 flex flex-wrap items-center gap-3 text-sm">
        <a v-if="site.downloadUrl" :href="site.downloadUrl"
           class="rounded-lg bg-white px-4 py-2 font-medium text-slate-900 hover:bg-slate-200">
          Download {{ site.name }} {{ site.version }}
        </a>
        <a v-else :href="site.contactUrl"
           class="rounded-lg bg-white px-4 py-2 font-medium text-slate-900 hover:bg-slate-200">
          Request access
        </a>
        <a href="#start" class="rounded-lg border border-white/15 px-4 py-2 hover:border-white/30">Quick start</a>
        <NuxtLink to="/verify" class="rounded-lg border border-white/15 px-4 py-2 hover:border-white/30">
          Verify a delivery
        </NuxtLink>
      </div>
    </section>

    <section id="start" class="border-t border-white/10 bg-slate-900/40">
      <div class="mx-auto max-w-6xl px-6 py-16">
        <h2 class="text-2xl font-semibold text-white">Quick start</h2>
        <p class="mt-2 text-slate-400">{{ site.sourceNote }}</p>

        <div class="mt-8 grid gap-4 md:grid-cols-3">
          <div v-for="block in quickStart" :key="block.key" class="rounded-xl border border-white/10 bg-slate-950 p-5">
            <div class="flex items-start justify-between gap-3">
              <h3 class="text-sm font-medium text-slate-300">{{ block.label }}</h3>
              <button class="shrink-0 rounded border border-white/15 px-2 py-1 text-xs text-slate-400 hover:text-white"
                      @click="copy(block.lines.join('\n'), block.key)">
                {{ copied === block.key ? 'Copied' : 'Copy' }}
              </button>
            </div>
            <pre class="mt-3 overflow-x-auto text-xs leading-relaxed text-slate-300"><code><span v-for="(line, i) in block.lines" :key="i" class="block"><span class="text-slate-600">$ </span>{{ line }}</span></code></pre>
          </div>
        </div>

        <div class="mt-4 rounded-xl border border-white/10 bg-slate-950 p-5">
          <h3 class="text-sm font-medium text-slate-300">Download</h3>
          <template v-if="site.downloadUrl">
            <p class="mt-2 text-sm text-slate-400">
              Prebuilt binary for Linux amd64, published alongside its sha256.
              Verify it before you run it — that habit is the whole point of this project.
            </p>
            <a :href="site.downloadUrl" class="mt-3 inline-block break-all text-sm text-sky-400 hover:text-sky-300">
              {{ site.downloadUrl }}
            </a>
          </template>
          <template v-else>
            <p class="mt-2 text-sm text-slate-400">
              No public build yet — Ratchet is in developer preview and I am handing it to a small
              number of people first. Ask for access and I will send a build together with its sha256:
            </p>
            <a :href="site.contactUrl" class="mt-3 inline-block text-sm text-sky-400 hover:text-sky-300">
              Request access →
            </a>
          </template>
        </div>
      </div>
    </section>

    <section id="approach" class="mx-auto max-w-6xl px-6 py-16">
      <h2 class="text-2xl font-semibold text-white">Design approach</h2>
      <div class="mt-8 grid gap-8 md:grid-cols-3">
        <article v-for="p in principles" :key="p.title">
          <h3 class="text-base font-medium text-white">{{ p.title }}</h3>
          <p class="mt-3 text-sm leading-relaxed text-slate-400">{{ p.body }}</p>
        </article>
      </div>
    </section>

    <section id="limits" class="border-t border-white/10 bg-slate-900/40">
      <div class="mx-auto max-w-6xl px-6 py-14">
        <h2 class="text-2xl font-semibold text-white">What it does not do</h2>
        <ul class="mt-6 grid gap-3 text-sm text-slate-400 md:grid-cols-2">
          <li>· It is not a sandbox and does not containerise anything.</li>
          <li>· It does not block tool calls. It decides what should be allowed and proves what happened.</li>
          <li>· A static scan only sees configs in formats it parses, and reports what it could not parse instead of guessing.</li>
          <li>· It records tool names, not arguments — so it cannot tell you what was inside a call.</li>
        </ul>
      </div>
    </section>

    <footer class="border-t border-white/10">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-3 px-6 py-8 text-xs text-slate-500">
        <span>{{ site.name }} {{ site.version }}</span>
        <span>·</span>
        <span>Developer preview — output formats will change.</span>
        <NuxtLink to="/verify" class="ml-auto hover:text-slate-300">Verify a delivery</NuxtLink>
      </div>
    </footer>
  </div>
</template>
