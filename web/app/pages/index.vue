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

/** 真实的扫描输出（本机实测，2026-09-18）。不美化、不补全。 */
const scanLines = [
  { t: '$ ratchet scan --home ~', c: 'cmd' },
  { t: '  harness   1   server   16', c: 'out' },
  { t: '  unpinned  12', c: 'flag' },
  { t: '      chrome-devtools     npx -y chrome-devtools-mcp@latest', c: 'flag' },
  { t: '      mcp-server-amap     npx -y @amap/amap-maps-mcp-server', c: 'flag' },
  { t: '      mcp-server-memory   npx -y @modelcontextprotocol/server-memory', c: 'flag' },
  { t: '      mcp-server-filesystem  /usr/local/bin/mcp-server-filesystem', c: 'out' },
  { t: '      x-docs              remote https://docs.x.com/mcp', c: 'out' },
] as const

const flow = [
  { label: 'Reads configs', note: 'runs nothing', icon: 'scan' },
  { label: 'Compiles policy', note: 'with reasons', icon: 'compile' },
  { label: 'Hashes delivery', note: 'per file', icon: 'hash' },
  { label: 'You verify', note: 'in your browser', icon: 'verify' },
] as const

const verifyRows = [
  { file: 'report.md', digest: '4f9a1c22e0b7…', state: 'ok' },
  { file: 'policy.json', digest: 'b20e77a91d43…', state: 'ok' },
  { file: 'evidence.json', digest: '—', state: 'absent' },
] as const

const limits = [
  'Not a sandbox. It does not contain anything.',
  'Does not block tool calls.',
  'Static scan: unparsed configs are reported, never guessed.',
  'Records tool names, not arguments.',
]
</script>

