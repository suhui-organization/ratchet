<script setup lang="ts">
import { ref } from 'vue'
import { parseBundle, verifyBundle, type VerifyResult } from '~/utils/verify'

const { locale, t, altHref, href } = useLocale()
const v = computed(() => t.value.verify)

const result = ref<VerifyResult | null>(null)
const error = ref('')
const fileName = ref('')
const dragging = ref(false)

async function handleFile(file: File) {
  error.value = ''
  result.value = null
  fileName.value = file.name
  try {
    const bundle = parseBundle(await file.text())
    result.value = await verifyBundle(bundle)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

function onDrop(event: DragEvent) {
  dragging.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file) void handleFile(file)
}

function onPick(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (file) void handleFile(file)
}

useHead(() => ({
  title: `${v.value.title} · Ratchet`,
  htmlAttrs: { lang: locale.value === 'zh' ? 'zh-CN' : 'en' },
}))
</script>

<template>
  <main class="mx-auto max-w-3xl px-6 py-16">
    <nav class="mb-12 flex items-center gap-5 text-sm text-muted">
      <NuxtLink :to="href('/')" class="py-1 font-semibold tracking-tight text-ink hover:text-brand">Ratchet</NuxtLink>
      <a :href="altHref" class="ml-auto py-1 whitespace-nowrap hover:text-brand">{{ t.switchLabel }}</a>
    </nav>

    <h1 class="text-[1.9rem] font-semibold tracking-[-0.025em] text-ink">{{ v.title }}</h1>
    <p class="mt-3 max-w-[62ch] text-[15px] leading-relaxed text-muted">
      {{ v.lead1 }}<code class="rounded border border-line bg-surface-2 px-1.5 py-0.5 font-mono text-[13px] text-ink">share.json</code>{{ v.lead2 }}
      <strong class="font-medium text-ink">{{ v.leadStrong }}</strong>{{ v.leadDot }}
    </p>

    <label
      class="mt-8 flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed px-6 py-12 text-center"
      :class="dragging ? 'border-brand bg-surface-2' : 'border-line bg-surface hover:bg-surface-2'"
      @dragover.prevent="dragging = true"
      @dragleave.prevent="dragging = false"
      @drop.prevent="onDrop"
    >
      <span class="text-[15px] font-medium text-ink">{{ v.dropTitle }}</span>
      <span class="mt-1 text-[13px] text-faint">{{ v.dropNote }}</span>
      <input type="file" accept="application/json,.json" class="hidden" @change="onPick" />
    </label>

    <p v-if="fileName" class="mt-3 text-[13px] text-faint">{{ v.readFile }}：{{ fileName }}</p>
    <p v-if="error" class="mt-4 rounded-lg border border-fail/40 bg-surface-2 p-3 text-[14px] text-fail">
      {{ error }}
    </p>

    <template v-if="result && !error">
      <div class="mt-6 rounded-xl border p-4"
           :class="result.passed ? 'border-pass/40 bg-surface-2' : 'border-fail/40 bg-surface-2'">
        <p class="font-medium" :class="result.passed ? 'text-pass' : 'text-fail'">
          {{ result.passed ? v.passed : v.failed }}
        </p>
        <p class="mt-1.5 text-[14px] leading-relaxed text-muted">
          <template v-if="result.passed">{{ v.okNote(result.checked) }}</template>
          <template v-else-if="result.error">{{ result.error }}</template>
          <template v-else>{{ v.badNote(result.modified, result.missing, result.checked) }}</template>
        </p>
        <p v-if="result.absent.length" class="mt-2 text-[14px] leading-relaxed text-muted">
          {{ v.absentNote(result.absent.length) }}
          <span class="font-mono text-[13px] break-all text-ink">{{ result.absent.join(', ') }}</span>
        </p>
      </div>

      <!-- 窄屏堆叠，宽屏成列。哈希列在手机上没法横着读。 -->
      <div class="mt-6">
        <div class="hidden border-b border-line pb-2 sm:grid sm:grid-cols-[1.4fr_5rem_1fr_1fr] sm:gap-x-4">
          <span class="text-[12px] font-medium tracking-wide text-faint uppercase">{{ v.table.file }}</span>
          <span class="text-[12px] font-medium tracking-wide text-faint uppercase">{{ v.table.verdict }}</span>
          <span class="text-[12px] font-medium tracking-wide text-faint uppercase">{{ v.table.expected }}</span>
          <span class="text-[12px] font-medium tracking-wide text-faint uppercase">{{ v.table.actual }}</span>
        </div>
        <ul class="divide-y divide-line border-t border-line sm:border-t-0">
          <li v-for="row in result.rows" :key="row.file"
              class="grid gap-x-4 gap-y-1 py-3 sm:grid-cols-[1.4fr_5rem_1fr_1fr]">
            <span class="font-mono text-[12.5px] break-all text-ink">{{ row.file }}</span>
            <span class="text-[13.5px]"
                  :class="row.status === 'ok' ? 'text-pass' : 'text-fail'">{{ v.status[row.status] }}</span>
            <span class="font-mono text-[12px] break-all text-faint">{{ row.expected.slice(0, 12) }}…</span>
            <span class="font-mono text-[12px] break-all text-faint">
              {{ row.actual ? row.actual.slice(0, 12) + '…' : v.dash }}
            </span>
          </li>
        </ul>
      </div>
    </template>
  </main>
</template>
