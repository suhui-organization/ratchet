<script setup lang="ts">
/**
 * 截图专用页（/shots/<name>）。
 *
 * 为什么不直接截产品页面：产品页面有导航、有响应式断点、有滚动位置，
 * 截出来的图尺寸不一致、边缘还带着半截导航。这里固定 1180px 宽、无导航，
 * 只渲染要展示的那一块，所以每次截出来的图都一样。
 *
 * 内容全部来自**真机输出**（见 docs/DELIVERY-0.2.0.md 的数字），不是编的。
 */
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()

const scan = [
  { t: '$ ratchet scan --home ~', c: 'cmd' },
  { t: '', c: 'out' },
  { t: '  harness   1（已解析 1）', c: 'out' },
  { t: '  server    16', c: 'out' },
  { t: '  未锁版本  12（同名包被替换时无法察觉）', c: 'flag' },
  { t: '', c: 'out' },
  { t: '      chrome-devtools     npx -y chrome-devtools-mcp@latest', c: 'flag' },
  { t: '      mcp-server-amap     npx -y @amap/amap-maps-mcp-server', c: 'flag' },
  { t: '      mcp-server-context7 npx -y @upstash/context7-mcp@latest', c: 'flag' },
  { t: '      mcp-server-memory   npx -y @modelcontextprotocol/server-memory', c: 'flag' },
  { t: '      mcp-server-playwright npx -y @playwright/mcp@latest', c: 'flag' },
  { t: '      mcp-server-filesystem /usr/local/bin/mcp-server-filesystem', c: 'out' },
  { t: '      x-docs              remote https://docs.x.com/mcp', c: 'out' },
] as const

const policy = [
  { t: '$ ratchet policy draft --from inventory.json --only-observed', c: 'cmd' },
  { t: '', c: 'out' },
  { t: '  工具      3 → allow 1 · approve 2 · deny 0', c: 'out' },
  { t: '  默认决策  deny（未登记的一律拒绝）', c: 'out' },
  { t: '', c: 'out' },
  { t: '  APPROVE  codex-tools/shell        名称命中「shell」；观测到 2 次调用', c: 'hold' },
  { t: '  APPROVE  codex-tools/write_file   名称命中「write」；观测到 1 次调用', c: 'hold' },
  { t: '  ALLOW    codex-tools/read_file    名称命中「read」；观测到 2 次调用', c: 'ok' },
  { t: '', c: 'out' },
  { t: '  从未被调用：codex-tools/delete_file  ← 不进策略，等于被收掉', c: 'dim' },
] as const

const verifyRows = [
  { file: 'report.md', digest: '4f9a1c22e0b7d8…', state: 'ok' },
  { file: 'policy.json', digest: 'b20e77a91d43af…', state: 'ok' },
  { file: 'evidence.json', digest: '—', state: 'absent' },
] as const

const name = computed(() => String(route.params.name || 'scan'))
const lines = computed(() => (name.value === 'policy' ? policy : scan))

const colorOf = (c: string) =>
  c === 'cmd' ? 'text-ink-050'
    : c === 'flag' ? 'text-flag'
      : c === 'ok' ? 'text-pass'
        : c === 'hold' ? 'text-ink-200'
          : c === 'dim' ? 'text-ink-400'
            : 'text-ink-400'
</script>

<template>
  <!-- 不留白：卡片的包围盒就是最终图片的边界，由截图时的 clip 精确裁切。
       上一版用 min-h-screen + place-items-center，导致卡片被 900px 高的画布居中，
       上下各留两百多像素黑边——图一半是空的。 -->
  <div class="inline-block bg-ink-950 p-0">
    <!-- 终端截图 -->
    <figure
      v-if="name !== 'verify'"
      class="w-[1120px] overflow-hidden rounded-xl border border-ink-800 bg-ink-900"
    >
      <div class="flex items-center gap-2 border-b border-ink-800 px-5 py-3.5">
        <span class="h-3 w-3 rounded-full bg-ink-700"></span>
        <span class="h-3 w-3 rounded-full bg-ink-800"></span>
        <span class="ml-3 font-mono text-[13px] text-ink-400">
          ratchet — {{ name === 'policy' ? 'least-privilege compile' : 'static scan, executes nothing' }}
        </span>
      </div>
      <pre class="px-7 py-6 font-mono text-[15px] leading-[2]"><code><span
        v-for="(l, i) in lines" :key="i" class="block whitespace-pre" :class="colorOf(l.c)">{{ l.t || ' ' }}</span></code></pre>
    </figure>

    <!-- 验证页截图 -->
    <figure v-else class="w-[1120px] overflow-hidden rounded-xl border border-ink-800 bg-ink-900">
      <div class="flex items-center gap-3 border-b border-ink-800 px-5 py-3">
        <span class="h-3 w-3 rounded-full bg-ink-700"></span>
        <span class="h-3 w-3 rounded-full bg-ink-800"></span>
        <span class="ml-3 rounded-md bg-ink-950 px-4 py-1.5 font-mono text-[12px] text-ink-400">
          localhost:3000/verify
        </span>
      </div>
      <div class="px-10 py-9">
        <h1 class="text-[26px] font-medium tracking-[-0.02em] text-ink-050">Independent verification</h1>
        <p class="mt-3 max-w-[64ch] text-[14px] leading-relaxed text-ink-400">
          Hashes are recomputed in your browser and compared with the manifest that shipped
          with the report. The verdict comes from your device.
        </p>

        <div class="mt-7 flex items-center gap-4 rounded-lg border border-pass/25 bg-pass/10 px-5 py-4">
          <span class="grid h-8 w-8 place-items-center rounded-full bg-pass/20">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" class="h-4 w-4 text-pass">
              <path d="M5 12.5 10 17.5 19 7" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </span>
          <div>
            <p class="text-[15px] font-medium text-ink-050">Verification passed</p>
            <p class="mt-0.5 text-[13px] text-ink-400">2 file(s) match manifest.json — this delivery has not changed since it was generated.</p>
          </div>
        </div>

        <table class="mt-7 w-full font-mono text-[13px]">
          <thead>
            <tr class="text-left text-[11px] tracking-wider text-ink-700 uppercase">
              <th class="border-b border-ink-800 pb-2.5">file</th>
              <th class="border-b border-ink-800 pb-2.5">sha256</th>
              <th class="border-b border-ink-800 pb-2.5 text-right">verdict</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in verifyRows" :key="row.file" class="border-b border-ink-900 last:border-0">
              <td class="py-3.5 text-ink-200">{{ row.file }}</td>
              <td class="tnum py-3.5 text-ink-700">{{ row.digest }}</td>
              <td class="py-3.5 text-right" :class="row.state === 'ok' ? 'text-pass' : 'text-ink-400'">
                {{ row.state === 'ok' ? 'match' : 'not in bundle' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </figure>
  </div>
</template>
