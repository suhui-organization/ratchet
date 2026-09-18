<script setup lang="ts">
import { ref } from 'vue'
import { parseBundle, verifyBundle, type VerifyResult } from '~/utils/verify'

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

const statusLabel: Record<string, string> = {
  ok: '一致',
  modified: '被改动',
  missing: '缺失',
  not_in_bundle: '不在包内',
}
</script>

<template>
  <main class="mx-auto max-w-3xl px-6 py-16">
    <h1 class="text-2xl font-semibold tracking-tight">独立验证</h1>
    <p class="mt-2 text-sm text-slate-600">
      把收到的 <code class="rounded bg-slate-100 px-1">share.json</code> 拖进来。
      哈希在你的浏览器里重算并与清单比对——<strong>结论由你的设备得出，不需要信任出具方</strong>。
    </p>

    <label
      class="mt-8 flex cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed px-6 py-12 text-center transition"
      :class="dragging ? 'border-slate-400 bg-slate-100' : 'border-slate-300 bg-white hover:bg-slate-50'"
      @dragover.prevent="dragging = true"
      @dragleave.prevent="dragging = false"
      @drop.prevent="onDrop"
    >
      <span class="text-sm font-medium">拖入 share.json，或点这里选择文件</span>
      <span class="mt-1 text-xs text-slate-500">文件只在本页读取，不会上传到任何服务器</span>
      <input type="file" accept="application/json,.json" class="hidden" @change="onPick" />
    </label>

    <p v-if="fileName" class="mt-3 text-xs text-slate-500">已读取：{{ fileName }}</p>
    <p v-if="error" class="mt-4 rounded border border-red-200 bg-red-50 p-3 text-sm text-red-800">
      {{ error }}
    </p>

    <template v-if="result && !error">
      <div
        class="mt-6 rounded-lg border p-4"
        :class="result.passed ? 'border-emerald-200 bg-emerald-50 text-emerald-900' : 'border-red-200 bg-red-50 text-red-900'"
      >
        <p class="font-medium">{{ result.passed ? '校验通过' : '校验失败' }}</p>
        <p class="mt-1 text-sm">
          <template v-if="result.passed">
            已校验 {{ result.checked }} 份产物，与清单完全一致——这份交付物自生成以来未被改动。
          </template>
          <template v-else-if="result.error">{{ result.error }}</template>
          <template v-else>
            {{ result.modified }} 份被改动、{{ result.missing }} 份缺失（共校验 {{ result.checked }} 份）。
          </template>
        </p>
        <p v-if="result.absent.length" class="mt-2 text-sm text-slate-700">
          另有 {{ result.absent.length }} 份不在包内，因此无法在这里验证：
          <span class="font-mono text-xs">{{ result.absent.join(', ') }}</span>
        </p>
      </div>

      <table class="mt-6 w-full border-collapse text-sm">
        <thead>
          <tr class="text-left text-xs uppercase tracking-wide text-slate-500">
            <th class="border-b border-slate-200 py-2">文件</th>
            <th class="border-b border-slate-200 py-2">结论</th>
            <th class="border-b border-slate-200 py-2">期望 sha256</th>
            <th class="border-b border-slate-200 py-2">实际 sha256</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in result.rows" :key="row.file">
            <td class="border-b border-slate-100 py-2 font-mono text-xs">{{ row.file }}</td>
            <td class="border-b border-slate-100 py-2">{{ statusLabel[row.status] }}</td>
            <td class="border-b border-slate-100 py-2 font-mono text-xs">{{ row.expected.slice(0, 12) }}…</td>
            <td class="border-b border-slate-100 py-2 font-mono text-xs">
              {{ row.actual ? row.actual.slice(0, 12) + '…' : '—' }}
            </td>
          </tr>
        </tbody>
      </table>
    </template>
  </main>
</template>