<template>
  <div class="min-h-screen bg-ink-950">
    <header class="border-b border-ink-800">
      <nav class="mx-auto flex max-w-6xl items-center gap-3 px-6 py-4 text-sm">
        <span class="font-medium tracking-tight text-ink-050">{{ site.name }}</span>
        <span class="tnum rounded-sm border border-ink-700 px-1.5 py-0.5 font-mono text-[11px] text-ink-400">
          {{ site.version }}
        </span>
        <span class="hidden text-[11px] text-ink-400 sm:inline">developer preview</span>
        <div class="ml-auto flex items-center gap-6 text-ink-400">
          <a href="#how" class="hover:text-ink-050">How</a>
          <a href="#start" class="hover:text-ink-050">Start</a>
          <NuxtLink to="/verify" class="hover:text-ink-050">Verify</NuxtLink>
        </div>
      </nav>
    </header>

    <section class="mx-auto grid max-w-6xl items-center gap-14 px-6 py-20 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)]">
      <div>
        <h1 class="rise text-[2.6rem] leading-[1.05] font-medium tracking-[-0.03em] text-ink-050 sm:text-[3.2rem]">
          Privileges are compiled, not hand-written.
        </h1>
        <p class="rise rise-2 mt-6 max-w-[46ch] text-[15px] leading-relaxed text-ink-400">
          Ratchet reads what your agents can actually reach, compiles that into a
          least-privilege policy, and hands over evidence the recipient verifies on their own machine.
        </p>
        <div class="rise rise-3 mt-8 flex flex-wrap items-center gap-3 text-sm">
          <a v-if="site.downloadUrl" :href="site.downloadUrl"
             class="rounded-md bg-ink-050 px-4 py-2 font-medium text-ink-950 hover:bg-white">
            Download {{ site.version }}
          </a>
          <a v-else :href="site.contactUrl"
             class="rounded-md bg-ink-050 px-4 py-2 font-medium text-ink-950 hover:bg-white">
            Request access
          </a>
          <a href="#how" class="rounded-md border border-ink-700 px-4 py-2 text-ink-200 hover:border-ink-400">
            See how it works
          </a>
        </div>
      </div>

      <!-- 真机截图：1280×900 的原始运行输出，不是重画的示意图 -->
      <figure class="rise rise-2">
        <img
          src="/shots/scan.jpg" width="1280" height="900" loading="eager" decoding="async"
          alt="ratchet scan output: 16 MCP servers found, 12 unpinned, with package names listed in orange"
          class="w-full rounded-lg border border-ink-800 shadow-[0_24px_60px_-30px_rgba(0,0,0,0.95)]"
        />
      </figure>
    </section>

    <!-- 第二个真机截图：编译产物长什么样 -->
    <section class="mx-auto max-w-6xl px-6 pb-20">
      <figure>
        <img
          src="/shots/policy.jpg" width="1280" height="900" loading="lazy" decoding="async"
          alt="ratchet policy draft output: three tools resolved to allow and approve, each with the reason it was decided that way"
          class="w-full rounded-lg border border-ink-800 shadow-[0_24px_60px_-30px_rgba(0,0,0,0.95)]"
        />
        <figcaption class="mt-3 text-[13px] text-ink-400">
          One command turns the surface into a policy. Every verdict carries the reason it was made —
          and the tools that were never called don't make it in at all.
        </figcaption>
      </figure>
    </section>

    <section class="border-y border-ink-800 bg-ink-900/40">
      <div class="mx-auto flex max-w-6xl flex-col gap-6 px-6 py-12 sm:flex-row sm:items-center sm:gap-12">
        <div class="flex flex-wrap gap-1.5" aria-hidden="true">
          <span v-for="i in 16" :key="i" class="h-3.5 w-3.5 rounded-[3px]"
                :class="i <= 12 ? 'bg-flag' : 'bg-ink-800'"></span>
        </div>
        <p class="max-w-[62ch] text-sm leading-relaxed text-ink-400">
          <span class="tnum font-mono text-ink-050">12 / 16</span>
          servers on one real developer machine are launched from a package manager, and
          <span class="text-flag">none of them pin a version</span> — three say
          <span class="font-mono text-ink-200">@latest</span>, which reads like a pin and resolves like nothing.
        </p>
      </div>
    </section>

    <section id="how" class="mx-auto max-w-6xl px-6 py-20">
      <ol class="grid gap-10 sm:grid-cols-2 lg:grid-cols-4">
        <li v-for="(step, i) in flow" :key="step.label" class="relative">
          <div class="flex items-center gap-3">
            <span class="grid h-10 w-10 place-items-center rounded-md border border-ink-800 bg-ink-900">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"
                   stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5 text-ink-200">
                <template v-if="step.icon === 'scan'">
                  <circle cx="10.5" cy="10.5" r="6.5" /><path d="M15.5 15.5 21 21" />
                </template>
                <template v-else-if="step.icon === 'compile'">
                  <path d="M4 7h9M4 12h13M4 17h7" /><path d="M18 4.5v15" />
                </template>
                <template v-else-if="step.icon === 'hash'">
                  <rect x="3.5" y="3.5" width="17" height="17" rx="3" />
                  <path d="M9 3.5v17M15 3.5v17M3.5 9h17M3.5 15h17" />
                </template>
                <template v-else>
                  <path d="M12 3.5 5 6.5v5c0 4 3 8 7 9 4-1 7-5 7-9v-5z" /><path d="M9 12l2 2 4-4" />
                </template>
              </svg>
            </span>
            <span class="tnum font-mono text-[11px] text-ink-700">{{ String(i + 1).padStart(2, '0') }}</span>
          </div>
          <p class="mt-4 text-[15px] text-ink-050">{{ step.label }}</p>
          <p class="mt-1 text-[13px] text-ink-400">{{ step.note }}</p>
          <span v-if="i < flow.length - 1"
                class="absolute top-5 right-[-1.25rem] hidden h-px w-10 bg-ink-800 lg:block"></span>
        </li>
      </ol>
    </section>

    <section class="border-y border-ink-800 bg-ink-900/40">
      <div class="mx-auto grid max-w-6xl items-center gap-14 px-6 py-20 lg:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)]">
        <div>
          <h2 class="text-[1.6rem] leading-tight font-medium tracking-[-0.02em] text-ink-050">
            The recipient verifies. You don't ask them to trust you.
          </h2>
          <p class="mt-5 max-w-[52ch] text-sm leading-relaxed text-ink-400">
            Every delivery carries a sha256 manifest. Hashes are recomputed on the reader's
            machine — in a browser, or with one standard-library script.
          </p>
          <NuxtLink to="/verify" class="mt-6 inline-flex items-center gap-2 text-sm text-ink-200 hover:text-ink-050">
            Open the verifier
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="h-4 w-4">
              <path d="M5 12h14M13 6l6 6-6 6" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </NuxtLink>
        </div>

        <figure>
          <img
            src="/shots/verify.jpg" width="1280" height="900" loading="lazy" decoding="async"
            alt="The verifier page: a green 'Verification passed' bar and a table of three files with match / not in bundle verdicts"
            class="w-full rounded-lg border border-ink-800 shadow-[0_24px_60px_-30px_rgba(0,0,0,0.95)]"
          />
        </figure>
      </div>
    </section>

    <section class="mx-auto max-w-6xl px-6 py-16">
      <h2 class="text-sm font-medium text-ink-050">What it does not do</h2>
      <ul class="mt-5 grid gap-x-10 gap-y-2.5 text-[13px] text-ink-400 sm:grid-cols-2">
        <li v-for="l in limits" :key="l" class="flex gap-3">
          <span class="mt-[0.55em] h-px w-3 shrink-0 bg-ink-700"></span>{{ l }}
        </li>
      </ul>
    </section>

    <section id="start" class="border-t border-ink-800 bg-ink-900/40">
      <div class="mx-auto max-w-6xl px-6 py-20">
        <h2 class="text-[1.6rem] font-medium tracking-[-0.02em] text-ink-050">Get started</h2>
        <div class="mt-8 grid gap-4 md:grid-cols-2">
          <div
            v-for="b in [
              { key: 'a', label: 'Build', lines: ['cd ratchet && make build', 'make dist'] },
              { key: 'b', label: 'Scan (reads configs, runs nothing)', lines: ['./bin/ratchet scan --home ~'] },
            ]"
            :key="b.key"
            class="overflow-hidden rounded-lg border border-ink-800 bg-ink-950"
          >
            <div class="flex items-center justify-between border-b border-ink-800 px-4 py-2.5">
              <span class="text-[12px] text-ink-400">{{ b.label }}</span>
              <button
                class="rounded-sm border border-ink-700 px-2 py-0.5 font-mono text-[11px] text-ink-400 hover:text-ink-050"
                @click="copy(b.lines.join('\n'), b.key)"
              >
                {{ copied === b.key ? 'copied' : 'copy' }}
              </button>
            </div>
            <pre class="overflow-x-auto px-4 py-3 font-mono text-[12px] leading-[2]"><code><span
              v-for="(l, i) in b.lines" :key="i" class="block text-ink-200"><span class="text-ink-700">$ </span>{{ l }}</span></code></pre>
          </div>
        </div>

        <div class="mt-6 flex flex-col gap-3 rounded-lg border border-ink-800 bg-ink-950 px-5 py-4 sm:flex-row sm:items-center">
          <template v-if="site.downloadUrl">
            <p class="text-[13px] text-ink-400">Prebuilt Linux binary, published next to its sha256.</p>
            <a :href="site.downloadUrl" class="break-all text-[13px] text-flag hover:underline sm:ml-auto">{{ site.downloadUrl }}</a>
          </template>
          <template v-else>
            <p class="max-w-[58ch] text-[13px] text-ink-400">
              No public build yet — it is going to a small number of people first.
              Ask for access and I'll send a build with its sha256.
            </p>
            <a :href="site.contactUrl" class="shrink-0 text-[13px] text-flag hover:underline sm:ml-auto">Request access →</a>
          </template>
        </div>
      </div>
    </section>

    <footer class="border-t border-ink-800">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-3 px-6 py-8 text-[12px] text-ink-700">
        <span>{{ site.name }} {{ site.version }}</span><span>·</span>
        <span>developer preview — output formats will change</span>
        <NuxtLink to="/verify" class="ml-auto hover:text-ink-400">Verify a delivery</NuxtLink>
      </div>
    </footer>
  </div>
</template>
