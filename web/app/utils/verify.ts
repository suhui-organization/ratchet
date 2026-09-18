/**
 * 交付包的本地校验逻辑。
 *
 * 这里是整个 Web 端存在的理由：**哈希在收货方的浏览器里算**。
 * 如果哪天有人把它改成"服务端算好返回结论"，这一页就失去了意义——
 * 那只是换个人来要求你相信。
 *
 * 与 Python 侧 `verify.py` 的结论必须一致：两边算的都是
 * sha256(UTF-8 字节)，输入都是同一份 manifest 与同样的文件内容。
 */

export interface ManifestEntry {
  file: string
  sha256: string
  bytes?: number
}

export interface Manifest {
  format?: string
  generatedAt?: string
  artifacts: ManifestEntry[]
}

export interface BundleArtifact {
  file: string
  content: string
}

export interface Bundle {
  format?: string
  generatedAt?: string
  manifest: Manifest
  artifacts: BundleArtifact[]
}

/** 清单里有、但包里没有的文件（例如按设计不上云/不外发的原始审计链）。 */
export type RowStatus = 'ok' | 'modified' | 'missing' | 'not_in_bundle'

export interface VerifyRow {
  file: string
  expected: string
  actual: string
  status: RowStatus
}

export interface VerifyResult {
  rows: VerifyRow[]
  checked: number
  modified: number
  missing: number
  absent: string[]
  /** 只有"有东西被验过、且没有不一致、也没有缺失"才算通过 */
  passed: boolean
  error?: string
}

export async function sha256Hex(text: string): Promise<string> {
  const bytes = new TextEncoder().encode(text)
  const digest = await crypto.subtle.digest('SHA-256', bytes)
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

export function parseBundle(raw: string): Bundle {
  const parsed = JSON.parse(raw) as Bundle
  if (!parsed || typeof parsed !== 'object') throw new Error('不是合法的 JSON 对象')
  if (!parsed.manifest || !Array.isArray(parsed.manifest.artifacts)) {
    throw new Error('缺少 manifest.artifacts —— 这不是一份 Ratchet 交付包')
  }
  if (!Array.isArray(parsed.artifacts)) {
    throw new Error('缺少 artifacts —— 包里没有可校验的内容')
  }
  return parsed
}

export async function verifyBundle(bundle: Bundle): Promise<VerifyResult> {
  const entries = bundle.manifest.artifacts
  if (entries.length === 0) {
    // 空清单不能算通过：否则"什么都没验"会变成"验证通过"
    return { rows: [], checked: 0, modified: 0, missing: 0, absent: [], passed: false, error: '清单里没有列出任何产物，无法验证' }
  }

  const byName = new Map(bundle.artifacts.map((a) => [a.file, a.content]))
  const rows: VerifyRow[] = []
  for (const entry of entries) {
    if (!byName.has(entry.file)) {
      rows.push({ file: entry.file, expected: entry.sha256, actual: '', status: 'not_in_bundle' })
      continue
    }
    const actual = await sha256Hex(byName.get(entry.file) as string)
    rows.push({
      file: entry.file,
      expected: entry.sha256,
      actual,
      status: actual === entry.sha256 ? 'ok' : 'modified',
    })
  }

  const modified = rows.filter((r) => r.status === 'modified').length
  const missing = rows.filter((r) => r.status === 'missing').length
  const absent = rows.filter((r) => r.status === 'not_in_bundle').map((r) => r.file)
  const checked = rows.filter((r) => r.status === 'ok' || r.status === 'modified').length

  return {
    rows,
    checked,
    modified,
    missing,
    absent,
    passed: checked > 0 && modified === 0 && missing === 0,
  }
}
